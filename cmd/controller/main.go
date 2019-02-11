package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	goflag "flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	flag "github.com/spf13/pflag"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	certUtil "k8s.io/client-go/util/cert"

	sealedsecrets "github.com/bitnami-labs/sealed-secrets/pkg/client/clientset/versioned"
	ssinformers "github.com/bitnami-labs/sealed-secrets/pkg/client/informers/externalversions"
)

var (
	keyListName     = flag.String("key-list", "sealed-secrets-keys", "Name of Secret containing names of public/private keys.")
	keySize         = flag.Int("key-size", 4096, "Size of encryption key.")
	validFor        = flag.Duration("key-ttl", 10*365*24*time.Hour, "Duration that certificate is valid for.")
	myCN            = flag.String("my-cn", "", "CN to use in generated certificate.")
	printVersion    = flag.Bool("version", false, "Print version information and exit")
	keyRotatePeriod = flag.Int("key-rotate", 14, "Key rotation period in days")

	// VERSION set from Makefile
	VERSION = "UNKNOWN"
)

func init() {
	// Standard goflags (glog in particular)
	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	if f := flag.CommandLine.Lookup("logtostderr"); f != nil {
		f.DefValue = "true"
		f.Value.Set(f.DefValue)
	}
}

type controller struct {
	clientset kubernetes.Interface
}

func readKey(client kubernetes.Interface, namespace, keyName string) (*rsa.PrivateKey, []*x509.Certificate, error) {
	secret, err := client.Core().Secrets(namespace).Get(keyName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}

	key, err := certUtil.ParsePrivateKeyPEM(secret.Data[v1.TLSPrivateKeyKey])
	if err != nil {
		return nil, nil, err
	}

	certs, err := certUtil.ParseCertsPEM(secret.Data[v1.TLSCertKey])
	if err != nil {
		return nil, nil, err
	}

	return key.(*rsa.PrivateKey), certs, nil
}

func writeKey(client kubernetes.Interface, key *rsa.PrivateKey, certs []*x509.Certificate, namespace, keyName string) error {
	certbytes := []byte{}
	for _, cert := range certs {
		certbytes = append(certbytes, certUtil.EncodeCertPEM(cert)...)
	}

	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      keyName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			v1.TLSPrivateKeyKey: certUtil.EncodePrivateKeyPEM(key),
			v1.TLSCertKey:       certbytes,
		},
		Type: v1.SecretTypeTLS,
	}

	_, err := client.Core().Secrets(namespace).Create(&secret)
	return err
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

func newKey(r io.Reader) (*rsa.PrivateKey, *x509.Certificate, error) {
	privKey, err := rsa.GenerateKey(r, *keySize)
	if err != nil {
		return nil, nil, err
	}

	cert, err := signKey(r, privKey)
	if err != nil {
		return nil, nil, err
	}
	return privKey, cert, nil
}

func readKeyNameList(client kubernetes.Interface, namespace, listName string) (map[string]struct{}, error) {
	secret, err := client.Core().Secrets(namespace).Get(listName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	keyNames := map[string]struct{}{}
	for keyName, _ := range secret.Data {
		if (keyName == v1.TLSPrivateKeyKey) || (keyName == v1.TLSCertKey) {
			keyNames[keyName] = struct{}{}
		}
	}
	return keyNames, nil
}

func updateKeyNameList(client kubernetes.Interface, namespace, listName, newKeyName string) error {
	secret, err := client.Core().Secrets(namespace).Get(listName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	secret.Data[newKeyName] = []byte{}
	if _, err := client.Core().Secrets(namespace).Update(secret); err != nil {
		return err
	}
	return nil
}

func writeKeyNameList(client kubernetes.Interface, key *rsa.PrivateKey, cert *x509.Certificate, namespace, listName string) error {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      listName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			v1.TLSPrivateKeyKey: certUtil.EncodePrivateKeyPEM(key),
			v1.TLSCertKey:       certUtil.EncodeCertPEM(cert),
		},
		Type: v1.SecretTypeTLS,
	}
	if _, err := client.Core().Secrets(namespace).Create(secret); err != nil {
		return err
	}
	return nil
}

func initKeyNameList(client kubernetes.Interface, r io.Reader, namespace, listName string) (*KeyRegistry, error) {
	list, err := readKeyNameList(client, namespace, listName)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Printf("Keyname list %s/%s not found, generating new keyname list", namespace, listName)

			privKey, cert, err := newKey(r)
			if err != nil {
				return nil, err
			}

			if err = writeKeyNameList(client, privKey, cert, namespace, listName); err != nil {
				return nil, err
			}
			log.Printf("New keyname list generated")
			return NewKeyRegistry(), nil
		} else {
			return nil, err
		}
	} else {
		keyRegistry := NewKeyRegistry()
		// for each key, get the stored private key
		for keyName := range list {
			key, certs, err := readKey(client, namespace, keyName)
			if err != nil {
				return nil, err
			}
			keyRegistry.registerNewKey(keyName, key, certs[0])
		}
		return keyRegistry, nil
	}
}

func myNamespace() string {
	if ns := os.Getenv("POD_NAMESPACE"); ns != "" {
		return ns
	}

	// Fall back to the namespace associated with the service account token, if available
	if data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
			return ns
		}
	}

	return metav1.NamespaceDefault
}

func initKeyRotation(client kubernetes.Interface, registry *KeyRegistry, namespace string) error {
	keyNameGenerator, _ := PrefixedNameGen(*keyListName)
	keyRotationFunc := createKeyRotationJob(client, registry, namespace, *keySize, keyNameGenerator)
	if err := keyRotationFunc(); err != nil { // create the first key
		return err
	}
	keyRotationJob := rotationErrorLogger(keyRotationFunc)
	trigger := make(chan struct{})
	rotationPeriod := time.Duration(*keyRotatePeriod*24) * time.Hour
	go ScheduleJobWithTrigger(rotationPeriod, trigger, keyRotationJob)
	return nil
}

func main2() error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	ssclient, err := sealedsecrets.NewForConfig(config)
	if err != nil {
		return err
	}

	myNs := myNamespace()

	keyRegistry, err := initKeyNameList(clientset, rand.Reader, myNs, *keyListName)
	if err != nil {
		return err
	}

	if err = initKeyRotation(clientset, keyRegistry, myNs); err != nil {
		return err
	}

	ssinformer := ssinformers.NewSharedInformerFactory(ssclient, 0)
	controller := NewController(clientset, ssinformer, keyRegistry)

	stop := make(chan struct{})
	defer close(stop)

	go controller.Run(stop)

	certProvider := func() ([]*x509.Certificate, error) {
		return []*x509.Certificate{keyRegistry.Cert()}, nil
	}
	go httpserver(certProvider, controller.AttemptUnseal)

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM)
	<-sigterm

	return nil
}

func main() {
	flag.Parse()
	goflag.CommandLine.Parse([]string{})

	if *printVersion {
		fmt.Printf("controller version: %s\n", VERSION)
		return
	}

	log.Printf("Starting sealed-secrets controller version: %s\n", VERSION)

	if err := main2(); err != nil {
		panic(err.Error())
	}
}
