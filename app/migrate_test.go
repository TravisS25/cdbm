package app

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/TravisS25/cdbm/cdbmutil"
	"github.com/TravisS25/webutil/webutil"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// func getTestMigrate(t *testing.T, importKey string) (*cdbm.CDBM, func()) {
// 	utilSettings, err := cdbmutil.GetCDBMUtilSettings("")

// 	if err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	utilSettings.DBAction.Import.ImportKey = importKey

// 	db, dbName, err := cdbmutil.GetNewDatabase(
// 		utilSettings,
// 		cdbm.DefaultExecCmd,
// 		cdbmutil.DefaultGetDB,
// 	)

// 	fmt.Printf("%s\n", dbName)

// 	if err != nil {
// 		t.Fatalf("%+v\n", err.Error())
// 	}

// 	dropDB := func() {
// 		dropCmd := exec.Command(
// 			"/bin/sh",
// 			"-c",
// 			fmt.Sprintf(utilSettings.DBAction.DropDB, dbName),
// 		)
// 		dropCmd.Start()
// 	}

// 	app, err := cdbm.NewCDBM(cdbm.RootFlagsConfig{}, nil)

// 	if err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	app.DB = db

// 	return app, dropDB
// }

func getSchemaInsert(t *testing.T, dbProtocol string) string {
	schemaInsert, _, err := webutil.InQueryRebind(
		DefaultProtocolMap[DBProtocol(dbProtocol)].SQLBindVar,
		`
		insert into schema_migrations(version, dirty, dirty_state, is_custom_migration) 
		values(?, ?, ?, ?);
		`,
		0,
		true,
		MigrateTypeUp,
		true,
	)

	if err != nil {
		t.Fatalf(err.Error())
	}

	return schemaInsert
}

func getSchemaUpdate(t *testing.T, dbProtocol string) string {
	schemaUpdate, _, err := webutil.InQueryRebind(
		DefaultProtocolMap[DBProtocol(dbProtocol)].SQLBindVar,
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
		MigrateTypeUp,
		true,
	)

	if err != nil {
		t.Fatalf(err.Error())
	}

	return schemaUpdate
}

func createMigrationTable(t *testing.T, db webutil.Executer) {
	var err error

	if _, err = db.Exec(
		`
		CREATE TABLE public.schema_migrations (
			version INT8 NOT NULL primary key,
			dirty boolean not null,
			dirty_state text,
			is_custom_migration boolean not null default false
		);
		`,
	); err != nil {
		t.Fatalf(err.Error())
	}
}

func deleteFromSchemaMigration(t *testing.T, db webutil.Executer) {
	_, err := db.Exec("delete from schema_migrations;")

	if err != nil {
		t.Fatalf(err.Error())
	}
}

func getSchemaMigration(t *testing.T, db webutil.Querier) schemaMigration {
	var err error
	var sm schemaMigration

	if err = db.QueryRowx(
		`
		select
			version,
			dirty,
			dirty_state,
			is_custom_migration
		from
			schema_migrations
		`,
	).Scan(&sm.StartingVersion, &sm.Dirty, &sm.DirtyState, &sm.IsCustomMigration); err != nil {
		t.Fatalf(err.Error())
	}

	return sm
}

func TestCheckMigrationsProtocol(t *testing.T) {
	var err error
	var mApp *CDBM

	// --------------------------------------------------------------------------

	mApp = &CDBM{}

	if err = mApp.checkMigrationsProtocol(); err == nil {
		t.Errorf("should have error")
	} else if err.Error() != "--migrations-protocol required" {
		t.Errorf("should have migrations protocol required error; got %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	mApp = &CDBM{
		MigrateFlags: MigrateFlagsConfig{
			MigrationsProtocol: MigrationsProtocol("foo"),
		},
	}

	if err = mApp.checkMigrationsProtocol(); err == nil {
		t.Errorf("should have error")
	} else if !strings.Contains(err.Error(), "invalid --migrations-protocol;  Valid --migrations-protocol are") {
		t.Errorf("should have migrations protocol error; got %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	mApp = &CDBM{
		MigrateFlags: MigrateFlagsConfig{
			MigrationsProtocol: FileProtocol,
		},
	}

	if err = mApp.checkMigrationsProtocol(); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}
}

func TestApplyQueries(t *testing.T) {
	settings, err := cdbmutil.GetCDBMUtilSettings("")

	if err != nil {
		t.Fatalf(err.Error())
	}

	mApp := &CDBM{
		DBProtocolCfg: DefaultProtocolMap[DBProtocol(settings.BaseDatabaseSettings.DatabaseProtocol)],
	}

	if err = mApp.applyQueries(); err != nil {
		t.Errorf("should not have error; gott %s\n", err.Error())
	}
}

func TestCreateLogsDirectory(t *testing.T) {
	md := "/tmp/cdbm-log/"

	defer os.RemoveAll(md)

	c := &CDBM{
		MigrateFlags: MigrateFlagsConfig{
			MigrationsDir: md,
		},
	}

	_, err := c.createLogsDirectory()

	if err != nil {
		t.Fatalf(err.Error())
	}
}

func TestVerifyFilesAndMigrations(t *testing.T) {
	var err error
	var mApp *CDBM

	migrationsDir := "/tmp/verify-files/"

	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
		t.Fatalf(err.Error())
	}
	defer os.RemoveAll(migrationsDir)

	mApp = &CDBM{
		MigrateFlags: MigrateFlagsConfig{
			MigrationsDir: migrationsDir,
		},
	}

	if _, err = mApp.verifyFilesAndMigrations(); err == nil {
		t.Errorf("should have error")
	} else if err.Error() != "no sql files or custom migrations found" {
		t.Errorf("should have no file or custom migrations err; got %s\n", err.Error())
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
		MigrateFlags: MigrateFlagsConfig{
			MigrationsDir: migrationsDir,
		},
	}

	if _, err = mApp.verifyFilesAndMigrations(); err == nil {
		t.Errorf("should have error")
	} else if err.Error() != "migration file version less than min version allowed (1)" {
		t.Errorf("should have version error; got %s\n", err.Error())
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
		MigrateFlags: MigrateFlagsConfig{
			MigrationsDir: migrationsDir,
		},
	}

	if _, err = mApp.verifyFilesAndMigrations(); err == nil {
		t.Errorf("should have error")
	} else if err == ErrInvalidFileName {
		t.Errorf("should have version error; got %s\n", err.Error())
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
		MigrateFlags: MigrateFlagsConfig{
			MigrationsDir: migrationsDir,
		},
	}

	if _, err = mApp.verifyFilesAndMigrations(); err == nil {
		t.Errorf("should have error")
	} else if err == ErrInvalidFileName {
		t.Errorf("should have version error; got %s\n", err.Error())
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
		MigrateFlags: MigrateFlagsConfig{
			MigrationsDir: migrationsDir,
		},
	}

	if _, err = mApp.verifyFilesAndMigrations(); err == nil {
		t.Errorf("should have error")
	} else if err == ErrInvalidFileName {
		t.Errorf("should have version error; got %s\n", err.Error())
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
		MigrateFlags: MigrateFlagsConfig{
			MigrationsDir: migrationsDir,
		},
	}

	if _, err = mApp.verifyFilesAndMigrations(); err == nil {
		t.Errorf("should have error")
	} else if err == ErrInvalidFileName {
		t.Errorf("should have version error; got %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	if err = os.RemoveAll(migrationsDir); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000001_update.up.invalid"); err != nil {
		t.Fatalf(err.Error())
	}

	// Should error out due to invalid file name
	mApp = &CDBM{
		MigrateFlags: MigrateFlagsConfig{
			MigrationsDir: migrationsDir,
		},
	}

	if _, err = mApp.verifyFilesAndMigrations(); err == nil {
		t.Errorf("should have error")
	} else if err == ErrInvalidFileName {
		t.Errorf("should have version error; got %s\n", err.Error())
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

	// Should error out due to duplicate version names between sql files
	// and custom migrations
	mApp = &CDBM{
		MigrateFlags: MigrateFlagsConfig{
			MigrationsDir: migrationsDir,
		},
		migrateCfg: migrateConfig{
			CustomMigrations: map[int]CustomMigration{
				1: {
					Up: func(db webutil.DBInterface) error {
						return nil
					},
				},
			},
		},
	}

	if _, err = mApp.verifyFilesAndMigrations(); err == nil {
		t.Errorf("should have error")
	} else if !strings.Contains(err.Error(), "following versions are duplicated between files and custom migrations") {
		t.Errorf("should have duplicate version error; got %s\n", err.Error())
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

	// Should be valid
	mApp = &CDBM{
		MigrateFlags: MigrateFlagsConfig{
			MigrationsDir: migrationsDir,
		},
		migrateCfg: migrateConfig{
			CustomMigrations: map[int]CustomMigration{},
		},
	}

	if _, err = mApp.verifyFilesAndMigrations(); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}
}

func TestApplyTargetVersion(t *testing.T) {
	var err error
	var mApp *CDBM

	mApp = &CDBM{
		MigrateFlags: MigrateFlagsConfig{
			TargetVersion: 2,
		},
	}

	if err = mApp.applyTargetVersion(
		[]migrationApplyConfig{
			{
				Version: 1,
			},
		},
	); err == nil {
		t.Errorf("should have error")
	} else if err.Error() != "--target-version does not exist" {
		t.Errorf("shoould have target error; got %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	mApp = &CDBM{
		MigrateFlags: MigrateFlagsConfig{
			TargetVersion: 1,
		},
	}

	if err = mApp.applyTargetVersion(
		[]migrationApplyConfig{
			{
				Version: 1,
			},
		},
	); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}

	if mApp.migrateCfg.TargetVersion != 1 {
		t.Errorf("target version should equal 1; got %d\n", mApp.migrateCfg.TargetVersion)
	}

	// --------------------------------------------------------------------------

	mApp = &CDBM{
		MigrateFlags: MigrateFlagsConfig{
			TargetVersion: -1,
		},
	}

	if err = mApp.applyTargetVersion(
		[]migrationApplyConfig{
			{
				Version: 1,
			},
		},
	); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}

	if mApp.migrateCfg.TargetVersion != 1 {
		t.Errorf("target version should equal 1; got %d\n", mApp.migrateCfg.TargetVersion)
	}
}

func TestResetDirtyFlag(t *testing.T) {
	var err error
	var mApp *CDBM
	var sm schemaMigration

	settings, err := cdbmutil.GetCDBMUtilSettings("")

	if err != nil {
		t.Fatalf(err.Error())
	}

	settings.DBSetup.FileServerSetup = nil
	settings.DBSetup.BaseSchemaFile = ""

	db, dbName, err := cdbmutil.GetNewDatabase(
		settings,
		DefaultExecCmd,
		cdbmutil.DefaultGetDB,
	)

	if err != nil {
		t.Fatalf(err.Error())
	}

	dropCmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(settings.DBAction.DropDB, dbName))
	defer dropCmd.Start()

	insertQuery := getSchemaInsert(t, settings.BaseDatabaseSettings.DatabaseProtocol)
	updateQuery := getSchemaUpdate(t, settings.BaseDatabaseSettings.DatabaseProtocol)
	createMigrationTable(t, db)

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	if _, err = db.Exec(insertQuery, 2, true, "Up", true); err != nil {
		t.Fatalf(err.Error())
	}

	up := "up"

	mApp = &CDBM{
		DB: db,
		MigrateFlags: MigrateFlagsConfig{
			ResetDirtyFlag: true,
		},
		migrateCfg: migrateConfig{
			UpdateQuery: updateQuery,
			SchemaMigration: schemaMigration{
				StartingVersion:   2,
				Dirty:             true,
				DirtyState:        &up,
				IsCustomMigration: true,
			},
		},
	}

	if err = mApp.resetDirtyFlag(); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}

	sm = getSchemaMigration(t, db)

	if sm.StartingVersion != 2 {
		t.Errorf("should be version 2; %d\n", sm.StartingVersion)
	}
	if sm.Dirty {
		t.Errorf("should not be dirty")
	}
	if sm.DirtyState == nil {
		t.Errorf("dirty state should not be nil")
	} else if *sm.DirtyState != "" {
		t.Errorf("dirty state should be empty; got %s\n", *sm.DirtyState)
	}
	if !sm.IsCustomMigration {
		t.Errorf("should be custom migration")
	}

	// --------------------------------------------------------------------------

	mApp = &CDBM{
		MigrateFlags: MigrateFlagsConfig{
			ResetDirtyFlag: false,
		},
		migrateCfg: migrateConfig{
			SchemaMigration: schemaMigration{
				Dirty: true,
			},
		},
	}

	if err = mApp.resetDirtyFlag(); err == nil {
		t.Errorf("should have error")
	} else if mApp.resetDirtyFlag().Error() != "must set --reset-dirty-flag to reset migrations dirty flag.  Use 'cdbm status' to see current status of migration" {
		t.Errorf("should have reset dirty flag error error; got %s\n", err.Error())
	}
}

