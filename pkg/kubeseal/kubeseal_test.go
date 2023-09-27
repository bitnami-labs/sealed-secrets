package kubeseal

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/utils/strings/slices"

	flag "github.com/spf13/pflag"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/keyutil"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealedsecrets/v1alpha1"
	"github.com/bitnami-labs/sealed-secrets/pkg/crypto"
	certUtil "k8s.io/client-go/util/cert"
)

const testCert = `
-----BEGIN CERTIFICATE-----
MIIErTCCApWgAwIBAgIQBekz48i8NbrzIpIrLMIULTANBgkqhkiG9w0BAQsFADAA
MB4XDTE3MDYyMDA0MzI0NVoXDTI3MDYxODA0MzI0NVowADCCAiIwDQYJKoZIhvcN
AQEBBQADggIPADCCAgoCggIBAL6ISW4MnHAmC6MdmJOwo9C6YYhKYDwPD2tF+j4p
I2duB3y7DLF+zWNHgbUlBZck8CudacJTuxOJFEqr4umqm0f4EGgRPwZgFvFLHKSZ
/hxUFnMcGVhY1qsk55peSghPHarOYyBhhHDtCu7qdMu9MqPZB68y16HdPvwWPadI
dBKSxDLvwYfjDnG/ZHX9rmlDKej7jPGdvqAY5VJteP30w6YHb1Uc4whppNcDSc2l
gOuKAWtQ5WfZbB0NpMhj4framNeXMYwjZytEdC1c/4O45zm5eK4FNPueCfxOlzFQ
D3y34OuQlJwlrPE4KmdMHtE1a8x0ihbglInJrtqcXK3vEdUJ2c/BKWgFtPOTz6Du
jV4j0OMVVGnk5jUmh+yfbgielIkPcpSTWP1cIPwK3eWbrvMziq6sv0x7QoOD3Pzm
GBE8Y9sa5uy+bJZt5MywbamZ3xWaxoQbSN8RPoxRhTe0DEpx6utCXSWpapT7kWZ3
R1PTuVx+Ktyz7MRoDUWvxfpMJ2hsJ71Az0AuUZ4N4fmmGdUcM81GPUOiMZ4uqySQ
A2phgikbJaTzcT85RcNFYSi4eKc5mYFNqr5xVa6uHhZ+OGeGy1yyOEWLgIZV3A/8
4eZshOyYtRlZjCkaGZTfXNft+8QJi8rEZRcJtVhqLzezBVRsL7pt6P/mQj4+XHsE
VSBrAgMBAAGjIzAhMA4GA1UdDwEB/wQEAwIAATAPBgNVHRMBAf8EBTADAQH/MA0G
CSqGSIb3DQEBCwUAA4ICAQCSizqBB3bjHCSGk/8lpqIyHJQR5u4Cf7LRrC9U8mxe
pvC3Fx3/RlVe87Y4cUb37xZc/TmB6Bq10Y6R7ydS3oe8PCh4UQRnEfBgtJ6m59ha
t3iPX0NdQVYz/D+yEiHjpI7gpyFNuGkd4/78JE51SO4yGYvWk/ChHoMvbLcxzfdK
PI2Ymf3MWtGfoF/TQ1jy/Biy+qumDPSz23MynQG39cdUInSK26oemUbTH0koLulN
fNl4TwSEdSm2DRl0la+vkrzu7SvF9SJ2ES6wMWVjYiJLNpApjGuF9/ZOFw9DvSSH
m+UYXn+IC7rTgvXKvXTlG//z/14Lx0GFIY+ZjdENwLH//orBQLg37TZatKEpaWO6
uRzFUxZVw3ic3RxoHfEbRA9vQlQdKnV+BpZe/Pb08RAh82OZyujqqyK7cPPOW5Vi
T9y+NeMwfKH8H4un7mQWkgWFw3LMIspYY5uHWp6jBwU9u/mjoK4+Y219dkaAhAcx
D+YIZRXwxc6ehLCavGF2DIepybzDlJbiCe8JxUDsrE/Xkm6x28uq35oZ3UQznubU
7LfAeRSI99sNvFnq0TqhSlp+CUDs8Z1LvDXzAHX4UeZQl4g+H+w1KudCvjO0mPPp
R9bIjJLIvp7CQPDkdRzJSjvetrKtI0l97VjsjbRB9v6ZekGY9SFI49KzKUTk8fsF
/A==
-----END CERTIFICATE-----
`

