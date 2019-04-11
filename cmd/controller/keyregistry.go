package main

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"

	"k8s.io/client-go/kubernetes"
)

type KeyRegistry struct {
	client         kubernetes.Interface
	namespace      string
	listname       string
	prefix         string
	keysize        int
	currentKeyName string
	keys           map[string]*rsa.PrivateKey
	certs          map[string]*x509.Certificate
}

func NewKeyRegistry(client kubernetes.Interface, namespace, listname string, keysize int) *KeyRegistry {
	return &KeyRegistry{
		client:    client,
		namespace: namespace,
		listname:  listname,
		keysize:   keysize,
		keys:      make(map[string]*rsa.PrivateKey),
		certs:     make(map[string]*x509.Certificate),
	}
}

func (kr *KeyRegistry) generateKey() (string, error) {
	key, cert, err := generatePrivateKeyAndCert(kr.keysize)
	if err != nil {
		return "", err
	}
	certs := []*x509.Certificate{cert}
	generatedName, err := writeKey(kr.client, key, certs, kr.namespace, kr.listname)
	if err != nil {
		return "", err
	}
	// Only store key to local store if write to k8s worked
	kr.registerNewKey(generatedName, key, cert)
	return generatedName, nil
}

// blacklistKey deletes a key from the local store and marks the corresponding k8s secret
// as compromised. This effectively deletes the key from the sealedsecrets controller
// while the key is still available to admins if need be
func (kr *KeyRegistry) blacklistKey(keyname string) error {
	if err := blacklistKey(kr.client, kr.namespace, keyname); err != nil {
		return err
	}
	// Only delete if modifying the k8s secret succeeded
	delete(kr.keys, keyname)
	delete(kr.certs, keyname)
	return nil
}

func (kr *KeyRegistry) registerNewKey(keyName string, privKey *rsa.PrivateKey, cert *x509.Certificate) {
	kr.keys[keyName] = privKey
	kr.certs[keyName] = cert
	kr.currentKeyName = keyName
}

func (kr *KeyRegistry) latestKeyName() string {
	return kr.currentKeyName
}

func (kr *KeyRegistry) getPrivateKey(keyname string) (*rsa.PrivateKey, error) {
	key, ok := kr.keys[keyname]
	if !ok {
		return nil, fmt.Errorf("No key exists with name %s", keyname)
	}
	return key, nil
}

func (kr *KeyRegistry) getCert(keyname string) (*x509.Certificate, error) {
	cert, ok := kr.certs[keyname]
	if !ok {
		return nil, fmt.Errorf("No key with name %s", keyname)
	}
	return cert, nil
}
