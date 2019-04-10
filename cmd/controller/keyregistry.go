package main

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	certUtil "k8s.io/client-go/util/cert"
)

func readKeyRegistry(client kubernetes.Interface, namespace, listName string) (map[string]struct{}, error) {
	secret, err := client.Core().Secrets(namespace).Get(listName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	keyNames := map[string]struct{}{}
	for keyName := range secret.Data {
		if (keyName != v1.TLSPrivateKeyKey) && (keyName != v1.TLSCertKey) {
			keyNames[keyName] = struct{}{}
		}
	}
	return keyNames, nil
}

func updateKeyRegistry(client kubernetes.Interface, namespace, listName, newKeyName string) error {
	secret, err := client.Core().Secrets(namespace).Get(listName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	secret.Data[newKeyName] = []byte{}
	if _, err := client.Core().Secrets(namespace).Update(secret); err != nil {
		return err
	}
	return nil
}

func writeKeyRegistry(client kubernetes.Interface, key *rsa.PrivateKey, cert *x509.Certificate, namespace, listName string) error {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      listName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			v1.TLSPrivateKeyKey: certUtil.EncodePrivateKeyPEM(key),
			v1.TLSCertKey:       certUtil.EncodeCertPEM(cert),
		},
		Type: v1.SecretTypeTLS,
	}
	if _, err := client.Core().Secrets(namespace).Create(secret); err != nil {
		return err
	}
	return nil
}

type KeyRegistry struct {
	client         kubernetes.Interface
	namespace      string
	listname       string
	prefix         string
	keysize        int
	currentKeyName string
	keys           map[string]*rsa.PrivateKey
	certs          map[string]*x509.Certificate
	blacklist      map[string]struct{}
}

func NewKeyRegistry(client kubernetes.Interface, namespace, listname string, keysize int) *KeyRegistry {
	return &KeyRegistry{
		client:    client,
		namespace: namespace,
		listname:  listname,
		keysize:   keysize,
		keys:      make(map[string]*rsa.PrivateKey),
		certs:     make(map[string]*x509.Certificate),
		blacklist: make(map[string]struct{}),
	}
}

func (kr *KeyRegistry) generateKey(keysize int) error {
	key, cert, err := generatePrivateKeyAndCert(keysize)
	if err != nil {
		return err
	}
	certs := []*x509.Certificate{cert}
	generatedName, err := writeKey(kr.client, key, certs, kr.namespace, kr.listname)
	if err != nil {
		return err
	}
	if err := updateKeyRegistry(kr.client, kr.namespace, kr.listname, generatedName); err != nil {
		return err
	}
	kr.keys[generatedName] = key
	kr.certs[generatedName] = cert
	kr.currentKeyName = generatedName
	return nil
}

func (kr *KeyRegistry) blacklistKey(keyname string) error {
	if err := blacklistKey(kr.client, kr.namespace, keyname); err != nil {
		return err
	}
	kr.blacklist[keyname] = struct{}{}
	delete(kr.keys, keyname)
	delete(kr.certs, keyname)
	return nil
}

func (kr *KeyRegistry) registerNewKey(keyName string, privKey *rsa.PrivateKey, cert *x509.Certificate) {
	kr.keys[keyName] = privKey
	kr.certs[keyName] = cert
	kr.currentKeyName = keyName
}

func (kr *KeyRegistry) isBlacklisted(keyname string) bool {
	_, ok := kr.blacklist[keyname]
	return ok
}

func (kr *KeyRegistry) latestKeyName() string {
	return kr.currentKeyName
}

func (kr *KeyRegistry) getPrivateKey(keyname string) (*rsa.PrivateKey, error) {
	key, ok := kr.keys[keyname]
	if !ok {
		return nil, fmt.Errorf("No key exists with name %s", keyname)
	}
	if kr.isBlacklisted(keyname) {
		return nil, ErrKeyBlacklisted
	}
	return key, nil
}

func (kr *KeyRegistry) getCert(keyname string) (*x509.Certificate, error) {
	cert, ok := kr.certs[keyname]
	if !ok {
		return nil, fmt.Errorf("No key with name %s", keyname)
	}
	if kr.isBlacklisted(keyname) {
		return nil, ErrKeyBlacklisted
	}
	return cert, nil
}
