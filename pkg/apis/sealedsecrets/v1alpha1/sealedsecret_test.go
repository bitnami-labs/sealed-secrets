package v1alpha1

import (
	"bytes"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"io"
	mathrand "math/rand"
	"reflect"
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/bitnami-labs/sealed-secrets/pkg/crypto"

	// Install standard API types.
	_ "k8s.io/client-go/kubernetes"
)

var _ runtime.Object = &SealedSecret{}
var _ metav1.ObjectMetaAccessor = &SealedSecret{}
var _ runtime.Object = &SealedSecretList{}
var _ metav1.ListMetaAccessor = &SealedSecretList{}

func TestSealingScope(t *testing.T) {
	testCases := []struct {
		scope SealingScope
		name  string
	}{
		{StrictScope, "strict"},
		{NamespaceWideScope, "namespace-wide"},
		{ClusterWideScope, "cluster-wide"},
	}

	for _, tc := range testCases {
		if got, want := tc.scope.String(), tc.name; got != want {
			t.Errorf("got: %q, want: %q", got, want)
		}

		var s SealingScope
		err := s.Set(tc.name)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := s, tc.scope; got != want {
			t.Errorf("got: %d, want: %d", got, want)
		}
	}

	var s SealingScope
	err := s.Set("")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := s, StrictScope; got != want {
		t.Errorf("got: %d, want: %d", got, want)
	}
}

func TestEncryptionLabel(t *testing.T) {
	const (
		ns   = "myns"
		name = "myname"
	)
	testCases := []struct {
		scope SealingScope
		label string
	}{
		{StrictScope, "myns/myname"},
		{NamespaceWideScope, "myns"},
		{ClusterWideScope, ""},
	}
	for _, tc := range testCases {
		if got, want := string(EncryptionLabel(ns, name, tc.scope)), tc.label; got != want {
			t.Errorf("got: %q, want: %q", got, want)
		}
	}
}

func TestLabel(t *testing.T) {
	s := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myname",
			Namespace: "myns",
		},
	}
	l := labelFor(&s)
	if string(l) != "myns/myname" {
		t.Errorf("Unexpected label: %#v", l)
	}
}

func TestClusterWide(t *testing.T) {
	s := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myname",
			Namespace: "myns",
			Annotations: map[string]string{
				SealedSecretClusterWideAnnotation: "true",
			},
		},
	}
	l := labelFor(&s)
	if string(l) != "" {
		t.Errorf("Unexpected label: %#v", l)
	}
}

func TestNamespaceWide(t *testing.T) {
	s := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myname",
			Namespace: "myns",
			Annotations: map[string]string{
				SealedSecretNamespaceWideAnnotation: "true",
			},
		},
	}
	l := labelFor(&s)
	if string(l) != "myns" {
		t.Errorf("Unexpected label: %#v", l)
	}
}

func TestClusterAndNamespaceWide(t *testing.T) {
	s := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myname",
			Namespace: "myns",
			Annotations: map[string]string{
				SealedSecretNamespaceWideAnnotation: "true",
				SealedSecretClusterWideAnnotation:   "true",
			},
		},
	}
	l := labelFor(&s)
	if string(l) != "" {
		t.Errorf("Unexpected label: %#v", l)
	}
}

func TestSerialize(t *testing.T) {
	s := SealedSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myname",
			Namespace: "myns",
		},
		Spec: SealedSecretSpec{
			EncryptedData: map[string]string{
				"foo": base64.StdEncoding.EncodeToString([]byte("secret1")),
				"bar": base64.StdEncoding.EncodeToString([]byte("secret2")),
			},
		},
	}

	info, ok := runtime.SerializerInfoForMediaType(scheme.Codecs.SupportedMediaTypes(), runtime.ContentTypeJSON)
	if !ok {
		t.Fatalf("binary can't serialize JSON")
	}

	enc := scheme.Codecs.EncoderForVersion(info.Serializer, SchemeGroupVersion)
	buf := bytes.Buffer{}
	if err := enc.Encode(&s, &buf); err != nil {
		t.Errorf("Error encoding: %v", err)
	}

	t.Logf("text is %s", buf.String())
}

// This is omg-not safe for real crypto use!
func testRand() io.Reader {
	return mathrand.New(mathrand.NewSource(42))
}

func generateTestKey(t *testing.T, rand io.Reader, bits int) (*rsa.PrivateKey, map[string]*rsa.PrivateKey) {
	key, err := rsa.GenerateKey(rand, 2048)
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}
	fingerprint, err := crypto.PublicKeyFingerprint(&key.PublicKey)
	if err != nil {
		t.Fatalf("Failed to generate fingerprint: %v", err)
	}
	keys := map[string]*rsa.PrivateKey{fingerprint: key}
	return key, keys
}

