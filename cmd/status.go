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
	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Displays current migration status",
	Long: `Displays current migration status depending on whether there is an entry or not

If no entry, simply displays "No migration entry"
If there is entry, will display: "migration state - version:%d / dirty:%v / dirty state:%s"
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return globalApp.Status()
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
