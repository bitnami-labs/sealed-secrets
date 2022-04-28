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
	}

	for _, tc := range testCases {
		if got, want := isMultiDocumentYAML([]byte(tc.src)), tc.ok; got != want {
			t.Errorf("got: %v, want: %v (src: %q)", got, want, tc.src)
		}
	}
}
