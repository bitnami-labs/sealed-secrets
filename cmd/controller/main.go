package main

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	goflag "flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	flag "github.com/spf13/pflag"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/bitnami-labs/sealed-secrets/pkg/flagenv"
	"github.com/bitnami-labs/sealed-secrets/pkg/pflagenv"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealedsecrets/v1alpha1"
	"github.com/bitnami-labs/sealed-secrets/pkg/buildinfo"
	sealedsecrets "github.com/bitnami-labs/sealed-secrets/pkg/client/clientset/versioned"
	ssinformers "github.com/bitnami-labs/sealed-secrets/pkg/client/informers/externalversions"
)

const (
	flagEnvPrefix         = "SEALED_SECRETS"
	defaultKeyRenewPeriod = 30 * 24 * time.Hour
)

var (
	// VERSION set from Makefile
	VERSION = buildinfo.DefaultVersion

	// Selector used to find existing public/private key pairs on startup
	keySelector = fields.OneTermEqualSelector(SealedSecretsKeyLabel, "active")
)

// Flags to configure the controller
type Flags struct {
	KeyPrefix            string
	KeySize              int
	ValidFor             time.Duration
	MyCN                 string
	KeyRenewPeriod       time.Duration
	AcceptV1Data         bool
	KeyCutoffTime        string
	NamespaceAll         bool
	AdditionalNamespaces string
	LabelSelector        string
	RateLimitPerSecond   int
	RateLimitBurst       int
	OldGCBehavior        bool
	UpdateStatus         bool
}

func bindControllerFlags(f *Flags) {
	flag.StringVar(&f.KeyPrefix, "key-prefix", "sealed-secrets-key", "Prefix used to name keys.")
	flag.IntVar(&f.KeySize, "key-size", 4096, "Size of encryption key.")
	flag.DurationVar(&f.ValidFor, "key-ttl", 10*365*24*time.Hour, "Duration that certificate is valid for.")
	flag.StringVar(&f.MyCN, "my-cn", "", "Common name to be used as issuer/subject DN in generated certificate.")

	flag.DurationVar(&f.KeyRenewPeriod, "key-renew-period", defaultKeyRenewPeriod, "New key generation period (automatic rotation deactivated if 0)")
	flag.BoolVar(&f.AcceptV1Data, "accept-deprecated-v1-data", true, "Accept deprecated V1 data field.")
	flag.StringVar(&f.KeyCutoffTime, "key-cutoff-time", "", "Create a new key if latest one is older than this cutoff time. RFC1123 format with numeric timezone expected.")
	flag.BoolVar(&f.NamespaceAll, "all-namespaces", true, "Scan all namespaces or only the current namespace (default=true).")
	flag.StringVar(&f.AdditionalNamespaces, "additional-namespaces", "", "Comma-separated list of additional namespaces to be scanned.")
	flag.StringVar(&f.LabelSelector, "label-selector", "", "Label selector which can be used to filter sealed secrets.")
	flag.IntVar(&f.RateLimitPerSecond, "rate-limit", 2, "Number of allowed sustained request per second for verify endpoint")
	flag.IntVar(&f.RateLimitBurst, "rate-limit-burst", 2, "Number of requests allowed to exceed the rate limit per second for verify endpoint")

	flag.BoolVar(&f.OldGCBehavior, "old-gc-behaviour", false, "Revert to old GC behavior where the controller deletes secrets instead of delegating that to k8s itself.")

	flag.BoolVar(&f.UpdateStatus, "update-status", true, "beta: if true, the controller will update the status subresource whenever it processes a sealed secret")

	flag.DurationVar(&f.KeyRenewPeriod, "rotate-period", defaultKeyRenewPeriod, "")
	_ = flag.CommandLine.MarkDeprecated("rotate-period", "please use key-renew-period instead")
}

func bindFlags(f *Flags, printVersion *bool) {
	buildinfo.FallbackVersion(&VERSION, buildinfo.DefaultVersion)
	flag.BoolVar(printVersion, "version", false, "Print version information and exit")

	bindControllerFlags(f)

	flagenv.SetFlagsFromEnv(flagEnvPrefix, goflag.CommandLine)
	pflagenv.SetFlagsFromEnv(flagEnvPrefix, flag.CommandLine)

	// Standard goflags (glog in particular)
	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	if f := flag.CommandLine.Lookup("logtostderr"); f != nil {
		f.DefValue = "true"
		_ = f.Value.Set(f.DefValue)
	}
}

func initKeyPrefix(keyPrefix string) (string, error) {
	return validateKeyPrefix(keyPrefix)
}

