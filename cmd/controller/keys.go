package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"

	"github.com/bitnami-labs/sealed-secrets/pkg/crypto"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	certUtil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
)

// SealedSecretsKeyLabel is that label used to locate active key pairs used to decrypt sealed secrets.
const SealedSecretsKeyLabel = "sealedsecrets.bitnami.com/sealed-secrets-key"

var (
	// ErrPrivateKeyNotRSA is returned when the private key is not a valid RSA key.
	ErrPrivateKeyNotRSA = errors.New("Private key is not an RSA key")
)

func generatePrivateKeyAndCert(keySize int) (*rsa.PrivateKey, *x509.Certificate, error) {
	return crypto.GeneratePrivateKeyAndCert(keySize, *validFor, *myCN)
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

type writeKeyOpt func(*writeKeyOpts)
type writeKeyOpts struct{ creationTime metav1.Time }

func writeKeyWithCreationTime(t metav1.Time) writeKeyOpt {
	return func(opts *writeKeyOpts) { opts.creationTime = t }
}

func writeKey(client kubernetes.Interface, key *rsa.PrivateKey, certs []*x509.Certificate, namespace, label, prefix string, optSetters ...writeKeyOpt) (string, error) {
	var opts writeKeyOpts
	for _, o := range optSetters {
		o(&opts)
	}

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
			CreationTimestamp: opts.creationTime,
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
