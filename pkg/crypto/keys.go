package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"math/big"
	"time"
)

// GeneratePrivateKeyAndCert generates a keypair and signed certificate.
func GeneratePrivateKeyAndCert(keySize int, validFor time.Duration, cn string) (*rsa.PrivateKey, *x509.Certificate, error) {
	r := rand.Reader
	privKey, err := rsa.GenerateKey(r, keySize)
	if err != nil {
		return nil, nil, err
	}
	cert, err := SignKey(r, privKey, validFor, cn)
	if err != nil {
		return nil, nil, err
	}
	return privKey, cert, nil
}

// SignKey returns a signed certificate.
func SignKey(r io.Reader, key *rsa.PrivateKey, validFor time.Duration, cn string) (*x509.Certificate, error) {
	// TODO: use certificates API to get this signed by the cluster root CA
	// See https://kubernetes.io/docs/tasks/tls/managing-tls-in-a-cluster/

	return SignKeyWithNotBefore(r, key, time.Now(), validFor, cn)
}

// SignKeyWithNotBefore returns a signed certificate with custom notBefore.
func SignKeyWithNotBefore(r io.Reader, key *rsa.PrivateKey, notBefore time.Time, validFor time.Duration, cn string) (*x509.Certificate, error) {
	// TODO: use certificates API to get this signed by the cluster root CA
	// See https://kubernetes.io/docs/tasks/tls/managing-tls-in-a-cluster/

	serialNo, err := rand.Int(r, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}

	cert := x509.Certificate{
		SerialNumber: serialNo,
		KeyUsage:     x509.KeyUsageEncipherOnly,
		NotBefore:    notBefore.UTC(),
		NotAfter:     notBefore.Add(validFor).UTC(),
		Issuer: pkix.Name{
			CommonName: cn,
		},
		Subject: pkix.Name{
			CommonName: cn,
		},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	data, err := x509.CreateCertificate(r, &cert, &cert, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}

	return x509.ParseCertificate(data)
}
