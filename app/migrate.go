package app

import (
	"bufio"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/TravisS25/webutil/webutil"
	"github.com/golang-migrate/migrate/v4"
	"github.com/pkg/errors"
)

const (
	MigrateTypeUp    MigrationType = "Up"
	MigrateTypeDown  MigrationType = "Down"
	MigrateTypeForce MigrationType = "Force"
)

var (
	ErrInvalidFileName = fmt.Errorf("invalid sql file name;  proper naming:<version>_<description>.<'up'|'down'>.sql")
)

var postgresMigrationTableSearch = func(db webutil.DBInterface) error {
	var filler interface{}

	return db.QueryRowx(
		`
		select 
			table_name 
		from 
			information_schema.tables 
		where  
			table_schema = 'public'
		and    
			table_name = 'schema_migrations' 
		`,
	).Scan(&filler)
}

var DefaultMigrateNameCfg = MigrateNameConfig{
	TargetVersion: FlagName{
		LongHand:  "target-version",
		ShortHand: "t",
	},
	RollbackOnFailure: FlagName{
		LongHand:  "rollback-on-failure",
		ShortHand: "f",
	},
	MigrationsDir: FlagName{
		LongHand:  "migrations-dir",
		ShortHand: "m",
	},
	ResetDirtyFlag: FlagName{
		LongHand:  "reset-dirty-flag",
		ShortHand: "r",
	},
}

type CustomMigration struct {
	Up   CustomMigrationFunc
	Down CustomMigrationFunc
}

type MigrateFlagsConfig struct {
	TargetVersion     int    `yaml:"target_version" mapstructure:"target_version"`
	RollbackOnFailure bool   `yaml:"rollback_on_failure" mapstructure:"rollback_on_failure"`
	ResetDirtyFlag    bool   `yaml:"reset_dirty_flag" mapstructure:"reset_dirty_flag"`
	MigrationsDir     string `yaml:"migrations_dir" mapstructure:"migrations_dir"`
}

type MigrateNameConfig struct {
	ResetDirtyFlag    FlagName
	TargetVersion     FlagName
	RollbackOnFailure FlagName
	MigrationsDir     FlagName
}

type MigrationType string

type FileMigrationFunc func(mig *migrate.Migrate, version int, mt MigrationType) error

type CustomMigrationFunc func(db webutil.DBInterface) error

type GetMigrationFunc func(migDir string, db *sql.DB, protocolCfg DBProtocolConfig) (*migrate.Migrate, error)

type migrationApplyConfig struct {
	Version         int
	CustomMigration CustomMigration
}

