package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	goflag "flag"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealedsecrets/v1alpha1"
	"github.com/google/renameio"
	"github.com/mattn/go-isatty"
	flag "github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
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

type Flags struct {
	CertURL        string
	ControllerNs   string
	ControllerName string
	OutputFormat   string
	OutputFileName string
	InputFileName  string
	Kubeconfig     string
	DumpCert       bool
	AllowEmptyData bool
	ValidateSecret bool
	MergeInto      string
	Raw            bool
	SecretName     string
	FromFile       []string
	SealingScope   ssv1alpha1.SealingScope
	ReEncrypt      bool
	Unseal         bool
	PrivKeys       []string
}

type Config struct {
	flags          *Flags
	clientConfig   clientcmd.ClientConfig
	ctx            context.Context
	solveNamespace kubeseal.NamespaceFn
}

func newConfig(clientConfig clientcmd.ClientConfig, flags *Flags) *Config {
	return &Config{
		flags:          flags,
		clientConfig:   clientConfig,
		ctx:            context.Background(),
		solveNamespace: initNamespaceFuncFromClient(clientConfig),
	}
}

func initNamespaceFuncFromClient(clientConfig clientcmd.ClientConfig) kubeseal.NamespaceFn {
	return func() (string, bool, error) { return clientConfig.Namespace() }
}

func initClient(kubeConfigPath string, cfgOverrides *clientcmd.ConfigOverrides, r io.Reader) clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	loadingRules.ExplicitPath = kubeConfigPath
	return clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, cfgOverrides, r)
}

func bindFlags(f *Flags, fs *flag.FlagSet) {
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
	_ = fs.MarkDeprecated("rotate", "please use --re-encrypt instead")

	fs.BoolVar(&f.Unseal, "recovery-unseal", false, "Decrypt a sealed secrets file obtained from stdin, using the private key passed with --recovery-private-key. Intended to be used in disaster recovery mode.")
	fs.StringSliceVar(&f.PrivKeys, "recovery-private-key", nil, "Private key filename used by the --recovery-unseal command. Multiple files accepted either via comma separated list or by repetition of the flag. Either PEM encoded private keys or a backup of a json/yaml encoded k8s sealed-secret controller secret (and v1.List) are accepted. ")
}

func bindClientFlags(fs *flag.FlagSet, gofs *goflag.FlagSet, overrides *clientcmd.ConfigOverrides) {
	flagenv.SetFlagsFromEnv(flagEnvPrefix, gofs)

	initUsualKubectlFlags(overrides, fs)

	pflagenv.SetFlagsFromEnv(flagEnvPrefix, fs)

	// add klog flags to goflags flagset
	klog.InitFlags(nil)
	// Standard goflags (glog in particular)
	fs.AddGoFlagSet(gofs)
}

func initUsualKubectlFlags(overrides *clientcmd.ConfigOverrides, fs *flag.FlagSet) {
	kflags := clientcmd.RecommendedConfigOverrideFlags("")
	clientcmd.BindOverrideFlags(overrides, fs, kflags)
}

