//go:generate ../../../../vendor/k8s.io/code-generator/generate-groups.sh all github.com/bitnami-labs/sealed-secrets/pkg/client github.com/bitnami-labs/sealed-secrets/pkg/apis sealed-secrets:v1alpha1
// +k8s:deepcopy-gen=package,register

// +groupName=bitnami.com
package v1alpha1