var (
	testModulus  *big.Int
	testExponent = 65537
)

func init() {
	testModulus = new(big.Int)
	_, err := fmt.Sscan("777304254876434297689544225447769213262492599515515837291621795936355252933930193245809942636192119684040605554803489669141565417296821660595336672178414512660751886699171738066307588619202437848899334837760648051656982184646490661921128886671800776058692981991859399404705935722225294811424879738586269551402668122524371718537515440568440102201259925611463161144897905846190044735554045001999198442528435295995584980713050916813579912296878368079243909549993116827192901474611239264189340401059113919551426849847211275352102674049634252149163111599977742365280992561904350781270344655927564475032580504276518647106167707150111291732645399166011800154961975117045723373023335778593638216165426988399138193230056486079421256484837299169853958601000282124667227789126483641999102102039577368681983584245367307077546423870452524154641890843463963116237003367269116435430641427113406369059991147359641266708862913786891945896441771663010146473536372286482453315017377528517965715554550898957321536181165129538808789201530141159181590893764287807749414277289452691723903046140558704697831351834538780165261072894792900501671534138992265545905216973214953125367388406669893889742303072755608685449114438926280862339744991872488262084141163", testModulus)
	if err != nil {
		panic(err)
	}
}

func tmpfile(t *testing.T, contents []byte) string {
	f, err := os.CreateTemp("", "testdata")
	if err != nil {
		t.Fatalf("Failed to create tempfile: %v", err)
	}
	if _, err := f.Write(contents); err != nil {
		t.Fatalf("Failed to write to tempfile: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Failed to close tempfile: %v", err)
	}
	return f.Name()
}

func TestParseKey(t *testing.T) {
	key, err := ParseKey(strings.NewReader(testCert))
	if err != nil {
		t.Fatalf("Failed to parse test key: %v", err)
	}

	if key.N.Cmp(testModulus) != 0 {
		t.Errorf("Unexpected key modulus: %v", key.N)
	}

	if key.E != testExponent {
		t.Errorf("Unexpected key exponent: %v", key.E)
	}
}

/* repeated from main here... STARTs */

func initClient(kubeConfigPath string, cfgOverrides *clientcmd.ConfigOverrides, r io.Reader) clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	loadingRules.ExplicitPath = kubeConfigPath
	return clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, cfgOverrides, r)
}

func testClientConfig() clientcmd.ClientConfig {
	return initClient("", testConfigOverrides(), os.Stdin)
}

/* repeated from main here... ENDs */

func initUsualKubectlFlagsForTests(overrides *clientcmd.ConfigOverrides, flagset *flag.FlagSet) {
	kflags := clientcmd.RecommendedConfigOverrideFlags("")
	clientcmd.BindOverrideFlags(overrides, flagset, kflags)
}

func testConfigOverrides() *clientcmd.ConfigOverrides {
	flagset := flag.NewFlagSet("test", flag.PanicOnError)
	var overrides clientcmd.ConfigOverrides
	initUsualKubectlFlagsForTests(&overrides, flagset)
	err := flagset.Parse([]string{"-n", "default"})
	if err != nil {
		fmt.Printf("flagset parse err: %v\n", err)
		os.Exit(1)
	}
	return &overrides
}

func TestOpenCertFile(t *testing.T) {
	ctx := context.Background()
	clientConfig := testClientConfig()
	controllerNs := "default"
	controllerName := "controller"
	certFile := tmpfile(t, []byte(testCert))

	s := httptest.NewServer(http.FileServer(http.Dir(filepath.Dir(certFile))))
	defer s.Close()

	testCases := []string{
		certFile,
		fmt.Sprintf("%s/%s", s.URL, filepath.Base(certFile)),
		// This should work on windows but it causes a 500 error in the file handler. TODO: investigate
		//		(&url.URL{Scheme: "file", Path: path.Join("/", filepath.ToSlash(certFile))}).String(),
	}
	if goruntime.GOOS != "windows" {
		testCases = append(testCases, fmt.Sprintf("file://%s", certFile))
	}

	for _, certURL := range testCases {
		f, err := OpenCert(ctx, clientConfig, controllerNs, controllerName, certURL)
		if err != nil {
			t.Fatalf("Error reading test cert file: %v", err)
		}

		data, err := io.ReadAll(f)
		if err != nil {
			t.Fatalf("Error reading from test cert file: %v", err)
		}

		if string(data) != testCert {
			t.Errorf("Read incorrect data from cert file?!")
		}
	}
}

