package cdbmutil

import (
	"os/exec"

	"github.com/TravisS25/webutil/webutil"
	"github.com/jmoiron/sqlx"
)

func DefaultExecCmd(c *exec.Cmd) error {
	return c.Run()
}

func DefaultGetDB(dbSettings BaseDatabaseSettings) (*sqlx.DB, error) {
	return webutil.NewDB(dbSettings.Settings, dbSettings.DatabaseType)
}
