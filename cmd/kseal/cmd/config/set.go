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
	"github.com/spf13/viper"
)

// NewCmdConfigSet creates a command object for the "config set" action.
func NewCmdConfigSet() *cobra.Command {
	setCmd := &cobra.Command{
		Use:   "set",
		Short: "Set an individual value in the kseal configuration file",
		Long: `Set an individual value in the kseal configuration file

Examples:

	kseal config set controller-name "sealed-secrets"                          Set "controller-name" to "sealed-secrets" in the default configuration file.
	kseal config set controller-name "sealed-secrets" --config "~/.foo.yaml"   Set "controller-name" to "sealed-secrets" in the "~/.foo.yaml" configuration file.
`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			viper.Set(args[0], args[1])
			if err := viper.WriteConfig(); err != nil {
				return err
			}
			return nil
		},
	}

	return setCmd
}