func TestSealWithMultiDocSecrets(t *testing.T) {
	key, err := ParseKey(strings.NewReader(testCert))
	if err != nil {
		t.Fatalf("Failed to parse gotSecrets key: %v", err)
	}

	testCases := []struct {
		name           string
		asYaml         bool
		inputSeparator string
		outputFormat   string
	}{
		{
			name:           "multi-doc json",
			asYaml:         false,
			inputSeparator: "\n",
			outputFormat:   "json",
		},
		{
			name:           "multi-doc yaml",
			asYaml:         true,
			inputSeparator: "---\n",
			outputFormat:   "yaml",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s1 := mkTestSecret(t, "foo", "1", withSecretName("s1"), asYAML(tc.asYaml))
			s2 := mkTestSecret(t, "bar", "2", withSecretName("s2"), asYAML(tc.asYaml))
			multiDocYaml := fmt.Sprintf("%s%s%s", s1, tc.inputSeparator, s2)

			clientConfig := testClientConfig()
			outputFormat := tc.outputFormat
			inbuf := bytes.Buffer{}
			_, err = bytes.NewBuffer([]byte(multiDocYaml)).WriteTo(&inbuf)
			if err != nil {
				t.Fatalf("Error writing to buffer: %v", err)
			}

			t.Logf("input is:\n%s", inbuf.String())

			outbuf := bytes.Buffer{}
			if err := Seal(clientConfig, outputFormat, &inbuf, &outbuf, scheme.Codecs, key, ssv1alpha1.NamespaceWideScope, false, "", ""); err != nil {
				t.Fatalf("seal() returned error: %v", err)
			}

			outBytes := outbuf.Bytes()
			t.Logf("output is:\n%s", outBytes)

			if tc.asYaml {
				if !strings.HasPrefix(string(outBytes), "---") {
					t.Errorf("YAML output should start with ---")
				}

				if strings.HasSuffix(string(outBytes), "---\n") {
					t.Errorf("YAML output should not end with ---")
				}
			}

			decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(outBytes), 4096)
			var gotSecrets []*ssv1alpha1.SealedSecret
			for {
				s := ssv1alpha1.SealedSecret{}
				err := decoder.Decode(&s)
				if err != nil {
					if err == io.EOF {
						break
					}
					t.Fatalf("Failed to parse result: %v", err)
				}
				gotSecrets = append(gotSecrets, &s)
			}

			if got, want := len(gotSecrets), 2; got != want {
				t.Errorf("Wrong element output length: got: %d, want: %d", got, want)
			}

			for _, gotSecret := range gotSecrets {
				if got, want := gotSecret.GetNamespace(), "testns"; got != want {
					t.Errorf("got: %q, want: %q", got, want)
				}
				if got, want := gotSecret.GetName(), []string{"s1", "s2"}; !slices.Contains(want, got) {
					t.Errorf("got: %q, want: %q", got, want)
				}
			}
		})
	}
}

