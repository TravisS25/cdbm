package app

import (
	"bufio"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/TravisS25/cdbm/cdbmutil"
	"github.com/TravisS25/webutil/webutil"
	"github.com/TravisS25/webutil/webutilcfg"
	"github.com/golang-migrate/migrate/v4"
	"github.com/pkg/errors"
)

// MigrateFlagsConfig is flag config struct for migrate command set by command line
// or set in code if used a library
type MigrateFlagsConfig struct {
	// TargetVersion should be the version you wish to set the database at
	// If set below 0 than the latest version is used by default
	TargetVersion int `yaml:"target_version" mapstructure:"target_version"`

	// RollbackOnFailure gives user ability to rollback a migration to starting state
	// if it fails on migration when set
	//
	// This only works when migrating up
	RollbackOnFailure bool `yaml:"rollback_on_failure" mapstructure:"rollback_on_failure"`

	// ResetDirtyFlag must be set if migration is in a dirty state to reset dirty state
	// or error will be returned without running anything
	//
	// This is used as a "saftey switch" to remind user that database is in dirty state
	ResetDirtyFlag bool `yaml:"reset_dirty_flag" mapstructure:"reset_dirty_flag"`

	// MigrateDownOnDirty is flag that when set will run the down migration of current file or
	// custom migration if the migrations's table is in a dirty state when trying to migrate up
	//
	// This is mostly used in conjunction with the --reset-dirty-flag option
	//MigrateDownIfDirty bool `yaml:"migrate_down_if_dirty" mapstructure:"migrate_down_if_dirty"`

	// SkipFileMigrationResetOnFailure is used to skip a migration reset for file migration if it fails
	//
	// A migration reset occurs when:
	// 	1) Starting migration version has dirty flag
	//	2) Starting migration version is file migration ie. NOT custom migration
	// 	3) Currently trying to migrate up
	//
	// Resetting a file migration is when we first call the down migration of current version
	// to try to undo previous bad up migration
	// If this fails, we will receive error and return unless SkipFileMigrationResetOnFailure
	// is set in which case we disregard the error and continue with the current up migration
	//SkipFileMigrationResetOnFailure bool `yaml:"skip_migration_reset_on_failure" mapstructure:"skip_migration_reset_on_failure"`

	// MigrationsProtocol is protocol used to retrieve migration files
	MigrationsProtocol cdbmutil.MigrationsProtocol `yaml:"migrations_protocol" mapstructure:"migrations_protocol"`

	// MigrationsDir is directory where migration files are stored whether locally or remotely
	MigrationsDir string `yaml:"migrations_dir" mapstructure:"migrations_dir"`
}

// migrationApplyConfig is config struct to apply migrations and version
type migrationApplyConfig struct {
	// Version is current version to apply to database
	Version int

	// CustomMigration is custom migrations to apply if set
	CustomMigration cdbmutil.CustomMigration
}

// migrateState is struct used to keep track of certain states as migration is being run
//
// This will be used in the CDBM struct and all properties of this struct should be
// set by the CDBM#Migrate function
type migrateState struct {
	// LogWriter will write migrate errors to file system
	LogWriter func(error)

	// InsertQuery is query to insert info into schema_migrations table
	InsertQuery string

	// UpdateQuery is query to update info in schema_migrations table
	UpdateQuery string

	// TargetVersion is version passed by --target-version flag
	TargetVersion int

	// MigrateType determines if we are migrating "Up" "Down" "Force"
	MigrateType cdbmutil.MigrationsType

	// FileMigration runs migration against database
	FileMigration cdbmutil.FileMigrationFunc

	// CustomMigrations is map of custom migrations
	CustomMigrations map[int]cdbmutil.CustomMigration

	// SchemaMigration represents schema_migrations table
	SchemaMigration schemaMigration

	// Migrate is migrate.Migrate instance to migrate database
	Migrate *migrate.Migrate
}

