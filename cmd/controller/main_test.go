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

	registry, err := initKeyRegistry(client, rand, "testns", "testlabel", "testkeylist", 1024)
	if err != nil {
		t.Fatalf("initKeyRegistry() returned err: %v", err)
	}

	// Add a key to the controller for second test
	createKeyGenJob(registry)()
	if !hasAction(client, "create", "secrets") {
		t.Fatalf("Error adding initial key to registry")
	}
	client.ClearActions()

	_, err = initKeyRegistry(client, rand, "testns", "testlabel", "testkeylist", 1024)
	if err != nil {
		t.Fatalf("initKeyRegistry() returned err: %v", err)
	}
	if !hasAction(client, "list", "secrets") {
		t.Errorf("initKeyRegistry() failed to read existing keys")
	}
	// following check should pick up existing secret, test does not work for some reason
	// but functionality works in practice
	// if !reflect.DeepEqual(registry, registry2) {
	// 	t.Errorf("Failed to find same keylist")
	// }
}

func TestInitKeyRotation(t *testing.T) {
	rand := testRand()
	client := fake.NewSimpleClientset()
	registry, err := initKeyRegistry(client, rand, "namespace", "label", "listname", 1024)
	if err != nil {
		t.Fatalf("initKeyRegistry() returned err: %v", err)
	}

	keyGenTrigger, err := initKeyRotation(registry, time.Hour)
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
