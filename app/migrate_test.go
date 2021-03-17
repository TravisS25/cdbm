package app

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/TravisS25/cdbm/cdbmutil"
	"github.com/TravisS25/webutil/webutil"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/pkg/errors"
)

// func getEnvSettings(t *testing.T) CDBMSettings {
// 	var err error

// 	viper.SetConfigFile(os.Getenv(defaultEnvVar))

// 	if err = viper.ReadInConfig(); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	var settings CDBMSettings

// 	if viper.Unmarshal(&settings); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	return settings
// }

func TestMigrate(t *testing.T) {
	var err error
	var mApp *CDBM

	settings, err := cdbmutil.GetCDBMUtilSettings("")

	if err != nil {
		t.Fatalf(err.Error())
	}

	dbProtocolCfg := DefaultProtocolMap[DBProtocol(settings.BaseDatabaseSettings.DatabaseProtocol)]
	dbName := cdbmutil.GetRandomString(10)
	stdErr := &bytes.Buffer{}
	createCmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(settings.DBAction.CreateDB, dbName))
	createCmd.Stderr = stdErr

	if err = createCmd.Run(); err != nil {
		t.Fatalf(stdErr.String())
	}

	dropCmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(settings.DBAction.DropDB, dbName))
	defer dropCmd.Start()

	dbSettings := settings.BaseDatabaseSettings.Settings
	dbSettings.DBName = dbName

	db, err := webutil.NewDB(dbSettings, dbProtocolCfg.DatabaseType)

	if err != nil {
		t.Fatalf(err.Error())
	}

	rootDir := "/tmp/migrate-tool/"
	migrationsDir := rootDir + "migrations/"

	defer os.RemoveAll(migrationsDir)

	testGetMigrationFunc := func(migDir string, db *sql.DB, protocolCfg DBProtocolConfig) (*migrate.Migrate, error) {
		return &migrate.Migrate{}, nil
	}
	testFileMigrationFunc := func(mig *migrate.Migrate, version int, mt MigrationType) error {
		return nil
	}

	// --------------------------------------------------------------------------

	// Test reading from directory where no files exists
	mApp = &CDBM{
		DB:            db,
		DBProtocolCfg: dbProtocolCfg,
		MigrateFlags: MigrateFlagsConfig{
			MigrationsDir: migrationsDir + "invalid/",
			TargetVersion: -1,
		},
	}

	if err = mApp.Migrate(
		testGetMigrationFunc,
		testFileMigrationFunc,
		map[int]CustomMigration{},
	); err == nil {
		t.Errorf("Should have error")
	} else if err.Error() != "no sql files or custom migrations found" {
		t.Errorf("Should have no sql files error; got %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	if err = os.RemoveAll(migrationsDir); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000000_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	// Should error out due to migration file being below min required
	mApp = &CDBM{
		DB:            db,
		DBProtocolCfg: dbProtocolCfg,
		MigrateFlags: MigrateFlagsConfig{
			TargetVersion: -1,
			MigrationsDir: migrationsDir,
		},
	}

	if err = mApp.Migrate(
		testGetMigrationFunc,
		testFileMigrationFunc,
		map[int]CustomMigration{},
	); err == nil {
		t.Errorf("should have error")
	} else {
		if err.Error() != "migration file version less than min version allowed (1)" {
			t.Errorf("Should have migration file error; got: %s\n", err.Error())
		}
	}

	// --------------------------------------------------------------------------

	if err = os.RemoveAll(migrationsDir); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "invalid.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	// Should error out due to invalid file name
	mApp = &CDBM{
		DB:            db,
		DBProtocolCfg: dbProtocolCfg,
		MigrateFlags: MigrateFlagsConfig{
			TargetVersion: -1,
			MigrationsDir: migrationsDir,
		},
	}

	if err = mApp.Migrate(
		testGetMigrationFunc,
		testFileMigrationFunc,
		map[int]CustomMigration{},
	); err == nil {
		t.Errorf("should have error")
	} else {
		if err.Error() != ErrInvalidFileName.Error() {
			t.Errorf("Should have invalid sql file name error; got %s\n", err.Error())
		}
	}

	// --------------------------------------------------------------------------

	if err = os.RemoveAll(migrationsDir); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "invalidVersion_update.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	// Should error out due to invalid file name
	mApp = &CDBM{
		DB:            db,
		DBProtocolCfg: dbProtocolCfg,
		MigrateFlags: MigrateFlagsConfig{
			TargetVersion: -1,
			MigrationsDir: migrationsDir,
		},
	}

	if err = mApp.Migrate(
		testGetMigrationFunc,
		testFileMigrationFunc,
		map[int]CustomMigration{},
	); err == nil {
		t.Errorf("should have error")
	} else {
		if err.Error() != ErrInvalidFileName.Error() {
			t.Errorf("Should have invalid sql file name error; got %s\n", err.Error())
		}
	}

	// --------------------------------------------------------------------------

	if err = os.RemoveAll(migrationsDir); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000001_foo.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	// Should error out due to invalid file name
	mApp = &CDBM{
		DB:            db,
		DBProtocolCfg: dbProtocolCfg,
		MigrateFlags: MigrateFlagsConfig{
			TargetVersion: -1,
			MigrationsDir: migrationsDir,
		},
	}

	if err = mApp.Migrate(
		testGetMigrationFunc,
		testFileMigrationFunc,
		map[int]CustomMigration{},
	); err == nil {
		t.Errorf("should have error")
	} else {
		if err.Error() != ErrInvalidFileName.Error() {
			t.Errorf("Should have invalid sql file name error; got %s\n", err.Error())
		}
	}

	// --------------------------------------------------------------------------

	if err = os.RemoveAll(migrationsDir); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000001_foo.invalid.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	// Should error out due to invalid file name
	mApp = &CDBM{
		DB:            db,
		DBProtocolCfg: dbProtocolCfg,
		MigrateFlags: MigrateFlagsConfig{
			TargetVersion: -1,
			MigrationsDir: migrationsDir,
		},
	}

	if err = mApp.Migrate(
		testGetMigrationFunc,
		testFileMigrationFunc,
		map[int]CustomMigration{},
	); err == nil {
		t.Errorf("should have error")
	} else {
		if err.Error() != ErrInvalidFileName.Error() {
			t.Errorf("Should have invalid sql file name error; got %s\n", err.Error())
		}
	}

	// --------------------------------------------------------------------------

	if err = os.RemoveAll(migrationsDir); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000001_invalid.up.invalid"); err != nil {
		t.Fatalf(err.Error())
	}

	// Should error out due to invalid file name
	mApp = &CDBM{
		DB:            db,
		DBProtocolCfg: dbProtocolCfg,
		MigrateFlags: MigrateFlagsConfig{
			TargetVersion: -1,
			MigrationsDir: migrationsDir,
		},
	}

	if err = mApp.Migrate(
		testGetMigrationFunc,
		testFileMigrationFunc,
		map[int]CustomMigration{},
	); err == nil {
		t.Errorf("should have error")
	} else {
		if err.Error() != ErrInvalidFileName.Error() {
			t.Errorf("Should have invalid sql file name error; got %s\n", err.Error())
		}
	}

	// --------------------------------------------------------------------------

	if err = os.RemoveAll(migrationsDir); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000001_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000004_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	// Should be invalid due to duplicate versions between files and custom migrations
	mApp = &CDBM{
		DB:            db,
		DBProtocolCfg: dbProtocolCfg,
		MigrateFlags: MigrateFlagsConfig{
			TargetVersion: -1,
			MigrationsDir: migrationsDir,
		},
	}

	if err = mApp.Migrate(
		testGetMigrationFunc,
		testFileMigrationFunc,
		map[int]CustomMigration{
			1: {
				Up: func(db webutil.DBInterface) error {
					return nil
				},
			},
		},
	); err == nil {
		t.Errorf("should have error")
	} else {
		if !strings.Contains(err.Error(), "following versions are duplicated between") {
			t.Errorf("Should have version duplicate error; got %s\n", err.Error())
		}
	}

	// --------------------------------------------------------------------------

	if err = os.RemoveAll(migrationsDir); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000001_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000002_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000003_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	// Should be valid; checking that we get no rows err for MigrationTableSearch as
	// the schema_migrations table shouldn't exist yet
	//
	// Also testing that we break out of loop based on lower target version than available
	mApp = &CDBM{
		DB:            db,
		DBProtocolCfg: dbProtocolCfg,
		MigrateFlags: MigrateFlagsConfig{
			TargetVersion: 2,
			MigrationsDir: migrationsDir,
		},
	}

	if err = mApp.Migrate(
		testGetMigrationFunc,
		testFileMigrationFunc,
		map[int]CustomMigration{},
	); err != nil {
		t.Errorf("should not have error; got: %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	if err = os.RemoveAll(migrationsDir); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000001_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000004_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	mApp = &CDBM{
		DB:            db,
		DBProtocolCfg: dbProtocolCfg,
		MigrateFlags: MigrateFlagsConfig{
			TargetVersion: 5,
			MigrationsDir: migrationsDir,
		},
	}

	var timeStamp3, timeStamp4, timeStamp6 time.Time
	var version3Called, version4Called, version6Called bool

	// Verifying that custom migrations are ordered properly and called in order
	// with a target-version set
	if err = mApp.Migrate(
		testGetMigrationFunc,
		testFileMigrationFunc,
		map[int]CustomMigration{
			3: {
				Up: func(db webutil.DBInterface) error {
					version4Called = true
					timeStamp4 = time.Now()
					//time.Sleep(time.Millisecond * 500)
					return nil
				},
			},
			2: {
				Up: func(db webutil.DBInterface) error {
					version3Called = true
					timeStamp3 = time.Now()
					//time.Sleep(time.Millisecond * 500)
					return nil
				},
			},
			5: {
				Up: func(db webutil.DBInterface) error {
					version6Called = true
					timeStamp6 = time.Now()
					//time.Sleep(time.Millisecond * 500)
					return nil
				},
			},
		},
	); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}

	if !version3Called {
		t.Errorf("version 3 not called")
	}
	if !version4Called {
		t.Errorf("version 4 not called")
	}
	if !version6Called {
		t.Errorf("version 6 not called")
	}

	if timeStamp3.After(timeStamp4) || timeStamp3.After(timeStamp6) {
		t.Errorf("timeStamp3 should not be after timeStamp4 or timeStamp6")
	}
	if timeStamp4.After(timeStamp6) || timeStamp4.Before(timeStamp3) {
		t.Errorf("timeStamp4 should not be after timeStamp6 or before timeStamp3")
	}
	if timeStamp6.Before(timeStamp3) || timeStamp6.Before(timeStamp4) {
		t.Errorf("timeStamp6 should not be before timeStamp3 or timeStamp4")
	}

	// --------------------------------------------------------------------------

	if _, err = db.Exec("delete from schema_migrations"); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.RemoveAll(migrationsDir); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000001_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000004_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	mApp = &CDBM{
		DB:            db,
		DBProtocolCfg: dbProtocolCfg,
		MigrateFlags: MigrateFlagsConfig{
			TargetVersion: -1,
			MigrationsDir: migrationsDir,
		},
	}

	// Verifying that custom migrations error activates
	if err = mApp.Migrate(
		testGetMigrationFunc,
		testFileMigrationFunc,
		map[int]CustomMigration{
			3: {
				Up: func(db webutil.DBInterface) error {
					return nil
				},
			},
			2: {
				Up: func(db webutil.DBInterface) error {
					return fmt.Errorf("custom migration error")
				},
			},
			5: {
				Up: func(db webutil.DBInterface) error {
					return nil
				},
			},
		},
	); err == nil {
		t.Errorf("should have error")
	} else {
		if errors.Cause(err).Error() != "failed on custom up migration for version: '2'" {
			t.Errorf("should have custom migration error; got %s\n", err.Error())
		}
	}

	// --------------------------------------------------------------------------

	if _, err = db.Exec("delete from schema_migrations"); err != nil {
		t.Fatalf(err.Error())
	}

	schemaInsert, args, err := webutil.InQueryRebind(
		dbProtocolCfg.SQLBindVar,
		`
		insert into schema_migrations(version, dirty)
		values(?, ?);
		`,
		2,
		true,
	)

	if err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = db.Exec(schemaInsert, args...); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.RemoveAll(migrationsDir); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000001_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	mApp = &CDBM{
		DB:            db,
		DBProtocolCfg: dbProtocolCfg,
		MigrateFlags: MigrateFlagsConfig{
			TargetVersion: -1,
			MigrationsDir: migrationsDir,
		},
	}

	// Verifying that we get error message for not setting --reset-dirty-flag flag when
	// a migration has a dirty flag
	if err = mApp.Migrate(
		testGetMigrationFunc,
		testFileMigrationFunc,
		map[int]CustomMigration{
			2: {
				Up: func(db webutil.DBInterface) error {
					return nil
				},
			},
		},
	); err == nil {
		t.Errorf("should have error")
	} else {
		if err.Error() != "must set --reset-dirty-flag to reset migrations dirty flag.  Use 'cdbm status' to see current status of migration" {
			t.Errorf("should have reset dirt flag error: got %s\n", err.Error())
		}
	}

	// --------------------------------------------------------------------------

	if _, err = db.Exec("delete from schema_migrations"); err != nil {
		t.Fatalf(err.Error())
	}

	schemaInsert, args, err = webutil.InQueryRebind(
		dbProtocolCfg.SQLBindVar,
		`
		insert into schema_migrations(version, dirty)
		values(?, ?);
		`,
		2,
		true,
	)

	if err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = db.Exec(schemaInsert, args...); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.RemoveAll(migrationsDir); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000001_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	mApp = &CDBM{
		DB:            db,
		DBProtocolCfg: dbProtocolCfg,
		MigrateFlags: MigrateFlagsConfig{
			TargetVersion:  -1,
			MigrationsDir:  migrationsDir,
			ResetDirtyFlag: true,
		},
	}

	// Verifying that we when set --reset-dirty-flag flag that we don't get an error
	if err = mApp.Migrate(
		testGetMigrationFunc,
		testFileMigrationFunc,
		map[int]CustomMigration{
			2: {
				Up: func(db webutil.DBInterface) error {
					return nil
				},
			},
		},
	); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	if _, err = db.Exec("delete from schema_migrations"); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.RemoveAll(migrationsDir); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000001_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	// Testing target version being too high
	mApp = &CDBM{
		DB:            db,
		DBProtocolCfg: dbProtocolCfg,
		MigrateFlags: MigrateFlagsConfig{
			TargetVersion: 100,
			MigrationsDir: migrationsDir,
		},
	}

	if err = mApp.Migrate(
		testGetMigrationFunc,
		testFileMigrationFunc,
		map[int]CustomMigration{},
	); err == nil {
		t.Errorf("should have error")
	} else {
		if err.Error() != "--target-version does not exist" {
			t.Errorf("should have --target-version error; got %s\n", err.Error())
		}
	}

	// --------------------------------------------------------------------------

	if _, err = db.Exec("delete from schema_migrations"); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.RemoveAll(migrationsDir); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000001_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000002_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	// Testing target version
	mApp = &CDBM{
		DB:            db,
		DBProtocolCfg: dbProtocolCfg,
		MigrateFlags: MigrateFlagsConfig{
			TargetVersion: 1,
			MigrationsDir: migrationsDir,
		},
	}

	if err = mApp.Migrate(
		testGetMigrationFunc,
		testFileMigrationFunc,
		map[int]CustomMigration{},
	); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	if _, err = db.Exec("delete from schema_migrations"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = db.Exec(
		`
		insert into schema_migrations(version, dirty)
		values(1, false)
		`,
	); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.RemoveAll(migrationsDir); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000001_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000002_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000003_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	// Testing rollback on failure for file migration and failure on the rollback
	// itself for file migration
	mApp = &CDBM{
		DB:            db,
		DBProtocolCfg: dbProtocolCfg,
		MigrateFlags: MigrateFlagsConfig{
			TargetVersion:     -1,
			MigrationsDir:     migrationsDir,
			RollbackOnFailure: true,
		},
	}

	if err = mApp.Migrate(
		testGetMigrationFunc,
		func(mig *migrate.Migrate, version int, mt MigrationType) error {
			if version == 3 {
				return fmt.Errorf("file migration error")
			}

			return nil
		},
		map[int]CustomMigration{},
	); err == nil {
		t.Errorf("should have error\n")
	} else {
		if !strings.Contains(err.Error(), "failed on file up migration for version: '3'") {
			t.Errorf("should have file up migration error; got %s\n", err.Error())
		}

		if !strings.Contains(err.Error(), "failed on file rollback migration for version: '3'") {
			t.Errorf("should have file rollback error; got %s\n", err.Error())
		}
	}

	// --------------------------------------------------------------------------

	if _, err = db.Exec("delete from schema_migrations"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = db.Exec(
		`
		insert into schema_migrations(version, dirty)
		values(1, false)
		`,
	); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.RemoveAll(migrationsDir); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000001_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000003_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	// Testing rollback on failure for custom migration and failure on the rollback
	// itself for custom migration
	mApp = &CDBM{
		DB:            db,
		DBProtocolCfg: dbProtocolCfg,
		MigrateFlags: MigrateFlagsConfig{
			TargetVersion:     -1,
			MigrationsDir:     migrationsDir,
			RollbackOnFailure: true,
		},
	}

	if err = mApp.Migrate(
		testGetMigrationFunc,
		testFileMigrationFunc,
		map[int]CustomMigration{
			2: {
				Up: func(db webutil.DBInterface) error {
					return fmt.Errorf("custom migration error")
				},
				Down: func(db webutil.DBInterface) error {
					return fmt.Errorf("custom migration error")
				},
			},
		},
	); err == nil {
		t.Errorf("should have error\n")
	} else {
		if !strings.Contains(err.Error(), "failed on custom up migration for version: '2'") {
			t.Errorf("should have custom up migration error; got %s\n", err.Error())
		}

		if !strings.Contains(err.Error(), "failed on custom rollback migration for version: '2'") {
			t.Errorf("should have custom rollback error; got %s\n", err.Error())
		}
	}

	// --------------------------------------------------------------------------

	if _, err = db.Exec("delete from schema_migrations"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = db.Exec(
		`
		insert into schema_migrations(version, dirty)
		values(2, false)
		`,
	); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.RemoveAll(migrationsDir); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000001_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000001_update.down.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000002_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000002_update.down.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000003_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000003_update.down.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	// Testing file down migration with error
	mApp = &CDBM{
		DB:            db,
		DBProtocolCfg: dbProtocolCfg,
		MigrateFlags: MigrateFlagsConfig{
			TargetVersion:     1,
			MigrationsDir:     migrationsDir,
			RollbackOnFailure: true,
		},
	}

	if err = mApp.Migrate(
		testGetMigrationFunc,
		func(mig *migrate.Migrate, version int, mt MigrationType) error {
			return fmt.Errorf("file migration error")
		},
		map[int]CustomMigration{},
	); err == nil {
		t.Errorf("should have error")
	} else if err.Error() != "failed on file down migration for version: '2'" {
		t.Errorf("should have file down migration; got %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	if _, err = db.Exec("delete from schema_migrations"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = db.Exec(
		`
		insert into schema_migrations(version, dirty)
		values(3, false)
		`,
	); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.RemoveAll(migrationsDir); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000001_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000001_update.down.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000003_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000003_update.down.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	// Testing custom down migration with error
	mApp = &CDBM{
		DB:            db,
		DBProtocolCfg: dbProtocolCfg,
		MigrateFlags: MigrateFlagsConfig{
			TargetVersion:     1,
			MigrationsDir:     migrationsDir,
			RollbackOnFailure: true,
		},
	}

	if err = mApp.Migrate(
		testGetMigrationFunc,
		testFileMigrationFunc,
		map[int]CustomMigration{
			2: {
				Down: func(db webutil.DBInterface) error {
					return fmt.Errorf("custom migration error")
				},
			},
		},
	); err == nil {
		t.Errorf("should have error")
	} else if err.Error() != "failed on custom down migration for version: '2'" {
		t.Errorf("should have file down migration; got %s\n", err.Error())
	}
}

func TestMigrateIntegration(t *testing.T) {
	var err error
	var mApp *CDBM

	settings, err := cdbmutil.GetCDBMUtilSettings("")

	if err != nil {
		t.Fatalf(err.Error())
	}

	dbProtocolCfg := DefaultProtocolMap[DBProtocol(settings.BaseDatabaseSettings.DatabaseProtocol)]
	dbName := cdbmutil.GetRandomString(10)
	stdErr := &bytes.Buffer{}
	createCmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(settings.DBAction.CreateDB, dbName))
	createCmd.Stderr = stdErr

	if err = createCmd.Run(); err != nil {
		t.Fatalf(stdErr.String())
	}

	dropCmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(settings.DBAction.DropDB, dbName))
	defer dropCmd.Start()

	dbSettings := settings.BaseDatabaseSettings.Settings
	dbSettings.DBName = dbName

	db, err := webutil.NewDB(dbSettings, dbProtocolCfg.DatabaseType)

	if err != nil {
		t.Fatalf(err.Error())
	}

	rootDir := "/tmp/migrate-integration/"
	migrationsDir := rootDir + "migrations/"

	defer os.RemoveAll(migrationsDir)

	var file1Up, file1Down, file2Up, file2Down *os.File

	// --------------------------------------------------------------------------

	if err = os.RemoveAll(migrationsDir); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
		t.Fatalf(err.Error())
	}

	if file1Up, err = os.Create(migrationsDir + "000001_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = file1Up.Write([]byte(
		`
		create table foo(
			id int not null primary key,
			name string not null
		);
		`,
	)); err != nil {
		t.Fatalf(err.Error())
	}

	if file1Down, err = os.Create(migrationsDir + "000001_update.down.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = file1Down.Write([]byte("drop table foo;")); err != nil {
		t.Fatalf(err.Error())
	}

	if file2Up, err = os.Create(migrationsDir + "000002_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = file2Up.Write([]byte(
		`
		create table bar(
			id int not null primary key,
			name string not null
		);
		`,
	)); err != nil {
		t.Fatalf(err.Error())
	}

	if file2Down, err = os.Create(migrationsDir + "000002_update.down.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = file2Down.Write([]byte("drop table bar;")); err != nil {
		t.Fatalf(err.Error())
	}

	// Should be valid
	mApp = &CDBM{
		DB:            db,
		DBProtocolCfg: dbProtocolCfg,
		DropFlags: DropFlagsConfig{
			Confirm: true,
		},
		MigrateFlags: MigrateFlagsConfig{
			TargetVersion: -1,
			MigrationsDir: migrationsDir,
		},
	}

	if err = mApp.Migrate(
		DefaultGetMigrationFunc,
		DefaultFileMigrationFunc,
		map[int]CustomMigration{},
	); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}

	if err = mApp.Drop(); err != nil {
		t.Fatalf(err.Error())
	}

	// --------------------------------------------------------------------------

	var file3Up, file3Down, file4Up, file4Down *os.File

	if err = os.RemoveAll(migrationsDir); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
		t.Fatalf(err.Error())
	}

	// Simulating migrations of first two migrations
	if _, err = db.Exec(
		`
		create table schema_migrations(
			version int primary key,
			dirty boolean not null,
			dirty_state text
		);

		insert into schema_migrations(version, dirty)
		values (2, false);

		create table foo(
			id serial,
			name string not null
		);

		insert into foo(name)
		values ('test1');

		insert into foo(name)
		values ('test2');
		`,
	); err != nil {
		t.Fatalf(err.Error())
	}

	if file1Up, err = os.Create(migrationsDir + "000001_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = file1Up.Write([]byte(
		`
		create table foo(
			id int not null primary key,
			name string not null
		);

		insert into foo(name)
		values ('test1');
		`,
	)); err != nil {
		t.Fatalf(err.Error())
	}

	if file1Down, err = os.Create(migrationsDir + "000001_update.down.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = file1Down.Write([]byte("drop table foo;")); err != nil {
		t.Fatalf(err.Error())
	}

	if file2Up, err = os.Create(migrationsDir + "000002_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = file2Up.Write([]byte(
		`
		insert into foo(name)
		values ('test2');
		`,
	)); err != nil {
		t.Fatalf(err.Error())
	}

	if file2Down, err = os.Create(migrationsDir + "000002_update.down.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = file2Down.Write([]byte("delete from foo where name = 'test2'")); err != nil {
		t.Fatalf(err.Error())
	}

	if file3Up, err = os.Create(migrationsDir + "000003_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = file3Up.Write([]byte(
		`
		insert into foo(name)
		values ('test3');
		`,
	)); err != nil {
		t.Fatalf(err.Error())
	}

	if file3Down, err = os.Create(migrationsDir + "000003_update.down.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = file3Down.Write([]byte("delete from foo where name = 'test3'")); err != nil {
		t.Fatalf(err.Error())
	}

	if file4Up, err = os.Create(migrationsDir + "000004_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = file4Up.Write([]byte(
		`
		insert into foo(name)
		values ('test4');
		`,
	)); err != nil {
		t.Fatalf(err.Error())
	}

	if file4Down, err = os.Create(migrationsDir + "000004_update.down.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = file4Down.Write([]byte("delete from foo where name = 'test4'")); err != nil {
		t.Fatalf(err.Error())
	}

	// Testing rollback on failure integration
	mApp = &CDBM{
		DB:            db,
		DBProtocolCfg: dbProtocolCfg,
		MigrateFlags: MigrateFlagsConfig{
			TargetVersion:     -1,
			MigrationsDir:     migrationsDir,
			RollbackOnFailure: true,
		},
	}

	if err = mApp.Migrate(
		DefaultGetMigrationFunc,
		func(mig *migrate.Migrate, version int, mt MigrationType) error {
			if version == 4 && mt == MigrateTypeUp {
				return fmt.Errorf("file error")
			}

			switch mt {
			case MigrateTypeUp:
				return mig.Steps(1)
			case MigrateTypeDown:
				return mig.Steps(-1)
			}

			return nil
		},
		map[int]CustomMigration{},
	); err == nil {
		t.Errorf("should have error")
	} else if err.Error() != "failed on file up migration for version: '4'" {
		t.Errorf("should have file up migration; got %s\n", err.Error())
	}

	var sm schemaMigration

	if err = db.QueryRow(
		`
		select
			schema_migrations.version,
			schema_migrations.dirty,
			schema_migrations.dirty_state
		from
			schema_migrations
		`,
	).Scan(&sm.StartingVersion, &sm.Dirty, &sm.DirtyState); err != nil {
		t.Fatalf(err.Error())
	}

	if sm.StartingVersion != 2 {
		t.Errorf("version should be 2; got %d\n", sm.StartingVersion)
	}
	if sm.Dirty {
		t.Errorf("should not be dirty\n")
	}
	if sm.DirtyState != nil && *sm.DirtyState != "" {
		t.Errorf("should not have a dirty state; got %s\n", *sm.DirtyState)
	}

	var filler interface{}

	err = db.QueryRow(
		`
		select
			foo.id
		from
			foo
		where
			name = 'test3'	
		`,
	).Scan(&filler)

	if err == nil {
		t.Errorf("should have error\n")
	} else if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("should no rows error; got %s\n", err.Error())
	}

	err = db.QueryRow(
		`
		select
			foo.id
		from
			foo
		where
			name = 'test4'	
		`,
	).Scan(&filler)

	if err == nil {
		t.Errorf("should have error\n")
	} else if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("should no rows error; got %s\n", err.Error())
	}
}

