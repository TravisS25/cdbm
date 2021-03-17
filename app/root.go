package app

type RootFlagsConfig struct {
	DBProtocol    string `yaml:"db_protocol" mapstructure:"db_protocol"`
	EnvVar        string `yaml:"env_var" mapstructure:"env_var"`
	Database      string `yaml:"database" mapstructure:"database"`
	Host          string `yaml:"host" mapstructure:"host"`
	User          string `yaml:"user" mapstructure:"user"`
	Password      string `yaml:"password" mapstructure:"password"`
	SSLMode       string `yaml:"ssl_mode" mapstructure:"ssl_mode"`
	SSLRootCert   string `yaml:"ssl_root_cert" mapstructure:"ssl_root_cert"`
	SSLKey        string `yaml:"ssl_key" mapstructure:"ssl_key"`
	SSLCert       string `yaml:"ssl_cert" mapstructure:"ssl_cert"`
	Port          int    `yaml:"port" mapstructure:"port"`
	SSL           bool   `yaml:"ssl" mapstructure:"ssl"`
	UseFileOnFail bool   `yaml:"use_file_on_fail" mapstructure:"use_file_on_fail"`
}

type RootNameConfig struct {
	DBProtocol    FlagName
	Env           FlagName
	Database      FlagName
	Host          FlagName
	User          FlagName
	Password      FlagName
	Port          FlagName
	SSLMode       FlagName
	SSLRootCert   FlagName
	SSLKey        FlagName
	SSLCert       FlagName
	SSL           FlagName
	UseFileOnFail FlagName
}

var DefaultRootNameCfg = RootNameConfig{
	DBProtocol: FlagName{
		LongHand:  "db-protocol",
		ShortHand: "b",
	},
	Env: FlagName{
		LongHand:  "env",
		ShortHand: "e",
	},
	Database: FlagName{
		LongHand:  "database",
		ShortHand: "d",
	},
	Host: FlagName{
		LongHand:  "host",
		ShortHand: "h",
	},
	User: FlagName{
		LongHand:  "user",
		ShortHand: "u",
	},
	Password: FlagName{
		LongHand:  "password",
		ShortHand: "w",
	},
	Port: FlagName{
		LongHand:  "port",
		ShortHand: "p",
	},
	SSLMode: FlagName{
		LongHand:  "ssl-mode",
		ShortHand: "m",
	},
	SSLRootCert: FlagName{
		LongHand:  "ssl-root-cert",
		ShortHand: "r",
	},
	SSLKey: FlagName{
		LongHand:  "ssl-key",
		ShortHand: "k",
	},
	SSLCert: FlagName{
		LongHand:  "ssl-cert",
		ShortHand: "c",
	},
	SSL: FlagName{
		LongHand:  "ssl",
		ShortHand: "s",
	},
	UseFileOnFail: FlagName{
		LongHand:  "use-file-on-fail",
		ShortHand: "f",
	},
}
