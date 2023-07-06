package multidocyaml

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v2"
)

func isMultiDocumentYAML(src []byte) bool {
	dec := yaml.NewDecoder(bytes.NewReader(src))
	var dummy struct{}
	_ = dec.Decode(&dummy)
	return dec.Decode(&dummy) == nil
}

// EnsureNotMultiDoc returns an error if the yaml.
func EnsureNotMultiDoc(src []byte) error {
	if isMultiDocumentYAML(src) {
		return fmt.Errorf("Multistream YAML not supported yet (see https://github.com/bitnami-labs/sealed-secrets/issues/114)")
	}
	return nil
}
