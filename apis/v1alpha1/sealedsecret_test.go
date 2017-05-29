package v1alpha1

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/google/gofuzz"

	apitesting "k8s.io/apimachinery/pkg/api/testing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
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
	if l := labelFor(&s); string(l) != "myns/myname" {
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

	seed := rand.Int63()
	fuzzerFuncs := apitesting.MergeFuzzerFuncs(t, apitesting.GenericFuzzerFuncs(t, codecs), ssecretFuzzerFuncs(t))
	fuzzer := apitesting.FuzzerFor(fuzzerFuncs, rand.NewSource(seed))

	apitesting.RoundTripSpecificKindWithoutProtobuf(t, SchemeGroupVersion.WithKind("SealedSecret"), scheme, codecs, fuzzer, nil)
	apitesting.RoundTripSpecificKindWithoutProtobuf(t, SchemeGroupVersion.WithKind("SealedSecretList"), scheme, codecs, fuzzer, nil)
}
