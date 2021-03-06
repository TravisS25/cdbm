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
	MigrateTypeUp    MigrationsType = "Up"
	MigrateTypeDown  MigrationsType = "Down"
	MigrateTypeForce MigrationsType = "Force"
)

const (
	FileProtocol               MigrationsProtocol = "file://"
	GoBindData                 MigrationsProtocol = "go-bindata"
	GithubProtocol             MigrationsProtocol = "github://"
	GitHubEnterpriseProtocol   MigrationsProtocol = "github-ee://"
	BitbucketProtocol          MigrationsProtocol = "bitbucket://"
	GitlabProtcol              MigrationsProtocol = "gitlab://"
	S3Protocol                 MigrationsProtocol = "s3://"
	GoogleCloudStorageProtocol MigrationsProtocol = "gcs://"
)

var (
	ErrInvalidFileName = fmt.Errorf("cdbm: invalid sql file name - proper naming:<version>_<description>.<'up'|'down'>.sql")

	migrationsProtocolList = []MigrationsProtocol{
		FileProtocol,
		GoBindData,
		GithubProtocol,
		GitHubEnterpriseProtocol,
		BitbucketProtocol,
		GitlabProtcol,
		S3Protocol,
		GoogleCloudStorageProtocol,
	}
)

// postgresMigrationTableSearch is default search function for postgres to
// determine if schema_migrations table exists in database table
var postgresMigrationTableSearch = func(db webutil.DBInterface) error {
	var filler string
	var err error

	if err = db.QueryRowx(
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
	).Scan(&filler); err != nil {
		return err
	}

	return nil
}

// CustomMigration is config struct used to migrate database with
// custom go code
type CustomMigration struct {
	// Up should migrate database to next state
	Up CustomMigrationFunc

	// Down should migrate database to previous state
	Down CustomMigrationFunc
}

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
	SkipFileMigrationResetOnFailure bool `yaml:"skip_migration_reset_on_failure" mapstructure:"skip_migration_reset_on_failure"`

	// MigrationsProtocol is protocol used to retrieve migration files
	MigrationsProtocol MigrationsProtocol `yaml:"migrations_protocol" mapstructure:"migrations_protocol"`

	// MigrationsDir is directory where migration files are stored whether locally or remotely
	MigrationsDir string `yaml:"migrations_dir" mapstructure:"migrations_dir"`
}

// MigrationsType is enum for different migration type ie. "Up", "Down", "Force"
type MigrationsType string

// MigrationsProtocol is enum for different database protocols
type MigrationsProtocol string

// FileMigrationFunc should implement migrating database up or down
type FileMigrationFunc func(mig *migrate.Migrate, version int, mt MigrationsType) error

// CustomMigrationFunc should implement migrating database up or down through custom code
type CustomMigrationFunc func(db webutil.DBInterface) error

// GetMigrationFunc should implement getting migrate.Migrate based on migrations
// directory and database instance
type GetMigrationFunc func(migDir string, db *sql.DB, protocolCfg DBProtocolConfig) (*migrate.Migrate, error)

// migrationApplyConfig is config struct to apply migrations and version
type migrationApplyConfig struct {
	// Version is current version to apply to database
	Version int

	// CustomMigration is custom migrations to apply if set
	CustomMigration CustomMigration
}

type migrateConfig struct {
	// LogWriter will write migrate errors to file system
	LogWriter func(error)

	// InsertQuery is query to insert info into schema_migrations table
	InsertQuery string

	// UpdateQuery is query to update info in schema_migrations table
	UpdateQuery string

	// TargetVersion is version passed by --target-version flag
	TargetVersion int

	// MigrateType determines if we are migrating "Up" "Down" "Force"
	MigrateType MigrationsType

	// FileMigration runs migration against database
	FileMigration FileMigrationFunc

	// CustomMigrations is map of custom migrations
	CustomMigrations map[int]CustomMigration

	// SchemaMigration represents schema_migrations table
	SchemaMigration schemaMigration

	// Migrate is migrate.Migrate instance to migrate database
	Migrate *migrate.Migrate
}

