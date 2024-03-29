package app

import "fmt"

// Status will print out to stdout the current database migration table status
func (cdbm *CDBM) Status() error {
	var err error

	cdbm.migrateCfg.SchemaMigration, err = cdbm.getSchemaMigration()

	if err != nil {
		return err
	}

	// If there are no rows in table, then no migration has happended so print to stdout
	//
	// Else display current migration status
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
