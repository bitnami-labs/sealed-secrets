package main

import (
	"crypto/rsa"
	"fmt"
	"io/ioutil"

	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	ssbackend "github.com/bitnami-labs/sealed-secrets/pkg/backend"
	"github.com/bitnami-labs/sealed-secrets/pkg/backend/aes"
	"github.com/bitnami-labs/sealed-secrets/pkg/backend/aws"
)

func fetchControllerBackend() (string, error) {
	conf, err := clientConfig.ClientConfig()
	if err != nil {
		return "", err
	}

	restClient, err := corev1.NewForConfig(conf)
	if err != nil {
		return "", err
	}

	f, err := restClient.
		Services(*controllerNs).
		ProxyGet("http", *controllerName, "", "/v1/backend", nil).
		Stream()
	if err != nil {
		return "", fmt.Errorf("cannot fetch backend type: %v", err)
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func getBackend() (backend ssbackend.Backend, providerData []byte, err error) {

	if *unseal && *encryptBackend == "" {
		return nil, nil, fmt.Errorf("missing encryption backend to unseal")
	}

	var controllerBackend string
	if *encryptBackend == "" {
		controllerBackend, err = fetchControllerBackend()
		if err != nil {
			return nil, nil, err
		}
	}

	if *encryptBackend == "AES-256" || controllerBackend == "AES-256" {
		return getAES256(controllerBackend)
	} else if *encryptBackend == "AWS-KMS" || controllerBackend == "AWS-KMS" {
		return getAWSKMS(controllerBackend)
	}

	return nil, nil, fmt.Errorf("invalid encryption backend: %s", *encryptBackend)

}

func getAES256(controllerBackend string) (backend ssbackend.Backend, providerData []byte, err error) {
	if *unseal && len(*privKeys) == 0 {
		return nil, nil, fmt.Errorf("must provide the --recovery-private-key to unseal using AES 256 backend")
	}
	if *encryptBackend != "" && *certURL == "" {
		return nil, nil, fmt.Errorf("must provide the --cert flag with AES 256 backend")
	}
	var pubKey *rsa.PublicKey
	if *certURL != "" {
		providerData, err = openCertLocal(*certURL)
		if err != nil {
			return nil, nil, err
		}
	}
	if controllerBackend != "" {
		providerData, err = openProvider()
		if err != nil {
			return nil, nil, err
		}
	}
	pubKey, err = parseKey(providerData)
	if err != nil {
		return nil, nil, err
	}
	privKeysMap, _ := readPrivKeys(*privKeys)
	backend = aes.NewAES256WithKey(pubKey, privKeysMap)
	return
}

func getAWSKMS(controllerBackend string) (backend ssbackend.Backend, providerData []byte, err error) {
	if *encryptBackend != "" && *awsKmsKeyID == "" {
		return nil, nil, fmt.Errorf("must provide the --aws-kms-key-id flag with AWS KMS key ID")
	}
	providerData = []byte(*awsKmsKeyID)
	if controllerBackend != "" {
		providerData, err = openProvider()
		if err != nil {
			return nil, nil, err
		}
	}
	backend, err = aws.NewKMS(string(providerData))
	if err != nil {
		return nil, nil, err
	}
	return
}