// func TestRollbackOnFailure(t *testing.T) {
// 	var err error
// 	var mApp *CDBM

// 	utilSettings, err := cdbmutil.GetCDBMUtilSettings("")

// 	if err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	dbName := cdbmutil.GetRandomString(10)
// 	stdErr := &bytes.Buffer{}
// 	createCmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(utilSettings.DBAction.CreateDB, dbName))
// 	createCmd.Stderr = stdErr

// 	if err = createCmd.Run(); err != nil {
// 		t.Fatalf(stdErr.String())
// 	}

// 	dropCmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(utilSettings.DBAction.DropDB, dbName))
// 	defer dropCmd.Start()

// 	protocolCfg := DefaultProtocolMap[DBProtocol(utilSettings.BaseDatabaseSettings.DatabaseProtocol)]
// 	dbSettings := utilSettings.BaseDatabaseSettings.Settings
// 	dbSettings.DBName = dbName
// 	db, err := webutil.NewDB(dbSettings, protocolCfg.DatabaseType)

// 	if err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	rootDir := "/tmp/migrate-tool/"
// 	migrationsDir := rootDir + "migrations/"

// 	testGetMigrationFunc := func(migDir string, db *sql.DB, protocolCfg DBProtocolConfig) (*migrate.Migrate, error) {
// 		return &migrate.Migrate{}, nil
// 	}
// 	// testFileMigrationFunc := func(mig *migrate.Migrate, version int, mt MigrationType) error {
// 	// 	return fmt.Errorf("file migration error")
// 	// }

