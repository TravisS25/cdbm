package app

import (
	"os"

	"github.com/TravisS25/webutil/webutil"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

func GetCDBMConfig(env string) (*CDBM, error) {
	var cdbm CDBM
	var err error
	var envUsed string

	if env != "" {
		envUsed = os.Getenv(env)
	} else {
		envUsed = os.Getenv(CDBM_CONFIG)
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
