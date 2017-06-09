package main

import (
	"crypto/rsa"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/util/cert"

	// Install standard API types
	_ "k8s.io/client-go/kubernetes"

	ssv1alpha1 "github.com/ksonnet/sealed-secrets/apis/v1alpha1"
)

var (
	// TODO: Fetch this automatically.
	// TODO: Verify k8s server signature against cert in kube client config.
	certFile = flag.String("cert", "", "Certificate / public key to use for encryption.")

	// TODO: Fetch default from regular kubectl config
	defaultNamespace = flag.String("namespace", api.NamespaceDefault, "Default namespace to assume for Secret.")
)

func readKey(r io.Reader) (*rsa.PublicKey, error) {
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

func seal(in io.Reader, out io.Writer, codecs runtimeserializer.CodecFactory) error {
	secret, err := readSecret(codecs.UniversalDecoder(), in)
	if err != nil {
		return err
	}

	if secret.GetNamespace() == "" {
		secret.SetNamespace(*defaultNamespace)
	}

	f, err := os.Open(*certFile)
	if err != nil {
		return fmt.Errorf("Error reading %s: %v", *certFile, err)
	}
	pubKey, err := readKey(f)
	if err != nil {
		return err
	}
	ssecret, err := ssv1alpha1.NewSealedSecret(codecs, pubKey, secret)
	if err != nil {
		return err
	}

	prettyEnc, err := prettyEncoder(codecs, runtime.ContentTypeJSON, ssv1alpha1.SchemeGroupVersion)
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

func main() {
	flag.Parse()

	if err := seal(os.Stdin, os.Stdout, api.Codecs); err != nil {
		panic(err.Error())
	}
}
