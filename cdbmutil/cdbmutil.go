package cdbmutil

import (
	"database/sql"
	"fmt"

	"github.com/TravisS25/webutil/webutil"
	migrate "github.com/golang-migrate/migrate/v4"
	"github.com/jmoiron/sqlx"
)

var (
	// ErrInvalidFileName is error to indicate that the sql files in given directory do not
	// have the correct naming convention
	ErrInvalidFileName = fmt.Errorf("cdbmutil: invalid sql file name - proper naming:<version>_<description>.<'up'|'down'>.sql")
)

// Below are migration types that determine which direction to migrate a database
const (
	// MigrateTypeUp is MigrationsType's type for migrating a database forward
	MigrateTypeUp MigrationsType = "Up"

	// MigrateTypeDown is MigrationsType's type for reversing a database backward
	MigrateTypeDown MigrationsType = "Down"

	// MigrateTypeForce is MigrationsType's type for forcing a database migration up or down
	MigrateTypeForce MigrationsType = "Force"
)

// Below are protocol strings used to connect to a variety of source urls for sql migrations files
const (
	// FileProtocol is protocol used to find migration files on host machine
	FileProtocol MigrationsProtocol = "file://"
	GoBindData   MigrationsProtocol = "go-bindata"

	// GithubProtocol is protocol used to find migration files on github servers
	GithubProtocol MigrationsProtocol = "github://"

	// GitHubEnterpriseProtocol is protocol used to find migration files on github enterprise servers
	GitHubEnterpriseProtocol MigrationsProtocol = "github-ee://"

	// BitbucketProtocol is protocol used to find migration files on bitbucket servers
	BitbucketProtocol MigrationsProtocol = "bitbucket://"

	// GitlabProtcol is protocol used to find migration files on gitlab servers
	GitlabProtcol MigrationsProtocol = "gitlab://"

	// S3Protocol is protocol used to find migration files on s3 compatible servers
	S3Protocol MigrationsProtocol = "s3://"

	// GoogleCloudStorageProtocol is protocol used to find migration files on google cloud servers
	GoogleCloudStorageProtocol MigrationsProtocol = "gcs://"
)

const (
	// CDBM_UTIL_CONFIG is default enviroment variable used to point to config file for cdmbutil
	CDBM_UTIL_CONFIG = "CDBM_UTIL_CONFIG"
)

const (
	// PostgresProtocol is postgres protocol string when making connection to database
	PostgresProtocol DBProtocol = "postgres"

	// CockroachdbProtocol is cockroach protocol string when making connection to database
	CockroachdbProtocol DBProtocol = "cockroachdb"
)

// DBProtocol represents different database protocols
type DBProtocol string

// MigrationsType is enum for different migration type ie. "Up", "Down", "Force"
type MigrationsType string

// MigrationsProtocol is enum for different database protocols
type MigrationsProtocol string

// FileMigrationFunc should implement migrating database up or down
type FileMigrationFunc func(mig *migrate.Migrate, version int, mt MigrationsType) error

// CustomMigrationFunc should implement migrating database up or down through custom code
type CustomMigrationFunc func(db webutil.DBInterface) error

// GetMigrationFunc should implement getting migrate.Migrate based on migrations
// directory and database instance
type GetMigrationFunc func(migDir string, db *sql.DB, protocolCfg DBProtocolConfig) (*migrate.Migrate, error)

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

// ConnectionStringConfig is config struct used to construct a connection string
// for the GetMigrationConnStr function
type ConnectionStringConfig struct {
	DBSettings webutil.DatabaseSetting
}

// CustomMigration is config struct used to migrate database with custom go code
type CustomMigration struct {
	// Up should migrate database to next state
	Up CustomMigrationFunc

	// Down should migrate database to previous state
	Down CustomMigrationFunc
}

type FileServerSetup struct {
	BaseSchemaDir string `yaml:"base_schema_dir" mapstructure:"base_schema_dir"`
	FileServerURL string `yaml:"file_server_url" mapstructure:"file_server_url"`
}

type DBSetup struct {
	BaseSchemaFile  string           `yaml:"base_schema_file" mapstructure:"base_schema_file"`
	FileServerSetup *FileServerSetup `yaml:"file_server_setup" mapstructure:"file_server_setup"`
}

type ImportConfig struct {
	ImportKeys []string          `yaml:"import_keys" mapstructure:"import_keys"`
	ImportMap  map[string]string `yaml:"import_map" mapstructure:"import_map"`
}

type DBAction struct {
	CreateDB string       `yaml:"create_db" mapstructure:"create_db"`
	DropDB   string       `yaml:"drop_db" mapstructure:"drop_db"`
	Import   ImportConfig `yaml:"import" mapstructure:"import"`
}

type BaseDatabaseSettings struct {
	Settings         webutil.DatabaseSetting `yaml:"settings" mapstructure:"settings"`
	DatabaseType     string                  `yaml:"database_type" mapstructure:"database_type"`
	DatabaseProtocol string                  `yaml:"database_protocol" mapstructure:"database_protocol"`
}

type CDBMUtilSettings struct {
	InsertEndQuery       string               `yaml:"insert_end_query" mapstructure:"insert_end_query"`
	DBAction             DBAction             `yaml:"db_action" mapstructure:"db_action"`
	DBSetup              DBSetup              `yaml:"db_setup" mapstructure:"db_setup"`
	BaseDatabaseSettings BaseDatabaseSettings `yaml:"base_database_settings" mapstructure:"base_database_settings"`
}

type MigrationSetupTeardownReturn struct {
	DB       *sqlx.DB
	Settings CDBMUtilSettings
	TearDown func()
}
