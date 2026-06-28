package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestActiveMention(t *testing.T) {
	cases := []struct {
		val     string
		partial string
		ok      bool
	}{
		{"see @ma", "ma", true},
		{"@", "", true},
		{"foo@bar", "", false},    // @ not preceded by whitespace
		{"@done file", "", false}, // whitespace after the mention closes it
		{"no mention", "", false},
		{"a @b c @d", "d", true}, // last @ word
	}
	for _, c := range cases {
		partial, ok := activeMention(c.val)
		if ok != c.ok || partial != c.partial {
			t.Errorf("activeMention(%q) = %q,%v want %q,%v", c.val, partial, ok, c.partial, c.ok)
		}
	}
}

func TestFileMatches(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "main.go"), "x")
	mustWrite(t, filepath.Join(dir, "pkg", "loop.go"), "x")
	mustWrite(t, filepath.Join(dir, ".git", "config"), "x")
	mustWrite(t, filepath.Join(dir, "node_modules", "dep.js"), "x")

	all := fileMatches(dir, "")
	for _, f := range all {
		if f == ".git/config" || f == "node_modules/dep.js" {
			t.Fatalf("fileMatches included skipped dir entry: %q (all=%v)", f, all)
		}
	}

	got := fileMatches(dir, "loop")
	if !reflect.DeepEqual(got, []string{"pkg/loop.go"}) {
		t.Fatalf("fileMatches(loop) = %v", got)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
