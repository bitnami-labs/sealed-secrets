package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	goflag "flag"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	flag "github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	certUtil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealed-secrets/v1alpha1"
	"github.com/bitnami-labs/sealed-secrets/pkg/crypto"
	"github.com/bitnami-labs/sealed-secrets/pkg/kubeseal"
)

var (
	testModulus *big.Int
)

func init() {
	testModulus = new(big.Int)
	_, err := fmt.Sscan("777304254876434297689544225447769213262492599515515837291621795936355252933930193245809942636192119684040605554803489669141565417296821660595336672178414512660751886699171738066307588619202437848899334837760648051656982184646490661921128886671800776058692981991859399404705935722225294811424879738586269551402668122524371718537515440568440102201259925611463161144897905846190044735554045001999198442528435295995584980713050916813579912296878368079243909549993116827192901474611239264189340401059113919551426849847211275352102674049634252149163111599977742365280992561904350781270344655927564475032580504276518647106167707150111291732645399166011800154961975117045723373023335778593638216165426988399138193230056486079421256484837299169853958601000282124667227789126483641999102102039577368681983584245367307077546423870452524154641890843463963116237003367269116435430641427113406369059991147359641266708862913786891945896441771663010146473536372286482453315017377528517965715554550898957321536181165129538808789201530141159181590893764287807749414277289452691723903046140558704697831351834538780165261072894792900501671534138992265545905216973214953125367388406669893889742303072755608685449114438926280862339744991872488262084141163", testModulus)
	if err != nil {
		panic(err)
	}
}

func TestMain(m *testing.M) {
	// ensure that the -test.* flags inserted by go test are properly processed
	// otherwise the pflag.Parse invocation below will interfere with test flags.
	goflag.Parse()

	// otherwise we'd require a working KUBECONFIG file when calling `run`.
	_ = flag.CommandLine.Parse([]string{"-n", "default"})
	os.Exit(m.Run())
}

func TestVersion(t *testing.T) {
	ctx := context.Background()
	var buf strings.Builder
	err := run(ctx, &buf, "", "", "", "", "", "", true, false, false, false, false, false, nil, "", false, nil)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := buf.String(), "kubeseal version: UNKNOWN\n"; got != want {
		t.Errorf("got: %q, want: %q", got, want)
	}
}

func TestMainError(t *testing.T) {
	ctx := context.Background()
	badFileName := filepath.Join("this", "file", "cannot", "possibly", "exist", "can", "it?")
	err := run(ctx, ioutil.Discard, "", "", "", "", "", badFileName, false, false, false, false, false, false, nil, "", false, nil)

	if err == nil || !os.IsNotExist(err) {
		t.Fatalf("expecting not exist error, got: %v", err)
	}
}

/*
Duped in kubeseal/main_test.go
*/
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
		sealErr   string
		unsealErr string
	}{
		// strict scope
		{ns: "", name: "", sealErr: "must provide the --namespace flag with --raw and --scope strict"},
		{ns: secretNS, name: "", sealErr: "must provide the --name flag with --raw and --scope strict"},

		{ns: secretNS, name: secretName},
		{ns: "youGiveRest", name: secretName, unsealErr: "no key could decrypt secret"},
		{ns: secretNS, name: "aBadName", unsealErr: "no key could decrypt secret"},

		// namespace-wide scope
		{scope: ssv1alpha1.NamespaceWideScope, name: secretName, sealErr: "must provide the --namespace flag with --raw and --scope namespace-wide"},

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
			if tc.sealErr != "" {
				if got, want := fmt.Sprint(err), tc.sealErr; !strings.HasPrefix(got, want) {
					t.Fatalf("got: %v, want: %v", err, want)
				}
				return
			}
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

			privKeys, err := kubeseal.ReadPrivKeys([]string{privKeyFilename})
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

func sealTestItem(certFilename, secretNS, secretName, secretValue string, scope ssv1alpha1.SealingScope) (string, error) {
	// we use a global k8s config from which we take the namespace (either default or set by flag).
	// it's a mess, for now let's hook in a test getter and restore the original getter after the test.
	defer func(s func() (string, bool, error)) { namespaceFromClientConfig = s }(namespaceFromClientConfig)
	namespaceFromClientConfig = func() (string, bool, error) { return secretNS, false, nil }

	// sadly, sealingscope is also global
	// TODO(mkm): refactor this mess
	defer func(s ssv1alpha1.SealingScope) { sealingScope = s }(sealingScope)
	sealingScope = scope
	/*
		if got, want := run(ioutil.Discard, "", "", "", certFilename, false, false, false, false, true, nil, "", false, nil), "must provide the --name flag with --raw and --scope strict"; got == nil || got.Error() != want {
			t.Fatalf("want matching: %q, got: %q", want, got.Error())
		}

		if got, want := run(ioutil.Discard, secretName, "", "", certFilename, false, false, false, false, true, nil, "", false, nil), "must provide the --from-file flag with --raw"; got == nil || got.Error() != want {
			t.Fatalf("want matching: %q, got: %q", want, got.Error())
		}
	*/

	dataFile, err := writeTempFile([]byte(secretValue))
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dataFile)

	fromFile := []string{dataFile}

	ctx := context.Background()
	var buf bytes.Buffer
	if err := run(ctx, &buf, "", "", secretName, "", "", certFilename, false, false, false, false, true, false, fromFile, "", false, nil); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func TestWriteToFile(t *testing.T) {
	certFilename, _, cleanup := testingKeypairFiles(t)
	defer cleanup()

	in, err := ioutil.TempFile("", "")
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

	out, err := ioutil.TempFile("", "*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	out.Close()
	defer os.RemoveAll(out.Name())

	ctx := context.Background()
	var buf bytes.Buffer
	if err := run(ctx, &buf, in.Name(), out.Name(), "", "", "", certFilename, false, false, false, false, false, false, nil, "", false, nil); err != nil {
		t.Fatal(err)
	}

	if got, want := buf.Len(), 0; got != want {
		t.Errorf("got: %d, want: %d", got, want)
	}

	b, err := ioutil.ReadFile(out.Name())
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

	in, err := ioutil.TempFile("", "")
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

	out, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}

	// if sealing error happens, the old content of the output file shouldn't be truncated.
	const testOldContent = "previous content"

	fmt.Fprint(out, testOldContent)
	out.Close()
	defer os.RemoveAll(out.Name())

	ctx := context.Background()
	var buf bytes.Buffer
	if err := run(ctx, &buf, in.Name(), out.Name(), "", "", "", certFilename, false, false, false, false, false, false, nil, "", false, nil); err == nil {
		t.Errorf("expecting error")
	}

	if got, want := buf.Len(), 0; got != want {
		t.Errorf("got: %d, want: %d", got, want)
	}

	b, err := ioutil.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(b), testOldContent; got != want {
		t.Errorf("got: %q, want: %q", got, want)
	}
}

// writeTempFile creates a temporary file, writes data into it and closes it.
func writeTempFile(b []byte) (string, error) {
	tmp, err := ioutil.TempFile("", "")
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
