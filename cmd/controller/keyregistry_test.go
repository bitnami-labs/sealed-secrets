package main

import (
	"errors"
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

func writeTestKeylistSecret(client *fake.Clientset) error {
	privkey, cert, err := generatePrivateKeyAndCert(1024)
	if err != nil {
		return err
	}
	if err := writeKeyRegistry(client, privkey, cert, "namespace", "listname"); err != nil {
		return err
	}
	if !hasAction(client, "create", "secrets") {
		return errors.New("writeKeyRegistry() did not create a secret")
	}
	client.ClearActions()
	return nil
}

func TestReadKeyRegistry(t *testing.T) {
	client := fake.NewSimpleClientset()
	if err := writeTestKeylistSecret(client); err != nil {
		t.Fatalf("Error writing keylist for test: %v", err)
	}
	keylist, err := readKeyRegistry(client, "namespace", "listname")
	if err != nil {
		t.Fatalf("readKeyRegistry() returned err: %v", err)
	}
	if !hasAction(client, "get", "secrets") {
		t.Fatalf("readKeyRegistry() did not read a list from the client")
	}
	if len(keylist) != 0 {
		t.Fatalf("readKeyRegistry() returned a non-empty list from an empty client list")
	}

	client.ClearActions()

	// Add a value to read
	if err := updateKeyRegistry(client, "namespace", "listname", "keyname"); err != nil {
		t.Fatalf("Error adding new keyname to client for test: %v", err)
	}
	client.ClearActions()
	keylist, err = readKeyRegistry(client, "namespace", "listname")
	if err != nil {
		t.Fatalf("readKeyRegistry() returned err: %v", err)
	}
	if !hasAction(client, "get", "secrets") {
		t.Fatalf("readKeyRegistry() did not read a list from the client")
	}
	if _, hasKey := keylist["keyname"]; !hasKey {
		t.Fatalf("readKeyRegistry() failed to read keynames in the keylist")
	}
}

func TestWriteKeyRegistry(t *testing.T) {
	client := fake.NewSimpleClientset()
	privkey, cert, err := generatePrivateKeyAndCert(1024)
	if err != nil {
		t.Fatalf("Unable to generate private key and cert: %v", err)
	}
	if err := writeKeyRegistry(client, privkey, cert, "namespace", "listname"); err != nil {
		t.Fatalf("writeKeylist() returned err: %v", err)
	}
	if !hasAction(client, "create", "secrets") {
		t.Errorf("writeKeyRegistry() failed to create a secret")
	}
}

func TestUpdateKeyRegistry(t *testing.T) {
	client := fake.NewSimpleClientset()
	if err := writeTestKeylistSecret(client); err != nil {
		t.Fatalf("Error writing keylist for test: %v", err)
	}
	if err := updateKeyRegistry(client, "namespace", "listname", "keyname"); err != nil {
		t.Fatalf("updateKeyRegistry() returned err: %v", err)
	}
	if !hasAction(client, "get", "secrets") {
		t.Fatalf("updateKeyRegistry() failed to read the current keylist from the client")
	}
	if !hasAction(client, "update", "secrets") {
		t.Fatalf("updateKeyRegistry() failed to update the clients keylist")
	}
}

func writeTestBlacklistToClient(client *fake.Clientset) error {
	privkey, cert, err := generatePrivateKeyAndCert(1024)
	if err != nil {
		return err
	}
	if err := writeKeyRegistry(client, privkey, cert, "namespace", "blacklist"); err != nil {
		return err
	}
	if !hasAction(client, "create", "secrets") {
		return errors.New("writeKeyRegistry() did not create a secret")
	}
	client.ClearActions()
	return nil
}
