package builtin

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/lcoder/lcoder/pkg/sandbox"
)

// denyFS is a FilesystemPolicy that rejects every path.
type denyFS struct{}

func (denyFS) Check(path string, op sandbox.FSOp) error {
	return fmt.Errorf("denied: %s", path)
}
func (denyFS) SubprocessMounts() []sandbox.Mount { return nil }

func denyingSandbox() *sandbox.FakeSandbox {
	f := sandbox.NewFakeSandbox()
	f.FSPolicy = denyFS{}
	return f
}

func TestResolveAndCheckAllowed(t *testing.T) {
	got, err := resolveAndCheck("/proj", sandbox.NewFakeSandbox(), "x.txt", sandbox.FSRead)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Clean("/proj/x.txt")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveAndCheckDenied(t *testing.T) {
	_, err := resolveAndCheck("/proj", denyingSandbox(), "x.txt", sandbox.FSWrite)
	if err == nil {
		t.Fatal("expected denial error")
	}
}

func TestResolveAndCheckNilSandbox(t *testing.T) {
	abs := filepath.Join(string(filepath.Separator), "abs", "x.txt")
	if vol := filepath.VolumeName("C:\\"); vol != "" {
		abs = "C:" + abs // ensure absolute on Windows
	}
	got, err := resolveAndCheck("/proj", nil, abs, sandbox.FSRead)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != filepath.Clean(abs) {
		t.Fatalf("got %q", got)
	}
}

func TestReadDeniedBySandbox(t *testing.T) {
	r := NewRead("/project").(*Read)
	r.UseSandbox(denyingSandbox())
	if _, err := r.Execute(context.Background(), "c", map[string]any{"path": "secret.txt"}); err == nil {
		t.Fatal("expected denial error from read")
	}
}

func TestWriteDeniedBySandbox(t *testing.T) {
	w := NewWrite("/project").(*Write)
	w.UseSandbox(denyingSandbox())
	if _, err := w.Execute(context.Background(), "c", map[string]any{"path": "out.txt", "content": "x"}); err == nil {
		t.Fatal("expected denial error from write")
	}
}
