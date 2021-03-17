// Copyright © 2019 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"os"

	"github.com/TravisS25/cdbm/app"
	"github.com/spf13/cobra"
)

type rootNameConfig struct {
	DBProtocol    flagName
	Env           flagName
	Database      flagName
	Host          flagName
	User          flagName
	Password      flagName
	Port          flagName
	SSLMode       flagName
	SSLRootCert   flagName
	SSLKey        flagName
	SSLCert       flagName
	SSL           flagName
	UseFileOnFail flagName
}

var rootNameCfg = rootNameConfig{
	DBProtocol: flagName{
		LongHand:  "db-protocol",
		ShortHand: "b",
	},
	Env: flagName{
		LongHand:  "env",
		ShortHand: "e",
	},
	Database: flagName{
		LongHand:  "database",
		ShortHand: "d",
	},
	Host: flagName{
		LongHand:  "host",
		ShortHand: "h",
	},
	User: flagName{
		LongHand:  "user",
		ShortHand: "u",
	},
	Password: flagName{
		LongHand:  "password",
		ShortHand: "w",
	},
	Port: flagName{
		LongHand:  "port",
		ShortHand: "p",
	},
	SSLMode: flagName{
		LongHand:  "ssl-mode",
		ShortHand: "m",
	},
	SSLRootCert: flagName{
		LongHand:  "ssl-root-cert",
		ShortHand: "r",
	},
	SSLKey: flagName{
		LongHand:  "ssl-key",
		ShortHand: "k",
	},
	SSLCert: flagName{
		LongHand:  "ssl-cert",
		ShortHand: "c",
	},
	SSL: flagName{
		LongHand:  "ssl",
		ShortHand: "s",
	},
	UseFileOnFail: flagName{
		LongHand:  "use-file-on-fail",
		ShortHand: "f",
	},
}

var globalApp *app.CDBM

var rootFlagsCfg app.RootFlagsConfig

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "cdbm",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// RunE: func(cmd *cobra.Command, args []string) error {
	// 	return nil
	// },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	//fmt.Printf("root init called\n")
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(
		&rootFlagsCfg.DBProtocol,
		rootNameCfg.DBProtocol.LongHand,
		"",
		"Protocol of connection string used to migrate database.  Available values: postgres | cockroachdb",
	)
	rootCmd.PersistentFlags().StringVar(&rootFlagsCfg.EnvVar, rootNameCfg.Env.LongHand, "", "Enviroment variable that points to config file")
	rootCmd.PersistentFlags().StringVar(&rootFlagsCfg.Database, rootNameCfg.Database.LongHand, "", "Name of database to connect to")
	rootCmd.PersistentFlags().IntVar(&rootFlagsCfg.Port, rootNameCfg.Port.LongHand, -1, "Port of database to connect to")
	rootCmd.PersistentFlags().StringVar(&rootFlagsCfg.Host, rootNameCfg.Host.LongHand, "", "Host of database to connect to")
	rootCmd.PersistentFlags().StringVar(&rootFlagsCfg.User, rootNameCfg.User.LongHand, "", "User of database to connect to")
	rootCmd.PersistentFlags().StringVar(&rootFlagsCfg.Password, rootNameCfg.Password.LongHand, "", "Password of database to connect to")
	rootCmd.PersistentFlags().StringVar(&rootFlagsCfg.SSLMode, rootNameCfg.SSLMode.LongHand, "", "SSL mode to use when connecting to database Ex. sslmode=require")
	rootCmd.PersistentFlags().StringVar(&rootFlagsCfg.SSLRootCert, rootNameCfg.SSLRootCert.LongHand, "", "File path where ssl root cert is located")
	rootCmd.PersistentFlags().StringVar(&rootFlagsCfg.SSLKey, rootNameCfg.SSLKey.LongHand, "", "File path where private key is located")
	rootCmd.PersistentFlags().StringVar(&rootFlagsCfg.SSLCert, rootNameCfg.SSLCert.LongHand, "", "File path where ssl cert is located")
	rootCmd.PersistentFlags().BoolVar(&rootFlagsCfg.SSL, rootNameCfg.SSL.LongHand, false, "Determine whether to use ssl when connecting to database")
	rootCmd.PersistentFlags().BoolVar(
		&rootFlagsCfg.UseFileOnFail,
		rootNameCfg.UseFileOnFail.LongHand,
		false,
		"If user enters database credentials through command line and connection fails resort to using config file credentials when this is set",
	)

	rootCmd.MarkFlagRequired(rootNameCfg.DBProtocol.LongHand)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	var err error

	if globalApp, err = app.NewCDBM(rootFlagsCfg, nil); err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}
}
