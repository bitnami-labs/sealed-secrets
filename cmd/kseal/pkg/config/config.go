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
	"context"
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"

	// Register Auth providers
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

// Config represents the configuration to use by kseal to
// interact with the Sealed Secrets controller
type Config struct {
	K8sClient           *corev1.CoreV1Client
	ControllerNamespace string
	ControllerName      string
}

var GlobalConfig Config

// InitConfig creates a kseal config object
func InitConfig() {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	k8sClientConfig := clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{}, os.Stdin)
	restConf, err := k8sClientConfig.ClientConfig()
	if err != nil {
		panic(err)
	}
	restClient, err := corev1.NewForConfig(restConf)
	if err != nil {
		panic(err)
	}
	GlobalConfig = Config{
		K8sClient: restClient,
	}
}

// GetServicePortName obtains the Sealed Secrets controller service port name
func (c Config) GetServicePortName() (string, error) {
	service, err := c.K8sClient.
		Services(c.ControllerNamespace).
		Get(context.Background(), c.ControllerName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("cannot get controller service: %v", err)
	}
	return service.Spec.Ports[0].Name, nil
}
