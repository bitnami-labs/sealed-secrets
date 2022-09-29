package kubeseal

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealedsecrets/v1alpha1"
	"github.com/bitnami-labs/sealed-secrets/pkg/crypto"
	"github.com/bitnami-labs/sealed-secrets/pkg/multidocyaml"
	"github.com/google/renameio"
	"github.com/mattn/go-isatty"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
)

type namespaceFn func() (string, bool, error)

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
	solveNamespace namespaceFn
}

func initNamespaceFuncFromClient(clientConfig clientcmd.ClientConfig) namespaceFn {
	return func() (string, bool, error) { return clientConfig.Namespace() }
}

func NewConfig(clientConfig clientcmd.ClientConfig, flags *Flags) *Config {
	return &Config{
		flags:          flags,
		clientConfig:   clientConfig,
		ctx:            context.Background(),
		solveNamespace: initNamespaceFuncFromClient(clientConfig),
	}
}

func InitClient(kubeConfigPath string, cfgOverrides *clientcmd.ConfigOverrides, r io.Reader) clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	loadingRules.ExplicitPath = kubeConfigPath
	return clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, cfgOverrides, r)
}

func parseKey(r io.Reader) (*rsa.PublicKey, error) {
	data, err := io.ReadAll(r)
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

	if time.Now().After(certs[0].NotAfter) {
		return nil, fmt.Errorf("failed to encrypt using an expired certificate on %v", certs[0].NotAfter.Format("January 2, 2006"))
	}

	return cert, nil
}

func readSecret(codec runtime.Decoder, r io.Reader) (*v1.Secret, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	if err := multidocyaml.EnsureNotMultiDoc(data); err != nil {
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
	// windows drive letters
	if s := strings.ToLower(u.Scheme); len(s) == 1 && s[0] >= 'a' && s[0] <= 'z' {
		return true, nil
	}
	return u.Scheme == "", nil
}

// getServicePortName obtains the SealedSecrets service port name
func getServicePortName(ctx context.Context, client corev1.CoreV1Interface, namespace, serviceName string) (string, error) {
	service, err := client.Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("cannot get sealed secret service: %v", err)
	}
	return service.Spec.Ports[0].Name, nil
}

// openCertLocal opens a cert URI or local filename, by fetching it locally from the client
// (as opposed as openCertCluster which fetches it via HTTP but through the k8s API proxy).
func openCertLocal(filenameOrURI string) (io.ReadCloser, error) {
	// detect if a certificate is a local file or an URI.
	if ok, err := isFilename(filenameOrURI); err != nil {
		return nil, err
	} else if ok {
		// #nosec G304 -- should open user provided file
		return os.Open(filenameOrURI)
	}
	return openCertURI(filenameOrURI)
}

func openCertURI(uri string) (io.ReadCloser, error) {
	// support file:// scheme. Note: we're opening the file using os.Open rather
	// than using the file:// scheme below because there is no point in complicating our lives
	// and escape the filename properly.

	t := &http.Transport{}
	// #nosec: G111 -- we want to allow all files to be opened
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
func openCertCluster(ctx context.Context, c corev1.CoreV1Interface, namespace, name string) (io.ReadCloser, error) {
	portName, err := getServicePortName(ctx, c, namespace, name)
	if err != nil {
		return nil, err
	}
	cert, err := c.Services(namespace).ProxyGet("http", name, portName, "/v1/cert.pem", nil).Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch certificate: %v", err)
	}
	return cert, nil
}

func openCert(cfg *Config, certURL string) (io.ReadCloser, error) {
	if certURL != "" {
		return openCertLocal(certURL)
	}

	conf, err := cfg.clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	conf.AcceptContentTypes = "application/x-pem-file, */*"
	restClient, err := corev1.NewForConfig(conf)
	if err != nil {
		return nil, err
	}
	return openCertCluster(cfg.ctx, restClient, cfg.flags.ControllerNs, cfg.flags.ControllerName)
}

// Seal reads a k8s Secret resource parsed from an input reader by a given codec, encrypts all its secrets
// with a given public key, using the name and namespace found in the input secret, unless explicitly overridden
// by the overrideName and overrideNamespace arguments.
func seal(cfg *Config, in io.Reader, out io.Writer, codecs runtimeserializer.CodecFactory, pubKey *rsa.PublicKey, scope ssv1alpha1.SealingScope, allowEmptyData bool, overrideName, overrideNamespace string) error {
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
		ns, _, err := cfg.solveNamespace()
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
	if err = sealedSecretOutput(out, cfg.flags, codecs, ssecret); err != nil {
		return err
	}
	return nil
}

func validateSealedSecret(cfg *Config, in io.Reader) error {
	flags := cfg.flags
	conf, err := cfg.clientConfig.ClientConfig()
	if err != nil {
		return err
	}
	restClient, err := corev1.NewForConfig(conf)
	if err != nil {
		return err
	}
	portName, err := getServicePortName(cfg.ctx, restClient, flags.ControllerNs, flags.ControllerName)
	if err != nil {
		return err
	}

	content, err := io.ReadAll(in)
	if err != nil {
		return err
	}

	req := restClient.RESTClient().Post().
		Namespace(flags.ControllerNs).
		Resource("services").
		SubResource("proxy").
		Name(net.JoinSchemeNamePort("http", flags.ControllerName, portName)).
		Suffix("/v1/verify")

	req.Body(content)
	res := req.Do(cfg.ctx)
	if err := res.Error(); err != nil {
		if status, ok := err.(*k8serrors.StatusError); ok && status.Status().Code == http.StatusConflict {
			return fmt.Errorf("unable to decrypt sealed secret")
		}
		return fmt.Errorf("cannot validate sealed secret: %v", err)
	}

	return nil
}

