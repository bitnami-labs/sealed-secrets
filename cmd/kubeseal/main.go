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
	"os"
	"strings"

	"github.com/bitnami-labs/sealed-secrets/pkg/crypto"
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
	// TODO: Verify k8s server signature against cert in kube client config.
	certFile       = flag.String("cert", "", "Certificate / public key to use for encryption. Overrides --controller-*")
	controllerNs   = flag.String("controller-namespace", metav1.NamespaceSystem, "Namespace of sealed-secrets controller.")
	controllerName = flag.String("controller-name", "sealed-secrets-controller", "Name of sealed-secrets controller.")
	outputFormat   = flag.StringP("format","o", "json", "Output format for sealed secret. Either json or yaml")
	dumpCert       = flag.Bool("fetch-cert", false, "Write certificate to stdout. Useful for later use with --cert")
	printVersion   = flag.Bool("version", false, "Print version information and exit")
	validateSecret = flag.Bool("validate", false, "Validate that the sealed secret can be decrypted")
	mergeInto      = flag.String("merge-into", "", "Merge items from secret into an existing sealed secret file, updating the file in-place instead of writing to stdout.")
	raw            = flag.Bool("raw", false, "Encrypt a raw value passed via the --from-* flags instead of the whole secret object")
	secretName     = flag.String("name", "", "Name of the sealed secret (required with --raw)")
	fromFile       = flag.StringSlice("from-file", nil, "(only with --raw) Secret items can be sourced from files. Pro-tip: you can use /dev/stdin to read pipe input. This flag tries to follow the same syntax as in kubectl")
	sealingScope   ssv1alpha1.SealingScope
	reEncrypt      bool // re-encrypt command

	// VERSION set from Makefile
	VERSION = "UNKNOWN"

	clientConfig clientcmd.ClientConfig
)

