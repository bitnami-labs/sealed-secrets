package multidocyaml

import (
	"bytes"
	"errors"
	"fmt"

	"gopkg.in/yaml.v2"
)

func isMultiDocumentYAML(src []byte) bool {
	dec := yaml.NewDecoder(bytes.NewReader(src))
	var dummy struct{}
	if err := dec.Decode(&dummy); err != nil {
		// A *yaml.TypeError only means the document doesn't fit the dummy
		// value; the decoder can still move on to the next document.
		// Any other error leaves the yaml.v2 decoder in a state where calling
		// Decode again panics, so stop here and let the caller's own parsing
		// report the malformed input.
		var typeErr *yaml.TypeError
		if !errors.As(err, &typeErr) {
			return false
		}
	}
	return dec.Decode(&dummy) == nil
}

// EnsureNotMultiDoc returns an error if the yaml.
func EnsureNotMultiDoc(src []byte) error {
	if isMultiDocumentYAML(src) {
		return fmt.Errorf("Multistream YAML not supported yet (see https://github.com/bitnami/sealed-secrets/issues/114)")
	}
	return nil
}
