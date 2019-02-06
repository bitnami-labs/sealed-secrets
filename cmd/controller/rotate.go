package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"log"
	"time"

	"k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	certUtil "k8s.io/client-go/util/cert"
)

type keyNameGen func() (string, error)

func ScheduleJobWithTrigger(period time.Duration, trigger chan struct{}, job func()) {
	sched := make(chan struct{})
	go func() {
		time.Sleep(period)
		sched <- struct{}{}
	}()
	select {
	case <-trigger:
	case <-sched:
	}
	go job()
	go ScheduleJobWithTrigger(period, trigger, job)
}

func rotationFunc(rotateKey func() error) func() {
	return func() {
		if err := rotateKey(); err != nil {
			log.Printf("Failed to generate new key : %v\n", err)
		}
	}
}

func createAutomaticKeyRotationJob(client kubernetes.Interface,
	privateKeyRegistry *KeyRegistry,
	certRegistry *CertRegistry,
	namespace string,
	keySize int,
	nameGen keyNameGen,
) func() error {
	return func() error {
		newKeyName, err := generateNewKeyName(client, namespace, nameGen)
		if err != nil {
			return err
		}
		privKey, cert, err := generatePrivateKeyAndCert(keySize)
		if err != nil {
			return err
		}
		if err = writeKeyToKube(client, privKey, cert, namespace, newKeyName); err != nil {
			return err
		}
		log.Printf("New key written to %s/%s\n", namespace, newKeyName)
		log.Printf("Certificate is \n%s\n", certUtil.EncodeCertPEM(cert))
		privateKeyRegistry.register(newKeyName, privKey)
		certRegistry.register(newKeyName, cert)
		return nil
	}
}

func generateNewKeyName(client kubernetes.Interface, namespace string, generateName keyNameGen) (string, error) {
	for i := 0; i < 10; i++ {
		keyName, err := generateName()
		if err != nil {
			return "", err
		}
		_, err = client.Core().Secrets(namespace).Get(keyName, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				// Found a keyname that doesn't exist
				return keyName, nil
			} else {
				return "", err
			}
		}
	}
	// If this fails 10 times, bad things
	return "", errors.New("Failed to generate new key name not in use")
}

func generatePrivateKeyAndCert(keySize int) (*rsa.PrivateKey, *x509.Certificate, error) {
	r := rand.Reader
	privKey, err := rsa.GenerateKey(r, keySize)
	if err != nil {
		return nil, nil, err
	}
	cert, err := signKey(r, privKey)
	if err != nil {
		return nil, nil, err
	}
	return privKey, cert, nil
}

func writeKeyToKube(client kubernetes.Interface, key *rsa.PrivateKey, cert *x509.Certificate, namespace, keyName string) error {
	data := certUtil.EncodeCertPEM(cert)
	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      keyName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			v1.TLSPrivateKeyKey: certUtil.EncodePrivateKeyPEM(key),
			v1.TLSCertKey:       data,
		},
		Type: v1.SecretTypeTLS,
	}
	_, err := client.Core().Secrets(namespace).Create(&secret)
	return err
}

type KeyBlacklist struct {
	list map[string]struct{}
}

func (bl *KeyBlacklist) blacklist(keyName string) {
	bl.list[keyName] = struct{}{}
}

type KeyRegistry struct {
	registry map[string]*rsa.PrivateKey
}

func (kr *KeyRegistry) register(keyName string, privKey *rsa.PrivateKey) {
	kr.registry[keyName] = privKey
}

func (kr *KeyRegistry) remove(keyName string) {
	_, ok := kr.registry[keyName]
	if !ok {
		log.Println("Attempted to remove a non-existant private key")
		return
	}
	delete(kr.registry, keyName)
}

type CertRegistry struct {
	registry map[string]*x509.Certificate
}

func (cr *CertRegistry) register(keyname string, cert *x509.Certificate) {
	cr.registry[keyname] = cert
}

func (cr *CertRegistry) remove(keyname string) {
	_, ok := cr.registry[keyname]
	if !ok {
		log.Println("Attempted to remove non-existant key")
		return
	}
	delete(cr.registry, keyname)
}

func (cr *CertRegistry) GetCertsArray() []*x509.Certificate {
	arr := make([]*x509.Certificate, len(cr.registry))
	count := 0
	for _, cert := range cr.registry {
		arr[count] = cert
		count++
	}
	return arr
}
