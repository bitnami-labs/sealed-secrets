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
	"github.com/mkmik/multierror"
)

const (
	// The StrictScope pins the sealed secret to a specific namespace and a specific name.
	StrictScope SealingScope = iota
	// The NamespaceWideScope only pins a sealed secret to a specific namespace.
	NamespaceWideScope
	// The ClusterWideScope allows the sealed secret to be unsealed in any namespace of the cluster.
	ClusterWideScope

	// The DefaultScope is currently the StrictScope.
	DefaultScope = StrictScope
)

var (
	// TODO(mkm): remove after a release
	AcceptDeprecatedV1Data = false
)

// SealedSecretExpansion has methods to work with SealedSecrets resources.
type SealedSecretExpansion interface {
	Unseal(codecs runtimeserializer.CodecFactory, privKeys map[string]*rsa.PrivateKey) (*v1.Secret, error)
}

// SealingScope is an enum that declares the mobility of a sealed secret by defining
// in which scopes
type SealingScope int

func (s *SealingScope) String() string {
	switch *s {
	case StrictScope:
		return "strict"
	case NamespaceWideScope:
		return "namespace-wide"
	case ClusterWideScope:
		return "cluster-wide"
	default:
		return fmt.Sprintf("undefined-%d", *s)
	}
}

func (s *SealingScope) Set(v string) error {
	switch v {
	case "":
		*s = DefaultScope
	case "strict":
		*s = StrictScope
	case "namespace-wide":
		*s = NamespaceWideScope
	case "cluster-wide":
		*s = ClusterWideScope
	default:
		return fmt.Errorf("must be one of: strict, namespace-wide, cluster-wide")
	}
	return nil
}

// Type implements the pflag.Value interface
func (s *SealingScope) Type() string { return "string" }

// EncryptionLabel returns the label meant to be used for encrypting a sealed secret according to scope.
func EncryptionLabel(namespace, name string, scope SealingScope) []byte {
	var l string
	switch scope {
	case ClusterWideScope:
		l = ""
	case NamespaceWideScope:
		l = namespace
	case StrictScope:
		fallthrough
	default:
		l = fmt.Sprintf("%s/%s", namespace, name)
	}
	return []byte(l)
}

// Returns labels followed by clusterWide followed by namespaceWide.
func labelFor(o metav1.Object) []byte {
	return EncryptionLabel(o.GetNamespace(), o.GetName(), SecretScope(o))
}

// SecretScope returns the scope of a secret to be sealed, as annotated in its metadata.
func SecretScope(o metav1.Object) SealingScope {
	if o.GetAnnotations()[SealedSecretClusterWideAnnotation] == "true" {
		return ClusterWideScope
	}
	if o.GetAnnotations()[SealedSecretNamespaceWideAnnotation] == "true" {
		return NamespaceWideScope
	}
	return StrictScope
}

// Scope returns the scope of the sealed secret, as annotated in its metadata.
func (s *SealedSecret) Scope() SealingScope {
	return SecretScope(&s.Spec.Template)
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

	if SecretScope(secret) != ClusterWideScope && secret.GetNamespace() == "" {
		return nil, fmt.Errorf("secret must declare a namespace")
	}

	codec := codecs.EncoderForVersion(info.Serializer, v1.SchemeGroupVersion)
	plaintext, err := runtime.Encode(codec, secret)
	if err != nil {
		return nil, err
	}

	// RSA-OAEP will fail to decrypt unless the same label is used
	// during decryption.
	label := labelFor(secret)

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

	s.Annotations = UpdateScopeAnnotations(s.Annotations, SecretScope(secret))

	return s, nil
}

// UpdateScopeAnnotations updates the annotation map so that it reflects the desired scope.
// It does so by updating and/or deleting existing annotations.
func UpdateScopeAnnotations(anno map[string]string, scope SealingScope) map[string]string {
	if anno == nil {
		anno = map[string]string{}
	}
	delete(anno, SealedSecretNamespaceWideAnnotation)
	delete(anno, SealedSecretClusterWideAnnotation)

	if scope == NamespaceWideScope {
		anno[SealedSecretNamespaceWideAnnotation] = "true"
	}
	if scope == ClusterWideScope {
		anno[SealedSecretClusterWideAnnotation] = "true"
	}
	return anno
}

// StripLastAppliedAnnotations strips annotations added by tools such as kubectl and kubecfg
// that contain a full copy of the original object kept in the annotation for strategic-merge-patch
// purposes. We need to remove these annotations when sealing an existing secret otherwise we'd leak
// the secrets.
func StripLastAppliedAnnotations(annotations map[string]string) {
	if annotations == nil {
		return
	}
	keys := []string{
		"kubectl.kubernetes.io/last-applied-configuration",
		"kubecfg.ksonnet.io/last-applied-configuration",
	}
	for _, k := range keys {
		delete(annotations, k)
	}
}

// NewSealedSecret creates a new SealedSecret object wrapping the
// provided secret. This encrypts only the values of each secrets
// individually, so secrets can be updated one by one.
func NewSealedSecret(codecs runtimeserializer.CodecFactory, pubKey *rsa.PublicKey, secret *v1.Secret) (*SealedSecret, error) {
	if SecretScope(secret) != ClusterWideScope && secret.GetNamespace() == "" {
		return nil, fmt.Errorf("secret must declare a namespace")
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

	// the input secret could come from a real secret object applied with `kubectl apply` or similar tools
	// which put a copy of the object version at application time in an annotation in order to support
	// strategic merge patch in subsequent updates. We need to strip those annotations or else we would
	// be leaking secrets in clear in a way that might be non obvious to users.
	// See https://github.com/bitnami-labs/sealed-secrets/issues/227
	StripLastAppliedAnnotations(s.Spec.Template.ObjectMeta.Annotations)

	// Cleanup ownerReference (See #243)
	s.Spec.Template.ObjectMeta.OwnerReferences = nil

	// RSA-OAEP will fail to decrypt unless the same label is used
	// during decryption.
	label := labelFor(secret)

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

	s.Annotations = UpdateScopeAnnotations(s.Annotations, SecretScope(secret))

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
	label := labelFor(smeta)

	var secret v1.Secret
	if len(s.Spec.EncryptedData) > 0 {
		s.Spec.Template.ObjectMeta.DeepCopyInto(&secret.ObjectMeta)
		secret.Type = s.Spec.Template.Type

		secret.Data = map[string][]byte{}

		var errs []error
		for key, value := range s.Spec.EncryptedData {
			valueBytes, err := base64.StdEncoding.DecodeString(value)
			if err != nil {
				return nil, err
			}
			plaintext, err := crypto.HybridDecrypt(rand.Reader, privKeys, valueBytes, label)
			if err != nil {
				errs = append(errs, multierror.Tag(key, err))
			}
			secret.Data[key] = plaintext
		}

		if errs != nil {
			return nil, multierror.Join(multierror.Uniq(errs), multierror.WithFormatter(multierror.InlineFormatter))
		}
	} else if AcceptDeprecatedV1Data { // Support decrypting old secrets for backward compatibility
		plaintext, err := crypto.HybridDecrypt(rand.Reader, privKeys, s.Spec.Data, label)
		if err != nil {
			return nil, err
		}

		dec := codecs.UniversalDecoder(secret.GroupVersionKind().GroupVersion())
		if err = runtime.DecodeInto(dec, plaintext, &secret); err != nil {
			return nil, err
		}
	} else {
		if s.Spec.Data != nil {
			return nil, fmt.Errorf("using deprecated 'data' field, use 'encryptedData' or flip the feature flag")
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
