package cdbmutil

import (
	"database/sql"
	"fmt"
	"os/exec"

	"github.com/TravisS25/webutil/webutil"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/cockroachdb"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

var (
	// DefaultProtocolMap is global database protocol map used to determine
	// different settings for migrate command based on what database user is using
	DefaultProtocolMap = map[DBProtocol]DBProtocolConfig{
		PostgresProtocol: {
			DBProtocol:           PostgresProtocol,
			DatabaseType:         webutil.Postgres,
			SQLBindVar:           sqlx.DOLLAR,
			DriverConfig:         &postgres.Config{},
			MigrationTableSearch: postgresMigrationTableSearch,
		},
		CockroachdbProtocol: {
			DBProtocol:           CockroachdbProtocol,
			DatabaseType:         webutil.Postgres,
			SQLBindVar:           sqlx.DOLLAR,
			DriverConfig:         &cockroachdb.Config{},
			MigrationTableSearch: postgresMigrationTableSearch,
		},
	}

	// postgresMigrationTableSearch is default search function for postgres to
	// determine if schema_migrations table exists in database table
	postgresMigrationTableSearch = func(db webutil.DBInterface) error {
		var filler string
		var err error

		if err = db.QueryRowx(
			`
			select 
				table_name 
			from 
				information_schema.tables 
			where  
				table_schema = 'public'
			and    
				table_name = 'schema_migrations' 
			`,
		).Scan(&filler); err != nil {
			return err
		}

		return nil
	}
)

// DefaultExecCmd is default function for executing a command line tool
func DefaultExecCmd(c *exec.Cmd) error {
	return c.Run()
}

// DefaultGetDB is default function for retrieving an instance of sqlx.DB based on settings passed
func DefaultGetDB(dbSettings BaseDatabaseSettings) (*sqlx.DB, error) {
	return webutil.NewDB(dbSettings.Settings, dbSettings.DatabaseType)
}

// DefaultGetMigrationFunc is the default function to retrieve migration configuration to use against database
func DefaultGetMigrationFunc(migDir string, db *sql.DB, protocolCfg DBProtocolConfig) (*migrate.Migrate, error) {
	driver, err := GetDatabaseDriver(db, protocolCfg.DBProtocol, protocolCfg.DriverConfig)

	if err != nil {
		return nil, errors.WithStack(err)
	}

	mig, err := migrate.NewWithDatabaseInstance(migDir, protocolCfg.DatabaseType, driver)

	if err != nil {
		return nil, errors.WithStack(err)
	}

	return mig, nil
}

// DefaultFileMigrationFunc is the default function to use with migrate library
// to determine whether to migrate database up, down or force
func DefaultFileMigrationFunc(mig *migrate.Migrate, version int, mt MigrationsType) error {
	switch mt {
	case MigrateTypeUp:
		return mig.Steps(1)
	case MigrateTypeDown:
		return mig.Steps(-1)
	case MigrateTypeForce:
		return mig.Force(version)
	default:
		return fmt.Errorf("invalid migration type")
	}
}
