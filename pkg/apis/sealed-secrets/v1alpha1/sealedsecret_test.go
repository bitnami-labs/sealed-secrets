package v1alpha1

import (
	"bytes"
	"crypto/rsa"
	"encoding/base64"
	"io"
	mathrand "math/rand"
	"reflect"
	"strings"
	"testing"

	fuzz "github.com/google/gofuzz"

	"github.com/bitnami-labs/sealed-secrets/pkg/crypto"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	rttesting "k8s.io/apimachinery/pkg/api/apitesting/roundtrip"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"

	// Install standard API types
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
		s.Set(tc.name)
		if got, want := s, tc.scope; got != want {
			t.Errorf("got: %d, want: %d", got, want)
		}
	}

	var s SealingScope
	s.Set("")
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
	l, c, _ := labelFor(&s)
	if c {
		t.Errorf("Unexpected value for cluster wide annotation: %#v", c)
	}
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
	l, c, _ := labelFor(&s)
	if !c {
		t.Errorf("Unexpected value for cluster wide annotation: %#v", c)
	}
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
	l, _, n := labelFor(&s)
	if !n {
		t.Errorf("Unexpected value for namespace wide annotation: %#v", n)
	}
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
	l, c, n := labelFor(&s)
	if !c {
		t.Errorf("Unexpected value for cluster wide annotation: %#v", c)
	}
	if n {
		t.Errorf("Unexpected value for namespace wide annotation: %#v", n)
	}
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

	t.Logf("text is %s", string(buf.Bytes()))
}

func ssecretFuzzerFuncs(codecs serializer.CodecFactory) []interface{} {
	return []interface{}{
		func(obj *SealedSecretList, c fuzz.Continue) {
			c.FuzzNoCustom(obj)
			obj.Items = make([]SealedSecret, c.Intn(10))
			for i := range obj.Items {
				c.Fuzz(&obj.Items[i])
			}
		},
	}
}

// TestRoundTrip tests that the third-party kinds can be marshaled and
// unmarshaled correctly to/from JSON without the loss of
// information. Moreover, deep copy is tested.
//
// Disabled because of spurious diffs caused by nil != []foo{}, e.g. in annotations
// labels, or other slices.
// TODO(mkm): fix
func disabledTestRoundTrip(t *testing.T) {
	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)

	SchemeBuilder.AddToScheme(scheme)

	seed := mathrand.Int63()
	fuzzer := fuzzer.FuzzerFor(ssecretFuzzerFuncs, mathrand.NewSource(seed), codecs)

	rttesting.RoundTripSpecificKindWithoutProtobuf(t, SchemeGroupVersion.WithKind("SealedSecret"), scheme, codecs, fuzzer, nil)
	rttesting.RoundTripSpecificKindWithoutProtobuf(t, SchemeGroupVersion.WithKind("SealedSecretList"), scheme, codecs, fuzzer, nil)
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
	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)

	SchemeBuilder.AddToScheme(scheme)
	v1.SchemeBuilder.AddToScheme(scheme)

	key, keys := generateTestKey(t, testRand(), 2048)

	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myname",
			Namespace: "myns",
		},
		Data: map[string][]byte{
			"foo": []byte("bar"),
		},
	}

	ssecret, err := NewSealedSecret(codecs, "cert", &key.PublicKey, &secret)
	if err != nil {
		t.Fatalf("NewSealedSecret returned error: %v", err)
	}

	secret2, err := ssecret.Unseal(codecs, "cert", keys)
	if err != nil {
		t.Fatalf("Unseal returned error: %v", err)
	}

	if !reflect.DeepEqual(secret.Data, secret2.Data) {
		t.Errorf("Unsealed secret != original secret: %v != %v", secret, secret2)
	}
}

func TestSealRoundTripVault(t *testing.T) {
	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)

	SchemeBuilder.AddToScheme(scheme)
	v1.SchemeBuilder.AddToScheme(scheme)

	key, keys := generateTestKey(t, testRand(), 2048)

	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myname",
			Namespace: "myns",
			Annotations: map[string]string{
				"encryption-type": "vault",
			},
		},
		Data: map[string][]byte{
			"foo": []byte("bar"),
		},
	}

	ssecret, err := NewSealedSecret(codecs, "vault", &key.PublicKey, &secret)
	if err != nil {
		t.Fatalf("NewSealedSecret returned error: %v", err)
	}

	secret2, err := ssecret.Unseal(codecs, "vault", keys)
	if err != nil {
		t.Fatalf("Unseal returned error: %v", err)
	}

	if !reflect.DeepEqual(secret.Data, secret2.Data) {
		t.Errorf("Unsealed secret != original secret: %v != %v", secret, secret2)
	}
}

func TestSealRoundTripStringDataConversion(t *testing.T) {
	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)

	SchemeBuilder.AddToScheme(scheme)
	v1.SchemeBuilder.AddToScheme(scheme)

	key, keys := generateTestKey(t, testRand(), 2048)

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

	ssecret, err := NewSealedSecret(codecs, "cert", &key.PublicKey, &secret)
	if err != nil {
		t.Fatalf("NewSealedSecret returned error: %v", err)
	}

	secret2, err := ssecret.Unseal(codecs, "cert", keys)
	if err != nil {
		t.Fatalf("Unseal returned error: %v", err)
	}

	if !reflect.DeepEqual(unsealed.Data, secret2.Data) {
		t.Errorf("Unsealed secret != original secret: %v != %v", unsealed, secret2)
	}
}

