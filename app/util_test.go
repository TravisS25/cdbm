package app

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/TravisS25/cdbm/cdbmutil"
	"github.com/TravisS25/webutil/webutil"
)

func TestAppendValuesQuery(t *testing.T) {
	var err error

	utilSettings, err := cdbmutil.GetCDBMUtilSettings("")

	if err != nil {
		t.Fatalf(err.Error())
	}

	utilSettings.DBSetup.FileServerSetup = nil
	utilSettings.DBSetup.BaseSchemaFile = ""

	db, dbName, err := cdbmutil.GetNewDatabase(
		utilSettings,
		cdbmutil.DefaultExecCmd,
		cdbmutil.DefaultGetDB,
	)

	if err != nil {
		t.Fatalf(err.Error())
	}

	dropCmd := exec.Command(
		"/bin/sh",
		"-c",
		fmt.Sprintf(utilSettings.DBAction.DropDB, dbName),
	)
	defer dropCmd.Start()

	if _, err = db.Exec(
		`
		create table foo(
			id serial,
			name text not null,
			migration_complete boolean
		);

		insert into foo(name)
		values ('test1'), ('test2'), ('test3'), ('test4');
		
		create table bar(
			id serial,
			name text not null
		);
		`,
	); err != nil {
		t.Fatalf(err.Error())
	}

	insertQuery := "insert into bar(name) values "
	updateQuery := "update foo set migration_complete = true where id in (?)"
	valuesQuery := ""
	sqlBindVar := cdbmutil.DefaultProtocolMap[cdbmutil.DBProtocol(utilSettings.BaseDatabaseSettings.DatabaseProtocol)].SQLBindVar
	counter := uint(0)

	fooRows, err := db.Queryx("select foo.name from foo")

	if err != nil {
		t.Fatalf(err.Error())
	}

	for fooRows.Next() {
		var name string

		if err = fooRows.Scan(&name); err != nil {
			t.Fatalf(err.Error())
		}

		valuesQuery += fmt.Sprintf("('%s')", name)

		if err = AppendValuesQuery(
			insertQuery,
			utilSettings.InsertEndQuery,
			updateQuery,
			sqlBindVar,
			3,
			&valuesQuery,
			&counter,
			false,
			db,
		); err != nil {
			t.Fatalf(err.Error())
		}
	}

	barIDs, err := webutil.QuerySingleColumn(
		db,
		sqlBindVar,
		`
		select
			bar.id
		from
			bar
		`,
	)

	if err != nil {
		t.Fatalf(err.Error())
	}

	if len(barIDs) != 3 {
		t.Errorf("should have length of 3 for ids; got %d", len(barIDs))
	}

	if err = AppendValuesQuery(
		insertQuery,
		utilSettings.InsertEndQuery,
		updateQuery,
		sqlBindVar,
		3,
		&valuesQuery,
		&counter,
		true,
		db,
	); err != nil {
		t.Fatalf(err.Error())
	}

	if barIDs, err = webutil.QuerySingleColumn(
		db,
		sqlBindVar,
		`
		select
			bar.id
		from
			bar
		`,
	); err != nil {
		t.Fatalf(err.Error())
	}

	if len(barIDs) != 4 {
		t.Errorf("should have length of 4 for ids; got %d", len(barIDs))
	}

	// -------------------------------------------------------------------------------

	if _, err = db.Exec("delete from bar;"); err != nil {
		t.Fatalf(err.Error())
	}

	valuesQuery = ""
	rowCounter := 0
	bulkInsert := 4

	for {
		fooRows, err = db.Queryx("select foo.name from foo where migration_complete != true")

		if err != nil {
			t.Fatalf(err.Error())
		}

		for fooRows.Next() {
			rowCounter++
			var name string

			if err = fooRows.Scan(&name); err != nil {
				t.Fatalf(err.Error())
			}

			valuesQuery += fmt.Sprintf("('%s')", name)

			if err = AppendValuesQuery(
				insertQuery,
				utilSettings.InsertEndQuery,
				updateQuery,
				sqlBindVar,
				uint(bulkInsert),
				&valuesQuery,
				&counter,
				false,
				db,
			); err != nil {
				t.Fatalf(err.Error())
			}
		}

		if rowCounter == bulkInsert {
			rowCounter = 0
			continue
		}

		break
	}

	if err = AppendValuesQuery(
		insertQuery,
		utilSettings.InsertEndQuery,
		updateQuery,
		sqlBindVar,
		uint(bulkInsert),
		&valuesQuery,
		&counter,
		true,
		db,
	); err != nil {
		t.Fatalf(err.Error())
	}

	t.Errorf("value query: %s", valuesQuery)
}
