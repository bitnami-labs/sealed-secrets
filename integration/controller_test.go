// +build integration

package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/onsi/gomega/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	certUtil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealed-secrets/v1alpha1"
	ssclient "github.com/bitnami-labs/sealed-secrets/pkg/client/clientset/versioned"
	"github.com/bitnami-labs/sealed-secrets/pkg/crypto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var keySelector = fields.OneTermEqualSelector("sealedsecrets.bitnami.com/sealed-secrets-key", "active").String()

const (
	Timeout         = 15 * time.Second
	PollingInterval = "100ms"
)

func getData(s *v1.Secret) map[string][]byte {
	return s.Data
}

func getSecretType(s *v1.Secret) v1.SecretType {
	return s.Type
}

func fetchKeys(c corev1.SecretsGetter) (map[string]*rsa.PrivateKey, []*x509.Certificate, error) {
	list, err := c.Secrets("kube-system").List(metav1.ListOptions{
		LabelSelector: keySelector,
	})
	if err != nil {
		return nil, nil, err
	}

	if len(list.Items) == 0 {
		return nil, nil, fmt.Errorf("found 0 keys")
	}

	sort.Sort(ssv1alpha1.ByCreationTimestamp(list.Items))
	latestKey := &list.Items[len(list.Items)-1]

	privKey, err := keyutil.ParsePrivateKeyPEM(latestKey.Data[v1.TLSPrivateKeyKey])
	if err != nil {
		return nil, nil, err
	}

	certs, err := certUtil.ParseCertsPEM(latestKey.Data[v1.TLSCertKey])
	if err != nil {
		return nil, nil, err
	}

	if len(certs) == 0 {
		return nil, nil, fmt.Errorf("Failed to read any certificates")
	}

	rsaPrivKey := privKey.(*rsa.PrivateKey)
	fp, err := crypto.PublicKeyFingerprint(&rsaPrivKey.PublicKey)
	privKeys := map[string]*rsa.PrivateKey{fp: rsaPrivKey}
	return privKeys, certs, nil
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
	const secretName1 = "testsecret"
	const secretName2 = "testsecret2"
	var ss *ssv1alpha1.SealedSecret
	var ssVault *ssv1alpha1.SealedSecret
	var s *v1.Secret
	var s2 *v1.Secret
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

		// Cert
		s = &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      secretName1,
				Labels: map[string]string{
					"mylabel": "myvalue",
				},
			},
			Data: map[string][]byte{
				"foo": []byte("bar"),
			},
		}
		// Vault
		s2 = &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      secretName2,
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
		ss, err = ssv1alpha1.NewSealedSecret(scheme.Codecs, "cert", pubKey, s)
		ssVault, err = ssv1alpha1.NewSealedSecret(scheme.Codecs, "vault", pubKey, s2)
		Expect(err).NotTo(HaveOccurred())
	})
	AfterEach(func() {
		deleteNsOrDie(c, ns)
		cancelLog()
	})

	JustBeforeEach(func() {
		var err error
		fmt.Fprintf(GinkgoWriter, "Creating SealedSecrets: %#v\n", ss)
		ss, err = ssc.BitnamiV1alpha1().SealedSecrets(ss.Namespace).Create(ss)
		Expect(err).NotTo(HaveOccurred())
		ssVault, err = ssc.BitnamiV1alpha1().SealedSecrets(ssVault.Namespace).Create(ssVault)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Simple change", func() {
		Context("With no existing object (create)", func() {
			It("should produce expected Secret", func() {
				expected := map[string][]byte{
					"foo": []byte("bar"),
				}
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(secretName1, metav1.GetOptions{})
					return c.Secrets(ns).Get(secretName2, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getData, Equal(expected)))
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(secretName1, metav1.GetOptions{})
					return c.Secrets(ns).Get(secretName2, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(metav1.Object.GetLabels,
					HaveKeyWithValue("mylabel", "myvalue")))

				Eventually(func() (*v1.EventList, error) {
					return c.Events(ns).Search(scheme.Scheme, ss)
				}, Timeout, PollingInterval).Should(
					containEventWithReason(Equal("Unsealed")),
				)
				Eventually(func() (*v1.EventList, error) {
					return c.Events(ns).Search(scheme.Scheme, ssVault)
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
				ss, err = ssv1alpha1.NewSealedSecret(scheme.Codecs, "cert", pubKey, s)
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
					return c.Secrets(ns).Get(secretName1, metav1.GetOptions{})
				}, 15*time.Second).Should(WithTransform(getData, Equal(expected)))
			})
		})

		Context("With renamed encrypted keys", func() {
			BeforeEach(func() {
				ss.Spec.EncryptedData = map[string]string{
					"xyzzy": ss.Spec.EncryptedData["foo"],
				}
			})
			It("should produce expected Secret", func() {
				expected := map[string][]byte{
					// renamed key
					"xyzzy": []byte("bar"),
				}
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(secretName1, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getData, Equal(expected)))
			})
		})

		Context("With appended encrypted keys", func() {
			BeforeEach(func() {
				label := fmt.Sprintf("%s/%s", s.Namespace, s.Name)
				ciphertext, err := crypto.HybridEncrypt(rand.Reader, pubKey, []byte("new!"), []byte(label))
				Expect(err).NotTo(HaveOccurred())

				ss.Spec.EncryptedData["foo2"] = base64.StdEncoding.EncodeToString(ciphertext)
			})
			It("should produce expected Secret", func() {
				expected := map[string][]byte{
					"foo":  []byte("bar"),
					"foo2": []byte("new!"),
				}
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(secretName1, metav1.GetOptions{})
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
			ss, err = ssv1alpha1.NewSealedSecret(scheme.Codecs, "cert", &wrongkey.PublicKey, s)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should *not* produce a Secret", func() {
			Consistently(func() error {
				_, err := c.Secrets(ns).Get(secretName1, metav1.GetOptions{})
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

	Describe("Custom Secret Type", func() {
		BeforeEach(func() {
			label := fmt.Sprintf("%s/%s", s.Namespace, s.Name)
			ciphertext, err := crypto.HybridEncrypt(rand.Reader, pubKey, []byte("{\"auths\": {\"https://index.docker.io/v1/\": {\"auth\": \"c3R...zE2\"}}}"), []byte(label))
			Expect(err).NotTo(HaveOccurred())

			ss.Spec.EncryptedData[".dockerconfigjson"] = base64.StdEncoding.EncodeToString(ciphertext)
			delete(ss.Spec.EncryptedData, "foo")
			ss.Spec.Template.Type = "kubernetes.io/dockerconfigjson"
		})
		It("should produce expected Secret", func() {
			expected := map[string][]byte{
				".dockerconfigjson": []byte("{\"auths\": {\"https://index.docker.io/v1/\": {\"auth\": \"c3R...zE2\"}}}"),
			}
			var expectedType v1.SecretType = "kubernetes.io/dockerconfigjson"

			Eventually(func() (*v1.Secret, error) {
				return c.Secrets(ns).Get(secretName1, metav1.GetOptions{})
			}, Timeout, PollingInterval).Should(WithTransform(getData, Equal(expected)))
			Eventually(func() (*v1.Secret, error) {
				return c.Secrets(ns).Get(secretName1, metav1.GetOptions{})
			}, Timeout, PollingInterval).Should(WithTransform(getSecretType, Equal(expectedType)))
			Eventually(func() (*v1.EventList, error) {
				return c.Events(ns).Search(scheme.Scheme, ss)
			}, Timeout, PollingInterval).Should(
				containEventWithReason(Equal("Unsealed")),
			)
		})
	})

	Describe("Different name/namespace", func() {
		Context("With wrong name", func() {
			const notSecret = "not-testsecret"
			BeforeEach(func() {
				ss.Name = notSecret
			})
			It("should *not* produce a Secret", func() {
				Consistently(func() error {
					_, err := c.Secrets(ns).Get(notSecret, metav1.GetOptions{})
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
					_, err := c.Secrets(ns2).Get(secretName1, metav1.GetOptions{})
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

		Context("With wrong name and cluster-wide annotation", func() {
			const notSecret = "not-testsecret"
			BeforeEach(func() {
				var err error

				s.Annotations = map[string]string{
					ssv1alpha1.SealedSecretClusterWideAnnotation: "true",
				}

				fmt.Fprintf(GinkgoWriter, "Re-sealing secret %#v\n", s)
				ss, err = ssv1alpha1.NewSealedSecret(scheme.Codecs, "cert", pubKey, s)
				Expect(err).NotTo(HaveOccurred())
			})
			BeforeEach(func() {
				ss.Name = notSecret
			})
			It("should produce expected Secret", func() {
				expected := map[string][]byte{
					"foo": []byte("bar"),
				}
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(notSecret, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getData, Equal(expected)))
			})
		})

		Context("With wrong namespace and cluster-wide annotation", func() {
			var ns2 string
			BeforeEach(func() {
				ns2 = createNsOrDie(c, "create")
			})
			BeforeEach(func() {
				var err error

				s.Annotations = map[string]string{
					ssv1alpha1.SealedSecretClusterWideAnnotation: "true",
				}

				fmt.Fprintf(GinkgoWriter, "Re-sealing secret %#v\n", s)
				ss, err = ssv1alpha1.NewSealedSecret(scheme.Codecs, "cert", pubKey, s)
				ss.Namespace = ns2
				Expect(err).NotTo(HaveOccurred())
			})
			AfterEach(func() {
				deleteNsOrDie(c, ns2)
			})
			It("should produce expected Secret", func() {
				expected := map[string][]byte{
					"foo": []byte("bar"),
				}
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns2).Get(secretName1, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getData, Equal(expected)))
			})
		})

		Context("With wrong name and namespace-wide annotation", func() {
			const notSecret = "not-testsecret"
			BeforeEach(func() {
				var err error

				s.Annotations = map[string]string{
					ssv1alpha1.SealedSecretNamespaceWideAnnotation: "true",
				}

				fmt.Fprintf(GinkgoWriter, "Re-sealing secret %#v\n", s)
				ss, err = ssv1alpha1.NewSealedSecret(scheme.Codecs, "cert", pubKey, s)
				Expect(err).NotTo(HaveOccurred())
			})
			BeforeEach(func() {
				ss.Name = notSecret
			})
			It("should produce expected Secret", func() {
				expected := map[string][]byte{
					"foo": []byte("bar"),
				}
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(notSecret, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getData, Equal(expected)))
			})
		})

		Context("With wrong namespace and namespace-wide annotation", func() {
			var ns2 string
			BeforeEach(func() {
				ns2 = createNsOrDie(c, "create")
			})
			BeforeEach(func() {
				var err error

				s.Annotations = map[string]string{
					ssv1alpha1.SealedSecretNamespaceWideAnnotation: "true",
				}

				fmt.Fprintf(GinkgoWriter, "Re-sealing secret %#v\n", s)
				ss, err = ssv1alpha1.NewSealedSecret(scheme.Codecs, "cert", pubKey, s)
				ss.Namespace = ns2
				Expect(err).NotTo(HaveOccurred())
			})
			AfterEach(func() {
				deleteNsOrDie(c, ns2)
			})

			It("should *not* produce a Secret", func() {
				Consistently(func() error {
					_, err := c.Secrets(ns2).Get(secretName1, metav1.GetOptions{})
					return err
				}).Should(WithTransform(errors.IsNotFound, Equal(true)))
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
