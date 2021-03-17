package cdbmutil

import (
	"bytes"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/TravisS25/webutil/webutil"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

func GetCDBMUtilSettings(envVar string) (CDBMUtilSettings, error) {
	var settings CDBMUtilSettings
	var err error
	var envUsed string

	if envVar != "" {
		envUsed = envVar
	} else {
		envUsed = os.Getenv(defaultEnvVar)
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

func DefaultGetDB(dbSettings BaseDatabaseSettings) (*sqlx.DB, error) {
	return webutil.NewDB(dbSettings.Settings, dbSettings.DatabaseType)
}

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

		go func() {
			// fmt.Printf("filter server port: %s\n", testSettings.FileServerURL)
			http.ListenAndServe(testSettings.DBSetup.FileServerSetup.FileServerURL, fs)
		}()

		cmdStr, ok := testSettings.DBAction.Import.ImportMap[testSettings.DBAction.Import.ImportKey]

		if !ok {
			return nil, "", fmt.Errorf("file import key does not exist")
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

		if hasError(cmdFunc(importCmd)) {
			return nil, "", errors.WithStack(fmt.Errorf(stdErr.String()))
		}
	} else if testSettings.DBSetup.BaseSchemaFile != "" {
		cmdStr, ok := testSettings.DBAction.Import.ImportMap[testSettings.DBAction.Import.ImportKey]

		if !ok {
			return nil, "", fmt.Errorf("file import key does not exist")
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

	return db, dbName, nil
}

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
