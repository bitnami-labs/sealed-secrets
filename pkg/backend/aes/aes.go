package aes

import (
	"fmt"
	"strings"
	"crypto/rand"
	"crypto/rsa"
	"github.com/bitnami-labs/sealed-secrets/pkg/crypto"

	"crypto/x509"
	"encoding/pem"
	certUtil "k8s.io/client-go/util/cert"
	"log"
	"net/http"

	"io"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealed-secrets/v1alpha1"
	"github.com/bitnami-labs/sealed-secrets/pkg/utils"
)

var (
	// Selector used to find existing public/private key pairs on startup
	keySelector = fields.OneTermEqualSelector(SealedSecretsKeyLabel, "active")
)

const (
	kubeChars     = "abcdefghijklmnopqrstuvwxyz0123456789-" // Acceptable characters in k8s resource name
	maxNameLength = 245                                     // Max resource name length is 253, leave some room for a suffix
)

type AES256 struct {
	keyRegistry *KeyRegistry
	pubKey      *rsa.PublicKey
	privKeys    map[string]*rsa.PrivateKey
}

func NewAES256WithKey(pubKey *rsa.PublicKey, privKeys map[string]*rsa.PrivateKey) *AES256 {
	return &AES256{
		pubKey:   pubKey,
		privKeys: privKeys,
	}
}

func NewAES256WithKeyRegistry(
	clientset kubernetes.Interface,
	namespace string,
	keyPrefix string,
	keySize int,
	validFor time.Duration,
	myCN string,
	keyRenewPeriod time.Duration,
	keyCutoffTime string) (*AES256, error) {

	prefix, err := initKeyPrefix(keyPrefix)
	if err != nil {
		return nil, err
	}

	keyRegistry, err := initKeyRegistry(clientset, rand.Reader, namespace, prefix, SealedSecretsKeyLabel, keySize, validFor, myCN)
	if err != nil {
		return nil, err
	}

	var ct time.Time
	if keyCutoffTime != "" {
		var err error
		ct, err = time.Parse(time.RFC1123Z, keyCutoffTime)
		if err != nil {
			return nil, err
		}
	}

	trigger, err := initKeyRenewal(keyRegistry, keyRenewPeriod, ct)
	if err != nil {
		return nil, err
	}

	initKeyGenSignalListener(trigger)

	return &AES256{
		keyRegistry: keyRegistry,
	}, nil
}

func (b *AES256) Encrypt(plaintext, label []byte) ([]byte, error) {

	var publicKey *rsa.PublicKey

	if b.pubKey != nil {
		publicKey = b.pubKey
	} else {
		latestPrivKey := b.keyRegistry.LatestPrivateKey()
		publicKey = &latestPrivKey.PublicKey
	}

	return crypto.HybridEncrypt(rand.Reader, publicKey, plaintext, label)
}

func (b *AES256) Decrypt(ciphertext, label []byte) ([]byte, error) {

	privateKeys := map[string]*rsa.PrivateKey{}

	if len(b.privKeys) != 0 {
		privateKeys = b.privKeys
	} else {
		for k, v := range b.keyRegistry.keys {
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

func initKeyPrefix(keyPrefix string) (string, error) {
	prefix, err := validateKeyPrefix(keyPrefix)
	if err != nil {
		return "", err
	}
	return prefix, err
}

func initKeyRegistry(client kubernetes.Interface, r io.Reader, namespace, prefix, label string, keysize int, validFor time.Duration, myCN string) (*KeyRegistry, error) {
	log.Printf("Searching for existing private keys")
	secretList, err := client.CoreV1().Secrets(namespace).List(metav1.ListOptions{
		LabelSelector: keySelector.String(),
	})
	if err != nil {
		return nil, err
	}
	items := secretList.Items

	s, err := client.CoreV1().Secrets(namespace).Get(prefix, metav1.GetOptions{})
	if !errors.IsNotFound(err) {
		if err != nil {
			return nil, err
		}
		items = append(items, *s)
		// TODO(mkm): add the label to the legacy secret to simplify discovery and backups.
	}

	keyRegistry := NewKeyRegistry(client, namespace, prefix, label, keysize, validFor, myCN)
	sort.Sort(ssv1alpha1.ByCreationTimestamp(items))
	for _, secret := range items {
		key, certs, err := readKey(secret)
		if err != nil {
			log.Printf("Error reading key %s: %v", secret.Name, err)
		}
		ct := secret.CreationTimestamp
		if err := keyRegistry.registerNewKey(secret.Name, key, certs[0], ct.Time); err != nil {
			return nil, err
		}
		log.Printf("----- %s", secret.Name)
	}
	return keyRegistry, nil
}

// Initialises the first key and starts the rotation job. returns an early trigger function.
// A period of 0 disables automatic rotation, but manual rotation (e.g. triggered by SIGUSR1)
// is still honoured.
func initKeyRenewal(registry *KeyRegistry, period time.Duration, cutoffTime time.Time) (func(), error) {
	// Create a new key if it's the first key,
	// or if it's older than cutoff time.
	if len(registry.keys) == 0 || registry.mostRecentKey.creationTime.Before(cutoffTime) {
		if _, err := registry.generateKey(); err != nil {
			return nil, err
		}
	}

	// wrapper function to log error thrown by generateKey function
	keyGenFunc := func() {
		if _, err := registry.generateKey(); err != nil {
			log.Printf("Failed to generate new key : %v\n", err)
		}
	}
	if period == 0 {
		return keyGenFunc, nil
	}

	// If key rotation is enabled, we'll rotate the key when the most recent
	// key becomes stale (older than period).
	mostRecentKeyAge := time.Since(registry.mostRecentKey.creationTime)
	initialDelay := period - mostRecentKeyAge
	if initialDelay < 0 {
		initialDelay = 0
	}
	return utils.ScheduleJobWithTrigger(initialDelay, period, keyGenFunc), nil
}

func initKeyGenSignalListener(trigger func()) {
	sigChannel := make(chan os.Signal)
	signal.Notify(sigChannel, syscall.SIGUSR1)
	go func() {
		for {
			<-sigChannel
			trigger()
		}
	}()
}

func validateKeyPrefix(name string) (string, error) {
	if len(name) > maxNameLength {
		return "", fmt.Errorf("name is too long, must be shorter than %d, got %d", maxNameLength, len(name))
	}
	for _, char := range name {
		if !strings.ContainsRune(kubeChars, char) {
			return "", fmt.Errorf("name contains illegal character %c", char)
		}
	}
	return name, nil
}