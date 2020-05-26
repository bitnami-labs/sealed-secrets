package backend

import (
	"net/http"
)

type Backend interface {
	Decrypt([]byte, []byte) ([]byte, error)
	Encrypt([]byte, []byte) ([]byte, error)
	Provider(w http.ResponseWriter, r *http.Request)
}
