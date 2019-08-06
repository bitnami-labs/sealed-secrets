package v1alpha1

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"

	"github.com/bitnami-labs/sealed-secrets/pkg/crypto"
)

// SealedSecretExpansion has methods to work with SealedSecrets resources.
type SealedSecretExpansion interface {
	Unseal(codecs runtimeserializer.CodecFactory, privKeys map[string]*rsa.PrivateKey) (*v1.Secret, error)
}

// Returns labels followed by clusterWide followed by namespaceWide.
func labelFor(o metav1.Object) ([]byte, bool, bool) {
	clusterWide := o.GetAnnotations()[SealedSecretClusterWideAnnotation]
	if clusterWide == "true" {
		return []byte(""), true, false
	}
	namespaceWide := o.GetAnnotations()[SealedSecretNamespaceWideAnnotation]
	if namespaceWide == "true" {
		return []byte(o.GetNamespace()), false, true
	}
	return []byte(fmt.Sprintf("%s/%s", o.GetNamespace(), o.GetName())), false, false
}

// NewSealedSecretV1 creates a new SealedSecret object wrapping the
// provided secret. This encrypts all the secrets into a single encrypted
// blob and stores it in the `Data` attribute. Keeping this for backward
// compatibility.
func NewSealedSecretV1(codecs runtimeserializer.CodecFactory, pubKey *rsa.PublicKey, secret *v1.Secret) (*SealedSecret, error) {
	info, ok := runtime.SerializerInfoForMediaType(codecs.SupportedMediaTypes(), runtime.ContentTypeJSON)
	if !ok {
		return nil, fmt.Errorf("binary can't serialize JSON")
	}

	if secret.GetNamespace() == "" {
		return nil, fmt.Errorf("Secret must declare a namespace")
	}

	codec := codecs.EncoderForVersion(info.Serializer, v1.SchemeGroupVersion)
	plaintext, err := runtime.Encode(codec, secret)
	if err != nil {
		return nil, err
	}

	// RSA-OAEP will fail to decrypt unless the same label is used
	// during decryption.
	label, clusterWide, namespaceWide := labelFor(secret)

	ciphertext, err := crypto.HybridEncrypt(rand.Reader, pubKey, plaintext, label)
	if err != nil {
		return nil, err
	}

	s := &SealedSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secret.GetName(),
			Namespace: secret.GetNamespace(),
		},
		Spec: SealedSecretSpec{
			Data: ciphertext,
		},
	}

	if clusterWide {
		s.Annotations = map[string]string{SealedSecretClusterWideAnnotation: "true"}
	}
	if namespaceWide {
		s.Annotations = map[string]string{SealedSecretNamespaceWideAnnotation: "true"}
	}
	return s, nil
}

// NewSealedSecret creates a new SealedSecret object wrapping the
// provided secret. This encrypts only the values of each secrets
// individually, so secrets can be updated one by one.
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
			Template: SecretTemplateSpec{
				// ObjectMeta copied below
				Type: secret.Type,
			},
			EncryptedData: map[string]string{},
		},
	}
	secret.ObjectMeta.DeepCopyInto(&s.Spec.Template.ObjectMeta)

	// RSA-OAEP will fail to decrypt unless the same label is used
	// during decryption.
	label, clusterWide, namespaceWide := labelFor(secret)

	for key, value := range secret.Data {
		ciphertext, err := crypto.HybridEncrypt(rand.Reader, pubKey, value, label)
		if err != nil {
			return nil, err
		}
		s.Spec.EncryptedData[key] = base64.StdEncoding.EncodeToString(ciphertext)
	}

	for key, value := range secret.StringData {
		ciphertext, err := crypto.HybridEncrypt(rand.Reader, pubKey, []byte(value), label)
		if err != nil {
			return nil, err
		}
		s.Spec.EncryptedData[key] = base64.StdEncoding.EncodeToString(ciphertext)
	}

	if clusterWide {
		if s.Annotations == nil {
			s.Annotations = map[string]string{}
		}
		s.Annotations[SealedSecretClusterWideAnnotation] = "true"
	}
	if namespaceWide {
		s.Annotations = map[string]string{SealedSecretNamespaceWideAnnotation: "true"}
	}
	return s, nil
}

// Unseal decrypts and returns the embedded v1.Secret.
func (s *SealedSecret) Unseal(codecs runtimeserializer.CodecFactory, privKeys map[string]*rsa.PrivateKey) (*v1.Secret, error) {
	boolTrue := true
	smeta := s.GetObjectMeta()

	// This will fail to decrypt unless the same label was used
	// during encryption.  This check ensures that we can't be
	// tricked into decrypting a sealed secret into an unexpected
	// namespace/name.
	label, _, _ := labelFor(smeta)

	var secret v1.Secret
	if len(s.Spec.EncryptedData) > 0 {
		s.Spec.Template.ObjectMeta.DeepCopyInto(&secret.ObjectMeta)
		secret.Type = s.Spec.Template.Type

		secret.Data = map[string][]byte{}
		for key, value := range s.Spec.EncryptedData {
			valueBytes, err := base64.StdEncoding.DecodeString(value)
			if err != nil {
				return nil, err
			}
			plaintext, err := crypto.HybridDecrypt(rand.Reader, privKeys, valueBytes, label)
			if err != nil {
				return nil, err
			}
			secret.Data[key] = plaintext
		}

	} else { // Support decrypting old secrets for backward compatibility
		plaintext, err := crypto.HybridDecrypt(rand.Reader, privKeys, s.Spec.Data, label)
		if err != nil {
			return nil, err
		}

		dec := codecs.UniversalDecoder(secret.GroupVersionKind().GroupVersion())
		if err = runtime.DecodeInto(dec, plaintext, &secret); err != nil {
			return nil, err
		}
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
