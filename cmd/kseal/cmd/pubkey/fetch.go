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
	"context"
	"fmt"
	"io"
	"os"

	kc "github.com/bitnami-labs/sealed-secrets/cmd/kseal/pkg/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NewCmdPubkeyFetch creates a command object for the "pubkey fetch" action.
func NewCmdPubkeyFetch() *cobra.Command {
	pubkeyFetchCmd := &cobra.Command{
		Use:   "fetch",
		Short: "Fetch latest public key from the controller",
		Long: `Fetch the latest public key to use to encrypt secrets from the Sealed Secrets controller

Examples:

    kseal pubkey fetch                  Fetch latest public key and write its content to stdout.
	kseal pubkey fetch > mycert.pem     Fetch latest public key and save it on a file.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			kc.GlobalConfig.ControllerNamespace = viper.GetString("controller-namespace")
			kc.GlobalConfig.ControllerName = viper.GetString("controller-name")
			key, err := fetchLatestKey(kc.GlobalConfig)
			if err != nil {
				return err
			}
			defer key.Close()
			_, err = io.Copy(os.Stdout, key)
			if err != nil {
				return err
			}

			return nil
		},
	}

	return pubkeyFetchCmd
}

// fetchLatestKey fetches the latest public key by performing an
// HTTP request to the controller through the k8s API proxy.
func fetchLatestKey(c kc.Config) (io.ReadCloser, error) {
	portName, err := c.GetServicePortName()
	if err != nil {
		return nil, fmt.Errorf("cannot get controller service port: %v", err)
	}
	cert, err := c.K8sClient.
		Services(c.ControllerNamespace).
		ProxyGet("http", c.ControllerName, portName, "/v1/cert.pem", nil).
		Stream(context.Background())
	if err != nil {
		return nil, fmt.Errorf("cannot fetch certificate: %v", err)
	}
	return cert, nil
}
