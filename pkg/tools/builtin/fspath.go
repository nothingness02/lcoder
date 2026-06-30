package builtin

import (
	"path/filepath"

	"github.com/lcoder/lcoder/pkg/sandbox"
)

// resolveAndCheck resolves rawPath against cwd (absolutizing + cleaning) and, if
// a sandbox is present, enforces sb.Filesystem().Check for the given op. It
// returns the cleaned absolute path or the policy error.
func resolveAndCheck(cwd string, sb sandbox.Sandbox, rawPath string, op sandbox.FSOp) (string, error) {
	path := rawPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}
	path = filepath.Clean(path)
	if sb != nil {
		if err := sb.Filesystem().Check(path, op); err != nil {
			return "", err
		}
	}
	return path, nil
}
