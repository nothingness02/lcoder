package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mkGitRoot marks dir as a git repository root by creating a .git directory.
func mkGitRoot(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// TestLoadGitRootBeforeProject verifies context files are concatenated from the
// git repository root down to the project directory (cwd), so repo-root context
// comes first and project-local context last.
func TestLoadGitRootBeforeProject(t *testing.T) {
	repo := t.TempDir()
	mkGitRoot(t, repo)
	proj := filepath.Join(repo, "sub", "proj")

	writeFile(t, repo, "CLAUDE.md", "ROOT-CONTEXT")
	writeFile(t, proj, "CLAUDE.md", "PROJECT-CONTEXT")

	out, err := NewLoader(proj).Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	rootIdx := strings.Index(out, "ROOT-CONTEXT")
	projIdx := strings.Index(out, "PROJECT-CONTEXT")
	if rootIdx == -1 || projIdx == -1 {
		t.Fatalf("expected both contexts present, got: %q", out)
	}
	if rootIdx > projIdx {
		t.Fatalf("expected git-root context before project context, got root at %d, project at %d:\n%s", rootIdx, projIdx, out)
	}
}

// TestLoadStopsAtGitRoot verifies the loader does not scan directories above the
// git repository root.
func TestLoadStopsAtGitRoot(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "repo")
	mkGitRoot(t, repo)
	proj := filepath.Join(repo, "proj")

	writeFile(t, base, "CLAUDE.md", "OUTSIDE-CONTEXT") // above the repo root
	writeFile(t, repo, "CLAUDE.md", "ROOT-CONTEXT")
	writeFile(t, proj, "CLAUDE.md", "PROJECT-CONTEXT")

	out, err := NewLoader(proj).Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if strings.Contains(out, "OUTSIDE-CONTEXT") {
		t.Fatalf("loader must not scan above the git root, but got:\n%s", out)
	}
	if !strings.Contains(out, "ROOT-CONTEXT") || !strings.Contains(out, "PROJECT-CONTEXT") {
		t.Fatalf("expected repo-root and project contexts, got:\n%s", out)
	}
}
