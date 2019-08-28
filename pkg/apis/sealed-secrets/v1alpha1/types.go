package v1alpha1

import (
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// SealedSecretName is the name used in SealedSecret CRD
	SealedSecretName = "sealed-secret." + GroupName
	// SealedSecretPlural is the collection plural used with SealedSecret API
	SealedSecretPlural = "sealedsecrets"

	// Annotation namespace prefix
	annoNs = "sealedsecrets." + GroupName + "/"

	// SealedSecretClusterWideAnnotation is the name for the annotation for
	// setting the secret to be available cluster wide.
	SealedSecretClusterWideAnnotation = annoNs + "cluster-wide"

	// SealedSecretNamespaceWideAnnotation is the name for the annotation for
	// setting the secret to be available namespace wide.
	SealedSecretNamespaceWideAnnotation = annoNs + "namespace-wide"

	// jenkinsKubernetesCredentialProvider is the name for the annotation and label for
	// setting the secret with jenkins annotation and label for kubernetes credentials provider in
	// order to be able to scan credentials and dynamically add those to jenkins
	JenkinsKubernetesCredentialProviderAnnotation = "jenkins.io/credentials-description"
	JenkinsKubernetesCredentialProviderLabel      = "jenkins.io/credentials-type"

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

	// +optional
	Type apiv1.SecretType `json:"type,omitempty" protobuf:"bytes,3,opt,name=type,casttype=SecretType"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SealedSecretList represents a list of SealedSecrets
type SealedSecretList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []SealedSecret `json:"items"`
}