func runKubeseal(w io.Writer, cfg *Config) (err error) {
	flags := cfg.flags
	if len(flags.FromFile) != 0 && !flags.Raw {
		return fmt.Errorf("--from-file requires --raw")
	}

	var input io.Reader = os.Stdin
	if flags.InputFileName != "" {
		// #nosec G304 -- should open user provided file
		f, err := os.Open(flags.InputFileName)
		if err != nil {
			return nil
		}
		// #nosec: G307 -- this deferred close is fine because it is not on a writable file
		defer f.Close()

		input = f
	} else if !flags.Raw && !flags.DumpCert {
		if isatty.IsTerminal(os.Stdin.Fd()) {
			fmt.Fprintf(os.Stderr, "(tty detected: expecting json/yaml k8s resource in stdin)\n")
		}
	}

	// reEncrypt is the only "in-place" update subcommand. When the user only provides one file (the input file)
	// we'll use the same file for output (see #405).
	if flags.ReEncrypt && (flags.OutputFileName == "" && flags.InputFileName != "") {
		flags.OutputFileName = flags.InputFileName
	}
	if flags.OutputFileName != "" {
		if ext := filepath.Ext(flags.OutputFileName); ext == ".yaml" || ext == ".yml" {
			flags.OutputFormat = "yaml"
		}

		var f *renameio.PendingFile
		f, err = renameio.TempFile("", flags.OutputFileName)
		if err != nil {
			return err
		}
		// only write the output file if the run function exits without errors.
		defer func() {
			if err == nil {
				_ = f.CloseAtomicallyReplace()
			}
		}()

		w = f
	}

	if flags.Unseal {
		return kubeseal.UnsealSealedSecret(w, input, flags.PrivKeys, flags.OutputFormat, scheme.Codecs)
	}
	if len(flags.PrivKeys) != 0 && isatty.IsTerminal(os.Stderr.Fd()) {
		fmt.Fprintf(os.Stderr, "warning: ignoring --recovery-private-key because unseal command not chosen with --recovery-unseal\n")
	}

	if flags.ValidateSecret {
		return kubeseal.ValidateSealedSecret(cfg.ctx, cfg.clientConfig, flags.ControllerNs, flags.ControllerName, input)
	}

	if flags.ReEncrypt {
		return kubeseal.ReEncryptSealedSecret(cfg.ctx, cfg.clientConfig, flags.ControllerNs, flags.ControllerName, flags.OutputFormat, input, w, scheme.Codecs)
	}

	f, err := kubeseal.OpenCert(cfg.ctx, cfg.clientConfig, flags.ControllerNs, flags.ControllerName, flags.CertURL)
	if err != nil {
		return err
	}
	defer f.Close()

	if flags.DumpCert {
		_, err := io.Copy(w, f)
		return err
	}

	pubKey, err := kubeseal.ParseKey(f)
	if err != nil {
		return err
	}

	if flags.MergeInto != "" {
		return kubeseal.SealMergingInto(cfg.solveNamespace, flags.OutputFormat, input, flags.MergeInto, scheme.Codecs, pubKey, flags.SealingScope, flags.AllowEmptyData)
	}

	if flags.Raw {
		var (
			ns  string
			err error
		)
		if flags.SealingScope < ssv1alpha1.ClusterWideScope {
			ns, _, err = cfg.solveNamespace()
			if err != nil {
				return err
			}

			if ns == "" {
				return fmt.Errorf("must provide the --namespace flag with --raw and --scope %s", flags.SealingScope.String())
			}

			if flags.SecretName == "" && flags.SealingScope < ssv1alpha1.NamespaceWideScope {
				return fmt.Errorf("must provide the --name flag with --raw and --scope %s", flags.SealingScope.String())
			}
		}

		var data []byte
		if len(flags.FromFile) > 0 {
			if len(flags.FromFile) > 1 {
				return fmt.Errorf("must provide only one --from-file when encrypting a single item with --raw")
			}

			_, filename := kubeseal.ParseFromFile(flags.FromFile[0])
			// #nosec G304 -- should open user provided file
			data, err = os.ReadFile(filename)
		} else {
			if isatty.IsTerminal(os.Stdin.Fd()) {
				fmt.Fprintf(os.Stderr, "(tty detected: expecting a secret to encrypt in stdin)\n")
			}
			data, err = io.ReadAll(os.Stdin)
		}
		if err != nil {
			return err
		}

		return kubeseal.EncryptSecretItem(w, flags.SecretName, ns, data, flags.SealingScope, pubKey)
	}

	return kubeseal.Seal(cfg.solveNamespace, flags.OutputFormat, input, w, scheme.Codecs, pubKey, flags.SealingScope, flags.AllowEmptyData, flags.SecretName, "")
}

func mainE(w io.Writer, fs *flag.FlagSet, gofs *goflag.FlagSet, args []string) error {
	var flags Flags
	var printVersion bool
	var overrides clientcmd.ConfigOverrides
	buildinfo.FallbackVersion(&VERSION, buildinfo.DefaultVersion)

	fs.BoolVar(&printVersion, "version", false, "Print version information and exit")
	bindFlags(&flags, fs)
	bindClientFlags(fs, gofs, &overrides)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := gofs.Parse([]string{}); err != nil {
		return err
	}

	if printVersion {
		fmt.Fprintf(w, "kubeseal version: %s\n", VERSION)
		return nil
	}

	clientConfig := initClient(flags.Kubeconfig, &overrides, os.Stdout)
	cfg := newConfig(clientConfig, &flags)
	return runKubeseal(w, cfg)
}

func main() {
	if err := mainE(os.Stdout, flag.CommandLine, goflag.CommandLine, os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
