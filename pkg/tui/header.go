package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// headerInfo carries the right-column metadata for the startup header.
type headerInfo struct {
	model   string
	cwd     string
	version string
}

// renderHeader composes the rounded accent box: logo (left, drawn to frame),
// metadata (right). width bounds the box.
func renderHeader(h headerInfo, frame, width int) string {
	logo := logoFrame(frame)

	meta := lipgloss.JoinVertical(lipgloss.Left,
		styleAccent().Bold(true).Render("Lcoder CLI ")+styleDim().Render("v"+h.version),
		styleDim().Render("model ")+h.model,
		styleDim().Render("cwd   ")+h.cwd,
		styleDim().Render("? for commands"),
	)

	body := lipgloss.JoinHorizontal(lipgloss.Top, logo, "  ", meta)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorAccent).
		Padding(0, 1)
	if width > 4 {
		box = box.MaxWidth(width)
	}
	return box.Render(body)
}
