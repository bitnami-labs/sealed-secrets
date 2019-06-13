// +build integration

package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"io"

	"github.com/onsi/gomega/types"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	certUtil "k8s.io/client-go/util/cert"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealed-secrets/v1alpha1"
	ssclient "github.com/bitnami-labs/sealed-secrets/pkg/client/clientset/versioned"
	"github.com/bitnami-labs/sealed-secrets/pkg/crypto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	Timeout = "5s"
	PollingInterval = "100ms"
)

func getData(s *v1.Secret) map[string][]byte {
	return s.Data
}

func fetchKeys(c corev1.SecretsGetter) (*rsa.PrivateKey, []*x509.Certificate, error) {
	s, err := c.Secrets("kube-system").Get("sealed-secrets-key", metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}

	privKey, err := certUtil.ParsePrivateKeyPEM(s.Data[v1.TLSPrivateKeyKey])
	if err != nil {
		return nil, nil, err
	}

	certs, err := certUtil.ParseCertsPEM(s.Data[v1.TLSCertKey])
	if err != nil {
		return nil, nil, err
	}

	if len(certs) == 0 {
		return nil, nil, fmt.Errorf("Failed to read any certificates")
	}

	return privKey.(*rsa.PrivateKey), certs, nil
}

func containEventWithReason(matcher types.GomegaMatcher) types.GomegaMatcher {
	return WithTransform(
		func(l *v1.EventList) []v1.Event { return l.Items },
		ContainElement(WithTransform(
			func(e v1.Event) string { return e.Reason },
			matcher,
		)),
	)
}

