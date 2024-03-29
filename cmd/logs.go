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

	"github.com/spf13/cobra"
)

type logsNameConfig struct {
	LogFile flagName
}

var logsNameCfg = migrateNameConfig{
	LogFile: flagName{
		LongHand:  "migrations-dir",
		ShortHand: "m",
	},
}

// logsCmd represents the logs command
var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Display error logs",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		migrationDir, _ := cmd.Flags().GetString(migrateNameCfg.LogFile.LongHand)

		if migrationDir != "" {
			globalApp.LogFlags.LogFile = migrationDir
		}

		if globalApp.LogFlags.LogFile == "" {
			return fmt.Errorf("--migrations-dir is required")
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return globalApp.Logs()
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)

	logsCmd.Flags().StringP(
		logsNameCfg.LogFile.LongHand,
		logsNameCfg.LogFile.ShortHand,
		"",
		"Directory where migration files are located",
	)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// logsCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// logsCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
