package multidocyaml

import "testing"

func TestIsMultiDocumentYAML(t *testing.T) {
	testCases := []struct {
		src string
		ok  bool
	}{
		{"foo", false},
		{"foo\nbar\n", false},
		{"---\nfoo", false},
		{"foo\n---\n", true},
		{"foo\n ---\n", false},
		{"---\nfoo\n---\n", true},
		{"", false},
		// Malformed YAML must not panic (see #857).
		{"a: b: c\n", false},
		{"key:\n\t- x\n", false},
		{"foo: [\n", false},
		{"a: 1\n  b: 2\n", false},
		{"\"unterminated\n", false},
		{"{a: 1\n---\nb: 2\n", false},
	}

	for _, tc := range testCases {
		if got, want := isMultiDocumentYAML([]byte(tc.src)), tc.ok; got != want {
			t.Errorf("got: %v, want: %v (src: %q)", got, want, tc.src)
		}
	}
}

func TestEnsureNotMultiDocMalformedYAML(t *testing.T) {
	if err := EnsureNotMultiDoc([]byte("a: b: c\n")); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
