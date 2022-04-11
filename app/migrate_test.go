package app

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"

	"github.com/TravisS25/cdbm/cdbmutil"
	"github.com/TravisS25/webutil/webutil"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func getSchemaInsert(t *testing.T, dbProtocol string) string {
	schemaInsert, _, err := webutil.InQueryRebind(
		cdbmutil.DefaultProtocolMap[cdbmutil.DBProtocol(dbProtocol)].SQLBindVar,
		`
		insert into schema_migrations(version, dirty, dirty_state, is_custom_migration) 
		values(?, ?, ?, ?);
		`,
		0,
		true,
		cdbmutil.MigrateTypeUp,
		true,
	)

	if err != nil {
		t.Fatalf(err.Error())
	}

	return schemaInsert
}

func getSchemaUpdate(t *testing.T, dbProtocol string) string {
	schemaUpdate, _, err := webutil.InQueryRebind(
		cdbmutil.DefaultProtocolMap[cdbmutil.DBProtocol(dbProtocol)].SQLBindVar,
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
		cdbmutil.MigrateTypeUp,
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
		t.Fatalf("%+v", errors.WithStack(err))
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
			MigrationsProtocol: cdbmutil.MigrationsProtocol("foo"),
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
			MigrationsProtocol: cdbmutil.FileProtocol,
		},
	}

	if err = mApp.checkMigrationsProtocol(); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}
}

func TestApplySchemaMigrationsQueries(t *testing.T) {
	settings, err := cdbmutil.GetCDBMUtilSettings("")

	if err != nil {
		t.Fatalf(err.Error())
	}

	mApp := &CDBM{
		DBProtocolCfg: cdbmutil.DefaultProtocolMap[cdbmutil.DBProtocol(settings.BaseDatabaseSettings.DatabaseProtocol)],
	}

	if err = mApp.applySchemaMigrationsQueries(); err != nil {
		t.Errorf("should not have error; gott %s\n", err.Error())
	}
}

func TestCreateLogsDirectory(t *testing.T) {
	md := "/tmp/cdbm-log/log.txt"

	defer os.RemoveAll(md)

	c := &CDBM{
		LogFlags: LogFlagsConfig{
			LogFile: md,
		},
	}

	_, err := c.createLogsDirectory()

	if err != nil {
		t.Fatalf(err.Error())
	}

	if _, err = c.createLogsDirectory(); err != nil {
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
	} else if err == cdbmutil.ErrInvalidFileName {
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
	} else if err == cdbmutil.ErrInvalidFileName {
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
	} else if err == cdbmutil.ErrInvalidFileName {
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
	} else if err == cdbmutil.ErrInvalidFileName {
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
	} else if err == cdbmutil.ErrInvalidFileName {
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

	// Should be valid
	mApp = &CDBM{
		MigrateFlags: MigrateFlagsConfig{
			MigrationsDir: migrationsDir,
		},
		migrateCfg: migrateState{
			CustomMigrations: map[int]cdbmutil.CustomMigration{
				2: {},
				3: {},
			},
		},
	}

	var cfgs []migrationApplyConfig

	if cfgs, err = mApp.verifyFilesAndMigrations(); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}

	if len(cfgs) != 3 {
		t.Errorf("should have len of 3; got %d\n", len(cfgs))
	}
}

