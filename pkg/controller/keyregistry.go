package controller

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/bitnami-labs/sealed-secrets/pkg/crypto"
	"k8s.io/client-go/kubernetes"
	certUtil "k8s.io/client-go/util/cert"
)

// A Key holds the cryptographic key pair and some metadata about it.
type Key struct {
	private      *rsa.PrivateKey
	cert         *x509.Certificate
	fingerprint  string
	orderingTime time.Time
}

// A KeyRegistry manages the key pairs used to (un)seal secrets.
type KeyRegistry struct {
	mu            sync.RWMutex
	client        kubernetes.Interface
	namespace     string
	keyPrefix     string
	keyLabel      string
	keysize       int
	keys          map[string]*Key
	mostRecentKey *Key
}

// NewKeyRegistry creates a new KeyRegistry.
func NewKeyRegistry(client kubernetes.Interface, namespace, keyPrefix, keyLabel string, keysize int) *KeyRegistry {
	return &KeyRegistry{
		client:    client,
		namespace: namespace,
		keyPrefix: keyPrefix,
		keysize:   keysize,
		keyLabel:  keyLabel,
		keys:      map[string]*Key{},
	}
}

func (kr *KeyRegistry) generateKey(ctx context.Context, validFor time.Duration, cn string, privateKeyAnnotations string, privateKeyLabels string) (string, error) {
	key, cert, err := generatePrivateKeyAndCert(kr.keysize, validFor, cn)
	if err != nil {
		return "", err
	}
	certs := []*x509.Certificate{cert}
	generatedName, err := writeKey(ctx, kr.client, key, certs, kr.namespace, kr.keyLabel, kr.keyPrefix, privateKeyAnnotations, privateKeyLabels)
	if err != nil {
		return "", err
	}
	// Only store key to local store if write to k8s worked
	if err := kr.registerNewKey(generatedName, key, cert, time.Now()); err != nil {
		return "", err
	}
	slog.Info("New key written", "namespace", kr.namespace, "name", generatedName)
	slog.Info("Certificate generated", "certificate", pem.EncodeToMemory(&pem.Block{Type: certUtil.CertificateBlockType, Bytes: cert.Raw}))
	return generatedName, nil
}

func (kr *KeyRegistry) registerNewKey(keyName string, privKey *rsa.PrivateKey, cert *x509.Certificate, orderingTime time.Time) error {
	fingerprint, err := crypto.PublicKeyFingerprint(&privKey.PublicKey)
	if err != nil {
		return err
	}

	k := &Key{
		private:      privKey,
		cert:         cert,
		fingerprint:  fingerprint,
		orderingTime: orderingTime,
	}

	kr.mu.Lock()
	defer kr.mu.Unlock()

	kr.keys[k.fingerprint] = k

	if kr.mostRecentKey == nil || kr.mostRecentKey.orderingTime.Before(orderingTime) {
		kr.mostRecentKey = k
	}

	return nil
}

func (kr *KeyRegistry) latestPrivateKey() (*rsa.PrivateKey, error) {
	kr.mu.RLock()
	defer kr.mu.RUnlock()

	if kr.mostRecentKey == nil {
		return nil, fmt.Errorf("key registry has no keys")
	}
	return kr.mostRecentKey.private, nil
}

// privateKeys returns a snapshot copy of the private keys so callers
// can iterate without holding the mutex.
func (kr *KeyRegistry) privateKeys() map[string]*rsa.PrivateKey {
	kr.mu.RLock()
	defer kr.mu.RUnlock()

	m := make(map[string]*rsa.PrivateKey, len(kr.keys))
	for k, v := range kr.keys {
		m[k] = v.private
	}
	return m
}

func (kr *KeyRegistry) keyLen() int {
	kr.mu.RLock()
	defer kr.mu.RUnlock()

	return len(kr.keys)
}

func (kr *KeyRegistry) mostRecentKeyTime() (time.Time, error) {
	kr.mu.RLock()
	defer kr.mu.RUnlock()

	if kr.mostRecentKey == nil {
		return time.Time{}, fmt.Errorf("key registry has no keys")
	}
	return kr.mostRecentKey.orderingTime, nil
}

// getCert returns the current certificate. This method can be called by another goroutine.
func (kr *KeyRegistry) getCert() (*x509.Certificate, error) {
	kr.mu.RLock()
	defer kr.mu.RUnlock()

	if kr.mostRecentKey == nil {
		return nil, fmt.Errorf("key registry has no keys")
	}
	return kr.mostRecentKey.cert, nil
}