func TestSeal(t *testing.T) {
	key, err := ParseKey(strings.NewReader(testCert))
	if err != nil {
		t.Fatalf("Failed to parse test key: %v", err)
	}

	testCases := []struct {
		secret v1.Secret
		scope  ssv1alpha1.SealingScope
		want   ssv1alpha1.SealedSecret // partial object
	}{
		{
			secret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysecret",
					Namespace: "myns",
				},
				Data: map[string][]byte{
					"foo": []byte("sekret"),
				},
				StringData: map[string]string{
					"foos": "stringsekret",
				},
			},
			want: ssv1alpha1.SealedSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysecret",
					Namespace: "myns",
				},
			},
		},
		{
			secret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mysecret",
				},
				Data: map[string][]byte{
					"foo": []byte("sekret"),
				},
			},
			want: ssv1alpha1.SealedSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysecret",
					Namespace: "default",
				},
			},
		},
		{
			secret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysecret",
					Namespace: "",
					Annotations: map[string]string{
						ssv1alpha1.SealedSecretNamespaceWideAnnotation: "true",
					},
				},
				Data: map[string][]byte{
					"foo": []byte("sekret"),
				},
			},
			want: ssv1alpha1.SealedSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysecret",
					Namespace: "default",
					Annotations: map[string]string{
						ssv1alpha1.SealedSecretNamespaceWideAnnotation: "true",
					},
				},
			},
		},
		{
			secret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysecret",
					Namespace: "",
					Annotations: map[string]string{
						ssv1alpha1.SealedSecretClusterWideAnnotation: "true",
					},
				},
				Data: map[string][]byte{
					"foo": []byte("sekret"),
				},
			},
			want: ssv1alpha1.SealedSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysecret",
					Namespace: "", // <--- we shouldn't force the default namespace for cluster wide secrets ...
					Annotations: map[string]string{
						ssv1alpha1.SealedSecretClusterWideAnnotation: "true",
					},
				},
			},
		},
		{
			secret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysecret",
					Namespace: "myns",
					Annotations: map[string]string{
						ssv1alpha1.SealedSecretClusterWideAnnotation: "true",
					},
				},
				Data: map[string][]byte{
					"foo": []byte("sekret"),
				},
			},
			want: ssv1alpha1.SealedSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysecret",
					Namespace: "myns", // <--- ... but we should preserve one if specified.
					Annotations: map[string]string{
						ssv1alpha1.SealedSecretClusterWideAnnotation: "true",
					},
				},
			},
		},
		{
			secret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysecret",
					Namespace: "",
				},
				Data: map[string][]byte{
					"foo": []byte("sekret"),
				},
			},
			scope: ssv1alpha1.NamespaceWideScope,
			want: ssv1alpha1.SealedSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysecret",
					Namespace: "default",
					Annotations: map[string]string{
						ssv1alpha1.SealedSecretNamespaceWideAnnotation: "true",
					},
				},
			},
		},
		{
			secret: v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysecret",
					Namespace: "",
				},
				Data: map[string][]byte{
					"foo": []byte("sekret"),
				},
			},
			scope: ssv1alpha1.ClusterWideScope,
			want: ssv1alpha1.SealedSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysecret",
					Namespace: "",
					Annotations: map[string]string{
						ssv1alpha1.SealedSecretClusterWideAnnotation: "true",
					},
				},
			},
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			clientConfig := testClientConfig()
			outputFormat := "json"
			info, ok := runtime.SerializerInfoForMediaType(scheme.Codecs.SupportedMediaTypes(), runtime.ContentTypeJSON)
			if !ok {
				t.Fatalf("binary can't serialize JSON")
			}
			enc := scheme.Codecs.EncoderForVersion(info.Serializer, v1.SchemeGroupVersion)
			inbuf := bytes.Buffer{}
			if err := enc.Encode(&tc.secret, &inbuf); err != nil {
				t.Fatalf("Error encoding: %v", err)
			}

			t.Logf("input is: %s", inbuf.String())

			outbuf := bytes.Buffer{}
			if err := Seal(clientConfig, outputFormat, &inbuf, &outbuf, scheme.Codecs, key, tc.scope, false, "", ""); err != nil {
				t.Fatalf("seal() returned error: %v", err)
			}

			outBytes := outbuf.Bytes()
			t.Logf("output is %s", outBytes)

			var result ssv1alpha1.SealedSecret
			if err = runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), outBytes, &result); err != nil {
				t.Fatalf("Failed to parse result: %v", err)
			}

			smeta := result.GetObjectMeta()
			if got, want := smeta.GetName(), tc.want.GetName(); got != want {
				t.Errorf("got: %q, want: %q", got, want)
			}
			if got, want := smeta.GetNamespace(), tc.want.GetNamespace(); got != want {
				t.Errorf("got: %q, want: %q", got, want)
			}
			if got, want := smeta.GetAnnotations(), tc.want.GetAnnotations(); !cmp.Equal(got, want, cmpopts.EquateEmpty()) {
				t.Errorf("got: %q, want: %q", got, want)
			}

			for n := range tc.secret.Data {
				if len(result.Spec.EncryptedData[n]) < 100 {
					t.Errorf("Encrypted data is implausibly short: %v", result.Spec.EncryptedData[n])
				}
			}
			for n := range tc.secret.StringData {
				if len(result.Spec.EncryptedData[n]) < 100 {
					t.Errorf("Encrypted data is implausibly short: %v", result.Spec.EncryptedData[n])
				}
			}
			// NB: See sealedsecret_test.go for e2e crypto test
		})
	}
}