func TestGetSchemaMigration(t *testing.T) {
	var err error

	tableSearchErr := errors.New("table search error")

	dbpCfg := cdbmutil.DefaultProtocolMap[cdbmutil.CockroachdbProtocol]
	dbpCfg.MigrationTableSearch = func(db webutil.DBInterface) error {
		return errors.WithStack(tableSearchErr)
	}

	c := &CDBM{
		DBProtocolCfg: dbpCfg,
	}

	if _, err = c.getSchemaMigration(); err == nil {
		t.Errorf("should have error")
	} else if err.Error() != tableSearchErr.Error() {
		t.Errorf("should have %s; got %s", tableSearchErr.Error(), err.Error())
	}

	db, mockDB, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlAnyMatcher))

	if err != nil {
		t.Fatalf(err.Error())
	}

	dbError := errors.New("db error")

	c.DB = sqlx.NewDb(db, webutil.Postgres)
	c.DBProtocolCfg.MigrationTableSearch = func(db webutil.DBInterface) error {
		return sql.ErrNoRows
	}

	mockDB.ExpectExec("").WillReturnError(dbError)

	if _, err = c.getSchemaMigration(); err == nil {
		t.Errorf("should have error")
	} else if err.Error() != dbError.Error() {
		t.Errorf("should have %s; got %s", dbError.Error(), err.Error())
	}

	mockDB.ExpectExec("").WillReturnResult(sqlmock.NewResult(1, 1))

	if _, err = c.getSchemaMigration(); err != nil {
		t.Errorf("should not have error; got %+v", err)
	}

	c.DBProtocolCfg.MigrationTableSearch = func(db webutil.DBInterface) error {
		return nil
	}

	mockDB.ExpectQuery("").WillReturnError(dbError)

	if _, err = c.getSchemaMigration(); err == nil {
		t.Errorf("should have error")
	} else if err.Error() != dbError.Error() {
		t.Errorf("should have %s; got %s", dbError.Error(), err.Error())
	}

	mockDB.ExpectQuery("").WillReturnRows(
		mockDB.NewRows([]string{"version", "dirty", "dirty_state", "is_custom_migration"}).
			AddRow(1, false, nil, true),
	)

	if _, err = c.getSchemaMigration(); err != nil {
		t.Errorf("should not have error; got %+v", err)
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
		cdbmutil.DefaultExecCmd,
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
		migrateCfg: migrateState{
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
		migrateCfg: migrateState{
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
		cdbmutil.DefaultExecCmd,
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

	// Validating custom down migration error with having rows in schema_migrations table
	mApp = &CDBM{
		DB: db,
		MigrateFlags: MigrateFlagsConfig{
			RollbackOnFailure: true,
		},
		migrateCfg: migrateState{
			InsertQuery: insertQuery,
			UpdateQuery: updateQuery,
			MigrateType: cdbmutil.MigrateTypeUp,
			LogWriter: func(err error) {
				if err == nil {
					t.Errorf("should have error for logger")
				} else if err.Error() != "custom down migration error" {
					t.Errorf("should have custom down migration error; got %s\n", err.Error())
				}
			},
			CustomMigrations: map[int]cdbmutil.CustomMigration{
				2: {
					Down: func(db webutil.DBInterface) error {
						return fmt.Errorf("custom down migration error")
					},
				},
			},
			SchemaMigration: schemaMigration{
				SchemaCfg: schemaConfig{
					NoRows: false,
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

	// Validating custom down migration error with having no rows in schema_migrations table
	mApp = &CDBM{
		DB: db,
		MigrateFlags: MigrateFlagsConfig{
			RollbackOnFailure: true,
		},
		migrateCfg: migrateState{
			InsertQuery: insertQuery,
			UpdateQuery: updateQuery,
			MigrateType: cdbmutil.MigrateTypeUp,
			LogWriter: func(err error) {
				if err == nil {
					t.Errorf("should have error for logger")
				} else if err.Error() != "custom down migration error" {
					t.Errorf("should have custom down migration error; got %s\n", err.Error())
				}
			},
			CustomMigrations: map[int]cdbmutil.CustomMigration{
				2: {
					Down: func(db webutil.DBInterface) error {
						return fmt.Errorf("custom down migration error")
					},
				},
			},
			SchemaMigration: schemaMigration{
				SchemaCfg: schemaConfig{
					NoRows: true,
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
		migrateCfg: migrateState{
			InsertQuery: insertQuery,
			UpdateQuery: updateQuery,
			MigrateType: cdbmutil.MigrateTypeUp,
			LogWriter: func(err error) {
				if err == nil {
					t.Errorf("should have error for logger")
				} else if err.Error() != "file down migration error" {
					t.Errorf("should have file down migration error; got %s\n", err.Error())
				}
			},
			FileMigration: func(mig *migrate.Migrate, version int, mt cdbmutil.MigrationsType) error {
				return fmt.Errorf("file down migration error")
			},
			SchemaMigration: schemaMigration{
				SchemaCfg: schemaConfig{
					NoRows: false,
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

	if _, err = db.Exec(insertQuery, 3, false, "", false); err != nil {
		t.Fatalf(err.Error())
	}

	// Validating file down migration error with having rows in schema_migration table
	mApp = &CDBM{
		DB: db,
		MigrateFlags: MigrateFlagsConfig{
			RollbackOnFailure: true,
		},
		migrateCfg: migrateState{
			InsertQuery: insertQuery,
			UpdateQuery: updateQuery,
			MigrateType: cdbmutil.MigrateTypeUp,
			LogWriter: func(err error) {
				if err == nil {
					t.Errorf("should have error for logger")
				} else if err.Error() != "file down migration error" {
					t.Errorf("should have file down migration error; got %s\n", err.Error())
				}
			},
			FileMigration: func(mig *migrate.Migrate, version int, mt cdbmutil.MigrationsType) error {
				return fmt.Errorf("file down migration error")
			},
			SchemaMigration: schemaMigration{
				SchemaCfg: schemaConfig{
					NoRows: true,
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

	// Should be valid with no rows
	mApp = &CDBM{
		DB: db,
		MigrateFlags: MigrateFlagsConfig{
			RollbackOnFailure: true,
		},
		migrateCfg: migrateState{
			InsertQuery: insertQuery,
			UpdateQuery: updateQuery,
			MigrateType: cdbmutil.MigrateTypeDown,
			LogWriter: func(err error) {
				if err == nil {
					t.Errorf("should have error for logger")
				} else if err.Error() != "file down migration error" {
					t.Errorf("should have file down migration error; got %s\n", err.Error())
				}
			},
			FileMigration: func(mig *migrate.Migrate, version int, mt cdbmutil.MigrationsType) error {
				return nil
			},
			SchemaMigration: schemaMigration{
				SchemaCfg: schemaConfig{
					NoRows: true,
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
		cdbmutil.DefaultExecCmd,
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
		migrateCfg: migrateState{
			MigrateType: cdbmutil.MigrateTypeUp,
			InsertQuery: insertQuery,
			UpdateQuery: updateQuery,
			LogWriter:   func(err error) {},
		},
	}

	if err = mApp.applyCustomMigration(
		migrationApplyConfig{
			Version: 2,
			CustomMigration: cdbmutil.CustomMigration{
				Up: func(db webutil.DBInterface) error {
					return nil
				},
				Down: func(db webutil.DBInterface) error {
					return nil
				},
			},
		},
	); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}

	if mApp.migrateCfg.SchemaMigration.SchemaCfg.NoRows {
		t.Errorf("should have rows")
	}

	// --------------------------------------------------------------------------

	// Validating successful custom migration with inital dirty state
	mApp = &CDBM{
		DB:           db,
		MigrateFlags: MigrateFlagsConfig{
			//MigrateDownIfDirty: true,
		},
		migrateCfg: migrateState{
			MigrateType: cdbmutil.MigrateTypeUp,
			InsertQuery: insertQuery,
			UpdateQuery: updateQuery,
			LogWriter:   func(err error) {},
			SchemaMigration: schemaMigration{
				SchemaCfg: schemaConfig{
					Dirty: true,
				},
			},
		},
	}

	if err = mApp.applyCustomMigration(
		migrationApplyConfig{
			Version: 2,
			CustomMigration: cdbmutil.CustomMigration{
				Up: func(db webutil.DBInterface) error {
					return nil
				},
				Down: func(db webutil.DBInterface) error {
					return nil
				},
			},
		},
	); err != nil {
		t.Errorf("should not have error; %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	// Validating error on migrating down on with inital dirty state
	mApp = &CDBM{
		DB:           db,
		MigrateFlags: MigrateFlagsConfig{
			//MigrateDownIfDirty: true,
		},
		migrateCfg: migrateState{
			MigrateType: cdbmutil.MigrateTypeUp,
			InsertQuery: insertQuery,
			UpdateQuery: updateQuery,
			LogWriter:   func(err error) {},
			SchemaMigration: schemaMigration{
				SchemaCfg: schemaConfig{
					Dirty: true,
				},
			},
		},
	}

	if err = mApp.applyCustomMigration(
		migrationApplyConfig{
			Version: 2,
			CustomMigration: cdbmutil.CustomMigration{
				Up: func(db webutil.DBInterface) error {
					return nil
				},
				Down: func(db webutil.DBInterface) error {
					return fmt.Errorf("custom migration error")
				},
			},
		},
	); err == nil {
		t.Errorf("should have error\n")
	} else if !strings.Contains(err.Error(), "failed of resetting custom migration for version") {
		t.Errorf("error should contain resetting custom migration; got %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	// Validating successful custom up migration with no rows in schema_migration table
	mApp = &CDBM{
		DB: db,
		migrateCfg: migrateState{
			MigrateType: cdbmutil.MigrateTypeUp,
			InsertQuery: insertQuery,
			UpdateQuery: updateQuery,
			LogWriter:   func(err error) {},
			SchemaMigration: schemaMigration{
				SchemaCfg: schemaConfig{
					NoRows: true,
				},
			},
		},
	}

	if err = mApp.applyCustomMigration(
		migrationApplyConfig{
			Version: 2,
			CustomMigration: cdbmutil.CustomMigration{
				Up: func(db webutil.DBInterface) error {
					return nil
				},
				Down: func(db webutil.DBInterface) error {
					return nil
				},
			},
		},
	); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}

	if mApp.migrateCfg.SchemaMigration.SchemaCfg.NoRows {
		t.Errorf("should have rows")
	}

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	mApp = &CDBM{
		DB: db,
		migrateCfg: migrateState{
			MigrateType: cdbmutil.MigrateTypeDown,
			InsertQuery: insertQuery,
			UpdateQuery: updateQuery,
			LogWriter:   func(err error) {},
		},
	}

	// Validating successful custom down migration
	if err = mApp.applyCustomMigration(
		migrationApplyConfig{
			Version: 2,
			CustomMigration: cdbmutil.CustomMigration{
				Up: func(db webutil.DBInterface) error {
					return nil
				},
				Down: func(db webutil.DBInterface) error {
					return nil
				},
			},
		},
	); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}

	if mApp.migrateCfg.SchemaMigration.SchemaCfg.NoRows {
		t.Errorf("should have rows")
	}

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	// Validating custom up migration error
	mApp = &CDBM{
		DB: db,
		migrateCfg: migrateState{
			InsertQuery: insertQuery,
			UpdateQuery: updateQuery,
			MigrateType: cdbmutil.MigrateTypeUp,
			LogWriter:   func(err error) {},
		},
	}

	if err = mApp.applyCustomMigration(
		migrationApplyConfig{
			Version: 2,
			CustomMigration: cdbmutil.CustomMigration{
				Up: func(db webutil.DBInterface) error {
					return fmt.Errorf("custom migration error")
				},
				Down: func(db webutil.DBInterface) error {
					return nil
				},
			},
		},
	); err == nil {
		t.Errorf("should have error")
	} else if err.Error() != "failed on custom up migration for version: '2'" {
		t.Errorf("should have custom up migration error; got %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	// Validating custom up migration error with no rows in schema_migrations table
	mApp = &CDBM{
		DB: db,
		migrateCfg: migrateState{
			InsertQuery: insertQuery,
			UpdateQuery: updateQuery,
			MigrateType: cdbmutil.MigrateTypeUp,
			LogWriter:   func(err error) {},
			SchemaMigration: schemaMigration{
				SchemaCfg: schemaConfig{
					NoRows: true,
				},
			},
		},
	}

	if err = mApp.applyCustomMigration(
		migrationApplyConfig{
			Version: 2,
			CustomMigration: cdbmutil.CustomMigration{
				Up: func(db webutil.DBInterface) error {
					return fmt.Errorf("custom migration error")
				},
				Down: func(db webutil.DBInterface) error {
					return nil
				},
			},
		},
	); err == nil {
		t.Errorf("should have error")
	} else if err.Error() != "failed on custom up migration for version: '2'" {
		t.Errorf("should have custom up migration error; got %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	// Validating custom up migration error with successful rollback
	mApp = &CDBM{
		DB: db,
		MigrateFlags: MigrateFlagsConfig{
			RollbackOnFailure: true,
		},
		migrateCfg: migrateState{
			InsertQuery: insertQuery,
			UpdateQuery: updateQuery,
			MigrateType: cdbmutil.MigrateTypeUp,
			LogWriter:   func(err error) {},
			FileMigration: func(mig *migrate.Migrate, version int, mt cdbmutil.MigrationsType) error {
				return nil
			},
			SchemaMigration: schemaMigration{
				StartingVersion: 1,
			},
		},
	}

	if err = mApp.applyCustomMigration(
		migrationApplyConfig{
			Version: 2,
			CustomMigration: cdbmutil.CustomMigration{
				Up: func(db webutil.DBInterface) error {
					return fmt.Errorf("custom migration error")
				},
				Down: func(db webutil.DBInterface) error {
					return nil
				},
			},
		},
	); err == nil {
		t.Errorf("should have error")
	} else {
		if !strings.Contains(err.Error(), "successfully rolled back") {
			t.Errorf("should have successful rollback; got %s\n", err.Error())
		}
	}

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	// Validating custom up migration error and failed on rollback
	mApp = &CDBM{
		DB: db,
		MigrateFlags: MigrateFlagsConfig{
			RollbackOnFailure: true,
		},
		migrateCfg: migrateState{
			InsertQuery: insertQuery,
			UpdateQuery: updateQuery,
			MigrateType: cdbmutil.MigrateTypeUp,
			LogWriter:   func(err error) {},
			CustomMigrations: map[int]cdbmutil.CustomMigration{
				2: {
					Down: func(db webutil.DBInterface) error {
						return fmt.Errorf("custom migration error")
					},
				},
			},
			FileMigration: func(mig *migrate.Migrate, version int, mt cdbmutil.MigrationsType) error {
				return fmt.Errorf("file migration error")
			},
			SchemaMigration: schemaMigration{
				StartingVersion: 1,
			},
		},
	}

	if err = mApp.applyCustomMigration(
		migrationApplyConfig{
			Version: 2,
			CustomMigration: cdbmutil.CustomMigration{
				Up: func(db webutil.DBInterface) error {
					return fmt.Errorf("custom migration error")
				},
				Down: func(db webutil.DBInterface) error {
					return nil
				},
			},
		},
	); err == nil {
		t.Errorf("should have error")
	} else {
		if !strings.Contains(err.Error(), "failed on custom rollback migration for version: '2'") {
			t.Errorf("should have failed rollback; got %s\n", err.Error())
		}
	}

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	// Validating custom down migration error
	mApp = &CDBM{
		DB: db,
		migrateCfg: migrateState{
			InsertQuery: insertQuery,
			UpdateQuery: updateQuery,
			MigrateType: cdbmutil.MigrateTypeDown,
			LogWriter:   func(err error) {},
			SchemaMigration: schemaMigration{
				SchemaCfg: schemaConfig{
					NoRows: false,
				},
			},
		},
	}

	if err = mApp.applyCustomMigration(
		migrationApplyConfig{
			Version: 2,
			CustomMigration: cdbmutil.CustomMigration{
				Up: func(db webutil.DBInterface) error {
					return nil
				},
				Down: func(db webutil.DBInterface) error {
					return fmt.Errorf("custom migration error")
				},
			},
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
		cdbmutil.DefaultExecCmd,
		cdbmutil.DefaultGetDB,
	)

	if err != nil {
		t.Fatalf(err.Error())
	}

	dbProtcol := cdbmutil.DefaultProtocolMap[cdbmutil.DBProtocol(settings.BaseDatabaseSettings.DatabaseProtocol)]

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

	// Validating successful file up migration
	mApp = &CDBM{
		DB:            db,
		DBProtocolCfg: dbProtcol,
		migrateCfg: migrateState{
			MigrateType: cdbmutil.MigrateTypeUp,
			UpdateQuery: updateQuery,
			LogWriter:   func(err error) {},
			FileMigration: func(mig *migrate.Migrate, version int, mt cdbmutil.MigrationsType) error {
				return nil
			},
		},
	}

	if err = mApp.applyFileMigration(2); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	// Validating file up migration error with successful rollback
	mApp = &CDBM{
		DB:            db,
		DBProtocolCfg: dbProtcol,
		MigrateFlags: MigrateFlagsConfig{
			RollbackOnFailure: true,
		},
		migrateCfg: migrateState{
			MigrateType: cdbmutil.MigrateTypeUp,
			UpdateQuery: updateQuery,
			LogWriter:   func(err error) {},
			FileMigration: func(mig *migrate.Migrate, version int, mt cdbmutil.MigrationsType) error {
				if mt == cdbmutil.MigrateTypeUp {
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

	// Validating file down migration error
	mApp = &CDBM{
		DB:            db,
		DBProtocolCfg: dbProtcol,
		migrateCfg: migrateState{
			MigrateType: cdbmutil.MigrateTypeDown,
			UpdateQuery: updateQuery,
			LogWriter:   func(err error) {},
			FileMigration: func(mig *migrate.Migrate, version int, mt cdbmutil.MigrationsType) error {
				return fmt.Errorf("file migration error")
			},
		},
	}

	if err = mApp.applyFileMigration(2); err == nil {
		t.Errorf("should have error")
	} else if err.Error() != "failed on file down migration for version: '2'" {
		t.Errorf("should have file down migration error; got %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	if _, err = db.DB.Exec(insertQuery, 2, true, cdbmutil.MigrateTypeUp, false); err != nil {
		t.Fatalf(err.Error())
	}

	// Validating migration reset error
	mApp = &CDBM{
		DB:            db,
		DBProtocolCfg: dbProtcol,
		MigrateFlags:  MigrateFlagsConfig{
			//MigrateDownIfDirty: true,
		},
		migrateCfg: migrateState{
			MigrateType: cdbmutil.MigrateTypeUp,
			UpdateQuery: updateQuery,
			LogWriter:   func(err error) {},
			SchemaMigration: schemaMigration{
				SchemaCfg: schemaConfig{
					Dirty: true,
				},
			},
			FileMigration: func(mig *migrate.Migrate, version int, mt cdbmutil.MigrationsType) error {
				return fmt.Errorf("file migration error")
			},
		},
	}

	if err = mApp.applyFileMigration(2); err == nil {
		t.Errorf("should have error")
	} else if err.Error() != "failed of resetting file migration for version '2'" {
		t.Errorf("should have resetting migration error; got %s\n", err.Error())
	}

	var version int

	if err = db.QueryRow(
		`
		select 
			schema_migrations.version
		from
			schema_migrations
		`,
	).Scan(&version); err != nil {
		t.Fatalf(err.Error())
	}

	if version != 2 {
		t.Errorf("should have version '2'; got %d\n", version)
	}

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	if _, err = db.DB.Exec(insertQuery, 2, true, cdbmutil.MigrateTypeUp, false); err != nil {
		t.Fatalf(err.Error())
	}

	// Validating migration reset error
	mApp = &CDBM{
		DB:            db,
		DBProtocolCfg: dbProtcol,
		MigrateFlags:  MigrateFlagsConfig{
			//MigrateDownIfDirty: true,
		},
		migrateCfg: migrateState{
			MigrateType: cdbmutil.MigrateTypeUp,
			UpdateQuery: updateQuery,
			LogWriter:   func(err error) {},
			SchemaMigration: schemaMigration{
				SchemaCfg: schemaConfig{
					Dirty: true,
				},
			},
			FileMigration: func(mig *migrate.Migrate, version int, mt cdbmutil.MigrationsType) error {
				return nil
			},
		},
	}

	if err = mApp.applyFileMigration(2); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	if _, err = db.DB.Exec(insertQuery, 2, true, cdbmutil.MigrateTypeUp, false); err != nil {
		t.Fatalf(err.Error())
	}

	// Validating successful file migration
	mApp = &CDBM{
		DB:            db,
		DBProtocolCfg: dbProtcol,
		migrateCfg: migrateState{
			MigrateType: cdbmutil.MigrateTypeUp,
			UpdateQuery: updateQuery,
			LogWriter:   func(err error) {},
			SchemaMigration: schemaMigration{
				SchemaCfg: schemaConfig{
					Dirty: true,
				},
			},
			FileMigration: func(mig *migrate.Migrate, version int, mt cdbmutil.MigrationsType) error {
				return nil
			},
		},
	}

	if err = mApp.applyFileMigration(2); err != nil {
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
		cdbmutil.DefaultExecCmd,
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
	var customMigrationCalled bool

	// --------------------------------------------------------------------------

	sm = schemaMigration{
		StartingVersion:   1,
		Dirty:             false,
		DirtyState:        &emptyStr,
		IsCustomMigration: false,
		SchemaCfg: schemaConfig{
			NoRows: false,
		},
	}

	mApp = &CDBM{
		DB: db,
		migrateCfg: migrateState{
			TargetVersion:   2,
			LogWriter:       func(err error) {},
			MigrateType:     cdbmutil.MigrateTypeUp,
			InsertQuery:     insertQuery,
			UpdateQuery:     updateQuery,
			SchemaMigration: sm,
			FileMigration: func(mig *migrate.Migrate, version int, mt cdbmutil.MigrationsType) error {
				if version != 2 {
					t.Errorf("should be version 2; got %d", version)
				}

				return nil
			},
		},
	}

	// Validating successful file up migration
	if err = mApp.runMigrationConfigs(
		[]migrationApplyConfig{
			{
				Version: 1,
			},
			{
				Version: 2,
			},
		},
	); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)

	sm = schemaMigration{
		StartingVersion:   1,
		Dirty:             false,
		DirtyState:        &emptyStr,
		IsCustomMigration: true,
		SchemaCfg: schemaConfig{
			NoRows: false,
		},
	}

	mApp = &CDBM{
		DB: db,
		migrateCfg: migrateState{
			TargetVersion:   2,
			LogWriter:       func(err error) {},
			MigrateType:     cdbmutil.MigrateTypeUp,
			InsertQuery:     insertQuery,
			UpdateQuery:     updateQuery,
			SchemaMigration: sm,
			FileMigration: func(mig *migrate.Migrate, version int, mt cdbmutil.MigrationsType) error {
				return nil
			},
		},
	}

	// Validating successful custom up migration
	if err = mApp.runMigrationConfigs(
		[]migrationApplyConfig{
			{
				Version: 1,
			},
			{
				Version: 2,
				CustomMigration: cdbmutil.CustomMigration{
					Up: func(db webutil.DBInterface) error {
						customMigrationCalled = true
						return nil
					},
				},
			},
			{
				Version: 3,
				CustomMigration: cdbmutil.CustomMigration{
					Up: func(db webutil.DBInterface) error {
						t.Errorf("should not have custom migration")
						return nil
					},
					Down: func(db webutil.DBInterface) error {
						t.Errorf("should not have custom migration")
						return nil
					},
				},
			},
		},
	); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}

	if !customMigrationCalled {
		t.Errorf("custom migration bool not called\n")
	}

	// --------------------------------------------------------------------------

	customMigrationCalled = false

	deleteFromSchemaMigration(t, db)

	sm = schemaMigration{
		StartingVersion:   1,
		Dirty:             false,
		DirtyState:        &emptyStr,
		IsCustomMigration: false,
		SchemaCfg: schemaConfig{
			NoRows: false,
		},
	}

	mApp = &CDBM{
		DB: db,
		migrateCfg: migrateState{
			TargetVersion:   2,
			LogWriter:       func(err error) {},
			MigrateType:     cdbmutil.MigrateTypeUp,
			InsertQuery:     insertQuery,
			UpdateQuery:     updateQuery,
			SchemaMigration: sm,
			FileMigration: func(mig *migrate.Migrate, version int, mt cdbmutil.MigrationsType) error {
				fmt.Printf("version: %d\n", version)
				_, innerErr := db.Exec(updateQuery, version-1, false, "", false)
				return innerErr
			},
		},
	}

	// Validating a file up migration error
	if err = mApp.runMigrationConfigs(
		[]migrationApplyConfig{
			{
				Version: 1,
			},
			{
				Version: 2,
				CustomMigration: cdbmutil.CustomMigration{
					Up: func(db webutil.DBInterface) error {
						return fmt.Errorf("custom migration error")
					},
				},
			},
		},
	); err == nil {
		t.Errorf("should have error\n")
	} else if err.Error() != "failed on custom up migration for version: '2'" {
		t.Errorf("should have failed on custom up migration error; got %s\n", err.Error())
	}

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)
	customMigrationCalled = false

	sm = schemaMigration{
		StartingVersion:   3,
		Dirty:             false,
		DirtyState:        &emptyStr,
		IsCustomMigration: true,
	}

	mApp = &CDBM{
		DB: db,
		migrateCfg: migrateState{
			TargetVersion:   2,
			LogWriter:       func(err error) {},
			MigrateType:     cdbmutil.MigrateTypeDown,
			InsertQuery:     insertQuery,
			UpdateQuery:     updateQuery,
			SchemaMigration: sm,
		},
	}

	// Validating a normal custom down migration
	if err = mApp.runMigrationConfigs(
		[]migrationApplyConfig{
			{
				Version: 1,
			},
			{
				Version: 2,
				CustomMigration: cdbmutil.CustomMigration{
					Up: func(db webutil.DBInterface) error {
						t.Errorf("should not be called\n")
						return nil
					},
					Down: func(db webutil.DBInterface) error {
						t.Errorf("should not be called\n")
						return nil
					},
				},
			},
			{
				Version: 3,
				CustomMigration: cdbmutil.CustomMigration{
					Up: func(db webutil.DBInterface) error {
						return nil
					},
					Down: func(db webutil.DBInterface) error {
						customMigrationCalled = true
						return nil
					},
				},
			},
			{
				Version: 4,
				CustomMigration: cdbmutil.CustomMigration{
					Up: func(db webutil.DBInterface) error {
						t.Errorf("should not be called\n")
						return nil
					},
					Down: func(db webutil.DBInterface) error {
						t.Errorf("should not be called\n")
						return nil
					},
				},
			},
		},
	); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}

	if !customMigrationCalled {
		t.Errorf("custom migration bool not called\n")
	}

	// --------------------------------------------------------------------------

	deleteFromSchemaMigration(t, db)
	customMigrationCalled = false

	sm = schemaMigration{
		StartingVersion:   3,
		Dirty:             false,
		DirtyState:        &emptyStr,
		IsCustomMigration: true,
	}

	mApp = &CDBM{
		DB: db,
		migrateCfg: migrateState{
			TargetVersion: 2,
			LogWriter:     func(err error) {},
			MigrateType:   cdbmutil.MigrateTypeDown,
			InsertQuery:   insertQuery,
			UpdateQuery:   updateQuery,
			FileMigration: func(mig *migrate.Migrate, version int, mt cdbmutil.MigrationsType) error {
				return nil
			},
			SchemaMigration: sm,
		},
	}

	// Validating a normal file down migration
	if err = mApp.runMigrationConfigs(
		[]migrationApplyConfig{
			{
				Version: 1,
			},
			{
				Version: 2,
			},
			{
				Version: 3,
			},
		},
	); err != nil {
		t.Errorf("should not have error; got %s\n", err.Error())
	}
}

func TestSuccessfulMigrate(t *testing.T) {
	var err error
	var mApp *CDBM

	settings, err := cdbmutil.GetCDBMUtilSettings("")

	if err != nil {
		t.Fatalf(err.Error())
	}

	dbProtocolCfg := cdbmutil.DefaultProtocolMap[cdbmutil.DBProtocol(settings.BaseDatabaseSettings.DatabaseProtocol)]

	if err != nil {
		t.Fatalf(err.Error())
	}

	settings.DBSetup.FileServerSetup = nil
	settings.DBSetup.BaseSchemaFile = ""

	db, dbName, err := cdbmutil.GetNewDatabase(
		settings,
		cdbmutil.DefaultExecCmd,
		cdbmutil.DefaultGetDB,
	)

	if err != nil {
		t.Fatalf(err.Error())
	}

	dropCmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(settings.DBAction.DropDB, dbName))
	defer dropCmd.Start()

	migrationsDir := "/tmp/migrate-integration/"
	defer os.RemoveAll(migrationsDir)

	var sm schemaMigration
	var file1Up, file1Down, file4Up, file4Down *os.File
	var id int64

	if err = os.RemoveAll(migrationsDir); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
		t.Fatalf(err.Error())
	}

	if file1Up, err = os.Create(migrationsDir + "000001_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if file1Down, err = os.Create(migrationsDir + "000001_update.down.sql"); err != nil {
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

	if file4Up, err = os.Create(migrationsDir + "000004_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if file4Down, err = os.Create(migrationsDir + "000004_update.down.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	file1UpW := bufio.NewWriter(file1Up)

	file1Contents :=
		`
	create table if not exists foo(
		id serial,
		name text not null
	);

	insert into foo(name)
	values('test1');
	`

	if _, err = file1UpW.WriteString(file1Contents); err != nil {
		t.Fatalf(err.Error())
	}

	if err = file1UpW.Flush(); err != nil {
		t.Fatalf(err.Error())
	}

	file1DownW := bufio.NewWriter(file1Down)

	if _, err = file1DownW.WriteString(
		`
		drop table if exists foo;
		`,
	); err != nil {
		t.Fatalf(err.Error())
	}

	if err = file1DownW.Flush(); err != nil {
		t.Fatalf(err.Error())
	}

	cmMap := map[int]cdbmutil.CustomMigration{
		2: {
			Up: func(db webutil.DBInterface) error {
				_, innerErr := db.Exec(
					`
					insert into foo(name)
					values('test2');
					`,
				)
				return innerErr
			},
			Down: func(db webutil.DBInterface) error {
				_, innerErr := db.Exec(
					`
					delete from foo where name = 'test2';
					`,
				)
				return innerErr
			},
		},
		3: {
			Up: func(db webutil.DBInterface) error {
				_, innerErr := db.Exec(
					`
					insert into foo(name)
					values('test3');
					`,
				)
				return innerErr
			},
			Down: func(db webutil.DBInterface) error {
				_, innerErr := db.Exec(
					`
					delete from foo where name = 'test3';
					`,
				)
				return innerErr
			},
		},
	}

	file3UpW := bufio.NewWriter(file4Up)

	if _, err = file3UpW.WriteString(
		`
		insert into foo(name)
		values('test4');
		`,
	); err != nil {
		t.Fatalf(err.Error())
	}

	if err = file3UpW.Flush(); err != nil {
		t.Fatalf(err.Error())
	}

	file3DownW := bufio.NewWriter(file4Down)

	if _, err = file3DownW.WriteString(
		`
		delete from foo where name = 'test4';
		`,
	); err != nil {
		t.Fatalf(err.Error())
	}

	if err = file3DownW.Flush(); err != nil {
		t.Fatalf(err.Error())
	}

	// --------------------------------------------------------------------------

	if mApp, err = NewTestCDBM(
		db,
		dbProtocolCfg.DBProtocol,
		MigrateFlagsConfig{
			MigrationsProtocol: cdbmutil.FileProtocol,
			MigrationsDir:      migrationsDir,
			TargetVersion:      1,
		}); err != nil {
		t.Fatalf(err.Error())
	}

	if err = mApp.Migrate(
		cdbmutil.DefaultGetMigrationFunc,
		cdbmutil.DefaultFileMigrationFunc,
		cmMap,
	); err != nil {
		t.Fatalf("should not have error; got %+v\n", err)
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

	if err = db.QueryRowx("select id from foo where name = 'test1'").Scan(&id); err != nil {
		t.Errorf(err.Error())
	}

	// --------------------------------------------------------------------------

	if mApp, err = NewTestCDBM(
		db,
		dbProtocolCfg.DBProtocol,
		MigrateFlagsConfig{
			MigrationsProtocol: cdbmutil.FileProtocol,
			MigrationsDir:      migrationsDir,
			TargetVersion:      2,
		}); err != nil {
		t.Fatalf(err.Error())
	}

	if err = mApp.Migrate(
		cdbmutil.DefaultGetMigrationFunc,
		cdbmutil.DefaultFileMigrationFunc,
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

	if err = db.QueryRowx("select id from foo where name = 'test2'").
		Scan(&id); err != nil {
		t.Errorf(err.Error())
	}

	// --------------------------------------------------------------------------

	if mApp, err = NewTestCDBM(
		db,
		dbProtocolCfg.DBProtocol,
		MigrateFlagsConfig{
			MigrationsProtocol: cdbmutil.FileProtocol,
			MigrationsDir:      migrationsDir,
			TargetVersion:      3,
		}); err != nil {
		t.Fatalf(err.Error())
	}

	if err = mApp.Migrate(
		cdbmutil.DefaultGetMigrationFunc,
		cdbmutil.DefaultFileMigrationFunc,
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
	if !sm.IsCustomMigration {
		t.Errorf("should be custom")
	}

	if err = db.QueryRowx("select id from foo where name = 'test3'").
		Scan(&id); err != nil {
		t.Errorf(err.Error())
	}

	// --------------------------------------------------------------------------

	if mApp, err = NewTestCDBM(
		db,
		dbProtocolCfg.DBProtocol,
		MigrateFlagsConfig{
			MigrationsProtocol: cdbmutil.FileProtocol,
			MigrationsDir:      migrationsDir,
			TargetVersion:      4,
		}); err != nil {
		t.Fatalf(err.Error())
	}

	if err = mApp.Migrate(
		cdbmutil.DefaultGetMigrationFunc,
		cdbmutil.DefaultFileMigrationFunc,
		cmMap,
	); err != nil {
		t.Errorf("should not have error; got %+v\n", err)
	}

	sm = getSchemaMigration(t, db)

	if sm.StartingVersion != 4 {
		t.Errorf("version should be 4; got %d\n", sm.StartingVersion)
	}
	if sm.Dirty {
		t.Errorf("should not be dirty")
	}
	if sm.IsCustomMigration {
		t.Errorf("should not be custom")
	}

	if err = db.QueryRowx("select id from foo where name = 'test4'").
		Scan(&id); err != nil {
		t.Errorf(err.Error())
	}

	// --------------------------------------------------------------------------

	if mApp, err = NewTestCDBM(
		db,
		dbProtocolCfg.DBProtocol,
		MigrateFlagsConfig{
			MigrationsProtocol: cdbmutil.FileProtocol,
			MigrationsDir:      migrationsDir,
			TargetVersion:      3,
		}); err != nil {
		t.Fatalf(err.Error())
	}

	if err = mApp.Migrate(
		cdbmutil.DefaultGetMigrationFunc,
		cdbmutil.DefaultFileMigrationFunc,
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

	if err = db.QueryRowx("select id from foo where name = 'test3'").
		Scan(&id); err != nil {
		t.Errorf(err.Error())
	}

	// --------------------------------------------------------------------------

	if mApp, err = NewTestCDBM(
		db,
		dbProtocolCfg.DBProtocol,
		MigrateFlagsConfig{
			MigrationsProtocol: cdbmutil.FileProtocol,
			MigrationsDir:      migrationsDir,
			TargetVersion:      2,
		}); err != nil {
		t.Fatalf(err.Error())
	}

	if err = mApp.Migrate(
		cdbmutil.DefaultGetMigrationFunc,
		cdbmutil.DefaultFileMigrationFunc,
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

	if err = db.QueryRowx("select id from foo where name = 'test2'").
		Scan(&id); err != nil {
		t.Errorf(err.Error())
	}

	// --------------------------------------------------------------------------

	if mApp, err = NewTestCDBM(
		db,
		dbProtocolCfg.DBProtocol,
		MigrateFlagsConfig{
			MigrationsProtocol: cdbmutil.FileProtocol,
			MigrationsDir:      migrationsDir,
			TargetVersion:      1,
		}); err != nil {
		t.Fatalf(err.Error())
	}

	if err = mApp.Migrate(
		cdbmutil.DefaultGetMigrationFunc,
		cdbmutil.DefaultFileMigrationFunc,
		cmMap,
	); err != nil {
		t.Fatalf("should not have error; got %+v\n", err)
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

	if err = db.QueryRowx("select id from foo where name = 'test1'").Scan(&id); err != nil {
		t.Errorf(err.Error())
	}

	// --------------------------------------------------------------------------

	if mApp, err = NewTestCDBM(
		db,
		dbProtocolCfg.DBProtocol,
		MigrateFlagsConfig{
			MigrationsProtocol: cdbmutil.FileProtocol,
			MigrationsDir:      migrationsDir,
			TargetVersion:      0,
		}); err != nil {
		t.Fatalf(err.Error())
	}

	if err = mApp.Migrate(
		cdbmutil.DefaultGetMigrationFunc,
		cdbmutil.DefaultFileMigrationFunc,
		cmMap,
	); err != nil {
		t.Fatalf("should not have error; got %+v\n", err)
	}

	var version int

	err = db.QueryRowx("select version from schema_migrations").Scan(&version)

	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("should not have any records in schema_migrations at this point\n")
	}

	// --------------------------------------------------------------------------

	////////////////////////////////////////////////////////////
	// Here we begin testing for dirty state and making sure
	// our migrations will properly migrate down first before
	// migrating up
	////////////////////////////////////////////////////////////

	if mApp, err = NewTestCDBM(
		db,
		dbProtocolCfg.DBProtocol,
		MigrateFlagsConfig{
			MigrationsProtocol: cdbmutil.FileProtocol,
			MigrationsDir:      migrationsDir,
			TargetVersion:      3,
		}); err != nil {
		t.Fatalf(err.Error())
	}

	invalidCmMap := map[int]cdbmutil.CustomMigration{
		2: {
			Up: func(db webutil.DBInterface) error {
				_, innerErr := db.Exec(
					`
					insert into foo(name)
					values('test2');
					`,
				)
				return innerErr
			},
			Down: func(db webutil.DBInterface) error {
				_, innerErr := db.Exec(
					`
					delete from foo where name = 'test2';
					`,
				)
				return innerErr
			},
		},
		3: {
			Up: func(db webutil.DBInterface) error {
				return fmt.Errorf("some error")
			},
			Down: func(db webutil.DBInterface) error {
				_, innerErr := db.Exec(
					`
					delete from foo where name = 'test3';
					`,
				)
				return innerErr
			},
		},
	}

	if err = mApp.Migrate(
		cdbmutil.DefaultGetMigrationFunc,
		cdbmutil.DefaultFileMigrationFunc,
		invalidCmMap,
	); err == nil {
		t.Fatalf("should have error")
	} else if !strings.Contains(err.Error(), "failed on custom up migration for version: '3'") {
		t.Fatalf("should have failed on custom up migration; got %s", err.Error())
	}

	sm = getSchemaMigration(t, db)

	if sm.StartingVersion != 3 {
		t.Errorf("version should be 3; got %d\n", sm.StartingVersion)
	}
	if !sm.Dirty {
		t.Errorf("should be dirty")
	}
	if !sm.IsCustomMigration {
		t.Errorf("should be custom")
	}
	if sm.DirtyState != nil {
		if *sm.DirtyState != string(cdbmutil.MigrateTypeUp) {
			t.Errorf("should have dirty state 'up'; got %s\n", *sm.DirtyState)
		}
	} else {
		t.Errorf("should have dirty state 'up'")
	}

	// --------------------------------------------------------------------------

	if mApp, err = NewTestCDBM(
		db,
		dbProtocolCfg.DBProtocol,
		MigrateFlagsConfig{
			MigrationsProtocol: cdbmutil.FileProtocol,
			MigrationsDir:      migrationsDir,
			TargetVersion:      4,
			ResetDirtyFlag:     true,
		}); err != nil {
		t.Fatalf(err.Error())
	}

	// Here we are testing our migration dirty state table
	// that we first do a down custom migration before applying up migrations
	if err = mApp.Migrate(
		cdbmutil.DefaultGetMigrationFunc,
		cdbmutil.DefaultFileMigrationFunc,
		cmMap,
	); err != nil {
		t.Fatalf("should not have error; got %+v\n", err)
	}

	sm = getSchemaMigration(t, db)

	if sm.StartingVersion != 4 {
		t.Errorf("version should be 4; got %d\n", sm.StartingVersion)
	}
	if sm.Dirty {
		t.Errorf("should not be dirty")
	}
	if sm.IsCustomMigration {
		t.Errorf("should not be custom")
	}

	// --------------------------------------------------------------------------

	if mApp, err = NewTestCDBM(
		db,
		dbProtocolCfg.DBProtocol,
		MigrateFlagsConfig{
			MigrationsProtocol: cdbmutil.FileProtocol,
			MigrationsDir:      migrationsDir,
			TargetVersion:      0,
		}); err != nil {
		t.Fatalf(err.Error())
	}

	if err = mApp.Migrate(
		cdbmutil.DefaultGetMigrationFunc,
		cdbmutil.DefaultFileMigrationFunc,
		cmMap,
	); err != nil {
		t.Fatalf("should not have error; got %+v\n", err)
	}

	err = db.QueryRowx("select version from schema_migrations").Scan(&version)

	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("should not have any records in schema_migrations at this point\n")
	}

	// --------------------------------------------------------------------------

	if _, err = file1UpW.WriteString("invalid migration"); err != nil {
		t.Fatalf("%+v", err.Error())
	}

	if err = file1UpW.Flush(); err != nil {
		t.Fatalf("%+v", err.Error())
	}

	if mApp, err = NewTestCDBM(
		db,
		dbProtocolCfg.DBProtocol,
		MigrateFlagsConfig{
			MigrationsProtocol: cdbmutil.FileProtocol,
			MigrationsDir:      migrationsDir,
			TargetVersion:      1,
		}); err != nil {
		t.Fatalf(err.Error())
	}

	if err = mApp.Migrate(
		cdbmutil.DefaultGetMigrationFunc,
		cdbmutil.DefaultFileMigrationFunc,
		cmMap,
	); err == nil {
		t.Fatalf("should have error")
	} else if !strings.Contains(err.Error(), "failed on file up migration") {
		t.Errorf("should have file up migration error; got %s\n", err.Error())
	}

	sm = getSchemaMigration(t, db)

	if sm.StartingVersion != 1 {
		t.Errorf("version should be 1; got %d\n", sm.StartingVersion)
	}
	if !sm.Dirty {
		t.Errorf("should be dirty")
	}
	if sm.IsCustomMigration {
		t.Errorf("should not be custom")
	}
	if sm.DirtyState != nil {
		if *sm.DirtyState != string(cdbmutil.MigrateTypeUp) {
			t.Errorf("should have dirty state 'up'; got %s\n", *sm.DirtyState)
		}
	} else {
		t.Errorf("should have dirty state 'up'")
	}

	// --------------------------------------------------------------------------

	if err = os.RemoveAll(migrationsDir + "000001_update.up.sql"); err != nil {
		t.Fatalf("%+v", err)
	}

	if file1Up, err = os.Create(migrationsDir + "000001_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	file1UpW.Reset(file1Up)

	if _, err = file1UpW.WriteString(file1Contents); err != nil {
		t.Fatalf("%+v", err)
	}

	if err = file1UpW.Flush(); err != nil {
		t.Fatalf("%+v", err)
	}

	if mApp, err = NewTestCDBM(
		db,
		dbProtocolCfg.DBProtocol,
		MigrateFlagsConfig{
			MigrationsProtocol: cdbmutil.FileProtocol,
			MigrationsDir:      migrationsDir,
			TargetVersion:      -1,
			ResetDirtyFlag:     true,
		}); err != nil {
		t.Fatalf(err.Error())
	}

	if err = mApp.Migrate(
		cdbmutil.DefaultGetMigrationFunc,
		cdbmutil.DefaultFileMigrationFunc,
		cmMap,
	); err != nil {
		t.Fatalf("should not have error; got %+v\n", err)
	}

	sm = getSchemaMigration(t, db)

	if sm.StartingVersion != 4 {
		t.Errorf("version should be 1; got %d\n", sm.StartingVersion)
	}
	if sm.Dirty {
		t.Errorf("should not be dirty")
	}
	if sm.IsCustomMigration {
		t.Errorf("should not be custom")
	}
}

func TestFoobar(t *testing.T) {
	var err error
	var mApp *CDBM

	settings, err := cdbmutil.GetCDBMUtilSettings("")

	if err != nil {
		t.Fatalf(err.Error())
	}

	dbProtocolCfg := cdbmutil.DefaultProtocolMap[cdbmutil.DBProtocol(settings.BaseDatabaseSettings.DatabaseProtocol)]

	if err != nil {
		t.Fatalf(err.Error())
	}

	settings.DBSetup.FileServerSetup = nil
	settings.DBSetup.BaseSchemaFile = ""

	db, dbName, err := cdbmutil.GetNewDatabase(
		settings,
		cdbmutil.DefaultExecCmd,
		cdbmutil.DefaultGetDB,
	)

	if err != nil {
		t.Fatalf(err.Error())
	}

	dropCmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(settings.DBAction.DropDB, dbName))
	defer dropCmd.Start()

	migrationsDir := "/tmp/migrate-integration/"
	defer os.RemoveAll(migrationsDir)

	var sm schemaMigration
	var file1Up, file1Down, file4Up, file4Down *os.File

	if err = os.RemoveAll(migrationsDir); err != nil {
		t.Fatalf(err.Error())
	}

	if err = os.MkdirAll(migrationsDir, os.ModePerm); err != nil {
		t.Fatalf(err.Error())
	}

	if file1Up, err = os.Create(migrationsDir + "000001_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if file1Down, err = os.Create(migrationsDir + "000001_update.down.sql"); err != nil {
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

	if file4Up, err = os.Create(migrationsDir + "000004_update.up.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	if file4Down, err = os.Create(migrationsDir + "000004_update.down.sql"); err != nil {
		t.Fatalf(err.Error())
	}

	file1UpW := bufio.NewWriter(file1Up)

	if _, err = file1UpW.WriteString(
		`
		invalid migration
		`,
	); err != nil {
		t.Fatalf(err.Error())
	}

	if err = file1UpW.Flush(); err != nil {
		t.Fatalf(err.Error())
	}

	file1DownW := bufio.NewWriter(file1Down)

	if _, err = file1DownW.WriteString(
		`
		drop table if exists foo;
		`,
	); err != nil {
		t.Fatalf(err.Error())
	}

	if err = file1DownW.Flush(); err != nil {
		t.Fatalf(err.Error())
	}

	cmMap := map[int]cdbmutil.CustomMigration{
		2: {
			Up: func(db webutil.DBInterface) error {
				_, innerErr := db.Exec(
					`
					insert into foo(name)
					values('test2');
					`,
				)
				return innerErr
			},
			Down: func(db webutil.DBInterface) error {
				_, innerErr := db.Exec(
					`
					delete from foo where name = 'test2';
					`,
				)
				return innerErr
			},
		},
		3: {
			Up: func(db webutil.DBInterface) error {
				_, innerErr := db.Exec(
					`
					insert into foo(name)
					values('test3');
					`,
				)
				return innerErr
			},
			Down: func(db webutil.DBInterface) error {
				_, innerErr := db.Exec(
					`
					delete from foo where name = 'test3';
					`,
				)
				return innerErr
			},
		},
	}

	file3UpW := bufio.NewWriter(file4Up)

	if _, err = file3UpW.WriteString(
		`
		insert into foo(name)
		values('test4');
		`,
	); err != nil {
		t.Fatalf(err.Error())
	}

	if err = file3UpW.Flush(); err != nil {
		t.Fatalf(err.Error())
	}

	file3DownW := bufio.NewWriter(file4Down)

	if _, err = file3DownW.WriteString(
		`
		delete from foo where name = 'test4';
		`,
	); err != nil {
		t.Fatalf(err.Error())
	}

	if err = file3DownW.Flush(); err != nil {
		t.Fatalf(err.Error())
	}

	// --------------------------------------------------------------------------

	if mApp, err = NewTestCDBM(
		db,
		dbProtocolCfg.DBProtocol,
		MigrateFlagsConfig{
			MigrationsProtocol: cdbmutil.FileProtocol,
			MigrationsDir:      migrationsDir,
			TargetVersion:      1,
		}); err != nil {
		t.Fatalf(err.Error())
	}

	if err = mApp.Migrate(
		cdbmutil.DefaultGetMigrationFunc,
		cdbmutil.DefaultFileMigrationFunc,
		cmMap,
	); err == nil {
		t.Fatalf("should have error")
	} else if !strings.Contains(err.Error(), "failed on file up migration") {
		t.Errorf("should have file up migration error; got %s\n", err.Error())
	}

	sm = getSchemaMigration(t, db)

	if sm.StartingVersion != 1 {
		t.Errorf("version should be 1; got %d\n", sm.StartingVersion)
	}
	if !sm.Dirty {
		t.Errorf("should be dirty")
	}
	if sm.IsCustomMigration {
		t.Errorf("should not be custom")
	}
	if sm.DirtyState != nil {
		if *sm.DirtyState != string(cdbmutil.MigrateTypeUp) {
			t.Errorf("should have dirty state 'up'; got %s\n", *sm.DirtyState)
		}
	} else {
		t.Errorf("should have dirty state 'up'")
	}
}
