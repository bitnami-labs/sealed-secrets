package main

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"
)

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