func TestMigrationRollbackFail(t *testing.T) {
	var err error
	var mApp *CDBM
	var version int64

	settings, err := cdbmutil.GetCDBMUtilSettings("")

	if err != nil {
		t.Fatalf(err.Error())
	}

	settings.DBSetup.FileServerSetup = nil
	settings.DBSetup.BaseSchemaFile = ""

	db, dbName, err := cdbmutil.GetNewDatabase(
		settings,
		DefaultExecCmd,
		cdbmutil.DefaultGetDB,
	)

	if err != nil {
		t.Fatalf(err.Error())
	}

	dropCmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(settings.DBAction.DropDB, dbName))
	defer dropCmd.Start()

	insertQuery := getSchemaInsert(t, settings.BaseDatabaseSettings.DatabaseProtocol)
	updateQuery := getSchemaUpdate(t, settings.BaseDatabaseSettings.DatabaseProtocol)

	//migrationErr := fmt.Errorf("migration error")
	createMigrationTable(t, db)

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	if _, err = db.Exec(insertQuery, 3, false, "", false); err != nil {
		t.Fatalf(err.Error())
	}

	// Validating custom down migration error
	mApp = &CDBM{
		DB: db,
		MigrateFlags: MigrateFlagsConfig{
			RollbackOnFailure: true,
		},
		migrateCfg: migrateConfig{
			InsertQuery: insertQuery,
			UpdateQuery: updateQuery,
			MigrateType: MigrateTypeUp,
			LogWriter: func(err error) {
				if err == nil {
					t.Errorf("should have error for logger")
				} else if err.Error() != "custom down migration error" {
					t.Errorf("should have custom down migration error; got %s\n", err.Error())
				}
			},
			CustomMigrations: map[int]CustomMigration{
				2: {
					Down: func(db webutil.DBInterface) error {
						return fmt.Errorf("custom down migration error")
					},
				},
			},
			SchemaMigration: schemaMigration{
				SchemaCfg: schemaConfig{
					HasEntry: true,
				},
			},
		},
	}

	if err = mApp.migrationRollbackFail(2); err == nil {
		t.Errorf("should have error")
	} else if err.Error() != "failed on custom rollback migration for version: '2'" {
		t.Errorf("should contain custom rollback migration error; got %s\n", err.Error())
	}

	if err = db.QueryRowx("select version from schema_migrations").Scan(&version); err != nil {
		t.Fatalf(err.Error())
	}

	if version != 2 {
		t.Errorf("should have version 2; got %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	if _, err = db.Exec(insertQuery, 3, false, "", false); err != nil {
		t.Fatalf(err.Error())
	}

	// Validating file down migration error
	mApp = &CDBM{
		DB: db,
		MigrateFlags: MigrateFlagsConfig{
			RollbackOnFailure: true,
		},
		migrateCfg: migrateConfig{
			InsertQuery: insertQuery,
			UpdateQuery: updateQuery,
			MigrateType: MigrateTypeUp,
			LogWriter: func(err error) {
				if err == nil {
					t.Errorf("should have error for logger")
				} else if err.Error() != "file down migration error" {
					t.Errorf("should have file down migration error; got %s\n", err.Error())
				}
			},
			FileMigration: func(mig *migrate.Migrate, version int, mt MigrationsType) error {
				return fmt.Errorf("file down migration error")
			},
			SchemaMigration: schemaMigration{
				SchemaCfg: schemaConfig{
					HasEntry: true,
				},
			},
		},
	}

	if err = mApp.migrationRollbackFail(2); err == nil {
		t.Errorf("should have error")
	} else if err.Error() != "failed on file rollback migration for version: '2'" {
		t.Errorf("should contain file rollback migration error; got %s\n", err.Error())
	}

	if err = db.QueryRowx("select version from schema_migrations").Scan(&version); err != nil {
		t.Fatalf(err.Error())
	}

	if version != 2 {
		t.Errorf("should have version 2; got %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	// Should be valid
	mApp = &CDBM{
		DB: db,
		MigrateFlags: MigrateFlagsConfig{
			RollbackOnFailure: true,
		},
		migrateCfg: migrateConfig{
			InsertQuery: insertQuery,
			UpdateQuery: updateQuery,
			MigrateType: MigrateTypeUp,
			LogWriter: func(err error) {
				if err == nil {
					t.Errorf("should have error for logger")
				} else if err.Error() != "file down migration error" {
					t.Errorf("should have file down migration error; got %s\n", err.Error())
				}
			},
			FileMigration: func(mig *migrate.Migrate, version int, mt MigrationsType) error {
				return nil
			},
			SchemaMigration: schemaMigration{
				SchemaCfg: schemaConfig{
					HasEntry: true,
				},
			},
		},
	}

	if err = mApp.migrationRollbackFail(2); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}
}

func TestApplyCustomMigration(t *testing.T) {
	var err error
	var mApp *CDBM

	settings, err := cdbmutil.GetCDBMUtilSettings("")

	if err != nil {
		t.Fatalf(err.Error())
	}

	settings.DBSetup.FileServerSetup = nil
	settings.DBSetup.BaseSchemaFile = ""

	db, dbName, err := cdbmutil.GetNewDatabase(
		settings,
		DefaultExecCmd,
		cdbmutil.DefaultGetDB,
	)

	if err != nil {
		t.Fatalf(err.Error())
	}

	dropCmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(settings.DBAction.DropDB, dbName))
	defer dropCmd.Start()

	createMigrationTable(t, db)
	insertQuery := getSchemaInsert(t, settings.BaseDatabaseSettings.DatabaseProtocol)
	updateQuery := getSchemaUpdate(t, settings.BaseDatabaseSettings.DatabaseProtocol)

	var sm schemaMigration
	var cm CustomMigration

	// --------------------------------------------------------------------------

	// Validating successful custom up migration
	mApp = &CDBM{
		DB: db,
		migrateCfg: migrateConfig{
			MigrateType: MigrateTypeUp,
			InsertQuery: insertQuery,
			LogWriter:   func(err error) {},
		},
	}

	if err = mApp.applyCustomMigration(
		2,
		func(db webutil.DBInterface) error {
			return nil
		},
	); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}

	if !mApp.migrateCfg.SchemaMigration.SchemaCfg.HasEntry {
		t.Errorf("entry should have changed to true")
	}

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	mApp = &CDBM{
		DB: db,
		migrateCfg: migrateConfig{
			MigrateType: MigrateTypeDown,
			InsertQuery: insertQuery,
			LogWriter:   func(err error) {},
		},
	}

	// Validating successful custom down migration
	if err = mApp.applyCustomMigration(
		2,
		func(db webutil.DBInterface) error {
			return nil
		},
	); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}

	if !mApp.migrateCfg.SchemaMigration.SchemaCfg.HasEntry {
		t.Errorf("entry should have changed to true")
	}

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	if _, err = db.Exec(insertQuery, 1, false, "", false); err != nil {
		t.Fatalf(err.Error())
	}

	// Validating custom up migration with schema entry already in db
	mApp = &CDBM{
		DB: db,
		migrateCfg: migrateConfig{
			MigrateType: MigrateTypeUp,
			UpdateQuery: updateQuery,
			LogWriter:   func(err error) {},
			SchemaMigration: schemaMigration{
				SchemaCfg: schemaConfig{
					HasEntry: true,
				},
			},
		},
	}

	if err = mApp.applyCustomMigration(
		2,
		func(db webutil.DBInterface) error {
			return nil
		},
	); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	// Validating custom up migration error
	mApp = &CDBM{
		DB: db,
		migrateCfg: migrateConfig{
			InsertQuery: insertQuery,
			UpdateQuery: updateQuery,
			MigrateType: MigrateTypeUp,
			LogWriter:   func(err error) {},
		},
	}

	if err = mApp.applyCustomMigration(
		2,
		func(db webutil.DBInterface) error {
			return fmt.Errorf("custom migration error")
		},
	); err == nil {
		t.Errorf("should have error")
	} else if err.Error() != "failed on custom up migration for version: '2'" {
		t.Errorf("should have custom up migration error; got %s\n", err.Error())
	}

	sm = getSchemaMigration(t, db)

	if sm.StartingVersion != 2 {
		t.Errorf("version should be 2; got %d\n", sm.StartingVersion)
	}
	if !sm.Dirty {
		t.Errorf("should be dirty")
	}
	if sm.DirtyState == nil {
		t.Errorf("should have dirty state")
	} else if *sm.DirtyState != string(MigrateTypeUp) {
		t.Errorf("should have dirty state 'Up'; got %s\n", *sm.DirtyState)
	}

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	cm = CustomMigration{
		Up: func(db webutil.DBInterface) error {
			return fmt.Errorf("custom migration error")
		},
		Down: func(db webutil.DBInterface) error {
			return nil
		},
	}

	// Validating custom up migration error with successful rollback
	mApp = &CDBM{
		DB: db,
		MigrateFlags: MigrateFlagsConfig{
			RollbackOnFailure: true,
		},
		migrateCfg: migrateConfig{
			InsertQuery: insertQuery,
			UpdateQuery: updateQuery,
			MigrateType: MigrateTypeUp,
			LogWriter:   func(err error) {},
			CustomMigrations: map[int]CustomMigration{
				2: cm,
			},
			SchemaMigration: schemaMigration{
				StartingVersion: 1,
			},
		},
	}

	if err = mApp.applyCustomMigration(2, cm.Up); err == nil {
		t.Errorf("should have error")
	} else {
		if !strings.Contains(err.Error(), "successfully rolled back") {
			t.Errorf("should have successful rollback; got %s\n", err.Error())
		}
	}

	sm = getSchemaMigration(t, db)

	if sm.StartingVersion != 1 {
		t.Errorf("version should be 1; got %d\n", sm.StartingVersion)
	}
	if sm.Dirty {
		t.Errorf("should not be dirty")
	}
	if sm.DirtyState == nil {
		t.Errorf("should have dirty state")
	} else if *sm.DirtyState != "" {
		t.Errorf("should have no dirty state; got %s\n", *sm.DirtyState)
	}

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	cm = CustomMigration{
		Up: func(db webutil.DBInterface) error {
			return fmt.Errorf("custom migration error")
		},
		Down: func(db webutil.DBInterface) error {
			return fmt.Errorf("custom migration error")
		},
	}

	// Validating custom up migration error and failed on rollback
	mApp = &CDBM{
		DB: db,
		MigrateFlags: MigrateFlagsConfig{
			RollbackOnFailure: true,
		},
		migrateCfg: migrateConfig{
			InsertQuery: insertQuery,
			UpdateQuery: updateQuery,
			MigrateType: MigrateTypeUp,
			LogWriter:   func(err error) {},
			CustomMigrations: map[int]CustomMigration{
				2: cm,
			},
			// FileMigration: func(mig *migrate.Migrate, version int, mt MigrationsType) error {
			// 	return nil
			// },
			SchemaMigration: schemaMigration{
				StartingVersion: 1,
			},
		},
	}

	if err = mApp.applyCustomMigration(2, cm.Up); err == nil {
		t.Errorf("should have error")
	} else {
		if !strings.Contains(err.Error(), "failed on custom rollback migration for version: '2'") {
			t.Errorf("should have failed rollback; got %s\n", err.Error())
		}
	}

	sm = getSchemaMigration(t, db)

	if sm.StartingVersion != 2 {
		t.Errorf("version should be 2; got %d\n", sm.StartingVersion)
	}
	if !sm.Dirty {
		t.Errorf("should be dirty")
	}
	if sm.DirtyState == nil {
		t.Errorf("should have dirty state")
	} else if *sm.DirtyState != string(MigrateTypeDown) {
		t.Errorf("should have dirty state 'Down'; got %s\n", *sm.DirtyState)
	}

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	// Validating custom down migration error
	mApp = &CDBM{
		DB: db,
		migrateCfg: migrateConfig{
			InsertQuery: insertQuery,
			UpdateQuery: updateQuery,
			MigrateType: MigrateTypeDown,
			LogWriter:   func(err error) {},
			SchemaMigration: schemaMigration{
				SchemaCfg: schemaConfig{
					HasEntry: true,
				},
			},
		},
	}

	if err = mApp.applyCustomMigration(
		2,
		func(db webutil.DBInterface) error {
			return fmt.Errorf("custom migration error")
		},
	); err == nil {
		t.Errorf("should have error")
	} else if err.Error() != "failed on custom down migration for version: '2'" {
		t.Errorf("should have custom down migration error; got %s\n", err.Error())
	}
}

