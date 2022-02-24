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

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	// Register Auth providers
	_ "k8s.io/client-go/plugin/pkg/client/auth"
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
			// Obtain K8s REST config
			restConf, err := k8sClientConfig.ClientConfig()
			if err != nil {
				return fmt.Errorf("cannot obtain k8s config: %v", err)
			}
			// Create kseal config object
			kc := &KsealConfig{
				K8sConfig:           restConf,
				ControllerNamespace: viper.GetString("controller-namespace"),
				ControllerName:      viper.GetString("controller-name"),
			}
			key, err := kc.fetchLatestKey()
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

// getServicePortName obtains the controller service port name
func (kc *KsealConfig) getServicePortName() (string, error) {
	restClient, err := corev1.NewForConfig(kc.K8sConfig)
	if err != nil {
		return "", fmt.Errorf("cannot create k8s client: %v", err)
	}
	service, err := restClient.
		Services(kc.ControllerNamespace).
		Get(context.Background(), kc.ControllerName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("cannot get controller service: %v", err)
	}
	return service.Spec.Ports[0].Name, nil
}

// fetchLatestKey fetches the latest public key by performing an
// HTTP request to the controller through the k8s API proxy.
func (kc *KsealConfig) fetchLatestKey() (io.ReadCloser, error) {
	portName, err := kc.getServicePortName()
	if err != nil {
		return nil, fmt.Errorf("cannot get controller service port: %v", err)
	}
	kc.K8sConfig.AcceptContentTypes = "application/x-pem-file, */*"
	restClient, err := corev1.NewForConfig(kc.K8sConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot create k8s client: %v", err)
	}
	cert, err := restClient.
		Services(kc.ControllerNamespace).
		ProxyGet("http", kc.ControllerName, portName, "/v1/cert.pem", nil).
		Stream(context.Background())
	if err != nil {
		return nil, fmt.Errorf("cannot fetch certificate: %v", err)
	}
	return cert, nil
}
