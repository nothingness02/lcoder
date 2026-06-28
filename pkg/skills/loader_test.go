package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSkill(t *testing.T) {
	dir, err := os.MkdirTemp("", "lcoder-skills-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	skillDir := filepath.Join(dir, "security-review")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `---
name: security-review
when_to_use: Review code for security vulnerabilities
steps:
  - Read the file
  - Identify risks
examples:
  - "Review auth.ts"
output_format: Markdown list
---
# Security Review

Do a security review.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load([]string{dir})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(loaded))
	}
	s := loaded[0]
	if s.Name != "security-review" {
		t.Fatalf("expected security-review, got %s", s.Name)
	}
	if s.WhenToUse != "Review code for security vulnerabilities" {
		t.Fatalf("unexpected when_to_use: %s", s.WhenToUse)
	}
	if len(s.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(s.Steps))
	}
}