// 	if _, err = db.Exec(
// 		`
// 		create table schema_migrations(
// 			version int not null primary key,
// 			dirty boolean not null default false,
// 			dirty_state text
// 		);
// 		`,
// 	); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = db.Exec(
// 		`
// 		insert into schema_migrations(version, dirty)
// 		values(1, false)
// 		`,
// 	); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if err = os.RemoveAll(migrationsDir); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	var file1Up, file1Down, file2Up, file2Down *os.File

// 	if file1Up, err = os.Create(migrationsDir + "000001_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = file1Up.Write([]byte(
// 		`
// 		create table foo(
// 			id int not null primary key,
// 			name string not null
// 		);
// 		`,
// 	)); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if file1Down, err = os.Create(migrationsDir + "000001_update.down.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = file1Down.Write([]byte("drop table foo;")); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if file2Up, err = os.Create(migrationsDir + "000003_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = file2Up.Write([]byte(
// 		`
// 		create table bar(
// 			id int not null primary key,
// 			name string not null
// 		);
// 		`,
// 	)); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if file2Down, err = os.Create(migrationsDir + "000003_update.down.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = file2Down.Write([]byte("drop table bar;")); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	// Testing rollback on failure for both file and custom migrations
// 	mApp = &CDBM{
// 		DB:            db,
// 		DBProtocolCfg: protocolCfg,
// 		//CurrentDBSettings: dbSettings,
// 		MigrateFlags: MigrateFlagsConfig{
// 			TargetVersion:     -1,
// 			MigrationsDir:     migrationsDir,
// 			RollbackOnFailure: true,
// 		},
// 	}

