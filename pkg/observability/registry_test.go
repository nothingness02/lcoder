package observability

import (
	"testing"
)

func TestDefaultRegistryHasBuiltins(t *testing.T) {
	r := DefaultRegistry()
	names := r.Names()
	for _, name := range []string{"file", "sqlite", "html", "prometheus"} {
		if !contains(names, name) {
			t.Fatalf("missing exporter %s", name)
		}
	}
}

func contains(names []string, target string) bool {
	for _, n := range names {
		if n == target {
			return true
		}
	}
	return false
}
