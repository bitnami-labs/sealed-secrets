package main

import (
	"reflect"
	"testing"

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
		t.Fatalf("initKey returned err: %v", err)
	}

	if !hasAction(client, "create", "secrets") {
		t.Errorf("initKey() failed to create secret")
	}

	client.ClearActions()

	registry2, err := initKeyRegistry(client, rand, "testns", "testkeylist")
	if err != nil {
		t.Fatalf("initKey returned err: %v", err)
	}

	if !reflect.DeepEqual(registry, registry2) {
		t.Errorf("Failed to find same key")
	}
}
