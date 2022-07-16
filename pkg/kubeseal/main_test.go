package kubeseal

import (
	"bytes"
	"crypto/rsa"
	"testing"
	"time"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealed-secrets/v1alpha1"

	"github.com/bitnami-labs/sealed-secrets/pkg/crypto"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

type mkTestSecretOpt func(*mkTestSecretOpts)
type mkTestSecretOpts struct {
	secretName      string
	secretNamespace string
	asYAML          bool
}

func withSecretName(n string) mkTestSecretOpt {
	return func(o *mkTestSecretOpts) {
		o.secretName = n
	}
}

func withSecretNamespace(n string) mkTestSecretOpt {
	return func(o *mkTestSecretOpts) {
		o.secretNamespace = n
	}
}

func asYAML(y bool) mkTestSecretOpt {
	return func(o *mkTestSecretOpts) {
		o.asYAML = y
	}
}

func mkTestSecret(t *testing.T, key, value string, opts ...mkTestSecretOpt) []byte {
	o := mkTestSecretOpts{
		secretName:      "testname",
		secretNamespace: "testns",
	}
	for _, opt := range opts {
		opt(&o)
	}

	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.secretName,
			Namespace: o.secretNamespace,
			Annotations: map[string]string{
				key: value, // putting secret here just to have a simple way to test annotation merges
			},
			Labels: map[string]string{
				key: value,
			},
		},
		Data: map[string][]byte{
			key: []byte(value),
		},
	}

	contentType := runtime.ContentTypeJSON
	if o.asYAML {
		contentType = runtime.ContentTypeYAML
	}

	info, ok := runtime.SerializerInfoForMediaType(scheme.Codecs.SupportedMediaTypes(), contentType)
	if !ok {
		t.Fatalf("binary can't serialize JSON")
	}
	enc := scheme.Codecs.EncoderForVersion(info.Serializer, v1.SchemeGroupVersion)
	var inbuf bytes.Buffer
	if err := enc.Encode(&secret, &inbuf); err != nil {
		t.Fatalf("Error encoding: %v", err)
	}
	return inbuf.Bytes()
}

func mkTestSealedSecret(t *testing.T, pubKey *rsa.PublicKey, key, value string, opts ...mkTestSecretOpt) []byte {
	inbuf := bytes.NewBuffer(mkTestSecret(t, key, value, opts...))
	var outbuf bytes.Buffer
	i := SealInstruction{
		In:                inbuf,
		Out:               &outbuf,
		Codecs:            scheme.Codecs,
		PubKey:            pubKey,
		Scope:             ssv1alpha1.DefaultScope,
		AllowEmptyData:    false,
		OverrideName:      "",
		OverrideNamespace: "",
	}
	if err := Seal(i); err != nil {
		t.Fatalf("seal() returned error: %v", err)
	}

	return outbuf.Bytes()
}

func newTestKeyPairSingle(t *testing.T) (*rsa.PublicKey, *rsa.PrivateKey) {
	privKey, _, err := crypto.GeneratePrivateKeyAndCert(2048, time.Hour, "testcn")
	if err != nil {
		t.Fatal(err)
	}
	return &privKey.PublicKey, privKey
}

// TODO(mkm): rename newTestKeyPair to newTestKeyPairs
func newTestKeyPair(t *testing.T) (*rsa.PublicKey, map[string]*rsa.PrivateKey) {
	privKey, _, err := crypto.GeneratePrivateKeyAndCert(2048, time.Hour, "testcn")
	if err != nil {
		t.Fatal(err)
	}
	pubKey := &privKey.PublicKey

	fp, err := crypto.PublicKeyFingerprint(pubKey)
	if err != nil {
		t.Fatal(err)
	}
	privKeys := map[string]*rsa.PrivateKey{fp: privKey}

	return pubKey, privKeys
}
