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

	// Selector used to find existing public/private key pairs on startup
	keySelector = SealedSecretsKeyLabel + "=" + "active"
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

func initKeyRegistry(client kubernetes.Interface, r io.Reader, namespace, listName string, keysize int) (*KeyRegistry, error) {
	log.Printf("Searching for existing private keys")
	secretList, err := client.Core().Secrets(namespace).List(metav1.ListOptions{
		LabelSelector: keySelector,
	})
	if err != nil {
		return nil, err
	}
	keyRegistry := NewKeyRegistry(client, namespace, listName, keysize)
	for _, secret := range secretList.Items {
		key, certs, err := readKey(secret)
		if err != nil {
			log.Printf("Error reading key %s: %v", secret.Name, err)
		}
		keyRegistry.registerNewKey(secret.Name, key, certs[0])
		log.Printf("----- %s", secret.Name)
	}
	return keyRegistry, nil
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

func initKeyRotation(registry *KeyRegistry, period time.Duration) (func(), error) {
	keyGenFunc := createKeyGenJob(registry)
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

	keyRegistry, err := initKeyRegistry(clientset, rand.Reader, myNs, *keyListName, *keySize)
	if err != nil {
		return err
	}

	_, err = initKeyRotation(keyRegistry, *keyRotatePeriod)
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
