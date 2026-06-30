package sandbox

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func newRestrictedFS(t *testing.T, root string) *restrictedFS {
	t.Helper()
	real, err := resolvePath(root)
	if err != nil {
		t.Fatalf("resolve root: %v", err)
	}
	return &restrictedFS{readable: []string{real}, writable: []string{real}}
}

func TestRestrictedFSAllowsInsideRoot(t *testing.T) {
	root := t.TempDir()
	fs := newRestrictedFS(t, root)
	if err := fs.Check(filepath.Join(root, "sub", "file.txt"), FSWrite); err != nil {
		t.Fatalf("expected allow inside root, got %v", err)
	}
}

func TestRestrictedFSDeniesTraversal(t *testing.T) {
	root := t.TempDir()
	fs := newRestrictedFS(t, root)
	escape := filepath.Join(root, "..", "..", "etc", "passwd")
	if err := fs.Check(escape, FSRead); err == nil {
		t.Fatal("expected traversal to be denied")
	}
}

func TestRestrictedFSDeniesSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is restricted on Windows CI")
	}
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	fs := newRestrictedFS(t, root)
	// link/secret.txt resolves into `outside`, which is outside the root.
	if err := fs.Check(filepath.Join(link, "secret.txt"), FSRead); err == nil {
		t.Fatal("expected symlink escape to be denied")
	}
}

func TestRestrictedFSPrefixIsSegmentAligned(t *testing.T) {
	// A sibling dir sharing a name prefix must not match (e.g. /p vs /pevil).
	parent := t.TempDir()
	root := filepath.Join(parent, "proj")
	sibling := filepath.Join(parent, "projevil")
	_ = os.MkdirAll(root, 0o755)
	_ = os.MkdirAll(sibling, 0o755)
	fs := newRestrictedFS(t, root)
	if err := fs.Check(filepath.Join(sibling, "x.txt"), FSRead); err == nil {
		t.Fatal("expected sibling prefix to be denied")
	}
}

func TestAllowAllFS(t *testing.T) {
	var fs allowAllFS
	if err := fs.Check("/anything", FSWrite); err != nil {
		t.Fatalf("allowAllFS should permit, got %v", err)
	}
	if fs.SubprocessMounts() != nil {
		t.Fatal("expected nil mounts")
	}
}