type mkTestSecretOpt func(*mkTestSecretOpts)
type mkTestSecretOpts struct {
	secretName      string
	secretNamespace string
	asYAML          bool
}

func withSecretName(n string) mkTestSecretOpt {
	return func(o *mkTestSecretOpts) {
		o.secretName = n
	}
}

func withSecretNamespace(n string) mkTestSecretOpt {
	return func(o *mkTestSecretOpts) {
		o.secretNamespace = n
	}
}

func asYAML(y bool) mkTestSecretOpt {
	return func(o *mkTestSecretOpts) {
		o.asYAML = y
	}
}

func mkTestSecret(t *testing.T, key, value string, opts ...mkTestSecretOpt) []byte {
	o := mkTestSecretOpts{
		secretName:      "testname",
		secretNamespace: "testns",
	}
	for _, opt := range opts {
		opt(&o)
	}

	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.secretName,
			Namespace: o.secretNamespace,
			Annotations: map[string]string{
				key: value, // putting secret here just to have a simple way to test annotation merges
			},
			Labels: map[string]string{
				key: value,
			},
		},
		Data: map[string][]byte{
			key: []byte(value),
		},
	}

	contentType := runtime.ContentTypeJSON
	if o.asYAML {
		contentType = runtime.ContentTypeYAML
	}

	info, ok := runtime.SerializerInfoForMediaType(scheme.Codecs.SupportedMediaTypes(), contentType)
	if !ok {
		t.Fatalf("binary can't serialize JSON")
	}
	enc := scheme.Codecs.EncoderForVersion(info.Serializer, v1.SchemeGroupVersion)
	var inbuf bytes.Buffer
	if err := enc.Encode(&secret, &inbuf); err != nil {
		t.Fatalf("Error encoding: %v", err)
	}
	return inbuf.Bytes()
}

func mkTestSealedSecret(t *testing.T, pubKey *rsa.PublicKey, key, value string, opts ...mkTestSecretOpt) []byte {
	clientConfig := testClientConfig()
	outputFormat := "json"
	inbuf := bytes.NewBuffer(mkTestSecret(t, key, value, opts...))
	var outbuf bytes.Buffer
	if err := Seal(clientConfig, outputFormat, inbuf, &outbuf, scheme.Codecs, pubKey, ssv1alpha1.DefaultScope, false, "", ""); err != nil {
		t.Fatalf("seal() returned error: %v", err)
	}

	return outbuf.Bytes()
}

// TODO(mkm): rename newTestKeyPair to newTestKeyPairs.
func newTestKeyPair(t *testing.T) (*rsa.PublicKey, map[string]*rsa.PrivateKey) {
	privKey, _, err := crypto.GeneratePrivateKeyAndCert(2048, time.Hour, "testcn")
	if err != nil {
		t.Fatal(err)
	}
	pubKey := &privKey.PublicKey

	fp, err := crypto.PublicKeyFingerprint(pubKey)
	if err != nil {
		t.Fatal(err)
	}
	privKeys := map[string]*rsa.PrivateKey{fp: privKey}

	return pubKey, privKeys
}

func TestUnseal(t *testing.T) {
	pubKey, privKeys := newTestKeyPair(t)
	pkFile, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(pkFile.Name())

	if len(privKeys) != 1 {
		t.Fatal("assuming only one test key-pair")
	}
	for _, key := range privKeys {
		err := pem.Encode(pkFile, &pem.Block{Type: keyutil.RSAPrivateKeyBlockType, Bytes: x509.MarshalPKCS1PrivateKey(key)})
		if err != nil {
			t.Fatal(err)
		}
	}
	pkFile.Close()

	const (
		secretItemKey   = "foo"
		secretItemValue = "secret1"
	)
	ss := mkTestSealedSecret(t, pubKey, secretItemKey, secretItemValue)

	var buf bytes.Buffer
	privKeysList := []string{pkFile.Name()}
	outputFormat := "json"
	if err := UnsealSealedSecret(&buf, bytes.NewBuffer(ss), privKeysList, outputFormat, scheme.Codecs); err != nil {
		t.Fatal(err)
	}

	secret, err := readSecrets(&buf)
	if err != nil {
		t.Fatal(err)
	}

	for _, secret := range secret {
		if got, want := string(secret.Data[secretItemKey]), secretItemValue; got != want {
			t.Fatalf("got: %q, want: %q", got, want)
		}
	}
}

