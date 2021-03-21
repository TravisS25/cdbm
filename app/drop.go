package app

import (
	"fmt"

	"github.com/pkg/errors"
)

type DropFlagsConfig struct {
	Confirm bool `yaml:"confirm" mapstructure:"confirm"`
}

func (cdbm *CDBM) Drop() error {
	var answer string
	var err error

	dropFunc := func() error {
		mig, err := DefaultGetMigrationFunc(
			"file://"+cdbm.MigrateFlags.MigrationsDir,
			cdbm.DB.DB,
			cdbm.DBProtocolCfg,
		)

		if err != nil {
			return errors.WithStack(err)
		}

		if err = mig.Drop(); err == nil {
			fmt.Printf("All tables dropped\n")
		}

		return err
	}

	if cdbm.DropFlags.Confirm {
		return dropFunc()
	}

	fmt.Printf("You are about to drop entire database.  Are you sure you want to continue (y/n)? ")

	for {
		if _, err = fmt.Scanln(&answer); err != nil {
			return errors.WithStack(err)
		}

		if answer == "y" || answer == "n" {
			break
		}

		fmt.Printf("(y/n)? ")
	}

	if answer == "y" {
		return dropFunc()
	}

	return nil
}
