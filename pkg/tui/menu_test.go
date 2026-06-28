package tui

import (
	"strings"
	"testing"
)

func TestMenuExactPrefixFirst(t *testing.T) {
	matches := menuMatches("he")
	if len(matches) == 0 {
		t.Fatal("no matches for 'he'")
	}
	if matches[0].entry.Name != "help" {
		t.Fatalf("first match = %q, want help", matches[0].entry.Name)
	}
}

func TestMenuFuzzy(t *testing.T) {
	matches := menuMatches("sesn")
	found := false
	for _, m := range matches {
		if m.entry.Name == "sessions" {
			found = true
		}
	}
	if !found {
		t.Fatal("fuzzy did not match 'sessions' for 'sesn'")
	}
}

func TestMenuRenderHighlights(t *testing.T) {
	matches := menuMatches("hel")
	out := renderMenu(matches, 0, 40)
	if !strings.Contains(stripANSI(out), "help") {
		t.Fatal("menu render missing help")
	}
}

func TestMenuEmptyQueryListsAll(t *testing.T) {
	if len(menuMatches("")) != len(commandRegistry) {
		t.Fatal("empty query should list all commands")
	}
}
