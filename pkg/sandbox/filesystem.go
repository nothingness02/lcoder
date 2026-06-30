package sandbox

import (
	"fmt"
	"path/filepath"
	"strings"
)

// allowAllFS permits every path (Passthrough backend).
type allowAllFS struct{}

func (allowAllFS) Check(string, FSOp) error  { return nil }
func (allowAllFS) SubprocessMounts() []Mount { return nil }

// restrictedFS confines in-process file access to configured roots. Roots must
// already be normalized via resolvePath at construction time.
type restrictedFS struct {
	readable []string
	writable []string
}

func (f *restrictedFS) Check(path string, op FSOp) error {
	real, err := resolvePath(path)
	if err != nil {
		return fmt.Errorf("sandbox: cannot resolve %q: %w", path, err)
	}
	roots := f.readable
	label := "read"
	if op == FSWrite {
		roots = f.writable
		label = "write"
	}
	if !withinAny(real, roots) {
		return fmt.Errorf("sandbox: %s access to %q denied by policy", label, path)
	}
	return nil
}

func (f *restrictedFS) SubprocessMounts() []Mount { return nil }

// resolvePath normalizes a path to its real physical form (separator-folded) for
// prefix matching: Abs -> Clean -> EvalSymlinks. For a not-yet-existing target
// (e.g. a write destination) it resolves the parent dir then re-attaches the
// basename, so a symlinked parent cannot escape the root.
func resolvePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		return normalizeSep(real), nil
	}
	parent := filepath.Dir(abs)
	realParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return normalizeSep(abs), nil // parent absent too; best we can do
	}
	return normalizeSep(filepath.Join(realParent, filepath.Base(abs))), nil
}

// normalizeSep folds OS separators to "/" so matching is separator-agnostic
// (matters on Windows where "\\" and "/" both appear).
func normalizeSep(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}

// withinAny reports whether target equals or is nested under any root. Prefix
// checks are segment-aligned so "/projevil" does not match root "/proj".
func withinAny(target string, roots []string) bool {
	for _, root := range roots {
		if target == root {
			return true
		}
		prefix := root
		if !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
		if strings.HasPrefix(target, prefix) {
			return true
		}
	}
	return false
}
