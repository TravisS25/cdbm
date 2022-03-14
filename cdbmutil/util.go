package cdbmutil

import (
	"bytes"
	"database/sql"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/TravisS25/webutil/webutil"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/cockroachdb"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

// GetCDBMUtilSettings retrieves CDBMUtilSettings based on envVar parameter passed
//
// If envVar is empty string, then CDBM_UTIL_CONFIG is used as default
func GetCDBMUtilSettings(envVar string) (CDBMUtilSettings, error) {
	var settings CDBMUtilSettings
	var err error
	var envUsed string

	if envVar != "" {
		envUsed = os.Getenv(envVar)
	} else {
		envUsed = os.Getenv(CDBM_UTIL_CONFIG)
	}

	viper.SetConfigFile(envUsed)

	if err = viper.ReadInConfig(); err != nil {
		return CDBMUtilSettings{}, errors.WithStack(err)
	}

	if viper.Unmarshal(&settings); err != nil {
		return CDBMUtilSettings{}, errors.WithStack(err)
	}

	return settings, nil
}

// GetNewDatabase will retrieve instance of sqlx.DB along with database name based on settings passed
func GetNewDatabase(
	testSettings CDBMUtilSettings,
	cmdFunc func(*exec.Cmd) error,
	getDBFunc func(BaseDatabaseSettings) (*sqlx.DB, error),
) (*sqlx.DB, string, error) {
	var err error

	dbName := GetRandomString(10)
	createCmd := exec.Command(
		"/bin/sh",
		"-c",
		fmt.Sprintf(testSettings.DBAction.CreateDB, dbName),
	)
	stdErr := &bytes.Buffer{}
	createCmd.Stderr = stdErr

	if err = cmdFunc(createCmd); err != nil {
		return nil, "", errors.WithStack(fmt.Errorf(stdErr.String()))
	}

	hasError := func(passedErr error) bool {
		if passedErr != nil {
			dropCmd := exec.Command(
				"/bin/sh",
				"-c",
				fmt.Sprintf(testSettings.DBAction.DropDB, dbName),
			)
			cmdFunc(dropCmd)
			return true
		}

		return false
	}

	baseDBSettings := testSettings.BaseDatabaseSettings
	baseDBSettings.Settings.DBName = dbName

	db, err := getDBFunc(baseDBSettings)

	if hasError(err) {
		return nil, "", err
	}

	if testSettings.DBSetup.FileServerSetup != nil {
		fs := http.FileServer(http.Dir(testSettings.DBSetup.FileServerSetup.BaseSchemaDir))

		fmt.Printf("dir: %s\n", testSettings.DBSetup.FileServerSetup.BaseSchemaDir)

		go func() {
			http.ListenAndServe(testSettings.DBSetup.FileServerSetup.FileServerURL, fs)
		}()

		for _, key := range testSettings.DBAction.Import.ImportKeys {
			cmdStr, ok := testSettings.DBAction.Import.ImportMap[key]

			if !ok {
				return nil, "", fmt.Errorf("file import key '%s' does not exist", key)
			}

			importCmd := exec.Command(
				"/bin/sh",
				"-c",
				fmt.Sprintf(
					cmdStr,
					dbName,
					testSettings.DBSetup.FileServerSetup.FileServerURL,
				),
			)
			importCmd.Stderr = stdErr

			fmt.Printf("importcmd: %s\n", importCmd)

			if hasError(cmdFunc(importCmd)) {
				return nil, "", errors.WithStack(fmt.Errorf(stdErr.String()))
			}
		}
	} else if testSettings.DBSetup.BaseSchemaFile != "" {
		for _, key := range testSettings.DBAction.Import.ImportKeys {
			cmdStr, ok := testSettings.DBAction.Import.ImportMap[key]

			if !ok {
				return nil, "", fmt.Errorf("file import key '%s' does not exist", key)
			}

			importCmd := exec.Command(
				"/bin/sh",
				"-c",
				fmt.Sprintf(
					cmdStr,
					dbName,
					testSettings.DBSetup.BaseSchemaFile,
				),
			)
			importCmd.Stderr = stdErr

			if hasError(cmdFunc(importCmd)) {
				return nil, "", errors.WithStack(fmt.Errorf(stdErr.String()))
			}
		}
	}

	return db, dbName, nil
}

// GetMigrationSetupTeardown will retrieve instance of sqlx.DB along with function, that when called,
// will delete the currently created database
func GetMigrationSetupTeardown(
	cdbmUtilEnvVar string,
	execCmd func(*exec.Cmd) error,
	getDB func(BaseDatabaseSettings) (*sqlx.DB, error),
	importKeys []string,
) (MigrationSetupTeardownReturn, error) {
	utilSettings, err := GetCDBMUtilSettings(cdbmUtilEnvVar)

	if err != nil {
		return MigrationSetupTeardownReturn{}, errors.WithStack(err)
	}

	utilSettings.DBAction.Import.ImportKeys = importKeys

	db, dbName, err := GetNewDatabase(
		utilSettings,
		execCmd,
		getDB,
	)

	if err != nil {
		return MigrationSetupTeardownReturn{}, errors.WithStack(err)
	}

	dropDB := func() {
		dropCmd := exec.Command(
			"/bin/sh",
			"-c",
			fmt.Sprintf(utilSettings.DBAction.DropDB, dbName),
		)
		dropCmd.Start()
	}

	return MigrationSetupTeardownReturn{
		DB:       db,
		Settings: utilSettings,
		TearDown: dropDB,
	}, nil
}

// GetRandomString generates random string based on length passed
func GetRandomString(length int) string {
	allowedChars := "abcdefghijklmnopqrstuvwxyz"
	size := len(allowedChars)
	b := make([]byte, length)
	rand.Seed(time.Now().UnixNano())

	for i := range b {
		c := rand.Intn(size)
		b[i] = allowedChars[c]
	}

	return string(b)
}

// GetDatabaseDriver retrieves database driver for migrate based on parameters passed
func GetDatabaseDriver(db *sql.DB, protcol DBProtocol, cfg interface{}) (database.Driver, error) {
	var ok bool

	switch protcol {
	case PostgresProtocol:
		if cfg == nil {
			return postgres.WithInstance(db, &postgres.Config{})
		}

		if _, ok = cfg.(*postgres.Config); !ok {
			return nil, fmt.Errorf("config must be type *postgres.Config")
		}

		return postgres.WithInstance(db, cfg.(*postgres.Config))
	case CockroachdbProtocol:
		if cfg == nil {
			return cockroachdb.WithInstance(db, &cockroachdb.Config{})
		}

		if _, ok = cfg.(*cockroachdb.Config); !ok {
			return nil, fmt.Errorf("config must be type *cockroachdb.Config")
		}

		return cockroachdb.WithInstance(db, cfg.(*cockroachdb.Config))
	default:
		return nil, fmt.Errorf("invalid db protocol")
	}
}

// MigrationInsertAndUpdateTable is util function that should take in a bulk insert query that returns
// the ids of all the inserts and will also execute multiple update queries based on the returned ids if passed
//
// updateQueries parameter can be nil
func MigrationInsertAndUpdateTable(
	db webutil.QuerierExec,
	sqlBindVar int,
	insertQuery string,
	updateQueries []string,
) ([]interface{}, error) {
	ids, err := webutil.QuerySingleColumn(
		db,
		sqlBindVar,
		insertQuery,
	)

	if err != nil {
		return nil, errors.WithStack(err)
	}

	for _, query := range updateQueries {
		var updateQuery string

		if updateQuery, _, err = webutil.InQueryRebind(
			sqlBindVar,
			query,
			ids,
		); err != nil {
			return nil, errors.WithStack(err)
		}

		if _, err = db.Exec(updateQuery, ids); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	return ids, nil
}
