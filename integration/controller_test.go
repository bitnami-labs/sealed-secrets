// +build integration

package integration

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"fmt"

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

func getData(s *v1.Secret) map[string][]byte {
	return s.Data
}

func fetchKeys(c corev1.SecretsGetter) (*rsa.PrivateKey, []*x509.Certificate, error) {
	s, err := c.Secrets("sealed-secrets").Get("sealed-secrets-key", metav1.GetOptions{})
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

var _ = Describe("create", func() {
	var c corev1.CoreV1Interface
	var ssc ssclient.Interface
	var ns string
	const secretName = "testsecret"
	var ss *ssv1alpha1.SealedSecret
	var s *v1.Secret
	var pubKey *rsa.PublicKey

	BeforeEach(func() {
		conf := clusterConfigOrDie()
		c = corev1.NewForConfigOrDie(conf)
		ssc = ssclient.NewForConfigOrDie(conf)
		ns = createNsOrDie(c, "create")

		s = &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      secretName,
			},
			Data: map[string][]byte{
				"foo": []byte("bar"),
			},
		}

		_, certs, err := fetchKeys(c)
		Expect(err).NotTo(HaveOccurred())
		pubKey = certs[0].PublicKey.(*rsa.PublicKey)

		fmt.Fprintf(GinkgoWriter, "Sealing Secret %#v", s)
		ss, err = ssv1alpha1.NewSealedSecret(scheme.Codecs, pubKey, s)
		Expect(err).NotTo(HaveOccurred())

	})
	AfterEach(func() {
		deleteNsOrDie(c, ns)
	})

	JustBeforeEach(func() {
		var err error
		fmt.Fprintf(GinkgoWriter, "Creating SealedSecret: %#v", ss)
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

				fmt.Fprintf(GinkgoWriter, "Updating to SealedSecret: %#v", ss)
				ss, err = ssc.BitnamiV1alpha1().SealedSecrets(ss.Namespace).Update(ss)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should produce updated Secret", func() {
				expected := map[string][]byte{
					"foo": []byte("baz"),
				}
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(secretName, metav1.GetOptions{})
				}).Should(WithTransform(getData, Equal(expected)))
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
				}).Should(WithTransform(getData, Equal(expected)))
			})
		})
	})

	Describe("Same name, wrong key", func() {
		BeforeEach(func() {
			// NB: weak keysize - this is just a test case
			wrongkey, err := rsa.GenerateKey(rand.Reader, 1024)
			Expect(err).NotTo(HaveOccurred())

			fmt.Fprintf(GinkgoWriter, "Resealing with wrong key")
			ss, err = ssv1alpha1.NewSealedSecret(scheme.Codecs, &wrongkey.PublicKey, s)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should *not* produce a Secret", func() {
			Consistently(func() error {
				_, err := c.Secrets(ns).Get(secretName, metav1.GetOptions{})
				return err
			}).Should(WithTransform(errors.IsNotFound, Equal(true)))
		})

		// TODO: Check for a suitable error event on the
		// SealedSecret (once implemented)
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

			// TODO: Check for a suitable error event on
			// the SealedSecret (once implemented)
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

			// TODO: Check for a suitable error event on
			// the SealedSecret (once implemented)
		})

		Context("With cluster-wide annotation", func() {
			const secretName2 = "not-testsecret"
			BeforeEach(func() {
				var err error

				s.Annotations = map[string]string{
					ssv1alpha1.SealedSecretClusterWideAnnotation: "true",
				}

				fmt.Fprintf(GinkgoWriter, "Re-sealing secret %#v", s)
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
				}).Should(WithTransform(getData, Equal(expected)))
			})
		})
	})
})
