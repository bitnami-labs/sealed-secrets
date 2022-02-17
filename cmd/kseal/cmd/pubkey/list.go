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

package pubkey

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"sigs.k8s.io/yaml"
)

// NewCmdPubkeyList creates a command object for the "pubkey list" action.
func NewCmdPubkeyList() *cobra.Command {
	pubkeyListCmd := &cobra.Command{
		Use:   "list",
		Short: "List the available public keys in the controller",
		Long: `List the available public keys in the Sealed Secrets controller key set.

Examples:

    kseal pubkey list              List the available public keys in the controller.
	kseal pubkey list -o json      List the available public keys in the controller in JSON format.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, _ := cmd.Flags().GetString("output")
			controllerName := viper.GetString("controller-name")
			controllerNamespace := viper.GetString("controller-namespace")
			keyset := []string{"foo", "bar"}
			fmt.Printf("I will list the keys in the controller \"%s\" in the namespace \"%s\"\n", controllerName, controllerNamespace)
			switch format {
			case "plaintext":
				fmt.Println("Key set:")
				for i, k := range keyset {
					fmt.Printf("[%d] -> %s\n", i, k)
				}
			case "yaml", "json":
				buf, err := json.MarshalIndent(keyset, "", "    ")
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

	// Flags particular to 'pubkey list' command
	pubkeyListCmd.Flags().StringP("output", "o", "plaintext", "Output format.  Supported values are: json, yaml, plaintext")

	return pubkeyListCmd
}
