package kubeseal

import (
	"bytes"
	"crypto/rsa"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/bitnami-labs/sealed-secrets/pkg/client/clientset/versioned/scheme"
	"github.com/bitnami-labs/sealed-secrets/pkg/crypto"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/util/keyutil"
)

type UnsealSealedSecretInstruction struct {
	OutputFormat     string
	Out              io.Writer
	In               io.Reader
	Codecs           runtimeserializer.CodecFactory
	PrivKeyFilenames []string
}

func ParsePrivKey(b []byte) (*rsa.PrivateKey, error) {
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

func ReadPrivKeysFromFile(filename string) ([]*rsa.PrivateKey, error) {
	// #nosec G304 -- should open user provided file
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	res, err := ParsePrivKey(b)
	if err == nil {
		return []*rsa.PrivateKey{res}, nil
	}

	var secrets []*v1.Secret

	// try to parse it as json/yaml encoded v1.List of secrets
	var lst v1.List
	if err = runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), b, &lst); err == nil {
		for _, r := range lst.Items {
			s, err := ReadSecret(scheme.Codecs.UniversalDecoder(), bytes.NewBuffer(r.Raw))
			if err != nil {
				return nil, err
			}
			secrets = append(secrets, s)
		}
	} else {
		// try to parse it as json/yaml encoded secret
		s, err := ReadSecret(scheme.Codecs.UniversalDecoder(), bytes.NewBuffer(b))
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
		pk, err := ParsePrivKey(tlsKey)
		if err != nil {
			return nil, err
		}
		keys = append(keys, pk)
	}

	return keys, nil
}

func ReadPrivKeys(filenames []string) (map[string]*rsa.PrivateKey, error) {
	res := map[string]*rsa.PrivateKey{}
	for _, filename := range filenames {
		pks, err := ReadPrivKeysFromFile(filename)
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

func Unseal(i UnsealSealedSecretInstruction) error {
	privKeys, err := ReadPrivKeys(i.PrivKeyFilenames)
	if err != nil {
		return err
	}

	b, err := ioutil.ReadAll(i.In)
	if err != nil {
		return err
	}

	ss, err := DecodeSealedSecret(i.Codecs, b)
	if err != nil {
		return err
	}

	sec, err := ss.Unseal(i.Codecs, privKeys)
	if err != nil {
		return err
	}

	return ResourceOutput(i.Out, i.Codecs, v1.SchemeGroupVersion, sec, i.OutputFormat)
}
