package tui

import (
	"strings"
	"testing"
)

func TestFindCommandByAlias(t *testing.T) {
	e, ok := findCommand("?")
	if !ok || e.Name != "help" {
		t.Fatalf("findCommand(?) = %v, %v", e.Name, ok)
	}
	e, ok = findCommand("resume")
	if !ok || e.Name != "sessions" {
		t.Fatalf("findCommand(resume) = %v, %v", e.Name, ok)
	}
}

func TestFindCommandUnknown(t *testing.T) {
	if _, ok := findCommand("nope"); ok {
		t.Fatal("unknown command matched")
	}
}

func TestFormatCommandHelpGrouped(t *testing.T) {
	out := formatCommandHelp()
	if !strings.Contains(out, "System") || !strings.Contains(out, "/help") {
		t.Fatalf("help missing category/command: %q", out)
	}
}
