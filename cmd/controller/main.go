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
	"sort"
	"strings"
	"syscall"
	"time"

	flag "github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealed-secrets/v1alpha1"
	sealedsecrets "github.com/bitnami-labs/sealed-secrets/pkg/client/clientset/versioned"
	ssinformers "github.com/bitnami-labs/sealed-secrets/pkg/client/informers/externalversions"
)

var (
	keyPrefix       = flag.String("key-prefix", "sealed-secrets-key", "Prefix used to name keys.")
	keySize         = flag.Int("key-size", 4096, "Size of encryption key.")
	validFor        = flag.Duration("key-ttl", 10*365*24*time.Hour, "Duration that certificate is valid for.")
	myCN            = flag.String("my-cn", "", "CN to use in generated certificate.")
	printVersion    = flag.Bool("version", false, "Print version information and exit")
	keyRotatePeriod = flag.Duration("rotate-period", 0, "New key generation period (automatic rotation disabled if 0)")

	// VERSION set from Makefile
	VERSION = "UNKNOWN"

	// Selector used to find existing public/private key pairs on startup
	keySelector = fields.OneTermEqualSelector(SealedSecretsKeyLabel, "active")
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

func initKeyPrefix(keyPrefix string) (string, error) {
	prefix, err := validateKeyPrefix(keyPrefix)
	if err != nil {
		return "", err
	}
	return prefix, err
}

func initKeyRegistry(client kubernetes.Interface, r io.Reader, namespace, prefix, label string, keysize int) (*KeyRegistry, error) {
	log.Printf("Searching for existing private keys")
	secretList, err := client.CoreV1().Secrets(namespace).List(metav1.ListOptions{
		LabelSelector: keySelector.String(),
	})
	if err != nil {
		return nil, err
	}
	items := secretList.Items
	if len(items) == 0 {
		s, err := client.CoreV1().Secrets(namespace).Get(prefix, metav1.GetOptions{})
		if !errors.IsNotFound(err) {
			if err != nil {
				return nil, err
			}
			items = append(items, *s)
			// TODO(mkm): add the label to the legacy secret
		}
	}
	keyRegistry := NewKeyRegistry(client, namespace, prefix, label, keysize)
	sort.Sort(ssv1alpha1.ByCreationTimestamp(items))
	for _, secret := range items {
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

// Initialises the first key and starts the rotation job. returns an early trigger function.
// A period of 0 disables automatic rotation, but manual rotation (e.g. triggered by SIGUSR1)
// is still honoured.
func initKeyRotation(registry *KeyRegistry, period time.Duration) (func(), error) {
	// Create a new key only if it's the first key or if we have automatic key rotation.
	// Since the rotation period might be longer than the average pod run time (eviction, updates, crashes etc)
	// we err on the side of increased rotation frequency rather than overshooting the rotation goals.
	//
	// TODO(mkm): implement rotation cadence based on resource times rather than just an in-process timer.
	if period != 0 || len(registry.privateKeys) == 0 {
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
	return ScheduleJobWithTrigger(period, keyGenFunc), nil
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

func main2() error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	ssclientset, err := sealedsecrets.NewForConfig(config)
	if err != nil {
		return err
	}

	myNs := myNamespace()

	prefix, err := initKeyPrefix(*keyPrefix)
	if err != nil {
		return err
	}

	keyRegistry, err := initKeyRegistry(clientset, rand.Reader, myNs, prefix, SealedSecretsKeyLabel, *keySize)
	if err != nil {
		return err
	}

	trigger, err := initKeyRotation(keyRegistry, *keyRotatePeriod)
	if err != nil {
		return err
	}

	initKeyGenSignalListener(trigger)

	ssinformer := ssinformers.NewSharedInformerFactory(ssclientset, 0)
	controller := NewController(clientset, ssclientset, ssinformer, keyRegistry)

	stop := make(chan struct{})
	defer close(stop)

	go controller.Run(stop)

	cp := func() []*x509.Certificate {
		return []*x509.Certificate{keyRegistry.cert}
	}

	go httpserver(cp, controller.AttemptUnseal, controller.Rotate)

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
