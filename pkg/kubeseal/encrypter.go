package kubeseal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealed-secrets/v1alpha1"
	"github.com/bitnami-labs/sealed-secrets/pkg/crypto"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/net"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

type SecretItemEncryptionInstruction struct {
	Out        io.Writer
	SecretName string
	Namespace  string
	Data       []byte
	Scope      ssv1alpha1.SealingScope
	PubKey     *rsa.PublicKey
}

func EncryptSecretItem(i SecretItemEncryptionInstruction) error {
	// TODO(mkm): refactor cluster-wide/namespace-wide to an actual enum so we can have a simple flag
	// to refer to the scope mode that is not a tuple of booleans.
	label := ssv1alpha1.EncryptionLabel(i.Namespace, i.SecretName, i.Scope)
	out, err := crypto.HybridEncrypt(rand.Reader, i.PubKey, i.Data, label)
	if err != nil {
		return err
	}
	fmt.Fprint(i.Out, base64.StdEncoding.EncodeToString(out))

	return nil
}

type ReEncryptSealedSecretInstruction struct {
	OutputFormat string
	Ctx          context.Context
	In           io.Reader
	Out          io.Writer
	Codecs       runtimeserializer.CodecFactory
	Namespace    string
	Name         string
	RestConfig   *rest.Config
}

func ReEncryptSealedSecret(i ReEncryptSealedSecretInstruction) error {
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
		Suffix("/v1/rotate")

	req.Body(content)
	res := req.Do(i.Ctx)
	if err := res.Error(); err != nil {
		if status, ok := err.(*k8serrors.StatusError); ok && status.Status().Code == http.StatusConflict {
			return fmt.Errorf("unable to rotate secret")
		}
		return fmt.Errorf("cannot re-encrypt secret: %v", err)
	}
	body, err := res.Raw()
	if err != nil {
		return err
	}
	ssecret := &ssv1alpha1.SealedSecret{}
	if err = json.Unmarshal(body, ssecret); err != nil {
		return err
	}
	ssecret.SetCreationTimestamp(metav1.Time{})
	ssecret.SetDeletionTimestamp(nil)
	ssecret.Generation = 0
	return ResourceOutput(i.Out, i.Codecs, ssv1alpha1.SchemeGroupVersion, ssecret, i.OutputFormat)
}