func TestSealRoundTrip(t *testing.T) {
	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myname",
			Namespace: "myns",
		},
		Data: map[string][]byte{
			"foo": []byte("bar"),
		},
	}

	ssecret, codecs, keys := sealSecret(t, &secret, NewSealedSecret)

	secret2, err := ssecret.Unseal(codecs, keys)
	if err != nil {
		t.Fatalf("Unseal returned error: %v", err)
	}

	if !reflect.DeepEqual(secret.Data, secret2.Data) {
		t.Errorf("Unsealed secret != original secret: %v != %v", secret, secret2)
	}
}

func TestSealRoundTripStringDataConversion(t *testing.T) {
	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myname",
			Namespace: "myns",
		},
		Data: map[string][]byte{
			"foo": []byte("bar"),
			"fss": []byte("brr"),
		},
		StringData: map[string]string{
			"fss": "baa",
		},
	}

	unsealed := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myname",
			Namespace: "myns",
		},
		Data: map[string][]byte{
			"foo": []byte("bar"),
			"fss": []byte("baa"),
		},
	}

	ssecret, codecs, keys := sealSecret(t, &secret, NewSealedSecret)

	secret2, err := ssecret.Unseal(codecs, keys)
	if err != nil {
		t.Fatalf("Unseal returned error: %v", err)
	}

	if !reflect.DeepEqual(unsealed.Data, secret2.Data) {
		t.Errorf("Unsealed secret != original secret: %v != %v", unsealed, secret2)
	}
}

func TestSealRoundTripWithClusterWide(t *testing.T) {
	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myname",
			Namespace: "myns",
			Annotations: map[string]string{
				SealedSecretClusterWideAnnotation: "true",
			},
		},
		Data: map[string][]byte{
			"foo": []byte("bar"),
		},
	}

	ssecret, codecs, keys := sealSecret(t, &secret, NewSealedSecret)

	secret2, err := ssecret.Unseal(codecs, keys)
	if err != nil {
		t.Fatalf("Unseal returned error: %v", err)
	}

	if !reflect.DeepEqual(secret.Data, secret2.Data) {
		t.Errorf("Unsealed secret != original secret: %v != %v", secret, secret2)
	}
}

func TestSealRoundTripWithMisMatchClusterWide(t *testing.T) {
	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myname",
			Namespace: "myns",
			Annotations: map[string]string{
				SealedSecretClusterWideAnnotation: "true",
			},
		},
		Data: map[string][]byte{
			"foo": []byte("bar"),
		},
	}

	ssecret, codecs, keys := sealSecret(t, &secret, NewSealedSecret)

	ssecret.ObjectMeta.Annotations[SealedSecretClusterWideAnnotation] = "false"

	_, err := ssecret.Unseal(codecs, keys)
	if err == nil {
		t.Fatal("Expecting error: got nil instead")
	}
}

func TestSealRoundTripWithNamespaceWide(t *testing.T) {
	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myname",
			Namespace: "myns",
			Annotations: map[string]string{
				SealedSecretNamespaceWideAnnotation: "true",
			},
		},
		Data: map[string][]byte{
			"foo": []byte("bar"),
		},
	}

	ssecret, codecs, keys := sealSecret(t, &secret, NewSealedSecret)

	secret2, err := ssecret.Unseal(codecs, keys)
	if err != nil {
		t.Fatalf("Unseal returned error: %v", err)
	}

	if !reflect.DeepEqual(secret.Data, secret2.Data) {
		t.Errorf("Unsealed secret != original secret: %v != %v", secret, secret2)
	}
}

func TestSealRoundTripWithMisMatchNamespaceWide(t *testing.T) {
	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myname",
			Namespace: "myns",
			Annotations: map[string]string{
				SealedSecretNamespaceWideAnnotation: "true",
			},
		},
		Data: map[string][]byte{
			"foo": []byte("bar"),
		},
	}

	ssecret, codecs, keys := sealSecret(t, &secret, NewSealedSecret)

	ssecret.ObjectMeta.Annotations[SealedSecretNamespaceWideAnnotation] = "false"

	_, err := ssecret.Unseal(codecs, keys)
	if err == nil {
		t.Fatalf("Unseal did not return expected error: %v", err)
	}
}