// 	//counter := 0

// 	if err = mApp.Migrate(
// 		testGetMigrationFunc,
// 		func(mig *migrate.Migrate, version int, mt MigrationType) error {
// 			switch mt {
// 			case MigrateTypeUp:
// 				return fmt.Errorf("file migration up error")
// 			case MigrateTypeDown:
// 				return nil
// 			default:
// 				return fmt.Errorf("invalid migration type")
// 			}
// 		},
// 		map[int]CustomMigration{
// 			2: {
// 				Up: func(db webutil.DBInterface) error {
// 					return nil
// 				},
// 				Down: func(db webutil.DBInterface) error {
// 					return fmt.Errorf("custom migration down error")
// 				},
// 			},
// 		},
// 	); err == nil {
// 		t.Errorf("should have error\n")
// 	} else {
// 		if errors.Cause(err).Error() != "file migration up error" {
// 			t.Errorf("should have file command migration error; got %s\n", err.Error())
// 		}

// 		t.Error(err.Error())

// 		err = errors.Unwrap(err)

// 		t.Errorf(err.Error())
// 	}
// }

// func TestMigrateTool(t *testing.T) {
// 	var err error

// 	utilSettings, err := cdbmutil.GetCDBMUtilSettings("")

// 	if err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	utilSettings.DBSetup.FileServerSetup = nil
// 	utilSettings.DBSetup.BaseSchemaFile = ""

