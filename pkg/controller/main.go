package controller

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"k8s.io/client-go/informers"

	ssv1alpha1 "github.com/bitnami/sealed-secrets/pkg/apis/sealedsecrets/v1alpha1"
	"github.com/bitnami/sealed-secrets/pkg/client/clientset/versioned"
	sealedsecrets "github.com/bitnami/sealed-secrets/pkg/client/clientset/versioned"
	ssinformers "github.com/bitnami/sealed-secrets/pkg/client/informers/externalversions"
)

// Cap concurrent namespace Gets and informer setup so a large
// --additional-namespaces list does not serialize one RTT per namespace
// and does not stampede the API server either.
const additionalNamespaceBootstrapConcurrency = 16

var (
	// Selector used to find existing public/private key pairs on startup.
	keySelector = fields.OneTermEqualSelector(SealedSecretsKeyLabel, "active")
)

// Flags to configure the controller.
type Flags struct {
	KeyPrefix               string
	KeySize                 int
	ValidFor                time.Duration
	MyCN                    string
	KeyRenewPeriod          time.Duration
	KeyOrderPriority        string
	AcceptV1Data            bool
	KeyCutoffTime           string
	NamespaceAll            bool
	AdditionalNamespaces    string
	LabelSelector           string
	RateLimitPerSecond      int
	RateLimitBurst          int
	MetricsOmitSecretLabels bool
	OldGCBehavior           bool
	UpdateStatus            bool
	SkipRecreate            bool
	LogInfoToStdout         bool
	LogLevel                string
	LogFormat               string
	PrivateKeyAnnotations   string
	PrivateKeyLabels        string
	MaxRetries              int
	WatchForSecrets         bool
	KubeClientQPS           float32
	KubeClientBurst         int
}

func initKeyPrefix(keyPrefix string) (string, error) {
	return validateKeyPrefix(keyPrefix)
}

func initKeyRegistry(ctx context.Context, client kubernetes.Interface, r io.Reader, namespace, prefix, label string, keysize int, keyOrderPriority string) (*KeyRegistry, error) {
	slog.Info("Searching for existing private keys")
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
		err = registryNewKeyWithSecret(&secret, keyRegistry, keyOrderPriority)
		if err != nil {
			return nil, err
		}
	}
	return keyRegistry, nil
}

func registryNewKeyWithSecret(secret *v1.Secret, keyRegistry *KeyRegistry, keyOrderPriority string) error {
	key, certs, err := readKey(secret)
	if err != nil {
		slog.Error("Error reading key", "secret", secret.Name, "error", err)
		return err
	}
	if len(certs) == 0 {
		return fmt.Errorf("no certificates found in secret %s", secret.Name)
	}

	// Select ordering time based on the keyOrderPriority flag
	orderingTime := getKeyOrderPriority(keyOrderPriority, certs[0], secret)

	if err := keyRegistry.registerNewKey(secret.Name, key, certs[0], orderingTime); err != nil {
		return err
	}
	slog.Info("registered private key", "secretname", secret.Name)
	return nil
}

