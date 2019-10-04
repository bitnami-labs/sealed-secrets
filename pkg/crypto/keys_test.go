package crypto

import (
	"crypto/rsa"
	"io"
	mathrand "math/rand"
	"reflect"
	"testing"
	"time"
)

// This is omg-not safe for real crypto use!
func testRand() io.Reader {
	return mathrand.New(mathrand.NewSource(42))
}

func TestSignKey(t *testing.T) {
	rand := testRand()

	key, err := rsa.GenerateKey(rand, 512)
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}

	cert, err := SignKey(rand, key, time.Hour, "mycn")
	if err != nil {
		t.Errorf("signKey() returned error: %v", err)
	}

	if !reflect.DeepEqual(cert.PublicKey, &key.PublicKey) {
		t.Errorf("cert pubkey != original pubkey")
	}
}