func TestSealRoundTripTemplateData(t *testing.T) {
	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myname",
			Namespace: "myns",
		},
		Data: map[string][]byte{
			"foo":      []byte("bar"),
			"password": []byte("hunter2'\"="),
		},
	}

	ssecret, codecs, keys := sealSecret(t, &secret, NewSealedSecret)

	ssecret.Spec.Template.Data = map[string]string{
		"bar":           `secret {{ index . "foo" }} !`,
		"password-json": `{{ toJson .password }}`,
	}

	secret2, err := ssecret.Unseal(codecs, keys)
	if err != nil {
		t.Fatalf("Unseal returned error: %v", err)
	}

	if got, want := string(secret2.Data["bar"]), "secret bar !"; got != want {
		t.Errorf("got: %q, want: %q", got, want)
	}

	want, err := json.Marshal(string(secret.Data["password"]))
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}

	if got := string(secret2.Data["password-json"]); got != string(want) {
		t.Errorf("got: %q, want: %q", got, want)
	}
}

func TestTemplateWithoutEncryptedData(t *testing.T) {
	sealed := SealedSecret{
		Spec: SealedSecretSpec{
			Template: SecretTemplateSpec{
				Data: map[string]string{"foo": "bar"},
			},
		},
	}

	unsealed, err := sealed.Unseal(serializer.CodecFactory{}, nil)
	if err != nil {
		t.Fatalf("Unseal returned error: %v", err)
	}

	if got, want := unsealed.Data, map[string][]byte{"foo": []byte("bar")}; !reflect.DeepEqual(got, want) {
		t.Errorf("got: %q, want: %q", got, want)
	}
}

func TestSkipSetOwnerReference(t *testing.T) {
	testCases := []struct {
		sealedSecret          SealedSecret
		skipSetOwnerReference bool
		secret                v1.Secret
	}{
		{
			sealedSecret: SealedSecret{
				Spec: SealedSecretSpec{
					Template: SecretTemplateSpec{
						Data: map[string]string{"foo": "bar"},
					},
				},
			},
			skipSetOwnerReference: true,
			secret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{},
			},
		},
		{
			sealedSecret: SealedSecret{
				Spec: SealedSecretSpec{
					Template: SecretTemplateSpec{
						Data: map[string]string{"foo": "bar"},
					},
				},
			},
			skipSetOwnerReference: false,
			secret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{},
				},
			},
		},
	}

	for _, tc := range testCases {
		if tc.skipSetOwnerReference {
			if tc.sealedSecret.Spec.Template.Annotations == nil {
				tc.sealedSecret.Spec.Template.Annotations = make(map[string]string)
			}
			tc.sealedSecret.Spec.Template.Annotations[SealedSecretSkipSetOwnerReferencesAnnotation] = "true"
		}
		unsealed, err := tc.sealedSecret.Unseal(serializer.CodecFactory{}, nil)
		if err != nil {
			t.Fatalf("Unseal returned error: %v", err)
		}
		if tc.sealedSecret.Spec.Template.Annotations[SealedSecretSkipSetOwnerReferencesAnnotation] == "true" &&
			len(unsealed.ObjectMeta.OwnerReferences) > 0 {
			t.Errorf("got: owner, want: no owner")
		} else if (tc.sealedSecret.Spec.Template.Annotations[SealedSecretSkipSetOwnerReferencesAnnotation] != "true") &&
			len(unsealed.ObjectMeta.OwnerReferences) == 0 {
			t.Errorf("got: no owner, want:  owner")
		}
	}
}

func TestSealMetadataPreservation(t *testing.T) {
	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)

	utilruntime.Must(SchemeBuilder.AddToScheme(scheme))
	utilruntime.Must(v1.SchemeBuilder.AddToScheme(scheme))

	key, _ := generateTestKey(t, testRand(), 2048)

	testCases := []struct {
		key       string
		preserved bool
	}{
		{"foo", true},
		{"foo.bar.io/foo-bar-baz", true},
		{"kubectl.kubernetes.io/last-applied-configuration", false},
		{"kubecfg.ksonnet.io/last-applied-configuration", false},
	}

	for _, tc := range testCases {
		secret := v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "myname",
				Namespace:   "myns",
				Annotations: map[string]string{tc.key: "test value"},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "foo/v1",
						Kind:       "Foo",
						Name:       "foo",
					},
				},
			},
			Data: map[string][]byte{
				"foo": []byte("bar"),
			},
		}

		ssecret, err := NewSealedSecret(codecs, &key.PublicKey, &secret)
		if err != nil {
			t.Fatalf("NewSealedSecret returned error: %v", err)
		}

		_, got := ssecret.Spec.Template.Annotations[tc.key]
		if want := tc.preserved; got != want {
			t.Errorf("key %q: exists: %v, expected to exist: %v", tc.key, got, want)
		}

		if got, want := len(ssecret.Spec.Template.OwnerReferences), 0; got != want {
			t.Errorf("got: %d, want: %d", got, want)
		}
	}
}

