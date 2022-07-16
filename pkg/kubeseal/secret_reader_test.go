package kubeseal

import (
	"fmt"
	"strings"
	"testing"

	"k8s.io/client-go/kubernetes/scheme"
)

func TestYAMLStream(t *testing.T) {
	s1 := mkTestSecret(t, "foo", "1", withSecretName("s1"), asYAML(true))
	s2 := mkTestSecret(t, "var", "2", withSecretName("s2"), asYAML(true))
	bad := fmt.Sprintf("%s\n---\n%s\n", s1, s2)

	_, err := ReadSecret(scheme.Codecs.UniversalDecoder(), strings.NewReader(bad))
	if err == nil {
		t.Fatalf("error expected")
	}
}
