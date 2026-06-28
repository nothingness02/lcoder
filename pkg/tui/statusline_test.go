package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestStatusLineFillsWidth(t *testing.T) {
	out := statusLine(40, "▌ build · kimi-k2", "? for commands")
	if lipgloss.Width(out) != 40 {
		t.Fatalf("status line width = %d, want 40", lipgloss.Width(out))
	}
}

func TestStatusLineTruncatesOverflow(t *testing.T) {
	left := "▌ verylongmodename-that-overflows-the-bar"
	out := statusLine(20, left, "right")
	if lipgloss.Width(out) > 20 {
		t.Fatalf("status line width = %d, want <= 20", lipgloss.Width(out))
	}
}
