package app

import (
	"fmt"

	"github.com/TravisS25/webutil/webutil"
	"github.com/golang-migrate/migrate/v4/database/cockroachdb"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

var (
	// DefaultProtocolMap is global database protcol map used to determine
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
)

const (
	defaultEnvVar = "CDBM_CONFIG"
)

const (
	PostgresProtocol    DBProtocol = "postgres"
	CockroachdbProtocol DBProtocol = "cockroachdb"
)

// DBProtocol represents different database protocols
type DBProtocol string

type ConnectionStringConfig struct {
	DBSettings webutil.DatabaseSetting
}

// CDBM is main struct for app and is used for all commands
type CDBM struct {
	// DB is database connection used
	//
	// This will be set in NewCDBM
	DB                *sqlx.DB                `yaml:"-" mapstructure:"-"`
	CurrentDBSettings webutil.DatabaseSetting `yaml:"-" mapstructure:"-"`

	// DBProtocolCfg is config used for different database settings based
	// on database being used
	//
	// This will be set in NewCDBM
	DBProtocolCfg DBProtocolConfig `yaml:"-" mapstructure:"-"`

	// MigrateFlags represents the flags for migrate command
	MigrateFlags MigrateFlagsConfig `yaml:"migrate_flags" mapstructure:"migrate_flags"`

	// RootFlags represents the flags for root command
	RootFlags RootFlagsConfig `yaml:"root_flags" mapstructure:"root_flags"`

	// DropFlags represents the flags for drop command
	DropFlags DropFlagsConfig `yaml:"drop_flags" mapstructure:"drop_flags"`

	// LogFlags represents the flags for log command
	LogFlags LogFlagsConfig `yaml:"log_flags" mapstructure:"log_flags"`

	// DatabaseConfig is map with different db connections to database to be used
	// if one or more fail
	DatabaseConfig map[string][]webutil.DatabaseSetting `yaml:"database_config" mapstructure:"database_config"`

	// migrateCfg is config that is built as the CDBM#Migrate function is run
	migrateCfg migrateConfig
}

// DBProtocolConfig is config struct used to set up different settings
// for migrate command based on database being used
type DBProtocolConfig struct {
	// SQLBindVar determines what bind var to use for database
	SQLBindVar int

	// DatabaseType is what database is currently being used
	DatabaseType string

	// DBProtocol is what protocol to use when connecting to database
	DBProtocol DBProtocol

	// MigrationTableSearch determines if schema_migrations table exists
	// in database or not
	//
	// Should return nil if schema_migrations is found
	MigrationTableSearch func(db webutil.DBInterface) error

	// DriverConfig is config struct used for migrate library
	// for different settings based on database
	DriverConfig interface{}
}

// FlagName is config struct to determine the long and short hand flag names
type FlagName struct {
	// LongHand is long hand form of flag ie. --name
	LongHand string

	// Shoartnad is short hand form of flag ie. -n
	ShortHand string
}

// schemaMigration represents schema_migration table along
// with having a dynamic config
type schemaMigration struct {
	StartingVersion   int
	Dirty             bool
	DirtyState        *string
	IsCustomMigration bool
	SchemaCfg         schemaConfig
}

// schemaConfig is used in conjunction with schemaMigration struct
//
// This config struct is used to keep different states that can happen
// within migrate command
//
// Basically schemaMigration struct is used to query for entry in schema_migrations
// and any changes made to that state is reflected in schemaConfig properties
// so we don't change state of the original
type schemaConfig struct {
	// NoRows determines if schema_migrations table has no rows
	NoRows bool

	// HasEntry determines if there are any entries in the schema_migrations table
	HasEntry bool

	// Dirty determines if schema_migrations table is dirty
	Dirty bool
}

// NewCDBM intiates a new *CDBM instance
//
// RootFlagsConfig#DBProtocol is a required if not read from file
// as it intiates what type of database we are working with
func NewCDBM(cfg RootFlagsConfig, driverCfg interface{}) (*CDBM, error) {
	cdbm, err := GetCDBMConfig(cfg.EnvVar)

	if err != nil {
		return nil, err
	}

	// Overriding settings if set by flags
	if cfg.DBProtocol != "" {
		cdbm.RootFlags.DBProtocol = cfg.DBProtocol
	}
	if cfg.Database != "" {
		cdbm.RootFlags.Database = cfg.Database
	}
	if cfg.Host != "" {
		cdbm.RootFlags.Host = cfg.Host
	}
	if cfg.User != "" {
		cdbm.RootFlags.User = cfg.User
	}
	if cfg.Port != -1 {
		cdbm.RootFlags.Port = cfg.Port
	}
	if cfg.Password != "" {
		cdbm.RootFlags.Password = cfg.Password
	}
	if cfg.SSLMode != "" {
		cdbm.RootFlags.SSLMode = cfg.SSLMode
	}
	if cfg.SSLRootCert != "" {
		cdbm.RootFlags.SSLRootCert = cfg.SSLRootCert
	}
	if cfg.SSLKey != "" {
		cdbm.RootFlags.SSLKey = cfg.SSLKey
	}
	if cfg.SSLCert != "" {
		cdbm.RootFlags.SSLCert = cfg.SSLCert
	}
	if cfg.SSL {
		cdbm.RootFlags.SSL = cfg.SSL
	}
	if cfg.UseFileOnFail {
		cdbm.RootFlags.UseFileOnFail = cfg.UseFileOnFail
	}

	protocolSlice := make([]DBProtocol, 0, len(DefaultProtocolMap))

	for k := range DefaultProtocolMap {
		protocolSlice = append(protocolSlice, k)
	}

	if cdbm.RootFlags.DBProtocol == "" {
		return nil, fmt.Errorf("--db-protocol flag required.  Valid --db-protocol values are: %v", protocolSlice)
	} else {
		var ok bool

		if cdbm.DBProtocolCfg, ok = DefaultProtocolMap[DBProtocol(cdbm.RootFlags.DBProtocol)]; !ok {
			return nil, fmt.Errorf("Invalid --db-protocol.  Valid --db-protocol values are: %v", protocolSlice)
		}
	}

	// Searches for database connection based on config and returns error
	// if one can't be established
	searchConn := func() error {
		foundConn := false

		for _, conns := range cdbm.DatabaseConfig {
			for _, conn := range conns {
				cdbm.CurrentDBSettings = webutil.DatabaseSetting{
					BaseAuthSetting: webutil.BaseAuthSetting{
						Host:     conn.Host,
						User:     conn.User,
						Password: conn.Password,
						Port:     conn.Port,
					},
					DBName:      conn.DBName,
					SSLMode:     conn.SSLMode,
					SSL:         conn.SSL,
					SSLRootCert: conn.SSLRootCert,
				}

				if cdbm.DB, err = webutil.NewDB(
					cdbm.CurrentDBSettings,
					cdbm.DBProtocolCfg.DatabaseType,
				); err == nil {
					foundConn = true
					break
				}
			}

			if foundConn {
				break
			}
		}

		if !foundConn {
			return errors.WithStack(err)
		}

		return nil
	}

	if cfg.Database != "" {
		if cfg.User == "" || cfg.Host == "" || cfg.Port == -1 {
			return nil, fmt.Errorf("--user, --host and --port must be set if --database is set")
		}

		cdbm.CurrentDBSettings = webutil.DatabaseSetting{
			BaseAuthSetting: webutil.BaseAuthSetting{
				Host:     cfg.Host,
				User:     cfg.User,
				Password: cfg.Password,
				Port:     cfg.Port,
			},
			DBName:      cfg.Database,
			SSLMode:     cfg.SSLMode,
			SSL:         cfg.SSL,
			SSLRootCert: cfg.SSLRootCert,
		}

		if cdbm.DB, err = webutil.NewDB(cdbm.CurrentDBSettings, cdbm.DBProtocolCfg.DatabaseType); err != nil {
			if cfg.UseFileOnFail {
				if err = searchConn(); err != nil {
					return nil, errors.WithStack(err)
				}
			} else {
				return nil, errors.WithStack(err)
			}
		}
	} else {
		if err = searchConn(); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	if cdbm.DB == nil {
		return nil, fmt.Errorf("No connection to database was established.  Check config file for proper settings")
	}

	if driverCfg != nil {
		if _, err = GetDatabaseDriver(
			cdbm.DB.DB,
			cdbm.DBProtocolCfg.DBProtocol,
			driverCfg,
		); err != nil {
			return nil, errors.WithStack(err)
		}

		cdbm.DBProtocolCfg.DriverConfig = driverCfg
	}

	return cdbm, nil
}
