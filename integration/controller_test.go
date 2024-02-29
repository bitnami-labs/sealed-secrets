//go:build integration
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

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealedsecrets/v1alpha1"
	ssclient "github.com/bitnami-labs/sealed-secrets/pkg/client/clientset/versioned"
	"github.com/bitnami-labs/sealed-secrets/pkg/crypto"

	. "github.com/onsi/ginkgo/v2"
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

func getAnnotations(s *v1.Secret) map[string]string {
	return s.ObjectMeta.Annotations
}

func getLabels(s *v1.Secret) map[string]string {
	return s.ObjectMeta.Labels
}

func getStatus(ss *ssv1alpha1.SealedSecret) *ssv1alpha1.SealedSecretStatus {
	return ss.Status
}

func getObservedGeneration(ss *ssv1alpha1.SealedSecret) int64 {
	return ss.Status.ObservedGeneration
}

// get the first owner name assuming there is only one owner which is the sealed-secret object
func getFirstOwnerName(s *v1.Secret) string {
	return s.OwnerReferences[0].Name
}

func getNumberOfOwners(s *v1.Secret) int {
	return len(s.OwnerReferences)
}

func getSecretType(s *v1.Secret) v1.SecretType {
	return s.Type
}

func getSecretImmutable(s *v1.Secret) bool {
	return *s.Immutable
}

func compareLastTimes(ss *ssv1alpha1.SealedSecret) bool {
	for i := range ss.Status.Conditions {
		if ss.Status.Conditions[i].Type == ssv1alpha1.SealedSecretSynced {
			return ss.Status.Conditions[i].LastTransitionTime == ss.Status.Conditions[i].LastUpdateTime
		}
	}
	return false
}

