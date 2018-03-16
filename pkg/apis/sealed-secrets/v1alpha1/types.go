package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// SealedSecretName is the name used in SealedSecret TPR
	SealedSecretName = "sealed-secret." + GroupName
	// SealedSecretPlural is the collection plural used with SealedSecret API
	SealedSecretPlural = "sealedsecrets"

	// Annotation namespace prefix
	annoNs = "sealedsecrets." + GroupName + "/"

	// SealedSecretClusterWideAnnotation is the name for the annotation for
	// setting the secret to be availible cluster wide.
	SealedSecretClusterWideAnnotation = annoNs + "cluster-wide"
)

// SealedSecretSpec is the specification of a SealedSecret
type SealedSecretSpec struct {
	// Data is deprecated and will be removed eventually. Use per-value EncryptedData instead.
	Data          []byte            `json:"data,omitempty"`
	EncryptedData map[string][]byte `json:"encryptedData"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient

// SealedSecret is the K8s representation of a "sealed Secret" - a
// regular k8s Secret that has been sealed (encrypted) using the
// controller's key.
type SealedSecret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec SealedSecretSpec `json:"spec"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SealedSecretList represents a list of SealedSecrets
type SealedSecretList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []SealedSecret `json:"items"`
}
