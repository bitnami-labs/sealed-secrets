package main

import (
	"crypto/x509"
	"fmt"
	"log"
	"strings"
	"time"

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

func createKeyGenJob(client kubernetes.Interface,
	keyRegistry *KeyRegistry,
	namespace, listname string,
	keySize int,
	nameGen keyNameGen,
) func() error {
	return func() error {
		newKeyName, err := nameGen()
		privKey, cert, err := generatePrivateKeyAndCert(keySize)
		if err != nil {
			return err
		}
		certs := []*x509.Certificate{cert}
		if err = writeKey(client, privKey, certs, namespace, newKeyName); err != nil {
			return err
		}
		if err = updateKeyRegistry(client, namespace, listname, newKeyName); err != nil {
			return err
		}
		log.Printf("New key written to %s/%s\n", namespace, newKeyName)
		log.Printf("Certificate is \n%s\n", certUtil.EncodeCertPEM(cert))
		keyRegistry.registerNewKey(newKeyName, privKey, cert)
		return nil
	}
}

func createBlacklister(client kubernetes.Interface, namespace, blacklistName string, keyRegistry *KeyRegistry, trigger func()) func(string) (bool, error) {
	return func(keyName string) (bool, error) {
		if _, ok := keyRegistry.keys[keyName]; !ok {
			return false, fmt.Errorf("key %s does not exist", keyName)
		}
		if _, ok := keyRegistry.blacklist[keyName]; ok {
			return false, fmt.Errorf("key %s is already blacklisted", keyName)
		}
		blacklist, err := client.Core().Secrets(namespace).Get(blacklistName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		blacklist.Data[keyName] = []byte{}
		if _, err = client.Core().Secrets(namespace).Update(blacklist); err != nil {
			return false, err
		}
		keyRegistry.blacklistKey(keyName)
		// If the latest key is being blacklisted, generate a new key
		if keyName == keyRegistry.latestKeyName() {
			trigger()
			return true, nil
		}
		return false, nil
	}
}

const kubeChars = "abcdefghijklmnopqrstuvwxyz0123456789-"

// PrefixedNameGen creates a function that generates keynames when called.
// Keynames are of the form <prefix>-<number>.
// where the inital count should be set to the number of existing keys,
// and is incremented every time the generator is called.
func PrefixedNameGen(prefix string, initialCount int) (func() (string, error), error) {
	count := initialCount
	maxLen := 245
	prefixLen := len(prefix)
	if prefixLen > maxLen {
		return nil, fmt.Errorf("keyname prefix is too long, must be shorter than %d, got %d", maxLen, prefixLen)
	}
	for _, char := range prefix {
		if !strings.ContainsRune(kubeChars, char) {
			return nil, fmt.Errorf("keyname prefix contains illegal character %c", char)
		}
	}
	return func() (string, error) {
		name := fmt.Sprintf("%s-%d", prefix, count)
		count++
		return name, nil
	}, nil
}
