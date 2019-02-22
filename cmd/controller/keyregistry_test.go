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

func TestReadBlacklist(t *testing.T) {
	client := fake.NewSimpleClientset()
	if err := writeTestBlacklistToClient(client); err != nil {
		t.Fatalf("Unable to generate private key and cert for test")
	}
	blacklist, err := readBlacklist(client, "namespace", "blacklist")
	if err != nil {
		t.Fatalf("readBlacklist() returned err: %v", err)
	}
	if !hasAction(client, "get", "secrets") {
		t.Fatal("readBlacklist() failed to read the blacklist from the client")
	}
	if len(blacklist) > 0 {
		t.Fatal("readBlacklist returned a non-empty blacklist from an empty client blacklist")
	}

	client.ClearActions()

	if err = updateBlacklist(client, "namespace", "blacklist", "keyname"); err != nil {
		t.Fatalf("Error adding test keyname to client blacklist: %v", err)
	}
	blacklist, err = readBlacklist(client, "namespace", "blacklist")
	if err != nil {
		t.Fatalf("readBlacklist() returned err: %v", err)
	}
	if !hasAction(client, "get", "secrets") {
		t.Fatal("readBlacklist() failed to read the blacklist from the client")
	}
	if _, hasKeyname := blacklist["keyname"]; !hasKeyname {
		t.Fatal("readBlacklist() failed to copy a keyname from the client blacklist")
	}
}

func TestWriteBlacklist(t *testing.T) {
	client := fake.NewSimpleClientset()
	privkey, cert, err := generatePrivateKeyAndCert(1024)
	if err != nil {
		t.Fatalf("Unable to generate private key and cert: %v", err)
	}
	if err := writeBlacklist(client, privkey, cert, "namespace", "blacklist"); err != nil {
		t.Fatalf("writeBlacklist() returned err: %v", err)
	}
	if !hasAction(client, "create", "secrets") {
		t.Errorf("writeBlacklist() failed to create a secret")
	}
}

func TestUpdateBlacklist(t *testing.T) {
	client := fake.NewSimpleClientset()
	if err := writeTestBlacklistToClient(client); err != nil {
		t.Fatalf("Error adding test keyname to client blacklist: %v", err)
	}
	if err := updateBlacklist(client, "namespace", "blacklist", "keyname"); err != nil {
		t.Fatalf("updateBlacklist() returned err: %v", err)
	}
	if !hasAction(client, "get", "secrets") {
		t.Fatal("updateBlacklist() failed to get the current blacklist from the client")
	}
	if !hasAction(client, "update", "secrets") {
		t.Fatal("updateBlacklist() failed to update the clients blacklist")
	}
}

func TestRegisterNewKey(t *testing.T) {
	registry := NewKeyRegistry()
	privkey, cert, err := generatePrivateKeyAndCert(1024)
	if err != nil {
		t.Fatalf("Error generating private key and cert for test: %v", err)
	}
	registry.registerNewKey("keyname", privkey, cert)
	if key, ok := registry.keys["keyname"]; !ok || key != privkey {
		t.Error("Registry failed to store private key")
	}
	if c, ok := registry.certs["keyname"]; !ok || c != cert {
		t.Error("Registry failed to store certificate")
	}
	if _, ok := registry.blacklist["keyname"]; ok {
		t.Error("Registry blacklisted a new key")
	}
	if "keyname" != registry.currentKeyName {
		t.Error("Registry did not list a new key as the latest key")
	}

	registry.registerNewKey("newkey", nil, nil)
	if "newkey" != registry.currentKeyName {
		t.Error("Registry did not list a new key as the latest key")
	}
}

func TestRegistryBlacklist(t *testing.T) {
	registry := NewKeyRegistry()
	registry.blacklistKey("keyname")
	if _, ok := registry.blacklist["keyname"]; !ok {
		t.Fatalf("registry failed to blacklist a key")
	}
}

func TestGetPrivateKeyAndCert(t *testing.T) {
	registry := NewKeyRegistry()
	keynames := []string{"key1", "key2"}
	for _, name := range keynames {
		privkey, cert, err := generatePrivateKeyAndCert(1024)
		if err != nil {
			t.Fatalf("Error generating private key and cert for test: %v", err)
		}
		registry.registerNewKey(name, privkey, cert)
	}
	// Case 1, keyname exists
	privkey1, err := registry.getPrivateKey("key1")
	if err != nil {
		t.Fatalf("getPrivateKey() returned err: %v", err)
	}
	privkey2, err := registry.getPrivateKey("key2")
	if err != nil {
		t.Fatalf("getPrivateKey() returned err: %v", err)
	}
	if privkey1 == privkey2 {
		t.Error("getPrivateKey() failed to retrieve unique keys for unique key names")
	}

	cert1, err := registry.getCert("key1")
	if err != nil {
		t.Fatalf("getCert() returned err: %v", err)
	}
	cert2, err := registry.getCert("key2")
	if err != nil {
		t.Fatalf("getCert() returned err: %v", err)
	}
	if cert1 == cert2 {
		t.Error("getCert() failed to retrieve unique keys for unique key names")
	}

	// Case 2: keyname does not exist
	if _, err := registry.getPrivateKey("missingKey"); err == nil {
		t.Error("getPrivateKey() failed to return an expected error")
	}
	if _, err := registry.getCert("missingKey"); err == nil {
		t.Error("getCert() failed to return an expected error")
	}

	// Case 3: keyname is blacklisted
	registry.blacklistKey("key1")
	if _, err := registry.getPrivateKey("key1"); err == nil {
		t.Error("getPrivateKey() failed to return an expected error")
	}
	if _, err := registry.getCert("key1"); err == nil {
		t.Error("getCert() failed to return an expected error")
	}
}
