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
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"sigs.k8s.io/yaml"
)

type config struct {
	ControllerName      string `mapstructure:"controller-name"`
	ControllerNamespace string `mapstructure:"controller-namespace"`
}

// NewCmdConfigView creates a command object for the "config view" action.
func NewCmdConfigView() *cobra.Command {
	setViewCmd := &cobra.Command{
		Use:   "view",
		Short: "Display current configuration",
		Long: `Display current configuration.

Examples:

	kseal config view                          Display configuration in the default configuration file.
	kseal config view -o json                  Display configuration in the default configuration file in JSON format.
	kseal config view --config "~/.foo.yaml"   Display configuration in the "~/.foo.yaml" configuration file.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, _ := cmd.Flags().GetString("output")
			var c config
			if err := viper.Unmarshal(&c); err != nil {
				return err
			}
			switch format {
			case "plaintext":
				fmt.Println("Current configuration:")
				fmt.Printf("-> Controller Name: %s\n", c.ControllerName)
				fmt.Printf("-> Controller Namespace: %s\n", c.ControllerNamespace)
			case "yaml", "json":
				buf, err := json.MarshalIndent(c, "", "    ")
				if err != nil {
					return err
				}
				if format == "yaml" {
					buf, err = yaml.JSONToYAML(buf)
					if err != nil {
						return err
					}
				}
				fmt.Println(string(buf))
			default:
				return errors.New("Unknown output format: %s" + format)
			}
			return nil
		},
	}
	// Flags particular to 'config view' command
	setViewCmd.Flags().StringP("output", "o", "plaintext", "Output format.  Supported values are: json, yaml, plaintext")

	return setViewCmd
}
