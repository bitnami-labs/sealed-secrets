package backend

import (
	"net/http"
)

type Backend interface {
	// Encrypt used for backend encryption
	Encrypt(plaintext []byte, label []byte) (ciphertext []byte, err error)
	// Encrypt used for backend decryption
	Decrypt(ciphertext []byte, label []byte) (plaintext []byte, err error)
	// ProviderHandler used to return provider backend information needed to encrypt
	// secret using client (kubeseal)
	ProviderHandler(w http.ResponseWriter, r *http.Request)
}