func reEncryptSealedSecret(cfg *Config, in io.Reader, out io.Writer, codecs runtimeserializer.CodecFactory) error {
	flags := cfg.flags
	conf, err := cfg.clientConfig.ClientConfig()
	if err != nil {
		return err
	}
	restClient, err := corev1.NewForConfig(conf)
	if err != nil {
		return err
	}
	portName, err := getServicePortName(cfg.ctx, restClient, flags.ControllerNs, flags.ControllerName)
	if err != nil {
		return err
	}

	content, err := io.ReadAll(in)
	if err != nil {
		return err
	}

	req := restClient.RESTClient().Post().
		Namespace(flags.ControllerNs).
		Resource("services").
		SubResource("proxy").
		Name(net.JoinSchemeNamePort("http", flags.ControllerName, portName)).
		Suffix("/v1/rotate")

	req.Body(content)
	res := req.Do(cfg.ctx)
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
	if err = sealedSecretOutput(out, cfg.flags, codecs, ssecret); err != nil {
		return err
	}
	return nil
}

func resourceOutput(out io.Writer, flags *Flags, codecs runtimeserializer.CodecFactory, gv runtime.GroupVersioner, obj runtime.Object) error {
	var contentType string
	switch strings.ToLower(flags.OutputFormat) {
	case "json", "":
		contentType = runtime.ContentTypeJSON
	case "yaml":
		contentType = runtime.ContentTypeYAML
	default:
		return fmt.Errorf("unsupported output format: %s", flags.OutputFormat)
	}
	prettyEnc, err := prettyEncoder(codecs, contentType, gv)
	if err != nil {
		return err
	}
	buf, err := runtime.Encode(prettyEnc, obj)
	if err != nil {
		return err
	}
	_, _ = out.Write(buf)
	fmt.Fprint(out, "\n")
	return nil
}

func sealedSecretOutput(out io.Writer, flags *Flags, codecs runtimeserializer.CodecFactory, ssecret *ssv1alpha1.SealedSecret) error {
	return resourceOutput(out, flags, codecs, ssv1alpha1.SchemeGroupVersion, ssecret)
}

func decodeSealedSecret(codecs runtimeserializer.CodecFactory, b []byte) (*ssv1alpha1.SealedSecret, error) {
	var ss ssv1alpha1.SealedSecret
	if err := runtime.DecodeInto(codecs.UniversalDecoder(), b, &ss); err != nil {
		return nil, err
	}
	return &ss, nil
}

func sealMergingInto(cfg *Config, in io.Reader, filename string, codecs runtimeserializer.CodecFactory, pubKey *rsa.PublicKey, scope ssv1alpha1.SealingScope, allowEmptyData bool) error {
	// #nosec G304 -- should open user provided file
	f, err := os.OpenFile(filename, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	// #nosec G307 -- we are explicitly managing a potential error from f.Close() at the end of the function
	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	orig, err := decodeSealedSecret(codecs, b)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := seal(cfg, in, &buf, codecs, pubKey, scope, allowEmptyData, orig.Name, orig.Namespace); err != nil {
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
	for k, v := range update.Spec.Template.Data {
		orig.Spec.Template.Data[k] = v
	}

	// updated sealed secret file in-place avoiding clobbering the file upon rendering errors.
	var out bytes.Buffer
	if err := sealedSecretOutput(&out, cfg.flags, codecs, orig); err != nil {
		return err
	}

	if err := f.Truncate(0); err != nil {
		return err
	}
	if _, err := f.Seek(0, 0); err != nil {
		return err
	}
	if _, err := io.Copy(f, &out); err != nil {
		return err
	}
	// we explicitly call f.Close() to return a pontential error when closing the file that wouldn't be returned in the deferred f.Close()
	if err := f.Close(); err != nil {
		return err
	}
	return nil
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
	// #nosec G304 -- should open user provided file
	b, err := os.ReadFile(filename)
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

func unsealSealedSecret(flags *Flags, w io.Writer, in io.Reader, codecs runtimeserializer.CodecFactory) error {
	privKeys, err := readPrivKeys(flags.PrivKeys)
	if err != nil {
		return err
	}

	b, err := io.ReadAll(in)
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

func Run(w io.Writer, cfg *Config) (err error) {
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
		return unsealSealedSecret(flags, w, input, scheme.Codecs)
	}
	if len(flags.PrivKeys) != 0 && isatty.IsTerminal(os.Stderr.Fd()) {
		fmt.Fprintf(os.Stderr, "warning: ignoring --recovery-private-key because unseal command not chosen with --recovery-unseal\n")
	}

	if flags.ValidateSecret {
		return validateSealedSecret(cfg, input)
	}

	if flags.ReEncrypt {
		return reEncryptSealedSecret(cfg, input, w, scheme.Codecs)
	}

	f, err := openCert(cfg, flags.CertURL)
	if err != nil {
		return err
	}
	defer f.Close()

	if flags.DumpCert {
		_, err := io.Copy(w, f)
		return err
	}

	pubKey, err := parseKey(f)
	if err != nil {
		return err
	}

	if flags.MergeInto != "" {
		return sealMergingInto(cfg, input, flags.MergeInto, scheme.Codecs, pubKey, flags.SealingScope, flags.AllowEmptyData)
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

			_, filename := parseFromFile(flags.FromFile[0])
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

		return encryptSecretItem(w, flags.SecretName, ns, data, flags.SealingScope, pubKey)
	}

	return seal(cfg, input, w, scheme.Codecs, pubKey, flags.SealingScope, flags.AllowEmptyData, flags.SecretName, "")
}
