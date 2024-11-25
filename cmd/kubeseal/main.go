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

	// Register Auth providers.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

const (
	flagEnvPrefix = "SEALED_SECRETS"
)

var (
	// VERSION set from Makefile.
	VERSION = buildinfo.DefaultVersion
)

type cliFlags struct {
	certURL        string
	controllerNs   string
	controllerName string
	outputFormat   string
	outputFileName string
	inputFileName  string
	kubeconfig     string
	dumpCert       bool
	allowEmptyData bool
	validateSecret bool
	mergeInto      string
	raw            bool
	secretName     string
	fromFile       []string
	sealingScope   ssv1alpha1.SealingScope
	reEncrypt      bool
	unseal         bool
	privKeys       []string
	help           bool
}

type config struct {
	flags        *cliFlags
	clientConfig kubeseal.ClientConfig
	ctx          context.Context
}

func newConfig(clientConfig clientcmd.ClientConfig, flags *cliFlags) *config {
	return &config{
		flags:        flags,
		clientConfig: clientConfig,
		ctx:          context.Background(),
	}
}

func initClient(kubeConfigPath string, cfgOverrides *clientcmd.ConfigOverrides, r io.Reader) clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	loadingRules.ExplicitPath = kubeConfigPath
	return clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, cfgOverrides, r)
}

