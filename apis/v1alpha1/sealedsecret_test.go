package v1alpha1

import (
	"bytes"
	"crypto/rsa"
	"io"
	mathrand "math/rand"
	"reflect"
	"testing"

	"github.com/google/gofuzz"

	apitesting "k8s.io/apimachinery/pkg/api/testing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"

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
	l, c := labelFor(&s)
	if c {
		t.Errorf("Unexpected value for custom: %#v", c)
	}
	if string(l) != "myns/myname" {
		t.Errorf("Unexpected label: %#v", l)
	}
}

func TestCustomLabel(t *testing.T) {
	s := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myname",
			Namespace: "myns",
			Annotations: map[string]string{
				SealedSecretLabelAnnotation: "my-label",
			},
		},
	}
	l, c := labelFor(&s)
	if !c {
		t.Errorf("Unexpected value for custom: %#v", c)
	}
	if string(l) != "my-label" {
		t.Errorf("Unexpected label: %#v", l)
	}
}

func TestSerialize(t *testing.T) {
	s := SealedSecret{
		Metadata: metav1.ObjectMeta{
			Name:      "myname",
			Namespace: "myns",
		},
		Spec: SealedSecretSpec{
			Data: []byte("xxx"),
		},
	}

	info, ok := runtime.SerializerInfoForMediaType(api.Codecs.SupportedMediaTypes(), runtime.ContentTypeJSON)
	if !ok {
		t.Fatalf("binary can't serialize JSON")
	}

	enc := api.Codecs.EncoderForVersion(info.Serializer, SchemeGroupVersion)
	buf := bytes.Buffer{}
	if err := enc.Encode(&s, &buf); err != nil {
		t.Errorf("Error encoding: %v", err)
	}

	t.Logf("text is %s", string(buf.Bytes()))
}

func ssecretFuzzerFuncs(t apitesting.TestingCommon) []interface{} {
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
func TestRoundTrip(t *testing.T) {
	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)

	SchemeBuilder.AddToScheme(scheme)

	seed := mathrand.Int63()
	fuzzerFuncs := apitesting.MergeFuzzerFuncs(t, apitesting.GenericFuzzerFuncs(t, codecs), ssecretFuzzerFuncs(t))
	fuzzer := apitesting.FuzzerFor(fuzzerFuncs, mathrand.NewSource(seed))

	apitesting.RoundTripSpecificKindWithoutProtobuf(t, SchemeGroupVersion.WithKind("SealedSecret"), scheme, codecs, fuzzer, nil)
	apitesting.RoundTripSpecificKindWithoutProtobuf(t, SchemeGroupVersion.WithKind("SealedSecretList"), scheme, codecs, fuzzer, nil)
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

func TestSealRoundTripWithCustomLabel(t *testing.T) {
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
				SealedSecretLabelAnnotation: "my-custom-label",
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

func TestSealRoundTripWithMisMatchCustomLabel(t *testing.T) {
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
				SealedSecretLabelAnnotation: "my-custom-label",
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

	ssecret.Metadata.Annotations[SealedSecretLabelAnnotation] = "mismatch"

	_, err = ssecret.Unseal(codecs, key)
	if err == nil {
		t.Fatalf("Unseal did not return expected error: %v", err)
	}
}
