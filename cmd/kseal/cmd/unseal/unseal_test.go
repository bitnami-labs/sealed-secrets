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
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/bitnami-labs/sealed-secrets/pkg/crypto"
	v1 "k8s.io/api/core/v1"
	certUtil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
)

func Test_parsePrivKey(t *testing.T) {
	// generate new private key
	key, _, _ := crypto.GeneratePrivateKeyAndCert(2046, 10*365*24*time.Hour, "sealed.secrets")
	// encode new private key
	encodedKey := pem.EncodeToMemory(&pem.Block{Type: keyutil.RSAPrivateKeyBlockType, Bytes: x509.MarshalPKCS1PrivateKey(key)})

	// test suite
	tests := []struct {
		name    string
		args    []byte
		want    *rsa.PrivateKey
		wantErr bool
	}{
		{
			name:    "Parse a valid encoded private key",
			args:    encodedKey,
			want:    key,
			wantErr: false,
		},
		{
			name:    "Parse a valid private key that's not encoded",
			args:    []byte(fmt.Sprint(key)),
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		tt := tt // overshadows loop variable tt to be able to run parallel tests
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePrivKey(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePrivKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parsePrivKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseSecret(t *testing.T) {
	// generate new private key and cert
	key, cert, _ := crypto.GeneratePrivateKeyAndCert(2046, 10*365*24*time.Hour, "sealed.secrets")
	// create Secret base on the private key and cert
	certs := []*x509.Certificate{cert}
	certbytes := []byte{}
	for _, cert := range certs {
		certbytes = append(certbytes, pem.EncodeToMemory(&pem.Block{Type: certUtil.CertificateBlockType, Bytes: cert.Raw})...)
	}
	secret := v1.Secret{
		Data: map[string][]byte{
			v1.TLSPrivateKeyKey: pem.EncodeToMemory(&pem.Block{Type: keyutil.RSAPrivateKeyBlockType, Bytes: x509.MarshalPKCS1PrivateKey(key)}),
			v1.TLSCertKey:       certbytes,
		},
		Type: v1.SecretTypeTLS,
	}
	// marshal secret
	secretBytes, _ := json.Marshal(secret)

	// test suite
	tests := []struct {
		name    string
		args    io.Reader
		want    *v1.Secret
		wantErr bool
	}{
		{
			name:    "Parse a valid Secret including private key",
			args:    bytes.NewReader(secretBytes),
			want:    &secret,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSecret(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseSecret() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseSecret() = %v, want %v", got, tt.want)
			}
		})
	}
}
