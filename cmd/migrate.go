/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"

	"github.com/TravisS25/cdbm/cdbmutil"
	"github.com/spf13/cobra"
)

type migrateNameConfig struct {
	ResetDirtyFlag     flagName
	TargetVersion      flagName
	RollbackOnFailure  flagName
	LogFile            flagName
	MigrationsProtocol flagName
	MigrateDownOnDirty flagName
}

var migrateNameCfg = migrateNameConfig{
	TargetVersion: flagName{
		LongHand:  "target-version",
		ShortHand: "t",
	},
	RollbackOnFailure: flagName{
		LongHand:  "rollback-on-failure",
		ShortHand: "f",
	},
	LogFile: flagName{
		LongHand:  "migrations-dir",
		ShortHand: "m",
	},
	ResetDirtyFlag: flagName{
		LongHand:  "reset-dirty-flag",
		ShortHand: "r",
	},
	MigrationsProtocol: flagName{
		LongHand:  "migrations-protocol",
		ShortHand: "p",
	},
	MigrateDownOnDirty: flagName{
		LongHand:  "migrate-down-on-dirty",
		ShortHand: "",
	},
}

// migrateCmd represents the migrate command
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrates database",
	Long: `
	Migrate is a mixture of sql files and custom go functions to migrate database

	The reason for having this instead of just using a pure db migration tool
	is that there are some updates that require querying under certain criteria and looping 
	through results to manipulate in some fashion and is either much easier to do in code or 
	impossible to do in sql migrations
	`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		targetVersion, _ := cmd.Flags().GetInt(migrateNameCfg.TargetVersion.LongHand)
		migrationDir, _ := cmd.Flags().GetString(migrateNameCfg.LogFile.LongHand)
		migrationsProtocol, _ := cmd.Flags().GetString(migrateNameCfg.MigrationsProtocol.LongHand)

		globalApp.MigrateFlags.RollbackOnFailure, _ = cmd.Flags().GetBool(migrateNameCfg.LogFile.LongHand)
		globalApp.MigrateFlags.ResetDirtyFlag, _ = cmd.Flags().GetBool(migrateNameCfg.ResetDirtyFlag.LongHand)
		//globalApp.MigrateFlags.MigrateDownIfDirty, _ = cmd.Flags().GetBool(migrateNameCfg.MigrateDownOnDirty.LongHand)

		if targetVersion != -1 {
			globalApp.MigrateFlags.TargetVersion = targetVersion
		}
		if migrationDir != "" {
			globalApp.MigrateFlags.MigrationsDir = migrationDir
		}
		if migrationsProtocol != "" {
			globalApp.MigrateFlags.MigrationsProtocol = cdbmutil.MigrationsProtocol(migrationsProtocol)
		} else if globalApp.MigrateFlags.MigrationsProtocol == "" {
			globalApp.MigrateFlags.MigrationsProtocol = cdbmutil.FileProtocol
		}

		if globalApp.MigrateFlags.MigrationsDir == "" {
			return fmt.Errorf("--migration-dir is required")
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		defer globalApp.DB.Close()
		return globalApp.Migrate(
			cdbmutil.DefaultGetMigrationFunc,
			cdbmutil.DefaultFileMigrationFunc,
			map[int]cdbmutil.CustomMigration{},
		)
	},
}

func init() {
	//fmt.Printf("migrate init called\n")

	rootCmd.AddCommand(migrateCmd)

	migrateCmd.Flags().IntP(
		migrateNameCfg.TargetVersion.LongHand,
		migrateNameCfg.TargetVersion.ShortHand,
		-1,
		"Migrate to specific version",
	)
	migrateCmd.Flags().StringP(
		migrateNameCfg.LogFile.LongHand,
		migrateNameCfg.LogFile.ShortHand,
		"",
		"Directory where migration files are located",
	)
	migrateCmd.Flags().StringP(
		migrateNameCfg.MigrationsProtocol.LongHand,
		migrateNameCfg.MigrationsProtocol.ShortHand,
		"",
		"Protocol used for connecting to migrations directory",
	)
	migrateCmd.Flags().BoolP(
		migrateNameCfg.RollbackOnFailure.LongHand,
		migrateNameCfg.RollbackOnFailure.ShortHand,
		false,
		"When set will rollback to version that was set before migration",
	)
	migrateCmd.Flags().BoolP(
		migrateNameCfg.ResetDirtyFlag.LongHand,
		migrateNameCfg.ResetDirtyFlag.ShortHand,
		false,
		"When set will reset dirty flag when migrating",
	)
	migrateCmd.Flags().BoolP(
		migrateNameCfg.MigrateDownOnDirty.LongHand,
		migrateNameCfg.MigrateDownOnDirty.ShortHand,
		false,
		"When set will use the current down migration of file or custom migration if migration table is in dirty state",
	)
}
