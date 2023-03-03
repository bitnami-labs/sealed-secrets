package controller

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"testing"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealedsecrets/v1alpha1"
	ssinformers "github.com/bitnami-labs/sealed-secrets/pkg/client/informers/externalversions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func TestConvert2SealedSecretBadType(t *testing.T) {
	obj := struct{}{}
	_, got := convertSealedSecret(obj)
	want := ErrCast
	if !errors.Is(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestConvert2SealedSecretFills(t *testing.T) {
	sealedSecret := ssv1alpha1.SealedSecret{}

	result, err := convertSealedSecret(any(&sealedSecret))
	if err != nil {
		t.Fatalf("unexpected failure converting to a sealed secret: %v", err)
	}
	got := fmt.Sprintf("%s %s", result.APIVersion, result.Kind)
	want := "bitnami.com/v1alpha1 SealedSecret"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestConvert2SealedSecretPassThrough(t *testing.T) {
	sealedSecret := ssv1alpha1.SealedSecret{}
	sealedSecret.APIVersion = "bitnami.com/v1alpha1"
	sealedSecret.Kind = "SealedSecrets"

	want := &sealedSecret
	got, err := convertSealedSecret(any(want))
	if err != nil {
		t.Fatalf("unexpected failure converting to a sealed secret: %v", err)
	}
	if got != want {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestDefaultConfigDoesNotSkipRecreate(t *testing.T) {
	ns := "some-namespace"
	var tweakopts func(*metav1.ListOptions)
	conf := clusterConfig(t)
	clientset := clientSetOrDie(conf)
	ssc := ssclient.NewForConfigOrDie(conf)
	sinformer := InitSecretInformerFactory(clientset, ns, tweakopts, false /* skip-recreate */)
	ssinformer := ssinformers.NewFilteredSharedInformerFactory(ssc, 0, ns, tweakopts)
	keyRegistry := keyRegister(t, context.Background(), clientset, ns)

	_, got := NewController(clientset, ssc, ssinformer, sinformer, keyRegistry)
	if got != nil {
		t.Fatalf("got %v want %v", got, nil)
	}

}

func clusterConfig(t *testing.T) *rest.Config {
	t.Helper()

	var config *rest.Config
	var err error

	if *kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	}
	if err != nil {
		t.Fatalf("failed to setup kubeconfig", err)
	}

	return config
}

func keyRegister(t *testing.T, ctx context.Context, clientset *kubernetes.Clientset, ns string) *controller.KeyRegistry {
	t.Helper()

	keyLabel := controller.SealedSecretsKeyLabel
	prefix := "test-keys"
	testKeySize := 4096
	fmt.Fprintf(GinkgoWriter, "initiating key registry\n")
	keyRegistry, err := controller.InitKeyRegistry(ctx, clientset, rand.Reader, ns, prefix, keyLabel, testKeySize)
	if err != nil {
		t.Fatalf("failed to provision key registry: %v", err)
	}
	return keyRegistry
}
