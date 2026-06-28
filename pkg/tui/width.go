package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// displayWidth returns the terminal cell width of PLAIN text (no ANSI escapes).
// For already-styled strings use lipgloss.Width, which strips escapes first.
func displayWidth(s string) int {
	return runewidth.StringWidth(s)
}

// truncateCells truncates s so its display width is at most maxCells, appending
// tail (whose width is included in the budget) when truncation occurs. A
// double-width rune is never split across the boundary.
func truncateCells(s string, maxCells int, tail string) string {
	if maxCells <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= maxCells {
		return s
	}
	return runewidth.Truncate(s, maxCells, tail)
}

// truncateCellsSafe truncates pessimistically: every non-ASCII rune is budgeted
// as 2 cells, so the result can never wrap even when runewidth would undercount.
// Reserve for free-form text in the animated live region; ASCII box-drawing must
// not use it.
func truncateCellsSafe(s string, maxCells int) string {
	if maxCells <= 0 {
		return ""
	}
	used := 0
	for i, r := range s {
		w := 1
		if r >= 0x80 {
			w = 2
		}
		if used+w > maxCells {
			return s[:i]
		}
		used += w
	}
	return s
}

// truncate clips s to width terminal cells, appending an ellipsis when it does
// not fit. (Relocated from chat.go; byte-based, used by legacy call paths.)
func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	w := lipgloss.Width(s)
	if w <= width {
		return s
	}
	return s[:width-1] + "…"
}
