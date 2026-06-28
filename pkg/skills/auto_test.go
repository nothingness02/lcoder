package skills

import (
	"testing"
)

func TestAutoDetect(t *testing.T) {
	skills := []Skill{
		{
			Name:      "security-review",
			WhenToUse: "Review code for security vulnerabilities",
			Steps:     []string{"Read the file", "Identify risks"},
			Examples:  []string{"Review auth.ts for security issues"},
		},
		{
			Name:      "refactor",
			WhenToUse: "Refactor code following project conventions",
			Steps:     []string{"Analyze code", "Apply improvements"},
			Examples:  []string{"Refactor this function"},
		},
	}

	score, ok := AutoDetect("Please review auth.go for security issues", skills)
	if !ok {
		t.Fatal("expected a skill match")
	}
	if score.Skill.Name != "security-review" {
		t.Fatalf("expected security-review, got %s", score.Skill.Name)
	}
}

func TestAutoDetectNoMatch(t *testing.T) {
	skills := []Skill{
		{
			Name:      "security-review",
			WhenToUse: "Review code for security vulnerabilities",
		},
	}

	_, ok := AutoDetect("What is the weather today?", skills)
	if ok {
		t.Fatal("expected no skill match")
	}
}
