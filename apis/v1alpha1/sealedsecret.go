package v1alpha1

import (
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"

	"github.com/golang/glog"

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
)

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

func labelFor(o metav1.Object) []byte {
	label := fmt.Sprintf("%s/%s", o.GetNamespace(), o.GetName())
	return []byte(label)
}

// NewSealedSecret creates a new SealedSecret object wrapping the provided secret.
func NewSealedSecret(codecs runtimeserializer.CodecFactory, rnd io.Reader, key *rsa.PublicKey, secret *v1.Secret) (*SealedSecret, error) {
	info, ok := runtime.SerializerInfoForMediaType(codecs.SupportedMediaTypes(), runtime.ContentTypeJSON)
	if !ok {
		return nil, fmt.Errorf("binary can't serialize JSON")
	}
	codec := codecs.EncoderForVersion(info.Serializer, secret.GroupVersionKind().GroupVersion())

	plaintext, err := runtime.Encode(codec, secret)
	if err != nil {
		return nil, err
	}

	// This will fail to decrypt unless the same label was used
	// during encryption.
	label := labelFor(secret)

	ciphertext, err := rsa.EncryptOAEP(sha256.New(), rnd, key, plaintext, label)
	if err != nil {
		return nil, err
	}

	return &SealedSecret{
		Metadata: metav1.ObjectMeta{
			Name:      secret.GetName(),
			Namespace: secret.GetNamespace(),
		},
		Spec: SealedSecretSpec{
			Data: ciphertext,
		},
	}, nil
}

// Unseal decypts and returns the embedded v1.Secret.
func (s *SealedSecret) Unseal(codecs runtimeserializer.CodecFactory, rnd io.Reader, key *rsa.PrivateKey) (*v1.Secret, error) {
	boolTrue := true
	smeta := s.GetObjectMeta()

	ciphertext := s.Spec.Data

	// This will fail to decrypt unless the same label was used
	// during encryption.
	label := labelFor(smeta)

	plaintext, err := rsa.DecryptOAEP(sha256.New(), rnd, key, ciphertext, label)
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

	// Refer back to owning SealedSecret
	ownerRefs := []metav1.OwnerReference{
		metav1.OwnerReference{
			APIVersion: s.GetObjectKind().GroupVersionKind().GroupVersion().String(),
			Kind:       s.GetObjectKind().GroupVersionKind().Kind,
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
	glog.Infof("unmarshaljson called")
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
	glog.Infof("unmarshaljson (list) called")
	tmp := SealedSecretListCopy{}
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}
	tmp2 := SealedSecretList(tmp)
	*sl = tmp2
	return nil
}
