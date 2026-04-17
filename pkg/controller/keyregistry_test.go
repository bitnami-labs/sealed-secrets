package controller

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bitnami-labs/sealed-secrets/pkg/crypto"
)

func TestRegisterNewKey(t *testing.T) {
	const keySize = 2048
	validFor := time.Hour
	cn := "my-cn"
	kr := NewKeyRegistry(nil, "namespace", "prefix", "label", keySize)

	if kr.keyLen() != 0 {
		t.Fatal("this test assumes a new key registry has no keys")
	}

	key1, cert1, err := generatePrivateKeyAndCert(keySize, validFor, cn)
	if err != nil {
		t.Fatal(err)
	}
	t1 := time.Now()

	key2, cert2, err := generatePrivateKeyAndCert(keySize, validFor, cn)
	if err != nil {
		t.Fatal(err)
	}
	t2 := time.Now()

	if err := kr.registerNewKey("k2", key2, cert2, t2); err != nil {
		t.Fatal(err)
	}
	got, err := kr.latestPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	if got != key2 {
		t.Errorf("got: %v, want: %v", got, key2)
	}

	// key1 is older, so it shouldn't replace key2 as the mostRecentKey
	if err := kr.registerNewKey("k1", key1, cert1, t1); err != nil {
		t.Fatal(err)
	}
	got, err = kr.latestPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	if got != key2 {
		t.Errorf("got: %v, want: %v", got, key2)
	}
}

func TestLatestPrivateKeyEmpty(t *testing.T) {
	const keySize = 2048
	kr := NewKeyRegistry(nil, "namespace", "prefix", "label", keySize)

	key, err := kr.latestPrivateKey()
	if err == nil {
		t.Fatal("expected error from latestPrivateKey on empty registry, got nil")
	}
	if key != nil {
		t.Fatalf("expected nil key, got: %v", key)
	}
}

