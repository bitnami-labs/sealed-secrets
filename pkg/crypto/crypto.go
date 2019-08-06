package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/ssh"
)

const (
	sessionKeyBytes = 32
)

// ErrTooShort indicates the provided data is too short to be valid
var ErrTooShort = errors.New("SealedSecret data is too short")

// PublicKeyFingerprint returns a fingerprint for a public key.
func PublicKeyFingerprint(rp *rsa.PublicKey) (string, error) {
	sp, err := ssh.NewPublicKey(rp)
	if err != nil {
		return "", err
	}
	return ssh.FingerprintSHA256(sp), nil
}

// HybridEncrypt performs a regular AES-GCM + RSA-OAEP encryption.
// The output bytestring is:
//   RSA ciphertext length || RSA ciphertext || AES ciphertext
func HybridEncrypt(rnd io.Reader, pubKey *rsa.PublicKey, plaintext, label []byte) ([]byte, error) {
	// Generate a random symmetric key
	sessionKey := make([]byte, sessionKeyBytes)
	if _, err := io.ReadFull(rnd, sessionKey); err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(sessionKey)
	if err != nil {
		return nil, err
	}

	aed, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Encrypt symmetric key
	rsaCiphertext, err := rsa.EncryptOAEP(sha256.New(), rnd, pubKey, sessionKey, label)
	if err != nil {
		return nil, err
	}

	// First 2 bytes are RSA ciphertext length, so we can separate
	// all the pieces later.
	ciphertext := make([]byte, 2)
	binary.BigEndian.PutUint16(ciphertext, uint16(len(rsaCiphertext)))
	ciphertext = append(ciphertext, rsaCiphertext...)

	// SessionKey is only used once, so zero nonce is ok
	zeroNonce := make([]byte, aed.NonceSize())

	// Append symmetrically encrypted Secret
	ciphertext = aed.Seal(ciphertext, zeroNonce, plaintext, nil)

	return ciphertext, nil
}

// HybridDecrypt performs a regular AES-GCM + RSA-OAEP decryption.
// The private keys map has a fingerprint of each public key as the map key.
func HybridDecrypt(rnd io.Reader, privKeys map[string]*rsa.PrivateKey, ciphertext, label []byte) ([]byte, error) {
	// TODO(mkm): use the key fingerprint encoded in ciphertext (if present) instead of trying all the possible keys
	for _, privKey := range privKeys {
		if secret, err := singleDecrypt(rnd, privKey, ciphertext, label); err == nil {
			return secret, nil
		}
	}
	return nil, fmt.Errorf("no key could decrypt secret")
}

// singleDecrypt performs a regular AES-GCM + RSA-OAEP decryption
func singleDecrypt(rnd io.Reader, privKey *rsa.PrivateKey, ciphertext, label []byte) ([]byte, error) {
	if len(ciphertext) < 2 {
		return nil, ErrTooShort
	}
	rsaLen := int(binary.BigEndian.Uint16(ciphertext))
	if len(ciphertext) < rsaLen+2 {
		return nil, ErrTooShort
	}

	rsaCiphertext := ciphertext[2 : rsaLen+2]
	aesCiphertext := ciphertext[rsaLen+2:]

	sessionKey, err := rsa.DecryptOAEP(sha256.New(), rnd, privKey, rsaCiphertext, label)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(sessionKey)
	if err != nil {
		return nil, err
	}

	aed, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Key is only used once, so zero nonce is ok
	zeroNonce := make([]byte, aed.NonceSize())

	plaintext, err := aed.Open(nil, zeroNonce, aesCiphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