func TestApplyFileMigration(t *testing.T) {
	var err error
	var mApp *CDBM

	settings, err := cdbmutil.GetCDBMUtilSettings("")

	if err != nil {
		t.Fatalf(err.Error())
	}

	settings.DBSetup.FileServerSetup = nil
	settings.DBSetup.BaseSchemaFile = ""

	db, dbName, err := cdbmutil.GetNewDatabase(
		settings,
		DefaultExecCmd,
		cdbmutil.DefaultGetDB,
	)

	if err != nil {
		t.Fatalf(err.Error())
	}

	dropCmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(settings.DBAction.DropDB, dbName))
	defer dropCmd.Start()

	createMigrationTable(t, db)
	insertQuery := getSchemaInsert(t, settings.BaseDatabaseSettings.DatabaseProtocol)
	updateQuery := getSchemaUpdate(t, settings.BaseDatabaseSettings.DatabaseProtocol)

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	if _, err = db.Exec(insertQuery, 1, false, "", false); err != nil {
		t.Fatalf(err.Error())
	}

	// Validating file up migration
	mApp = &CDBM{
		DB: db,
		migrateCfg: migrateConfig{
			MigrateType: MigrateTypeUp,
			UpdateQuery: updateQuery,
			LogWriter:   func(err error) {},
			FileMigration: func(mig *migrate.Migrate, version int, mt MigrationsType) error {
				return nil
			},
		},
	}

	if err = mApp.applyFileMigration(2); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	// Validating file up migration error
	mApp = &CDBM{
		DB: db,
		migrateCfg: migrateConfig{
			MigrateType: MigrateTypeUp,
			UpdateQuery: updateQuery,
			LogWriter:   func(err error) {},
			SchemaMigration: schemaMigration{
				StartingVersion: 1,
			},
			FileMigration: func(mig *migrate.Migrate, version int, mt MigrationsType) error {
				return fmt.Errorf("file migration error")
			},
		},
	}

	if err = mApp.applyFileMigration(2); err == nil {
		t.Errorf("should have error")
	} else if err.Error() != "failed on file up migration for version: '2'" {
		t.Errorf("should have file up migration error; got %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	// Validating file up migration error with successful rollback
	mApp = &CDBM{
		DB: db,
		MigrateFlags: MigrateFlagsConfig{
			RollbackOnFailure: true,
		},
		migrateCfg: migrateConfig{
			MigrateType: MigrateTypeUp,
			UpdateQuery: updateQuery,
			LogWriter:   func(err error) {},
			FileMigration: func(mig *migrate.Migrate, version int, mt MigrationsType) error {
				if mt == MigrateTypeUp {
					return fmt.Errorf("file migration error")
				}

				return nil
			},
		},
	}

	if err = mApp.applyFileMigration(2); err == nil {
		t.Errorf("should have error")
	} else if !strings.Contains(err.Error(), "successfully rolled back") {
		t.Errorf("should have file up migration error; got %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	// Validating file up migration error and rollback error
	mApp = &CDBM{
		DB: db,
		MigrateFlags: MigrateFlagsConfig{
			RollbackOnFailure: true,
		},
		migrateCfg: migrateConfig{
			MigrateType: MigrateTypeUp,
			UpdateQuery: updateQuery,
			LogWriter:   func(err error) {},
			FileMigration: func(mig *migrate.Migrate, version int, mt MigrationsType) error {
				return fmt.Errorf("file migration error")
			},
		},
	}

	if err = mApp.applyFileMigration(2); err == nil {
		t.Errorf("should have error")
	} else if !strings.Contains(err.Error(), "failed on file rollback migration for version: '2'") {
		t.Errorf("should have file up migration error; got %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	// Validating file down migration error
	mApp = &CDBM{
		DB: db,
		migrateCfg: migrateConfig{
			MigrateType: MigrateTypeDown,
			UpdateQuery: updateQuery,
			LogWriter:   func(err error) {},
			FileMigration: func(mig *migrate.Migrate, version int, mt MigrationsType) error {
				return fmt.Errorf("file migration error")
			},
		},
	}

	if err = mApp.applyFileMigration(2); err == nil {
		t.Errorf("should have error")
	} else if err.Error() != "failed on file down migration for version: '2'" {
		t.Errorf("should have file down migration error; got %s\n", err.Error())
	}
}

func TestApplyMigrationConfig(t *testing.T) {
	var err error
	var mApp *CDBM

	settings, err := cdbmutil.GetCDBMUtilSettings("")

	if err != nil {
		t.Fatalf(err.Error())
	}

	settings.DBSetup.FileServerSetup = nil
	settings.DBSetup.BaseSchemaFile = ""

	db, dbName, err := cdbmutil.GetNewDatabase(
		settings,
		DefaultExecCmd,
		cdbmutil.DefaultGetDB,
	)

	if err != nil {
		t.Fatalf(err.Error())
	}

	dropCmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(settings.DBAction.DropDB, dbName))
	defer dropCmd.Start()

	createMigrationTable(t, db)
	insertQuery := getSchemaInsert(t, settings.BaseDatabaseSettings.DatabaseProtocol)
	updateQuery := getSchemaUpdate(t, settings.BaseDatabaseSettings.DatabaseProtocol)

	// --------------------------------------------------------------------------

	// Validating successful custom up migration
	mApp = &CDBM{
		DB: db,
		migrateCfg: migrateConfig{
			MigrateType: MigrateTypeUp,
			InsertQuery: insertQuery,
			LogWriter:   func(err error) {},
		},
	}

	if err = mApp.applyMigrationConfig(
		migrationApplyConfig{
			Version: 2,
			CustomMigration: CustomMigration{
				Up: func(db webutil.DBInterface) error {
					return nil
				},
			},
		},
	); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	// Validating successful custom down migration
	mApp = &CDBM{
		DB: db,
		migrateCfg: migrateConfig{
			MigrateType: MigrateTypeDown,
			InsertQuery: insertQuery,
			LogWriter:   func(err error) {},
		},
	}

	if err = mApp.applyMigrationConfig(
		migrationApplyConfig{
			Version: 2,
			CustomMigration: CustomMigration{
				Up: func(db webutil.DBInterface) error {
					return nil
				},
			},
		},
	); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	// Validating file up migration
	mApp = &CDBM{
		DB: db,
		migrateCfg: migrateConfig{
			MigrateType: MigrateTypeUp,
			UpdateQuery: updateQuery,
			LogWriter:   func(err error) {},
			FileMigration: func(mig *migrate.Migrate, version int, mt MigrationsType) error {
				return nil
			},
		},
	}

	if err = mApp.applyMigrationConfig(
		migrationApplyConfig{
			Version: 2,
		},
	); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	// Validating file down migration
	mApp = &CDBM{
		DB: db,
		migrateCfg: migrateConfig{
			MigrateType: MigrateTypeDown,
			UpdateQuery: updateQuery,
			LogWriter:   func(err error) {},
			FileMigration: func(mig *migrate.Migrate, version int, mt MigrationsType) error {
				return nil
			},
		},
	}

	if err = mApp.applyMigrationConfig(
		migrationApplyConfig{
			Version: 2,
		},
	); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}
}

func TestRunMigrationConfigs(t *testing.T) {
	var err error
	var mApp *CDBM

	settings, err := cdbmutil.GetCDBMUtilSettings("")

	if err != nil {
		t.Fatalf(err.Error())
	}

	settings.DBSetup.FileServerSetup = nil
	settings.DBSetup.BaseSchemaFile = ""

	db, dbName, err := cdbmutil.GetNewDatabase(
		settings,
		DefaultExecCmd,
		cdbmutil.DefaultGetDB,
	)

	if err != nil {
		t.Fatalf(err.Error())
	}

	dropCmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(settings.DBAction.DropDB, dbName))
	defer dropCmd.Start()

	createMigrationTable(t, db)
	insertQuery := getSchemaInsert(t, settings.BaseDatabaseSettings.DatabaseProtocol)
	updateQuery := getSchemaUpdate(t, settings.BaseDatabaseSettings.DatabaseProtocol)
	emptyStr := ""
	var sm schemaMigration
	var version2Called bool

	// --------------------------------------------------------------------------

	sm = schemaMigration{
		StartingVersion:   1,
		Dirty:             false,
		DirtyState:        &emptyStr,
		IsCustomMigration: false,
		SchemaCfg: schemaConfig{
			HasEntry: true,
		},
	}

	if _, err = db.Exec(
		insertQuery,
		sm.StartingVersion,
		sm.Dirty,
		sm.DirtyState,
		sm.IsCustomMigration,
	); err != nil {
		t.Fatalf(err.Error())
	}

	mApp = &CDBM{
		DB: db,
		migrateCfg: migrateConfig{
			TargetVersion:   2,
			LogWriter:       func(err error) {},
			MigrateType:     MigrateTypeUp,
			InsertQuery:     insertQuery,
			UpdateQuery:     updateQuery,
			SchemaMigration: sm,
			FileMigration: func(mig *migrate.Migrate, version int, mt MigrationsType) error {
				if version == 2 {
					t.Errorf("should not have version 2")
				}

				_, innerErr := db.Exec(updateQuery, version, false, "", false)
				return innerErr
			},
		},
	}

	// Validating a normal file up migration
	if err = mApp.runMigrationConfigs(
		[]migrationApplyConfig{
			{
				Version: 1,
			},
			{
				Version: 2,
				CustomMigration: CustomMigration{
					Up: func(db webutil.DBInterface) error {
						return nil
					},
				},
			},
			{
				Version: 3,
			},
		},
	); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}

	sm = getSchemaMigration(t, db)

	if sm.StartingVersion != 2 {
		t.Errorf("should be version 2; got %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	sm = schemaMigration{
		StartingVersion:   2,
		Dirty:             true,
		DirtyState:        &emptyStr,
		IsCustomMigration: true,
		SchemaCfg: schemaConfig{
			HasEntry: true,
		},
	}

	if _, err = db.Exec(
		insertQuery,
		sm.StartingVersion,
		sm.Dirty,
		sm.DirtyState,
		sm.IsCustomMigration,
	); err != nil {
		t.Fatalf(err.Error())
	}

	version2Called = false

	mApp = &CDBM{
		DB: db,
		migrateCfg: migrateConfig{
			TargetVersion:   3,
			LogWriter:       func(err error) {},
			MigrateType:     MigrateTypeUp,
			InsertQuery:     insertQuery,
			UpdateQuery:     updateQuery,
			SchemaMigration: sm,
			FileMigration: func(mig *migrate.Migrate, version int, mt MigrationsType) error {
				return nil
			},
		},
	}

	// Validating a normal custom up migration which has dirty flag
	if err = mApp.runMigrationConfigs(
		[]migrationApplyConfig{
			{
				Version: 1,
			},
			{
				Version: 2,
				CustomMigration: CustomMigration{
					Up: func(db webutil.DBInterface) error {
						version2Called = true
						return nil
					},
				},
			},
			{
				Version: 3,
			},
		},
	); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}

	if !version2Called {
		t.Errorf("custom migration version 2 not called\n")
	}

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	sm = schemaMigration{
		StartingVersion:   2,
		Dirty:             false,
		DirtyState:        &emptyStr,
		IsCustomMigration: true,
		SchemaCfg: schemaConfig{
			HasEntry: true,
		},
	}

	if _, err = db.Exec(
		insertQuery,
		sm.StartingVersion,
		sm.Dirty,
		sm.DirtyState,
		sm.IsCustomMigration,
	); err != nil {
		t.Fatalf(err.Error())
	}

	mApp = &CDBM{
		DB: db,
		migrateCfg: migrateConfig{
			TargetVersion:   3,
			LogWriter:       func(err error) {},
			MigrateType:     MigrateTypeUp,
			InsertQuery:     insertQuery,
			UpdateQuery:     updateQuery,
			SchemaMigration: sm,
			FileMigration: func(mig *migrate.Migrate, version int, mt MigrationsType) error {
				return nil
			},
		},
	}

	// Validating a normal custom up migration with no dirty flag
	if err = mApp.runMigrationConfigs(
		[]migrationApplyConfig{
			{
				Version: 1,
			},
			{
				Version: 2,
				CustomMigration: CustomMigration{
					Up: func(db webutil.DBInterface) error {
						t.Errorf("should not be called")
						return nil
					},
				},
			},
			{
				Version: 3,
			},
		},
	); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	sm = schemaMigration{
		StartingVersion:   3,
		Dirty:             false,
		DirtyState:        &emptyStr,
		IsCustomMigration: false,
		SchemaCfg: schemaConfig{
			HasEntry: true,
		},
	}

	if _, err = db.Exec(
		insertQuery,
		sm.StartingVersion,
		sm.Dirty,
		sm.DirtyState,
		sm.IsCustomMigration,
	); err != nil {
		t.Fatalf(err.Error())
	}

	mApp = &CDBM{
		DB: db,
		migrateCfg: migrateConfig{
			TargetVersion:   2,
			LogWriter:       func(err error) {},
			MigrateType:     MigrateTypeDown,
			InsertQuery:     insertQuery,
			UpdateQuery:     updateQuery,
			SchemaMigration: sm,
			FileMigration: func(mig *migrate.Migrate, version int, mt MigrationsType) error {
				fmt.Printf("version: %d\n", version)
				_, innerErr := db.Exec(updateQuery, version-1, false, "", false)
				return innerErr
			},
		},
	}

	version2Called = false

	// Validating a normal file down migration
	if err = mApp.runMigrationConfigs(
		[]migrationApplyConfig{
			{
				Version: 1,
			},
			{
				Version: 2,
				CustomMigration: CustomMigration{
					Up: func(db webutil.DBInterface) error {
						t.Errorf("should not be called")
						return nil
					},
				},
			},
			{
				Version: 3,
			},
		},
	); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}

	sm = getSchemaMigration(t, db)

	if sm.StartingVersion != 2 {
		t.Errorf("file version should be 2; got %d\n", sm.StartingVersion)
	}

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	sm = schemaMigration{
		StartingVersion:   2,
		Dirty:             true,
		DirtyState:        &emptyStr,
		IsCustomMigration: true,
		SchemaCfg: schemaConfig{
			HasEntry: true,
		},
	}

	if _, err = db.Exec(
		insertQuery,
		sm.StartingVersion,
		sm.Dirty,
		sm.DirtyState,
		sm.IsCustomMigration,
	); err != nil {
		t.Fatalf(err.Error())
	}

	mApp = &CDBM{
		DB: db,
		migrateCfg: migrateConfig{
			TargetVersion:   1,
			LogWriter:       func(err error) {},
			MigrateType:     MigrateTypeDown,
			InsertQuery:     insertQuery,
			UpdateQuery:     updateQuery,
			SchemaMigration: sm,
		},
	}

	version2Called = false

	// Validating a normal custom down migration which is dirty
	if err = mApp.runMigrationConfigs(
		[]migrationApplyConfig{
			{
				Version: 1,
			},
			{
				Version: 2,
				CustomMigration: CustomMigration{
					Down: func(db webutil.DBInterface) error {
						version2Called = true
						return nil
					},
				},
			},
			{
				Version: 3,
			},
		},
	); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}

	if !version2Called {
		t.Errorf("version 2 should have been called")
	}

	sm = getSchemaMigration(t, db)

	if sm.StartingVersion != 1 {
		t.Errorf("file version should be 1; got %d\n", sm.StartingVersion)
	}
}