func bindFlags(f *cliFlags, fs *flag.FlagSet) {
	// TODO: Verify k8s server signature against cert in kube client config.
	fs.StringVar(&f.certURL, "cert", "", "Certificate / public key file/URL to use for encryption. Overrides --controller-*")
	fs.StringVar(&f.controllerNs, "controller-namespace", metav1.NamespaceSystem, "Namespace of sealed-secrets controller.")
	fs.StringVar(&f.controllerName, "controller-name", "sealed-secrets-controller", "Name of sealed-secrets controller.")
	fs.StringVarP(&f.outputFormat, "format", "o", "json", "Output format for sealed secret. Either json or yaml")
	fs.StringVarP(&f.outputFileName, "sealed-secret-file", "w", "", "Sealed-secret (output) file")
	fs.StringVarP(&f.inputFileName, "secret-file", "f", "", "Secret (input) file")
	fs.BoolVar(&f.dumpCert, "fetch-cert", false, "Write certificate to stdout. Useful for later use with --cert")
	fs.BoolVar(&f.allowEmptyData, "allow-empty-data", false, "Allow empty data in the secret object")
	fs.BoolVar(&f.validateSecret, "validate", false, "Validate that the sealed secret can be decrypted")
	fs.StringVar(&f.mergeInto, "merge-into", "", "Merge items from secret into an existing sealed secret file, updating the file in-place instead of writing to stdout.")
	fs.BoolVar(&f.raw, "raw", false, "Encrypt a raw value passed via the --from-* flags instead of the whole secret object")
	fs.StringVar(&f.secretName, "name", "", "Name of the sealed secret (required with --raw and default (strict) scope)")
	fs.StringSliceVar(&f.fromFile, "from-file", nil, "(only with --raw) Secret items can be sourced from files. Pro-tip: you can use /dev/stdin to read pipe input. This flag tries to follow the same syntax as in kubectl")
	fs.StringVar(&f.kubeconfig, "kubeconfig", "", "Path to a kube config. Only required if out-of-cluster")

	fs.Var(&f.sealingScope, "scope", "Set the scope of the sealed secret: strict, namespace-wide, cluster-wide (defaults to strict). Mandatory for --raw, otherwise the 'sealedsecrets.bitnami.com/cluster-wide' and 'sealedsecrets.bitnami.com/namespace-wide' annotations on the input secret can be used to select the scope.")
	fs.BoolVar(&f.reEncrypt, "rotate", false, "")
	fs.BoolVar(&f.reEncrypt, "re-encrypt", false, "Re-encrypt the given sealed secret to use the latest cluster key.")
	_ = fs.MarkDeprecated("rotate", "please use --re-encrypt instead")

	fs.BoolVar(&f.unseal, "recovery-unseal", false, "Decrypt a sealed secrets file obtained from stdin, using the private key passed with --recovery-private-key. Intended to be used in disaster recovery mode.")
	fs.StringSliceVar(&f.privKeys, "recovery-private-key", nil, "Private key filename used by the --recovery-unseal command. Multiple files accepted either via comma separated list or by repetition of the flag. Either PEM encoded private keys or a backup of a json/yaml encoded k8s sealed-secret controller secret (and v1.List) are accepted. ")
	fs.BoolVar(&f.help, "help", false, "Print this help message")

	fs.SetOutput(os.Stdout)
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

func runCLI(w io.Writer, cfg *config) (err error) {
	flags := cfg.flags

	if flags.help {
		fmt.Fprintf(os.Stdout, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		return nil
	}

	if len(flags.fromFile) != 0 && !flags.raw {
		return fmt.Errorf("--from-file requires --raw")
	}

	var input io.Reader = os.Stdin
	if flags.inputFileName != "" {
		// #nosec G304 -- should open user provided file
		f, err := os.Open(flags.inputFileName)
		if err != nil {
			return fmt.Errorf("Could not read file specified with --secret-file")
		}
		// #nosec: G307 -- this deferred close is fine because it is not on a writable file
		defer f.Close()

		input = f
	} else if !flags.raw && !flags.dumpCert {
		if isatty.IsTerminal(os.Stdin.Fd()) {
			fmt.Fprintf(os.Stderr, "(tty detected: expecting json/yaml k8s resource in stdin)\n")
		}
	}

	// reEncrypt is the only "in-place" update subcommand. When the user only provides one file (the input file)
	// we'll use the same file for output (see #405).
	if flags.reEncrypt && (flags.outputFileName == "" && flags.inputFileName != "") {
		flags.outputFileName = flags.inputFileName
	}
	if flags.outputFileName != "" {
		if ext := filepath.Ext(flags.outputFileName); ext == ".yaml" || ext == ".yml" {
			flags.outputFormat = "yaml"
		}

		var f *renameio.PendingFile
		f, err = renameio.TempFile("", flags.outputFileName)
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

	if flags.unseal {
		return kubeseal.UnsealSealedSecret(w, input, flags.privKeys, flags.outputFormat, scheme.Codecs)
	}
	if len(flags.privKeys) != 0 && isatty.IsTerminal(os.Stderr.Fd()) {
		fmt.Fprintf(os.Stderr, "warning: ignoring --recovery-private-key because unseal command not chosen with --recovery-unseal\n")
	}

	if flags.validateSecret {
		return kubeseal.ValidateSealedSecret(cfg.ctx, cfg.clientConfig, flags.controllerNs, flags.controllerName, input)
	}

	if flags.reEncrypt {
		return kubeseal.ReEncryptSealedSecret(cfg.ctx, cfg.clientConfig, flags.controllerNs, flags.controllerName, flags.outputFormat, input, w, scheme.Codecs)
	}

	f, err := kubeseal.OpenCert(cfg.ctx, cfg.clientConfig, flags.controllerNs, flags.controllerName, flags.certURL)
	if err != nil {
		return err
	}
	// #nosec: G307 -- this deferred close is fine because it is not on a writable file
	defer f.Close()

	if flags.dumpCert {
		_, err := io.Copy(w, f)
		return err
	}

	pubKey, err := kubeseal.ParseKey(f)
	if err != nil {
		return err
	}

	if flags.mergeInto != "" {
		return kubeseal.SealMergingInto(cfg.clientConfig, flags.outputFormat, input, flags.mergeInto, scheme.Codecs, pubKey, flags.sealingScope, flags.allowEmptyData)
	}

	if flags.raw {
		var (
			ns  string
			err error
		)
		if flags.sealingScope < ssv1alpha1.ClusterWideScope {
			ns, _, err = cfg.clientConfig.Namespace()
			if err != nil {
				return err
			}

			if ns == "" {
				return fmt.Errorf("must provide the --namespace flag with --raw and --scope %s", flags.sealingScope.String())
			}

			if flags.secretName == "" && flags.sealingScope < ssv1alpha1.NamespaceWideScope {
				return fmt.Errorf("must provide the --name flag with --raw and --scope %s", flags.sealingScope.String())
			}
		}

		var data []byte
		if len(flags.fromFile) > 0 {
			if len(flags.fromFile) > 1 {
				return fmt.Errorf("must provide only one --from-file when encrypting a single item with --raw")
			}

			_, filename := kubeseal.ParseFromFile(flags.fromFile[0])
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

		return kubeseal.EncryptSecretItem(w, flags.secretName, ns, data, flags.sealingScope, pubKey)
	}

	return kubeseal.Seal(cfg.clientConfig, flags.outputFormat, input, w, scheme.Codecs, pubKey, flags.sealingScope, flags.allowEmptyData, flags.secretName, "")
}

func mainE(w io.Writer, fs *flag.FlagSet, gofs *goflag.FlagSet, args []string) error {
	var flags cliFlags
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

	clientConfig := initClient(flags.kubeconfig, &overrides, os.Stdout)
	cfg := newConfig(clientConfig, &flags)
	return runCLI(w, cfg)
}

func main() {
	if err := mainE(os.Stdout, flag.CommandLine, goflag.CommandLine, os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
