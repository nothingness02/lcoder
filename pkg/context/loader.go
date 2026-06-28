package context

import (
	"os"
	"path/filepath"
	"strings"
)

// Loader discovers and reads AGENTS.md / CLAUDE.md context files.
type Loader struct {
	cwd string
}

// NewLoader creates a context loader.
func NewLoader(cwd string) *Loader {
	return &Loader{cwd: cwd}
}

// Load returns concatenated context file contents from cwd up to root.
func (l *Loader) Load() (string, error) {
	var contents []string
	current := l.cwd

	for {
		for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
			path := filepath.Join(current, name)
			if data, err := os.ReadFile(path); err == nil {
				contents = append(contents, string(data))
			}
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return strings.Join(contents, "\n\n"), nil
}