func (cdbm *CDBM) Migrate(getMigFunc GetMigrationFunc, fMigFunc FileMigrationFunc, cMigrations map[int]CustomMigration) error {
	var err error

	logDir := cdbm.MigrateFlags.MigrationsDir + "logs/"

	if err = os.MkdirAll(logDir, os.ModePerm); err != nil {
		return errors.WithStack(err)
	}

	logFile, err := os.OpenFile(logDir+"logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.ModePerm)

	if err != nil {
		return errors.WithStack(err)
	}

	defer logFile.Close()

	logWriter := bufio.NewWriter(logFile)
	defer logWriter.Flush()

	logWriterFunc := func(innerErr error) {
		logWriter.WriteString(time.Now().UTC().Format(webutil.FormDateTimeLayout) + ": " + innerErr.Error() + "\n")
	}

	sm, err := cdbm.querySchemaMigration()

	if err != nil {
		return err
	}

	files, err := ioutil.ReadDir(cdbm.MigrateFlags.MigrationsDir)

	if err != nil {
		return errors.WithStack(err)
	}

	migrationApplyCfgs := make([]migrationApplyConfig, 0)

	// dupVersionsMap is used to keep track of versions between files and custom
	// migrations to make sure there are no duplicate versioning
	dupVersionsMap := make(map[int]bool)

	// Loop through files and make sure they follow naming convention
	for _, v := range files {
		if v.IsDir() {
			continue
		}

		fileNameSlice := strings.Split(v.Name(), "_")

		if len(fileNameSlice) == 1 {
			return errors.WithStack(ErrInvalidFileName)
		}

		version, err := strconv.Atoi(fileNameSlice[0])

		if err != nil {
			return errors.WithStack(ErrInvalidFileName)
		}

		// Migrations should not be lower than 1
		if version < 1 {
			return errors.WithStack(
				fmt.Errorf("migration file version less than min version allowed (1)"),
			)
		}

		bodySlice := strings.Split(fileNameSlice[1], ".")

		if len(bodySlice) != 3 {
			return errors.WithStack(ErrInvalidFileName)
		}

		if bodySlice[1] != "up" && bodySlice[1] != "down" {
			return errors.WithStack(ErrInvalidFileName)
		}

		if bodySlice[2] != "sql" {
			return errors.WithStack(ErrInvalidFileName)
		}

		_, ok := dupVersionsMap[version]

		if !ok {
			migrationApplyCfgs = append(
				migrationApplyCfgs,
				migrationApplyConfig{
					Version: version,
				},
			)
		}

		dupVersionsMap[version] = true
	}

	dupVersionSlice := make([]int, 0)

	// Loop through custom migrations to make sure there are no duplicate
	// versions between files and custom migrations
	for k, v := range cMigrations {
		if _, ok := dupVersionsMap[k]; ok {
			dupVersionSlice = append(dupVersionSlice, k)
		}

		mac := migrationApplyConfig{
			Version:         k,
			CustomMigration: v,
		}

		migrationApplyCfgs = append(migrationApplyCfgs, mac)
	}

	if len(migrationApplyCfgs) == 0 {
		return fmt.Errorf("no sql files or custom migrations found")
	}

	if len(dupVersionSlice) > 0 {
		return errors.WithStack(
			fmt.Errorf(
				"following versions are duplicated between files and custom migrations: %v",
				dupVersionSlice,
			),
		)
	}

	// Sort migrationApplyCfgs by version so migrations can happen in order
	sort.SliceStable(migrationApplyCfgs, func(i, j int) bool {
		return migrationApplyCfgs[i].Version < migrationApplyCfgs[j].Version
	})

	var targetVersion int

	// If user is targeting specific version, make sure it exists
	// Else choose the highest version
	if cdbm.MigrateFlags.TargetVersion > -1 {
		if cdbm.MigrateFlags.TargetVersion > migrationApplyCfgs[len(migrationApplyCfgs)-1].Version {
			return fmt.Errorf("--target-version does not exist")
		}

		targetVersion = cdbm.MigrateFlags.TargetVersion
	} else {
		targetVersion = migrationApplyCfgs[len(migrationApplyCfgs)-1].Version
	}

	mig, err := getMigFunc("file://"+cdbm.MigrateFlags.MigrationsDir, cdbm.DB.DB, cdbm.DBProtocolCfg)

	if err != nil {
		return errors.WithStack(err)
	}

	// If schema_migrations is currently dirty, check if user sent --reset-dirty-flag flag
	// and if they did, reset the dirty flag in the database
	//
	// Else return error stating they need to set --reset-dirty-flag flag in order to continue
	if sm.Dirty {
		if cdbm.MigrateFlags.ResetDirtyFlag {
			if err = fMigFunc(mig, sm.StartingVersion, MigrateTypeForce); err != nil {
				return errors.WithStack(err)
			}
		} else {
			return fmt.Errorf(
				"must set --reset-dirty-flag to reset migrations dirty flag.  Use 'cdbm status' to see current status of migration",
			)
		}
	}

	// If startingVersion != targetVersion than we know user wants to migrate up or down
	// or if schemaMigrations#NoRows is true than we know no migrations have
	//
	if sm.StartingVersion != targetVersion || sm.NoRows {
		mType := MigrateTypeUp

		if sm.StartingVersion > targetVersion {
			mType = MigrateTypeDown
		}

		hasEntry := true

		if sm.NoRows {
			hasEntry = false
		}

		schemaInsert, _, err := webutil.InQueryRebind(
			cdbm.DBProtocolCfg.SQLBindVar,
			`
			insert into schema_migrations(version, dirty, dirty_state, is_custom_migration) 
			values(?, ?, ?, ?);
			`,
			0,
			true,
			mType,
			true,
		)

		if err != nil {
			return errors.WithStack(err)
		}

		schemaUpdate, _, err := webutil.InQueryRebind(
			cdbm.DBProtocolCfg.SQLBindVar,
			`
			update
				schema_migrations
			set
				version = ?,
				dirty = ?,
				dirty_state = ?,
				is_custom_migration = ?
			`,
			0,
			true,
			mType,
			true,
		)

		if err != nil {
			return errors.WithStack(err)
		}

		// migrationRollbackFailFunc is in charge of rolling back to starting
		// version if any migrations fails if --rollback-on-failure is set
		//
		// Will wrap given funcErr with any errors that occur within function
		// and return them along with version number that rollback failed on
		//
		// Else version returned will be -1
		migrationRollbackFailFunc := func(funcErr error, version int) error {
			// If a migration fails, rollBackOnFailure is set and is currently an up migration,
			// begin rolling back to version we started with
			if funcErr != nil && cdbm.MigrateFlags.RollbackOnFailure && mType == MigrateTypeUp {
				var innerErr error

				for version > sm.StartingVersion {
					//fmt.Printf("current version: %d\n", version)
					// Check if current version is apart of a custom migration
					//
					// If it is, apply down migration of version if func is not nil
					//
					// Else run file down migrations
					if migration, ok := cMigrations[version]; ok {
						if migration.Down != nil {
							if innerErr = migration.Down(cdbm.DB); innerErr != nil {
								logWriterFunc(innerErr)

								var query string

								if !hasEntry {
									query = schemaInsert
								} else {
									query = schemaUpdate
								}

								if _, innerErr = cdbm.DB.Exec(query, version, true, mType, true); innerErr != nil {
									logWriterFunc(innerErr)
								}

								return fmt.Errorf(
									"%w; %s",
									funcErr,
									fmt.Sprintf(
										"failed on custom rollback migration for version: '%d'",
										version,
									),
								)
							}
						}
					} else {
						if innerErr = fMigFunc(mig, version, MigrateTypeDown); innerErr != nil {
							logWriterFunc(innerErr)

							if _, innerErr = cdbm.DB.Exec(schemaUpdate, version, true, mType, true); innerErr != nil {
								logWriterFunc(innerErr)
							}

							return fmt.Errorf(
								"%w; %s",
								funcErr,
								fmt.Sprintf(
									"failed on file rollback migration for version: '%d'",
									version,
								),
							)
						}
					}

					version--
				}
			}

			return nil
		}

		// customMigrationFunc executes passed custom migration function and if error
		// occurs, it calls migrationRollbackFailFunc function
		//
		// Will wrap any errors that occur and return them along with version that
		// either custom migration failed on or version failing to rollback on
		customMigrationFunc := func(cmFunc CustomMigrationFunc, version int) error {
			if err = cmFunc(cdbm.DB); err != nil {
				var innerErr error
				var errMsg string

				if mType == MigrateTypeUp {
					errMsg = "failed on custom up migration for version: '%d'"
				} else {
					errMsg = "failed on custom down migration for version: '%d'"
				}

				logWriterFunc(err)
				innerErr = fmt.Errorf(errMsg, version)

				if err = migrationRollbackFailFunc(innerErr, version); err != nil {
					return err
				}

				return innerErr
			}

			return nil
		}

		migrationApplyFunc := func(cfg migrationApplyConfig) error {
			var cm CustomMigrationFunc

			isCustom := false

			if cfg.CustomMigration.Up != nil || cfg.CustomMigration.Down != nil {
				isCustom = true
			}

			if mType == MigrateTypeUp {
				cm = cfg.CustomMigration.Up
			} else {
				cm = cfg.CustomMigration.Down
			}

			if isCustom {
				if cm != nil {
					if !hasEntry {
						if mType == MigrateTypeDown {
							cfg.Version--
						}

						if err = customMigrationFunc(cm, cfg.Version); err != nil {
							return err
						}

						if _, err = cdbm.DB.Exec(schemaInsert, cfg.Version, false, "", true); err != nil {
							logWriterFunc(err)
							return errors.WithStack(err)
						}

						hasEntry = true
					} else {
						if err = customMigrationFunc(cm, cfg.Version); err != nil {
							return err
						}

						if _, err = cdbm.DB.Exec(schemaUpdate, cfg.Version, false, "", true); err != nil {
							logWriterFunc(err)
							return errors.WithStack(err)
						}
					}
				}
			} else {
				fileVersion := cfg.Version

				if err = fMigFunc(mig, cfg.Version, mType); err != nil {
					var innerErr error
					var errMsg string

					if mType == MigrateTypeUp {
						fileVersion--
						errMsg = "failed on file up migration for version: '%d'"
					} else {
						fileVersion++
						errMsg = "failed on file down migration for version: '%d'"
					}

					logWriterFunc(err)

					err = fmt.Errorf(errMsg, fileVersion)

					if innerErr = migrationRollbackFailFunc(err, cfg.Version); innerErr != nil {
						return innerErr
					}

					return err
				} else {
					if mType == MigrateTypeUp {
						fileVersion++
					} else {
						fileVersion--
					}

					if _, err = cdbm.DB.Exec(schemaUpdate, cfg.Version, false, "", false); err != nil {
						logWriterFunc(err)
						return errors.WithStack(err)
					}
				}
			}

			return nil
		}

		if mType == MigrateTypeUp {
			for _, v := range migrationApplyCfgs {
				if sm.IsCustomMigration {
					if sm.Dirty {
						if v.Version < sm.StartingVersion {
							continue
						}
					} else {
						if v.Version <= sm.StartingVersion {
							continue
						}
					}
				} else {
					if v.Version <= sm.StartingVersion {
						continue
					}
				}

				if targetVersion < v.Version {
					break
				}

				if err = migrationApplyFunc(v); err != nil {
					return err
				}
			}
		} else {
			for i := len(migrationApplyCfgs) - 1; i >= 0; i-- {
				fmt.Printf("starting version: %d\n", sm.StartingVersion)

				if sm.IsCustomMigration {
					if migrationApplyCfgs[i].Version > sm.StartingVersion {
						continue
					}
				} else {
					if migrationApplyCfgs[i].Version >= sm.StartingVersion {
						continue
					}
				}

				if targetVersion > migrationApplyCfgs[i].Version {
					break
				}

				fmt.Printf("version apply cfg: %v\n", migrationApplyCfgs[i])

				if err = migrationApplyFunc(migrationApplyCfgs[i]); err != nil {
					return err
				}
			}
		}
	} else {
		fmt.Printf("No Change\n")
	}

	return nil
}
