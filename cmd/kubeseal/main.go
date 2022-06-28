package main

import (
	"context"
	goflag "flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/renameio"
	"github.com/mattn/go-isatty"
	flag "github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/bitnami-labs/sealed-secrets/pkg/buildinfo"
	"github.com/bitnami-labs/sealed-secrets/pkg/kubeseal"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealed-secrets/v1alpha1"

	// Register Auth providers
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/bitnami-labs/sealed-secrets/pkg/flagenv"
	"github.com/bitnami-labs/sealed-secrets/pkg/pflagenv"
)

const (
	flagEnvPrefix = "SEALED_SECRETS"
)

var (
	// TODO: Verify k8s server signature against cert in kube client config.
	certURL        = flag.String("cert", "", "Certificate / public key file/URL to use for encryption. Overrides --controller-*")
	controllerNs   = flag.String("controller-namespace", metav1.NamespaceSystem, "Namespace of sealed-secrets controller.")
	controllerName = flag.String("controller-name", "sealed-secrets-controller", "Name of sealed-secrets controller.")
	outputFormat   = flag.StringP("format", "o", "json", "Output format for sealed secret. Either json or yaml")
	outputFileName = flag.StringP("sealed-secret-file", "w", "", "Sealed-secret (output) file")
	inputFileName  = flag.StringP("secret-file", "f", "", "Secret (input) file")
	dumpCert       = flag.Bool("fetch-cert", false, "Write certificate to stdout. Useful for later use with --cert")
	allowEmptyData = flag.Bool("allow-empty-data", false, "Allow empty data in the secret object")
	printVersion   = flag.Bool("version", false, "Print version information and exit")
	validateSecret = flag.Bool("validate", false, "Validate that the sealed secret can be decrypted")
	mergeInto      = flag.String("merge-into", "", "Merge items from secret into an existing sealed secret file, updating the file in-place instead of writing to stdout.")
	raw            = flag.Bool("raw", false, "Encrypt a raw value passed via the --from-* flags instead of the whole secret object")
	secretName     = flag.String("name", "", "Name of the sealed secret (required with --raw and default (strict) scope)")
	fromFile       = flag.StringSlice("from-file", nil, "(only with --raw) Secret items can be sourced from files. Pro-tip: you can use /dev/stdin to read pipe input. This flag tries to follow the same syntax as in kubectl")
	sealingScope   ssv1alpha1.SealingScope
	reEncrypt      bool // re-encrypt command
	unseal         = flag.Bool("recovery-unseal", false, "Decrypt a sealed secrets file obtained from stdin, using the private key passed with --recovery-private-key. Intended to be used in disaster recovery mode.")
	privKeys       = flag.StringSlice("recovery-private-key", nil, "Private key filename used by the --recovery-unseal command. Multiple files accepted either via comma separated list or by repetition of the flag. Either PEM encoded private keys or a backup of a json/yaml encoded k8s sealed-secret controller secret (and v1.List) are accepted. ")

	// VERSION set from Makefile
	VERSION = buildinfo.DefaultVersion

	clientConfig clientcmd.ClientConfig

	// testing hook for clientConfig.Namespace()
	namespaceFromClientConfig = func() (string, bool, error) { return clientConfig.Namespace() }
)

func init() {
	buildinfo.FallbackVersion(&VERSION, buildinfo.DefaultVersion)

	flag.Var(&sealingScope, "scope", "Set the scope of the sealed secret: strict, namespace-wide, cluster-wide (defaults to strict). Mandatory for --raw, otherwise the 'sealedsecrets.bitnami.com/cluster-wide' and 'sealedsecrets.bitnami.com/namespace-wide' annotations on the input secret can be used to select the scope.")
	flag.BoolVar(&reEncrypt, "rotate", false, "")
	flag.BoolVar(&reEncrypt, "re-encrypt", false, "Re-encrypt the given sealed secret to use the latest cluster key.")
	_ = flag.CommandLine.MarkDeprecated("rotate", "please use --re-encrypt instead")

	flagenv.SetFlagsFromEnv(flagEnvPrefix, goflag.CommandLine)

	// The "usual" clientcmd/kubectl flags
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	overrides := clientcmd.ConfigOverrides{}
	kflags := clientcmd.RecommendedConfigOverrideFlags("")
	flag.StringVar(&loadingRules.ExplicitPath, "kubeconfig", "", "Path to a kube config. Only required if out-of-cluster")
	clientcmd.BindOverrideFlags(&overrides, flag.CommandLine, kflags)
	clientConfig = clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, &overrides, os.Stdin)

	pflagenv.SetFlagsFromEnv(flagEnvPrefix, flag.CommandLine)

	// add klog flags to goflags flagset
	klog.InitFlags(nil)
	// Standard goflags (glog in particular)
	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
}

// parseFromFile parses a value of the kubectl --from-file flag, which can optionally include an item name
// preceding the first equals sign.
func parseFromFile(s string) (string, string) {
	c := strings.SplitN(s, "=", 2)
	if len(c) == 1 {
		return "", c[0]
	}
	return c[0], c[1]
}