func TestUnsealList(t *testing.T) {
	pubKey, privKeys := newTestKeyPair(t)
	pkFile, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(pkFile.Name())

	// encode a v1.List containing all the privKeys into one file.
	prettyEnc, err := prettyEncoder(scheme.Codecs, runtime.ContentTypeJSON, v1.SchemeGroupVersion)
	if err != nil {
		t.Fatal(err)
	}

	var secrets [][]byte
	for _, key := range privKeys {
		b := pem.EncodeToMemory(&pem.Block{Type: keyutil.RSAPrivateKeyBlockType, Bytes: x509.MarshalPKCS1PrivateKey(key)})
		buf, err := runtime.Encode(prettyEnc, &v1.Secret{Data: map[string][]byte{"tls.key": b}})
		if err != nil {
			t.Fatal(err)
		}
		secrets = append(secrets, buf)
	}
	lst := &v1.List{}
	for _, s := range secrets {
		lst.Items = append(lst.Items, runtime.RawExtension{Raw: s})
	}
	blst, err := runtime.Encode(prettyEnc, lst)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pkFile.Write(blst); err != nil {
		t.Fatal(err)
	}
	pkFile.Close()

	const (
		secretItemKey   = "foo"
		secretItemValue = "secret1"
	)
	ss := mkTestSealedSecret(t, pubKey, secretItemKey, secretItemValue)

	var buf bytes.Buffer
	privKeysList := []string{pkFile.Name()}
	outputFormat := "json"
	if err := UnsealSealedSecret(&buf, bytes.NewBuffer(ss), privKeysList, outputFormat, scheme.Codecs); err != nil {
		t.Fatal(err)
	}

	secret, err := readSecrets(&buf)
	if err != nil {
		t.Fatal(err)
	}

	for _, secret := range secret {
		if got, want := string(secret.Data[secretItemKey]), secretItemValue; got != want {
			t.Fatalf("got: %q, want: %q", got, want)
		}
	}
}

func TestMergeInto(t *testing.T) {
	clientConfig := testClientConfig()
	outputFormat := "json"
	pubKey, privKeys := newTestKeyPair(t)

	merge := func(t *testing.T, newSecret, oldSealedSecret []byte) *ssv1alpha1.SealedSecret {
		f, err := os.CreateTemp("", "*.json")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(oldSealedSecret); err != nil {
			t.Fatal(err)
		}
		f.Close()

		buf := bytes.NewBuffer(newSecret)
		if err := SealMergingInto(clientConfig, outputFormat, buf, f.Name(), scheme.Codecs, pubKey, ssv1alpha1.DefaultScope, false); err != nil {
			t.Fatal(err)
		}

		b, err := os.ReadFile(f.Name())
		if err != nil {
			t.Fatal(err)
		}

		merged, err := decodeSealedSecret(scheme.Codecs, b)
		if err != nil {
			t.Fatal(err)
		}

		_, err = merged.Unseal(scheme.Codecs, privKeys)
		if err != nil {
			t.Fatal(err)
		}

		return merged
	}

	t.Run("added", func(t *testing.T) {
		merged := merge(t,
			mkTestSecret(t, "foo", "secret1"),
			mkTestSealedSecret(t, pubKey, "bar", "secret2"),
		)

		checkAdded := func(m map[string]string, old, new string) {
			if got, want := len(m), 2; got != want {
				t.Fatalf("got: %d, want: %d", got, want)
			}

			if _, ok := m[old]; !ok {
				t.Fatalf("cannot find expected key")
			}

			if _, ok := m[new]; !ok {
				t.Fatalf("cannot find expected key")
			}
		}

		checkAdded(merged.Spec.EncryptedData, "foo", "bar")
		checkAdded(merged.Spec.Template.Annotations, "foo", "bar")
		checkAdded(merged.Spec.Template.Labels, "foo", "bar")
	})

	t.Run("updated", func(t *testing.T) {
		origSrc := mkTestSealedSecret(t, pubKey, "foo", "secret2")
		orig, err := decodeSealedSecret(scheme.Codecs, origSrc)
		if err != nil {
			t.Fatal(err)
		}

		merged := merge(t,
			mkTestSecret(t, "foo", "secret1"),
			origSrc,
		)

		checkUpdated := func(before, after map[string]string, key string) {
			if got, want := len(after), 1; got != want {
				t.Fatalf("got: %d, want: %d", got, want)
			}

			if old, new := before[key], after[key]; old == new {
				t.Fatalf("expecting %q and %q to be different", old, new)
			}
		}

		checkUpdated(orig.Spec.EncryptedData, merged.Spec.EncryptedData, "foo")
		checkUpdated(orig.Spec.Template.Annotations, merged.Spec.Template.Annotations, "foo")
		checkUpdated(orig.Spec.Template.Labels, merged.Spec.Template.Labels, "foo")
	})

	t.Run("bad name", func(t *testing.T) {
		// should not fail even if input has a bad secret name because the name in existing existing sealed secret
		// should win (same for namespace).
		// TODO(mkm): test for case with scope mismatch too.
		merge(t,
			mkTestSecret(t, "foo", "secret1", withSecretName("badname"), withSecretNamespace("badns")),
			mkTestSealedSecret(t, pubKey, "bar", "secret2"),
		)
	})
}

