package kubeseal

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"os"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/keyutil"
)

func TestUnseal(t *testing.T) {
	pubKey, privKeys := newTestKeyPair(t)
	pkFile, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(pkFile.Name())

	if len(privKeys) != 1 {
		t.Fatal("assuming only one test key-pair")
	}
	for _, key := range privKeys {
		err := pem.Encode(pkFile, &pem.Block{Type: keyutil.RSAPrivateKeyBlockType, Bytes: x509.MarshalPKCS1PrivateKey(key)})
		if err != nil {
			t.Fatal(err)
		}
	}
	pkFile.Close()

	const (
		secretItemKey   = "foo"
		secretItemValue = "secret1"
	)
	ss := mkTestSealedSecret(t, pubKey, secretItemKey, secretItemValue)

	var buf bytes.Buffer
	i := UnsealSealedSecretInstruction{
		OutputFormat:     "json",
		In:               bytes.NewBuffer(ss),
		Out:              &buf,
		Codecs:           scheme.Codecs,
		PrivKeyFilenames: []string{pkFile.Name()},
	}
	if err := Unseal(i); err != nil {
		t.Fatal(err)
	}

	secret, err := ReadSecret(scheme.Codecs.UniversalDecoder(), &buf)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := string(secret.Data[secretItemKey]), secretItemValue; got != want {
		t.Fatalf("got: %q, want: %q", got, want)
	}
}

func TestUnsealList(t *testing.T) {
	pubKey, privKeys := newTestKeyPair(t)
	pkFile, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(pkFile.Name())

	// encode a v1.List containing all the privKeys into one file.
	prettyEnc, err := PrettyEncoder(scheme.Codecs, runtime.ContentTypeJSON, v1.SchemeGroupVersion)
	if err != nil {
		t.Fatal(err)
	}

	var secrets [][]byte
	for _, key := range privKeys {
		b := pem.EncodeToMemory(&pem.Block{Type: keyutil.RSAPrivateKeyBlockType, Bytes: x509.MarshalPKCS1PrivateKey(key)})
		buf, err := runtime.Encode(prettyEnc, &v1.Secret{Data: map[string][]byte{"tls.key": b}})
		if err != nil {
			t.Fatal(err)
		}
		secrets = append(secrets, buf)
	}
	lst := &v1.List{}
	for _, s := range secrets {
		lst.Items = append(lst.Items, runtime.RawExtension{Raw: s})
	}
	blst, err := runtime.Encode(prettyEnc, lst)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pkFile.Write(blst); err != nil {
		t.Fatal(err)
	}
	pkFile.Close()

	const (
		secretItemKey   = "foo"
		secretItemValue = "secret1"
	)
	ss := mkTestSealedSecret(t, pubKey, secretItemKey, secretItemValue)

	var buf bytes.Buffer
	i := UnsealSealedSecretInstruction{
		OutputFormat:     "json",
		In:               bytes.NewBuffer(ss),
		Out:              &buf,
		Codecs:           scheme.Codecs,
		PrivKeyFilenames: []string{pkFile.Name()},
	}
	if err := Unseal(i); err != nil {
		t.Fatal(err)
	}

	secret, err := ReadSecret(scheme.Codecs.UniversalDecoder(), &buf)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := string(secret.Data[secretItemKey]), secretItemValue; got != want {
		t.Fatalf("got: %q, want: %q", got, want)
	}
}

func TestReadPrivKeyPEM(t *testing.T) {
	_, pkw := newTestKeyPairSingle(t)

	b, err := keyutil.MarshalPrivateKeyToPEM(pkw)
	if err != nil {
		t.Fatal(err)
	}
	tmp, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp.Name())

	if _, err := tmp.Write(b); err != nil {
		t.Fatal(err)
	}
	tmp.Close()

	pkr, err := ReadPrivKeysFromFile(tmp.Name())
	if err != nil {
		t.Fatal(err)
	}

	if got, want := pkr[0].D.String(), pkw.D.String(); got != want {
		t.Errorf("got: %q, want: %q", got, want)
	}
}

// func TestReadPrivKeySecret(t *testing.T) {
// 	_, pkw := newTestKeyPairSingle(t)

// 	b, err := keyutil.MarshalPrivateKeyToPEM(pkw)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	sec := &v1.Secret{
// 		Data: map[string][]byte{
// 			"tls.key": b,
// 		},
// 	}

// 	tmp, err := ioutil.TempFile("", "")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer os.RemoveAll(tmp.Name())

// 	if err := ResourceOutput(tmp, scheme.Codecs, v1.SchemeGroupVersion, sec, "json"); err != nil {
// 		t.Fatal(err)
// 	}
// 	tmp.Close()

// 	pkr, err := ReadPrivKeysFromFile(tmp.Name())
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	if got, want := pkr[0].D.String(), pkw.D.String(); got != want {
// 		t.Errorf("got: %q, want: %q", got, want)
// 	}
// }