// 	db, dbName, err := cdbmutil.GetNewDatabase(utilSettings, DefaultExecCmd, cdbmutil.DefaultGetDB)

// 	if err != nil {
// 		t.Fatalf("%+v\n", err.Error())
// 	}

// 	fmt.Printf(dbName)

// 	rootDir := "/tmp/migrate-tool/"
// 	migrationsDir := rootDir + "migrations/"

// 	//defer os.RemoveAll(migrationsDir)

// 	if err = os.RemoveAll(migrationsDir); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	var file1Up, file1Down, file2Up, file2Down *os.File

// 	if file1Up, err = os.Create(migrationsDir + "000001_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	fooTable :=
// 		`
// 	create table foo(
// 		id int not null primary key,
// 		name string not null
// 	);
// 	`

// 	if _, err = db.Exec(fooTable); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = file1Up.Write([]byte(fooTable)); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if file1Down, err = os.Create(migrationsDir + "000001_update.down.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = file1Down.Write([]byte("drop table foo;")); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if file2Up, err = os.Create(migrationsDir + "000002_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = file2Up.Write([]byte(
// 		`
// 		create table bar(
// 			id int not null primary key,
// 			name string not null
// 		);
// 		`,
// 	)); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if file2Down, err = os.Create(migrationsDir + "000002_update.down.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = file2Down.Write([]byte("drop table bar;")); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = db.Exec(
// 		`
// 		create table public.schema_migrations(
// 			version int not null primary key,
// 			dirty boolean not null default false
// 		);
// 		`,
// 	); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = db.Exec(
// 		`
// 		insert into schema_migrations(version, dirty)
// 		values(1, false);
// 		`,
// 	); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	driver, err := GetDatabaseDriver(
// 		db.DB,
// 		DBProtocol(utilSettings.BaseDatabaseSettings.DatabaseProtocol),
// 		nil,
// 	)