func TestMigrate(t *testing.T) {
	var err error
	var mApp *CDBM

	settings, err := cdbmutil.GetCDBMUtilSettings("")

	if err != nil {
		t.Fatalf(err.Error())
	}

	dbProtocolCfg := DefaultProtocolMap[DBProtocol(settings.BaseDatabaseSettings.DatabaseProtocol)]

	if err != nil {
		t.Fatalf(err.Error())
	}

	settings.DBSetup.FileServerSetup = nil
	settings.DBSetup.BaseSchemaFile = ""

	db, dbName, err := cdbmutil.GetNewDatabase(
		settings,
		DefaultExecCmd,
		cdbmutil.DefaultGetDB,
	)

	if err != nil {
		t.Fatalf(err.Error())
	}

	dropCmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(settings.DBAction.DropDB, dbName))
	defer dropCmd.Start()

	migrationsDir := "/tmp/migrate-integration/"
	//defer os.RemoveAll(migrationsDir)

	var sm schemaMigration

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

	if _, err = os.Create(migrationsDir + "000001_update.down.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000003_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = os.Create(migrationsDir + "000003_update.down.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	cmMap := map[int]CustomMigration{
		2: {
			Up: func(db webutil.DBInterface) error {
				return nil
			},
			Down: func(db webutil.DBInterface) error {
				return nil
			},
		},
	}

	mApp = &CDBM{
		DB:            db,
		DBProtocolCfg: dbProtocolCfg,
		MigrateFlags: MigrateFlagsConfig{
			TargetVersion:      1,
			MigrationsProtocol: FileProtocol,
			MigrationsDir:      migrationsDir,
		},
	}

	if err = mApp.Migrate(
		DefaultGetMigrationFunc,
		DefaultFileMigrationFunc,
		cmMap,
	); err != nil {
		t.Errorf("should not have error; got %+v\n", err)
	}

	sm = getSchemaMigration(t, db)

	if sm.StartingVersion != 1 {
		t.Errorf("version should be 1; got %d\n", sm.StartingVersion)
	}
	if sm.Dirty {
		t.Errorf("should not be dirty")
	}
	if sm.IsCustomMigration {
		t.Errorf("should not be custom")
	}

	mApp.MigrateFlags.TargetVersion = 2

	if err = mApp.Migrate(
		DefaultGetMigrationFunc,
		DefaultFileMigrationFunc,
		cmMap,
	); err != nil {
		t.Errorf("should not have error; got %+v\n", err)
	}

	sm = getSchemaMigration(t, db)

	if sm.StartingVersion != 2 {
		t.Errorf("version should be 2; got %d\n", sm.StartingVersion)
	}
	if sm.Dirty {
		t.Errorf("should not be dirty")
	}
	if !sm.IsCustomMigration {
		t.Errorf("should be custom")
	}

	mApp.MigrateFlags.TargetVersion = 3

	if err = mApp.Migrate(
		DefaultGetMigrationFunc,
		DefaultFileMigrationFunc,
		cmMap,
	); err != nil {
		t.Errorf("should not have error; got %+v\n", err)
	}

	sm = getSchemaMigration(t, db)

	if sm.StartingVersion != 3 {
		t.Errorf("version should be 3; got %d\n", sm.StartingVersion)
	}
	if sm.Dirty {
		t.Errorf("should not be dirty")
	}
	if sm.IsCustomMigration {
		t.Errorf("should not be custom")
	}
}

