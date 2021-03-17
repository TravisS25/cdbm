package app

// func TestDrop(t *testing.T) {
// 	utilSettings, err := cdbmutil.GetCDBMUtilSettings("")

// 	if err != nil {
// 		t.Fatalf(err.Error())
// 		return
// 	}

// 	utilSettings.DBSetup.FileServerSetup = nil
// 	utilSettings.DBSetup.BaseSchemaFile = ""

// 	db, dbName, err := cdbmutil.GetNewDatabase(
// 		utilSettings,
// 		DefaultExecCmd,
// 		cdbmutil.DefaultGetDB,
// 	)

// 	if err != nil {
// 		t.Fatalf(err.Error())
// 		return
// 	}

// 	defer exec.Command("/bin/sh", "-c", fmt.Sprintf(utilSettings.DBAction.DropDB, dbName)).Start()

// 	rootDir := "/tmp/migrate-drop/"
// 	migrationsDir := rootDir + "migrations/"

// 	cdbm := &CDBM{
// 		DB: db,
// 		MigrateFlags: MigrateFlagsConfig{
// 			MigrationsDir: migrationsDir,
// 		},
// 		DBProtocolCfg: DefaultProtocolMap[DBProtocol(utilSettings.BaseDatabaseSettings.DatabaseProtocol)],
// 	}

// 	defer os.RemoveAll(migrationsDir)

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

// 	if err = cdbm.Drop(); err != nil {
// 		t.Fatalf(err.Error())
// 	}
// }
