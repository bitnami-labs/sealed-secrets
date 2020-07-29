package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	goflag "flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitnami-labs/sealed-secrets/pkg/buildinfo"
	"github.com/bitnami-labs/sealed-secrets/pkg/crypto"
	"github.com/google/renameio"
	"github.com/mattn/go-isatty"
	flag "github.com/spf13/pflag"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
	"k8s.io/klog"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealed-secrets/v1alpha1"

	// Register Auth providers
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/bitnami-labs/flagenv"
	"github.com/bitnami-labs/pflagenv"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	flagEnvPrefix = "SEALED_SECRETS"
)

var (
	// VERSION set from Makefile
	VERSION = buildinfo.DefaultVersion

	clientConfig clientcmd.ClientConfig

	// testing hook for clientConfig.Namespace()
	namespaceFromClientConfig = func() (string, bool, error) { return clientConfig.Namespace() }
)

type Flags struct {
	certURL        string
	controllerNs   string
	controllerName string
	outputFormat   string
	outputFileName string
	inputFileName  string
	dumpCert       bool
	allowEmptyData bool
	printVersion   bool
	validateSecret bool
	mergeInto      string
	raw            bool
	secretName     string
	fromFile       []string
	sealingScope   ssv1alpha1.SealingScope
	reEncrypt      bool
	unseal         bool
	privKeys       []string
}

func (f *Flags) Bind(fs *flag.FlagSet) {
	if fs == nil {
		fs = flag.CommandLine
	}

	// TODO: Verify k8s server signature against cert in kube client config.
	fs.StringVar(&f.certURL, "cert", "", "Certificate / public key file/URL to use for encryption. Overrides --controller-*")
	fs.StringVar(&f.controllerNs, "controller-namespace", metav1.NamespaceSystem, "Namespace of sealed-secrets controller.")
	fs.StringVar(&f.controllerName, "controller-name", "sealed-secrets-controller", "Name of sealed-secrets controller.")
	fs.StringVarP(&f.outputFormat, "format", "o", "json", "Output format for sealed secret. Either json or yaml")
	fs.StringVarP(&f.outputFileName, "sealed-secret-file", "w", "", "Sealed-secret (output) file")
	fs.StringVarP(&f.inputFileName, "secret-file", "f", "", "Secret (input) file")
	fs.BoolVar(&f.dumpCert, "fetch-cert", false, "Write certificate to stdout. Useful for later use with --cert")
	fs.BoolVar(&f.allowEmptyData, "allow-empty-data", false, "Allow empty data in the secret object")
	fs.BoolVar(&f.printVersion, "version", false, "Print version information and exit")
	fs.BoolVar(&f.validateSecret, "validate", false, "Validate that the sealed secret can be decrypted")
	fs.StringVar(&f.mergeInto, "merge-into", "", "Merge items from secret into an existing sealed secret file, updating the file in-place instead of writing to stdout.")
	fs.BoolVar(&f.raw, "raw", false, "Encrypt a raw value passed via the --from-* flags instead of the whole secret object")
	fs.StringVar(&f.secretName, "name", "", "Name of the sealed secret (required with --raw and default (strict) scope)")
	fs.StringSliceVar(&f.fromFile, "from-file", nil, "(only with --raw) Secret items can be sourced from files. Pro-tip: you can use /dev/stdin to read pipe input. This flag tries to follow the same syntax as in kubectl")

	fs.Var(&f.sealingScope, "scope", "Set the scope of the sealed secret: strict, namespace-wide, cluster-wide (defaults to strict). Mandatory for --raw, otherwise the 'sealedsecrets.bitnami.com/cluster-wide' and 'sealedsecrets.bitnami.com/namespace-wide' annotations on the input secret can be used to select the scope.")
	fs.BoolVar(&f.reEncrypt, "rotate", false, "")
	fs.BoolVar(&f.reEncrypt, "re-encrypt", false, "Re-encrypt the given sealed secret to use the latest cluster key.")
	flag.CommandLine.MarkDeprecated("rotate", "please use --re-encrypt instead")

	fs.BoolVar(&f.unseal, "recovery-unseal", false, "Decrypt a sealed secrets file obtained from stdin, using the private key passed with --recovery-private-key. Intended to be used in disaster recovery mode.")
	fs.StringSliceVar(&f.privKeys, "recovery-private-key", nil, "Private key filename used by the --recovery-unseal command. Multiple files accepted either via comma separated list or by repetition of the flag. Either PEM encoded private keys or a backup of a json/yaml encoded k8s sealed-secret controller secret (and v1.List) are accepted. ")
}