// func TestMigrate(t *testing.T) {
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

// 	defer os.RemoveAll(migrationsDir)

// 	testGetMigrationFunc := func(migDir string, db *sql.DB, protocolCfg DBProtocolConfig) (*migrate.Migrate, error) {
// 		return &migrate.Migrate{}, nil
// 	}
// 	testFileMigrationFunc := func(mig *migrate.Migrate, version int, mt MigrationsType) error {
// 		return nil
// 	}

// 	// --------------------------------------------------------------------------

// 	// Test reading from directory where no files exists
// 	mApp = &CDBM{
// 		DB:            db,
// 		DBProtocolCfg: dbProtocolCfg,
// 		MigrateFlags: MigrateFlagsConfig{
// 			MigrationsDir: migrationsDir + "invalid/",
// 			TargetVersion: -1,
// 		},
// 	}

// 	if err = mApp.Migrate(
// 		testGetMigrationFunc,
// 		testFileMigrationFunc,
// 		map[int]CustomMigration{},
// 	); err == nil {
// 		t.Errorf("Should have error")
// 	} else if err.Error() != "no sql files or custom migrations found" {
// 		t.Errorf("Should have no sql files error; got %s\n", err.Error())
// 	}

// 	// --------------------------------------------------------------------------

// 	if err = os.RemoveAll(migrationsDir); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = os.Create(migrationsDir + "000000_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	// Should error out due to migration file being below min required
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
// 		testFileMigrationFunc,
// 		map[int]CustomMigration{},
// 	); err == nil {
// 		t.Errorf("should have error")
// 	} else {
// 		if err.Error() != "migration file version less than min version allowed (1)" {
// 			t.Errorf("Should have migration file error; got: %s\n", err.Error())
// 		}
// 	}

