/*
Copyright 2022 - Bitnami <containers@bitnami.com>

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
	"os"
	"path/filepath"

	"github.com/bitnami-labs/sealed-secrets/cmd/kseal/cmd/config"
	"github.com/bitnami-labs/sealed-secrets/cmd/kseal/cmd/create"
	"github.com/bitnami-labs/sealed-secrets/cmd/kseal/cmd/pubkey"
	"github.com/bitnami-labs/sealed-secrets/cmd/kseal/cmd/verify"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// NewKsealCommand creates the `kseal` command and its nested children.
func NewKsealCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kseal",
		Short: "kseal is a CLI that uses asymmetric crypto to encrypt secrets that only the Sealed Secrets controller can decrypt.",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	// Flags common to all sub commands
	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.kseal/config)")
	// Subcommands
	cmd.AddCommand(config.NewCmdConfig())
	cmd.AddCommand(create.NewCmdCreate())
	cmd.AddCommand(pubkey.NewCmdPubkey())
	cmd.AddCommand(verify.NewCmdVerify())
	// Initialize configuration on every sub command
	cobra.OnInitialize(initConfig)

	return cmd
}

func er(msg interface{}) {
	fmt.Println("Error:", msg)
	os.Exit(1)
}

func initConfig() {
	// Find home directory.
	home, err := homedir.Dir()
	if err != nil {
		er(err)
	}
	ksealHome := filepath.Join(home, ".kseal")
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Search config in home directory with name "config" (without extension).
		viper.AddConfigPath(ksealHome)
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			if _, err := os.Stat(ksealHome); os.IsNotExist(err) {
				os.Mkdir(ksealHome, 0755)
			}
			if err := viper.WriteConfigAs(filepath.Join(ksealHome, "config")); err != nil {
				er(err)
			}
		} else {
			er(err)
		}
	}
}
