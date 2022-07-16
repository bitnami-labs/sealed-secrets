package kubeseal

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/net"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

type ValidateSealedSecretInstruction struct {
	Ctx        context.Context
	In         io.Reader
	Namespace  string
	Name       string
	RestConfig *rest.Config
}

func ValidateSealedSecret(i ValidateSealedSecretInstruction) error {
	restClient, err := corev1.NewForConfig(i.RestConfig)
	if err != nil {
		return err
	}
	portName, err := GetServicePortName(i.Ctx, restClient, i.Namespace, i.Name)
	if err != nil {
		return err
	}

	content, err := ioutil.ReadAll(i.In)
	if err != nil {
		return err
	}

	req := restClient.RESTClient().Post().
		Namespace(i.Namespace).
		Resource("services").
		SubResource("proxy").
		Name(net.JoinSchemeNamePort("http", i.Name, portName)).
		Suffix("/v1/verify")

	req.Body(content)
	res := req.Do(i.Ctx)
	if err := res.Error(); err != nil {
		if status, ok := err.(*k8serrors.StatusError); ok && status.Status().Code == http.StatusConflict {
			return fmt.Errorf("unable to decrypt sealed secret")
		}
		return fmt.Errorf("cannot validate sealed secret: %v", err)
	}

	return nil
}
