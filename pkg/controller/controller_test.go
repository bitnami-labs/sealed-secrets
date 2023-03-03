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
	"k8s.io/client-go/kubernetes/fake"

	ssfake "github.com/bitnami-labs/sealed-secrets/pkg/client/clientset/versioned/fake"
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
	clientset := fake.NewSimpleClientset()
	ssc := ssfake.NewSimpleClientset()
	sinformer := initSecretInformerFactory(clientset, ns, tweakopts, false /* skip-recreate */)
	if sinformer == nil {
		t.Fatalf("sinformer %v want non nil", sinformer)
	}
	ssinformer := ssinformers.NewFilteredSharedInformerFactory(ssc, 0, ns, tweakopts)
	keyRegistry := testKeyRegister(t, context.Background(), clientset, ns)

	_, got := NewController(clientset, ssc, ssinformer, sinformer, keyRegistry)
	if got != nil {
		t.Fatalf("got %v want %v", got, nil)
	}
}

func TestSkipReleaseConfigDoesSkipIt(t *testing.T) {
	ns := "some-namespace"
	var tweakopts func(*metav1.ListOptions)
	clientset := fake.NewSimpleClientset()
	ssc := ssfake.NewSimpleClientset()
	sinformer := initSecretInformerFactory(clientset, ns, tweakopts, true /* skip-recreate */)
	if sinformer != nil {
		t.Fatalf("sinformer %v want nil", sinformer)
	}
	ssinformer := ssinformers.NewFilteredSharedInformerFactory(ssc, 0, ns, tweakopts)
	keyRegistry := testKeyRegister(t, context.Background(), clientset, ns)

	_, got := NewController(clientset, ssc, ssinformer, sinformer, keyRegistry)
	if got != nil {
		t.Fatalf("got %v want %v", got, nil)
	}
}

func testKeyRegister(t *testing.T, ctx context.Context, clientset kubernetes.Interface, ns string) *KeyRegistry {
	t.Helper()

	keyLabel := SealedSecretsKeyLabel
	prefix := "test-keys"
	testKeySize := 4096
	keyRegistry, err := initKeyRegistry(ctx, clientset, rand.Reader, ns, prefix, keyLabel, testKeySize)
	if err != nil {
		t.Fatalf("failed to provision key registry: %v", err)
	}
	return keyRegistry
}
