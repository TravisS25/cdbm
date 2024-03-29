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

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type dropNameConfig struct {
	Confirm flagName
}

var dropNameCfg = dropNameConfig{
	Confirm: flagName{
		LongHand:  "confirm",
		ShortHand: "c",
	},
}

// dropCmd represents the drop command
var dropCmd = &cobra.Command{
	Use:   "drop",
	Short: "Drops all tables in database",
	Long:  `Drops all tables in database.  This should only be used for testing purposes only`,
	PreRun: func(cmd *cobra.Command, args []string) {
		confirm, _ := cmd.Flags().GetBool(dropNameCfg.Confirm.LongHand)
		globalApp.DropFlags.Confirm = confirm
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if globalApp.DropFlags.Confirm {
			return globalApp.Drop()
		}

		var answer string
		var err error

		fmt.Printf("You are about to drop entire database.  Are you sure you want to continue (y/n)? ")

		for {
			if _, err = fmt.Scanln(&answer); err != nil {
				return errors.WithStack(err)
			}

			if answer == "y" || answer == "n" {
				break
			}

			fmt.Printf("(y/n)? \n")
		}

		if answer == "y" {
			return globalApp.Drop()
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(dropCmd)

	dropCmd.Flags().BoolP(
		dropNameCfg.Confirm.LongHand,
		dropNameCfg.Confirm.ShortHand,
		false,
		"Skips drop confirmation and drops all tables in database",
	)
}
