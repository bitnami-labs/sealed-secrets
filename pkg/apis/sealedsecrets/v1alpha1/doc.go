// go mod vendor doesn't preserve executable perm bits
//go:generate bash -c "go mod download && cd ../../../.. && bash $(go list -mod=mod -m -f '{{.Dir}}' k8s.io/code-generator)/generate-groups.sh deepcopy,client,informer,lister github.com/bitnami-labs/sealed-secrets/pkg/client github.com/bitnami-labs/sealed-secrets/pkg/apis sealedsecrets:v1alpha1 --go-header-file pkg/apis/sealedsecrets/v1alpha1/boilerplate.go.txt --trim-path-prefix github.com/bitnami-labs/sealed-secrets"
// +k8s:deepcopy-gen=package,register

// +groupName=bitnami.com

// Package v1alpha1 contains the definition of the sealed-secrets v1alpha1 API. Some of the code in this package is generated.
package v1alpha1