// Migrate migrates database based on given settings
func (cdbm *CDBM) Migrate(
	getMigFunc cdbmutil.GetMigrationFunc,
	fMigFunc cdbmutil.FileMigrationFunc,
	cMigrations map[int]cdbmutil.CustomMigration,
) error {
	var err error

	if err = cdbm.checkMigrationsProtocol(); err != nil {
		return err
	}

	if err = cdbm.applySchemaMigrationsQueries(); err != nil {
		return err
	}

	cdbm.migrateCfg.FileMigration = fMigFunc
	cdbm.migrateCfg.CustomMigrations = cMigrations

	logFile, err := cdbm.createLogsDirectory()

	if err != nil {
		return err
	}

	if logFile != nil {
		defer logFile.Close()

		logWriter := bufio.NewWriter(logFile)
		defer logWriter.Flush()

		cdbm.migrateCfg.LogWriter = func(innerErr error) {
			logWriter.WriteString(time.Now().UTC().Format(webutilcfg.FormDateTimeLayout) + ": " + innerErr.Error() + "\n")
		}
	}

	// Query current migration status and set to CDBM#migrateCfg#SchemaMigration
	if cdbm.migrateCfg.SchemaMigration, err = cdbm.getSchemaMigration(); err != nil {
		return err
	}

	migrationApplyCfgs, err := cdbm.verifyFilesAndMigrations()

	if err != nil {
		return err
	}

	if cdbm.migrateCfg.Migrate, err = getMigFunc(
		string(cdbm.MigrateFlags.MigrationsProtocol)+cdbm.MigrateFlags.MigrationsDir,
		cdbm.DB.DB,
		cdbm.DBProtocolCfg,
	); err != nil {
		return errors.WithStack(err)
	}

	// If user is targeting specific version, make sure it exists
	// Else choose the highest version
	if err = cdbm.applyTargetVersion(migrationApplyCfgs); err != nil {
		return err
	}

	// If schema_migrations is currently dirty, check if user sent --reset-dirty-flag flag
	// and if they did, reset the dirty flag in the database
	//
	// Else return error stating they need to set --reset-dirty-flag flag in order to continue
	if err = cdbm.resetDirtyFlag(); err != nil {
		return err
	}

	// fmt.Printf("dirty: %v\n", cdbm.migrateCfg.SchemaMigration.SchemaCfg.Dirty)
	// fmt.Printf("migrate type: %v\n", cdbm.migrateCfg.MigrateType)
	// fmt.Printf("migrate if dirty: %v\n", cdbm.MigrateFlags.MigrateDownIfDirty)

	// If startingVersion != targetVersion than we know user wants to migrate up or down
	// or if schemaMigrations#NoRows is true than we know no migrations have been applied yet
	if cdbm.migrateCfg.SchemaMigration.StartingVersion != cdbm.migrateCfg.TargetVersion ||
		cdbm.migrateCfg.SchemaMigration.SchemaCfg.NoRows || cdbm.migrateCfg.SchemaMigration.Dirty {
		cdbm.migrateCfg.MigrateType = cdbmutil.MigrateTypeUp

		if cdbm.migrateCfg.SchemaMigration.StartingVersion > cdbm.migrateCfg.TargetVersion {
			cdbm.migrateCfg.MigrateType = cdbmutil.MigrateTypeDown
		}

		if err = cdbm.runMigrationConfigs(migrationApplyCfgs); err != nil {
			return err
		}
	} else {
		fmt.Printf("No Change\n")
	}

	return nil
}

