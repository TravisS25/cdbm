package cdbmutil

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestGetNewDatabase(t *testing.T) {
	var err error

	settings, err := GetCDBMUtilSettings("")

	if err != nil {
		t.Fatalf(err.Error())
	}

	if _, _, err = GetNewDatabase(
		settings,
		func(c *exec.Cmd) error {
			cmdErr := fmt.Errorf("cmd error")
			c.Stderr.Write([]byte(cmdErr.Error()))
			return cmdErr
		},
		nil,
	); err == nil {
		t.Errorf("Should have error")
	} else {
		if err.Error() != "cmd error" {
			t.Errorf("Should have cmd error; got: %s\n", err.Error())
		}
	}

	// -----------------------------------------------------------------

	if _, _, err = GetNewDatabase(
		settings,
		func(c *exec.Cmd) error {
			return nil
		},
		func(bs BaseDatabaseSettings) (*sqlx.DB, error) {
			return nil, fmt.Errorf("db error")
		},
	); err == nil {
		t.Errorf("Should have error")
	} else {
		if err.Error() != "db error" {
			t.Errorf("Should have db error; got: %s\n", err.Error())
		}
	}

	// -----------------------------------------------------------------

	mockSettings := settings
	mockSettings.DBSetup.FileServerSetup = &FileServerSetup{
		BaseSchemaDir: "/tmp/",
		FileServerURL: "http://localhost:9945/",
	}

	if _, _, err = GetNewDatabase(
		mockSettings,
		func(c *exec.Cmd) error {
			fmt.Printf("command: %v\n", c.Args[2])
			if strings.Contains(c.Args[2], mockSettings.DBSetup.FileServerSetup.FileServerURL) {
				importErr := fmt.Errorf("import error")
				c.Stderr.Write([]byte(importErr.Error()))
				return importErr
			}

			return nil
		},
		func(bs BaseDatabaseSettings) (*sqlx.DB, error) {
			return &sqlx.DB{}, nil
		},
	); err == nil {
		t.Errorf("Should have error")
	} else {
		if err.Error() != "import error" {
			t.Errorf("Should have import error; got: %s\n", err.Error())
		}
	}

	// -----------------------------------------------------------------

	mockSettings.DBAction.Import.ImportKeys = []string{}

	if _, _, err = GetNewDatabase(
		mockSettings,
		func(c *exec.Cmd) error {
			return nil
		},
		func(bs BaseDatabaseSettings) (*sqlx.DB, error) {
			return &sqlx.DB{}, nil
		},
	); err == nil {
		t.Errorf("Should have error")
	} else {
		if err.Error() != "file import key does not exist" {
			t.Errorf("Should have import key error; got: %s\n", err.Error())
		}
	}

	// -----------------------------------------------------------------

	mockSettings.DBSetup.FileServerSetup = nil
	mockSettings.DBSetup.BaseSchemaFile = "foobar"

	if _, _, err = GetNewDatabase(
		mockSettings,
		func(c *exec.Cmd) error {
			return nil
		},
		func(bs BaseDatabaseSettings) (*sqlx.DB, error) {
			return &sqlx.DB{}, nil
		},
	); err == nil {
		t.Errorf("Should have error")
	} else {
		if err.Error() != "file import key does not exist" {
			t.Errorf("Should have file key error; got: %s\n", err.Error())
		}
	}

	// -----------------------------------------------------------------

	mockSettings.DBAction.Import.ImportKeys = []string{"schema"}

	if _, _, err = GetNewDatabase(
		mockSettings,
		func(c *exec.Cmd) error {
			fmt.Printf("command: %v\n", c.Args[2])
			if strings.Contains(c.Args[2], mockSettings.DBSetup.BaseSchemaFile) {
				importErr := fmt.Errorf("file error")
				c.Stderr.Write([]byte(importErr.Error()))
				return importErr
			}

			return nil
		},
		func(bs BaseDatabaseSettings) (*sqlx.DB, error) {
			return &sqlx.DB{}, nil
		},
	); err == nil {
		t.Errorf("Should have error")
	} else {
		if err.Error() != "file error" {
			t.Errorf("Should have file error; got: %s\n", err.Error())
		}
	}
}

func TestGetNewDatabaseIntegrationTest(t *testing.T) {
	var err error

	settings, err := GetCDBMUtilSettings("")

	if err != nil {
		t.Fatalf(err.Error())
	}

	if _, _, err = GetNewDatabase(
		settings,
		func(c *exec.Cmd) error {
			return c.Run()
		},
		DefaultGetDB,
	); err == nil {
		t.Errorf("Should have error")
	} else {
		if err.Error() != "file error" {
			t.Errorf("Should have file error; got: %s\n", err.Error())
		}
	}
}

func TestGenerateNewDatabase(t *testing.T) {
	var err error

	settings, err := GetCDBMUtilSettings("")

	if err != nil {
		t.Fatalf(err.Error())
	}

	settings.DBSetup.FileServerSetup = nil
	settings.DBSetup.BaseSchemaFile = ""

	if _, _, err = GetNewDatabase(
		settings,
		func(c *exec.Cmd) error {
			return c.Run()
		},
		DefaultGetDB,
	); err != nil {
		t.Errorf("Should not have error; got %s\n", err.Error())
	}
}