// 	// --------------------------------------------------------------------------

// 	if err = os.RemoveAll(migrationsDir); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = os.Create(migrationsDir + "invalid.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	// Should error out due to invalid file name
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
// 		testFileMigrationFunc,
// 		map[int]CustomMigration{},
// 	); err == nil {
// 		t.Errorf("should have error")
// 	} else {
// 		if err.Error() != ErrInvalidFileName.Error() {
// 			t.Errorf("Should have invalid sql file name error; got %s\n", err.Error())
// 		}
// 	}

// 	// --------------------------------------------------------------------------

// 	if err = os.RemoveAll(migrationsDir); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = os.Create(migrationsDir + "invalidVersion_update.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	// Should error out due to invalid file name
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
// 		testFileMigrationFunc,
// 		map[int]CustomMigration{},
// 	); err == nil {
// 		t.Errorf("should have error")
// 	} else {
// 		if err.Error() != ErrInvalidFileName.Error() {
// 			t.Errorf("Should have invalid sql file name error; got %s\n", err.Error())
// 		}
// 	}

// 	// --------------------------------------------------------------------------

// 	if err = os.RemoveAll(migrationsDir); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = os.Create(migrationsDir + "000001_foo.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	// Should error out due to invalid file name
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
// 		testFileMigrationFunc,
// 		map[int]CustomMigration{},
// 	); err == nil {
// 		t.Errorf("should have error")
// 	} else {
// 		if err.Error() != ErrInvalidFileName.Error() {
// 			t.Errorf("Should have invalid sql file name error; got %s\n", err.Error())
// 		}
// 	}

// 	// --------------------------------------------------------------------------

// 	if err = os.RemoveAll(migrationsDir); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = os.Create(migrationsDir + "000001_foo.invalid.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	// Should error out due to invalid file name
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
// 		testFileMigrationFunc,
// 		map[int]CustomMigration{},
// 	); err == nil {
// 		t.Errorf("should have error")
// 	} else {
// 		if err.Error() != ErrInvalidFileName.Error() {
// 			t.Errorf("Should have invalid sql file name error; got %s\n", err.Error())
// 		}
// 	}

// 	// --------------------------------------------------------------------------

// 	if err = os.RemoveAll(migrationsDir); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = os.Create(migrationsDir + "000001_invalid.up.invalid"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	// Should error out due to invalid file name
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
// 		testFileMigrationFunc,
// 		map[int]CustomMigration{},
// 	); err == nil {
// 		t.Errorf("should have error")
// 	} else {
// 		if err.Error() != ErrInvalidFileName.Error() {
// 			t.Errorf("Should have invalid sql file name error; got %s\n", err.Error())
// 		}
// 	}

// 	// --------------------------------------------------------------------------

// 	if err = os.RemoveAll(migrationsDir); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = os.Create(migrationsDir + "000001_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = os.Create(migrationsDir + "000004_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	// Should be invalid due to duplicate versions between files and custom migrations
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
// 		testFileMigrationFunc,
// 		map[int]CustomMigration{
// 			1: {
// 				Up: func(db webutil.DBInterface) error {
// 					return nil
// 				},
// 			},
// 		},
// 	); err == nil {
// 		t.Errorf("should have error")
// 	} else {
// 		if !strings.Contains(err.Error(), "following versions are duplicated between") {
// 			t.Errorf("Should have version duplicate error; got %s\n", err.Error())
// 		}
// 	}

// 	// --------------------------------------------------------------------------

// 	if err = os.RemoveAll(migrationsDir); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = os.Create(migrationsDir + "000001_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = os.Create(migrationsDir + "000002_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = os.Create(migrationsDir + "000003_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	// Should be valid; checking that we get no rows err for MigrationTableSearch as
// 	// the schema_migrations table shouldn't exist yet
// 	//
// 	// Also testing that we break out of loop based on lower target version than available
// 	mApp = &CDBM{
// 		DB:            db,
// 		DBProtocolCfg: dbProtocolCfg,
// 		MigrateFlags: MigrateFlagsConfig{
// 			TargetVersion: 2,
// 			MigrationsDir: migrationsDir,
// 		},
// 	}

// 	if err = mApp.Migrate(
// 		testGetMigrationFunc,
// 		testFileMigrationFunc,
// 		map[int]CustomMigration{},
// 	); err != nil {
// 		t.Errorf("should not have error; got: %s\n", err.Error())
// 	}

// 	// --------------------------------------------------------------------------

// 	if err = os.RemoveAll(migrationsDir); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = os.Create(migrationsDir + "000001_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = os.Create(migrationsDir + "000004_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	mApp = &CDBM{
// 		DB:            db,
// 		DBProtocolCfg: dbProtocolCfg,
// 		MigrateFlags: MigrateFlagsConfig{
// 			TargetVersion: 5,
// 			MigrationsDir: migrationsDir,
// 		},
// 	}

// 	var timeStamp3, timeStamp4, timeStamp6 time.Time
// 	var version3Called, version4Called, version6Called bool

// 	// Verifying that custom migrations are ordered properly and called in order
// 	// with a target-version set
// 	if err = mApp.Migrate(
// 		testGetMigrationFunc,
// 		testFileMigrationFunc,
// 		map[int]CustomMigration{
// 			3: {
// 				Up: func(db webutil.DBInterface) error {
// 					version4Called = true
// 					timeStamp4 = time.Now()
// 					//time.Sleep(time.Millisecond * 500)
// 					return nil
// 				},
// 			},
// 			2: {
// 				Up: func(db webutil.DBInterface) error {
// 					version3Called = true
// 					timeStamp3 = time.Now()
// 					//time.Sleep(time.Millisecond * 500)
// 					return nil
// 				},
// 			},
// 			5: {
// 				Up: func(db webutil.DBInterface) error {
// 					version6Called = true
// 					timeStamp6 = time.Now()
// 					//time.Sleep(time.Millisecond * 500)
// 					return nil
// 				},
// 			},
// 		},
// 	); err != nil {
// 		t.Errorf("should not have error; got %s\n", err.Error())
// 	}

// 	if !version3Called {
// 		t.Errorf("version 3 not called")
// 	}
// 	if !version4Called {
// 		t.Errorf("version 4 not called")
// 	}
// 	if !version6Called {
// 		t.Errorf("version 6 not called")
// 	}

