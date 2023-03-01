package main

import (
	goflag "flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/bitnami-labs/sealed-secrets/pkg/controller"
	"github.com/bitnami-labs/sealed-secrets/pkg/flagenv"
	"github.com/bitnami-labs/sealed-secrets/pkg/pflagenv"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealedsecrets/v1alpha1"
	"github.com/bitnami-labs/sealed-secrets/pkg/buildinfo"
)

const (
	flagEnvPrefix         = "SEALED_SECRETS"
	defaultKeyRenewPeriod = 30 * 24 * time.Hour
)

var (
	// VERSION set from Makefile
	VERSION = buildinfo.DefaultVersion
)

func bindControllerFlags(f *controller.Flags, fs *flag.FlagSet) {
	fs.StringVar(&f.KeyPrefix, "key-prefix", "sealed-secrets-key", "Prefix used to name keys.")
	fs.IntVar(&f.KeySize, "key-size", 4096, "Size of encryption key.")
	fs.DurationVar(&f.ValidFor, "key-ttl", 10*365*24*time.Hour, "Duration that certificate is valid for.")
	fs.StringVar(&f.MyCN, "my-cn", "", "Common name to be used as issuer/subject DN in generated certificate.")

	fs.DurationVar(&f.KeyRenewPeriod, "key-renew-period", defaultKeyRenewPeriod, "New key generation period (automatic rotation deactivated if 0)")
	fs.BoolVar(&f.AcceptV1Data, "accept-deprecated-v1-data", true, "Accept deprecated V1 data field.")
	fs.StringVar(&f.KeyCutoffTime, "key-cutoff-time", "", "Create a new key if latest one is older than this cutoff time. RFC1123 format with numeric timezone expected.")
	fs.BoolVar(&f.NamespaceAll, "all-namespaces", true, "Scan all namespaces or only the current namespace (default=true).")
	fs.StringVar(&f.AdditionalNamespaces, "additional-namespaces", "", "Comma-separated list of additional namespaces to be scanned.")
	fs.StringVar(&f.LabelSelector, "label-selector", "", "Label selector which can be used to filter sealed secrets.")
	fs.IntVar(&f.RateLimitPerSecond, "rate-limit", 2, "Number of allowed sustained request per second for verify endpoint")
	fs.IntVar(&f.RateLimitBurst, "rate-limit-burst", 2, "Number of requests allowed to exceed the rate limit per second for verify endpoint")

	fs.BoolVar(&f.OldGCBehavior, "old-gc-behavior", false, "Revert to old GC behavior where the controller deletes secrets instead of delegating that to k8s itself.")

	fs.BoolVar(&f.UpdateStatus, "update-status", true, "beta: if true, the controller will update the status sub-resource whenever it processes a sealed secret")

	fs.BoolVar(&f.Recreate, "recreate", true, "if true the controller will listen for secret changes to recreate managed secrets on removal. Helps setting it to false on limited permission environments.")

	fs.DurationVar(&f.KeyRenewPeriod, "rotate-period", defaultKeyRenewPeriod, "")
	_ = fs.MarkDeprecated("rotate-period", "please use key-renew-period instead")
}

func bindFlags(f *controller.Flags, fs *flag.FlagSet, gofs *goflag.FlagSet) {
	bindControllerFlags(f, fs)

	flagenv.SetFlagsFromEnv(flagEnvPrefix, gofs)
	pflagenv.SetFlagsFromEnv(flagEnvPrefix, fs)

	// Standard goflags (glog in particular)
	fs.AddGoFlagSet(gofs)
	if f := fs.Lookup("logtostderr"); f != nil {
		f.DefValue = "true"
		_ = f.Value.Set(f.DefValue)
	}
}

func mainE(w io.Writer, fs *flag.FlagSet, gofs *goflag.FlagSet, args []string) error {
	var printVersion bool
	var flags controller.Flags

	buildinfo.FallbackVersion(&VERSION, buildinfo.DefaultVersion)
	fs.BoolVar(&printVersion, "version", false, "Print version information and exit")
	bindFlags(&flags, fs, gofs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := gofs.Parse([]string{}); err != nil {
		return err
	}

	ssv1alpha1.AcceptDeprecatedV1Data = flags.AcceptV1Data

	fmt.Fprintf(w, "controller version: %s\n", VERSION)
	if printVersion {
		return nil
	}

	log.Printf("Starting sealed-secrets controller version: %s\n", VERSION)
	if err := controller.Main(&flags, VERSION); err != nil {
		panic(err)
	}
	return nil
}

func main() {
	if err := mainE(os.Stdout, flag.CommandLine, goflag.CommandLine, os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