func init() {
	buildinfo.FallbackVersion(&VERSION, buildinfo.DefaultVersion)

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

func parseKey(r io.Reader) (*rsa.PublicKey, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	certs, err := cert.ParseCertsPEM(data)
	if err != nil {
		return nil, err
	}

	// ParseCertsPem returns error if len(certs) == 0, but best to be sure...
	if len(certs) == 0 {
		return nil, errors.New("Failed to read any certificates")
	}

	cert, ok := certs[0].PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("Expected RSA public key but found %v", certs[0].PublicKey)
	}

	return cert, nil
}

func readSecret(codec runtime.Decoder, r io.Reader) (*v1.Secret, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var ret v1.Secret
	if err = runtime.DecodeInto(codec, data, &ret); err != nil {
		return nil, err
	}

	return &ret, nil
}

func prettyEncoder(codecs runtimeserializer.CodecFactory, mediaType string, gv runtime.GroupVersioner) (runtime.Encoder, error) {
	info, ok := runtime.SerializerInfoForMediaType(codecs.SupportedMediaTypes(), mediaType)
	if !ok {
		return nil, fmt.Errorf("binary can't serialize %s", mediaType)
	}

	prettyEncoder := info.PrettySerializer
	if prettyEncoder == nil {
		prettyEncoder = info.Serializer
	}

	enc := codecs.EncoderForVersion(prettyEncoder, gv)
	return enc, nil
}

func isFilename(name string) (bool, error) {
	u, err := url.Parse(name)
	if err != nil {
		return false, err
	}
	return u.Scheme == "", nil
}

// openCertLocal opens a cert URI or local filename, by fetching it locally from the client
// (as opposed as openCertCluster which fetches it via HTTP but through the k8s API proxy).
func openCertLocal(filenameOrURI string) (io.ReadCloser, error) {
	// detect if a certificate is a local file or an URI.
	if ok, err := isFilename(filenameOrURI); err != nil {
		return nil, err
	} else if ok {
		return os.Open(filenameOrURI)
	}
	return openCertURI(filenameOrURI)
}

func openCertURI(uri string) (io.ReadCloser, error) {
	// support file:// scheme. Note: we're opening the file using os.Open rather
	// than using the file:// scheme below because there is no point in complicating our lives
	// and escape the filename properly.

	t := &http.Transport{}
	t.RegisterProtocol("file", http.NewFileTransport(http.Dir("/")))
	c := &http.Client{Transport: t}

	resp, err := c.Get(uri)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cannot fetch %q: %s", uri, resp.Status)
	}
	return resp.Body, nil
}

// openCertCluster fetches a certificate by performing an HTTP request to the controller
// through the k8s API proxy.
func openCertCluster(c corev1.CoreV1Interface, namespace, name string) (io.ReadCloser, error) {
	f, err := c.
		Services(namespace).
		ProxyGet("http", name, "", "/v1/cert.pem", nil).
		Stream()
	if err != nil {
		return nil, fmt.Errorf("cannot fetch certificate: %v", err)
	}
	return f, nil
}

func openCert(flags Flags, certURL string) (io.ReadCloser, error) {
	if certURL != "" {
		return openCertLocal(certURL)
	}

	conf, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	conf.AcceptContentTypes = "application/x-pem-file, */*"
	restClient, err := corev1.NewForConfig(conf)
	if err != nil {
		return nil, err
	}
	return openCertCluster(restClient, flags.controllerNs, flags.controllerName)
}

