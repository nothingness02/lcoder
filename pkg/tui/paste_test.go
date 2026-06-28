package tui

import (
	"strings"
	"testing"
)

func TestPasteStashRoundTrip(t *testing.T) {
	p := newPasteStash()
	big := strings.Repeat("x", 1500)
	placeholder := p.stash(big)
	if !strings.HasPrefix(placeholder, "[Pasted #1") {
		t.Fatalf("placeholder = %q", placeholder)
	}
	expanded := p.expand("before " + placeholder + " after")
	if !strings.Contains(expanded, big) {
		t.Fatal("expand did not restore original text")
	}
}

func TestPasteSmallNotStashed(t *testing.T) {
	p := newPasteStash()
	if p.shouldStash("short") {
		t.Fatal("short text should not stash")
	}
}
