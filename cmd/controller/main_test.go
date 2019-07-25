package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	certUtil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
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

func TestInitKeyRegistry(t *testing.T) {
	rand := testRand()
	client := fake.NewSimpleClientset()

	registry, err := initKeyRegistry(client, rand, "namespace", "prefix", "label", 1024)
	if err != nil {
		t.Fatalf("initKeyRegistry() returned err: %v", err)
	}

	// Add a key to the controller for second test
	registry.generateKey()
	if !hasAction(client, "create", "secrets") {
		t.Fatalf("Error adding initial key to registry")
	}
	client.ClearActions()

	// Due to limitations of the fake client, we cannot test whether initKeyRegistry is able
	// to pick up existing keys
	_, err = initKeyRegistry(client, rand, "namespace", "prefix", "label", 1024)
	if err != nil {
		t.Fatalf("initKeyRegistry() returned err: %v", err)
	}
	if !hasAction(client, "list", "secrets") {
		t.Errorf("initKeyRegistry() failed to read existing keys")
	}
}

func TestInitKeyRotation(t *testing.T) {
	rand := testRand()
	client := fake.NewSimpleClientset()
	registry, err := initKeyRegistry(client, rand, "namespace", "prefix", "label", 1024)
	if err != nil {
		t.Fatalf("initKeyRegistry() returned err: %v", err)
	}

	keyGenTrigger, err := initKeyRotation(registry, 0)
	if err != nil {
		t.Fatalf("initKeyRotation() returned err: %v", err)
	}
	if !hasAction(client, "create", "secrets") {
		t.Errorf("initKeyRotation() failed to generate an initial key")
	}

	client.ClearActions()

	// Test the trigger function
	// Activates trigger and polls client every 50 ms up to 10s for the appropriate action
	keyGenTrigger()
	maxWait := 10 * time.Second
	endTime := time.Now().Add(maxWait)
	successful := false
	for time.Now().Before(endTime) {
		time.Sleep(50 * time.Millisecond)
		if hasAction(client, "create", "secrets") {
			successful = true
			break
		}
	}
	if !successful {
		t.Errorf("trigger function failed to activate early key generation")
	}
}

func TestInitKeyRotationTick(t *testing.T) {
	rand := testRand()
	client := fake.NewSimpleClientset()
	registry, err := initKeyRegistry(client, rand, "namespace", "prefix", "label", 1024)
	if err != nil {
		t.Fatalf("initKeyRegistry() returned err: %v", err)
	}

	_, err = initKeyRotation(registry, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("initKeyRotation() returned err: %v", err)
	}
	if !hasAction(client, "create", "secrets") {
		t.Errorf("initKeyRotation() failed to generate an initial key")
	}

	client.ClearActions()

	maxWait := 10 * time.Second
	endTime := time.Now().Add(maxWait)
	successful := false
	for time.Now().Before(endTime) {
		time.Sleep(50 * time.Millisecond)
		if hasAction(client, "create", "secrets") {
			successful = true
			break
		}
	}
	if !successful {
		t.Errorf("trigger function failed to activate early key generation")
	}
}

func TestReuseKey(t *testing.T) {
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
	_, err = writeKey(client, key, []*x509.Certificate{cert}, "namespace", SealedSecretsKeyLabel, "prefix")
	if err != nil {
		t.Errorf("writeKey() failed with: %v", err)
	}

	client.ClearActions()

	registry, err := initKeyRegistry(client, rand, "namespace", "prefix", SealedSecretsKeyLabel, 1024)
	if err != nil {
		t.Fatalf("initKeyRegistry() returned err: %v", err)
	}

	_, err = initKeyRotation(registry, 0)
	if err != nil {
		t.Fatalf("initKeyRotation() returned err: %v", err)
	}
	if hasAction(client, "create", "secrets") {
		t.Errorf("initKeyRotation() should not create a new secret when one already exist and rotation is disabled")
	}
}

func writeLegacyKey(client kubernetes.Interface, key *rsa.PrivateKey, certs []*x509.Certificate, namespace, name string) (string, error) {
	certbytes := []byte{}
	for _, cert := range certs {
		certbytes = append(certbytes, pem.EncodeToMemory(&pem.Block{Type: certUtil.CertificateBlockType, Bytes: cert.Raw})...)
	}
	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Data: map[string][]byte{
			v1.TLSPrivateKeyKey: pem.EncodeToMemory(&pem.Block{Type: keyutil.RSAPrivateKeyBlockType, Bytes: x509.MarshalPKCS1PrivateKey(key)}),
			v1.TLSCertKey:       certbytes,
		},
		Type: v1.SecretTypeTLS,
	}

	createdSecret, err := client.CoreV1().Secrets(namespace).Create(&secret)
	if err != nil {
		return "", err
	}
	return createdSecret.Name, nil
}

func TestLegacySecret(t *testing.T) {
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

	_, err = writeLegacyKey(client, key, []*x509.Certificate{cert}, "namespace", "prefix")
	if err != nil {
		t.Errorf("writeKey() failed with: %v", err)
	}

	client.ClearActions()

	registry, err := initKeyRegistry(client, rand, "namespace", "prefix", SealedSecretsKeyLabel, 1024)
	if err != nil {
		t.Fatalf("initKeyRegistry() returned err: %v", err)
	}

	_, err = initKeyRotation(registry, 0)
	if err != nil {
		t.Fatalf("initKeyRotation() returned err: %v", err)
	}
	if hasAction(client, "create", "secrets") {
		t.Errorf("initKeyRotation() should not create a new secret when one already exist and rotation is disabled")
	}
}
