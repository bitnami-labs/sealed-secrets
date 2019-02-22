package main

import (
	"fmt"
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

func TestPrefixedNameGen(t *testing.T) {
	prefix := "prefix"
	initialCount := 0
	namegen, err := PrefixedNameGen(prefix, initialCount)
	if err != nil {
		t.Errorf("Prefixed name generator incorretly rejected a valid prefix: %v", err)
	}

	result, err := namegen()
	if err != nil {
		t.Errorf("Prefixed name gen returned err: %v", err)
	}
	expected := "prefix-0"
	if result != expected {
		t.Errorf("Prefixed name generator expected %s, returned %s", expected, result)
	}

	result, err = namegen()
	if err != nil {
		t.Errorf("Prefixed name gen returned err: %v", err)
	}
	expected = "prefix-1"
	if result != expected {
		t.Errorf("Prefixed name generator expected %s, returned %s", expected, result)
	}

	// Test for bad prefixes
	badprefix := "bad%"
	if _, err = PrefixedNameGen(badprefix, 0); err == nil {
		t.Error("Prefixed name gen incorrectly accepted an invalid prefix")
	}

	// Construct a string of 255 1's without destroying line length
	longBytes := make([]byte, 255)
	for i := range longBytes {
		longBytes[i] = '1'
	}
	longprefix := string(longBytes)
	fmt.Println(longprefix)
	if _, err = PrefixedNameGen(longprefix, 0); err == nil {
		t.Error("Prefixed name gen incorrectly accepted a prefix that is too long")
	}
}

func TestCreateKeyGenJob(t *testing.T) {
	privkey, cert, err := generatePrivateKeyAndCert(1024)
	if err != nil {
		t.Fatalf("Could not generate private key and cert for keylist secret: %v", err)
	}
	client := fake.NewSimpleClientset()
	if err := writeKeyRegistry(client, privkey, cert, "namespace", "listname"); err != nil {
		t.Fatalf("writeKeyRegistry() returned err: %v", err)
	}
	registry := NewKeyRegistry()

	client.ClearActions()

	namegen, _ := PrefixedNameGen("prefix", 0)
	keygen := createKeyGenJob(client, registry, "namespace", "listname", 1024, namegen)
	keygen()
	if !hasAction(client, "create", "secrets") {
		t.Errorf("createKeyGenJob failed to create an initial key")
	}
	if !hasAction(client, "update", "secrets") {
		t.Errorf("createKeyGenJob failed to update the keylist")
	}
}

func TestCreateBlacklister(t *testing.T) {
	privkey, cert, err := generatePrivateKeyAndCert(1024)
	if err != nil {
		t.Fatalf("Could not generate private key and cert for blacklist secret: %v", err)
	}
	client := fake.NewSimpleClientset()
	if err := writeBlacklist(client, privkey, cert, "namespace", "blacklist"); err != nil {
		t.Errorf("writeBlacklist() returned err: %v", err)
	}
	registry := NewKeyRegistry()
	blacklister := createBlacklister(client, "namespace", "blacklist", registry, func() {})

	client.ClearActions()

	// Case 1: blacklisting a non-blacklisted key
	key1 := "key1"
	registry.registerNewKey(key1, nil, nil)
	blacklisted, err := blacklister(key1)
	if err != nil {
		t.Errorf("Blacklist function failed to blacklist a key: %v", err)
	}
	clientUpdated := hasAction(client, "update", "secrets")
	_, localHasKey := registry.blacklist[key1]
	if clientUpdated != localHasKey {
		t.Errorf("Blacklist function caused client and local registry to desync. Client has key: %v, local blacklist has key: %v", clientUpdated, localHasKey)
	}
	if blacklisted != localHasKey {
		t.Errorf("Blacklist function incorrectly reported whether key was blacklisted. actual %v, reported %v", clientUpdated, blacklisted)
	}

	client.ClearActions()

	// Case 2: blacklisting a non-existing key
	missingKey := "missingKey"
	blacklisted, err = blacklister(missingKey)
	if err == nil {
		t.Error("Blacklist function didn't return an expected error")
	}
	if blacklisted {
		t.Error("Blacklist function reported that it blacklisted a non-existent key")
	}
	if hasAction(client, "update", "secrets") {
		t.Error("Blacklist function incorrectly blacklisted a non-existent key to kube blacklist")
	}
	if _, localHasKey = registry.blacklist[missingKey]; localHasKey {
		t.Error("Blacklist function incorrectly added a non-existent key to the local blacklist")
	}

	client.ClearActions()

	// Case 3: blacklisting an already blacklisted key
	blacklisted, err = blacklister(key1)
	if err == nil {
		t.Errorf("Blacklist function did not return an expected error")
	}
	if blacklisted {
		t.Errorf("Blacklist function reported that it blacklisted an already blacklisted key")
	}
	clientUpdated = hasAction(client, "update", "secrets")
	if hasAction(client, "updated", "secrets") {
		t.Error("Blacklist function attempted to update blacklist with an already blacklisted key")
	}
	if _, localHasKey = registry.blacklist[key1]; !localHasKey {
		t.Error("Local blacklist missing a blacklisted key")
	}
}
