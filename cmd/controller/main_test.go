package main

import (
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

	registry, err := initKeyRegistry(client, rand, "namespace", "prefix", "label", 1024)
	if err != nil {
		t.Fatalf("initKeyRegistry() returned err: %v", err)
	}

	// Add a key to the controller for second test
	registry.generateKey()
	if !hasAction(client, "create", "secrets") {
		t.Fatalf("Error adding initial key to registry")
	}
	client.ClearActions()

	// Due to limitations of the fake client, we cannot test whether initKeyRegistry is able
	// to pick up existing keys
	_, err = initKeyRegistry(client, rand, "namespace", "prefix", "label", 1024)
	if err != nil {
		t.Fatalf("initKeyRegistry() returned err: %v", err)
	}
	if !hasAction(client, "list", "secrets") {
		t.Errorf("initKeyRegistry() failed to read existing keys")
	}
}

func TestInitKeyRotation(t *testing.T) {
	rand := testRand()
	client := fake.NewSimpleClientset()
	registry, err := initKeyRegistry(client, rand, "namespace", "prefix", "label", 1024)
	if err != nil {
		t.Fatalf("initKeyRegistry() returned err: %v", err)
	}

	keyGenTrigger, err := initKeyRotation(registry, 0)
	if err != nil {
		t.Fatalf("initKeyRotation() returned err: %v", err)
	}
	if !hasAction(client, "create", "secrets") {
		t.Errorf("initKeyRotation() failed to generate an initial key")
	}

	client.ClearActions()

	// Test the trigger function
	// Activates trigger and polls client every 50 ms up to 10s for the appropriate action
	keyGenTrigger()
	maxWait := 10 * time.Second
	endTime := time.Now().Add(maxWait)
	successful := false
	for time.Now().Before(endTime) {
		time.Sleep(50 * time.Millisecond)
		if hasAction(client, "create", "secrets") {
			successful = true
			break
		}
	}
	if !successful {
		t.Errorf("trigger function failed to activate early key generation")
	}
}

func TestInitKeyRotationTick(t *testing.T) {
	rand := testRand()
	client := fake.NewSimpleClientset()
	registry, err := initKeyRegistry(client, rand, "namespace", "prefix", "label", 1024)
	if err != nil {
		t.Fatalf("initKeyRegistry() returned err: %v", err)
	}

	_, err = initKeyRotation(registry, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("initKeyRotation() returned err: %v", err)
	}
	if !hasAction(client, "create", "secrets") {
		t.Errorf("initKeyRotation() failed to generate an initial key")
	}

	client.ClearActions()

	maxWait := 10 * time.Second
	endTime := time.Now().Add(maxWait)
	successful := false
	for time.Now().Before(endTime) {
		time.Sleep(50 * time.Millisecond)
		if hasAction(client, "create", "secrets") {
			successful = true
			break
		}
	}
	if !successful {
		t.Errorf("trigger function failed to activate early key generation")
	}
}
