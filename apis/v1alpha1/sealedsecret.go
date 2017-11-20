package v1alpha1

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/pkg/api/v1"
)

const (
	// SealedSecretName is the name used in SealedSecret TPR
	SealedSecretName = "sealed-secret." + GroupName
	// SealedSecretPlural is the collection plural used with SealedSecret API
	SealedSecretPlural = "sealedsecrets"

	// SealedSecretClusterWideAnnotation is the name for the annotation for
	// setting the secret to be availible cluster wide.
	SealedSecretClusterWideAnnotation = "sealedsecrets.bitnami.com/cluster-wide"

	sessionKeyBytes = 32
)

// ErrTooShort indicates the provided data is too short to be valid
var ErrTooShort = errors.New("SealedSecret data is too short")

// SealedSecretSpec is the specification of a SealedSecret
type SealedSecretSpec struct {
	Data []byte `json:"data"`
}

// SealedSecret is the K8s representation of a "sealed Secret" - a
// regular k8s Secret that has been sealed (encrypted) using the
// controller's key.
type SealedSecret struct {
	metav1.TypeMeta `json:",inline"`
	// Note, can't use implicit object here:
	// https://github.com/kubernetes/client-go/issues/8
	Metadata metav1.ObjectMeta `json:"metadata"`

	Spec SealedSecretSpec `json:"spec"`
}

// SealedSecretList represents a list of SealedSecrets
type SealedSecretList struct {
	metav1.TypeMeta `json:",inline"`
	Metadata        metav1.ListMeta `json:"metadata"`

	Items []SealedSecret `json:"items"`
}

// GetObjectKind is required for Object interface
func (s *SealedSecret) GetObjectKind() schema.ObjectKind {
	return &s.TypeMeta
}

// GetObjectMeta is required for ObjectMetaAccessor interface
func (s *SealedSecret) GetObjectMeta() metav1.Object {
	return &s.Metadata
}

// GetObjectKind is required for Object interface
func (sl *SealedSecretList) GetObjectKind() schema.ObjectKind {
	return &sl.TypeMeta
}

// GetListMeta is required for ListMetaAccessor interface
func (sl *SealedSecretList) GetListMeta() metav1.List {
	return &sl.Metadata
}

func labelFor(o metav1.Object) ([]byte, bool) {
	label := o.GetAnnotations()[SealedSecretClusterWideAnnotation]
	if label == "true" {
		return []byte(""), true
	}
	label = fmt.Sprintf("%s/%s", o.GetNamespace(), o.GetName())
	return []byte(label), false
}

func hybridEncrypt(rnd io.Reader, pubKey *rsa.PublicKey, plaintext, label []byte) ([]byte, error) {
	// Generate a random symmetric key
	sessionKey := make([]byte, sessionKeyBytes)
	if _, err := io.ReadFull(rnd, sessionKey); err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(sessionKey)
	if err != nil {
		return nil, err
	}

	aed, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Encrypt symmetric key
	rsaCiphertext, err := rsa.EncryptOAEP(sha256.New(), rnd, pubKey, sessionKey, label)
	if err != nil {
		return nil, err
	}

	// First 2 bytes are RSA ciphertext length, so we can separate
	// all the pieces later.
	ciphertext := make([]byte, 2)
	binary.BigEndian.PutUint16(ciphertext, uint16(len(rsaCiphertext)))
	ciphertext = append(ciphertext, rsaCiphertext...)

	// SessionKey is only used once, so zero nonce is ok
	zeroNonce := make([]byte, aed.NonceSize())

	// Append symmetrically encrypted Secret
	ciphertext = aed.Seal(ciphertext, zeroNonce, plaintext, nil)

	return ciphertext, nil
}

func hybridDecrypt(rnd io.Reader, privKey *rsa.PrivateKey, ciphertext, label []byte) ([]byte, error) {
	if len(ciphertext) < 2 {
		return nil, ErrTooShort
	}
	rsaLen := int(binary.BigEndian.Uint16(ciphertext))
	if len(ciphertext) < rsaLen+2 {
		return nil, ErrTooShort
	}

	rsaCiphertext := ciphertext[2 : rsaLen+2]
	aesCiphertext := ciphertext[rsaLen+2:]

	sessionKey, err := rsa.DecryptOAEP(sha256.New(), rnd, privKey, rsaCiphertext, label)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(sessionKey)
	if err != nil {
		return nil, err
	}

	aed, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Key is only used once, so zero nonce is ok
	zeroNonce := make([]byte, aed.NonceSize())

	plaintext, err := aed.Open(nil, zeroNonce, aesCiphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// NewSealedSecret creates a new SealedSecret object wrapping the
// provided secret.
func NewSealedSecret(codecs runtimeserializer.CodecFactory, pubKey *rsa.PublicKey, secret *v1.Secret) (*SealedSecret, error) {
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
	label, clusterWide := labelFor(secret)

	ciphertext, err := hybridEncrypt(rand.Reader, pubKey, plaintext, label)
	if err != nil {
		return nil, err
	}

	s := &SealedSecret{
		Metadata: metav1.ObjectMeta{
			Name:      secret.GetName(),
			Namespace: secret.GetNamespace(),
		},
		Spec: SealedSecretSpec{
			Data: ciphertext,
		},
	}

	if clusterWide {
		s.Metadata.Annotations = map[string]string{SealedSecretClusterWideAnnotation: "true"}
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

	plaintext, err := hybridDecrypt(rand.Reader, privKey, s.Spec.Data, label)
	if err != nil {
		return nil, err
	}

	var secret v1.Secret
	dec := codecs.UniversalDecoder(secret.GroupVersionKind().GroupVersion())
	if err = runtime.DecodeInto(dec, plaintext, &secret); err != nil {
		return nil, err
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

// The code below is used only to work around a known problem with third-party
// resources and ugorji. If/when these issues are resolved, the code below
// should no longer be required.

// SealedSecretListCopy is a workaround for a ugorji issue
type SealedSecretListCopy SealedSecretList

// SealedSecretCopy is a workaround for a ugorji issue
type SealedSecretCopy SealedSecret

// UnmarshalJSON ensures SealedSecret objects can be decoded from JSON successfully
func (s *SealedSecret) UnmarshalJSON(data []byte) error {
	tmp := SealedSecretCopy{}
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}
	tmp2 := SealedSecret(tmp)
	*s = tmp2
	return nil
}

// UnmarshalJSON ensures SealedSecretList objects can be decoded from JSON successfully
func (sl *SealedSecretList) UnmarshalJSON(data []byte) error {
	tmp := SealedSecretListCopy{}
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}
	tmp2 := SealedSecretList(tmp)
	*sl = tmp2
	return nil
}
