package main

import (
	"crypto/rand"
	"crypto/x509"
	goflag "flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	flag "github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	sealedsecrets "github.com/bitnami-labs/sealed-secrets/pkg/client/clientset/versioned"
	ssinformers "github.com/bitnami-labs/sealed-secrets/pkg/client/informers/externalversions"
)

var (
	keyListName     = flag.String("key-list", "sealed-secrets-keys", "Name of Secret containing names of public/private keys.")
	blacklistName   = flag.String("blacklist", "sealed-secrets-keys-blacklist", "Name of the blacklist of keys")
	keySize         = flag.Int("key-size", 4096, "Size of encryption key.")
	validFor        = flag.Duration("key-ttl", 10*365*24*time.Hour, "Duration that certificate is valid for.")
	myCN            = flag.String("my-cn", "", "CN to use in generated certificate.")
	printVersion    = flag.Bool("version", false, "Print version information and exit")
	keyRotatePeriod = flag.Duration("key-rotate", time.Minute, "New key generation period")

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

func initKeyRegistry(client kubernetes.Interface, r io.Reader, namespace, listName string) (*KeyRegistry, error) {
	list, err := readKeyRegistry(client, namespace, listName)
	if err != nil {
		if errors.IsNotFound(err) {
			// keylist isn't found, create a new one
			log.Printf("Keyname list %s/%s not found, generating new keyname list", namespace, listName)

			privKey, cert, err := generatePrivateKeyAndCert(*keySize)
			if err != nil {
				return nil, err
			}

			if err = writeKeyRegistry(client, privKey, cert, namespace, listName); err != nil {
				return nil, err
			}
			log.Printf("New keyname list generated")
			return NewKeyRegistry(), nil
		}
		return nil, err
	}
	// If a keylist is found, read each value, retrive the key and add to the registry
	log.Printf("Keyname list %s/%s found, copying values into local store", namespace, listName)
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

func initBlacklist(client kubernetes.Interface, r io.Reader, registry *KeyRegistry, namespace, blacklistName string, trigger func()) (func(string) (bool, error), error) {
	blacklist, err := readBlacklist(client, namespace, blacklistName)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Printf("Blacklist name %s/%s not found, generating a new blacklist", namespace, blacklistName)
			privkey, cert, err := generatePrivateKeyAndCert(*keySize)
			if err != nil {
				return nil, err
			}
			if err = writeBlacklist(client, privkey, cert, namespace, blacklistName); err != nil {
				return nil, err
			}
			log.Printf("New blacklist generated")
		} else {
			return nil, err
		}
	} else {
		log.Printf("Blacklist found, copying values into local store")
		for keyname := range blacklist {
			registry.blacklistKey(keyname)
		}
	}
	return createBlacklister(client, namespace, blacklistName, registry, trigger), nil
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

func initKeyRotation(client kubernetes.Interface, registry *KeyRegistry, namespace, listname string, keysize int, period time.Duration) (func(), error) {
	keyGenFunc := createKeyGenJob(client, registry, namespace, listname, keysize, listname)
	if err := keyGenFunc(); err != nil { // create the first key
		return nil, err
	}
	keyRotationJob := rotationErrorLogger(keyGenFunc)
	return ScheduleJobWithTrigger(period, keyRotationJob), nil
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

	if err := validateKeyName(*keyListName); err != nil {
		return err
	}

	keyRegistry, err := initKeyRegistry(clientset, rand.Reader, myNs, *keyListName)
	if err != nil {
		return err
	}

	_, err = initKeyRotation(clientset, keyRegistry, myNs, *keyListName, *keySize, *keyRotatePeriod)
	if err != nil {
		return err
	}

	ssinformer := ssinformers.NewSharedInformerFactory(ssclient, 0)
	controller := NewController(clientset, ssinformer, keyRegistry)

	stop := make(chan struct{})
	defer close(stop)

	go controller.Run(stop)

	cp := func(keyname string) ([]*x509.Certificate, error) {
		cert, err := keyRegistry.getCert(keyname)
		if err != nil {
			return nil, err
		}
		return []*x509.Certificate{cert}, nil
	}
	cnp := func() (string, error) {
		return keyRegistry.latestKeyName(), nil
	}

	go httpserver(cp, cnp, controller.AttemptUnseal, controller.Rotate)

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
