package main

import (
	"context"
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

	"github.com/bitnami-labs/flagenv"
	"github.com/bitnami-labs/pflagenv"
	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealed-secrets/v1alpha1"
	"github.com/bitnami-labs/sealed-secrets/pkg/buildinfo"
	sealedsecrets "github.com/bitnami-labs/sealed-secrets/pkg/client/clientset/versioned"
	ssinformers "github.com/bitnami-labs/sealed-secrets/pkg/client/informers/externalversions"
	"github.com/bitnami-labs/sealed-secrets/pkg/vault"
)

const (
	flagEnvPrefix         = "SEALED_SECRETS"
	defaultKeyRenewPeriod = 30 * 24 * time.Hour
)

var (
	keyPrefix      = flag.String("key-prefix", "sealed-secrets-key", "Prefix used to name keys.")
	vaultEnable    = flag.Bool("vault-enable", false, "Use vault transit engine for encryption.")
	keySize        = flag.Int("key-size", 4096, "Size of encryption key.")
	validFor       = flag.Duration("key-ttl", 10*365*24*time.Hour, "Duration that certificate is valid for.")
	myCN           = flag.String("my-cn", "", "CN to use in generated certificate.")
	printVersion   = flag.Bool("version", false, "Print version information and exit")
	keyRenewPeriod = flag.Duration("key-renew-period", defaultKeyRenewPeriod, "New key generation period (automatic rotation disabled if 0)")
	acceptV1Data   = flag.Bool("accept-deprecated-v1-data", false, "Accept deprecated V1 data field")

	oldGCBehavior = flag.Bool("old-gc-behaviour", false, "Revert to old GC behavior where the controller deletes secrets instead of delegating that to k8s itself.")

	// VERSION set from Makefile
	VERSION = buildinfo.DefaultVersion

	// Selector used to find existing public/private key pairs on startup
	keySelector = fields.OneTermEqualSelector(SealedSecretsKeyLabel, "active")
)

func init() {
	buildinfo.FallbackVersion(&VERSION, buildinfo.DefaultVersion)

	flag.DurationVar(keyRenewPeriod, "rotate-period", defaultKeyRenewPeriod, "")
	flag.CommandLine.MarkDeprecated("rotate-period", "please use key-renew-period instead")

	flagenv.SetFlagsFromEnv(flagEnvPrefix, goflag.CommandLine)
	pflagenv.SetFlagsFromEnv(flagEnvPrefix, flag.CommandLine)

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

	s, err := client.CoreV1().Secrets(namespace).Get(prefix, metav1.GetOptions{})
	if !errors.IsNotFound(err) {
		if err != nil {
			return nil, err
		}
		items = append(items, *s)
		// TODO(mkm): add the label to the legacy secret to simplify discovery and backups.
	}

	keyRegistry := NewKeyRegistry(client, namespace, prefix, label, keysize)
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
func initKeyRenewal(registry *KeyRegistry, period time.Duration) (func(), error) {
	// Create a new key only if it's the first key.
	if len(registry.keys) == 0 {
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
	return ScheduleJobWithTrigger(initialDelay, period, keyGenFunc), nil
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

	trigger, err := initKeyRenewal(keyRegistry, *keyRenewPeriod)
	if err != nil {
		return err
	}

	if *vaultEnable {
		// Create the API client for Vault and start the renew process for the
		// token in a new goroutine.
		err := vault.CreateClient()
		if err != nil {
			log.Printf("Error, could not create API client for vault: %s", err.Error())
			os.Exit(1)
		}

		go vault.RenewToken()
	}

	initKeyGenSignalListener(trigger)

	ssinformer := ssinformers.NewSharedInformerFactory(ssclientset, 0)
	controller := NewController(clientset, ssclientset, ssinformer, keyRegistry)
	controller.oldGCBehavior = *oldGCBehavior

	stop := make(chan struct{})
	defer close(stop)

	go controller.Run(stop)

	cp := func() ([]*x509.Certificate, error) {
		cert, err := keyRegistry.getCert()
		if err != nil {
			return nil, err
		}
		return []*x509.Certificate{cert}, nil
	}

	server := httpserver(cp, controller.AttemptUnseal, controller.Rotate)

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM)
	<-sigterm

	return server.Shutdown(context.Background())
}

func main() {
	flag.Parse()
	goflag.CommandLine.Parse([]string{})

	ssv1alpha1.AcceptDeprecatedV1Data = *acceptV1Data

	if *printVersion {
		fmt.Printf("controller version: %s\n", VERSION)
		return
	}

	log.Printf("Starting sealed-secrets controller version: %s\n", VERSION)

	if err := main2(); err != nil {
		panic(err.Error())
	}
}
