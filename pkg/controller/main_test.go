package controller

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	krand "k8s.io/apimachinery/pkg/util/rand"
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

// generateNameReactor implements the logic required for the GenerateName field to work when using
// the fake client. Add it with client.PrependReactor to your fake client.
func generateNameReactor(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
	s := action.(ktesting.CreateAction).GetObject().(*v1.Secret)
	if s.Name == "" && s.GenerateName != "" {
		s.Name = fmt.Sprintf("%s-%s", s.GenerateName, krand.String(16))
	}
	return false, nil, nil
}

func TestInitKeyRegistry(t *testing.T) {
	ctx := context.Background()
	rand := testRand()
	client := fake.NewSimpleClientset()
	client.PrependReactor("create", "secrets", generateNameReactor)

	registry, err := initKeyRegistry(ctx, client, rand, "namespace", "prefix", "label", 1024)
	if err != nil {
		t.Fatalf("initKeyRegistry() returned err: %v", err)
	}

	// Add a key to the controller for second test
	validFor := time.Hour
	cn := "my-cn"
	_, err = registry.generateKey(ctx, validFor, cn, "", "")
	if err != nil {
		t.Fatal(err)
	}

	if !hasAction(client, "create", "secrets") {
		t.Fatalf("Error adding initial key to registry")
	}
	client.ClearActions()

	// Due to limitations of the fake client, we cannot test whether initKeyRegistry is able
	// to pick up existing keys
	_, err = initKeyRegistry(ctx, client, rand, "namespace", "prefix", "label", 1024)
	if err != nil {
		t.Fatalf("initKeyRegistry() returned err: %v", err)
	}
	if !hasAction(client, "list", "secrets") {
		t.Errorf("initKeyRegistry() failed to read existing keys")
	}
}

