package cdbmutil

import (
	"github.com/TravisS25/webutil/webutil"
)

const (
	defaultEnvVar = "CDBM_UTIL_CONFIG"
)

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