func init() {
	flag.Var(&sealingScope, "scope", "Set the scope of the sealed secret: strict, namespace-wide, cluster-wide. Mandatory for --raw, otherwise the 'sealedsecrets.bitnami.com/cluster-wide' and 'sealedsecrets.bitnami.com/namespace-wide' annotations on the input secret can be used to select the scope.")
	flag.BoolVar(&reEncrypt, "rotate", false, "")
	flag.BoolVar(&reEncrypt, "re-encrypt", false, "Re-encrypt the given sealed secret to use the latest cluster key.")
	flag.CommandLine.MarkDeprecated("rotate", "please use --re-encrypt instead")

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

func openCertFile(certFile string) (io.ReadCloser, error) {
	f, err := os.Open(certFile)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func openCertHTTP(c corev1.CoreV1Interface, namespace, name string) (io.ReadCloser, error) {
	f, err := c.
		Services(namespace).
		ProxyGet("http", name, "", "/v1/cert.pem", nil).
		Stream()
	if err != nil {
		return nil, fmt.Errorf("cannot fetch certificate: %v", err)
	}
	return f, nil
}

func openCert(certFile string) (io.ReadCloser, error) {
	if certFile != "" {
		return openCertFile(certFile)
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
	return openCertHTTP(restClient, *controllerNs, *controllerName)
}

func seal(in io.Reader, out io.Writer, codecs runtimeserializer.CodecFactory, pubKey *rsa.PublicKey) error {
	secret, err := readSecret(codecs.UniversalDecoder(), in)
	if err != nil {
		return err
	}

	if len(secret.Data) == 0 && len(secret.StringData) == 0 {
		// No data. This is _theoretically_ just fine, but
		// almost certainly indicates a misuse of the tools.
		// If you _really_ want to encrypt an empty secret,
		// then a PR to skip this check with some sort of
		// --force flag would be welcomed.
		return fmt.Errorf("Secret.data is empty in input Secret, assuming this is an error and aborting")
	}

	if secret.GetName() == "" {
		return fmt.Errorf("Missing metadata.name in input Secret")
	}

	if secret.GetNamespace() == "" {
		ns, _, err := clientConfig.Namespace()
		if err != nil {
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
	if err = sealedSecretOutput(out, codecs, ssecret); err != nil {
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

func reEncryptSealedSecret(in io.Reader, out io.Writer, codecs runtimeserializer.CodecFactory, namespace, name string) error {
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
	if err = sealedSecretOutput(out, codecs, ssecret); err != nil {
		return err
	}
	return nil
}

func sealedSecretOutput(out io.Writer, codecs runtimeserializer.CodecFactory, ssecret *ssv1alpha1.SealedSecret) error {
	var contentType string
	switch strings.ToLower(*outputFormat) {
	case "json", "":
		contentType = runtime.ContentTypeJSON
	case "yaml":
		contentType = "application/yaml"
	default:
		return fmt.Errorf("unsupported output format: %s", *outputFormat)
	}
	prettyEnc, err := prettyEncoder(codecs, contentType, ssv1alpha1.SchemeGroupVersion)
	if err != nil {
		return err
	}
	buf, err := runtime.Encode(prettyEnc, ssecret)
	if err != nil {
		return err
	}
	out.Write(buf)
	fmt.Fprint(out, "\n")
	return nil
}

func decodeSealedSecret(codecs runtimeserializer.CodecFactory, b []byte) (*ssv1alpha1.SealedSecret, error) {
	var ss ssv1alpha1.SealedSecret
	if err := runtime.DecodeInto(codecs.UniversalDecoder(), b, &ss); err != nil {
		return nil, err
	}
	return &ss, nil
}

func sealMergingInto(in io.Reader, filename string, codecs runtimeserializer.CodecFactory, pubKey *rsa.PublicKey) error {
	var buf bytes.Buffer
	if err := seal(in, &buf, codecs, pubKey); err != nil {
		return err
	}

	update, err := decodeSealedSecret(codecs, buf.Bytes())
	if err != nil {
		return err
	}

	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	orig, err := decodeSealedSecret(codecs, b)
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
	if err := sealedSecretOutput(&out, codecs, orig); err != nil {
		return err
	}

	return ioutil.WriteFile(filename, out.Bytes(), 0)
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

func run(w io.Writer, secretName, controllerNs, controllerName, certFile string, printVersion, validateSecret, reEncrypt, dumpCert, raw bool, fromFile []string, mergeInto string) error {
	if len(fromFile) != 0 && !raw {
		return fmt.Errorf("--from-file requires --raw")
	}

	if printVersion {
		fmt.Fprintf(w, "kubeseal version: %s\n", VERSION)
		return nil
	}

	if validateSecret {
		return validateSealedSecret(os.Stdin, controllerNs, controllerName)
	}

	if reEncrypt {
		return reEncryptSealedSecret(os.Stdin, os.Stdout, scheme.Codecs, controllerNs, controllerName)
	}

	f, err := openCert(certFile)
	if err != nil {
		return err
	}
	defer f.Close()

	if dumpCert {
		_, err := io.Copy(os.Stdout, f)
		return err
	}

	pubKey, err := parseKey(f)
	if err != nil {
		return err
	}

	if mergeInto != "" {
		return sealMergingInto(os.Stdin, mergeInto, scheme.Codecs, pubKey)
	}

	if raw {
		ns, _, err := clientConfig.Namespace()
		if err != nil {
			return err
		}
		if ns == "" {
			return fmt.Errorf("must provide the --namespace flag with --raw")
		}
		if secretName == "" {
			return fmt.Errorf("must provide the --name flag with --raw")
		}

		if len(fromFile) == 0 {
			return fmt.Errorf("must provide the --from-file flag with --raw")
		}
		if len(fromFile) > 1 {
			return fmt.Errorf("must provide only one --from-file when encrypting a single item with --raw")
		}

		_, filename := parseFromFile(fromFile[0])
		data, err := ioutil.ReadFile(filename)
		if err != nil {
			return err
		}

		return encryptSecretItem(w, secretName, ns, data, sealingScope, pubKey)
	}

	return seal(os.Stdin, os.Stdout, scheme.Codecs, pubKey)
}

func main() {
	flag.Parse()
	goflag.CommandLine.Parse([]string{})

	if err := run(os.Stdout, *secretName, *controllerNs, *controllerName, *certFile, *printVersion, *validateSecret, reEncrypt, *dumpCert, *raw, *fromFile, *mergeInto); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
