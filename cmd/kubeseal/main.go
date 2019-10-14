package main

import (
	"bytes"
	"context"
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
	"strings"

	"github.com/bitnami-labs/sealed-secrets/pkg/buildinfo"
	"github.com/bitnami-labs/sealed-secrets/pkg/crypto"
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/sdk/helper/logging"
	"github.com/hashicorp/vault/vault/seal/transit"
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
	certURL        = flag.String("cert", "", "Certificate / public key file/URL to use for encryption. Overrides --controller-*")
	controllerNs   = flag.String("controller-namespace", metav1.NamespaceSystem, "Namespace of sealed-secrets controller.")
	controllerName = flag.String("controller-name", "sealed-secrets-controller", "Name of sealed-secrets controller.")
	outputFormat   = flag.StringP("format", "o", "json", "Output format for sealed secret. Either json or yaml")
	dumpCert       = flag.Bool("fetch-cert", false, "Write certificate to stdout. Useful for later use with --cert")
	encryptType    = flag.String("encrypt-type", "cert", "Encrypt and decrypt with method (default is public key/cert)")
	vaultToken     = flag.String("vault-token", "", "Vault token used to auth to vault")
	vaultAddress   = flag.String("vault-addr", "http://localhost:8200", "Vault address")
	vaultPath      = flag.String("vault-path", "transit", "Vault path to transit engine")
	vaultKeyName   = flag.String("vault-key", "sealed-secrets", "Vault key to use")
	printVersion   = flag.Bool("version", false, "Print version information and exit")
	validateSecret = flag.Bool("validate", false, "Validate that the sealed secret can be decrypted")
	mergeInto      = flag.String("merge-into", "", "Merge items from secret into an existing sealed secret file, updating the file in-place instead of writing to stdout.")
	raw            = flag.Bool("raw", false, "Encrypt a raw value passed via the --from-* flags instead of the whole secret object")
	secretName     = flag.String("name", "", "Name of the sealed secret (required with --raw and default (strict) scope)")
	fromFile       = flag.StringSlice("from-file", nil, "(only with --raw) Secret items can be sourced from files. Pro-tip: you can use /dev/stdin to read pipe input. This flag tries to follow the same syntax as in kubectl")
	sealingScope   ssv1alpha1.SealingScope
	reEncrypt      bool // re-encrypt command

	// VERSION set from Makefile
	VERSION = buildinfo.DefaultVersion

	clientConfig clientcmd.ClientConfig
)

