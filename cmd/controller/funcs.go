package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	certUtil "k8s.io/client-go/util/cert"
)

const (
	compromised = "compromised"
)

// ScheduleJobWithTrigger creates a long-running loop that runs a job each
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

func createKeyGenJob(keyRegistry *KeyRegistry) func() error {
	return func() error {
		generatedName, err := keyRegistry.generateKey()
		if err != nil {
			return err
		}
		log.Printf("New key written to %s/%s\n", keyRegistry.namespace, generatedName)
		log.Printf("Certificate is \n%s\n", certUtil.EncodeCertPEM(keyRegistry.certs[generatedName]))
		return nil
	}
}

const (
	kubeChars     = "abcdefghijklmnopqrstuvwxyz0123456789-" // Acceptable characters in k8s resource name
	maxNameLength = 245                                     // Max resource name length is 253, leave some room for a suffix
)

// validateKeyName is used to validate whether a string can be used as part of a keyname in kubernetes
func validateKeyName(name string) error {
	if len(name) > maxNameLength {
		return fmt.Errorf("name is too long, must be shorter than %d, got %d", maxNameLength, len(name))
	}
	for _, char := range name {
		if !strings.ContainsRune(kubeChars, char) {
			return fmt.Errorf("name contains illegal character %c", char)
		}
	}
	return nil
}