// 	if err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	mig, err := migrate.NewWithDatabaseInstance(
// 		"file://"+migrationsDir,
// 		DefaultProtocolMap[DBProtocol(utilSettings.BaseDatabaseSettings.DatabaseProtocol)].DatabaseType,
// 		driver,
// 	)

// 	if err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if err = mig.Steps(-1); err != nil {
// 		t.Fatalf(err.Error())
// 	}
// }

// func TestBlah(t *testing.T) {
// 	var err error
// 	var mApp *CDBM

// 	settings, err := cdbmutil.GetCDBMUtilSettings("")

// 	if err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	dbProtocolCfg := DefaultProtocolMap[DBProtocol(settings.BaseDatabaseSettings.DatabaseProtocol)]
// 	dbName := cdbmutil.GetRandomString(10)
// 	stdErr := &bytes.Buffer{}
// 	createCmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(settings.DBAction.CreateDB, dbName))
// 	createCmd.Stderr = stdErr

// 	if err = createCmd.Run(); err != nil {
// 		t.Fatalf(stdErr.String())
// 	}

// 	dropCmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(settings.DBAction.DropDB, dbName))
// 	defer dropCmd.Start()

// 	dbSettings := settings.BaseDatabaseSettings.Settings
// 	dbSettings.DBName = dbName