func init() {
	buildinfo.FallbackVersion(&VERSION, buildinfo.DefaultVersion)

	flag.Var(&sealingScope, "scope", "Set the scope of the sealed secret: strict, namespace-wide, cluster-wide (defaults to strict). Mandatory for --raw, otherwise the 'sealedsecrets.bitnami.com/cluster-wide' and 'sealedsecrets.bitnami.com/namespace-wide' annotations on the input secret can be used to select the scope.")
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

func openCert(certURL string) (io.ReadCloser, error) {
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
	return openCertCluster(restClient, *controllerNs, *controllerName)
}

// Seal reads a k8s Secret resource parsed from an input reader by a given codec, encrypts all its secrets
// with a given public key, using the name and namespace found in the input secret, unless explicitly overridden
// by the overrideName and overrideNamespace arguments.
func seal(in io.Reader, out io.Writer, codecs runtimeserializer.CodecFactory, pubKey *rsa.PublicKey, overrideName, overrideNamespace string) error {
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

	if overrideName != "" {
		secret.Name = overrideName
	}

	if secret.GetName() == "" {
		return fmt.Errorf("Missing metadata.name in input Secret")
	}

	if overrideNamespace != "" {
		secret.Namespace = overrideNamespace
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

	ssecret, err := ssv1alpha1.NewSealedSecret(codecs, *encryptType, pubKey, secret)
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
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	orig, err := decodeSealedSecret(codecs, b)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := seal(in, &buf, codecs, pubKey, orig.Name, orig.Namespace); err != nil {
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

func run(w io.Writer, secretName, controllerNs, controllerName, certURL string, printVersion, validateSecret, reEncrypt, dumpCert, raw bool, fromFile []string, mergeInto string, encryptType string, vaultToken string, vaultAddress string, vaultPath string, vaultKeyName string) error {
	var pubKey *rsa.PublicKey

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

	if strings.ToLower(encryptType) == "cert" {
		f, err := openCert(certURL)
		if err != nil {
			return err
		}
		defer f.Close()

		if dumpCert {
			_, err := io.Copy(os.Stdout, f)
			return err
		}

		pubKey, err = parseKey(f)
		if err != nil {
			return err
		}
	}

	if mergeInto != "" {
		return sealMergingInto(os.Stdin, mergeInto, scheme.Codecs, pubKey)
	}

	if raw {
		out := []byte{}
		ns, _, err := clientConfig.Namespace()
		if err != nil {
			return err
		}

		if ns == "" && sealingScope < ssv1alpha1.ClusterWideScope {
			return fmt.Errorf("must provide the --namespace flag with --raw and --scope %s", sealingScope.String())
		}

		if secretName == "" && sealingScope < ssv1alpha1.NamespaceWideScope {
			return fmt.Errorf("must provide the --name flag with --raw and --scope %s", sealingScope.String())
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

		label := ssv1alpha1.EncryptionLabel(ns, secretName, sealingScope)
		switch strings.ToLower(encryptType) {
		case "cert":
			enc := encryptData{
				certConfig: certConfig{
					pubKey: pubKey,
					label:  label,
				},
				plaintext: data,
			}
			s := certEncrypt{}
			out, err = s.encrypt(enc)
			if err != nil {
				return err
			}
		case "vault":
			enc := encryptData{
				plaintext: data,
				vaultConfig: vaultConfig{
					address:   vaultAddress,
					mountPath: vaultPath,
					keyName:   vaultKeyName,
					token:     vaultToken,
				},
			}
			s := vaultEncrypt{}
			out, err = s.encrypt(enc)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported encryption method: %s", encryptType)
		}
		fmt.Fprint(w, base64.StdEncoding.EncodeToString(out))
		return nil
	}

	return seal(os.Stdin, os.Stdout, scheme.Codecs, pubKey, secretName, "")
}

func main() {
	flag.Parse()
	goflag.CommandLine.Parse([]string{})

	if err := run(os.Stdout, *secretName, *controllerNs, *controllerName, *certURL, *printVersion, *validateSecret, reEncrypt, *dumpCert, *raw, *fromFile, *mergeInto, *encryptType, *vaultToken, *vaultAddress, *vaultPath, *vaultKeyName); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

type encryptor interface {
	encrypt(encryptData) ([]byte, error)
}

type certEncrypt struct {
	encryptor
}

type vaultEncrypt struct {
	encryptor
}

type encryptData struct {
	certConfig  certConfig
	vaultConfig vaultConfig
	plaintext   []byte
}

type certConfig struct {
	pubKey *rsa.PublicKey
	label  []byte
}

type vaultConfig struct {
	address   string
	token     string
	keyName   string
	mountPath string
}

func (c certEncrypt) encrypt(d encryptData) ([]byte, error) {
	out, err := crypto.HybridEncrypt(rand.Reader, d.certConfig.pubKey, d.plaintext, d.certConfig.label)
	if err != nil {
		return []byte{}, err
	}
	return out, nil
}

func (c vaultEncrypt) encrypt(d encryptData) ([]byte, error) {
	s := transit.NewSeal(logging.NewVaultLogger(log.Trace))
	config := map[string]string{
		"address":    d.vaultConfig.address,
		"key_name":   d.vaultConfig.keyName,
		"token":      d.vaultConfig.token,
		"mount_path": d.vaultConfig.mountPath,
	}
	s.SetConfig(config)

	swi, err := s.Encrypt(context.Background(), d.plaintext)
	if err != nil {
		return []byte{}, err
	}
	return swi.Ciphertext, nil
}
