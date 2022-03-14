package app

import (
	"os"
	"testing"

	"github.com/TravisS25/cdbm/cdbmutil"
)

func TestDrop(t *testing.T) {
	returnCfg, err := cdbmutil.GetMigrationSetupTeardown(
		cdbmutil.CDBM_UTIL_CONFIG,
		cdbmutil.DefaultExecCmd,
		cdbmutil.DefaultGetDB,
		[]string{"base_schema"},
	)

	if err != nil {
		t.Fatalf("%+v", err)
	}

	defer returnCfg.TearDown()

	rootDir := "/tmp/migrate-drop/"
	migrationsDir := rootDir + "migrations/"

	cdbm, err := NewTestCDBM(
		returnCfg.DB,
		cdbmutil.DBProtocol(returnCfg.Settings.BaseDatabaseSettings.DatabaseProtocol),
		MigrateFlagsConfig{
			MigrationsDir:      migrationsDir,
			MigrationsProtocol: cdbmutil.FileProtocol,
		},
	)

	if err != nil {
		t.Fatalf(err.Error())
	}

	defer os.RemoveAll(migrationsDir)

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

	if err = cdbm.Drop(); err != nil {
		t.Fatalf("%+v", err)
	}
}