// checkMigrationsProtocol makes sure user sets --db-protocol flag as we need
// this in order to apply other settings
func (cdbm *CDBM) checkMigrationsProtocol() error {
	if cdbm.MigrateFlags.MigrationsProtocol == "" {
		return errors.WithStack(fmt.Errorf("--migrations-protocol required"))
	} else {
		found := false

		pList := []cdbmutil.MigrationsProtocol{
			cdbmutil.FileProtocol,
			cdbmutil.GoBindData,
			cdbmutil.GithubProtocol,
			cdbmutil.GitHubEnterpriseProtocol,
			cdbmutil.BitbucketProtocol,
			cdbmutil.GitlabProtcol,
			cdbmutil.S3Protocol,
			cdbmutil.GoogleCloudStorageProtocol,
		}

		for _, v := range pList {
			if v == cdbm.MigrateFlags.MigrationsProtocol {
				found = true
				break
			}
		}

		if !found {
			return errors.WithStack(
				fmt.Errorf("invalid --migrations-protocol;  Valid --migrations-protocol are %v", pList),
			)
		}
	}

	return nil
}

// applySchemaMigrationsQueries sets up our insert and update schema_migrations queries by
// applying sql bind var
func (cdbm *CDBM) applySchemaMigrationsQueries() error {
	schemaInsert, _, err := webutil.InQueryRebind(
		cdbm.DBProtocolCfg.SQLBindVar,
		`
		insert into schema_migrations(version, dirty, dirty_state, is_custom_migration) 
		values(?, ?, ?, ?);
		`,
		0,
		true,
		"",
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
		"",
		true,
	)

	if err != nil {
		return errors.WithStack(err)
	}

	cdbm.migrateCfg.InsertQuery = schemaInsert
	cdbm.migrateCfg.UpdateQuery = schemaUpdate
	return nil
}

// createLogsDirectory creates logs directory where errors during migration
// will be written to
func (cdbm *CDBM) createLogsDirectory() (*os.File, error) {
	if _, err := os.Stat(cdbm.LogFlags.LogFile); errors.Is(err, os.ErrNotExist) {
		if cdbm.LogFlags.LogFile != "" {
			baseDir, fileName := path.Split(cdbm.LogFlags.LogFile)

			if err = os.MkdirAll(baseDir, os.ModePerm); err != nil {
				return nil, errors.WithStack(err)
			}

			return os.Create(baseDir + fileName)
		} else {
			return nil, nil
		}
	}

	return os.Open(cdbm.LogFlags.LogFile)
}

// verifyFilesAndMigrations loops through given migration files and
// verifies that they have appropriate naming convention and then
// orders them based on file naming
func (cdbm *CDBM) verifyFilesAndMigrations() ([]migrationApplyConfig, error) {
	files, err := ioutil.ReadDir(cdbm.MigrateFlags.MigrationsDir)

	if err != nil {
		return nil, errors.WithStack(err)
	}

	// fileVersions is used to keep track of versions between files and custom
	// migrations to make sure there are no duplicate versioning
	fileVersions := make(map[int]bool)
	migrationApplyCfgs := make([]migrationApplyConfig, 0)

	// Loop through files and make sure they follow naming convention
	for _, file := range files {
		// If current file is directory, continue loop as we are only looking
		// for files in migrations direcroty
		if file.IsDir() {
			continue
		}

		fileNameSlice := strings.Split(file.Name(), "_")

		if len(fileNameSlice) == 1 {
			return nil, errors.WithStack(cdbmutil.ErrInvalidFileName)
		}

		// File names should be numbers
		version, err := strconv.Atoi(fileNameSlice[0])

		if err != nil {
			return nil, errors.WithStack(cdbmutil.ErrInvalidFileName)
		}

		// Migrations should not be lower than 1
		if version < 1 {
			return nil, errors.WithStack(
				fmt.Errorf("migration file version less than min version allowed (1)"),
			)
		}

		bodySlice := strings.Split(fileNameSlice[1], ".")

		if len(bodySlice) != 3 {
			return nil, errors.WithStack(cdbmutil.ErrInvalidFileName)
		}

		if bodySlice[1] != "up" && bodySlice[1] != "down" {
			return nil, errors.WithStack(cdbmutil.ErrInvalidFileName)
		}

		if bodySlice[2] != "sql" {
			return nil, errors.WithStack(cdbmutil.ErrInvalidFileName)
		}

		_, ok := fileVersions[version]
		_, customOK := cdbm.migrateCfg.CustomMigrations[version]

		// If current version is not found in either file migrations or custom migrations,
		// add it to our migration apply config slice
		if !ok && !customOK {
			migrationApplyCfgs = append(
				migrationApplyCfgs,
				migrationApplyConfig{
					Version: version,
				},
			)
		}

		fileVersions[version] = true
	}

	// Loop through custom migrations to make sure there are no duplicate
	// versions between files and custom migrations
	for k, v := range cdbm.migrateCfg.CustomMigrations {
		mac := migrationApplyConfig{
			Version:         k,
			CustomMigration: v,
		}

		if v.Up == nil || v.Down == nil {
			return nil, fmt.Errorf("custom migrations must have both an up and down defined function")
		}

		migrationApplyCfgs = append(migrationApplyCfgs, mac)
	}

	if len(migrationApplyCfgs) == 0 {
		return nil, fmt.Errorf("no sql files or custom migrations found")
	}

	// Sort migrationApplyCfgs by version so migrations can happen in order
	sort.SliceStable(migrationApplyCfgs, func(i, j int) bool {
		return migrationApplyCfgs[i].Version < migrationApplyCfgs[j].Version
	})

	return migrationApplyCfgs, nil
}