// 	if timeStamp3.After(timeStamp4) || timeStamp3.After(timeStamp6) {
// 		t.Errorf("timeStamp3 should not be after timeStamp4 or timeStamp6")
// 	}
// 	if timeStamp4.After(timeStamp6) || timeStamp4.Before(timeStamp3) {
// 		t.Errorf("timeStamp4 should not be after timeStamp6 or before timeStamp3")
// 	}
// 	if timeStamp6.Before(timeStamp3) || timeStamp6.Before(timeStamp4) {
// 		t.Errorf("timeStamp6 should not be before timeStamp3 or timeStamp4")
// 	}

// 	// --------------------------------------------------------------------------

// 	if _, err = db.Exec("delete from schema_migrations"); err != nil {
// 		t.Fatalf(err.Error())
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

// 	if _, err = os.Create(migrationsDir + "000004_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	mApp = &CDBM{
// 		DB:            db,
// 		DBProtocolCfg: dbProtocolCfg,
// 		MigrateFlags: MigrateFlagsConfig{
// 			TargetVersion: -1,
// 			MigrationsDir: migrationsDir,
// 		},
// 	}

// 	// Verifying that custom migrations error activates
// 	if err = mApp.Migrate(
// 		testGetMigrationFunc,
// 		testFileMigrationFunc,
// 		map[int]CustomMigration{
// 			3: {
// 				Up: func(db webutil.DBInterface) error {
// 					return nil
// 				},
// 			},
// 			2: {
// 				Up: func(db webutil.DBInterface) error {
// 					return fmt.Errorf("custom migration error")
// 				},
// 			},
// 			5: {
// 				Up: func(db webutil.DBInterface) error {
// 					return nil
// 				},
// 			},
// 		},
// 	); err == nil {
// 		t.Errorf("should have error")
// 	} else {
// 		if errors.Cause(err).Error() != "failed on custom up migration for version: '2'" {
// 			t.Errorf("should have custom migration error; got %s\n", err.Error())
// 		}
// 	}

// 	// --------------------------------------------------------------------------

// 	if _, err = db.Exec("delete from schema_migrations"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	schemaInsert, args, err := webutil.InQueryRebind(
// 		dbProtocolCfg.SQLBindVar,
// 		`
// 		insert into schema_migrations(version, dirty)
// 		values(?, ?);
// 		`,
// 		2,
// 		true,
// 	)

// 	if err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = db.Exec(schemaInsert, args...); err != nil {
// 		t.Fatalf(err.Error())
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

// 	mApp = &CDBM{
// 		DB:            db,
// 		DBProtocolCfg: dbProtocolCfg,
// 		MigrateFlags: MigrateFlagsConfig{
// 			TargetVersion: -1,
// 			MigrationsDir: migrationsDir,
// 		},
// 	}

// 	// Verifying that we get error message for not setting --reset-dirty-flag flag when
// 	// a migration has a dirty flag
// 	if err = mApp.Migrate(
// 		testGetMigrationFunc,
// 		testFileMigrationFunc,
// 		map[int]CustomMigration{
// 			2: {
// 				Up: func(db webutil.DBInterface) error {
// 					return nil
// 				},
// 			},
// 		},
// 	); err == nil {
// 		t.Errorf("should have error")
// 	} else {
// 		if err.Error() != "must set --reset-dirty-flag to reset migrations dirty flag.  Use 'cdbm status' to see current status of migration" {
// 			t.Errorf("should have reset dirt flag error: got %s\n", err.Error())
// 		}
// 	}

// 	// --------------------------------------------------------------------------

// 	if _, err = db.Exec("delete from schema_migrations"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	schemaInsert, args, err = webutil.InQueryRebind(
// 		dbProtocolCfg.SQLBindVar,
// 		`
// 		insert into schema_migrations(version, dirty)
// 		values(?, ?);
// 		`,
// 		2,
// 		true,
// 	)

// 	if err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = db.Exec(schemaInsert, args...); err != nil {
// 		t.Fatalf(err.Error())
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

// 	mApp = &CDBM{
// 		DB:            db,
// 		DBProtocolCfg: dbProtocolCfg,
// 		MigrateFlags: MigrateFlagsConfig{
// 			TargetVersion:  -1,
// 			MigrationsDir:  migrationsDir,
// 			ResetDirtyFlag: true,
// 		},
// 	}

// 	// Verifying that we when set --reset-dirty-flag flag that we don't get an error
// 	if err = mApp.Migrate(
// 		testGetMigrationFunc,
// 		testFileMigrationFunc,
// 		map[int]CustomMigration{
// 			2: {
// 				Up: func(db webutil.DBInterface) error {
// 					return nil
// 				},
// 			},
// 		},
// 	); err != nil {
// 		t.Errorf("should not have error; got %s\n", err.Error())
// 	}

// 	// --------------------------------------------------------------------------

// 	if _, err = db.Exec("delete from schema_migrations"); err != nil {
// 		t.Fatalf(err.Error())
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

// 	// Testing target version being too high
// 	mApp = &CDBM{
// 		DB:            db,
// 		DBProtocolCfg: dbProtocolCfg,
// 		MigrateFlags: MigrateFlagsConfig{
// 			TargetVersion: 100,
// 			MigrationsDir: migrationsDir,
// 		},
// 	}

// 	if err = mApp.Migrate(
// 		testGetMigrationFunc,
// 		testFileMigrationFunc,
// 		map[int]CustomMigration{},
// 	); err == nil {
// 		t.Errorf("should have error")
// 	} else {
// 		if err.Error() != "--target-version does not exist" {
// 			t.Errorf("should have --target-version error; got %s\n", err.Error())
// 		}
// 	}

// 	// --------------------------------------------------------------------------

// 	if _, err = db.Exec("delete from schema_migrations"); err != nil {
// 		t.Fatalf(err.Error())
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

// 	if _, err = os.Create(migrationsDir + "000002_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	// Testing target version
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
// 		testFileMigrationFunc,
// 		map[int]CustomMigration{},
// 	); err != nil {
// 		t.Errorf("should not have error; got %s\n", err.Error())
// 	}

// 	// --------------------------------------------------------------------------

// 	if _, err = db.Exec("delete from schema_migrations"); err != nil {
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

// 	if _, err = os.Create(migrationsDir + "000001_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = os.Create(migrationsDir + "000002_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = os.Create(migrationsDir + "000003_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	// Testing rollback on failure for file migration and failure on the rollback
// 	// itself for file migration
// 	mApp = &CDBM{
// 		DB:            db,
// 		DBProtocolCfg: dbProtocolCfg,
// 		MigrateFlags: MigrateFlagsConfig{
// 			TargetVersion:     -1,
// 			MigrationsDir:     migrationsDir,
// 			RollbackOnFailure: true,
// 		},
// 	}

// 	if err = mApp.Migrate(
// 		testGetMigrationFunc,
// 		func(mig *migrate.Migrate, version int, mt MigrationsType) error {
// 			if version == 3 {
// 				return fmt.Errorf("file migration error")
// 			}

// 			return nil
// 		},
// 		map[int]CustomMigration{},
// 	); err == nil {
// 		t.Errorf("should have error\n")
// 	} else {
// 		if !strings.Contains(err.Error(), "failed on file up migration for version: '3'") {
// 			t.Errorf("should have file up migration error; got %s\n", err.Error())
// 		}

// 		if !strings.Contains(err.Error(), "failed on file rollback migration for version: '3'") {
// 			t.Errorf("should have file rollback error; got %s\n", err.Error())
// 		}
// 	}

// 	// --------------------------------------------------------------------------

// 	if _, err = db.Exec("delete from schema_migrations"); err != nil {
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

// 	if _, err = os.Create(migrationsDir + "000001_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = os.Create(migrationsDir + "000003_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	// Testing rollback on failure for custom migration and failure on the rollback
// 	// itself for custom migration
// 	mApp = &CDBM{
// 		DB:            db,
// 		DBProtocolCfg: dbProtocolCfg,
// 		MigrateFlags: MigrateFlagsConfig{
// 			TargetVersion:     -1,
// 			MigrationsDir:     migrationsDir,
// 			RollbackOnFailure: true,
// 		},
// 	}

// 	if err = mApp.Migrate(
// 		testGetMigrationFunc,
// 		testFileMigrationFunc,
// 		map[int]CustomMigration{
// 			2: {
// 				Up: func(db webutil.DBInterface) error {
// 					return fmt.Errorf("custom migration error")
// 				},
// 				Down: func(db webutil.DBInterface) error {
// 					return fmt.Errorf("custom migration error")
// 				},
// 			},
// 		},
// 	); err == nil {
// 		t.Errorf("should have error\n")
// 	} else {
// 		if !strings.Contains(err.Error(), "failed on custom up migration for version: '2'") {
// 			t.Errorf("should have custom up migration error; got %s\n", err.Error())
// 		}