// Seal reads a k8s Secret resource parsed from an input reader by a given codec, encrypts all its secrets
// with a given public key, using the name and namespace found in the input secret, unless explicitly overridden
// by the overrideName and overrideNamespace arguments.
func seal(flags Flags, in io.Reader, out io.Writer, codecs runtimeserializer.CodecFactory, pubKey *rsa.PublicKey, scope ssv1alpha1.SealingScope, allowEmptyData bool, overrideName, overrideNamespace string) error {
	secret, err := readSecret(codecs.UniversalDecoder(), in)
	if err != nil {
		return err
	}

	if len(secret.Data) == 0 && len(secret.StringData) == 0 && !allowEmptyData {
		return fmt.Errorf("Secret.data is empty in input Secret, assuming this is an error and aborting. To work with empty data, --allow-empty-data can be used.")
	}

	if overrideName != "" {
		secret.Name = overrideName
	}

	if secret.GetName() == "" {
		return fmt.Errorf("Missing metadata.name in input Secret")
	}

	if overrideNamespace != "" {
		secret.Namespace = overrideNamespace
	}

	if scope != ssv1alpha1.DefaultScope {
		secret.Annotations = ssv1alpha1.UpdateScopeAnnotations(secret.Annotations, scope)
	}

	if ssv1alpha1.SecretScope(secret) != ssv1alpha1.ClusterWideScope && secret.GetNamespace() == "" {
		ns, _, err := namespaceFromClientConfig()
		if clientcmd.IsEmptyConfig(err) {
			return fmt.Errorf("input secret has no namespace and cannot infer the namespace automatically when no kube config is available")
		} else if err != nil {
			return err
		}
		secret.SetNamespace(ns)
	}

	// Strip read-only server-side ObjectMeta (if present)
	secret.SetSelfLink("")
	secret.SetUID("")
	secret.SetResourceVersion("")
	secret.Generation = 0
	secret.SetCreationTimestamp(metav1.Time{})
	secret.SetDeletionTimestamp(nil)
	secret.DeletionGracePeriodSeconds = nil

	ssecret, err := ssv1alpha1.NewSealedSecret(codecs, pubKey, secret)
	if err != nil {
		return err
	}
	if err = sealedSecretOutput(out, flags, codecs, ssecret); err != nil {
		return err
	}
	return nil
}

func validateSealedSecret(in io.Reader, namespace, name string) error {
	conf, err := clientConfig.ClientConfig()
	if err != nil {
		return err
	}
	restClient, err := corev1.NewForConfig(conf)
	if err != nil {
		return err
	}

	content, err := ioutil.ReadAll(in)
	if err != nil {
		return err
	}

	req := restClient.RESTClient().Post().
		Namespace(namespace).
		Resource("services").
		SubResource("proxy").
		Name(net.JoinSchemeNamePort("http", name, "")).
		Suffix("/v1/verify")

	req.Body(content)
	res := req.Do()
	if err := res.Error(); err != nil {
		if status, ok := err.(*k8serrors.StatusError); ok && status.Status().Code == http.StatusConflict {
			return fmt.Errorf("unable to decrypt sealed secret")
		}
		return fmt.Errorf("cannot validate sealed secret: %v", err)
	}

	return nil
}

func reEncryptSealedSecret(flags Flags, in io.Reader, out io.Writer, codecs runtimeserializer.CodecFactory, namespace, name string) error {
	conf, err := clientConfig.ClientConfig()
	if err != nil {
		return err
	}
	restClient, err := corev1.NewForConfig(conf)
	if err != nil {
		return err
	}

	content, err := ioutil.ReadAll(in)
	if err != nil {
		return err
	}

	req := restClient.RESTClient().Post().
		Namespace(namespace).
		Resource("services").
		SubResource("proxy").
		Name(net.JoinSchemeNamePort("http", name, "")).
		Suffix("/v1/rotate")

	req.Body(content)
	res := req.Do()
	if err := res.Error(); err != nil {
		if status, ok := err.(*k8serrors.StatusError); ok && status.Status().Code == http.StatusConflict {
			return fmt.Errorf("unable to rotate secret")
		}
		return fmt.Errorf("cannot re-encrypt secret: %v", err)
	}
	body, err := res.Raw()
	if err != nil {
		return err
	}
	ssecret := &ssv1alpha1.SealedSecret{}
	if err = json.Unmarshal(body, ssecret); err != nil {
		return err
	}
	ssecret.SetCreationTimestamp(metav1.Time{})
	ssecret.SetDeletionTimestamp(nil)
	ssecret.Generation = 0
	if err = sealedSecretOutput(out, flags, codecs, ssecret); err != nil {
		return err
	}
	return nil
}

func resourceOutput(out io.Writer, flags Flags, codecs runtimeserializer.CodecFactory, gv runtime.GroupVersioner, obj runtime.Object) error {
	var contentType string
	switch strings.ToLower(flags.outputFormat) {
	case "json", "":
		contentType = runtime.ContentTypeJSON
	case "yaml":
		contentType = "application/yaml"
	default:
		return fmt.Errorf("unsupported output format: %s", flags.outputFormat)
	}
	prettyEnc, err := prettyEncoder(codecs, contentType, gv)
	if err != nil {
		return err
	}
	buf, err := runtime.Encode(prettyEnc, obj)
	if err != nil {
		return err
	}
	out.Write(buf)
	fmt.Fprint(out, "\n")
	return nil
}