// getSchemaMigration queries and returns schema_migration table info
//
// If schema_migrations table doesn't exist, it create its and return base info
func (cdbm *CDBM) getSchemaMigration() (schemaMigration, error) {
	var err error
	var sm schemaMigration

	// MigrationTableSearch should be custom query depending on database
	// on whether the "schema_migrations" table exists
	//
	// If it doesn't exist, then we assume we are starting at version 1
	//
	// Else query for lastest version
	if err = cdbm.DBProtocolCfg.MigrationTableSearch(cdbm.DB); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return schemaMigration{}, errors.WithStack(err)
		}

		if _, err = cdbm.DB.Exec(
			`
			CREATE TABLE public.schema_migrations (
				version INT8 NOT NULL primary key,
				dirty boolean not null,
				dirty_state text,
				is_custom_migration boolean not null default false
			);
			`,
		); err != nil {
			return schemaMigration{}, errors.WithStack(err)
		}

		sm.SchemaCfg.NoRows = true
	} else {
		if err = cdbm.DB.QueryRowx(
			`
			select
				schema_migrations.version,
				schema_migrations.dirty,
				schema_migrations.dirty_state,
				schema_migrations.is_custom_migration
			from
				schema_migrations
			`,
		).Scan(&sm.StartingVersion, &sm.Dirty, &sm.DirtyState, &sm.IsCustomMigration); err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return schemaMigration{}, errors.WithStack(err)
			}

			sm.SchemaCfg.NoRows = true
		}
	}

	return sm, nil
}

// applyTargetVersion sets given target version and will return error if
// version doesn't exist
func (cdbm *CDBM) applyTargetVersion(cfgs []migrationApplyConfig) error {
	if cdbm.MigrateFlags.TargetVersion > -1 {
		if cdbm.MigrateFlags.TargetVersion > cfgs[len(cfgs)-1].Version {
			return errors.WithStack(fmt.Errorf("--target-version does not exist"))
		}

		cdbm.migrateCfg.TargetVersion = cdbm.MigrateFlags.TargetVersion
	} else {
		cdbm.migrateCfg.TargetVersion = cfgs[len(cfgs)-1].Version
	}
	return nil
}

// resetDirtyFlag resets dirty flag if set and will return error if database
// has dirty flag and --reset-dirty-flag is not set
func (cdbm *CDBM) resetDirtyFlag() error {
	var err error

	if cdbm.migrateCfg.SchemaMigration.Dirty {
		cdbm.migrateCfg.SchemaMigration.SchemaCfg.Dirty = true

		if cdbm.MigrateFlags.ResetDirtyFlag {
			if _, err = cdbm.DB.Exec(
				cdbm.migrateCfg.UpdateQuery,
				cdbm.migrateCfg.SchemaMigration.StartingVersion,
				false,
				"",
				cdbm.migrateCfg.SchemaMigration.IsCustomMigration,
			); err != nil {
				return errors.WithStack(err)
			}
		} else {
			return fmt.Errorf(
				"must set --reset-dirty-flag to reset migrations dirty flag.  Use 'cdbm status' to see current status of migration",
			)
		}
	}

	return nil
}

