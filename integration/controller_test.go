// +build integration

package integration

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	certUtil "k8s.io/client-go/util/cert"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealed-secrets/v1alpha1"
	ssclient "github.com/bitnami-labs/sealed-secrets/pkg/client/clientset/versioned"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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

var _ = Describe("create", func() {
	var c corev1.CoreV1Interface
	var ssc ssclient.Interface
	var ns string
	const secretName = "testsecret"
	var ss *ssv1alpha1.SealedSecret
	var s *v1.Secret

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
	})
	AfterEach(func() {
		deleteNsOrDie(c, ns)
	})

	Describe("Simple change", func() {
		BeforeEach(func() {
			_, certs, err := fetchKeys(c)
			Expect(err).NotTo(HaveOccurred())

			ss, err = ssv1alpha1.NewSealedSecret(scheme.Codecs, certs[0].PublicKey.(*rsa.PublicKey), s)
			Expect(err).NotTo(HaveOccurred())
		})
		JustBeforeEach(func() {
			var err error
			ss, err = ssc.BitnamiV1alpha1().SealedSecrets(ns).Create(ss)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("With no existing object (create)", func() {
			It("should produce expected Secret", func() {
				Eventually(func() (*v1.Secret, error) {
					return c.Secrets(ns).Get(secretName, metav1.GetOptions{})
				}).Should(WithTransform(getData, Equal(s.Data)))
			})
		})
	})
})
