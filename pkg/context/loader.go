package context

import (
	"os"
	"path/filepath"
	"strings"
)

// contextFileNames are the context file basenames searched in each directory,
// in priority order within a directory.
var contextFileNames = []string{"AGENTS.md", "CLAUDE.md", "LCODER.md"}

// Loader discovers and reads AGENTS.md / CLAUDE.md / LCODER.md context files.
type Loader struct {
	cwd string
}

// NewLoader creates a context loader.
func NewLoader(cwd string) *Loader {
	return &Loader{cwd: cwd}
}

// Load returns concatenated context file contents from the git repository root
// down to cwd. Repository-root context comes first and project-local context
// last, so the more specific (deeper) context can override the broader one.
// Scanning stops at the git repository root and never ascends above it.
func (l *Loader) Load() (string, error) {
	var contents []string
	for _, dir := range l.dirChain() {
		for _, name := range contextFileNames {
			path := filepath.Join(dir, name)
			if data, err := os.ReadFile(path); err == nil {
				contents = append(contents, string(data))
			}
		}
	}
	return strings.Join(contents, "\n\n"), nil
}

// dirChain returns the directories to scan, ordered from the git repository
// root down to cwd. If no git root is found at or above cwd, only cwd is
// returned so the loader never scans the entire filesystem.
func (l *Loader) dirChain() []string {
	var chain []string
	current := l.cwd
	for {
		chain = append(chain, current)
		if isGitRoot(current) {
			// Include the git root, then stop ascending.
			break
		}
		parent := filepath.Dir(current)
		if parent == current {
			// Reached the filesystem root without finding a git root: scan
			// only cwd rather than the whole filesystem.
			return []string{l.cwd}
		}
		current = parent
	}
	// chain is ordered cwd-first; reverse to root-first.
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}

// isGitRoot reports whether dir contains a .git entry (directory for a normal
// clone, or a file for worktrees and submodules).
func isGitRoot(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}