func TestSealRoundTripWithClusterWide(t *testing.T) {
	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)

	SchemeBuilder.AddToScheme(scheme)
	v1.SchemeBuilder.AddToScheme(scheme)

	key, keys := generateTestKey(t, testRand(), 2048)

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

	ssecret, err := NewSealedSecret(codecs, "cert", &key.PublicKey, &secret)
	if err != nil {
		t.Fatalf("NewSealedSecret returned error: %v", err)
	}

	secret2, err := ssecret.Unseal(codecs, "cert", keys)
	if err != nil {
		t.Fatalf("Unseal returned error: %v", err)
	}

	if !reflect.DeepEqual(secret.Data, secret2.Data) {
		t.Errorf("Unsealed secret != original secret: %v != %v", secret, secret2)
	}
}

func TestSealRoundTripWithMisMatchClusterWide(t *testing.T) {
	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)

	SchemeBuilder.AddToScheme(scheme)
	v1.SchemeBuilder.AddToScheme(scheme)

	key, keys := generateTestKey(t, testRand(), 2048)

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

	ssecret, err := NewSealedSecret(codecs, "cert", &key.PublicKey, &secret)
	if err != nil {
		t.Fatalf("NewSealedSecret returned error: %v", err)
	}

	ssecret.ObjectMeta.Annotations[SealedSecretClusterWideAnnotation] = "false"

	_, err = ssecret.Unseal(codecs, "cert", keys)
	if err == nil {
		t.Fatalf("Unseal did not return expected error: %v", err)
	}
}

func TestSealRoundTripWithNamespaceWide(t *testing.T) {
	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)

	SchemeBuilder.AddToScheme(scheme)
	v1.SchemeBuilder.AddToScheme(scheme)

	key, keys := generateTestKey(t, testRand(), 2048)

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

	ssecret, err := NewSealedSecret(codecs, "cert", &key.PublicKey, &secret)
	if err != nil {
		t.Fatalf("NewSealedSecret returned error: %v", err)
	}

	secret2, err := ssecret.Unseal(codecs, "cert", keys)
	if err != nil {
		t.Fatalf("Unseal returned error: %v", err)
	}

	if !reflect.DeepEqual(secret.Data, secret2.Data) {
		t.Errorf("Unsealed secret != original secret: %v != %v", secret, secret2)
	}
}

func TestSealRoundTripWithMisMatchNamespaceWide(t *testing.T) {
	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)

	SchemeBuilder.AddToScheme(scheme)
	v1.SchemeBuilder.AddToScheme(scheme)

	key, keys := generateTestKey(t, testRand(), 2048)

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

	ssecret, err := NewSealedSecret(codecs, "cert", &key.PublicKey, &secret)
	if err != nil {
		t.Fatalf("NewSealedSecret returned error: %v", err)
	}

	ssecret.ObjectMeta.Annotations[SealedSecretNamespaceWideAnnotation] = "false"

	_, err = ssecret.Unseal(codecs, "cert", keys)
	if err == nil {
		t.Fatalf("Unseal did not return expected error: %v", err)
	}
}

func TestSealMetadataPreservation(t *testing.T) {
	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)

	SchemeBuilder.AddToScheme(scheme)
	v1.SchemeBuilder.AddToScheme(scheme)

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

		ssecret, err := NewSealedSecret(codecs, "cert", &key.PublicKey, &secret)
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
	testUnsealingV1Format(t, true)
	testUnsealingV1Format(t, false)
}

func testUnsealingV1Format(t *testing.T, acceptDeprecated bool) {
	defer func(saved bool) {
		AcceptDeprecatedV1Data = saved
	}(AcceptDeprecatedV1Data)
	AcceptDeprecatedV1Data = acceptDeprecated

	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)

	SchemeBuilder.AddToScheme(scheme)
	v1.SchemeBuilder.AddToScheme(scheme)

	rand := testRand()
	key, err := rsa.GenerateKey(rand, 2048)
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}

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

	ssecret, err := NewSealedSecretV1(codecs, &key.PublicKey, &secret)
	if err != nil {
		t.Fatalf("NewSealedSecret returned error: %v", err)
	}

	fp, err := crypto.PublicKeyFingerprint(&key.PublicKey)
	if err != nil {
		t.Fatalf("cannot compute fingerprint: %v", err)
	}
	secret2, err := ssecret.Unseal(codecs, "cert", map[string]*rsa.PrivateKey{fp: key})
	if acceptDeprecated {
		if err != nil {
			t.Fatalf("Unseal returned error: %v", err)
		}

		if !reflect.DeepEqual(secret.Data, secret2.Data) {
			t.Errorf("Unsealed secret != original secret: %v != %v", secret, secret2)
		}
	} else {
		if needle := "deprecated"; err == nil || !strings.Contains(err.Error(), needle) {
			t.Fatalf("Expecting error: %v to contain %q", err, needle)
		}
	}
}