func getKeyOrderPriority(keyOrderPriority string, cert *x509.Certificate, secret *v1.Secret) time.Time {
	switch keyOrderPriority {
	case "CertNotBefore":
		return cert.NotBefore
	case "SecretCreationTimestamp":
		return secret.GetCreationTimestamp().Time
	default:
		slog.Error("Invalid keyOrderPriority. Use CertNotBefore or SecretCreationTimestamp", "keyOrderPriority", keyOrderPriority)
	}
	return cert.NotBefore
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
func initKeyRenewal(ctx context.Context, registry *KeyRegistry, period, validFor time.Duration, cutoffTime time.Time, cn string, privateKeyAnnotations string, privateKeyLabels string) (func(), error) {
	// Create a new key if it's the first key,
	// or if it's older than cutoff time.
	mostRecentTime, err := registry.mostRecentKeyTime()
	if err != nil || mostRecentTime.Before(cutoffTime) {
		if _, err := registry.generateKey(ctx, validFor, cn, privateKeyAnnotations, privateKeyLabels); err != nil {
			return nil, err
		}
	}

	// wrapper function to log error thrown by generateKey function
	keyGenFunc := func() {
		if _, err := registry.generateKey(ctx, validFor, cn, privateKeyAnnotations, privateKeyLabels); err != nil {
			slog.Error("Failed to generate new key", "error", err)
		}
	}
	if period == 0 {
		return keyGenFunc, nil
	}

	// If key rotation is enabled, we'll rotate the key when the most recent
	// key becomes stale (older than period).
	mostRecentTime, _ = registry.mostRecentKeyTime()
	mostRecentKeyAge := time.Since(mostRecentTime)
	initialDelay := period - mostRecentKeyAge
	if initialDelay < 0 {
		initialDelay = 0
	}
	return ScheduleJobWithTrigger(initialDelay, period, keyGenFunc), nil
}

func Main(f *Flags, version string) error {
	SetMetricsOmitSecretLabels(f.MetricsOmitSecretLabels)
	registerMetrics(version)

	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	config.QPS = f.KubeClientQPS
	config.Burst = f.KubeClientBurst

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

	keyRegistry, err := initKeyRegistry(ctx, clientset, rand.Reader, myNs, prefix, SealedSecretsKeyLabel, f.KeySize, f.KeyOrderPriority)
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

	trigger, err := initKeyRenewal(ctx, keyRegistry, f.KeyRenewPeriod, f.ValidFor, ct, f.MyCN, f.PrivateKeyAnnotations, f.PrivateKeyLabels)
	if err != nil {
		return err
	}

	initKeyGenSignalListener(trigger)

	namespace := v1.NamespaceAll
	if !f.NamespaceAll || f.AdditionalNamespaces != "" {
		namespace = myNamespace()
		slog.Info("Starting informer", "namespace", namespace)
	}

	var tweakopts func(*metav1.ListOptions) = nil
	if f.LabelSelector != "" {
		tweakopts = func(options *metav1.ListOptions) {
			options.LabelSelector = f.LabelSelector
		}
	}

	controller, err := prepareController(clientset, namespace, myNs, tweakopts, f, ssclientset, keyRegistry)
	if err != nil {
		return err
	}
	controller.oldGCBehavior = f.OldGCBehavior
	controller.updateStatus = f.UpdateStatus

	stop := make(chan struct{})
	defer close(stop)

	go controller.Run(stop)

	// ready becomes true after additional-namespace informers are started.
	// HTTP must come up first so liveness probes succeed during that work.
	var bootstrapped atomic.Bool
	cp := func() ([]*x509.Certificate, error) {
		cert, err := keyRegistry.getCert()
		if err != nil {
			return nil, err
		}
		return []*x509.Certificate{cert}, nil
	}
	server := httpserver(cp, controller.AttemptUnseal, controller.Rotate, f.RateLimitBurst, f.RateLimitPerSecond, bootstrapped.Load)
	serverMetrics := httpserverMetrics()

	if f.AdditionalNamespaces != "" {
		addNS := removeDuplicates(strings.Split(f.AdditionalNamespaces, ","))
		// Bind ctx to the namespace probe here so the bootstrap helper does not
		// take a context parameter (contextcheck would otherwise require
		// prepareController/Run to accept one; those still use stopCh lifecycle).
		probeNS := func(ns string) error {
			_, err := clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
			return err
		}
		if err := startAdditionalNamespaceControllers(probeNS, clientset, ssclientset, keyRegistry, f, namespace, myNs, tweakopts, addNS, stop); err != nil {
			return err
		}
	}
	bootstrapped.Store(true)

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM)
	<-sigterm

	if err := server.Shutdown(context.Background()); err != nil {
		return err
	}

	if err := serverMetrics.Shutdown(context.Background()); err != nil {
		return err
	}

	return nil
}

// startAdditionalNamespaceControllers validates and starts a controller for each
// extra namespace. Work is bounded-parallel so wall-clock startup scales better
// than one serial API Get per namespace.
//
// probeNS should check that ns exists (typically clientset Get with the caller's
// context). Missing namespaces are logged and skipped; other probe errors abort.
func startAdditionalNamespaceControllers(
	probeNS func(ns string) error,
	clientset kubernetes.Interface,
	ssclientset versioned.Interface,
	keyRegistry *KeyRegistry,
	f *Flags,
	homeNamespace string,
	keyNamespace string,
	tweakopts func(*metav1.ListOptions),
	namespaces []string,
	stop <-chan struct{},
) error {
	sem := make(chan struct{}, additionalNamespaceBootstrapConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	setErr := func(err error) {
		if err == nil {
			return
		}
		mu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		mu.Unlock()
	}

	for _, ns := range namespaces {
		ns := strings.TrimSpace(ns)
		if ns == "" || ns == homeNamespace {
			continue
		}
		wg.Add(1)
		go func(ns string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := probeNS(ns); err != nil {
				if errors.IsNotFound(err) {
					slog.Error("namespace doesn't exist", "namespace", ns)
					return
				}
				setErr(err)
				return
			}
			ctlr, err := prepareController(clientset, ns, keyNamespace, tweakopts, f, ssclientset, keyRegistry)
			if err != nil {
				setErr(err)
				return
			}
			ctlr.oldGCBehavior = f.OldGCBehavior
			ctlr.updateStatus = f.UpdateStatus
			slog.Info("Starting informer", "namespace", ns)
			go ctlr.Run(stop)
		}(ns)
	}
	wg.Wait()
	return firstErr
}

func prepareController(
	clientset kubernetes.Interface,
	namespace string,
	keyNamespace string,
	tweakopts func(*metav1.ListOptions),
	f *Flags,
	ssclientset versioned.Interface,
	keyRegistry *KeyRegistry,
) (*Controller, error) {
	kinformer := initSecretInformerFactory(clientset, keyNamespace, func(options *metav1.ListOptions) {
		options.LabelSelector = keySelector.String()
	}, f.WatchForSecrets)
	sinformer := initSecretInformerFactory(clientset, namespace, tweakopts, !f.SkipRecreate)
	ssinformer := ssinformers.NewFilteredSharedInformerFactory(ssclientset, 0, namespace, tweakopts)
	controller, err := NewController(clientset, ssclientset, ssinformer, sinformer, kinformer, keyRegistry, f.MaxRetries, f.KeyOrderPriority)
	return controller, err
}

func initSecretInformerFactory(clientset kubernetes.Interface, ns string, tweakopts func(*metav1.ListOptions), enabled bool) informers.SharedInformerFactory {
	if !enabled {
		return nil
	}
	return informers.NewSharedInformerFactoryWithOptions(clientset, 0, informers.WithNamespace(ns), informers.WithTweakListOptions(tweakopts))
}
