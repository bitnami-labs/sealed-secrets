package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"log"

	"k8s.io/client-go/kubernetes"
	certUtil "k8s.io/client-go/util/cert"
)

type KeyRegistry struct {
	client      kubernetes.Interface
	namespace   string
	keyPrefix   string
	keyLabel    string
	keysize     int
	privateKeys []*rsa.PrivateKey
	cert        *x509.Certificate
}

func NewKeyRegistry(client kubernetes.Interface, namespace, keyPrefix, keyLabel string, keysize int) *KeyRegistry {
	return &KeyRegistry{
		client:      client,
		namespace:   namespace,
		keyPrefix:   keyPrefix,
		keysize:     keysize,
		keyLabel:    keyLabel,
		privateKeys: []*rsa.PrivateKey{},
	}
}

func (kr *KeyRegistry) generateKey() (string, error) {
	key, cert, err := generatePrivateKeyAndCert(kr.keysize)
	if err != nil {
		return "", err
	}
	certs := []*x509.Certificate{cert}
	generatedName, err := writeKey(kr.client, key, certs, kr.namespace, kr.keyLabel, kr.keyPrefix)
	if err != nil {
		return "", err
	}
	// Only store key to local store if write to k8s worked
	kr.registerNewKey(generatedName, key, cert)
	log.Printf("New key written to %s/%s\n", kr.namespace, generatedName)
	log.Printf("Certificate is \n%s\n", pem.EncodeToMemory(&pem.Block{Type: certUtil.CertificateBlockType, Bytes: cert.Raw}))
	return generatedName, nil
}

func (kr *KeyRegistry) registerNewKey(keyName string, privKey *rsa.PrivateKey, cert *x509.Certificate) {
	kr.privateKeys = append(kr.privateKeys, privKey)
	kr.cert = cert
}

func (kr *KeyRegistry) latestPrivateKey() *rsa.PrivateKey {
	return kr.privateKeys[len(kr.privateKeys)-1]
}

func (kr *KeyRegistry) getCert(keyname string) (*x509.Certificate, error) {
	return kr.cert, nil
}
