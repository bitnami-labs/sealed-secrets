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

func readBlacklist(client kubernetes.Interface, namespace, blacklistName string) (map[string]struct{}, error) {
	secret, err := client.Core().Secrets(namespace).Get(blacklistName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	keynames := map[string]struct{}{}
	for keyname := range secret.Data {
		if (keyname != v1.TLSPrivateKeyKey) && (keyname != v1.TLSCertKey) {
			keynames[keyname] = struct{}{}
		}
	}
	return keynames, nil
}

func updateBlacklist(client kubernetes.Interface, namespace, blacklistName, keyname string) error {
	blacklist, err := client.Core().Secrets(namespace).Get(blacklistName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	blacklist.Data[keyname] = []byte{}
	if _, err = client.Core().Secrets(namespace).Update(blacklist); err != nil {
		return err
	}
	return nil
}

func writeBlacklist(client kubernetes.Interface, privkey *rsa.PrivateKey, cert *x509.Certificate, namespace, blacklistName string) error {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      blacklistName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			v1.TLSPrivateKeyKey: certUtil.EncodePrivateKeyPEM(privkey),
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
	currentKeyName string
	keys           map[string]*rsa.PrivateKey
	certs          map[string]*x509.Certificate
	blacklist      map[string]struct{}
}

func NewKeyRegistry() *KeyRegistry {
	return &KeyRegistry{
		keys:      make(map[string]*rsa.PrivateKey),
		certs:     make(map[string]*x509.Certificate),
		blacklist: make(map[string]struct{}),
	}
}

func (kr *KeyRegistry) checkBlacklist(keyname string) error {
	if _, ok := kr.blacklist[keyname]; ok {
		return fmt.Errorf("%s is blacklisted", keyname)
	}
	return nil
}

func (kr *KeyRegistry) CurrentKeyName() string {
	return kr.currentKeyName
}

func (kr *KeyRegistry) GetPrivateKey(keyname string) (*rsa.PrivateKey, error) {
	key, ok := kr.keys[keyname]
	if !ok {
		return nil, fmt.Errorf("No key exists with name %s", keyname)
	}
	if err := kr.checkBlacklist(keyname); err != nil {
		return nil, err
	}
	return key, nil
}

func (kr *KeyRegistry) registerNewKey(keyName string, privKey *rsa.PrivateKey, cert *x509.Certificate) {
	kr.keys[keyName] = privKey
	kr.certs[keyName] = cert
	kr.currentKeyName = keyName
}

func (kr *KeyRegistry) Cert() *x509.Certificate {
	return kr.certs[kr.currentKeyName]
}

func (kr *KeyRegistry) GetCert(keyname string) (*x509.Certificate, error) {
	cert, ok := kr.certs[keyname]
	if !ok {
		return nil, fmt.Errorf("No key with name %s", keyname)
	}
	if err := kr.checkBlacklist(keyname); err != nil {
		return nil, err
	}
	return cert, nil
}

func (kr *KeyRegistry) blacklistKey(keyName string) {
	kr.blacklist[keyName] = struct{}{}
}

func (kr *KeyRegistry) getBlacklistedKeys() []string {
	list := make([]string, len(kr.blacklist))
	count := 0
	for name, _ := range kr.blacklist {
		list[count] = name
		count++
	}
	return list
}

func (kr *KeyRegistry) PrivateKey() *rsa.PrivateKey {
	return kr.keys[kr.currentKeyName]
}
