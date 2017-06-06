package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/rest"
	certUtil "k8s.io/client-go/util/cert"

	ssv1alpha1 "github.com/ksonnet/sealed-secrets/apis/v1alpha1"
)

var (
	keyName  = flag.String("key-name", "seal-key", "Name of Secret containing public/private key.")
	keySize  = flag.Int("key-size", 4096, "Size of encryption key.")
	validFor = flag.Duration("key-ttl", 10*365*24*time.Hour, "Duration that certificate is valid for.")
	myCN     = flag.String("my-cn", "", "CN to use in generated certificate.")
)

type controller struct {
	clientset kubernetes.Interface
}

func createTPR(client kubernetes.Interface) error {
	tpr := &v1beta1.ThirdPartyResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: ssv1alpha1.SealedSecretName,
		},
		Versions: []v1beta1.APIVersion{
			{Name: ssv1alpha1.SchemeGroupVersion.Version},
		},
		Description: "A sealed (encrypted) Secret",
	}
	result, err := client.ExtensionsV1beta1().ThirdPartyResources().Create(tpr)
	if err != nil && errors.IsAlreadyExists(err) {
		result, err = client.ExtensionsV1beta1().ThirdPartyResources().Update(tpr)
	}
	if err != nil {
		return err
	}
	log.Printf("Created/updated ThirdPartyResource: %#v", result)
	return nil
}

func readKey(client kubernetes.Interface, namespace, keyName string) (*rsa.PrivateKey, []*x509.Certificate, error) {
	secret, err := client.Core().Secrets(namespace).Get(keyName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}

	key, err := certUtil.ParsePrivateKeyPEM(secret.Data[v1.TLSPrivateKeyKey])
	if err != nil {
		return nil, nil, err
	}

	certs, err := certUtil.ParseCertsPEM(secret.Data[v1.TLSCertKey])
	if err != nil {
		return nil, nil, err
	}

	return key.(*rsa.PrivateKey), certs, nil
}

func writeKey(client kubernetes.Interface, key *rsa.PrivateKey, certs []*x509.Certificate, namespace, keyName string) error {
	certbytes := []byte{}
	for _, cert := range certs {
		certbytes = append(certbytes, certUtil.EncodeCertPEM(cert)...)
	}

	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      keyName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			v1.TLSPrivateKeyKey: certUtil.EncodePrivateKeyPEM(key),
			v1.TLSCertKey:       certbytes,
		},
		Type: v1.SecretTypeTLS,
	}

	_, err := client.Core().Secrets(namespace).Create(&secret)
	return err
}

func signKey(r io.Reader, key *rsa.PrivateKey) (*x509.Certificate, error) {
	// TODO: use certificates API to get this signed by the cluster root CA
	// See https://kubernetes.io/docs/tasks/tls/managing-tls-in-a-cluster/

	notBefore := time.Now()

	serialNo, err := rand.Int(r, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}

	cert := x509.Certificate{
		SerialNumber: serialNo,
		KeyUsage:     x509.KeyUsageEncipherOnly,
		NotBefore:    notBefore.UTC(),
		NotAfter:     notBefore.Add(*validFor).UTC(),
		Subject: pkix.Name{
			CommonName: *myCN,
		},
		BasicConstraintsValid: true,
		IsCA: true,
	}

	data, err := x509.CreateCertificate(r, &cert, &cert, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}

	return x509.ParseCertificate(data)
}

func initKey(client kubernetes.Interface, r io.Reader, keySize int, namespace, keyName string) (*rsa.PrivateKey, error) {
	privKey, certs, err := readKey(client, namespace, keyName)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Printf("Key %s/%s not found, generating new %d bit key", namespace, keyName, keySize)
			privKey, err = rsa.GenerateKey(r, keySize)
			if err != nil {
				return nil, err
			}

			cert, err := signKey(r, privKey)
			if err != nil {
				return nil, err
			}
			certs = []*x509.Certificate{cert}

			if err = writeKey(client, privKey, certs, namespace, keyName); err != nil {
				return nil, err
			}
			log.Printf("New key written to %s/%s", namespace, keyName)
		} else {
			return nil, err
		}
	}

	pubText, err := certUtil.EncodePublicKeyPEM(&privKey.PublicKey)
	if err != nil {
		return nil, err
	}
	log.Printf("Public key is:\n%s\n", pubText)

	for _, cert := range certs {
		log.Printf("Certificate is:\n%s\n", certUtil.EncodeCertPEM(cert))
	}

	return privKey, nil
}

func myNamespace() string {
	if ns := os.Getenv("POD_NAMESPACE"); ns != "" {
		return ns
	}

	// Fall back to the namespace associated with the service account token, if available
	if data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
			return ns
		}
	}

	return api.NamespaceDefault
}

func tprClient(c *rest.Config, gv *schema.GroupVersion) (rest.Interface, error) {
	tprconfig := *c // shallow copy
	tprconfig.GroupVersion = gv
	tprconfig.APIPath = "/apis"
	tprconfig.ContentType = runtime.ContentTypeJSON
	tprconfig.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: api.Codecs}
	return rest.RESTClientFor(&tprconfig)
}

func main2() error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	ssclient, err := tprClient(config, &ssv1alpha1.SchemeGroupVersion)
	if err != nil {
		return err
	}

	myNs := myNamespace()

	if err = createTPR(clientset); err != nil {
		return err
	}

	privKey, err := initKey(clientset, rand.Reader, *keySize, myNs, *keyName)
	if err != nil {
		return err
	}

	controller := NewController(clientset, ssclient, rand.Reader, privKey)

	stop := make(chan struct{})
	defer close(stop)

	go controller.Run(stop)

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM)
	<-sigterm

	return nil
}

func main() {
	flag.Parse()

	if err := main2(); err != nil {
		panic(err.Error())
	}
}
