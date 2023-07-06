package buildinfo

import "runtime/debug"

// DefaultVersion is the default version string if it's unset.
const DefaultVersion = "UNKNOWN"

// FallbackVersion initializes the automatic version detection.
func FallbackVersion(v *string, unchanged string) {
	if *v != unchanged {
		return
	}
	b, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	if modv := b.Main.Version; modv != "(devel)" {
		*v = modv
	}
}