// 		if !strings.Contains(err.Error(), "failed on custom rollback migration for version: '2'") {
// 			t.Errorf("should have custom rollback error; got %s\n", err.Error())
// 		}
// 	}

// 	// --------------------------------------------------------------------------

// 	if _, err = db.Exec("delete from schema_migrations"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = db.Exec(
// 		`
// 		insert into schema_migrations(version, dirty)
// 		values(2, false)
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

// 	if _, err = os.Create(migrationsDir + "000001_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = os.Create(migrationsDir + "000001_update.down.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = os.Create(migrationsDir + "000002_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = os.Create(migrationsDir + "000002_update.down.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = os.Create(migrationsDir + "000003_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = os.Create(migrationsDir + "000003_update.down.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	// Testing file down migration with error
// 	mApp = &CDBM{
// 		DB:            db,
// 		DBProtocolCfg: dbProtocolCfg,
// 		MigrateFlags: MigrateFlagsConfig{
// 			TargetVersion:     1,
// 			MigrationsDir:     migrationsDir,
// 			RollbackOnFailure: true,
// 		},
// 	}

// 	if err = mApp.Migrate(
// 		testGetMigrationFunc,
// 		func(mig *migrate.Migrate, version int, mt MigrationsType) error {
// 			return fmt.Errorf("file migration error")
// 		},
// 		map[int]CustomMigration{},
// 	); err == nil {
// 		t.Errorf("should have error")
// 	} else if err.Error() != "failed on file down migration for version: '2'" {
// 		t.Errorf("should have file down migration; got %s\n", err.Error())
// 	}

// 	// --------------------------------------------------------------------------

// 	if _, err = db.Exec("delete from schema_migrations"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = db.Exec(
// 		`
// 		insert into schema_migrations(version, dirty)
// 		values(3, false)
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

// 	if _, err = os.Create(migrationsDir + "000001_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = os.Create(migrationsDir + "000001_update.down.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = os.Create(migrationsDir + "000003_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = os.Create(migrationsDir + "000003_update.down.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	// Testing custom down migration with error
// 	mApp = &CDBM{
// 		DB:            db,
// 		DBProtocolCfg: dbProtocolCfg,
// 		MigrateFlags: MigrateFlagsConfig{
// 			TargetVersion:     1,
// 			MigrationsDir:     migrationsDir,
// 			RollbackOnFailure: true,
// 		},
// 	}

// 	if err = mApp.Migrate(
// 		testGetMigrationFunc,
// 		testFileMigrationFunc,
// 		map[int]CustomMigration{
// 			2: {
// 				Down: func(db webutil.DBInterface) error {
// 					return fmt.Errorf("custom migration error")
// 				},
// 			},
// 		},
// 	); err == nil {
// 		t.Errorf("should have error")
// 	} else if err.Error() != "failed on custom down migration for version: '2'" {
// 		t.Errorf("should have file down migration; got %s\n", err.Error())
// 	}
// }

// func TestMigrateIntegration(t *testing.T) {
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

// 	rootDir := "/tmp/migrate-integration/"
// 	migrationsDir := rootDir + "migrations/"

// 	defer os.RemoveAll(migrationsDir)

// 	var file1Up, file1Down, file2Up, file2Down *os.File

// 	// --------------------------------------------------------------------------

// 	if err = os.RemoveAll(migrationsDir); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
// 		t.Fatalf(err.Error())
// 	}

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

// 	// Should be valid
// 	mApp = &CDBM{
// 		DB:            db,
// 		DBProtocolCfg: dbProtocolCfg,
// 		DropFlags: DropFlagsConfig{
// 			Confirm: true,
// 		},
// 		MigrateFlags: MigrateFlagsConfig{
// 			TargetVersion: -1,
// 			MigrationsDir: migrationsDir,
// 		},
// 	}

// 	if err = mApp.Migrate(
// 		DefaultGetMigrationFunc,
// 		DefaultFileMigrationFunc,
// 		map[int]CustomMigration{},
// 	); err != nil {
// 		t.Errorf("should not have error; got %s\n", err.Error())
// 	}

// 	if err = mApp.Drop(); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	// --------------------------------------------------------------------------

// 	var file3Up, file3Down, file4Up, file4Down *os.File

// 	if err = os.RemoveAll(migrationsDir); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	// Simulating migrations of first two migrations
// 	if _, err = db.Exec(
// 		`
// 		create table schema_migrations(
// 			version int primary key,
// 			dirty boolean not null,
// 			dirty_state text
// 		);

// 		insert into schema_migrations(version, dirty)
// 		values (2, false);

// 		create table foo(
// 			id serial,
// 			name string not null
// 		);

// 		insert into foo(name)
// 		values ('test1');

// 		insert into foo(name)
// 		values ('test2');
// 		`,
// 	); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if file1Up, err = os.Create(migrationsDir + "000001_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = file1Up.Write([]byte(
// 		`
// 		create table foo(
// 			id int not null primary key,
// 			name string not null
// 		);

// 		insert into foo(name)
// 		values ('test1');
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

// 	if file2Up, err = os.Create(migrationsDir + "000002_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = file2Up.Write([]byte(
// 		`
// 		insert into foo(name)
// 		values ('test2');
// 		`,
// 	)); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if file2Down, err = os.Create(migrationsDir + "000002_update.down.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = file2Down.Write([]byte("delete from foo where name = 'test2'")); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if file3Up, err = os.Create(migrationsDir + "000003_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = file3Up.Write([]byte(
// 		`
// 		insert into foo(name)
// 		values ('test3');
// 		`,
// 	)); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if file3Down, err = os.Create(migrationsDir + "000003_update.down.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = file3Down.Write([]byte("delete from foo where name = 'test3'")); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if file4Up, err = os.Create(migrationsDir + "000004_update.up.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = file4Up.Write([]byte(
// 		`
// 		insert into foo(name)
// 		values ('test4');
// 		`,
// 	)); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if file4Down, err = os.Create(migrationsDir + "000004_update.down.sql"); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if _, err = file4Down.Write([]byte("delete from foo where name = 'test4'")); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	// Testing rollback on failure integration
// 	mApp = &CDBM{
// 		DB:            db,
// 		DBProtocolCfg: dbProtocolCfg,
// 		MigrateFlags: MigrateFlagsConfig{
// 			TargetVersion:     -1,
// 			MigrationsDir:     migrationsDir,
// 			RollbackOnFailure: true,
// 		},
// 	}

// 	if err = mApp.Migrate(
// 		DefaultGetMigrationFunc,
// 		func(mig *migrate.Migrate, version int, mt MigrationsType) error {
// 			if version == 4 && mt == MigrateTypeUp {
// 				return fmt.Errorf("file error")
// 			}

// 			switch mt {
// 			case MigrateTypeUp:
// 				return mig.Steps(1)
// 			case MigrateTypeDown:
// 				return mig.Steps(-1)
// 			}

// 			return nil
// 		},
// 		map[int]CustomMigration{},
// 	); err == nil {
// 		t.Errorf("should have error")
// 	} else if err.Error() != "failed on file up migration for version: '4'" {
// 		t.Errorf("should have file up migration; got %s\n", err.Error())
// 	}

// 	var sm schemaMigration

// 	if err = db.QueryRow(
// 		`
// 		select
// 			schema_migrations.version,
// 			schema_migrations.dirty,
// 			schema_migrations.dirty_state
// 		from
// 			schema_migrations
// 		`,
// 	).Scan(&sm.StartingVersion, &sm.Dirty, &sm.DirtyState); err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	if sm.StartingVersion != 2 {
// 		t.Errorf("version should be 2; got %d\n", sm.StartingVersion)
// 	}
// 	if sm.Dirty {
// 		t.Errorf("should not be dirty\n")
// 	}
// 	if sm.DirtyState != nil && *sm.DirtyState != "" {
// 		t.Errorf("should not have a dirty state; got %s\n", *sm.DirtyState)
// 	}

// 	var filler interface{}

// 	err = db.QueryRow(
// 		`
// 		select
// 			foo.id
// 		from
// 			foo
// 		where
// 			name = 'test3'
// 		`,
// 	).Scan(&filler)

// 	if err == nil {
// 		t.Errorf("should have error\n")
// 	} else if !errors.Is(err, sql.ErrNoRows) {
// 		t.Errorf("should no rows error; got %s\n", err.Error())
// 	}

// 	err = db.QueryRow(
// 		`
// 		select
// 			foo.id
// 		from
// 			foo
// 		where
// 			name = 'test4'
// 		`,
// 	).Scan(&filler)

// 	if err == nil {
// 		t.Errorf("should have error\n")
// 	} else if !errors.Is(err, sql.ErrNoRows) {
// 		t.Errorf("should no rows error; got %s\n", err.Error())
// 	}
// }
