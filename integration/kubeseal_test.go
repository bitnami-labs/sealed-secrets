// +build integration

package integration

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io"
	"io/ioutil"
	"os"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealed-secrets/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	certUtil "k8s.io/client-go/util/cert"
)

var _ = Describe("kubeseal", func() {
	var c corev1.CoreV1Interface
	const secretName = "testSecret"
	var ns string
	var input *v1.Secret
	var ss *ssv1alpha1.SealedSecret
	var args []string
	var privKeys map[string]*rsa.PrivateKey
	var certs []*x509.Certificate
	var config *clientcmdapi.Config
	var kubeconfigFile string

	BeforeEach(func() {
		clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: *kubeconfig},
			&clientcmd.ConfigOverrides{})
		rawconf, err := clientConfig.RawConfig()
		Expect(err).NotTo(HaveOccurred())
		config = rawconf.DeepCopy()
	})

	JustBeforeEach(func() {
		f, err := ioutil.TempFile("", "kubeconfig")
		Expect(err).NotTo(HaveOccurred())

		buf, err := runtime.Encode(clientcmdlatest.Codec, config)
		Expect(err).NotTo(HaveOccurred())

		_, err = f.Write(buf)
		Expect(err).NotTo(HaveOccurred())

		err = f.Close()
		Expect(err).NotTo(HaveOccurred())

		kubeconfigFile = f.Name()
		args = append(args, "--kubeconfig", kubeconfigFile)
	})
	AfterEach(func() {
		os.Remove(kubeconfigFile)
	})

	BeforeEach(func() {
		c = corev1.NewForConfigOrDie(clusterConfigOrDie())

		input = &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      secretName,
			},
			Data: map[string][]byte{
				"foo": []byte("bar"),
			},
		}

		var err error
		privKeys, certs, err = fetchKeys(c)
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		outobj, err := runKubesealWith(args, input)
		Expect(err).NotTo(HaveOccurred())
		ss = outobj.(*ssv1alpha1.SealedSecret)
	})

	Context("Without args", func() {
		const testNs = "testns"
		BeforeEach(func() {
			input.Namespace = testNs
		})

		It("should have the right objectmeta", func() {
			Expect(ss.Kind).To(Equal("SealedSecret"))
			Expect(ss.GetName()).To(Equal(secretName))
			Expect(ss.GetNamespace()).To(Equal(testNs))
		})

		It("should contain the right value", func() {
			s, err := ss.Unseal(scheme.Codecs, "cert", privKeys)
			Expect(err).NotTo(HaveOccurred())
			Expect(s.Data).To(HaveKeyWithValue("foo", []byte("bar")))
		})
	})

	Context("No input namespace", func() {
		const testNs = "nons"

		BeforeEach(func() {
			// set kubeconfig default namespace to testNs
			config.Contexts[config.CurrentContext].Namespace = testNs
		})

		It("should use namespace from kubeconfig", func() {
			Expect(ss.GetNamespace()).To(Equal(testNs))
		})

		It("should qualify the Secret", func() {
			s, err := ss.Unseal(scheme.Codecs, "cert", privKeys)
			Expect(err).NotTo(HaveOccurred())
			Expect(s.GetNamespace()).To(Equal(testNs))
		})
	})

	Context("With --namespace", func() {
		const testNs = "argns"
		BeforeEach(func() {
			args = append(args, "-n", testNs)
		})

		It("should qualify the output SealedSecret", func() {
			Expect(ss.GetNamespace()).To(Equal(testNs))
		})

		It("should qualify the Secret", func() {
			s, err := ss.Unseal(scheme.Codecs, "cert", privKeys)
			Expect(err).NotTo(HaveOccurred())
			Expect(s.GetNamespace()).To(Equal(testNs))
		})
	})

	Context("Offline, with --cert", func() {
		var certfile *os.File

		BeforeEach(func() {
			// Invalidate address of current cluster
			cluster := config.Contexts[config.CurrentContext].Cluster
			config.Clusters[cluster].Server = "http://0.0.0.0:1"
		})

		BeforeEach(func() {
			var err error
			certfile, err = ioutil.TempFile("", "kubeseal-test")
			Expect(err).NotTo(HaveOccurred())

			for _, cert := range certs {
				certfile.Write(pem.EncodeToMemory(&pem.Block{Type: certUtil.CertificateBlockType, Bytes: cert.Raw}))
			}
			certfile.Close()

			args = append(args, "--cert", certfile.Name())
		})
		AfterEach(func() {
			if certfile != nil {
				os.Remove(certfile.Name())
				certfile = nil
			}
		})

		It("should output the right value", func() {
			s, err := ss.Unseal(scheme.Codecs, "cert", privKeys)
			Expect(err).NotTo(HaveOccurred())
			Expect(s.Data).To(HaveKeyWithValue("foo", []byte("bar")))
		})
	})
})

