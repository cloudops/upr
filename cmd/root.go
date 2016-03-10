// Copyright © 2016 Will Stevens <wstevens@cloudops.com>
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

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// This represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "upr",
	Short: "Manipulate pull requests on GitHub",
	Long: `A command line tool to manipulate pull requests on Github.
	
This tool is designed to be integrated into a CI implementation
in order to update the Status or add a Comment.`,
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().String("config", "", "config file (default is ./config.yaml)")
	RootCmd.PersistentFlags().StringP("commit", "c", "", "commit you are working with")
	RootCmd.PersistentFlags().String("token", "", "required: Github access token (https://github.com/settings/tokens)")
	RootCmd.PersistentFlags().String("owner", "", "required: owner of the repo you are working with")
	RootCmd.PersistentFlags().String("repo", "", "required: name of the repo you are working with")
	viper.BindPFlag("commit", RootCmd.PersistentFlags().Lookup("commit"))
	viper.BindPFlag("token", RootCmd.PersistentFlags().Lookup("token"))
	viper.BindPFlag("owner", RootCmd.PersistentFlags().Lookup("owner"))
	viper.BindPFlag("repo", RootCmd.PersistentFlags().Lookup("repo"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	cfg_file := viper.GetString("config")
	if cfg_file != "" { // enable ability to specify config file via flag
		viper.SetConfigFile(cfg_file)
	}

	viper.SetConfigName("config") // name of config file (without extension)
	viper.AddConfigPath(".")      // optionally look for config in the working directory
	viper.AutomaticEnv()          // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
