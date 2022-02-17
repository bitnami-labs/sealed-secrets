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

package encrypt

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NewCmdEncrypt creates a command object for the "encrypt" action.
func NewCmdEncrypt() *cobra.Command {
	encryptCmd := &cobra.Command{
		Use:   "encrypt",
		Short: "Encrypt a secret",
		Long: `Encrypt a secret using a public key.

Examples:

    kubectl create secret generic secret-name --dry-run=client --from-literal=foo=bar | \
        kseal encrypt > mysealedsecret.json
    kubectl create secret generic secret-name --dry-run=client --from-literal=foo=bar -o yaml | \
        kseal encrypt -o yaml > mysealedsecret.yaml
    kubectl create secret generic secret-name --dry-run=client --from-literal=foo=bar | \
        kseal encrypt -c mycert.pem > mysealedsecret.json
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			output, _ := cmd.Flags().GetString("output")
			certificate, _ := cmd.Flags().GetString("cert")
			controllerName := viper.GetString("controller-name")
			controllerNamespace := viper.GetString("controller-namespace")
			if certificate != "" {
				fmt.Printf("I will encrypt the secret with the cert \"%s\". Output format is \"%s\"\n", certificate, output)
				return nil
			}
			fmt.Printf("I will encrypt the secret fetching the key from the controller \"%s\" in the namespace \"%s\". Output format is \"%s\"\n", controllerName, controllerNamespace, output)
			return nil
		},
	}

	// Flags common to all sub commands
	encryptCmd.PersistentFlags().StringP("cert", "n", "", "Certificate / public key file/URL to use for encryption")
	encryptCmd.PersistentFlags().StringP("output", "o", "json", "Output format. Supported values are: json, yaml")

	return encryptCmd
}