func TestPrivateKeys(t *testing.T) {
	const keySize = 2048
	validFor := time.Hour
	cn := "my-cn"
	kr := NewKeyRegistry(nil, "namespace", "prefix", "label", keySize)

	key1, cert1, err := generatePrivateKeyAndCert(keySize, validFor, cn)
	if err != nil {
		t.Fatal(err)
	}
	key2, cert2, err := generatePrivateKeyAndCert(keySize, validFor, cn)
	if err != nil {
		t.Fatal(err)
	}

	if err := kr.registerNewKey("k1", key1, cert1, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := kr.registerNewKey("k2", key2, cert2, time.Now()); err != nil {
		t.Fatal(err)
	}

	pkeys := kr.privateKeys()
	if got, want := len(pkeys), 2; got != want {
		t.Fatalf("privateKeys length: got %d, want %d", got, want)
	}

	fp1, err := crypto.PublicKeyFingerprint(&key1.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	fp2, err := crypto.PublicKeyFingerprint(&key2.PublicKey)
	if err != nil {
		t.Fatal(err)
	}

	if got, ok := pkeys[fp1]; !ok {
		t.Errorf("privateKeys missing fingerprint %s", fp1)
	} else if got.PublicKey.N.Cmp(key1.PublicKey.N) != 0 {
		t.Errorf("privateKeys[%s]: public key mismatch", fp1)
	}

	if got, ok := pkeys[fp2]; !ok {
		t.Errorf("privateKeys missing fingerprint %s", fp2)
	} else if got.PublicKey.N.Cmp(key2.PublicKey.N) != 0 {
		t.Errorf("privateKeys[%s]: public key mismatch", fp2)
	}
}

func TestKeyLen(t *testing.T) {
	const keySize = 2048
	validFor := time.Hour
	cn := "my-cn"
	kr := NewKeyRegistry(nil, "namespace", "prefix", "label", keySize)

	if got := kr.keyLen(); got != 0 {
		t.Fatalf("keyLen on empty registry: got %d, want 0", got)
	}

	key1, cert1, err := generatePrivateKeyAndCert(keySize, validFor, cn)
	if err != nil {
		t.Fatal(err)
	}
	if err := kr.registerNewKey("k1", key1, cert1, time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := kr.keyLen(); got != 1 {
		t.Fatalf("keyLen after one key: got %d, want 1", got)
	}

	key2, cert2, err := generatePrivateKeyAndCert(keySize, validFor, cn)
	if err != nil {
		t.Fatal(err)
	}
	if err := kr.registerNewKey("k2", key2, cert2, time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := kr.keyLen(); got != 2 {
		t.Fatalf("keyLen after two keys: got %d, want 2", got)
	}
}

func TestMostRecentKeyTimeEmpty(t *testing.T) {
	const keySize = 2048
	kr := NewKeyRegistry(nil, "namespace", "prefix", "label", keySize)

	_, err := kr.mostRecentKeyTime()
	if err == nil {
		t.Fatal("expected error from mostRecentKeyTime on empty registry, got nil")
	}
}

func TestMostRecentKeyTime(t *testing.T) {
	const keySize = 2048
	validFor := time.Hour
	cn := "my-cn"
	kr := NewKeyRegistry(nil, "namespace", "prefix", "label", keySize)

	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	key1, cert1, err := generatePrivateKeyAndCert(keySize, validFor, cn)
	if err != nil {
		t.Fatal(err)
	}
	key2, cert2, err := generatePrivateKeyAndCert(keySize, validFor, cn)
	if err != nil {
		t.Fatal(err)
	}

	// Register key1 with earlier time first.
	if err := kr.registerNewKey("k1", key1, cert1, t1); err != nil {
		t.Fatal(err)
	}
	got, err := kr.mostRecentKeyTime()
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(t1) {
		t.Errorf("mostRecentKeyTime after k1: got %v, want %v", got, t1)
	}

	// Register key2 with later time; mostRecentKeyTime should update.
	if err := kr.registerNewKey("k2", key2, cert2, t2); err != nil {
		t.Fatal(err)
	}
	got, err = kr.mostRecentKeyTime()
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(t2) {
		t.Errorf("mostRecentKeyTime after k2: got %v, want %v", got, t2)
	}

	// Register another key with an earlier time; mostRecentKeyTime should stay t2.
	key3, cert3, err := generatePrivateKeyAndCert(keySize, validFor, cn)
	if err != nil {
		t.Fatal(err)
	}
	tOld := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := kr.registerNewKey("k3", key3, cert3, tOld); err != nil {
		t.Fatal(err)
	}
	got, err = kr.mostRecentKeyTime()
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(t2) {
		t.Errorf("mostRecentKeyTime after older k3: got %v, want %v", got, t2)
	}
}

// pregenKey holds a pre-generated key and certificate for concurrent tests.
type pregenKey struct {
	key  *rsa.PrivateKey
	cert *x509.Certificate
}

func TestConcurrentAccess(t *testing.T) {
	const keySize = 2048
	const numKeys = 5
	const goroutinesPerOp = 10
	validFor := time.Hour
	cn := "my-cn"

	// Pre-generate all keys before spawning goroutines (key gen is slow).
	pregenKeys := make([]pregenKey, numKeys)
	for i := 0; i < numKeys; i++ {
		key, cert, err := generatePrivateKeyAndCert(keySize, validFor, cn)
		if err != nil {
			t.Fatal(err)
		}
		pregenKeys[i] = pregenKey{key: key, cert: cert}
	}

	kr := NewKeyRegistry(nil, "namespace", "prefix", "label", keySize)

	var wg sync.WaitGroup

	// Spawn goroutines that concurrently register keys.
	for i := 0; i < numKeys; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := fmt.Sprintf("k%d", idx)
			if err := kr.registerNewKey(name, pregenKeys[idx].key, pregenKeys[idx].cert, time.Now()); err != nil {
				t.Errorf("registerNewKey(%s): %v", name, err)
			}
		}(i)
	}

	// Spawn goroutines that concurrently read latestPrivateKey.
	for i := 0; i < goroutinesPerOp; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Ignore error since registry may be empty initially.
			_, _ = kr.latestPrivateKey()
		}()
	}

	// Spawn goroutines that concurrently read privateKeys.
	for i := 0; i < goroutinesPerOp; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = kr.privateKeys()
		}()
	}

	// Spawn goroutines that concurrently read keyLen.
	for i := 0; i < goroutinesPerOp; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = kr.keyLen()
		}()
	}

	// Spawn goroutines that concurrently read getCert.
	for i := 0; i < goroutinesPerOp; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Ignore error since registry may be empty initially.
			_, _ = kr.getCert()
		}()
	}

	// Spawn goroutines that concurrently read mostRecentKeyTime.
	// This can panic if called before any key is registered, so we
	// only call it after ensuring at least one key is registered.
	// We register one key synchronously first.
	wg.Wait()

	// At this point all keys are registered. Now test concurrent reads
	// of mostRecentKeyTime alongside other operations.
	for i := 0; i < goroutinesPerOp; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = kr.mostRecentKeyTime()
		}()
	}

	for i := 0; i < goroutinesPerOp; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = kr.latestPrivateKey()
		}()
	}

	for i := 0; i < goroutinesPerOp; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = kr.privateKeys()
		}()
	}

	for i := 0; i < goroutinesPerOp; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = kr.getCert()
		}()
	}

	wg.Wait()

	// Verify final state: all keys were registered.
	if got := kr.keyLen(); got != numKeys {
		t.Errorf("keyLen after concurrent registration: got %d, want %d", got, numKeys)
	}
}
