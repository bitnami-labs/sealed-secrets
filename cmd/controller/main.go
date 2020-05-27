package main

import (
	"context"
	goflag "flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	flag "github.com/spf13/pflag"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/bitnami-labs/flagenv"
	"github.com/bitnami-labs/pflagenv"
	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealed-secrets/v1alpha1"
	ssbackend "github.com/bitnami-labs/sealed-secrets/pkg/backend"
	"github.com/bitnami-labs/sealed-secrets/pkg/backend/aes"
	"github.com/bitnami-labs/sealed-secrets/pkg/backend/aws"
	"github.com/bitnami-labs/sealed-secrets/pkg/buildinfo"
	sealedsecrets "github.com/bitnami-labs/sealed-secrets/pkg/client/clientset/versioned"
	ssinformers "github.com/bitnami-labs/sealed-secrets/pkg/client/informers/externalversions"
	"github.com/bitnami-labs/sealed-secrets/pkg/utils"
)

const (
	flagEnvPrefix         = "SEALED_SECRETS"
	defaultKeyRenewPeriod = 30 * 24 * time.Hour
)

var (
	encryptBackend = flag.String("backend", "AES-256", "Encryption backend used to encrypt/secret (AES-256, AWS-KMS).")
	keyPrefix      = flag.String("key-prefix", "sealed-secrets-key", "Prefix used to name keys.")
	keySize        = flag.Int("key-size", 4096, "Size of encryption key.")
	validFor       = flag.Duration("key-ttl", 10*365*24*time.Hour, "Duration that certificate is valid for.")
	myCN           = flag.String("my-cn", "", "CN to use in generated certificate.")
	printVersion   = flag.Bool("version", false, "Print version information and exit")
	keyRenewPeriod = flag.Duration("key-renew-period", defaultKeyRenewPeriod, "New key generation period (automatic rotation disabled if 0)")
	acceptV1Data   = flag.Bool("accept-deprecated-v1-data", false, "Accept deprecated V1 data field.")
	keyCutoffTime  = flag.String("key-cutoff-time", "", "Create a new key if latest one is older than this cutoff time. RFC1123 format with numeric timezone expected.")
	awsKmsKeyID    = flag.String("aws-kms-key-id", "", "AWS KMS key ID used to encrypt/decrypt secrets.")
	namespaceAll   = flag.Bool("all-namespaces", true, "Scan all namespaces or only the current namespace (default=true).")

	oldGCBehavior = flag.Bool("old-gc-behaviour", false, "Revert to old GC behavior where the controller deletes secrets instead of delegating that to k8s itself.")

	updateStatus = flag.Bool("update-status", false, "beta: if true, the controller will update the status subresource whenever it processes a sealed secret")

	// VERSION set from Makefile
	VERSION = buildinfo.DefaultVersion
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

	myNs := utils.MyNamespace()

	var backend ssbackend.Backend

	switch *encryptBackend {
	case "AES-256":
		backend, err = aes.NewAES256WithKeyRegistry(clientset, myNs, *keyPrefix, *keySize, *validFor, *myCN, *keyRenewPeriod, *keyCutoffTime)
		if err != nil {
			return err
		}
	case "AWS-KMS":
		if *awsKmsKeyID == "" {
			return fmt.Errorf("must provide the --aws-kms-key-id flag with AWS-KMS backend")
		}
		backend, err = aws.NewKMS(*awsKmsKeyID)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid encryption backend: %s", *encryptBackend)
	}

	namespace := v1.NamespaceAll
	if !*namespaceAll {
		namespace = utils.MyNamespace()
	}

	ssinformer := ssinformers.NewFilteredSharedInformerFactory(ssclientset, 0, namespace, nil)
	controller := NewController(clientset, ssclientset, ssinformer, &backend)
	controller.oldGCBehavior = *oldGCBehavior
	controller.updateStatus = *updateStatus

	stop := make(chan struct{})
	defer close(stop)

	go controller.Run(stop)

	server := httpserver(backend.ProviderHandler, controller.AttemptUnseal, controller.Rotate)

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM)
	<-sigterm

	return server.Shutdown(context.Background())
}

func main() {
	flag.Parse()
	goflag.CommandLine.Parse([]string{})

	ssv1alpha1.AcceptDeprecatedV1Data = *acceptV1Data

	fmt.Printf("controller version: %s\n", VERSION)
	if *printVersion {
		return
	}

	log.Printf("Starting sealed-secrets controller version: %s\n", VERSION)

	if err := main2(); err != nil {
		panic(err.Error())
	}
}
