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

package fetch

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NewCmdFetch creates a command object for the "fetch" action.
func NewCmdFetch() *cobra.Command {
	fetchCmd := &cobra.Command{
		Use:   "fetch",
		Short: "Fetch public key from the controller",
		Long: `Fetch the public key to use to encrypt secrets from the Sealed Secrets controller

Examples:

    kseal fetch                  Fetch public key and write its content to stdout.
	kseal fetch > mycert.pem     Fetch public key and save it on a file.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			controllerName := viper.GetString("controller-name")
			controllerNamespace := viper.GetString("controller-namespace")
			fmt.Printf("I will fetch key from the controller \"%s\" in the namespace \"%s\"\n", controllerName, controllerNamespace)
			return nil
		},
	}

	return fetchCmd
}
