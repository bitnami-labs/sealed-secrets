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
	"strings"
	"time"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealedsecrets/v1alpha1"
	"github.com/bitnami-labs/sealed-secrets/pkg/crypto"
	"github.com/bitnami-labs/sealed-secrets/pkg/multidocyaml"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
)

type ClientConfig interface {
	ClientConfig() (*rest.Config, error)
	Namespace() (string, bool, error)
}

func ParseKey(r io.Reader) (*rsa.PublicKey, error) {
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
		return nil, errors.New("failed to read any certificates")
	}

	cert, ok := certs[0].PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("expected RSA public key but found %v", certs[0].PublicKey)
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

// getServicePortName obtains the SealedSecrets service port name.
func getServicePortName(ctx context.Context, client corev1.CoreV1Interface, namespace, serviceName string) (string, error) {
	service, err := client.Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("cannot get sealed secret service: %v.\nPlease, use the flag --controller-name and --controller-namespace to set up the name and namespace of the sealed secrets controller", err)
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

func OpenCert(ctx context.Context, clientConfig ClientConfig, controllerNs, controllerName string, certURL string) (io.ReadCloser, error) {
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
	return openCertCluster(ctx, restClient, controllerNs, controllerName)
}

// Seal reads a k8s Secret resource parsed from an input reader by a given codec, encrypts all its secrets
// with a given public key, using the name and namespace found in the input secret, unless explicitly overridden
// by the overrideName and overrideNamespace arguments.
func Seal(clientConfig ClientConfig, outputFormat string, in io.Reader, out io.Writer, codecs runtimeserializer.CodecFactory, pubKey *rsa.PublicKey, scope ssv1alpha1.SealingScope, allowEmptyData bool, overrideName, overrideNamespace string) error {
	secret, err := readSecret(codecs.UniversalDecoder(), in)
	if err != nil {
		return err
	}

	if len(secret.Data) == 0 && len(secret.StringData) == 0 && !allowEmptyData {
		return fmt.Errorf("secret.data is empty in input Secret, assuming this is an error and aborting. To work with empty data, --allow-empty-data can be used")
	}

	if overrideName != "" {
		secret.Name = overrideName
	}

	if secret.GetName() == "" {
		return fmt.Errorf("missing metadata.name in input Secret")
	}

	if overrideNamespace != "" {
		secret.Namespace = overrideNamespace
	}

	if scope != ssv1alpha1.DefaultScope {
		secret.Annotations = ssv1alpha1.UpdateScopeAnnotations(secret.Annotations, scope)
	}

	if ssv1alpha1.SecretScope(secret) != ssv1alpha1.ClusterWideScope && secret.GetNamespace() == "" {
		ns, _, err := clientConfig.Namespace()
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
	if err = sealedSecretOutput(out, outputFormat, codecs, ssecret); err != nil {
		return err
	}
	return nil
}

func ValidateSealedSecret(ctx context.Context, clientConfig ClientConfig, controllerNs, controllerName string, in io.Reader) error {
	conf, err := clientConfig.ClientConfig()
	if err != nil {
		return err
	}
	restClient, err := corev1.NewForConfig(conf)
	if err != nil {
		return err
	}
	portName, err := getServicePortName(ctx, restClient, controllerNs, controllerName)
	if err != nil {
		return err
	}

	content, err := io.ReadAll(in)
	if err != nil {
		return err
	}

	req := restClient.RESTClient().Post().
		Namespace(controllerNs).
		Resource("services").
		SubResource("proxy").
		Name(net.JoinSchemeNamePort("http", controllerName, portName)).
		Suffix("/v1/verify")

	req.Body(content)
	res := req.Do(ctx)
	if err := res.Error(); err != nil {
		if status, ok := err.(*k8serrors.StatusError); ok && status.Status().Code == http.StatusConflict {
			return fmt.Errorf("unable to decrypt sealed secret")
		}
		return fmt.Errorf("cannot validate sealed secret: %v", err)
	}

	return nil
}

func ReEncryptSealedSecret(ctx context.Context, clientConfig ClientConfig, controllerNs, controllerName, outputFormat string, in io.Reader, out io.Writer, codecs runtimeserializer.CodecFactory) error {
	conf, err := clientConfig.ClientConfig()
	if err != nil {
		return err
	}
	restClient, err := corev1.NewForConfig(conf)
	if err != nil {
		return err
	}
	portName, err := getServicePortName(ctx, restClient, controllerNs, controllerName)
	if err != nil {
		return err
	}

	content, err := io.ReadAll(in)
	if err != nil {
		return err
	}

	req := restClient.RESTClient().Post().
		Namespace(controllerNs).
		Resource("services").
		SubResource("proxy").
		Name(net.JoinSchemeNamePort("http", controllerName, portName)).
		Suffix("/v1/rotate")

	req.Body(content)
	res := req.Do(ctx)
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
	if err = sealedSecretOutput(out, outputFormat, codecs, ssecret); err != nil {
		return err
	}
	return nil
}

func resourceOutput(out io.Writer, outputFormat string, codecs runtimeserializer.CodecFactory, gv runtime.GroupVersioner, obj runtime.Object) error {
	var contentType string
	switch strings.ToLower(outputFormat) {
	case "json", "":
		contentType = runtime.ContentTypeJSON
	case "yaml":
		contentType = runtime.ContentTypeYAML
	default:
		return fmt.Errorf("unsupported output format: %s", outputFormat)
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

func sealedSecretOutput(out io.Writer, outputFormat string, codecs runtimeserializer.CodecFactory, ssecret *ssv1alpha1.SealedSecret) error {
	return resourceOutput(out, outputFormat, codecs, ssv1alpha1.SchemeGroupVersion, ssecret)
}

func decodeSealedSecret(codecs runtimeserializer.CodecFactory, b []byte) (*ssv1alpha1.SealedSecret, error) {
	var ss ssv1alpha1.SealedSecret
	if err := runtime.DecodeInto(codecs.UniversalDecoder(), b, &ss); err != nil {
		return nil, err
	}
	return &ss, nil
}

func SealMergingInto(clientConfig ClientConfig, outputFormat string, in io.Reader, filename string, codecs runtimeserializer.CodecFactory, pubKey *rsa.PublicKey, scope ssv1alpha1.SealingScope, allowEmptyData bool) error {
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
	if err := Seal(clientConfig, outputFormat, in, &buf, codecs, pubKey, scope, allowEmptyData, orig.Name, orig.Namespace); err != nil {
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
	if err := sealedSecretOutput(&out, outputFormat, codecs, orig); err != nil {
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
	// we explicitly call f.Close() to return a potential error when closing the file that wouldn't be returned in the deferred f.Close()
	if err := f.Close(); err != nil {
		return err
	}
	return nil
}

func EncryptSecretItem(w io.Writer, secretName, ns string, data []byte, scope ssv1alpha1.SealingScope, pubKey *rsa.PublicKey) error {
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
func ParseFromFile(s string) (string, string) {
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

func UnsealSealedSecret(w io.Writer, in io.Reader, privKeysFilenames []string, outputFormat string, codecs runtimeserializer.CodecFactory) error {
	privKeys, err := readPrivKeys(privKeysFilenames)
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

	return resourceOutput(w, outputFormat, codecs, v1.SchemeGroupVersion, sec)
}