func sealedSecretOutput(out io.Writer, flags Flags, codecs runtimeserializer.CodecFactory, ssecret *ssv1alpha1.SealedSecret) error {
	return resourceOutput(out, flags, codecs, ssv1alpha1.SchemeGroupVersion, ssecret)
}

func decodeSealedSecret(codecs runtimeserializer.CodecFactory, b []byte) (*ssv1alpha1.SealedSecret, error) {
	var ss ssv1alpha1.SealedSecret
	if err := runtime.DecodeInto(codecs.UniversalDecoder(), b, &ss); err != nil {
		return nil, err
	}
	return &ss, nil
}

func sealMergingInto(flags Flags, in io.Reader, filename string, codecs runtimeserializer.CodecFactory, pubKey *rsa.PublicKey, scope ssv1alpha1.SealingScope, allowEmptyData bool) error {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	orig, err := decodeSealedSecret(codecs, b)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := seal(flags, in, &buf, codecs, pubKey, scope, allowEmptyData, orig.Name, orig.Namespace); err != nil {
		return err
	}

	update, err := decodeSealedSecret(codecs, buf.Bytes())
	if err != nil {
		return err
	}

	// merge encrypted data and metadata
	for k, v := range update.Spec.EncryptedData {
		orig.Spec.EncryptedData[k] = v
	}
	for k, v := range update.Spec.Template.Annotations {
		orig.Spec.Template.Annotations[k] = v
	}
	for k, v := range update.Spec.Template.Labels {
		orig.Spec.Template.Labels[k] = v
	}

	// updated sealed secret file in-place avoiding clobbering the file upon rendering errors.
	var out bytes.Buffer
	if err := sealedSecretOutput(&out, flags, codecs, orig); err != nil {
		return err
	}
	// On windows the permission bits are used also when truncating existing files
	// (see https://github.com/golang/go/issues/38225)
	// Thus we need to set some reasonable permissions.
	// The actual permissions will be filtered by the user's umask.
	// We still drop any permissions to the other group because despite the sealed secret
	// being encrypted, we still don't know how the end users feel about it.
	return ioutil.WriteFile(filename, out.Bytes(), 0660)
}

func encryptSecretItem(w io.Writer, secretName, ns string, data []byte, scope ssv1alpha1.SealingScope, pubKey *rsa.PublicKey) error {
	// TODO(mkm): refactor cluster-wide/namespace-wide to an actual enum so we can have a simple flag
	// to refer to the scope mode that is not a tuple of booleans.
	label := ssv1alpha1.EncryptionLabel(ns, secretName, scope)
	out, err := crypto.HybridEncrypt(rand.Reader, pubKey, data, label)
	if err != nil {
		return err
	}
	fmt.Fprint(w, base64.StdEncoding.EncodeToString(out))
	return nil
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

func readPrivKeysFromFile(filename string) ([]*rsa.PrivateKey, error) {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	res, err := parsePrivKey(b)
	if err == nil {
		return []*rsa.PrivateKey{res}, nil
	}

	var secrets []*v1.Secret

	// try to parse it as json/yaml encoded v1.List of secrets
	var lst v1.List
	if err = runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), b, &lst); err == nil {
		for _, r := range lst.Items {
			s, err := readSecret(scheme.Codecs.UniversalDecoder(), bytes.NewBuffer(r.Raw))
			if err != nil {
				return nil, err
			}
			secrets = append(secrets, s)
		}
	} else {
		// try to parse it as json/yaml encoded secret
		s, err := readSecret(scheme.Codecs.UniversalDecoder(), bytes.NewBuffer(b))
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, s)
	}

	var keys []*rsa.PrivateKey
	for _, s := range secrets {
		tlsKey, ok := s.Data["tls.key"]
		if !ok {
			return nil, fmt.Errorf("secret must contain a 'tls.data' key")
		}
		pk, err := parsePrivKey(tlsKey)
		if err != nil {
			return nil, err
		}
		keys = append(keys, pk)
	}

	return keys, nil
}

func readPrivKey(filename string) (*rsa.PrivateKey, error) {
	pks, err := readPrivKeysFromFile(filename)
	if err != nil {
		return nil, err
	}
	return pks[0], nil
}