// migrationRollbackFail will rollback up migration if it fails to starting migration version
// if --rollback-on-failure is set
func (cdbm *CDBM) migrationRollbackFail(version int) error {
	// If a migration fails, rollBackOnFailure is set and is currently an up migration,
	// begin rolling back to version we started with
	var err error

	for version > cdbm.migrateCfg.SchemaMigration.StartingVersion {
		// Check if current version is apart of a custom migration
		//
		// Else run file down migrations
		if migration, ok := cdbm.migrateCfg.CustomMigrations[version]; ok {
			if migration.Down != nil {
				if err = migration.Down(cdbm.DB); err != nil {
					if cdbm.migrateCfg.LogWriter != nil {
						cdbm.migrateCfg.LogWriter(err)
					}

					var query string

					if cdbm.migrateCfg.SchemaMigration.SchemaCfg.NoRows {
						query = cdbm.migrateCfg.InsertQuery
					} else {
						query = cdbm.migrateCfg.UpdateQuery
					}

					if _, err = cdbm.DB.Exec(
						query,
						version,
						true,
						cdbmutil.MigrateTypeDown,
						true,
					); err != nil {
						if cdbm.migrateCfg.LogWriter != nil {
							cdbm.migrateCfg.LogWriter(err)
						}
					}

					return fmt.Errorf(
						"failed on custom rollback migration for version: '%d'",
						version,
					)
				}
			}
		} else {
			if err = cdbm.migrateCfg.FileMigration(
				cdbm.migrateCfg.Migrate,
				version,
				cdbmutil.MigrateTypeDown,
			); err != nil {
				if cdbm.migrateCfg.LogWriter != nil {
					cdbm.migrateCfg.LogWriter(err)
				}

				if _, err = cdbm.DB.Exec(
					cdbm.migrateCfg.UpdateQuery,
					version,
					true,
					cdbmutil.MigrateTypeDown,
					false,
				); err != nil {
					if cdbm.migrateCfg.LogWriter != nil {
						cdbm.migrateCfg.LogWriter(err)
					}
				}

				return fmt.Errorf(
					"failed on file rollback migration for version: '%d'",
					version,
				)
			}
		}

		version--
	}

	return nil
}

