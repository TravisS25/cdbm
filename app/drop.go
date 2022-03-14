package app

import (
	"fmt"

	"github.com/TravisS25/cdbm/cdbmutil"
	"github.com/pkg/errors"
)

// DropFlagsConfig is config settings used for CDBM#Drop function
type DropFlagsConfig struct {
	Confirm bool `yaml:"confirm" mapstructure:"confirm"`
}

// Drop will drop all tables to current database
func (cdbm *CDBM) Drop() error {
	mig, err := cdbmutil.DefaultGetMigrationFunc(
		string(cdbm.MigrateFlags.MigrationsProtocol)+cdbm.MigrateFlags.MigrationsDir,
		cdbm.DB.DB,
		cdbm.DBProtocolCfg,
	)

	if err != nil {
		return errors.WithStack(err)
	}

	if err = mig.Drop(); err == nil {
		fmt.Printf("All tables dropped\n")
	}

	return errors.WithStack(err)
}
