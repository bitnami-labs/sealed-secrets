package controller

import (
	"testing"
	"time"
)

func TestRegisterNewKey(t *testing.T) {
	const keySize = 2048
	validFor := time.Hour
	cn := "my-cn"
	kr := NewKeyRegistry(nil, "namespace", "prefix", "label", keySize)

	if kr.mostRecentKey != nil {
		t.Fatal("this test assumes a new key registry has no keys")
	}

	key1, cert1, err := generatePrivateKeyAndCert(keySize, validFor, cn)
	if err != nil {
		t.Fatal(err)
	}
	t1 := time.Now()

	key2, cert2, err := generatePrivateKeyAndCert(keySize, validFor, cn)
	if err != nil {
		t.Fatal(err)
	}
	t2 := time.Now()

	if err := kr.registerNewKey("k2", key2, cert2, t2); err != nil {
		t.Fatal(err)
	}
	if got, want := kr.mostRecentKey.private, key2; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}

	// key1 is older, so it shouldn't replace key2 as the mostRecentKey
	if err := kr.registerNewKey("k1", key1, cert1, t1); err != nil {
		t.Fatal(err)
	}
	if got, want := kr.mostRecentKey.private, key2; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}