// applyCustomMigration applies custom migration to database
func (cdbm *CDBM) applyCustomMigration(applyCfg migrationApplyConfig) error {
	var err, innerErr error

	if cdbm.migrateCfg.SchemaMigration.SchemaCfg.Dirty &&
		cdbm.migrateCfg.MigrateType == cdbmutil.MigrateTypeUp &&
		applyCfg.CustomMigration.Down != nil {
		if err = applyCfg.CustomMigration.Down(cdbm.DB); err != nil {
			if cdbm.migrateCfg.LogWriter != nil {
				cdbm.migrateCfg.LogWriter(err)
			}

			if _, err = cdbm.DB.Exec(
				cdbm.migrateCfg.UpdateQuery,
				applyCfg.Version,
				true,
				cdbmutil.MigrateTypeDown,
				false,
			); err != nil {
				if cdbm.migrateCfg.LogWriter != nil {
					cdbm.migrateCfg.LogWriter(err)
				}
			}

			return errors.WithStack(
				fmt.Errorf("failed of resetting custom migration for version '%d'", applyCfg.Version),
			)
		}

		cdbm.migrateCfg.SchemaMigration.SchemaCfg.Dirty = false
		cdbm.migrateCfg.SchemaMigration.SchemaCfg.NoRows = false
	}

	var cmFunc cdbmutil.CustomMigrationFunc

	if cdbm.migrateCfg.MigrateType == cdbmutil.MigrateTypeUp {
		cmFunc = applyCfg.CustomMigration.Up
	} else {
		cmFunc = applyCfg.CustomMigration.Down
	}

	// If custom migration function has error, begin process of logging and trying
	// to rollback migration if set
	if err = cmFunc(cdbm.DB); err != nil {
		if cdbm.migrateCfg.LogWriter != nil {
			cdbm.migrateCfg.LogWriter(err)
		}

		if cdbm.migrateCfg.MigrateType == cdbmutil.MigrateTypeUp {
			err = fmt.Errorf("failed on custom up migration for version: '%d'.  Error: %+v", applyCfg.Version, err)
		} else {
			err = fmt.Errorf("failed on custom down migration for version: '%d'.  Error: %+v", applyCfg.Version, err)
		}

		var query string

		// If schema_migrations table already had entry, use update query
		// Else use create query
		if cdbm.migrateCfg.SchemaMigration.SchemaCfg.NoRows {
			query = cdbm.migrateCfg.InsertQuery
		} else {
			query = cdbm.migrateCfg.UpdateQuery
		}

		// If failing on up migration and the --rollback-on-failure flag is set,
		// begin process of rolling back current migration
		if cdbm.MigrateFlags.RollbackOnFailure &&
			cdbm.migrateCfg.MigrateType == cdbmutil.MigrateTypeUp {

			// If error occurs during rollback, add to logger and return both
			// migration and rollback errors
			if innerErr = cdbm.migrationRollbackFail(applyCfg.Version); innerErr != nil {
				if cdbm.migrateCfg.LogWriter != nil {
					cdbm.migrateCfg.LogWriter(err)
				}

				return fmt.Errorf(err.Error() + " and " + innerErr.Error())
			}

			if _, innerErr = cdbm.DB.Exec(
				query,
				cdbm.migrateCfg.SchemaMigration.StartingVersion,
				false,
				"",
				true,
			); innerErr != nil {
				if cdbm.migrateCfg.LogWriter != nil {
					cdbm.migrateCfg.LogWriter(err)
				}
			}

			return fmt.Errorf(
				err.Error()+" but successfully rolled back to version: '%d'",
				cdbm.migrateCfg.SchemaMigration.StartingVersion,
			)
		} else {
			if _, innerErr = cdbm.DB.Exec(
				query,
				applyCfg.Version,
				true,
				cdbm.migrateCfg.MigrateType,
				true,
			); innerErr != nil {
				if cdbm.migrateCfg.LogWriter != nil {
					cdbm.migrateCfg.LogWriter(err)
				}
			}
		}

		return err
	}

	if cdbm.migrateCfg.SchemaMigration.SchemaCfg.NoRows {
		if _, err = cdbm.DB.Exec(
			cdbm.migrateCfg.InsertQuery,
			applyCfg.Version,
			false,
			"",
			true,
		); err != nil {
			if cdbm.migrateCfg.LogWriter != nil {
				cdbm.migrateCfg.LogWriter(err)
			}

			return errors.WithStack(err)
		}
	} else {
		if _, err = cdbm.DB.Exec(
			cdbm.migrateCfg.UpdateQuery,
			applyCfg.Version,
			false,
			"",
			true,
		); err != nil {
			if cdbm.migrateCfg.LogWriter != nil {
				cdbm.migrateCfg.LogWriter(err)
			}

			return errors.WithStack(err)
		}
	}

	// At this point, no migration errors have occured and we know that at least
	// onc migration has to occur so make NoRows = false
	cdbm.migrateCfg.SchemaMigration.SchemaCfg.NoRows = false

	return nil
}