var _ = Describe("create", func() {
	var c corev1.CoreV1Interface
	var ssc ssclient.Interface
	var ns string
	const secretName = "testsecret"
	var ss *ssv1alpha1.SealedSecret
	var s *v1.Secret
	var pubKey *rsa.PublicKey
	var cancelLog context.CancelFunc

	BeforeEach(func() {
		var ctx context.Context
		ctx, cancelLog = context.WithCancel(context.Background())

		conf := clusterConfigOrDie()
		c = corev1.NewForConfigOrDie(conf)
		ssc = ssclient.NewForConfigOrDie(conf)
		ns = createNsOrDie(c, "create")

		go streamLog(ctx, c, ns, "sealed-secrets-controller", "sealed-secrets-controller", GinkgoWriter, fmt.Sprintf("[%s] ", ns))

		s = &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      secretName,
				Labels: map[string]string{
					"mylabel": "myvalue",
				},
			},
			Data: map[string][]byte{
				"foo": []byte("bar"),
			},
		}

		_, certs, err := fetchKeys(c)
		Expect(err).NotTo(HaveOccurred())
		pubKey = certs[0].PublicKey.(*rsa.PublicKey)

		fmt.Fprintf(GinkgoWriter, "Sealing Secret %#v\n", s)
		ss, err = ssv1alpha1.NewSealedSecret(scheme.Codecs, pubKey, s)
		Expect(err).NotTo(HaveOccurred())
	})
	AfterEach(func() {
		deleteNsOrDie(c, ns)
		cancelLog()
	})

	JustBeforeEach(func() {
		var err error
		fmt.Fprintf(GinkgoWriter, "Creating SealedSecret: %#v\n", ss)
		ss, err = ssc.BitnamiV1alpha1().SealedSecrets(ss.Namespace).Create(ss)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Simple change", func() {
		Context("With no existing object (create)", func() {
			It("should produce expected Secret", func() {
				expected := map[string][]byte{
					"foo": []byte("bar"),
				}
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(secretName, metav1.GetOptions{})
				}).Should(WithTransform(getData, Equal(expected)))
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(secretName, metav1.GetOptions{})
				}).Should(WithTransform(metav1.Object.GetLabels,
					HaveKeyWithValue("mylabel", "myvalue")))

				Eventually(func() (*v1.EventList, error) {
					return c.Events(ns).Search(scheme.Scheme, ss)
				}, Timeout, PollingInterval).Should(
					containEventWithReason(Equal("Unsealed")),
				)
			})
		})

		Context("With existing object (update)", func() {
			JustBeforeEach(func() {
				var err error
				resVer := ss.ResourceVersion

				// update
				s.Data["foo"] = []byte("baz")
				ss, err = ssv1alpha1.NewSealedSecret(scheme.Codecs, pubKey, s)
				ss.ResourceVersion = resVer

				fmt.Fprintf(GinkgoWriter, "Updating to SealedSecret: %#v\n", ss)
				ss, err = ssc.BitnamiV1alpha1().SealedSecrets(ss.Namespace).Update(ss)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should produce updated Secret", func() {
				expected := map[string][]byte{
					"foo": []byte("baz"),
				}
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getData, Equal(expected)))
			})
		})

		Context("With renamed encrypted keys", func() {
			BeforeEach(func() {
				ss.Spec.EncryptedData = map[string][]byte{
					"xyzzy": ss.Spec.EncryptedData["foo"],
				}
			})
			It("should produce expected Secret", func() {
				expected := map[string][]byte{
					// renamed key
					"xyzzy": []byte("bar"),
				}
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(secretName, metav1.GetOptions{})
				}).Should(WithTransform(getData, Equal(expected)))
			})
		})

		Context("With appended encrypted keys", func() {
			BeforeEach(func() {
				label := fmt.Sprintf("%s/%s", s.Namespace, s.Name)
				ciphertext, err := crypto.HybridEncrypt(rand.Reader, pubKey, []byte("new!"), []byte(label))
				Expect(err).NotTo(HaveOccurred())

				ss.Spec.EncryptedData["foo2"] = ciphertext
			})
			It("should produce expected Secret", func() {
				expected := map[string][]byte{
					"foo":  []byte("bar"),
					"foo2": []byte("new!"),
				}
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getData, Equal(expected)))
			})
		})
	})

	Describe("Same name, wrong key", func() {
		BeforeEach(func() {
			// NB: weak keysize - this is just a test case
			wrongkey, err := rsa.GenerateKey(rand.Reader, 1024)
			Expect(err).NotTo(HaveOccurred())

			fmt.Fprintf(GinkgoWriter, "Resealing with wrong key\n")
			ss, err = ssv1alpha1.NewSealedSecret(scheme.Codecs, &wrongkey.PublicKey, s)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should *not* produce a Secret", func() {
			Consistently(func() error {
				_, err := c.Secrets(ns).Get(secretName, metav1.GetOptions{})
				return err
			}).Should(WithTransform(errors.IsNotFound, Equal(true)))
		})

		It("should produce an error Event", func() {
			// Check for a suitable error event on the
			// SealedSecret
			Eventually(func() (*v1.EventList, error) {
				return c.Events(ns).Search(scheme.Scheme, ss)
			}, Timeout, PollingInterval).Should(
				containEventWithReason(Equal("ErrUnsealFailed")),
			)
		})
	})

	Describe("Different name/namespace", func() {
		Context("With wrong name", func() {
			const secretName2 = "not-testsecret"
			BeforeEach(func() {
				ss.Name = secretName2
			})
			It("should *not* produce a Secret", func() {
				Consistently(func() error {
					_, err := c.Secrets(ns).Get(secretName2, metav1.GetOptions{})
					return err
				}).Should(WithTransform(errors.IsNotFound, Equal(true)))
			})

			It("should produce an error Event", func() {
				// Check for a suitable error event on the
				// SealedSecret
				Eventually(func() (*v1.EventList, error) {
					return c.Events(ns).Search(scheme.Scheme, ss)
				}, Timeout, PollingInterval).Should(
					containEventWithReason(Equal("ErrUnsealFailed")),
				)
			})
		})

		Context("With wrong namespace", func() {
			var ns2 string
			BeforeEach(func() {
				ns2 = createNsOrDie(c, "create")
				ss.Namespace = ns2
			})
			AfterEach(func() {
				deleteNsOrDie(c, ns2)
			})

			It("should *not* produce a Secret", func() {
				Consistently(func() error {
					_, err := c.Secrets(ns2).Get(secretName, metav1.GetOptions{})
					return err
				}).Should(WithTransform(errors.IsNotFound, Equal(true)))
			})

			It("should produce an error Event", func() {
				// Check for a suitable error event on the
				// SealedSecret
				Eventually(func() (*v1.EventList, error) {
					return c.Events(ns2).Search(scheme.Scheme, ss)
				}, Timeout, PollingInterval).Should(
					containEventWithReason(Equal("ErrUnsealFailed")),
				)
			})
		})

		Context("With wrong name via template.metadata", func() {
			const secretName2 = "not-testsecret"
			BeforeEach(func() {
				ss.Spec.Template.Name = secretName2
			})
			It("should *not* produce a Secret", func() {
				Consistently(func() error {
					_, err := c.Secrets(ns).Get(secretName2, metav1.GetOptions{})
					return err
				}).Should(WithTransform(errors.IsNotFound, Equal(true)))
			})

			It("should produce an error Event", func() {
				// Check for a suitable error event on the
				// SealedSecret
				Eventually(func() (*v1.EventList, error) {
					return c.Events(ns).Search(scheme.Scheme, ss)
				}, Timeout, PollingInterval).Should(
					containEventWithReason(Equal("ErrUnsealFailed")),
				)
			})
		})

		Context("With wrong namespace via template.metadata", func() {
			var ns2 string
			BeforeEach(func() {
				ns2 = createNsOrDie(c, "create")
				ss.Spec.Template.Namespace = ns2
			})
			AfterEach(func() {
				deleteNsOrDie(c, ns2)
			})
			It("should *not* produce a Secret", func() {
				Consistently(func() error {
					_, err := c.Secrets(ns2).Get(secretName, metav1.GetOptions{})
					return err
				}).Should(WithTransform(errors.IsNotFound, Equal(true)))
			})

			It("should produce an error Event", func() {
				// Check for a suitable error event on the
				// SealedSecret
				Eventually(func() (*v1.EventList, error) {
					return c.Events(ns).Search(scheme.Scheme, ss)
				}, Timeout, PollingInterval).Should(
					containEventWithReason(Equal("ErrUnsealFailed")),
				)
			})
		})

		Context("With cluster-wide annotation", func() {
			const secretName2 = "not-testsecret"
			BeforeEach(func() {
				var err error

				s.Annotations = map[string]string{
					ssv1alpha1.SealedSecretClusterWideAnnotation: "true",
				}

				fmt.Fprintf(GinkgoWriter, "Re-sealing secret %#v\n", s)
				ss, err = ssv1alpha1.NewSealedSecret(scheme.Codecs, pubKey, s)
				Expect(err).NotTo(HaveOccurred())
			})
			BeforeEach(func() {
				ss.Name = secretName2
			})
			It("should produce expected Secret", func() {
				expected := map[string][]byte{
					"foo": []byte("bar"),
				}
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(secretName2, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getData, Equal(expected)))
			})
		})
	})
})

var _ = Describe("controller --version", func() {
	var input io.Reader
	var output *bytes.Buffer
	var args []string

	BeforeEach(func() {
		args = []string{"--version"}
		output = &bytes.Buffer{}
	})

	JustBeforeEach(func() {
		err := runController(args, input, output)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should produce the version", func() {
		Expect(output.String()).Should(MatchRegexp("^controller version: (v[0-9]+\\.[0-9]+\\.[0-9]+|[0-9a-f]{40})(\\+dirty)?"))
	})
})