func run(ctx context.Context, w io.Writer, inputFileName, outputFileName, secretName, controllerNs, controllerName, certURL string, printVersion, validateSecret, reEncrypt, dumpCert, raw, allowEmptyData bool, fromFile []string, mergeInto string, unseal bool, privKeys []string) (err error) {
	if len(fromFile) != 0 && !raw {
		return fmt.Errorf("--from-file requires --raw")
	}

	if printVersion {
		fmt.Fprintf(w, "kubeseal version: %s\n", VERSION)
		return nil
	}

	var input io.Reader = os.Stdin
	if inputFileName != "" {
		// #nosec G304 -- should open user provided file
		f, err := os.Open(inputFileName)
		if err != nil {
			return nil
		}
		// #nosec: G307 -- this deferred close is fine because it is not on a writable file
		defer f.Close()

		input = f
	} else if !raw && !dumpCert {
		if isatty.IsTerminal(os.Stdin.Fd()) {
			fmt.Fprintf(os.Stderr, "(tty detected: expecting json/yaml k8s resource in stdin)\n")
		}
	}
	// reEncrypt is the only "in-place" update subcommand. When the user only provides one file (the input file)
	// we'll use the same file for output (see #405).
	if reEncrypt && (outputFileName == "" && inputFileName != "") {
		outputFileName = inputFileName
	}
	if outputFileName != "" {
		// TODO(mkm): get rid of these horrible global variables
		if ext := filepath.Ext(outputFileName); ext == ".yaml" || ext == ".yml" {
			*outputFormat = "yaml"
		}

		var f *renameio.PendingFile
		f, err = renameio.TempFile("", outputFileName)
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
	if unseal {
		return kubeseal.Unseal(kubeseal.UnsealSealedSecretInstruction{
			OutputFormat:     *outputFormat,
			Out:              w,
			In:               input,
			Codecs:           scheme.Codecs,
			PrivKeyFilenames: privKeys,
		})
	}
	if len(privKeys) != 0 && isatty.IsTerminal(os.Stderr.Fd()) {
		fmt.Fprintf(os.Stderr, "warning: ignoring --recovery-private-key because unseal command not chosen with --recovery-unseal\n")
	}

	if validateSecret {
		return kubeseal.ValidateSealedSecret(kubeseal.ValidateSealedSecretInstruction{
			Ctx:       ctx,
			In:        input,
			Namespace: controllerNs,
			Name:      controllerName,
		})
	}

	if reEncrypt {
		return kubeseal.ReEncryptSealedSecret(kubeseal.ReEncryptSealedSecretInstruction{
			OutputFormat: *outputFormat,
			Ctx:          ctx,
			In:           input,
			Out:          w,
			Codecs:       scheme.Codecs,
			Namespace:    controllerNs,
			Name:         controllerName,
		})
	}

	f, err := kubeseal.OpenCert(ctx, clientConfig, certURL, controllerNs, controllerName)
	if err != nil {
		return err
	}
	defer f.Close()

	if dumpCert {
		_, err := io.Copy(w, f)
		return err
	}

	pubKey, err := kubeseal.ParseKey(f)
	if err != nil {
		return err
	}

	if mergeInto != "" {
		return kubeseal.SealMergingInto(kubeseal.SealMergeIntoInstruction{
			OutputFormat:   *outputFormat,
			In:             input,
			Filename:       mergeInto,
			Codecs:         scheme.Codecs,
			PubKey:         pubKey,
			Scope:          sealingScope,
			AllowEmptyData: allowEmptyData,
		})
	}

	if raw {
		var (
			ns  string
			err error
		)

		if sealingScope < ssv1alpha1.ClusterWideScope {
			ns, _, err = namespaceFromClientConfig()
			if err != nil {
				return err
			}

			if ns == "" {
				return fmt.Errorf("must provide the --namespace flag with --raw and --scope %s", sealingScope.String())
			}

			if secretName == "" && sealingScope < ssv1alpha1.NamespaceWideScope {
				return fmt.Errorf("must provide the --name flag with --raw and --scope %s", sealingScope.String())
			}
		}

		var data []byte
		if len(fromFile) > 0 {
			if len(fromFile) > 1 {
				return fmt.Errorf("must provide only one --from-file when encrypting a single item with --raw")
			}

			_, filename := parseFromFile(fromFile[0])
			// #nosec G304 -- should open user provided file
			data, err = ioutil.ReadFile(filename)
		} else {
			if isatty.IsTerminal(os.Stdin.Fd()) {
				fmt.Fprintf(os.Stderr, "(tty detected: expecting a secret to encrypt in stdin)\n")
			}
			data, err = ioutil.ReadAll(os.Stdin)
		}
		if err != nil {
			return err
		}
		return kubeseal.EncryptSecretItem(kubeseal.SecretItemEncryptionInstruction{
			Out:        w,
			SecretName: secretName,
			Namespace:  ns,
			Data:       data,
			Scope:      sealingScope,
			PubKey:     pubKey,
		})
	}

	return kubeseal.Seal(kubeseal.SealInstruction{
		OutputFormat:   *outputFormat,
		In:             input,
		Out:            w,
		Codecs:         scheme.Codecs,
		PubKey:         pubKey,
		Scope:          sealingScope,
		AllowEmptyData: allowEmptyData,
		OverrideName:   secretName,
	})
}

func main() {
	flag.Parse()
	_ = goflag.CommandLine.Parse([]string{})

	if err := run(context.Background(), os.Stdout, *inputFileName, *outputFileName, *secretName, *controllerNs, *controllerName, *certURL, *printVersion, *validateSecret, reEncrypt, *dumpCert, *raw, *allowEmptyData, *fromFile, *mergeInto, *unseal, *privKeys); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
