// go mod vendor doesn't preserve executable perm bits
//go:generate bash ../../../../vendor/k8s.io/code-generator/generate-groups.sh all github.com/bitnami-labs/sealed-secrets/pkg/client github.com/bitnami-labs/sealed-secrets/pkg/apis sealed-secrets:v1alpha1 --go-header-file boilerplate.go.txt
// +k8s:deepcopy-gen=package,register

// +groupName=bitnami.com
package v1alpha1
