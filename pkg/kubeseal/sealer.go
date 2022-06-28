package kubeseal

import (
	"bytes"
	"crypto/rsa"
	"fmt"
	"io"
	"os"

	ssv1alpha1 "github.com/bitnami-labs/sealed-secrets/pkg/apis/sealed-secrets/v1alpha1"
	"github.com/bitnami-labs/sealed-secrets/pkg/client/clientset/versioned/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
)

type SealInstruction struct {
	OutputFormat      string
	In                io.Reader
	Out               io.Writer
	Codecs            runtimeserializer.CodecFactory
	PubKey            *rsa.PublicKey
	Scope             ssv1alpha1.SealingScope
	AllowEmptyData    bool
	DefaultNamespace  string
	OverrideName      string
	OverrideNamespace string
}

// Seal reads a k8s Secret resource parsed from an input reader by a given codec, encrypts all its secrets
// with a given public key, using the name and namespace found in the input secret, unless explicitly overridden
// by the overrideName and overrideNamespace arguments.
func Seal(i SealInstruction) error {
	secret, err := ReadSecret(i.Codecs.UniversalDecoder(), i.In)
	if err != nil {
		return err
	}

	if len(secret.Data) == 0 && len(secret.StringData) == 0 && !i.AllowEmptyData {
		return fmt.Errorf("Secret.data is empty in input Secret, assuming this is an error and aborting. To work with empty data, --allow-empty-data can be used.")
	}

	if i.OverrideName != "" {
		secret.Name = i.OverrideName
	}

	if secret.GetName() == "" {
		return fmt.Errorf("Missing metadata.name in input Secret")
	}

	if i.OverrideNamespace != "" {
		secret.Namespace = i.OverrideNamespace
	}

	if i.Scope != ssv1alpha1.DefaultScope {
		secret.Annotations = ssv1alpha1.UpdateScopeAnnotations(secret.Annotations, i.Scope)
	}

	if ssv1alpha1.SecretScope(secret) != ssv1alpha1.ClusterWideScope && secret.GetNamespace() == "" {
		secret.SetNamespace(i.DefaultNamespace)
	}

	// Strip read-only server-side ObjectMeta (if present)
	secret.SetSelfLink("")
	secret.SetUID("")
	secret.SetResourceVersion("")
	secret.Generation = 0
	secret.SetCreationTimestamp(metav1.Time{})
	secret.SetDeletionTimestamp(nil)
	secret.DeletionGracePeriodSeconds = nil

	ssecret, err := ssv1alpha1.NewSealedSecret(i.Codecs, i.PubKey, secret)
	if err != nil {
		return err
	}
	if err = ResourceOutput(i.Out, i.Codecs, ssv1alpha1.SchemeGroupVersion, ssecret, i.OutputFormat); err != nil {
		return err
	}
	return nil
}

type SealMergeIntoInstruction struct {
	OutputFormat   string
	In             io.Reader
	Filename       string
	Codecs         runtimeserializer.CodecFactory
	PubKey         *rsa.PublicKey
	Scope          ssv1alpha1.SealingScope
	AllowEmptyData bool
}

func SealMergingInto(i SealMergeIntoInstruction) error {
	// #nosec G304 -- should open user provided file
	f, err := os.OpenFile(i.Filename, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	// #nosec G307 -- we are explicitly managing a potential error from f.Close() at the end of the function
	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	orig, err := DecodeSealedSecret(i.Codecs, b)
	if err != nil {
		return err
	}

	var buf bytes.Buffer

	sealInstruction := SealInstruction{
		In:                i.In,
		Out:               &buf,
		Codecs:            scheme.Codecs,
		PubKey:            i.PubKey,
		Scope:             i.Scope,
		AllowEmptyData:    i.AllowEmptyData,
		OverrideName:      orig.Name,
		OverrideNamespace: orig.Namespace,
	}

	if err := Seal(sealInstruction); err != nil {
		return err
	}

	update, err := DecodeSealedSecret(i.Codecs, buf.Bytes())
	if err != nil {
		return err
	}

	// merge encrypted data and metadata
	for k, v := range update.Spec.EncryptedData {
		orig.Spec.EncryptedData[k] = v
	}
	for k, v := range update.Spec.Template.Annotations {
		orig.Spec.Template.Annotations[k] = v
	}
	for k, v := range update.Spec.Template.Labels {
		orig.Spec.Template.Labels[k] = v
	}
	for k, v := range update.Spec.Template.Data {
		orig.Spec.Template.Data[k] = v
	}

	// updated sealed secret file in-place avoiding clobbering the file upon rendering errors.
	var out bytes.Buffer
	if err := ResourceOutput(&out, i.Codecs, ssv1alpha1.SchemeGroupVersion, orig, i.OutputFormat); err != nil {
		return err
	}

	if err := f.Truncate(0); err != nil {
		return err
	}
	if _, err := f.Seek(0, 0); err != nil {
		return err
	}
	if _, err := io.Copy(f, &out); err != nil {
		return err
	}
	// we explicitly call f.Close() to return a pontential error when closing the file that wouldn't be returned in the deferred f.Close()
	if err := f.Close(); err != nil {
		return err
	}
	return nil
}
