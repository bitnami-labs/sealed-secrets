package main

import (
	"crypto/rsa"
	"errors"
	goflag "flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/util/net"

	flag "github.com/spf13/pflag"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/cert"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealed-secrets/v1alpha1"

	// Register Auth providers
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

var (
	// TODO: Verify k8s server signature against cert in kube client config.
	certFile       = flag.String("cert", "", "Certificate / public key to use for encryption. Overrides --controller-*")
	controllerNs   = flag.String("controller-namespace", metav1.NamespaceSystem, "Namespace of sealed-secrets controller.")
	controllerName = flag.String("controller-name", "sealed-secrets-controller", "Name of sealed-secrets controller.")
	outputFormat   = flag.String("format", "json", "Output format for sealed secret. Either json or yaml")
	keyName        = flag.String("keyname", "", "Name of a key to use.")
	rotate         = flag.Bool("rotate", false, "Rotate the given key to use the latest private key in the cluster.")
	dumpCert       = flag.Bool("fetch-cert", false, "Write certificate to stdout. Useful for later use with --cert")
	dumpKeyName    = flag.Bool("fetch-keyname", false, "Write keyname so stdout. Useful for later use with --keyname")
	printVersion   = flag.Bool("version", false, "Print version information and exit")
	validateSecret = flag.Bool("validate", false, "Validate that the sealed secret can be decrypted")

	// VERSION set from Makefile
	VERSION = "UNKNOWN"

	clientConfig clientcmd.ClientConfig
)

func init() {
	// The "usual" clientcmd/kubectl flags
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	overrides := clientcmd.ConfigOverrides{}
	kflags := clientcmd.RecommendedConfigOverrideFlags("")
	flag.StringVar(&loadingRules.ExplicitPath, "kubeconfig", "", "Path to a kube config. Only required if out-of-cluster")
	clientcmd.BindOverrideFlags(&overrides, flag.CommandLine, kflags)
	clientConfig = clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, &overrides, os.Stdin)

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

func openCertFile(certFile string) (io.ReadCloser, string, error) {
	f, err := os.Open(certFile)
	if err != nil {
		return nil, "", fmt.Errorf("Error reading %s: %v", certFile, err)
	}
	return f, *keyName, nil
}

func openCertHTTP(c corev1.CoreV1Interface, namespace, name string) (io.ReadCloser, string, error) {
	f, err := c.
		Services(namespace).
		ProxyGet("http", name, "", "/v1/cert.pem", map[string]string{"keyname": *keyName}).
		Stream()
	if err != nil {
		return nil, "", fmt.Errorf("Error fetching certificate: %v", err)
	}
	return f, *keyName, nil
}

func openCert() (io.ReadCloser, string, error) {
	if *certFile != "" {
		return openCertFile(*certFile)
	}

	if *keyName == "" {
		name, err := getKeyName()
		if err != nil {
			return nil, "", err
		}
		*keyName = name
	}

	conf, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, "", err
	}
	conf.AcceptContentTypes = "application/x-pem-file, */*"
	restClient, err := corev1.NewForConfig(conf)
	if err != nil {
		return nil, "", err
	}
	return openCertHTTP(restClient, *controllerNs, *controllerName)
}

func getKeyName() (string, error) {
	if *keyName != "" {
		return *keyName, nil
	}

	// Setup client
	conf, err := clientConfig.ClientConfig()
	if err != nil {
		return "", err
	}
	conf.AcceptContentTypes = "plain/text"
	restClient, err := corev1.NewForConfig(conf)
	if err != nil {
		return "", err
	}
	// Get the latest keyname from controller
	f, err := restClient.
		Services(*controllerNs).
		ProxyGet("http", *controllerName, "", "/v1/keyname", nil).
		Stream()
	if err != nil {
		return "", fmt.Errorf("Error fetching keyname: %v", err)
	}
	defer f.Close()
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("Error reading fetched keyname: %v", err)
	}
	return string(data), nil
}

func seal(in io.Reader, out io.Writer, codecs runtimeserializer.CodecFactory, pubKey *rsa.PublicKey, keyname string) error {
	secret, err := readSecret(codecs.UniversalDecoder(), in)
	if err != nil {
		return err
	}

	if len(secret.Data) == 0 {
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

	ssecret, err := ssv1alpha1.NewSealedSecret(codecs, keyname, pubKey, secret)
	if err != nil {
		return err
	}

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
			return fmt.Errorf("Unable to decrypt sealed secret")
		}
		return fmt.Errorf("Error occurred while validating sealed secret")
	}

	return nil
}

func rotateSealedSecret(in io.Reader, out io.Writer, codecs runtimeserializer.CodecFactory, namespace, name string) error {
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
			return fmt.Errorf("Unable to rotate secret")
		}
		return fmt.Errorf("Error occurred while rotating secret")
	}
	body, err := res.Raw()
	if err != nil {
		return err
	}
	if _, err = out.Write(body); err != nil {
		return err
	}
	return nil
}

func generateKey(namespace, name string) error {
	conf, err := clientConfig.ClientConfig()
	if err != nil {
		return err
	}
	restClient, err := corev1.NewForConfig(conf)
	if err != nil {
		return err
	}
	req := restClient.RESTClient().Post().
		Namespace(namespace).
		Resource("services").
		SubResource("proxy").
		Name(net.JoinSchemeNamePort("http", name, "")).
		Suffix("/v1/keygen")
	res := req.Do()
	if err = res.Error(); err != nil {
		return err
	}
	return nil
}

func main() {
	flag.Parse()
	goflag.CommandLine.Parse([]string{})

	if *printVersion {
		fmt.Printf("kubeseal version: %s\n", VERSION)
		return
	}

	if *validateSecret {
		err := validateSealedSecret(os.Stdin, *controllerNs, *controllerName)
		if err != nil {
			panic(err.Error())
		}
		return
	}

	if *dumpKeyName {
		keyName, err := getKeyName()
		if err != nil {
			panic(err.Error())
		}
		os.Stdout.WriteString(keyName)
		return
	}

	if *rotate {
		if err := rotateSealedSecret(os.Stdin, os.Stdout, scheme.Codecs, *controllerNs, *controllerName); err != nil {
			panic(err.Error())
		}
		return
	}

	f, name, err := openCert()
	if err != nil {
		panic(err.Error())
	}
	defer f.Close()

	if *dumpCert {
		if _, err := io.Copy(os.Stdout, f); err != nil {
			panic(err.Error())
		}
		return
	}

	pubKey, err := parseKey(f)
	if err != nil {
		panic(err.Error())
	}

	if err := seal(os.Stdin, os.Stdout, scheme.Codecs, pubKey, name); err != nil {
		panic(err.Error())
	}
}
