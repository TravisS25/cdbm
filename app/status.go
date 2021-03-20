package app

import "fmt"

func (cdbm *CDBM) Status() error {
	err := cdbm.querySchemaMigration()

	if err != nil {
		return err
	}

	if cdbm.migrateCfg.SchemaMigration.SchemaCfg.NoRows {
		fmt.Printf("No migration entry\n")
	} else {
		var ds string

		if cdbm.migrateCfg.SchemaMigration.DirtyState != nil {
			ds = *cdbm.migrateCfg.SchemaMigration.DirtyState
		}

		fmt.Printf(
			fmt.Sprintf(
				"migration state - version:%d / dirty:%v / dirty state:%s \n",
				cdbm.migrateCfg.SchemaMigration.StartingVersion,
				cdbm.migrateCfg.SchemaMigration.Dirty,
				ds,
			),
		)
	}

	return nil
}
