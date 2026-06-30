package sandbox

import (
	"slices"
	"testing"
)

func TestScrubEnvAllowlist(t *testing.T) {
	env := []string{"PATH=/usr/bin", "AWS_SECRET=top", "HOME=/home/u", "no_equals_entry"}
	got := scrubEnvFold(env, []string{"PATH", "HOME"}, false)
	want := []string{"PATH=/usr/bin", "HOME=/home/u"}
	if !slices.Equal(got, want) {
		t.Fatalf("scrubEnvFold = %v, want %v", got, want)
	}
}

func TestScrubEnvCaseFold(t *testing.T) {
	// Windows stores "Path"; folding must keep it when allowlist has "PATH".
	env := []string{"Path=C:\\Windows", "SECRET_TOKEN=x"}

	folded := scrubEnvFold(env, []string{"PATH"}, true)
	if !slices.Equal(folded, []string{"Path=C:\\Windows"}) {
		t.Fatalf("folded keep Path: got %v", folded)
	}

	strict := scrubEnvFold(env, []string{"PATH"}, false)
	if len(strict) != 0 {
		t.Fatalf("strict should drop Path (case-sensitive miss): got %v", strict)
	}
}
