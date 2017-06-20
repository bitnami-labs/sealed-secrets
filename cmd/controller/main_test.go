package main

import (
	"crypto/rsa"
	"crypto/x509"
	"io"
	mathrand "math/rand"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/pkg/api/v1"
	ktesting "k8s.io/client-go/testing"
	certUtil "k8s.io/client-go/util/cert"
)

func findAction(fake *fake.Clientset, verb, resource string) ktesting.Action {
	for _, a := range fake.Actions() {
		if a.Matches(verb, resource) {
			return a
		}
	}
	return nil
}

func hasAction(fake *fake.Clientset, verb, resource string) bool {
	return findAction(fake, verb, resource) != nil
}

// This is omg-not safe for real crypto use!
func testRand() io.Reader {
	return mathrand.New(mathrand.NewSource(42))
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
			v1.TLSPrivateKeyKey: certUtil.EncodePrivateKeyPEM(key),
			v1.TLSCertKey:       certUtil.EncodeCertPEM(cert),
		},
		Type: v1.SecretTypeTLS,
	}

	client := fake.NewSimpleClientset(&secret)

	key2, _, err := readKey(client, "myns", "mykey")
	if err != nil {
		t.Errorf("readKey() failed with: %v", err)
	}

	t.Logf("actions: %v", client.Actions())

	if !reflect.DeepEqual(key, key2) {
		t.Errorf("Fetched key != original key: %v != %v", key, key2)
	}
}

func TestWriteKey(t *testing.T) {
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

	if err := writeKey(client, key, []*x509.Certificate{cert}, "myns", "mykey"); err != nil {
		t.Errorf("writeKey() failed with: %v", err)
	}

	t.Logf("actions: %v", client.Actions())

	if a := findAction(client, "create", "secrets"); a == nil {
		t.Errorf("writeKey didn't create a secret")
	} else if a.GetNamespace() != "myns" {
		t.Errorf("writeKey() created key in wrong namespace!")
	}
}

func TestSignKey(t *testing.T) {
	rand := testRand()

	key, err := rsa.GenerateKey(rand, 512)
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}

	cert, err := signKey(rand, key)
	if err != nil {
		t.Errorf("signKey() returned error: %v", err)
	}

	if !reflect.DeepEqual(cert.PublicKey, &key.PublicKey) {
		t.Errorf("cert pubkey != original pubkey")
	}
}

func TestInitKey(t *testing.T) {
	rand := testRand()
	client := fake.NewSimpleClientset()

	key, certs, err := initKey(client, rand, 1024, "testns", "testkey")
	if err != nil {
		t.Fatalf("initKey returned err: %v", err)
	}

	if !hasAction(client, "create", "secrets") {
		t.Errorf("initKey() failed to create secret")
	}

	client.ClearActions()

	key2, certs2, err := initKey(client, rand, 1024, "testns", "testkey")
	if err != nil {
		t.Fatalf("initKey returned err: %v", err)
	}

	if !reflect.DeepEqual(key, key2) {
		t.Errorf("Failed to find same key")
	}

	if !reflect.DeepEqual(certs, certs2) {
		t.Errorf("Failed to find same certs")
	}
}
