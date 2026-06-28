package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// statusLine composes a full-width bar: left caption, a dim ─ filler, right
// caption. If left+right overflow width, the left is truncated (cells-safe) and
// at least one filler dash is kept.
func statusLine(width int, left, right string) string {
	if width <= 0 {
		return ""
	}
	dim := styleDim()
	lw := lipgloss.Width(left)
	rw := lipgloss.Width(right)

	if lw+rw+1 > width {
		// Truncate left to fit, drop right if still too tight.
		budget := width - rw - 1
		if budget < 1 {
			return dim.Render(truncateCellsSafe(stripANSI(left), width))
		}
		left = truncateCellsSafe(stripANSI(left), budget)
		lw = lipgloss.Width(left)
	}
	fill := width - lw - rw
	if fill < 1 {
		fill = 1
	}
	return left + dim.Render(strings.Repeat("─", fill)) + right
}

// stripANSI removes ANSI escapes so width math on plain text is exact.
func stripANSI(s string) string {
	var sb strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}
