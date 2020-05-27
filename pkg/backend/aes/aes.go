package aes

import (
	"crypto/rand"
	"crypto/rsa"
	"github.com/bitnami-labs/sealed-secrets/pkg/crypto"

	"crypto/x509"
	"encoding/pem"
	certUtil "k8s.io/client-go/util/cert"
	"log"
	"net/http"
)

type AES256 struct {
	keyRegistry   *KeyRegistry
	localPubKey   *rsa.PublicKey
	localPrivKeys map[string]*rsa.PrivateKey
}

func NewAES256(kr *KeyRegistry, pubKey *rsa.PublicKey, privKeys map[string]*rsa.PrivateKey) *AES256 {
	return &AES256{
		keyRegistry:   kr,
		localPubKey:   pubKey,
		localPrivKeys: privKeys,
	}
}

func (b *AES256) Encrypt(plaintext, label []byte) ([]byte, error) {

	var publicKey *rsa.PublicKey

	if b.localPubKey != nil {
		publicKey = b.localPubKey
	} else {
		latestPrivKey := b.keyRegistry.LatestPrivateKey()
		publicKey = &latestPrivKey.PublicKey
	}

	return crypto.HybridEncrypt(rand.Reader, publicKey, plaintext, label)
}

func (b *AES256) Decrypt(ciphertext, label []byte) ([]byte, error) {

	privateKeys := map[string]*rsa.PrivateKey{}

	if len(b.localPrivKeys) != 0 {
		privateKeys = b.localPrivKeys
	} else {
		for k, v := range b.keyRegistry.GetKeys() {
			privateKeys[k] = v.private
		}
	}

	return crypto.HybridDecrypt(rand.Reader, privateKeys, ciphertext, label)

}

func (b *AES256) ProviderHandler(w http.ResponseWriter, r *http.Request) {
	cert, err := b.keyRegistry.GetCert()
	if err != nil {
		log.Printf("cannot get certificates: %v", err)
		http.Error(w, "cannot get certificate", http.StatusInternalServerError)
		return
	}

	certs := []*x509.Certificate{cert}

	w.Header().Set("Content-Type", "application/x-pem-file")
	for _, cert := range certs {
		w.Write(pem.EncodeToMemory(&pem.Block{Type: certUtil.CertificateBlockType, Bytes: cert.Raw}))
	}

}
