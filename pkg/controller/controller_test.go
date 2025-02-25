package controller

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"testing"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealedsecrets/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	ssfake "github.com/bitnami-labs/sealed-secrets/pkg/client/clientset/versioned/fake"
)

func TestIsAnnotatedToBePatched(t *testing.T) {
	tests := []struct {
		annotations map[string]string
		want        bool
	}{
		{annotations: map[string]string{ssv1alpha1.SealedSecretPatchAnnotation: "true"}, want: true},
		{annotations: map[string]string{ssv1alpha1.SealedSecretPatchAnnotation: "TRUE"}, want: false},
		{annotations: map[string]string{ssv1alpha1.SealedSecretPatchAnnotation: "false"}, want: false},
		{annotations: map[string]string{ssv1alpha1.SealedSecretPatchAnnotation: ""}, want: false},
		{annotations: map[string]string{"something": "else"}, want: false},
		{annotations: map[string]string{}, want: false},
	}

	for i, tc := range tests {
		s := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   "test-ns",
				Name:        "test-secret",
				Annotations: tc.annotations,
			},
			Data: map[string][]byte{
				"foo": []byte("bar"),
			},
		}

		got := isAnnotatedToBePatched(s)
		if got != tc.want {
			t.Fatalf("test %d: expected: %v, got: %v", i+1, tc.want, got)
		}
	}
}

func TestIsAnnotatedToBeManaged(t *testing.T) {
	tests := []struct {
		annotations map[string]string
		want        bool
	}{
		{annotations: map[string]string{ssv1alpha1.SealedSecretManagedAnnotation: "true"}, want: true},
		{annotations: map[string]string{ssv1alpha1.SealedSecretManagedAnnotation: "TRUE"}, want: false},
		{annotations: map[string]string{ssv1alpha1.SealedSecretManagedAnnotation: "false"}, want: false},
		{annotations: map[string]string{ssv1alpha1.SealedSecretManagedAnnotation: ""}, want: false},
		{annotations: map[string]string{"something": "else"}, want: false},
		{annotations: map[string]string{}, want: false},
	}

	for i, tc := range tests {
		s := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   "test-ns",
				Name:        "test-secret",
				Annotations: tc.annotations,
			},
			Data: map[string][]byte{
				"foo": []byte("bar"),
			},
		}

		got := isAnnotatedToBeManaged(s)
		if got != tc.want {
			t.Fatalf("test %d: expected: %v, got: %v", i+1, tc.want, got)
		}
	}
}

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
	keyRegistry := testKeyRegister(t, context.Background(), clientset, ns)

	got, err := prepareController(clientset, ns, tweakopts, &Flags{SkipRecreate: false}, ssc, keyRegistry)
	if err != nil {
		t.Fatalf("err %v want %v", got, nil)
	}
	if got == nil {
		t.Fatalf("ctrl %v want non nil", got)
	}
	if got.sInformer == nil {
		t.Fatalf("sInformer %v want non nil", got.sInformer)
	}
}

func TestSkipRecreateConfigDoesSkipIt(t *testing.T) {
	ns := "some-namespace"
	var tweakopts func(*metav1.ListOptions)
	clientset := fake.NewSimpleClientset()
	ssc := ssfake.NewSimpleClientset()
	keyRegistry := testKeyRegister(t, context.Background(), clientset, ns)

	got, err := prepareController(clientset, ns, tweakopts, &Flags{SkipRecreate: true}, ssc, keyRegistry)
	if err != nil {
		t.Fatalf("err %v want %v", got, nil)
	}
	if got == nil {
		t.Fatalf("ctrl %v want non nil", got)
	}
	if got.sInformer != nil {
		t.Fatalf("sInformer %v want nil", got.sInformer)
	}
}

func TestEmptyStatusSendsUpdate(t *testing.T) {
	updateRequired := updateSealedSecretsStatusConditions(&ssv1alpha1.SealedSecretStatus{}, nil)

	if !updateRequired {
		t.Fatalf("expected status update, but no update was send")
	}
}

func TestStatusUpdateSendsUpdate(t *testing.T) {
	status := &ssv1alpha1.SealedSecretStatus{
		Conditions: []ssv1alpha1.SealedSecretCondition{{
			Status:         "False",
			Type:           ssv1alpha1.SealedSecretSynced,
			LastUpdateTime: metav1.Now(),
		}},
	}
	updateRequired := updateSealedSecretsStatusConditions(status, nil)

	if !updateRequired {
		t.Fatalf("expected status update, but no update was send")
	}

	if status.Conditions[0].LastTransitionTime.IsZero() {
		t.Fatalf("expected LastTransitionTime is not empty")
	}

	if status.Conditions[0].LastUpdateTime.IsZero() {
		t.Fatalf("expected LastUpdateTime is not empty")
	}
}

func TestSameStatusNoUpdate(t *testing.T) {
	updateRequired := updateSealedSecretsStatusConditions(&ssv1alpha1.SealedSecretStatus{
		Conditions: []ssv1alpha1.SealedSecretCondition{{
			Type:   ssv1alpha1.SealedSecretSynced,
			Status: "False",
		}},
	}, errors.New("testerror"))

	if updateRequired {
		t.Fatalf("expected no status update, but update was send")
	}
}

func TestSyncedSecretWithErrorSendsUpdate(t *testing.T) {
	updateRequired := updateSealedSecretsStatusConditions(&ssv1alpha1.SealedSecretStatus{
		Conditions: []ssv1alpha1.SealedSecretCondition{{
			Type:   ssv1alpha1.SealedSecretSynced,
			Status: "True",
		}},
	}, errors.New("testerror"))

	if !updateRequired {
		t.Fatalf("expected status update, but no update was send")
	}
}

func testKeyRegister(t *testing.T, ctx context.Context, clientset kubernetes.Interface, ns string) *KeyRegistry {
	t.Helper()

	keyLabel := SealedSecretsKeyLabel
	prefix := "test-keys"
	testKeySize := 4096
	keyRegistry, err := initKeyRegistry(ctx, clientset, rand.Reader, ns, prefix, keyLabel, testKeySize, "CertNotBefore")
	if err != nil {
		t.Fatalf("failed to provision key registry: %v", err)
	}
	return keyRegistry
}
