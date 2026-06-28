package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// DiffLine represents a single line in a diff.
type DiffLine struct {
	Kind string // add | remove | context
	Text string
}

// ParseDiff parses a unified-diff-like text into diff lines.
func ParseDiff(text string) []DiffLine {
	var lines []DiffLine
	for _, raw := range strings.Split(text, "\n") {
		if len(raw) == 0 {
			continue
		}
		switch raw[0] {
		case '+':
			lines = append(lines, DiffLine{Kind: "add", Text: raw})
		case '-':
			lines = append(lines, DiffLine{Kind: "remove", Text: raw})
		case '@':
			lines = append(lines, DiffLine{Kind: "header", Text: raw})
		default:
			lines = append(lines, DiffLine{Kind: "context", Text: raw})
		}
	}
	return lines
}

// RenderDiff renders diff lines with color coding.
func RenderDiff(lines []DiffLine, width int) string {
	if len(lines) == 0 {
		return styleDim().Render("No diff to display.")
	}

	addStyle := styleSuccess()
	removeStyle := styleError()
	headerStyle := styleAccent()
	contextStyle := styleSecondary()

	var out []string
	for _, line := range lines {
		text := truncate(line.Text, width-4)
		switch line.Kind {
		case "add":
			out = append(out, addStyle.Render(text))
		case "remove":
			out = append(out, removeStyle.Render(text))
		case "header":
			out = append(out, headerStyle.Render(text))
		default:
			out = append(out, contextStyle.Render(text))
		}
	}

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorFaint).
		Padding(0, 1).
		Width(width)
	return border.Render(strings.Join(out, "\n"))
}
