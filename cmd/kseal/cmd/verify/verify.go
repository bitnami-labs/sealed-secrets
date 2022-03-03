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

package verify

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	kc "github.com/bitnami-labs/sealed-secrets/cmd/kseal/pkg/config"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/net"
)

// NewCmdVerify creates a command object for the "verify" action.
func NewCmdVerify() *cobra.Command {
	verifyCmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify a Sealed Secret",
		Long: `Verify a Sealed Secret consulting the Sealed Secrets controller API

Examples:

    kseal verify -f mysealedsecret.json            Verify Sealed Secret from file
    cat mysealedsecret.json | kseal verify         Verify Sealed Secret from stdin
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			filename, _ := cmd.Flags().GetString("filename")
			var input io.Reader
			if filename == "" || filename == "-" {
				if isatty.IsTerminal(os.Stdin.Fd()) {
					return fmt.Errorf("tty detected: expecting json/yaml k8s resource in stdin")
				}
				input = os.Stdin
			} else {
				f, err := os.Open(filename)
				if err != nil {
					return fmt.Errorf("cannot open file: %v", err)
				}
				defer f.Close()
				input = f
			}
			kc.GlobalConfig.ControllerNamespace = viper.GetString("controller-namespace")
			kc.GlobalConfig.ControllerName = viper.GetString("controller-name")
			err := verify(kc.GlobalConfig, input)
			if err != nil {
				return err
			}
			return nil
		},
	}

	// Initialize K8s Config
	cobra.OnInitialize(kc.InitConfig)
	// Flags
	verifyCmd.Flags().StringP("filename", "f", "", "Filename that contains the Sealed Secrets object to verify")

	return verifyCmd
}

// verify verifies a Sealed Secrets object by performing a POST
// HTTP request to the controller through the k8s API proxy.
func verify(c kc.Config, in io.Reader) error {
	portName, err := c.GetServicePortName()
	if err != nil {
		return fmt.Errorf("cannot get controller service port: %v", err)
	}
	content, err := ioutil.ReadAll(in)
	if err != nil {
		return fmt.Errorf("cannot read object to validate: %v", err)
	}
	req := c.K8sClient.RESTClient().Post().
		Namespace(c.ControllerNamespace).
		Resource("services").
		SubResource("proxy").
		Name(net.JoinSchemeNamePort("http", c.ControllerName, portName)).
		Suffix("/v1/verify")
	req.Body(content)
	res := req.Do(context.Background())
	if err := res.Error(); err != nil {
		if status, ok := err.(*k8serrors.StatusError); ok && status.Status().Code == http.StatusConflict {
			return fmt.Errorf("unable to decrypt sealed secret")
		}
		return fmt.Errorf("cannot verify sealed secret: %v", err)
	}
	return nil
}