// writeTempFile creates a temporary file, writes data into it and closes it.
func writeTempFile(b []byte) (string, error) {
	tmp, err := os.CreateTemp("", "")
	if err != nil {
		return "", err
	}
	defer tmp.Close()

	if _, err := tmp.Write(b); err != nil {
		os.RemoveAll(tmp.Name())
		return "", err
	}

	return tmp.Name(), nil
}

// testingKeypairFiles returns a path to a PEM encoded certificate and a PEM encoded private key
// along with a function to be called to cleanup those files.
func testingKeypairFiles(t *testing.T) (string, string, func()) {
	_, pk := newTestKeyPairSingle(t)

	cert, err := crypto.SignKey(rand.Reader, pk, time.Hour, "testcn")
	if err != nil {
		t.Fatal(err)
	}

	certFile, err := writeTempFile(pem.EncodeToMemory(&pem.Block{Type: certUtil.CertificateBlockType, Bytes: cert.Raw}))
	if err != nil {
		t.Fatal(err)
	}

	pkPEM, err := keyutil.MarshalPrivateKeyToPEM(pk)
	if err != nil {
		t.Fatal(err)
	}
	pkFile, err := writeTempFile(pkPEM)
	if err != nil {
		t.Fatal(err)
	}

	return certFile, pkFile, func() {
		os.RemoveAll(certFile)
		os.RemoveAll(pkFile)
	}
}