func fetchKeys(ctx context.Context, c corev1.SecretsGetter) (map[string]*rsa.PrivateKey, []*x509.Certificate, error) {
	list, err := c.Secrets(*controllerNs).List(ctx, metav1.ListOptions{
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
		return nil, nil, fmt.Errorf("failed to read any certificates")
	}

	rsaPrivKey := privKey.(*rsa.PrivateKey)
	fp, err := crypto.PublicKeyFingerprint(&rsaPrivKey.PublicKey)
	if err != nil {
		return nil, nil, err
	}
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

func containEventWithMessage(matcher types.GomegaMatcher) types.GomegaMatcher {
	return WithTransform(
		func(l *v1.EventList) []v1.Event { return l.Items },
		ContainElement(WithTransform(
			func(e v1.Event) string { return e.Message },
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
	var (
		ctx       context.Context
		cancelLog context.CancelFunc
	)

	BeforeEach(func() {
		ctx, cancelLog = context.WithCancel(context.Background())

		conf := clusterConfigOrDie()
		c = corev1.NewForConfigOrDie(conf)
		ssc = ssclient.NewForConfigOrDie(conf)
		ns = createNsOrDie(ctx, c, "create")

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

		_, certs, err := fetchKeys(ctx, c)
		Expect(err).NotTo(HaveOccurred())
		pubKey = certs[0].PublicKey.(*rsa.PublicKey)

		fmt.Fprintf(GinkgoWriter, "Sealing Secret %#v\n", s)
		ss, err = ssv1alpha1.NewSealedSecret(scheme.Codecs, pubKey, s)
		Expect(err).NotTo(HaveOccurred())
	})
	AfterEach(func() {
		deleteNsOrDie(ctx, c, ns)
		cancelLog()
	})

	JustBeforeEach(func() {
		var err error
		fmt.Fprintf(GinkgoWriter, "Creating SealedSecret: %#v\n", ss)
		ss, err = ssc.BitnamiV1alpha1().SealedSecrets(ss.Namespace).Create(context.Background(), ss, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Simple change", func() {
		Context("With no existing object (create)", func() {
			It("should produce expected Secret", func() {
				expected := map[string][]byte{
					"foo": []byte("bar"),
				}
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getData, Equal(expected)))
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(metav1.Object.GetLabels,
					HaveKeyWithValue("mylabel", "myvalue")))
				Eventually(func() (*ssv1alpha1.SealedSecret, error) {
					return ssc.BitnamiV1alpha1().SealedSecrets(ns).Get(context.Background(), secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).ShouldNot(WithTransform(getStatus, BeNil()))
				Eventually(func() (*ssv1alpha1.SealedSecret, error) {
					return ssc.BitnamiV1alpha1().SealedSecrets(ns).Get(context.Background(), secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(compareLastTimes, Equal(true)))
				Eventually(func() (*v1.EventList, error) {
					return c.Events(ns).Search(scheme.Scheme, ss)
				}, Timeout, PollingInterval).Should(
					containEventWithReason(Equal("Unsealed")))
			})
		})

		Context("With existing object (update)", func() {
			JustBeforeEach(func() {
				var err error

				Eventually(func() (*ssv1alpha1.SealedSecret, error) {
					return ssc.BitnamiV1alpha1().SealedSecrets(ss.Namespace).Get(context.Background(), secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).ShouldNot(WithTransform(getStatus, BeNil()))

				ss, err = ssc.BitnamiV1alpha1().SealedSecrets(ss.Namespace).Get(context.Background(), secretName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())

				resVer := ss.ResourceVersion

				// update
				s.Data["foo"] = []byte("baz")
				ss, err = ssv1alpha1.NewSealedSecret(scheme.Codecs, pubKey, s)
				Expect(err).NotTo(HaveOccurred())
				ss.ResourceVersion = resVer

				time.Sleep(1 * time.Second)
				fmt.Fprintf(GinkgoWriter, "Updating to SealedSecret: %#v\n", ss)
				ss, err = ssc.BitnamiV1alpha1().SealedSecrets(ss.Namespace).Update(context.Background(), ss, metav1.UpdateOptions{})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should produce updated Secret", func() {
				expected := map[string][]byte{
					"foo": []byte("baz"),
				}
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getData, Equal(expected)))
				Eventually(func() (*ssv1alpha1.SealedSecret, error) {
					return ssc.BitnamiV1alpha1().SealedSecrets(ns).Get(context.Background(), secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).ShouldNot(WithTransform(getStatus, BeNil()))
				Eventually(func() (*ssv1alpha1.SealedSecret, error) {
					return ssc.BitnamiV1alpha1().SealedSecrets(ns).Get(context.Background(), secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getObservedGeneration, Equal(int64(2))))
				Eventually(func() (*ssv1alpha1.SealedSecret, error) {
					return ssc.BitnamiV1alpha1().SealedSecrets(ns).Get(context.Background(), secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(compareLastTimes, Equal(false)))
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
					return c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getData, Equal(expected)))
				Eventually(func() (*ssv1alpha1.SealedSecret, error) {
					return ssc.BitnamiV1alpha1().SealedSecrets(ns).Get(context.Background(), secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).ShouldNot(WithTransform(getStatus, BeNil()))
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
					return c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getData, Equal(expected)))
				Eventually(func() (*ssv1alpha1.SealedSecret, error) {
					return ssc.BitnamiV1alpha1().SealedSecrets(ns).Get(context.Background(), secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).ShouldNot(WithTransform(getStatus, BeNil()))
			})
		})
	})

	Describe("Secret already exists", func() {
		Context("With managed annotation", func() {
			BeforeEach(func() {
				s.Data = map[string][]byte{
					"foo":  []byte("bar1"),
					"foo2": []byte("bar2"),
				}
				s.Annotations = map[string]string{
					ssv1alpha1.SealedSecretManagedAnnotation: "true",
				}
				s.Labels["anotherlabel"] = "anothervalue"
				c.Secrets(ns).Create(ctx, s, metav1.CreateOptions{})
			})
			It("should take ownership of the existing Secret overwriting the whole Secret", func() {
				expectedData := map[string][]byte{
					"foo": []byte("bar"),
				}
				var expectedAnnotations map[string]string
				expectedLabels := map[string]string{
					"mylabel": "myvalue",
				}
				Eventually(func() (*v1.EventList, error) {
					return c.Events(ns).Search(scheme.Scheme, ss)
				}, Timeout, PollingInterval).Should(
					containEventWithReason(Equal("Unsealed")),
				)
				Eventually(func() (*ssv1alpha1.SealedSecret, error) {
					return ssc.BitnamiV1alpha1().SealedSecrets(ns).Get(context.Background(), secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).ShouldNot(WithTransform(getStatus, BeNil()))
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getFirstOwnerName, Equal(ss.GetName())))
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getData, Equal(expectedData)))
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getAnnotations, Equal(expectedAnnotations)))
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getLabels, Equal(expectedLabels)))
			})
		})

		Context("With managed and patch annotation", func() {
			BeforeEach(func() {
				s.Data = map[string][]byte{
					"foo":  []byte("bar1"),
					"foo2": []byte("bar2"),
				}
				s.Annotations = map[string]string{
					ssv1alpha1.SealedSecretManagedAnnotation: "true",
					ssv1alpha1.SealedSecretPatchAnnotation:   "true",
				}
				s.Labels["anotherlabel"] = "anothervalue"
				c.Secrets(ns).Create(ctx, s, metav1.CreateOptions{})
			})

			It("should take ownership of the existing Secret patching instead of overwriting the whole Secret", func() {
				expectedData := map[string][]byte{
					"foo":  []byte("bar"),
					"foo2": []byte("bar2"),
				}
				expectedAnnotations := map[string]string{
					ssv1alpha1.SealedSecretManagedAnnotation: "true",
					ssv1alpha1.SealedSecretPatchAnnotation:   "true",
				}
				expectedLabels := map[string]string{
					"mylabel":      "myvalue",
					"anotherlabel": "anothervalue",
				}
				Eventually(func() (*v1.EventList, error) {
					return c.Events(ns).Search(scheme.Scheme, ss)
				}, Timeout, PollingInterval).Should(
					containEventWithReason(Equal("Unsealed")),
				)
				Eventually(func() (*ssv1alpha1.SealedSecret, error) {
					return ssc.BitnamiV1alpha1().SealedSecrets(ns).Get(context.Background(), secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).ShouldNot(WithTransform(getStatus, BeNil()))
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getFirstOwnerName, Equal(ss.GetName())))
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getData, Equal(expectedData)))
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getAnnotations, Equal(expectedAnnotations)))
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getLabels, Equal(expectedLabels)))
			})
		})

		Context("With patch annotation", func() {
			BeforeEach(func() {
				s.Data = map[string][]byte{
					"foo":  []byte("bar1"),
					"foo2": []byte("bar2"),
				}
				s.Annotations = map[string]string{
					ssv1alpha1.SealedSecretPatchAnnotation: "true",
				}
				s.Labels["anotherlabel"] = "anothervalue"
				c.Secrets(ns).Create(ctx, s, metav1.CreateOptions{})
			})

			It("should not take ownership of existing Secret while patching the Secret", func() {
				expectedData := map[string][]byte{
					"foo":  []byte("bar"),
					"foo2": []byte("bar2"),
				}
				expectedAnnotations := map[string]string{
					ssv1alpha1.SealedSecretPatchAnnotation: "true",
				}
				expectedLabels := map[string]string{
					"mylabel":      "myvalue",
					"anotherlabel": "anothervalue",
				}
				Eventually(func() (*v1.EventList, error) {
					return c.Events(ns).Search(scheme.Scheme, ss)
				}, Timeout, PollingInterval).Should(
					containEventWithReason(Equal("Unsealed")),
				)
				Eventually(func() (*ssv1alpha1.SealedSecret, error) {
					return ssc.BitnamiV1alpha1().SealedSecrets(ns).Get(context.Background(), secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).ShouldNot(WithTransform(getStatus, BeNil()))
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getNumberOfOwners, Equal(0)))
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getData, Equal(expectedData)))
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getAnnotations, Equal(expectedAnnotations)))
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getLabels, Equal(expectedLabels)))
			})
		})

		Context("With patch annotation and empty secret", func() {
			BeforeEach(func() {
				// Empty secret has no data nor labels field
				s.Data = nil
				s.Labels = nil
				s.Annotations = map[string]string{
					ssv1alpha1.SealedSecretPatchAnnotation: "true",
				}
				c.Secrets(ns).Create(ctx, s, metav1.CreateOptions{})
			})

			It("should not take ownership of existing Secret while patching the Secret", func() {
				expectedData := map[string][]byte{
					"foo": []byte("bar"),
				}
				expectedAnnotations := map[string]string{
					ssv1alpha1.SealedSecretPatchAnnotation: "true",
				}
				expectedLabels := map[string]string{
					"mylabel": "myvalue",
				}
				Eventually(func() (*v1.EventList, error) {
					return c.Events(ns).Search(scheme.Scheme, ss)
				}, Timeout, PollingInterval).Should(
					containEventWithReason(Equal("Unsealed")),
				)
				Eventually(func() (*ssv1alpha1.SealedSecret, error) {
					return ssc.BitnamiV1alpha1().SealedSecrets(ns).Get(context.Background(), secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).ShouldNot(WithTransform(getStatus, BeNil()))
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getNumberOfOwners, Equal(0)))
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getData, Equal(expectedData)))
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getAnnotations, Equal(expectedAnnotations)))
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getLabels, Equal(expectedLabels)))
			})
		})
	})

	Describe("Secret Recreation", func() {
		Context("With owned secret", func() {
			JustBeforeEach(func() {
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getFirstOwnerName, Equal(ss.GetName())))
				err := c.Secrets(ns).Delete(ctx, secretName, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			})
			It("should recreate the secret", func() {
				expected := map[string][]byte{
					"foo": []byte("bar"),
				}
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getData, Equal(expected)))
				Eventually(func() (*v1.EventList, error) {
					return c.Events(ns).Search(scheme.Scheme, ss)
				}, Timeout, PollingInterval).Should(
					containEventWithReason(Equal("Unsealed")),
				)
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getFirstOwnerName, Equal(ss.GetName())))
			})
		})

		Context("With unowned secret with managed annotation", func() {
			BeforeEach(func() {
				s.Data["foo2"] = []byte("bar2")
				s.Annotations = map[string]string{
					ssv1alpha1.SealedSecretManagedAnnotation: "true",
				}
				c.Secrets(ns).Create(ctx, s, metav1.CreateOptions{})
			})
			JustBeforeEach(func() {
				err := c.Secrets(ns).Delete(ctx, secretName, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			})
			It("should recreate the secret", func() {
				expected := map[string][]byte{
					"foo": []byte("bar"),
				}
				Eventually(func() (*v1.EventList, error) {
					return c.Events(ns).Search(scheme.Scheme, ss)
				}, Timeout, PollingInterval).Should(
					containEventWithReason(Equal("Unsealed")),
				)
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getFirstOwnerName, Equal(ss.GetName())))
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getData, Equal(expected)))
			})
		})

		Context("With unowned secret without managed annotation", func() {
			BeforeEach(func() {
				s.Annotations = map[string]string{}
				c.Secrets(ns).Create(ctx, s, metav1.CreateOptions{})
			})
			JustBeforeEach(func() {
				err := c.Secrets(ns).Delete(ctx, secretName, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			})
			It("should not recreate the secret", func() {
				Consistently(func() error {
					_, err := c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
					return err
				}).Should(WithTransform(errors.IsNotFound, Equal(true)))
			})
		})

		Context("With unowned secret with patch annotation", func() {
			BeforeEach(func() {
				s.Data = map[string][]byte{
					"foo":  []byte("bar1"),
					"foo2": []byte("bar2"),
				}
				s.Annotations = map[string]string{
					ssv1alpha1.SealedSecretPatchAnnotation: "true",
				}
				s.Labels["anotherlabel"] = "anothervalue"
				c.Secrets(ns).Create(ctx, s, metav1.CreateOptions{})
			})
			JustBeforeEach(func() {
				err := c.Secrets(ns).Delete(ctx, secretName, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not recreate the secret", func() {
				Consistently(func() error {
					_, err := c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
					return err
				}).Should(WithTransform(errors.IsNotFound, Equal(true)))
			})
		})
	})

	Describe("Same name, wrong key", func() {
		BeforeEach(func() {
			// NB: weak key-size - this is just a test case
			wrongKey, err := rsa.GenerateKey(rand.Reader, 1024)
			Expect(err).NotTo(HaveOccurred())

			fmt.Fprintf(GinkgoWriter, "Resealing with wrong key\n")
			ss, err = ssv1alpha1.NewSealedSecret(scheme.Codecs, &wrongKey.PublicKey, s)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should *not* produce a Secret", func() {
			Consistently(func() error {
				_, err := c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
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
				return c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
			}, Timeout, PollingInterval).Should(WithTransform(getData, Equal(expected)))
			Eventually(func() (*v1.Secret, error) {
				return c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
			}, Timeout, PollingInterval).Should(WithTransform(getSecretType, Equal(expectedType)))
			Eventually(func() (*ssv1alpha1.SealedSecret, error) {
				return ssc.BitnamiV1alpha1().SealedSecrets(ns).Get(context.Background(), secretName, metav1.GetOptions{})
			}, Timeout, PollingInterval).ShouldNot(WithTransform(getStatus, BeNil()))
			Eventually(func() (*v1.EventList, error) {
				return c.Events(ns).Search(scheme.Scheme, ss)
			}, Timeout, PollingInterval).Should(
				containEventWithReason(Equal("Unsealed")),
			)
		})
	})

	Describe("Immutable Secret", func() {
		BeforeEach(func() {
			ss.Spec.Template.Immutable = new(bool)
			*ss.Spec.Template.Immutable = true
		})

		It("should produce expected Secret", func() {
			Eventually(func() (*v1.Secret, error) {
				return c.Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
			}, Timeout, PollingInterval).Should(WithTransform(getSecretImmutable, Equal(true)))
			Eventually(func() (*ssv1alpha1.SealedSecret, error) {
				return ssc.BitnamiV1alpha1().SealedSecrets(ns).Get(context.Background(), secretName, metav1.GetOptions{})
			}, Timeout, PollingInterval).ShouldNot(WithTransform(getStatus, BeNil()))
			Eventually(func() (*v1.EventList, error) {
				return c.Events(ns).Search(scheme.Scheme, ss)
			}, Timeout, PollingInterval).Should(
				containEventWithReason(Equal("Unsealed")),
			)
		})
	})

	Describe("Immutable Secret Error", func() {
		BeforeEach(func() {
			ss.Spec.Template.Immutable = new(bool)
			*ss.Spec.Template.Immutable = true
		})

		JustBeforeEach(func() {
			var err error

			Eventually(func() (*ssv1alpha1.SealedSecret, error) {
				return ssc.BitnamiV1alpha1().SealedSecrets(ss.Namespace).Get(context.Background(), secretName, metav1.GetOptions{})
			}, Timeout, PollingInterval).ShouldNot(WithTransform(getStatus, BeNil()))

			ss, err = ssc.BitnamiV1alpha1().SealedSecrets(ss.Namespace).Get(context.Background(), secretName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			resVer := ss.ResourceVersion

			// update
			s.Data["foo"] = []byte("baz")
			ss, err = ssv1alpha1.NewSealedSecret(scheme.Codecs, pubKey, s)
			Expect(err).NotTo(HaveOccurred())
			ss.ResourceVersion = resVer

			fmt.Fprintf(GinkgoWriter, "Updating to SealedSecret: %#v\n", ss)
			ss, err = ssc.BitnamiV1alpha1().SealedSecrets(ss.Namespace).Update(context.Background(), ss, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should record update failure as an event", func() {
			Eventually(func() (*ssv1alpha1.SealedSecret, error) {
				return ssc.BitnamiV1alpha1().SealedSecrets(ns).Get(context.Background(), secretName, metav1.GetOptions{})
			}, Timeout, PollingInterval).ShouldNot(WithTransform(getStatus, BeNil()))
			Eventually(func() (*ssv1alpha1.SealedSecret, error) {
				return ssc.BitnamiV1alpha1().SealedSecrets(ns).Get(context.Background(), secretName, metav1.GetOptions{})
			}, Timeout, PollingInterval).Should(WithTransform(getObservedGeneration, Equal(int64(2))))
			Eventually(func() (*v1.EventList, error) {
				return c.Events(ns).Search(scheme.Scheme, ss)
			}, Timeout, PollingInterval).Should(
				containEventWithReason(Equal("ErrUpdateFailed")),
			)
			Eventually(func() (*v1.EventList, error) {
				return c.Events(ns).Search(scheme.Scheme, ss)
			}, Timeout, PollingInterval).Should(
				containEventWithMessage(ContainSubstring("the target Secret is immutable")),
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
					_, err := c.Secrets(ns).Get(ctx, secretName2, metav1.GetOptions{})
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
				ns2 = createNsOrDie(ctx, c, "create")
				ss.Namespace = ns2
			})
			AfterEach(func() {
				deleteNsOrDie(ctx, c, ns2)
			})

			It("should *not* produce a Secret", func() {
				Consistently(func() error {
					_, err := c.Secrets(ns2).Get(ctx, secretName, metav1.GetOptions{})
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
					return c.Secrets(ns).Get(ctx, secretName2, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getData, Equal(expected)))
				Eventually(func() (*ssv1alpha1.SealedSecret, error) {
					return ssc.BitnamiV1alpha1().SealedSecrets(ns).Get(context.Background(), secretName2, metav1.GetOptions{})
				}, Timeout, PollingInterval).ShouldNot(WithTransform(getStatus, BeNil()))
			})
		})

		Context("With wrong namespace and cluster-wide annotation", func() {
			var ns2 string
			BeforeEach(func() {
				ns2 = createNsOrDie(ctx, c, "create")
			})
			BeforeEach(func() {
				var err error

				s.Annotations = map[string]string{
					ssv1alpha1.SealedSecretClusterWideAnnotation: "true",
				}

				fmt.Fprintf(GinkgoWriter, "Re-sealing secret %#v\n", s)
				ss, err = ssv1alpha1.NewSealedSecret(scheme.Codecs, pubKey, s)
				ss.Namespace = ns2
				Expect(err).NotTo(HaveOccurred())
			})
			AfterEach(func() {
				deleteNsOrDie(ctx, c, ns2)
			})
			It("should produce expected Secret", func() {
				expected := map[string][]byte{
					"foo": []byte("bar"),
				}
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns2).Get(ctx, secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getData, Equal(expected)))
				Eventually(func() (*ssv1alpha1.SealedSecret, error) {
					return ssc.BitnamiV1alpha1().SealedSecrets(ns2).Get(context.Background(), secretName, metav1.GetOptions{})
				}, Timeout, PollingInterval).ShouldNot(WithTransform(getStatus, BeNil()))
			})
		})

		Context("With wrong name and namespace-wide annotation", func() {
			const secretName2 = "not-testsecret"
			BeforeEach(func() {
				var err error

				s.Annotations = map[string]string{
					ssv1alpha1.SealedSecretNamespaceWideAnnotation: "true",
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
					return c.Secrets(ns).Get(ctx, secretName2, metav1.GetOptions{})
				}, Timeout, PollingInterval).Should(WithTransform(getData, Equal(expected)))
			})
		})

		Context("With wrong namespace and namespace-wide annotation", func() {
			var ns2 string
			BeforeEach(func() {
				ns2 = createNsOrDie(ctx, c, "create")
			})
			BeforeEach(func() {
				var err error

				s.Annotations = map[string]string{
					ssv1alpha1.SealedSecretNamespaceWideAnnotation: "true",
				}

				fmt.Fprintf(GinkgoWriter, "Re-sealing secret %#v\n", s)
				ss, err = ssv1alpha1.NewSealedSecret(scheme.Codecs, pubKey, s)
				ss.Namespace = ns2
				Expect(err).NotTo(HaveOccurred())
			})
			AfterEach(func() {
				deleteNsOrDie(ctx, c, ns2)
			})

			It("should *not* produce a Secret", func() {
				Consistently(func() error {
					_, err := c.Secrets(ns2).Get(ctx, secretName, metav1.GetOptions{})
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
