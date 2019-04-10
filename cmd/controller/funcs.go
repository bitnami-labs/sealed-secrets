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

// ScheduleJobWithTrigger creates a long-running loop that runs a jub each
// loop
// returns a trigger function that runs the job early when called
func ScheduleJobWithTrigger(period time.Duration, job func()) func() {
	trigger := make(chan struct{})
	go func() {
		for {
			<-trigger
			job()
		}
	}()
	go func() {
		for {
			time.Sleep(period)
			trigger <- struct{}{}
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
	prefix string,
) func() error {
	return func() error {
		privKey, cert, err := generatePrivateKeyAndCert(keySize)
		if err != nil {
			return err
		}
		certs := []*x509.Certificate{cert}
		newKeyName, err := writeKey(client, privKey, certs, namespace, prefix)
		if err != nil {
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

// validateKeyName is used to validate whether a string can be used as part of a keyname in kubernetes
func validateKeyName(name string) error {
	maxLen := 245
	nameLen := len(name)
	if nameLen > maxLen {
		return fmt.Errorf("keyname name is too long, must be shorter than %d, got %d", maxLen, nameLen)
	}
	for _, char := range name {
		if !strings.ContainsRune(kubeChars, char) {
			return fmt.Errorf("name contains illegal character %c", char)
		}
	}
	return nil
}
