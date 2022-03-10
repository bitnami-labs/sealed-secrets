/*
Copyright 2022 - Bitnami <containers@bitnami.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package unseal

import (
	"bytes"
	"crypto/rsa"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/keyutil"

	"github.com/bitnami-labs/sealed-secrets/cmd/kseal/pkg/utils"
	"github.com/bitnami-labs/sealed-secrets/pkg/crypto"
	"github.com/bitnami-labs/sealed-secrets/pkg/multidocyaml"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealed-secrets/v1alpha1"
)

// NewCmdUnseal creates a command object for the "unseal" action.
func NewCmdUnseal() *cobra.Command {
	unsealCmd := &cobra.Command{
		Use:   "unseal",
		Short: "Unseal a Sealed Secret",
		Long: `Unseal a Sealed Secret consulting the Sealed Secrets controller API

Examples:

    kseal unseal -k privkeys.json -f mysealedsecret.json            Unseal Sealed Secret from file
    cat mysealedsecret.json | kseal unseal -k privkeys.json         Unseal Sealed Secret from stdin
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			filename, _ := cmd.Flags().GetString("filename")
			format, _ := cmd.Flags().GetString("format")
			keys, _ := cmd.Flags().GetStringSlice("keys")
			var input io.Reader
			if filename == "" || filename == "-" {
				if isatty.IsTerminal(os.Stdin.Fd()) {
					return fmt.Errorf("tty detected: expecting json/yaml k8s resource in stdin")
				}
				input = os.Stdin
			} else {
				f, err := os.Open(filename)
				if err != nil {
					return fmt.Errorf("cannot open file: %v", err)
				}
				defer f.Close()
				input = f
			}
			err := unseal(input, os.Stdout, format, keys)
			if err != nil {
				return err
			}
			return nil
		},
	}

	// Flags
	unsealCmd.Flags().StringSliceP("keys", "k", nil, "List of filenames that contain the private keys to use to unseal Sealed Secrets objects. Either PEM encoded private keys or a backup of a json/yaml encoded K8s sealed-secret controller secret (and v1.List) are accepted.")
	unsealCmd.Flags().StringP("filename", "f", "", "Filename that contains the Sealed Secrets object to unseal")
	unsealCmd.Flags().StringP("output", "o", "json", "Output format.  Supported values are: json, yaml")
	unsealCmd.MarkFlagRequired("keys")

	return unsealCmd
}

// parsePrivKey parses a private key in PEM encoded format
func parsePrivKey(b []byte) (*rsa.PrivateKey, error) {
	key, err := keyutil.ParsePrivateKeyPEM(b)
	if err != nil {
		return nil, fmt.Errorf("error parsing private key: %v", err)
	}
	switch rsaKey := key.(type) {
	case *rsa.PrivateKey:
		return rsaKey, nil
	default:
		return nil, fmt.Errorf("unexpected private key type %T", key)
	}
}

// parseSecret parses an input as a json/yaml encoded K8s Secret
func parseSecret(in io.Reader) (*v1.Secret, error) {
	data, err := ioutil.ReadAll(in)
	if err != nil {
		return nil, fmt.Errorf("error parsing secret: %v", err)
	}
	if err := multidocyaml.EnsureNotMultiDoc(data); err != nil {
		return nil, fmt.Errorf("error parsing secret: %v", err)
	}

	var secret v1.Secret
	if err = runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), data, &secret); err != nil {
		return nil, fmt.Errorf("error parsing secret: %v", err)
	}
	return &secret, nil
}

// readPrivKeysFromFile read the private keys available in certain file
func readPrivKeysFromFile(filename string) ([]*rsa.PrivateKey, error) {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading private keys: %v", err)
	}

	// try to parse keys as a PEM encoded private key
	if res, err := parsePrivKey(b); err == nil {
		return []*rsa.PrivateKey{res}, nil
	}

	var secrets []*v1.Secret
	// try to parse it as json/yaml encoded v1.List of secrets
	var lst v1.List
	if err = runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), b, &lst); err == nil {
		for _, r := range lst.Items {
			s, err := parseSecret(bytes.NewBuffer(r.Raw))
			if err != nil {
				return nil, fmt.Errorf("error reading private keys: %v", err)
			}
			secrets = append(secrets, s)
		}
	} else {
		// try to parse it as json/yaml encoded secret
		s, err := parseSecret(bytes.NewBuffer(b))
		if err != nil {
			return nil, fmt.Errorf("error reading private keys: %v", err)
		}
		secrets = append(secrets, s)
	}

	// obtain private keys from secrets
	var keys []*rsa.PrivateKey
	for _, s := range secrets {
		tlsKey, ok := s.Data["tls.key"]
		if !ok {
			return nil, fmt.Errorf("secret must contain a 'tls.data' key")
		}
		pk, err := parsePrivKey(tlsKey)
		if err != nil {
			return nil, fmt.Errorf("error reading private keys: %v", err)
		}
		keys = append(keys, pk)
	}
	return keys, nil
}

// readPrivKeys reads the private keys to use to decrypt Sealed Secrets from a series of files
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

// parseSealedSecret parses an input as a Sealed Secret
func parseSealedSecret(in io.Reader) (*ssv1alpha1.SealedSecret, error) {
	b, err := ioutil.ReadAll(in)
	if err != nil {
		return nil, fmt.Errorf("error parsing sealed secret: %v", err)
	}
	var ss ssv1alpha1.SealedSecret
	if err := runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), b, &ss); err != nil {
		return nil, fmt.Errorf("error parsing sealed secret: %v", err)
	}
	return &ss, nil
}

// unseal decrypts a Sealed Secrets using on a series of private keys
func unseal(in io.Reader, out io.Writer, format string, privKeyFilenames []string) error {
	privKeys, err := readPrivKeys(privKeyFilenames)
	if err != nil {
		return fmt.Errorf("cannot obtain private keys: %v", err)
	}

	ss, err := parseSealedSecret(in)
	if err != nil {
		return fmt.Errorf("cannot obtain object to unseal: %v", err)
	}

	sec, err := ss.Unseal(scheme.Codecs, privKeys)
	if err != nil {
		return err
	}
	return utils.PrintResource(format, out, sec, v1.SchemeGroupVersion)
}
