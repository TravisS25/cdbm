package app

import (
	"fmt"

	"github.com/TravisS25/cdbm/cdbmutil"
	"github.com/TravisS25/webutil/webutil"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

const (
	// CDBM_CONFIG is the default enviroment variable that should point to config file
	CDBM_CONFIG = "CDBM_CONFIG"
)

// CDBM is main struct for app and is used for all commands
type CDBM struct {
	// DB is database connection used
	//
	// This will be set in NewCDBM
	DB *sqlx.DB

	// DBProtocolCfg is config used for different database settings based
	// on database being used
	//
	// This will be set in NewCDBM
	DBProtocolCfg cdbmutil.DBProtocolConfig `yaml:"-" mapstructure:"-"`

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

	// migrateCfg is config that is built as the CDBM#Migrate function is ran
	migrateCfg migrateState

	// currentDBSettings represents the current active connection to db
	currentDBSettings webutil.DatabaseSetting
}

func (cdbm *CDBM) GetCurrentDBSettings() webutil.DatabaseSetting {
	return cdbm.currentDBSettings
}

// NewCDBM initiates a new *CDBM instance
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

	protocolSlice := make([]cdbmutil.DBProtocol, 0, len(cdbmutil.DefaultProtocolMap))

	for k := range cdbmutil.DefaultProtocolMap {
		protocolSlice = append(protocolSlice, k)
	}

	// If dbprotocol is not set by user either from file or cli, throw error
	// as we need to to know which type of db we are connecting to
	if cdbm.RootFlags.DBProtocol == "" {
		return nil, fmt.Errorf("--db-protocol flag required.  Valid --db-protocol values are: %v", protocolSlice)
	} else {
		var ok bool

		if cdbm.DBProtocolCfg, ok = cdbmutil.DefaultProtocolMap[cdbmutil.DBProtocol(cdbm.RootFlags.DBProtocol)]; !ok {
			return nil, fmt.Errorf("invalid --db-protocol.  Valid --db-protocol values are: %v", protocolSlice)
		}
	}

	// Searches for database connection based on config and returns error
	// if one can't be established
	searchConn := func() error {
		foundConn := false

		for _, conns := range cdbm.DatabaseConfig {
			for _, conn := range conns {
				cdbm.currentDBSettings = webutil.DatabaseSetting{
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
					cdbm.currentDBSettings,
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

	// If user sets --database flag, then --user, --host, and --port flags are also required at minimum
	if cfg.Database != "" {
		if cfg.User == "" || cfg.Host == "" || cfg.Port == -1 {
			return nil, fmt.Errorf("--user, --host and --port must be set if --database is set")
		}

		cdbm.currentDBSettings = webutil.DatabaseSetting{
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

		if cdbm.DB, err = webutil.NewDB(cdbm.currentDBSettings, cdbm.DBProtocolCfg.DatabaseType); err != nil {
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
		return nil, fmt.Errorf("no connection to database was established.  Check config file or cli settings for proper settings")
	}

	if driverCfg != nil {
		if _, err = cdbmutil.GetDatabaseDriver(
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

// NewTestCDBM will initiate a new *CDBM instance with the bare minimum fields required to run CDBM#Migrate
//
// This function should only be used for tests
func NewTestCDBM(db *sqlx.DB, dbProtocol cdbmutil.DBProtocol, migCfg MigrateFlagsConfig) (*CDBM, error) {
	protocolCfg, ok := cdbmutil.DefaultProtocolMap[dbProtocol]

	if !ok {
		return nil, errors.Errorf("invalid dbProtocol parameter passed")
	}

	if migCfg.MigrationsDir == "" {
		return nil, errors.Errorf("'MigrationsDir' property required for MigrateFlagsConfig")
	}

	if migCfg.MigrationsProtocol == "" {
		return nil, errors.Errorf("'MigrationsProtocol' property required for MigrateFlagsConfig")
	}

	return &CDBM{
		DB:            db,
		DBProtocolCfg: protocolCfg,
		MigrateFlags:  migCfg,
		RootFlags: RootFlagsConfig{
			DBProtocol: string(protocolCfg.DBProtocol),
		},
	}, nil
}

// FlagName is config struct to determine the long and short hand flag names
type FlagName struct {
	// LongHand is long hand form of flag ie. --name
	LongHand string

	// Shoartnad is short hand form of flag ie. -n
	ShortHand string
}

// schemaMigration represents schema_migration table along with having a dynamic config
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
	//HasEntry bool

	// Dirty determines if schema_migrations table is dirty
	Dirty bool
}