// 	db, err := webutil.NewDB(dbSettings, dbProtocolCfg.DatabaseType)

// 	if err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	rootDir := "/tmp/migrate-tool/"
// 	migrationsDir := rootDir + "migrations/"

// 	testGetMigrationFunc := func(migDir string, db *sql.DB, protocolCfg DBProtocolConfig) (*migrate.Migrate, error) {
// 		return &migrate.Migrate{}, nil
// 	}

// 	if err = os.RemoveAll(migrationsDir); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = os.Create(migrationsDir + "000001_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	// Testing file migration error
// 	mApp = &CDBM{
// 		DB:            db,
// 		DBProtocolCfg: dbProtocolCfg,
// 		MigrateFlags: MigrateFlagsConfig{
// 			TargetVersion: 1,
// 			MigrationsDir: migrationsDir,
// 		},
// 	}

// 	if err = mApp.Migrate(
// 		testGetMigrationFunc,
// 		func(mig *migrate.Migrate, version int, mt MigrationType) error {
// 			return fmt.Errorf("file migration error")
// 		},
// 		map[int]CustomMigration{},
// 	); err == nil {
// 		t.Errorf("should have error\n")
// 	} else {
// 		if errors.Cause(err).Error() != "failed on file up migration for version: '1'" {
// 			t.Errorf("should have file migration error; got %s\n", err.Error())
// 		}
// 	}
// }