// applyFileMigration applies file migration to database
func (cdbm *CDBM) applyFileMigration(version int) error {
	var err, innerErr error

	if cdbm.migrateCfg.SchemaMigration.SchemaCfg.Dirty &&
		cdbm.migrateCfg.MigrateType == cdbmutil.MigrateTypeUp {

		if err = cdbm.migrateCfg.FileMigration(
			cdbm.migrateCfg.Migrate,
			version,
			cdbmutil.MigrateTypeDown,
		); err != nil {
			if cdbm.migrateCfg.LogWriter != nil {
				cdbm.migrateCfg.LogWriter(err)
			}

			if _, err = cdbm.DB.Exec(
				cdbm.migrateCfg.UpdateQuery,
				version,
				true,
				cdbmutil.MigrateTypeDown,
				false,
			); err != nil {
				if cdbm.migrateCfg.LogWriter != nil {
					cdbm.migrateCfg.LogWriter(err)
				}
			}

			return errors.WithStack(
				fmt.Errorf("failed of resetting file migration for version '%d'", version),
			)
		}

		// For file migration, if down migration is successful to "undirty"
		// migration state, then we must increase current version to then migrate up
		version++

		cdbm.migrateCfg.SchemaMigration.SchemaCfg.Dirty = false
	}

	// If file migration function returns error, begin process of logging
	// and resetting back to previous version if --rollback-on-failure is set
	if err = cdbm.migrateCfg.FileMigration(
		cdbm.migrateCfg.Migrate,
		version,
		cdbm.migrateCfg.MigrateType,
	); err != nil {
		if cdbm.migrateCfg.LogWriter != nil {
			cdbm.migrateCfg.LogWriter(err)
		}

		if cdbm.migrateCfg.MigrateType == cdbmutil.MigrateTypeUp {
			err = fmt.Errorf("failed on file up migration for version: '%d'.  Error: %+v\n", version, err)
		} else {
			err = fmt.Errorf("failed on file down migration for version: '%d'.  Error: %+v\n", version, err)
		}

		// If failing on up migration and the --rollback-on-failure flag is set,
		// begin process of rolling back current migration
		if cdbm.MigrateFlags.RollbackOnFailure &&
			cdbm.migrateCfg.MigrateType == cdbmutil.MigrateTypeUp {

			// If error occurs during rollback, add to logger and return both
			// migration and rollback errors
			if innerErr = cdbm.migrationRollbackFail(version); innerErr != nil {
				if cdbm.migrateCfg.LogWriter != nil {
					cdbm.migrateCfg.LogWriter(err)
				}

				return fmt.Errorf(err.Error() + "\n" + innerErr.Error())
			}

			if _, innerErr = cdbm.DB.Exec(
				cdbm.migrateCfg.UpdateQuery,
				cdbm.migrateCfg.SchemaMigration.StartingVersion,
				false,
				"",
				false,
			); innerErr != nil {
				if cdbm.migrateCfg.LogWriter != nil {
					cdbm.migrateCfg.LogWriter(err)
				}
			}

			return fmt.Errorf(
				err.Error()+" but successfully rolled back to version: '%d'",
				cdbm.migrateCfg.SchemaMigration.StartingVersion,
			)
		} else {
			if _, innerErr = cdbm.DB.Exec(
				cdbm.migrateCfg.UpdateQuery,
				version,
				true,
				cdbm.migrateCfg.MigrateType,
				false,
			); innerErr != nil {
				if cdbm.migrateCfg.LogWriter != nil {
					cdbm.migrateCfg.LogWriter(err)
				}
			}
		}

		return err
	} else {
		if cdbm.migrateCfg.MigrateType != cdbmutil.MigrateTypeDown && version != 0 {
			if _, err = cdbm.DB.Exec(
				cdbm.migrateCfg.UpdateQuery,
				version,
				false,
				"",
				false,
			); err != nil {
				if cdbm.migrateCfg.LogWriter != nil {
					cdbm.migrateCfg.LogWriter(err)
				}

				return errors.WithStack(err)
			}
		}
	}

	// At this point, no migration errors have occured and we know that at least
	// onc migration has to occur so make NoRows = false
	cdbm.migrateCfg.SchemaMigration.SchemaCfg.NoRows = false

	return nil
}

