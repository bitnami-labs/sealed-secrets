package controller

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io"
	mathrand "math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/bitnami-labs/sealed-secrets/pkg/crypto"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	certUtil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
)

// This is omg-not safe for real crypto use!
func testRand() io.Reader {
	return mathrand.New(mathrand.NewSource(42))
}

func signKey(r io.Reader, key *rsa.PrivateKey) (*x509.Certificate, error) {
	return crypto.SignKey(r, key, time.Hour, "testcn")
}

func signKeyWithNotBefore(r io.Reader, key *rsa.PrivateKey, notBefore time.Time) (*x509.Certificate, error) {
	return crypto.SignKeyWithNotBefore(r, key, notBefore, time.Hour, "testcn")
}

func TestReadKey(t *testing.T) {
	rand := testRand()

	key, err := rsa.GenerateKey(rand, 512)
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}

	cert, err := signKey(rand, key)
	if err != nil {
		t.Fatalf("Failed to self-sign key: %v", err)
	}

	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mykey",
			Namespace: "myns",
		},
		Data: map[string][]byte{
			v1.TLSPrivateKeyKey: pem.EncodeToMemory(&pem.Block{Type: keyutil.RSAPrivateKeyBlockType, Bytes: x509.MarshalPKCS1PrivateKey(key)}),
			v1.TLSCertKey:       pem.EncodeToMemory(&pem.Block{Type: certUtil.CertificateBlockType, Bytes: cert.Raw}),
		},
		Type: v1.SecretTypeTLS,
	}

	key2, cert2, err := readKey(secret)
	if err != nil {
		t.Errorf("readKey() failed with: %v", err)
	}

	if !reflect.DeepEqual(key, key2) {
		t.Errorf("Extracted key != original key")
	}

	if !reflect.DeepEqual(cert, cert2[0]) {
		t.Errorf("Extracted cert != original cert")
	}
}

func TestWriteKey(t *testing.T) {
	ctx := context.Background()
	rand := testRand()
	key, err := rsa.GenerateKey(rand, 512)
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}

	cert, err := signKey(rand, key)
	if err != nil {
		t.Fatalf("signKey failed: %v", err)
	}

	client := fake.NewSimpleClientset()

	namespace := "myns"
	defaultLabel := "default-label"
	myKey := "mykey"
	additionalAnnotations := "testAnnotation1=additional.annotation,test.annotation.2=test/2"
	additionalLabels := "testLabel1=additional.label,test.label.2=test/2"
	_, err = writeKey(ctx, client, key, []*x509.Certificate{cert}, namespace, defaultLabel, myKey, additionalAnnotations, additionalLabels)
	if err != nil {
		t.Errorf("writeKey() failed with: %v", err)
	}

	t.Logf("actions: %v", client.Actions())

	if a := findAction(client, "create", "secrets"); a == nil {
		t.Errorf("writeKey didn't create a secret")
	} else if a.GetNamespace() != namespace {
		t.Errorf("writeKey() created key in wrong namespace!")
	}
	a := findAction(client, "create", "secrets").(ktesting.CreateActionImpl)
	secret, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(a.Object)
	generateName := secret["metadata"].(map[string]interface{})["generateName"].(string)

	if generateName != myKey {
		t.Errorf("writeKey didn't set the correct name")
	}

	labels := secret["metadata"].(map[string]interface{})["labels"]
	annotations := secret["metadata"].(map[string]interface{})["annotations"]

	if labels.(map[string]interface{})[defaultLabel] != "active" {
		t.Errorf("writeKey didn't set default label")
	}

	for _, label := range strings.Split(additionalLabels, ",") {
		labelKey := strings.Split(label, "=")[0]
		labelValue := strings.Split(label, "=")[1]
		if labels.(map[string]interface{})[labelKey] != labelValue {
			t.Errorf("writeKey didn't set label " + labelKey + " to value '" + labelValue + "'")
		}
	}

	for _, annotation := range strings.Split(additionalAnnotations, ",") {
		annotationKey := strings.Split(annotation, "=")[0]
		annotationValue := strings.Split(annotation, "=")[1]
		if annotations.(map[string]interface{})[annotationKey] != annotationValue {
			t.Errorf("writeKey didn't set annotation '" + annotationKey + "' to value '" + annotationValue + "'")
		}
	}
}