// func TestFileMigrationError(t *testing.T) {
// 	var err error
// 	var mApp *CDBM

// 	settings, err := cdbmutil.GetCDBMUtilSettings("")

// 	if err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	dbProtocolCfg := DefaultProtocolMap[DBProtocol(settings.BaseDatabaseSettings.DatabaseProtocol)]
// 	dbName := cdbmutil.GetRandomString(10)
// 	stdErr := &bytes.Buffer{}
// 	createCmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(settings.DBAction.CreateDB, dbName))
// 	createCmd.Stderr = stdErr

// 	if err = createCmd.Run(); err != nil {
// 		t.Fatalf(stdErr.String())
// 	}

// 	dropCmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(settings.DBAction.DropDB, dbName))
// 	defer dropCmd.Start()

// 	dbSettings := settings.BaseDatabaseSettings.Settings
// 	dbSettings.DBName = dbName

// 	db, err := webutil.NewDB(dbSettings, dbProtocolCfg.DatabaseType)

// 	if err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	rootDir := "/tmp/migrate-tool/"
// 	migrationsDir := rootDir + "migrations/"

// 	testGetMigrationFunc := func(migDir string, db *sql.DB, protocolCfg DBProtocolConfig) (*migrate.Migrate, error) {
// 		return &migrate.Migrate{}, nil
// 	}

// 	if err = os.RemoveAll(migrationsDir); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = os.Create(migrationsDir + "000001_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	// Testing file migration error
// 	mApp = &CDBM{
// 		DB:            db,
// 		DBProtocolCfg: dbProtocolCfg,
// 		MigrateFlags: MigrateFlagsConfig{
// 			TargetVersion: -1,
// 			MigrationsDir: migrationsDir,
// 		},
// 	}

// 	if err = mApp.Migrate(
// 		testGetMigrationFunc,
// 		func(mig *migrate.Migrate, version int, mt MigrationType) error {
// 			return fmt.Errorf("file migration error")
// 		},
// 		map[int]CustomMigration{},
// 	); err == nil {
// 		t.Errorf("should have error\n")
// 	} else {
// 		if errors.Cause(err).Error() != "failed on file up migration for version: '1'" {
// 			t.Errorf("should have file migration error; got %s\n", err.Error())
// 		}
// 	}
// }

// func TestHello(t *testing.T) {
// 	var err error

// 	settings, err := cdbmutil.GetCDBMUtilSettings("")

// 	if err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	dbProtocolCfg := DefaultProtocolMap[DBProtocol(settings.BaseDatabaseSettings.DatabaseProtocol)]
// 	dbName := cdbmutil.GetRandomString(10)
// 	stdErr := &bytes.Buffer{}
// 	createCmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(settings.DBAction.CreateDB, dbName))
// 	createCmd.Stderr = stdErr

// 	if err = createCmd.Run(); err != nil {
// 		t.Fatalf(stdErr.String())
// 	}

// 	// dropCmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(settings.DBAction.DropDB, dbName))
// 	// defer dropCmd.Start()

// 	dbSettings := settings.BaseDatabaseSettings.Settings
// 	dbSettings.DBName = dbName

// 	db, err := webutil.NewDB(dbSettings, dbProtocolCfg.DatabaseType)

// 	if err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	rootDir := "/tmp/migrate-hello/"
// 	migrationsDir := rootDir + "migrations/"

// 	if err = os.RemoveAll(migrationsDir); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = db.Exec(
// 		`
// 		create table foo(
// 			id int primary key,
// 			name text
// 		);
// 		`,
// 	); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	mig, err := DefaultGetMigrationFunc("file://"+migrationsDir, db.DB, dbProtocolCfg)

// 	if err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if err = mig.Drop(); err != nil {
// 		t.Fatalf(err.Error())
// 	}
// }
