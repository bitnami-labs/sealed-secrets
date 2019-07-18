package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	certUtil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
)

const SealedSecretsKeyLabel = "sealedsecrets.bitnami.com/sealed-secrets-key"

var (
	ErrPrivateKeyNotRSA = errors.New("Private key is not an rsa key")
)

func generatePrivateKeyAndCert(keySize int) (*rsa.PrivateKey, *x509.Certificate, error) {
	r := rand.Reader
	privKey, err := rsa.GenerateKey(r, keySize)
	if err != nil {
		return nil, nil, err
	}
	cert, err := signKey(r, privKey)
	if err != nil {
		return nil, nil, err
	}
	return privKey, cert, nil
}

func readKey(secret v1.Secret) (*rsa.PrivateKey, []*x509.Certificate, error) {
	key, err := keyutil.ParsePrivateKeyPEM(secret.Data[v1.TLSPrivateKeyKey])
	if err != nil {
		return nil, nil, err
	}
	switch rsaKey := key.(type) {
	case *rsa.PrivateKey:
		certs, err := certUtil.ParseCertsPEM(secret.Data[v1.TLSCertKey])
		if err != nil {
			return nil, nil, err
		}
		return rsaKey, certs, nil
	default:
		return nil, nil, ErrPrivateKeyNotRSA
	}
}

func writeKey(client kubernetes.Interface, key *rsa.PrivateKey, certs []*x509.Certificate, namespace, label, prefix string) (string, error) {
	certbytes := []byte{}
	for _, cert := range certs {
		certbytes = append(certbytes, pem.EncodeToMemory(&pem.Block{Type: certUtil.CertificateBlockType, Bytes: cert.Raw})...)
	}
	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    namespace,
			GenerateName: prefix,
			Labels: map[string]string{
				label: "active",
			},
		},
		Data: map[string][]byte{
			v1.TLSPrivateKeyKey: pem.EncodeToMemory(&pem.Block{Type: keyutil.RSAPrivateKeyBlockType, Bytes: x509.MarshalPKCS1PrivateKey(key)}),
			v1.TLSCertKey:       certbytes,
		},
		Type: v1.SecretTypeTLS,
	}

	createdSecret, err := client.CoreV1().Secrets(namespace).Create(&secret)
	if err != nil {
		return "", err
	}
	return createdSecret.Name, nil
}

func signKey(r io.Reader, key *rsa.PrivateKey) (*x509.Certificate, error) {
	// TODO: use certificates API to get this signed by the cluster root CA
	// See https://kubernetes.io/docs/tasks/tls/managing-tls-in-a-cluster/

	notBefore := time.Now()

	serialNo, err := rand.Int(r, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}

	cert := x509.Certificate{
		SerialNumber: serialNo,
		KeyUsage:     x509.KeyUsageEncipherOnly,
		NotBefore:    notBefore.UTC(),
		NotAfter:     notBefore.Add(*validFor).UTC(),
		Subject: pkix.Name{
			CommonName: *myCN,
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
