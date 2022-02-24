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

package config

import (
	"github.com/spf13/cobra"
)

// NewCmdConfig creates a command object for the "config" action, and adds all child commands to it.
func NewCmdConfig() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Configure kseal",
		Long: `Modify kseal configuration files using subcommands like "kseal config set controller-name [CONTROLLER_NAME]"

Examples:

    kseal config set controller-name "sealed-secrets"     Set "controller-name" to "sealed-secrets" in the configuration file.
    kseal config view --config "~/.foo.yaml"              Display settings in the "~/.foo.yaml" configuration file.
`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	// Subcommands
	configCmd.AddCommand(NewCmdConfigSet())
	configCmd.AddCommand(NewCmdConfigView())
	return configCmd
}