// runMigrationConfigs will apply given slice of migrationApplyConfig to database
func (cdbm *CDBM) runMigrationConfigs(cfgs []migrationApplyConfig) error {
	var err error

	applyMigration := func(cfg migrationApplyConfig) error {
		// If CustomMigration functions are defined then we are currently on custom migration
		// so apply custom migration of current version
		//
		// Else apply file migration
		if cfg.CustomMigration.Up != nil || cfg.CustomMigration.Down != nil {
			if err = cdbm.applyCustomMigration(cfg); err != nil {
				return err
			}
		} else {
			if err = cdbm.applyFileMigration(cfg.Version); err != nil {
				return err
			}
		}

		return nil
	}

	if cdbm.migrateCfg.MigrateType == cdbmutil.MigrateTypeUp {
		for _, cfg := range cfgs {
			// If schema config is dirty and loop is currently on starting version of migration,
			// apply migration without any version check as our migration functions will first
			// do a down migration to "undirty" current state of migration before applying up migration
			//
			// Else do version checks for current version and apply migration when neccessary
			if cdbm.migrateCfg.SchemaMigration.SchemaCfg.Dirty && cfg.Version == cdbm.migrateCfg.SchemaMigration.StartingVersion {
				if cfg.CustomMigration.Up != nil || cfg.CustomMigration.Down != nil {
					if err = cdbm.applyCustomMigration(cfg); err != nil {
						return err
					}
				} else {
					// With file migrations, we have to apply the previous state version if dirty
					// so the migrate library will do down
					prevVersion := cfg.Version - 1

					if err = cdbm.applyFileMigration(prevVersion); err != nil {
						return err
					}
				}
			} else {
				// If current apply config version is less than or equal to starting version
				// continue loop as we are migrating up and want the next version up
				if cfg.Version <= cdbm.migrateCfg.SchemaMigration.StartingVersion {
					continue
				}

				// If target version specified by user is less than current apply config version
				// break from loop as this implies we are done migrating
				if cdbm.migrateCfg.TargetVersion < cfg.Version {
					break
				}

				if err = applyMigration(cfg); err != nil {
					return err
				}
			}
		}
	} else {
		for i := len(cfgs) - 1; i >= 0; i-- {
			if cdbm.migrateCfg.TargetVersion == 0 && i == 0 {
				if cfgs[i].CustomMigration.Up != nil || cfgs[i].CustomMigration.Down != nil {
					if err = cdbm.applyCustomMigration(cfgs[i]); err != nil {
						return err
					}
				} else if err = cdbm.applyFileMigration(0); err != nil {
					return err
				}
			} else {
				// If current apply config version is greater than or equal to starting version
				// continue loop as we are migrating down and want the next version down
				if cfgs[i].Version >= cdbm.migrateCfg.SchemaMigration.StartingVersion {
					continue
				}

				// If target version specified by user is greater than current apply config version
				// break from loop as this implies we are down migrating
				if cdbm.migrateCfg.TargetVersion > cfgs[i].Version {
					break
				}

				// If CustomMigration functions are defined then we are currently on custom migration
				if cfgs[i].CustomMigration.Up != nil || cfgs[i].CustomMigration.Down != nil {

					// If we are currently on custom migration config, we must first check if the
					// config in next index is also a custom migration
					//
					// If it is, then we apply that down migration
					// Else apply current file config
					if cfgs[i+1].CustomMigration.Up != nil || cfgs[i+1].CustomMigration.Down != nil {
						// When migrating down with custom migrations, we have to make copy of current config
						// to lower version to what it will be after migration
						copyCfg := cfgs[i+1]
						copyCfg.Version--

						if err = cdbm.applyCustomMigration(copyCfg); err != nil {
							return err
						}
					} else if err = cdbm.applyFileMigration(cfgs[i].Version); err != nil {
						return err
					}
				} else {
					if err = cdbm.applyFileMigration(cfgs[i].Version); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}
