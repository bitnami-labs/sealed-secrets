package v1alpha1

import (
	"bytes"
	"crypto/rsa"
	"io"
	mathrand "math/rand"
	"reflect"
	"testing"

	fuzz "github.com/google/gofuzz"

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
			EncryptedData: map[string][]byte{
				"foo": []byte("secret1"),
				"bar": []byte("secret2"),
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

func TestSealRoundTrip(t *testing.T) {
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
		},
		Data: map[string][]byte{
			"foo": []byte("bar"),
		},
	}

	ssecret, err := NewSealedSecret(codecs, &key.PublicKey, &secret)
	if err != nil {
		t.Fatalf("NewSealedSecret returned error: %v", err)
	}

	secret2, err := ssecret.Unseal(codecs, key)
	if err != nil {
		t.Fatalf("Unseal returned error: %v", err)
	}

	if !reflect.DeepEqual(secret.Data, secret2.Data) {
		t.Errorf("Unsealed secret != original secret: %v != %v", secret, secret2)
	}
}

func TestSealRoundTripWithClusterWide(t *testing.T) {
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
				SealedSecretClusterWideAnnotation: "true",
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

	secret2, err := ssecret.Unseal(codecs, key)
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
				SealedSecretClusterWideAnnotation: "true",
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

	ssecret.ObjectMeta.Annotations[SealedSecretClusterWideAnnotation] = "false"

	_, err = ssecret.Unseal(codecs, key)
	if err == nil {
		t.Fatalf("Unseal did not return expected error: %v", err)
	}
}

func TestSealRoundTripWithNamespaceWide(t *testing.T) {
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
				SealedSecretNamespaceWideAnnotation: "true",
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

	secret2, err := ssecret.Unseal(codecs, key)
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
				SealedSecretNamespaceWideAnnotation: "true",
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

	ssecret.ObjectMeta.Annotations[SealedSecretNamespaceWideAnnotation] = "false"

	_, err = ssecret.Unseal(codecs, key)
	if err == nil {
		t.Fatalf("Unseal did not return expected error: %v", err)
	}
}

func TestUnsealingV1Format(t *testing.T) {
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

	secret2, err := ssecret.Unseal(codecs, key)
	if err != nil {
		t.Fatalf("Unseal returned error: %v", err)
	}

	if !reflect.DeepEqual(secret.Data, secret2.Data) {
		t.Errorf("Unsealed secret != original secret: %v != %v", secret, secret2)
	}
}
