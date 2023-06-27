package controller

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"strings"
	"time"

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
	ErrPrivateKeyNotRSA = errors.New("private key is not an RSA key")
)

func generatePrivateKeyAndCert(keySize int, validFor time.Duration, cn string) (*rsa.PrivateKey, *x509.Certificate, error) {
	return crypto.GeneratePrivateKeyAndCert(keySize, validFor, cn)
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

func writeKey(ctx context.Context, client kubernetes.Interface, key *rsa.PrivateKey, certs []*x509.Certificate, namespace, krLabel, prefix string, additionalAnnotations string, additionalLabels string, optSetters ...writeKeyOpt) (string, error) {
	var opts writeKeyOpts
	for _, o := range optSetters {
		o(&opts)
	}

	certbytes := []byte{}
	for _, cert := range certs {
		certbytes = append(certbytes, pem.EncodeToMemory(&pem.Block{Type: certUtil.CertificateBlockType, Bytes: cert.Raw})...)
	}

	labels := map[string]string{
		krLabel: "active",
	}

	annotations := map[string]string{}

	if additionalLabels != "" {
		for _, label := range removeDuplicates(strings.Split(additionalLabels, ",")) {
			key := strings.Split(label, "=")[0]
			value := strings.Split(label, "=")[1]
			if key != krLabel {
				labels[key] = value
			}
		}
	}

	if additionalAnnotations != "" {
		for _, label := range removeDuplicates(strings.Split(additionalAnnotations, ",")) {
			key := strings.Split(label, "=")[0]
			value := strings.Split(label, "=")[1]
			annotations[key] = value
		}
	}

	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:         namespace,
			GenerateName:      prefix,
			Labels:            labels,
			Annotations:       annotations,
			CreationTimestamp: opts.creationTime,
		},
		Data: map[string][]byte{
			v1.TLSPrivateKeyKey: pem.EncodeToMemory(&pem.Block{Type: keyutil.RSAPrivateKeyBlockType, Bytes: x509.MarshalPKCS1PrivateKey(key)}),
			v1.TLSCertKey:       certbytes,
		},
		Type: v1.SecretTypeTLS,
	}

	createdSecret, err := client.CoreV1().Secrets(namespace).Create(ctx, &secret, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}
	return createdSecret.Name, nil
}
