package v1alpha1

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"

	"github.com/bitnami/sealed-secrets/pkg/crypto"
)

func labelFor(o metav1.Object) ([]byte, bool) {
	label := o.GetAnnotations()[SealedSecretClusterWideAnnotation]
	if label == "true" {
		return []byte(""), true
	}
	label = fmt.Sprintf("%s/%s", o.GetNamespace(), o.GetName())
	return []byte(label), false
}

// NewSealedSecret creates a new SealedSecret object wrapping the
// provided secret.
func NewSealedSecret(codecs runtimeserializer.CodecFactory, pubKey *rsa.PublicKey, secret *v1.Secret) (*SealedSecret, error) {
	if secret.GetNamespace() == "" {
		return nil, fmt.Errorf("Secret must declare a namespace")
	}

	s := &SealedSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secret.GetName(),
			Namespace: secret.GetNamespace(),
		},
		Spec: SealedSecretSpec{
			Data: map[string][]byte{},
		},
	}

	// RSA-OAEP will fail to decrypt unless the same label is used
	// during decryption.
	label, clusterWide := labelFor(secret)

	for key, value := range secret.Data {
		ciphertext, err := crypto.HybridEncrypt(rand.Reader, pubKey, value, label)
		if err != nil {
			return nil, err
		}
		s.Spec.Data[key] = ciphertext
	}

	if clusterWide {
		s.Annotations = map[string]string{SealedSecretClusterWideAnnotation: "true"}
	}
	return s, nil
}

// Unseal decypts and returns the embedded v1.Secret.
func (s *SealedSecret) Unseal(codecs runtimeserializer.CodecFactory, privKey *rsa.PrivateKey) (*v1.Secret, error) {
	boolTrue := true
	smeta := s.GetObjectMeta()

	// This will fail to decrypt unless the same label was used
	// during encryption.  This check ensures that we can't be
	// tricked into decrypting a sealed secret into an unexpected
	// namespace/name.
	label, _ := labelFor(smeta)

	var secret v1.Secret
	secret.Data = map[string][]byte{}
	for key, value := range s.Spec.Data {
		plaintext, err := crypto.HybridDecrypt(rand.Reader, privKey, value, label)
		if err != nil {
			return nil, err
		}
		secret.Data[key] = plaintext
	}

	// Ensure these are set to what we expect
	secret.SetNamespace(smeta.GetNamespace())
	secret.SetName(smeta.GetName())

	// This is sometimes empty?  Fine - we know what the answer is
	// going to be anyway.
	//gvk := s.GetObjectKind().GroupVersionKind()
	gvk := SchemeGroupVersion.WithKind("SealedSecret")

	// Refer back to owning SealedSecret
	ownerRefs := []metav1.OwnerReference{
		{
			APIVersion: gvk.GroupVersion().String(),
			Kind:       gvk.Kind,
			Name:       smeta.GetName(),
			UID:        smeta.GetUID(),
			Controller: &boolTrue,
		},
	}
	secret.SetOwnerReferences(ownerRefs)

	return &secret, nil
}