func initKeyRegistry(ctx context.Context, client kubernetes.Interface, r io.Reader, namespace, prefix, label string, keysize int) (*KeyRegistry, error) {
	log.Printf("Searching for existing private keys")
	secretList, err := client.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: keySelector.String(),
	})
	if err != nil {
		return nil, err
	}
	items := secretList.Items

	s, err := client.CoreV1().Secrets(namespace).Get(ctx, prefix, metav1.GetOptions{})
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
	if data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
			return ns
		}
	}

	return metav1.NamespaceDefault
}

// Initialises the first key and starts the rotation job. returns an early trigger function.
// A period of 0 deactivates automatic rotation, but manual rotation (e.g. triggered by SIGUSR1)
// is still honoured.
func initKeyRenewal(ctx context.Context, registry *KeyRegistry, period, validFor time.Duration, cutoffTime time.Time, cn string) (func(), error) {
	// Create a new key if it's the first key,
	// or if it's older than cutoff time.
	if len(registry.keys) == 0 || registry.mostRecentKey.creationTime.Before(cutoffTime) {
		if _, err := registry.generateKey(ctx, validFor, cn); err != nil {
			return nil, err
		}
	}

	// wrapper function to log error thrown by generateKey function
	keyGenFunc := func() {
		if _, err := registry.generateKey(ctx, validFor, cn); err != nil {
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

func run(f *Flags, version string) error {
	registerMetrics(version)
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
	ctx := context.Background()

	prefix, err := initKeyPrefix(f.KeyPrefix)
	if err != nil {
		return err
	}

	keyRegistry, err := initKeyRegistry(ctx, clientset, rand.Reader, myNs, prefix, SealedSecretsKeyLabel, f.KeySize)
	if err != nil {
		return err
	}

	var ct time.Time
	if f.KeyCutoffTime != "" {
		var err error
		ct, err = time.Parse(time.RFC1123Z, f.KeyCutoffTime)
		if err != nil {
			return err
		}
	}

	trigger, err := initKeyRenewal(ctx, keyRegistry, f.KeyRenewPeriod, f.ValidFor, ct, f.MyCN)
	if err != nil {
		return err
	}

	initKeyGenSignalListener(trigger)

	namespace := v1.NamespaceAll
	if !f.NamespaceAll || f.AdditionalNamespaces != "" {
		namespace = myNamespace()
		log.Printf("Starting informer for namespace: %s\n", namespace)
	}

	var tweakopts func(*metav1.ListOptions) = nil
	if f.LabelSelector != "" {
		tweakopts = func(options *metav1.ListOptions) {
			options.LabelSelector = f.LabelSelector
		}
	}

	ssinformer := ssinformers.NewFilteredSharedInformerFactory(ssclientset, 0, namespace, tweakopts)
	controller := NewController(clientset, ssclientset, ssinformer, keyRegistry)
	controller.oldGCBehavior = f.OldGCBehavior
	controller.updateStatus = f.UpdateStatus

	stop := make(chan struct{})
	defer close(stop)

	go controller.Run(stop)

	if f.AdditionalNamespaces != "" {
		addNS := removeDuplicates(strings.Split(f.AdditionalNamespaces, ","))

		var inf ssinformers.SharedInformerFactory
		var ctlr *Controller

		for _, ns := range addNS {
			if _, err := clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{}); err != nil {
				if errors.IsNotFound(err) {
					log.Printf("Warning: namespace '%s' doesn't exist\n", ns)
					continue
				}
				return err
			}
			if ns != namespace {
				inf = ssinformers.NewFilteredSharedInformerFactory(ssclientset, 0, ns, tweakopts)
				ctlr = NewController(clientset, ssclientset, inf, keyRegistry)
				ctlr.oldGCBehavior = f.OldGCBehavior
				ctlr.updateStatus = f.UpdateStatus
				log.Printf("Starting informer for namespace: %s\n", ns)
				go ctlr.Run(stop)
			}
		}
	}

	cp := func() ([]*x509.Certificate, error) {
		cert, err := keyRegistry.getCert()
		if err != nil {
			return nil, err
		}
		return []*x509.Certificate{cert}, nil
	}

	server := httpserver(cp, controller.AttemptUnseal, controller.Rotate, f.RateLimitBurst, f.RateLimitPerSecond)

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM)
	<-sigterm

	return server.Shutdown(context.Background())
}

func main() {
	var printVersion bool
	var flags Flags
	bindFlags(&flags, &printVersion)
	flag.Parse()
	_ = goflag.CommandLine.Parse([]string{})

	ssv1alpha1.AcceptDeprecatedV1Data = flags.AcceptV1Data

	fmt.Printf("controller version: %s\n", VERSION)
	if printVersion {
		return
	}

	log.Printf("Starting sealed-secrets controller version: %s\n", VERSION)
	if err := run(&flags, VERSION); err != nil {
		panic(err.Error())
	}
}
