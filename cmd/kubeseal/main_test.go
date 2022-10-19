package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	goflag "flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealedsecrets/v1alpha1"
	"github.com/bitnami-labs/sealed-secrets/pkg/crypto"
	"github.com/bitnami-labs/sealed-secrets/pkg/kubeseal"
	flag "github.com/spf13/pflag"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	certUtil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
)

func TestVersion(t *testing.T) {
	buf := bytes.NewBufferString("")
	testVersionFlags := flag.NewFlagSet("testVersionFlags", flag.ExitOnError)
	nopFlags := goflag.NewFlagSet("nop", goflag.ExitOnError)
	err := mainE(buf, testVersionFlags, nopFlags, []string{"--version"})
	if err != nil {
		t.Fatal(err)
	}

	if got, want := buf.String(), "kubeseal version: UNKNOWN\n"; got != want {
		t.Errorf("got: %q, want: %q", got, want)
	}
}

func testClientConfig() clientcmd.ClientConfig {
	return initClient("", testConfigOverrides(), os.Stdin)
}

func testConfig(flags *cliFlags) *config {
	clientConfig := testClientConfig()
	return &config{
		flags:        flags,
		clientConfig: clientConfig,
		ctx:          context.Background(),
	}
}

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

func TestMainError(t *testing.T) {
	badFileName := filepath.Join("this", "file", "cannot", "possibly", "exist", "can", "it?")
	flags := cliFlags{certURL: badFileName}

	err := runCLI(io.Discard, testConfig(&flags))
	if err == nil || !os.IsNotExist(err) {
		t.Fatalf("expecting not exist error, got: %v", err)
	}
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

func newTestKeyPairSingle(t *testing.T) (*rsa.PublicKey, *rsa.PrivateKey) {
	privKey, _, err := crypto.GeneratePrivateKeyAndCert(2048, time.Hour, "testcn")
	if err != nil {
		t.Fatal(err)
	}
	return &privKey.PublicKey, privKey
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

func TestWriteToFile(t *testing.T) {
	certFilename, _, cleanup := testingKeypairFiles(t)
	defer cleanup()

	in, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(in.Name())
	fmt.Fprintf(in, `apiVersion: v1
kind: Secret
metadata:
  name: foo
  namespace: bar
data:
  super: c2VjcmV0
`)
	in.Close()

	out, err := os.CreateTemp("", "*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	out.Close()
	defer os.RemoveAll(out.Name())

	var buf bytes.Buffer
	flags := cliFlags{
		inputFileName:  in.Name(),
		outputFileName: out.Name(),
		certURL:        certFilename,
	}

	if err := runCLI(&buf, testConfig(&flags)); err != nil {
		t.Fatal(err)
	}

	if got, want := buf.Len(), 0; got != want {
		t.Errorf("got: %d, want: %d", got, want)
	}

	b, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	if sub := "kind: SealedSecret"; !bytes.Contains(b, []byte(sub)) {
		t.Errorf("expecting to find %q in %q", sub, b)
	}
}

func TestFailToWriteToFile(t *testing.T) {
	certFilename, _, cleanup := testingKeypairFiles(t)
	defer cleanup()

	in, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(in.Name())
	fmt.Fprintf(in, `apiVersion: v1
kind: BadInput
metadata:
  name: foo
  namespace: bar
`)
	in.Close()

	out, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}

	// if sealing error happens, the old content of the output file shouldn't be truncated.
	const testOldContent = "previous content"

	fmt.Fprint(out, testOldContent)
	out.Close()
	defer os.RemoveAll(out.Name())

	var buf bytes.Buffer
	flags := cliFlags{
		inputFileName:  in.Name(),
		outputFileName: out.Name(),
		certURL:        certFilename,
	}

	if err := runCLI(&buf, testConfig(&flags)); err == nil {
		t.Errorf("expecting error")
	}

	if got, want := buf.Len(), 0; got != want {
		t.Errorf("got: %d, want: %d", got, want)
	}

	b, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(b), testOldContent; got != want {
		t.Errorf("got: %q, want: %q", got, want)
	}
}

func Test_runCLI(t *testing.T) {
	type args struct {
		cfg *config
	}
	tests := []struct {
		name    string
		args    args
		wantW   string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &bytes.Buffer{}
			if err := runCLI(w, tt.args.cfg); (err != nil) != tt.wantErr {
				t.Errorf("runCLI() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotW := w.String(); gotW != tt.wantW {
				t.Errorf("runCLI() = %v, want %v", gotW, tt.wantW)
			}
		})
	}
}

type tweakedClientConfig struct {
	ccfg      kubeseal.ClientConfig
	namespace string
}

func (tcc *tweakedClientConfig) Namespace() (string, bool, error) {
	return tcc.namespace, false, nil
}

func (tcc *tweakedClientConfig) ClientConfig() (*rest.Config, error) {
	return tcc.ccfg.ClientConfig()
}

func trySealTestItem(certFilename, secretNS, secretName, secretValue string, scope ssv1alpha1.SealingScope) (string, error) {
	dataFile, err := writeTempFile([]byte(secretValue))
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dataFile)

	fromFile := []string{dataFile}
	var buf bytes.Buffer
	flags := cliFlags{
		sealingScope: scope,
		secretName:   secretName,
		certURL:      certFilename,
		raw:          true,
		fromFile:     fromFile,
	}
	cfg := testConfig(&flags)
	cfg.clientConfig = &tweakedClientConfig{cfg.clientConfig, secretNS}

	if err := runCLI(&buf, cfg); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func TestRawSealErrors(t *testing.T) {
	certFilename, _, cleanup := testingKeypairFiles(t)
	defer cleanup()

	const (
		secretNS    = "myns"
		secretName  = "mysecret"
		secretValue = "supersecret"
	)

	testCases := []struct {
		ns      string
		name    string
		scope   ssv1alpha1.SealingScope
		sealErr string
	}{
		{ns: "", name: "", sealErr: "must provide the --namespace flag with --raw and --scope strict"},
		{ns: secretNS, name: "", sealErr: "must provide the --name flag with --raw and --scope strict"},
		{scope: ssv1alpha1.NamespaceWideScope, name: secretName, sealErr: "must provide the --namespace flag with --raw and --scope namespace-wide"},
	}
	for i, tc := range testCases {
		// try to encrypt an item and check error response
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			_, err := trySealTestItem(certFilename, tc.ns, tc.name, secretValue, tc.scope)
			if got, want := fmt.Sprint(err), tc.sealErr; !strings.HasPrefix(got, want) {
				t.Fatalf("got: %v, want: %v", err, want)
			}
		})
	}
}
