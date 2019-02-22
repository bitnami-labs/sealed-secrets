package main

import (
	"reflect"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func findAction(fake *fake.Clientset, verb, resource string) ktesting.Action {
	for _, a := range fake.Actions() {
		if a.Matches(verb, resource) {
			return a
		}
	}
	return nil
}

func hasAction(fake *fake.Clientset, verb, resource string) bool {
	return findAction(fake, verb, resource) != nil
}

func TestInitKeyRegistry(t *testing.T) {
	rand := testRand()
	client := fake.NewSimpleClientset()

	registry, err := initKeyRegistry(client, rand, "testns", "testkeylist")
	if err != nil {
		t.Fatalf("initKeyRegistry() returned err: %v", err)
	}

	if !hasAction(client, "create", "secrets") {
		t.Errorf("initKeyRegistry() failed to create keylist secret")
	}

	// Add a key to the controller for second test
	nameGen := func() (string, error) { return "name", nil }
	createKeyGenJob(client, registry, "testns", "testkeylist", 1024, nameGen)()
	if registry.latestKeyName() != "name" {
		t.Fatalf("Error adding key to registry")
	}
	client.ClearActions()

	registry2, err := initKeyRegistry(client, rand, "testns", "testkeylist")
	if err != nil {
		t.Fatalf("initKeyRegistry() returned err: %v", err)
	}
	if !hasAction(client, "get", "secrets") {
		t.Errorf("initKeyRegistry() failed to read existing keylist")
	}
	// Checks the second init picked up the key created after the first init
	if !reflect.DeepEqual(registry, registry2) {
		t.Errorf("Failed to find same keylist")
	}
}

func TestInitKeyBlacklist(t *testing.T) {
	rand := testRand()
	client := fake.NewSimpleClientset()

	registry := NewKeyRegistry()
	_, err := initBlacklist(client, rand, registry, "testns", "testblacklistname", func() {})
	if err != nil {
		t.Fatalf("initBlacklist() returned err: %v", err)
	}
	if !hasAction(client, "create", "secrets") {
		t.Errorf("initBlacklist() failed to create blacklist secret")
	}
	client.ClearActions()

	_, err = initBlacklist(client, rand, registry, "testns", "testblacklistname", func() {})
	if err != nil {
		t.Fatalf("initBlacklist() returned err: %v", err)
	}
	// Check a blacklist is retrieved rather than created
	if !hasAction(client, "get", "secrets") {
		t.Errorf("initBlacklist() failed to read existing blacklist")
	}
}

func TestInitKeyRotation(t *testing.T) {
	rand := testRand()
	client := fake.NewSimpleClientset()
	registry, err := initKeyRegistry(client, rand, "namespace", "listname")
	if err != nil {
		t.Fatalf("initKeyRegistry() returned err: %v", err)
	}

	keyGenTrigger, err := initKeyRotation(client, registry, "namespace", "listname", 1024, time.Minute)
	if err != nil {
		t.Fatalf("initKeyRotation() returned err: %v", err)
	}
	if !hasAction(client, "create", "secrets") {
		t.Errorf("initKeyRotation() failed to generate an initial key")
	}

	client.ClearActions()

	keyGenTrigger()
	time.Sleep(50 * time.Millisecond) // TODO: investigate if testing the trigger function can be improved
	if !hasAction(client, "create", "secrets") {
		t.Errorf("trigger function failed to activate early key generation")
	}
}