var _ = Describe("kubeseal --fetch-cert", func() {
	var c corev1.CoreV1Interface
	var input io.Reader
	var output *bytes.Buffer
	var args []string

	BeforeEach(func() {
		c = corev1.NewForConfigOrDie(clusterConfigOrDie())

		args = append(args, "--fetch-cert")
		output = &bytes.Buffer{}
	})
	JustBeforeEach(func() {
		err := runKubeseal(args, input, output)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should produce the certificate", func() {
		_, certs, err := fetchKeys(c)
		Expect(err).NotTo(HaveOccurred())

		Expect(certUtil.ParseCertsPEM(output.Bytes())).
			Should(Equal(certs))
	})
})

var _ = Describe("kubeseal --version", func() {
	var input io.Reader
	var output *bytes.Buffer
	var args []string

	BeforeEach(func() {
		args = []string{"--version"}
		output = &bytes.Buffer{}
	})

	JustBeforeEach(func() {
		err := runKubeseal(args, input, output)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should produce the version", func() {
		Expect(output.String()).Should(MatchRegexp("^kubeseal version: (v[0-9]+\\.[0-9]+\\.[0-9]+|[0-9a-f]{40})(\\+dirty)?"))
	})
})

var _ = Describe("kubeseal --verify", func() {
	const secretName = "testSecret"
	const testNs = "testverifyns"
	var input io.Reader
	var output *bytes.Buffer
	var ss *ssv1alpha1.SealedSecret
	var args []string
	var err error

	BeforeEach(func() {
		args = append(args, "--validate")
		output = &bytes.Buffer{}
	})

	BeforeEach(func() {
		input := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNs,
				Name:      secretName,
			},
			Data: map[string][]byte{
				"foo": []byte("bar"),
			},
		}
		outobj, err := runKubesealWith([]string{}, input)
		Expect(err).NotTo(HaveOccurred())
		ss = outobj.(*ssv1alpha1.SealedSecret)
	})

	JustBeforeEach(func() {
		enc := scheme.Codecs.LegacyCodec(ssv1alpha1.SchemeGroupVersion)
		indata, err := runtime.Encode(enc, ss)
		Expect(err).NotTo(HaveOccurred())
		input = bytes.NewReader(indata)
	})

	JustBeforeEach(func() {
		err = runKubeseal(args, input, output)
	})

	Context("valid sealed secret", func() {
		It("should see the sealed secret as valid", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("invalid sealed secret", func() {
		BeforeEach(func() {
			ss.Name = "a-completely-different-name"
		})

		It("should see the sealed secret as invalid", func() {
			Expect(err).To(HaveOccurred())
		})
	})

})

var _ = Describe("kubeseal --cert", func() {
	var input io.Reader
	var output *bytes.Buffer
	var args []string

	BeforeEach(func() {
		args = []string{"--cert", "/?this/file/cannot/possibly/exist/right?"}
		output = &bytes.Buffer{}
	})

	JustBeforeEach(func() {
		err := runKubeseal(args, input, ioutil.Discard, runAppWithStderr(output))
		Expect(err).To(HaveOccurred())
	})

	It("should return an error", func() {
		Expect(output.String()).Should(MatchRegexp("^error:.*no such file or directory"))
	})
})
