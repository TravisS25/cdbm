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

type DBProtocol string

type ConnectionStringConfig struct {
	DBSettings webutil.DatabaseSetting
}

type CDBM struct {
	DB                *sqlx.DB                             `yaml:"-" mapstructure:"-"`
	CurrentDBSettings webutil.DatabaseSetting              `yaml:"-" mapstructure:"-"`
	DBProtocolCfg     DBProtocolConfig                     `yaml:"-" mapstructure:"-"`
	MigrateFlags      MigrateFlagsConfig                   `yaml:"migrate_flags" mapstructure:"migrate_flags"`
	RootFlags         RootFlagsConfig                      `yaml:"root_flags" mapstructure:"root_flags"`
	DropFlags         DropFlagsConfig                      `yaml:"drop_flags" mapstructure:"drop_flags"`
	LogFlags          LogFlagsConfig                       `yaml:"log_flags" mapstructure:"log_flags"`
	DatabaseConfig    map[string][]webutil.DatabaseSetting `yaml:"database_config" mapstructure:"database_config"`

	migrateCfg migrateConfig
}

type DBProtocolConfig struct {
	SQLBindVar           int
	DatabaseType         string
	DBProtocol           DBProtocol
	MigrationTableSearch func(db webutil.DBInterface) error
	DriverConfig         interface{}
}

type FlagName struct {
	LongHand  string
	ShortHand string
}

type schemaMigration struct {
	StartingVersion   int
	Dirty             bool
	DirtyState        *string
	IsCustomMigration bool
	SchemaCfg         schemaConfig
}

type schemaConfig struct {
	NoRows   bool
	HasEntry bool
	Dirty    bool
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
		return nil, fmt.Errorf("--db-protocol flag required.  Valid --db-protocol values are: %v\n", protocolSlice)
	} else {
		var ok bool

		if cdbm.DBProtocolCfg, ok = DefaultProtocolMap[DBProtocol(cdbm.RootFlags.DBProtocol)]; !ok {
			return nil, fmt.Errorf("Invalid --db-protocol.  Valid --db-protocol values are: %v\n", protocolSlice)
		}
	}

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
			//return errors.WithStack(fmt.Errorf("--user, --host and --port must be set if --database is set"))
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
		return nil, fmt.Errorf("No connection to database was established.  Check config file for proper settings\n")
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
