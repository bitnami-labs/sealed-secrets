package main

import (
	"fmt"
	"io"
	"os"

	goflag "flag"

	flag "github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/bitnami-labs/sealed-secrets/pkg/buildinfo"
	"github.com/bitnami-labs/sealed-secrets/pkg/flagenv"
	"github.com/bitnami-labs/sealed-secrets/pkg/kubeseal"
	"github.com/bitnami-labs/sealed-secrets/pkg/pflagenv"

	// Register Auth providers
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

const (
	flagEnvPrefix = "SEALED_SECRETS"
)

var (
	// VERSION set from Makefile
	VERSION = buildinfo.DefaultVersion
)

func bindFlags(f *kubeseal.Flags, fs *flag.FlagSet) {
	// TODO: Verify k8s server signature against cert in kube client config.
	fs.StringVar(&f.CertURL, "cert", "", "Certificate / public key file/URL to use for encryption. Overrides --controller-*")
	fs.StringVar(&f.ControllerNs, "controller-namespace", metav1.NamespaceSystem, "Namespace of sealed-secrets controller.")
	fs.StringVar(&f.ControllerName, "controller-name", "sealed-secrets-controller", "Name of sealed-secrets controller.")
	fs.StringVarP(&f.OutputFormat, "format", "o", "json", "Output format for sealed secret. Either json or yaml")
	fs.StringVarP(&f.OutputFileName, "sealed-secret-file", "w", "", "Sealed-secret (output) file")
	fs.StringVarP(&f.InputFileName, "secret-file", "f", "", "Secret (input) file")
	fs.BoolVar(&f.DumpCert, "fetch-cert", false, "Write certificate to stdout. Useful for later use with --cert")
	fs.BoolVar(&f.AllowEmptyData, "allow-empty-data", false, "Allow empty data in the secret object")
	fs.BoolVar(&f.ValidateSecret, "validate", false, "Validate that the sealed secret can be decrypted")
	fs.StringVar(&f.MergeInto, "merge-into", "", "Merge items from secret into an existing sealed secret file, updating the file in-place instead of writing to stdout.")
	fs.BoolVar(&f.Raw, "raw", false, "Encrypt a raw value passed via the --from-* flags instead of the whole secret object")
	fs.StringVar(&f.SecretName, "name", "", "Name of the sealed secret (required with --raw and default (strict) scope)")
	fs.StringSliceVar(&f.FromFile, "from-file", nil, "(only with --raw) Secret items can be sourced from files. Pro-tip: you can use /dev/stdin to read pipe input. This flag tries to follow the same syntax as in kubectl")
	fs.StringVar(&f.Kubeconfig, "kubeconfig", "", "Path to a kube config. Only required if out-of-cluster")

	fs.Var(&f.SealingScope, "scope", "Set the scope of the sealed secret: strict, namespace-wide, cluster-wide (defaults to strict). Mandatory for --raw, otherwise the 'sealedsecrets.bitnami.com/cluster-wide' and 'sealedsecrets.bitnami.com/namespace-wide' annotations on the input secret can be used to select the scope.")
	fs.BoolVar(&f.ReEncrypt, "rotate", false, "")
	fs.BoolVar(&f.ReEncrypt, "re-encrypt", false, "Re-encrypt the given sealed secret to use the latest cluster key.")
	_ = flag.CommandLine.MarkDeprecated("rotate", "please use --re-encrypt instead")

	fs.BoolVar(&f.Unseal, "recovery-unseal", false, "Decrypt a sealed secrets file obtained from stdin, using the private key passed with --recovery-private-key. Intended to be used in disaster recovery mode.")
	fs.StringSliceVar(&f.PrivKeys, "recovery-private-key", nil, "Private key filename used by the --recovery-unseal command. Multiple files accepted either via comma separated list or by repetition of the flag. Either PEM encoded private keys or a backup of a json/yaml encoded k8s sealed-secret controller secret (and v1.List) are accepted. ")
}

func bindClientFlags(overrides *clientcmd.ConfigOverrides) {
	flagenv.SetFlagsFromEnv(flagEnvPrefix, goflag.CommandLine)

	initUsualKubectlFlags(overrides, flag.CommandLine)

	pflagenv.SetFlagsFromEnv(flagEnvPrefix, flag.CommandLine)

	// add klog flags to goflags flagset
	klog.InitFlags(nil)
	// Standard goflags (glog in particular)
	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
}

func initUsualKubectlFlags(overrides *clientcmd.ConfigOverrides, flagset *flag.FlagSet) {
	kflags := clientcmd.RecommendedConfigOverrideFlags("")
	clientcmd.BindOverrideFlags(overrides, flagset, kflags)
}

func mainE(w io.Writer, fs *flag.FlagSet, args []string) error {
	var flags kubeseal.Flags
	var printVersion bool
	var overrides clientcmd.ConfigOverrides
	buildinfo.FallbackVersion(&VERSION, buildinfo.DefaultVersion)

	fs.BoolVar(&printVersion, "version", false, "Print version information and exit")
	bindFlags(&flags, fs)
	bindClientFlags(&overrides)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := goflag.CommandLine.Parse([]string{}); err != nil {
		return err
	}

	if printVersion {
		fmt.Fprintf(w, "kubeseal version: %s\n", VERSION)
		return nil
	}

	clientConfig := kubeseal.InitClient(flags.Kubeconfig, &overrides, os.Stdout)
	cfg := kubeseal.NewConfig(clientConfig, &flags)
	return kubeseal.Run(w, cfg)
}

func main() {
	if err := mainE(os.Stdout, flag.CommandLine, os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