func TestUnsealingV1Format(t *testing.T) {
	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myname",
			Namespace: "myns",
			Annotations: map[string]string{
				SealedSecretClusterWideAnnotation:   "true",
				SealedSecretNamespaceWideAnnotation: "true",
			},
		},
		Data: map[string][]byte{
			"foo": []byte("bar"),
		},
	}

	ssecret, codecs, keys := sealSecret(t, &secret, NewSealedSecretV1)

	t.Run("AcceptDeprecatedV1Data", testWithAcceptDeprecatedV1Data(true, func(t *testing.T) {
		secret2, err := ssecret.Unseal(codecs, keys)
		if err != nil {
			t.Fatalf("Unseal returned error: %v", err)
		}

		if !reflect.DeepEqual(secret.Data, secret2.Data) {
			t.Errorf("Unsealed secret != original secret: %v != %v", secret, secret2)
		}
	}))

	t.Run("RejectDeprecatedV1Data", testWithAcceptDeprecatedV1Data(false, func(t *testing.T) {
		_, err := ssecret.Unseal(codecs, keys)
		if needle := "deprecated"; err == nil || !strings.Contains(err.Error(), needle) {
			t.Fatalf("Expecting error: %v to contain %q", err, needle)
		}
	}))
}

func TestRejectBothEncryptedDataAndDeprecatedV1Data(t *testing.T) {
	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myname",
			Namespace: "myns",
		},
		StringData: map[string]string{"foo": "bar"},
	}

	sealedSecret, codecs, keys := sealSecret(t, &secret, NewSealedSecret)
	sealedSecret.Spec.Data = []byte{}

	t.Run("AcceptDeprecatedV1Data", testWithAcceptDeprecatedV1Data(true, func(t *testing.T) {
		_, err := sealedSecret.Unseal(codecs, keys)
		if needle := "at the same time"; err == nil || !strings.Contains(err.Error(), needle) {
			t.Fatalf("Expecting error: %v to contain %q", err, needle)
		}
	}))

	t.Run("RejectDeprecatedV1Data", testWithAcceptDeprecatedV1Data(false, func(t *testing.T) {
		_, err := sealedSecret.Unseal(codecs, keys)
		if needle := "deprecated"; err == nil || !strings.Contains(err.Error(), needle) {
			t.Fatalf("Expecting error: %v to contain %q", err, needle)
		}
	}))
}

func TestInvalidBase64(t *testing.T) {
	sealedSecret := &SealedSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myname",
			Namespace: "myns",
		},
		Spec: SealedSecretSpec{
			EncryptedData: map[string]string{
				"foo": "NOTVALIDBASE64",
			},
		},
	}

	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)
	_, keys := generateTestKey(t, testRand(), 2048)

	_, err := sealedSecret.Unseal(codecs, keys)
	if err == nil {
		t.Fatal("Expecting error: got nil instead")
	}

	if !strings.Contains(err.Error(), "foo") {
		t.Errorf("Expecting error: %q to contain field %q", err, "foo")
	}

	if strings.Contains(err.Error(), "decrypt") {
		t.Errorf("Expecting error: %q to not contain %q (invalid base64 should skip decryption)", err, "decrypt")
	}
}

func sealSecret(t *testing.T, secret *v1.Secret, newSealedSecret func(serializer.CodecFactory, *rsa.PublicKey, *v1.Secret) (*SealedSecret, error)) (*SealedSecret, serializer.CodecFactory, map[string]*rsa.PrivateKey) {
	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)

	utilruntime.Must(SchemeBuilder.AddToScheme(scheme))
	utilruntime.Must(v1.SchemeBuilder.AddToScheme(scheme))

	key, keys := generateTestKey(t, testRand(), 2048)

	sealedSecret, err := newSealedSecret(codecs, &key.PublicKey, secret)
	if err != nil {
		t.Fatalf("NewSealedSecret returned error: %v", err)
	}

	return sealedSecret, codecs, keys
}

func testWithAcceptDeprecatedV1Data(acceptDeprecated bool, inner func(t *testing.T)) func(*testing.T) {
	return func(t *testing.T) {
		defer func(saved bool) {
			AcceptDeprecatedV1Data = saved
		}(AcceptDeprecatedV1Data)
		AcceptDeprecatedV1Data = acceptDeprecated

		inner(t)
	}
}
