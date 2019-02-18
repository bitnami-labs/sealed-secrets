package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"time"

	"k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	certUtil "k8s.io/client-go/util/cert"
)

type keyNameGen func() (string, error)

func ScheduleJobWithTrigger(period time.Duration, job func()) func() {
	trigger := make(chan struct{})
	go func() {
		for {
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
		}
	}()
	return func() {
		trigger <- struct{}{}
	}
}

func rotationErrorLogger(rotateKey func() error) func() {
	return func() {
		if err := rotateKey(); err != nil {
			log.Printf("Failed to generate new key : %v\n", err)
		}
	}
}

func createKeyRotationJob(client kubernetes.Interface,
	keyRegistry *KeyRegistry,
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
		keyRegistry.registerNewKey(newKeyName, privKey, cert)
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

func createBlacklister(keyRegistry *KeyRegistry, trigger chan struct{}) func(string) error {
	return func(keyName string) error {
		key, err := keyRegistry.GetPrivateKey(keyName)
		if err != nil {
			return err
		}
		keyRegistry.blacklistKey(keyName)
		// If the latest key is being blacklisted, generate a new key
		if key == keyRegistry.PrivateKey() {
			trigger <- struct{}{}
		}
		return nil
	}
}

func PrefixedNameGen(prefix string) (func() (string, error), error) {
	count := 0
	// TODO: validate prefix string for kubernetes compatibility
	return func() (string, error) {
		name := fmt.Sprintf("%s-%d", prefix, count)
		count++
		return name, nil
	}, nil
}