func TestInitKeyRotation(t *testing.T) {
	ctx := context.Background()
	rand := testRand()
	client := fake.NewSimpleClientset()
	client.PrependReactor("create", "secrets", generateNameReactor)

	registry, err := initKeyRegistry(ctx, client, rand, "namespace", "prefix", "label", 1024)
	if err != nil {
		t.Fatalf("initKeyRegistry() returned err: %v", err)
	}

	validFor := time.Hour
	cn := "my-cn"
	keyGenTrigger, err := initKeyRenewal(ctx, registry, 0, validFor, time.Time{}, cn, "", "")
	if err != nil {
		t.Fatalf("initKeyRenewal() returned err: %v", err)
	}
	if !hasAction(client, "create", "secrets") {
		t.Errorf("initKeyRenewal() failed to generate an initial key")
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
	ctx := context.Background()
	rand := testRand()
	client := fake.NewSimpleClientset()
	client.PrependReactor("create", "secrets", generateNameReactor)

	registry, err := initKeyRegistry(ctx, client, rand, "namespace", "prefix", "label", 1024)
	if err != nil {
		t.Fatalf("initKeyRegistry() returned err: %v", err)
	}

	validFor := time.Hour
	cn := "my-cn"
	_, err = initKeyRenewal(ctx, registry, 100*time.Millisecond, validFor, time.Time{}, cn, "", "")
	if err != nil {
		t.Fatalf("initKeyRenewal() returned err: %v", err)
	}
	if !hasAction(client, "create", "secrets") {
		t.Errorf("initKeyRenewal() failed to generate an initial key")
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
	client.PrependReactor("create", "secrets", generateNameReactor)

	_, err = writeKey(ctx, client, key, []*x509.Certificate{cert}, "namespace", SealedSecretsKeyLabel, "prefix", "", "")
	if err != nil {
		t.Errorf("writeKey() failed with: %v", err)
	}

	client.ClearActions()

	registry, err := initKeyRegistry(ctx, client, rand, "namespace", "prefix", SealedSecretsKeyLabel, 1024)
	if err != nil {
		t.Fatalf("initKeyRegistry() returned err: %v", err)
	}

	validFor := time.Hour
	cn := "my-cn"
	_, err = initKeyRenewal(ctx, registry, 0, validFor, time.Time{}, cn, "", "")
	if err != nil {
		t.Fatalf("initKeyRenewal() returned err: %v", err)
	}
	if hasAction(client, "create", "secrets") {
		t.Errorf("initKeyRenewal() should not create a new secret when one already exist and rotation is deactivated")
	}
}

func TestRenewStaleKey(t *testing.T) {
	ctx := context.Background()
	rand := testRand()
	key, err := rsa.GenerateKey(rand, 512)
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}

	// we'll simulate the existence of a secret that is about to expire
	// by making it old enough so that it's just "staleness" short of using
	// the full rotation "period".
	const (
		period    = 20 * time.Second
		staleness = 100 * time.Millisecond
		oldAge    = period - staleness
	)
	notBefore := time.Now().Add(-oldAge)

	cert, err := signKeyWithNotBefore(rand, key, notBefore)
	if err != nil {
		t.Fatalf("signKey failed: %v", err)
	}

	client := fake.NewSimpleClientset()
	client.PrependReactor("create", "secrets", generateNameReactor)

	_, err = writeKey(ctx, client, key, []*x509.Certificate{cert}, "namespace", SealedSecretsKeyLabel, "prefix", "", "")
	if err != nil {
		t.Errorf("writeKey() failed with: %v", err)
	}

	registry, err := initKeyRegistry(ctx, client, rand, "namespace", "prefix", SealedSecretsKeyLabel, 1024)
	if err != nil {
		t.Fatalf("initKeyRegistry() returned err: %v", err)
	}

	validFor := time.Hour
	cn := "my-cn"
	_, err = initKeyRenewal(ctx, registry, period, validFor, time.Time{}, cn, "", "")
	if err != nil {
		t.Fatalf("initKeyRenewal() returned err: %v", err)
	}

	client.ClearActions()

	maxWait := 1 * time.Second
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

func TestKeyCutoff(t *testing.T) {
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

	// we'll simulate the existence of a secret that would be still valid
	// according to our rotation period, if it were not for it being older than the cutoff date.
	const (
		period = 24 * time.Hour
		oldAge = 1 * time.Hour
	)
	client := fake.NewSimpleClientset()
	client.PrependReactor("create", "secrets", generateNameReactor)

	_, err = writeKey(ctx, client, key, []*x509.Certificate{cert}, "namespace", SealedSecretsKeyLabel, "prefix", "", "",
		writeKeyWithCreationTime(metav1.NewTime(time.Now().Add(-oldAge))))
	if err != nil {
		t.Errorf("writeKey() failed with: %v", err)
	}

	registry, err := initKeyRegistry(ctx, client, rand, "namespace", "prefix", SealedSecretsKeyLabel, 1024)
	if err != nil {
		t.Fatalf("initKeyRegistry() returned err: %v", err)
	}

	client.ClearActions()

	// by setting cutoff to "now" we effectively force the creation of a new key.
	validFor := time.Hour
	cn := "my-cn"
	_, err = initKeyRenewal(ctx, registry, period, validFor, time.Now(), cn, "", "")
	if err != nil {
		t.Fatalf("initKeyRenewal() returned err: %v", err)
	}

	if !hasAction(client, "create", "secrets") {
		t.Errorf("trigger function failed to activate early key generation")
	}
}

func writeLegacyKey(ctx context.Context, client kubernetes.Interface, key *rsa.PrivateKey, certs []*x509.Certificate, namespace, name string) (string, error) {
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

	createdSecret, err := client.CoreV1().Secrets(namespace).Create(ctx, &secret, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}
	return createdSecret.Name, nil
}

func TestLegacySecret(t *testing.T) {
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
	client.PrependReactor("create", "secrets", generateNameReactor)

	_, err = writeLegacyKey(ctx, client, key, []*x509.Certificate{cert}, "namespace", "prefix")
	if err != nil {
		t.Errorf("writeKey() failed with: %v", err)
	}

	client.ClearActions()

	registry, err := initKeyRegistry(ctx, client, rand, "namespace", "prefix", SealedSecretsKeyLabel, 1024)
	if err != nil {
		t.Fatalf("initKeyRegistry() returned err: %v", err)
	}

	validFor := time.Hour
	cn := "my-cn"
	_, err = initKeyRenewal(ctx, registry, 0, validFor, time.Time{}, cn, "", "")
	if err != nil {
		t.Fatalf("initKeyRenewal() returned err: %v", err)
	}
	if hasAction(client, "create", "secrets") {
		t.Errorf("initKeyRenewal() should not create a new secret when one already exist and rotation is deactivated")
	}
}