// Migrate migrates database based on given settings
func (cdbm *CDBM) Migrate(getMigFunc GetMigrationFunc, fMigFunc FileMigrationFunc, cMigrations map[int]CustomMigration) error {
	var err error

	if err = cdbm.checkMigrationsProtocol(); err != nil {
		return err
	}

	if err = cdbm.applyQueries(); err != nil {
		return err
	}

	cdbm.migrateCfg.FileMigration = fMigFunc
	cdbm.migrateCfg.CustomMigrations = cMigrations

	logFile, err := cdbm.createLogsDirectory()

	if err != nil {
		return err
	}

	defer logFile.Close()

	logWriter := bufio.NewWriter(logFile)
	defer logWriter.Flush()

	cdbm.migrateCfg.LogWriter = func(innerErr error) {
		logWriter.WriteString(time.Now().UTC().Format(webutil.FormDateTimeLayout) + ": " + innerErr.Error() + "\n")
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

	// If startingVersion != cdbm.migrateCfg.TargetVersion than we know user wants to migrate up or down
	// or if schemaMigrations#NoRows is true than we know no migrations have been applied yet
	if cdbm.migrateCfg.SchemaMigration.StartingVersion != cdbm.migrateCfg.TargetVersion ||
		cdbm.migrateCfg.SchemaMigration.SchemaCfg.NoRows || cdbm.migrateCfg.SchemaMigration.Dirty {
		cdbm.migrateCfg.MigrateType = MigrateTypeUp

		if cdbm.migrateCfg.SchemaMigration.StartingVersion > cdbm.migrateCfg.TargetVersion {
			cdbm.migrateCfg.MigrateType = MigrateTypeDown
		}

		cdbm.migrateCfg.SchemaMigration.SchemaCfg.HasEntry = true

		if cdbm.migrateCfg.SchemaMigration.SchemaCfg.NoRows {
			cdbm.migrateCfg.SchemaMigration.SchemaCfg.HasEntry = false
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

		for _, v := range migrationsProtocolList {
			if v == cdbm.MigrateFlags.MigrationsProtocol {
				found = true
				break
			}
		}

		if !found {
			return errors.WithStack(
				fmt.Errorf("invalid --migrations-protocol;  Valid --migrations-protocol are %v", migrationsProtocolList),
			)
		}
	}

	return nil
}

// applyQueries sets up our insert and update schema_migrations queries by
// apply sql bind var
func (cdbm *CDBM) applyQueries() error {
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
	var err error

	logDir := cdbm.MigrateFlags.MigrationsDir + "logs/"

	if err = os.MkdirAll(logDir, os.ModePerm); err != nil {
		return nil, errors.WithStack(err)
	}

	return os.OpenFile(logDir+"logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.ModePerm)
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
	for _, v := range files {
		if v.IsDir() {
			continue
		}

		fileNameSlice := strings.Split(v.Name(), "_")

		if len(fileNameSlice) == 1 {
			return nil, errors.WithStack(ErrInvalidFileName)
		}

		version, err := strconv.Atoi(fileNameSlice[0])

		if err != nil {
			return nil, errors.WithStack(ErrInvalidFileName)
		}

		// Migrations should not be lower than 1
		if version < 1 {
			return nil, errors.WithStack(
				fmt.Errorf("migration file version less than min version allowed (1)"),
			)
		}

		bodySlice := strings.Split(fileNameSlice[1], ".")

		if len(bodySlice) != 3 {
			return nil, errors.WithStack(ErrInvalidFileName)
		}

		if bodySlice[1] != "up" && bodySlice[1] != "down" {
			return nil, errors.WithStack(ErrInvalidFileName)
		}

		if bodySlice[2] != "sql" {
			return nil, errors.WithStack(ErrInvalidFileName)
		}

		_, ok := fileVersions[version]
		_, customOK := cdbm.migrateCfg.CustomMigrations[version]

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

	invalidCustomVersions := make([]int, 0)

	// Loop through custom migrations to make sure there are no duplicate
	// versions between files and custom migrations
	for k, v := range cdbm.migrateCfg.CustomMigrations {
		if _, ok := fileVersions[k]; !ok {
			invalidCustomVersions = append(invalidCustomVersions, k)
		}

		mac := migrationApplyConfig{
			Version:         k,
			CustomMigration: v,
		}

		migrationApplyCfgs = append(migrationApplyCfgs, mac)
	}

	if len(migrationApplyCfgs) == 0 {
		return nil, fmt.Errorf("no sql files or custom migrations found")
	}

	if len(invalidCustomVersions) > 0 {
		return nil, errors.WithStack(
			fmt.Errorf(
				"following custom versions are out of range from files: %v",
				invalidCustomVersions,
			),
		)
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
		//fmt.Printf("current version: %d\n", version)
		// Check if current version is apart of a custom migration
		//
		// If it is, apply down migration of version if func is not nil
		//
		// Else run file down migrations
		if migration, ok := cdbm.migrateCfg.CustomMigrations[version]; ok {
			if migration.Down != nil {
				if err = migration.Down(cdbm.DB); err != nil {
					cdbm.migrateCfg.LogWriter(err)

					var query string

					if !cdbm.migrateCfg.SchemaMigration.SchemaCfg.HasEntry {
						query = cdbm.migrateCfg.InsertQuery
					} else {
						query = cdbm.migrateCfg.UpdateQuery
					}

					if _, err = cdbm.DB.Exec(
						query,
						version,
						true,
						MigrateTypeDown,
						true,
					); err != nil {
						cdbm.migrateCfg.LogWriter(err)
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
				MigrateTypeDown,
			); err != nil {
				cdbm.migrateCfg.LogWriter(err)

				if _, err = cdbm.DB.Exec(
					cdbm.migrateCfg.UpdateQuery,
					version,
					true,
					MigrateTypeDown,
					false,
				); err != nil {
					cdbm.migrateCfg.LogWriter(err)
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
func (cdbm *CDBM) applyCustomMigration(version int, cmFunc CustomMigrationFunc) error {
	var err, innerErr error

	if err = cmFunc(cdbm.DB); err != nil {
		cdbm.migrateCfg.LogWriter(err)

		var errMsg string

		if cdbm.migrateCfg.MigrateType == MigrateTypeUp {
			errMsg = "failed on custom up migration for version: '%d'"
		} else {
			errMsg = "failed on custom down migration for version: '%d'"
		}

		err = fmt.Errorf(errMsg, version)

		var query string

		if cdbm.migrateCfg.SchemaMigration.SchemaCfg.HasEntry {
			query = cdbm.migrateCfg.UpdateQuery
		} else {
			query = cdbm.migrateCfg.InsertQuery
		}

		if cdbm.MigrateFlags.RollbackOnFailure &&
			cdbm.migrateCfg.MigrateType == MigrateTypeUp {

			if innerErr = cdbm.migrationRollbackFail(version); innerErr != nil {
				return fmt.Errorf(err.Error() + " and " + innerErr.Error())
			}

			if _, innerErr = cdbm.DB.Exec(
				query,
				cdbm.migrateCfg.SchemaMigration.StartingVersion,
				false,
				"",
				cdbm.migrateCfg.SchemaMigration.IsCustomMigration,
			); innerErr != nil {
				cdbm.migrateCfg.LogWriter(innerErr)
			}

			return fmt.Errorf(
				err.Error()+" but successfully rolled back to version: '%d'",
				cdbm.migrateCfg.SchemaMigration.StartingVersion,
			)
		} else {
			if _, innerErr = cdbm.DB.Exec(
				query,
				version,
				true,
				cdbm.migrateCfg.MigrateType,
				true,
			); innerErr != nil {
				cdbm.migrateCfg.LogWriter(innerErr)
			}
		}

		return err
	}

	isCustom := true

	if cdbm.migrateCfg.MigrateType == MigrateTypeDown {
		version--

		if _, ok := cdbm.migrateCfg.CustomMigrations[version]; !ok {
			isCustom = false
		}
	}

	if !cdbm.migrateCfg.SchemaMigration.SchemaCfg.HasEntry {
		if _, err = cdbm.DB.Exec(
			cdbm.migrateCfg.InsertQuery,
			version,
			false,
			"",
			isCustom,
		); err != nil {
			cdbm.migrateCfg.LogWriter(err)
			return errors.WithStack(err)
		}

		cdbm.migrateCfg.SchemaMigration.SchemaCfg.HasEntry = true
	} else {
		if _, err = cdbm.DB.Exec(
			cdbm.migrateCfg.UpdateQuery,
			version,
			false,
			"",
			isCustom,
		); err != nil {
			cdbm.migrateCfg.LogWriter(err)
			return errors.WithStack(err)
		}
	}

	return nil
}

// applyFileMigration applies file migration to database
func (cdbm *CDBM) applyFileMigration(version int) error {
	var err, innerErr error

	if cdbm.migrateCfg.SchemaMigration.SchemaCfg.Dirty && cdbm.migrateCfg.MigrateType == MigrateTypeUp {
		if err = cdbm.migrateCfg.FileMigration(
			cdbm.migrateCfg.Migrate,
			version,
			MigrateTypeDown,
		); err != nil && !cdbm.MigrateFlags.SkipFileMigrationResetOnFailure {
			cdbm.migrateCfg.LogWriter(err)

			if _, err = cdbm.DB.Exec(
				cdbm.migrateCfg.UpdateQuery,
				version,
				true,
				MigrateTypeDown,
				false,
			); err != nil {
				cdbm.migrateCfg.LogWriter(err)
			}

			return errors.WithStack(
				fmt.Errorf("failed of resetting migration for version '%d'", version),
			)
		}

		cdbm.migrateCfg.SchemaMigration.SchemaCfg.Dirty = false
	}

	if err = cdbm.migrateCfg.FileMigration(
		cdbm.migrateCfg.Migrate,
		version,
		cdbm.migrateCfg.MigrateType,
	); err != nil {
		var errMsg string

		if cdbm.migrateCfg.MigrateType == MigrateTypeUp {
			errMsg = "failed on file up migration for version: '%d'"
		} else {
			errMsg = "failed on file down migration for version: '%d'"
		}

		cdbm.migrateCfg.LogWriter(err)
		err = fmt.Errorf(errMsg, version)

		if cdbm.MigrateFlags.RollbackOnFailure &&
			cdbm.migrateCfg.MigrateType == MigrateTypeUp {
			if !cdbm.migrateCfg.SchemaMigration.SchemaCfg.Dirty {
				if _, innerErr = cdbm.DB.Exec(
					cdbm.migrateCfg.UpdateQuery,
					version,
					false,
					"",
					false,
				); innerErr != nil {
					cdbm.migrateCfg.LogWriter(innerErr)
				}
			}

			if innerErr = cdbm.migrationRollbackFail(version); innerErr != nil {
				return fmt.Errorf(err.Error() + " and " + innerErr.Error())
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
				cdbm.migrateCfg.LogWriter(innerErr)
			}
		}

		return err
	} else {
		isCustom := false

		if cdbm.migrateCfg.MigrateType == MigrateTypeDown {
			version--

			if _, ok := cdbm.migrateCfg.CustomMigrations[version]; ok {
				isCustom = true
			}
		}

		if _, err = cdbm.DB.Exec(
			cdbm.migrateCfg.UpdateQuery,
			version,
			false,
			"",
			isCustom,
		); err != nil {
			cdbm.migrateCfg.LogWriter(err)
			return errors.WithStack(err)
		}
	}

	if !cdbm.migrateCfg.SchemaMigration.SchemaCfg.HasEntry {
		sm, err := cdbm.getSchemaMigration()

		if err != nil {
			return errors.WithStack(err)
		}

		if !sm.SchemaCfg.NoRows {
			cdbm.migrateCfg.SchemaMigration.SchemaCfg.HasEntry = true
		}
	}

	return nil
}

// applyMigrationConfig will apply given migrationApplyConfig to database
func (cdbm *CDBM) applyMigrationConfig(cfg migrationApplyConfig) error {
	fmt.Printf("apply migration version: %d\n", cfg.Version)

	var cm CustomMigrationFunc
	var err error

	isCustom := false

	if cfg.CustomMigration.Up != nil || cfg.CustomMigration.Down != nil {
		isCustom = true
	}

	if cdbm.migrateCfg.MigrateType == MigrateTypeUp {
		cm = cfg.CustomMigration.Up
	} else {
		cm = cfg.CustomMigration.Down
	}

	if isCustom {
		if cm != nil {
			if err = cdbm.applyCustomMigration(cfg.Version, cm); err != nil {
				return err
			}
		}
	} else {
		if err = cdbm.applyFileMigration(cfg.Version); err != nil {
			return err
		}
	}

	return nil
}

// runMigrationConfigs will apply given slice of migrationApplyConfig to database
func (cdbm *CDBM) runMigrationConfigs(cfgs []migrationApplyConfig) error {
	var err error

	if cdbm.migrateCfg.MigrateType == MigrateTypeUp {
		for _, v := range cfgs {
			if cdbm.migrateCfg.SchemaMigration.Dirty {
				if v.Version < cdbm.migrateCfg.SchemaMigration.StartingVersion {
					continue
				}
			} else {
				if v.Version <= cdbm.migrateCfg.SchemaMigration.StartingVersion {
					continue
				}
			}

			if cdbm.migrateCfg.TargetVersion < v.Version {
				break
			}

			if err = cdbm.applyMigrationConfig(v); err != nil {
				return err
			}
		}
	} else {
		for i := len(cfgs) - 1; i >= 0; i-- {
			if cfgs[i].Version > cdbm.migrateCfg.SchemaMigration.StartingVersion {
				continue
			}

			if cdbm.migrateCfg.TargetVersion >= cfgs[i].Version {
				break
			}

			if err = cdbm.applyMigrationConfig(cfgs[i]); err != nil {
				return err
			}
		}
	}

	return nil
}
