package app

import (
	"database/sql"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/TravisS25/webutil/webutil"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/cockroachdb"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/file"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

type FileOverride struct {
	*file.File
}

func (fo *FileOverride) ReadUp(version uint) (r io.ReadCloser, identifier string, err error) {
	stringReader := strings.NewReader("shiny!")
	stringReadCloser := ioutil.NopCloser(stringReader)
	return stringReadCloser, "", nil
}

func init() {
	//source.Register("")
}

func DefaultExecCmd(c *exec.Cmd) error {
	return c.Run()
}

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

func GetCDBMConfig(env string) (*CDBM, error) {
	var cdbm CDBM
	var err error
	var envUsed string

	if env != "" {
		envUsed = os.Getenv(env)
	} else {
		envUsed = os.Getenv(defaultEnvVar)
	}

	viper.SetConfigFile(envUsed)

	if err = viper.ReadInConfig(); err != nil {
		return nil, errors.WithStack(err)
	}

	if viper.Unmarshal(&cdbm); err != nil {
		return nil, errors.WithStack(err)
	}

	cdbm.RootFlags.EnvVar = envUsed

	return &cdbm, nil
}

func GetMigrationConnStr(cfg ConnectionStringConfig, typ DBProtocol) string {
	switch typ {
	case PostgresProtocol:
		return fmt.Sprintf(
			"postgres://%s:%s@%s:%d/%s?ssl=%v&sslmode=%s&sslcert=%s&sslkey=%s&sslrootcert=%s",
			cfg.DBSettings.User,
			cfg.DBSettings.Password,
			cfg.DBSettings.Host,
			cfg.DBSettings.Port,
			cfg.DBSettings.DBName,
			cfg.DBSettings.SSL,
			cfg.DBSettings.SSLMode,
			cfg.DBSettings.SSLCert,
			cfg.DBSettings.SSLKey,
			cfg.DBSettings.SSLRootCert,
		)
	case CockroachdbProtocol:
		return fmt.Sprintf(
			"cockroachdb://%s:%s@%s:%d/%s?ssl=%v&sslmode=%s&sslcert=%s&sslkey=%s&sslrootcert=%s",
			cfg.DBSettings.User,
			cfg.DBSettings.Password,
			cfg.DBSettings.Host,
			cfg.DBSettings.Port,
			cfg.DBSettings.DBName,
			cfg.DBSettings.SSL,
			cfg.DBSettings.SSLMode,
			cfg.DBSettings.SSLCert,
			cfg.DBSettings.SSLKey,
			cfg.DBSettings.SSLRootCert,
		)
	}

	return ""
}

func AppendValuesQuery(
	insertQuery,
	insertEndQuery,
	updateQuery string,
	sqlBindVar int,
	bulkInsert uint,
	insertValuesQuery *string,
	counter *uint,
	isFinalExec bool,
	db webutil.QuerierExec,
) error {
	insertFunc := func() error {
		if *insertValuesQuery != "" {
			if isFinalExec {
				q := *insertValuesQuery
				q = q[:len(q)-1]
				*insertValuesQuery = q
			}

			if insertEndQuery != "" {
				*insertValuesQuery += insertEndQuery
			} else {
				*insertValuesQuery += ";"
			}

			ids, err := webutil.QuerySingleColumn(
				db,
				sqlBindVar,
				insertQuery+*insertValuesQuery,
			)

			if err != nil {
				return errors.WithStack(err)
			}

			if updateQuery != "" {
				if updateQuery, _, err = webutil.InQueryRebind(
					sqlBindVar,
					updateQuery,
					ids,
				); err != nil {
					return errors.WithStack(err)
				}

				if _, err = db.Exec(updateQuery, ids...); err != nil {
					return errors.WithStack(err)
				}
			}

			*counter = 0
			*insertValuesQuery = ""
		}

		return nil
	}

	if isFinalExec {
		return insertFunc()
	}

	*counter++

	if *counter != bulkInsert {
		*insertValuesQuery += ","
	} else {
		return insertFunc()
	}

	return nil
}

func GetDatabaseDriver(db *sql.DB, protcol DBProtocol, cfg interface{}) (database.Driver, error) {
	var ok bool

	switch protcol {
	case PostgresProtocol:
		if cfg == nil {
			return postgres.WithInstance(db, &postgres.Config{})
		}

		if _, ok = cfg.(*postgres.Config); !ok {
			return nil, fmt.Errorf("config must be type *postgres.Config")
		}

		return postgres.WithInstance(db, cfg.(*postgres.Config))
	case CockroachdbProtocol:
		if cfg == nil {
			return cockroachdb.WithInstance(db, &cockroachdb.Config{})
		}

		if _, ok = cfg.(*cockroachdb.Config); !ok {
			return nil, fmt.Errorf("config must be type *cockroachdb.Config")
		}

		return cockroachdb.WithInstance(db, cfg.(*cockroachdb.Config))
	default:
		return nil, fmt.Errorf("invalid db protocol")
	}
}
