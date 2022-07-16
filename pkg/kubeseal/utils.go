package kubeseal

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// Used by: Encrypter, Validator, CertificateReader
func GetServicePortName(ctx context.Context, client corev1.CoreV1Interface, namespace, serviceName string) (string, error) {
	service, err := client.Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("cannot get sealed secret service: %v", err)
	}
	return service.Spec.Ports[0].Name, nil
}
