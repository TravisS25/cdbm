package app

import (
	"fmt"
	"os/exec"

	"github.com/TravisS25/cdbm/cdbmutil"
)

func ExampleCDBM_Status_a() {
	utilSettings, err := cdbmutil.GetCDBMUtilSettings("")

	if err != nil {
		fmt.Printf(err.Error())
		return
	}

	settings, err := GetCDBMConfig("")

	if err != nil {
		fmt.Printf(err.Error())
		return
	}

	utilSettings.DBSetup.FileServerSetup = nil
	utilSettings.DBSetup.BaseSchemaFile = ""

	db, dbName, err := cdbmutil.GetNewDatabase(
		utilSettings,
		DefaultExecCmd,
		cdbmutil.DefaultGetDB,
	)

	if err != nil {
		fmt.Printf(err.Error())
		return
	}

	defer exec.Command("/bin/sh", "-c", fmt.Sprintf(utilSettings.DBAction.DropDB, dbName)).Start()

	cdbm := &CDBM{
		DB:            db,
		DBProtocolCfg: DefaultProtocolMap[DBProtocol(settings.RootFlags.DBProtocol)],
	}

	if err = cdbm.Status(); err != nil {
		fmt.Printf(err.Error())
		return
	}

	// Output: No migration entry
}

func ExampleCDBM_Status_b() {
	utilSettings, err := cdbmutil.GetCDBMUtilSettings("")

	if err != nil {
		fmt.Printf(err.Error())
		return
	}

	settings, err := GetCDBMConfig("")

	if err != nil {
		fmt.Printf(err.Error())
		return
	}

	utilSettings.DBSetup.FileServerSetup = nil
	utilSettings.DBSetup.BaseSchemaFile = ""

	db, dbName, err := cdbmutil.GetNewDatabase(
		utilSettings,
		DefaultExecCmd,
		cdbmutil.DefaultGetDB,
	)

	if err != nil {
		fmt.Printf(err.Error())
		return
	}

	defer exec.Command("/bin/sh", "-c", fmt.Sprintf(utilSettings.DBAction.DropDB, dbName)).Start()

	cdbm := &CDBM{
		DB:            db,
		DBProtocolCfg: DefaultProtocolMap[DBProtocol(settings.RootFlags.DBProtocol)],
	}

	if _, err = db.Exec(
		`
		CREATE TABLE public.schema_migrations (
			version INT8 NOT NULL primary key,
			dirty boolean not null,
			dirty_state text
		);
		`,
	); err != nil {
		fmt.Printf(err.Error())
		return
	}

	if _, err = db.Exec(
		`
		insert into schema_migrations(version, dirty, dirty_state)
		values(2, true, 'Up');
		`,
	); err != nil {
		fmt.Printf(err.Error())
		return
	}

	if err = cdbm.Status(); err != nil {
		fmt.Printf(err.Error())
		return
	}

	// Output: migration state - version:2 / dirty:true / dirty state:Up
}
