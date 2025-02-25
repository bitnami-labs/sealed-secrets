package main

import (
	goflag "flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/bitnami-labs/sealed-secrets/pkg/controller"
	"github.com/bitnami-labs/sealed-secrets/pkg/flagenv"
	"github.com/bitnami-labs/sealed-secrets/pkg/log"
	"github.com/bitnami-labs/sealed-secrets/pkg/pflagenv"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealedsecrets/v1alpha1"
	"github.com/bitnami-labs/sealed-secrets/pkg/buildinfo"
)

const (
	flagEnvPrefix           = "SEALED_SECRETS"
	defaultKeyRenewPeriod   = 30 * 24 * time.Hour
	defaultKeyOrderPriority = "CertNotBefore"
)

var (
	// VERSION set from Makefile.
	VERSION = buildinfo.DefaultVersion
)

func bindControllerFlags(f *controller.Flags, fs *flag.FlagSet) {
	fs.StringVar(&f.KeyPrefix, "key-prefix", "sealed-secrets-key", "Prefix used to name keys.")
	fs.IntVar(&f.KeySize, "key-size", 4096, "Size of encryption key.")
	fs.DurationVar(&f.ValidFor, "key-ttl", 10*365*24*time.Hour, "Duration that certificate is valid for.")
	fs.StringVar(&f.MyCN, "my-cn", "", "Common name to be used as issuer/subject DN in generated certificate.")

	fs.DurationVar(&f.KeyRenewPeriod, "key-renew-period", defaultKeyRenewPeriod, "New key generation period (automatic rotation deactivated if 0)")
	fs.StringVar(&f.KeyOrderPriority, "key-order-priority", defaultKeyOrderPriority, "Ordering of keys based on NotBefore certificate attribute or secret creation timestamp.")
	fs.BoolVar(&f.AcceptV1Data, "accept-deprecated-v1-data", true, "Accept deprecated V1 data field.")
	fs.StringVar(&f.KeyCutoffTime, "key-cutoff-time", "", "Create a new key if latest one is older than this cutoff time. RFC1123 format with numeric timezone expected.")
	fs.BoolVar(&f.NamespaceAll, "all-namespaces", true, "Scan all namespaces or only the current namespace (default=true).")
	fs.StringVar(&f.AdditionalNamespaces, "additional-namespaces", "", "Comma-separated list of additional namespaces to be scanned.")
	fs.StringVar(&f.LabelSelector, "label-selector", "", "Label selector which can be used to filter sealed secrets.")
	fs.IntVar(&f.RateLimitPerSecond, "rate-limit", 2, "Number of allowed sustained request per second for verify endpoint")
	fs.IntVar(&f.RateLimitBurst, "rate-limit-burst", 2, "Number of requests allowed to exceed the rate limit per second for verify endpoint")
	fs.StringVar(&f.PrivateKeyAnnotations, "privatekey-annotations", "", "Comma-separated list of additional annotations to be put on renewed sealing keys.")
	fs.StringVar(&f.PrivateKeyLabels, "privatekey-labels", "", "Comma-separated list of additional labels to be put on renewed sealing keys.")

	fs.BoolVar(&f.OldGCBehavior, "old-gc-behavior", false, "Revert to old GC behavior where the controller deletes secrets instead of delegating that to k8s itself.")

	fs.BoolVar(&f.UpdateStatus, "update-status", true, "beta: if true, the controller will update the status sub-resource whenever it processes a sealed secret")

	fs.BoolVar(&f.SkipRecreate, "skip-recreate", false, "if true the controller will skip listening for managed secret changes to recreate them. This helps on limited permission environments.")

	fs.BoolVar(&f.LogInfoToStdout, "log-info-stdout", false, "if true the controller will log info to stdout.")
	fs.StringVar(&f.LogLevel, "log-level", "INFO", "Log level (INFO|ERROR).")
	fs.StringVar(&f.LogFormat, "log-format", "text", "Log format (text|json).")

	fs.DurationVar(&f.KeyRenewPeriod, "rotate-period", defaultKeyRenewPeriod, "")
	_ = fs.MarkDeprecated("rotate-period", "please use key-renew-period instead")

	fs.IntVar(&f.MaxRetries, "max-unseal-retries", 5, "Max unseal retries.")
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

	// Set logging
	logLevel := slog.Level(0)
	_ = logLevel.UnmarshalText([]byte(flags.LogLevel))
	opts := &slog.HandlerOptions{
		Level: logLevel,
	}
	if flags.LogInfoToStdout {
		slog.SetDefault(slog.New(log.New(os.Stdout, os.Stderr, flags.LogFormat, opts)))
	} else {
		slog.SetDefault(slog.New(log.New(os.Stderr, os.Stderr, flags.LogFormat, opts)))
	}

	ssv1alpha1.AcceptDeprecatedV1Data = flags.AcceptV1Data

	if printVersion {
		fmt.Fprintf(w, "controller version: %s\n", VERSION)
		return nil
	}

	slog.Info("Starting sealed-secrets controller", "version", VERSION)
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