func sealTestItem(certFilename, secretNS, secretName, secretValue string, scope ssv1alpha1.SealingScope) (string, error) {
	var buf bytes.Buffer

	ctx := context.Background()
	clientConfig := testClientConfig()
	controllerNs := "default"
	controllerName := "controller"
	f, err := OpenCert(ctx, clientConfig, controllerNs, controllerName, certFilename)
	if err != nil {
		return "", err
	}
	defer f.Close()
	pubKey, err := ParseKey(f)
	if err != nil {
		return "", err
	}

	if err := EncryptSecretItem(&buf, secretName, secretNS, []byte(secretValue), scope, pubKey); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func TestRaw(t *testing.T) {
	certFilename, privKeyFilename, cleanup := testingKeypairFiles(t)
	defer cleanup()

	const (
		secretNS    = "myns"
		secretName  = "mysecret"
		secretItem  = "foo"
		secretValue = "supersecret"
	)

	testCases := []struct {
		ns        string
		name      string
		scope     ssv1alpha1.SealingScope
		unsealErr string
	}{
		// strict scope
		{ns: secretNS, name: secretName},
		{ns: "youGiveRest", name: secretName, unsealErr: "no key could decrypt secret"},
		{ns: secretNS, name: "aBadName", unsealErr: "no key could decrypt secret"},

		// namespace-wide scope
		{scope: ssv1alpha1.NamespaceWideScope, ns: secretNS, name: secretName},
		{scope: ssv1alpha1.NamespaceWideScope, ns: "youGiveRest", unsealErr: "no key could decrypt secret"},
		{scope: ssv1alpha1.NamespaceWideScope, ns: "youGiveRest", name: "aBadName", unsealErr: "no key could decrypt secret"},
		{scope: ssv1alpha1.NamespaceWideScope, ns: secretNS, name: ""},
		{scope: ssv1alpha1.NamespaceWideScope, ns: secretNS, name: "aBadName"},

		// cluster-wide scope
		{scope: ssv1alpha1.ClusterWideScope, ns: secretNS, name: secretName},
		{scope: ssv1alpha1.ClusterWideScope, ns: "youGiveRest", name: secretName},
		{scope: ssv1alpha1.ClusterWideScope, ns: secretNS, name: ""},
		{scope: ssv1alpha1.ClusterWideScope, ns: secretNS, name: "aBadName"},
		{scope: ssv1alpha1.ClusterWideScope, ns: "", name: ""},
		{scope: ssv1alpha1.ClusterWideScope, ns: "", name: "aBadName"},
	}

	for i, tc := range testCases {
		// encrypt an item with data from the testCase and put it
		// in a sealed secret with the metadata from the constants above
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			enc, err := sealTestItem(certFilename, tc.ns, tc.name, secretValue, tc.scope)
			if err != nil {
				t.Fatal(err)
			}

			ss := &ssv1alpha1.SealedSecret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: secretNS,
					Name:      secretName,
					Annotations: map[string]string{
						fmt.Sprintf("sealedsecrets.bitnami.com/%s", tc.scope.String()): "true",
					},
				},
				Spec: ssv1alpha1.SealedSecretSpec{
					EncryptedData: map[string]string{
						secretItem: enc,
					},
				},
			}

			privKeys, err := readPrivKeys([]string{privKeyFilename})
			if err != nil {
				t.Fatal(err)
			}
			sec, err := ss.Unseal(scheme.Codecs, privKeys)
			if tc.unsealErr != "" {
				if got, want := err.Error(), tc.unsealErr; !strings.HasPrefix(got, want) {
					t.Fatalf("got: %v, want: %v", err, want)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}

			if got, want := string(sec.Data[secretItem]), secretValue; got != want {
				t.Errorf("got: %q, want: %q", got, want)
			}
		})
	}
}

func newTestKeyPairSingle(t *testing.T) (*rsa.PublicKey, *rsa.PrivateKey) {
	privKey, _, err := crypto.GeneratePrivateKeyAndCert(2048, time.Hour, "testcn")
	if err != nil {
		t.Fatal(err)
	}
	return &privKey.PublicKey, privKey
}

func TestReadPrivKeySecret(t *testing.T) {
	outputFormat := "json"
	_, pkw := newTestKeyPairSingle(t)

	b, err := keyutil.MarshalPrivateKeyToPEM(pkw)
	if err != nil {
		t.Fatal(err)
	}

	sec := &v1.Secret{
		Data: map[string][]byte{
			"tls.key": b,
		},
	}

	tmp, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	// defer os.RemoveAll(tmp.Name())

	if err := resourceOutput(tmp, outputFormat, scheme.Codecs, v1.SchemeGroupVersion, sec); err != nil {
		t.Fatal(err)
	}
	tmp.Close()

	pkr, err := readPrivKey(tmp.Name())
	if err != nil {
		t.Fatal(err)
	}

	if got, want := pkr.D.String(), pkw.D.String(); got != want {
		t.Errorf("got: %q, want: %q", got, want)
	}
}

func TestReadPrivKeyPEM(t *testing.T) {
	_, pkw := newTestKeyPairSingle(t)

	b, err := keyutil.MarshalPrivateKeyToPEM(pkw)
	if err != nil {
		t.Fatal(err)
	}
	tmp, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp.Name())

	if _, err := tmp.Write(b); err != nil {
		t.Fatal(err)
	}
	tmp.Close()

	pkr, err := readPrivKey(tmp.Name())
	if err != nil {
		t.Fatal(err)
	}

	if got, want := pkr.D.String(), pkw.D.String(); got != want {
		t.Errorf("got: %q, want: %q", got, want)
	}
}
