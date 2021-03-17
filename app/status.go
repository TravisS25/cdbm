package app

import "fmt"

func (cdbm *CDBM) Status() error {
	sm, err := cdbm.querySchemaMigration()

	if err != nil {
		return err
	}

	if sm.NoRows {
		fmt.Printf("No migration entry\n")
	} else {
		var ds string

		if sm.DirtyState != nil {
			ds = *sm.DirtyState
		}

		fmt.Printf(
			fmt.Sprintf(
				"migration state - version:%d / dirty:%v / dirty state:%s \n",
				sm.StartingVersion,
				sm.Dirty,
				ds,
			),
		)
	}

	return nil
}