func parsePrivKey(b []byte) (*rsa.PrivateKey, error) {
	key, err := keyutil.ParsePrivateKeyPEM(b)
	if err != nil {
		return nil, err
	}
	switch rsaKey := key.(type) {
	case *rsa.PrivateKey:
		return rsaKey, nil
	default:
		return nil, fmt.Errorf("unexpected private key type %T", key)
	}
}

func readPrivKeys(filenames []string) (map[string]*rsa.PrivateKey, error) {
	res := map[string]*rsa.PrivateKey{}
	for _, filename := range filenames {
		pks, err := readPrivKeysFromFile(filename)
		if err != nil {
			return nil, err
		}
		for _, pk := range pks {
			fingerprint, err := crypto.PublicKeyFingerprint(&pk.PublicKey)
			if err != nil {
				return nil, err
			}

			res[fingerprint] = pk
		}
	}
	return res, nil
}

func unsealSealedSecret(flags Flags, w io.Writer, in io.Reader, codecs runtimeserializer.CodecFactory, privKeyFilenames []string) error {
	privKeys, err := readPrivKeys(privKeyFilenames)
	if err != nil {
		return err
	}

	b, err := ioutil.ReadAll(in)
	if err != nil {
		return err
	}

	ss, err := decodeSealedSecret(codecs, b)
	if err != nil {
		return err
	}
	sec, err := ss.Unseal(codecs, privKeys)
	if err != nil {
		return err
	}

	return resourceOutput(w, flags, codecs, v1.SchemeGroupVersion, sec)
}

func run(w io.Writer, flags Flags) (err error) {
	if len(flags.fromFile) != 0 && !flags.raw {
		return fmt.Errorf("--from-file requires --raw")
	}

	if flags.printVersion {
		fmt.Fprintf(w, "kubeseal version: %s\n", VERSION)
		return nil
	}

	var input io.Reader = os.Stdin
	if flags.inputFileName != "" {
		f, err := os.Open(flags.inputFileName)
		if err != nil {
			return nil
		}
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
				f.CloseAtomicallyReplace()
			}
		}()

		w = f
	}

	if flags.unseal {
		return unsealSealedSecret(flags, w, input, scheme.Codecs, flags.privKeys)
	}
	if len(flags.privKeys) != 0 && isatty.IsTerminal(os.Stderr.Fd()) {
		fmt.Fprintf(os.Stderr, "warning: ignoring --recovery-private-key because unseal command not chosen with --recovery-unseal\n")
	}

	if flags.validateSecret {
		return validateSealedSecret(input, flags.controllerNs, flags.controllerName)
	}

	if flags.reEncrypt {
		return reEncryptSealedSecret(flags, input, w, scheme.Codecs, flags.controllerNs, flags.controllerName)
	}

	f, err := openCert(flags, flags.certURL)
	if err != nil {
		return err
	}
	defer f.Close()

	if flags.dumpCert {
		_, err := io.Copy(w, f)
		return err
	}

	pubKey, err := parseKey(f)
	if err != nil {
		return err
	}

	if flags.mergeInto != "" {
		return sealMergingInto(flags, input, flags.mergeInto, scheme.Codecs, pubKey, flags.sealingScope, flags.allowEmptyData)
	}

	if flags.raw {
		ns, _, err := namespaceFromClientConfig()
		if err != nil {
			return err
		}

		if ns == "" && flags.sealingScope < ssv1alpha1.ClusterWideScope {
			return fmt.Errorf("must provide the --namespace flag with --raw and --scope %s", flags.sealingScope.String())
		}

		if flags.secretName == "" && flags.sealingScope < ssv1alpha1.NamespaceWideScope {
			return fmt.Errorf("must provide the --name flag with --raw and --scope %s", flags.sealingScope.String())
		}

		var data []byte
		if len(flags.fromFile) > 0 {
			if len(flags.fromFile) > 1 {
				return fmt.Errorf("must provide only one --from-file when encrypting a single item with --raw")
			}

			_, filename := parseFromFile(flags.fromFile[0])
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

		return encryptSecretItem(w, flags.secretName, ns, data, flags.sealingScope, pubKey)
	}

	return seal(flags, input, w, scheme.Codecs, pubKey, flags.sealingScope, flags.allowEmptyData, flags.secretName, "")
}

func main() {
	var flags Flags
	flags.Bind(nil)

	flag.Parse()
	goflag.CommandLine.Parse([]string{})

	if err := run(os.Stdout, flags); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
