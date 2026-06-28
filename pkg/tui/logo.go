package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// logoArt is the full pixel/half-block Lcoder mark (4 rows). Each row is the
// same rune-length so the draw-in reveal is uniform.
var logoArt = []string{
	"█▀▀▀▀▀▀█",
	"█ ▀▶_   █",
	"█      █",
	"█▄▄▄▄▄▄█",
}

const (
	logoHeight   = 4
	logoRuneCols = 9  // max runes per row (drives the column reveal)
	logoWidth    = 18 // max rendered cell width (block runes are 2 cells wide)
	logoFrames   = 8  // number of draw-in steps to full reveal
)

// gradientColors approximate a cyan diagonal sweep, brightest top-left.
var gradientColors = []string{"#8FBCBB", "#88C0D0", "#81A1C1", "#5E81AC"}

// logoFrame returns the logo revealed up to step n (0=hidden, logoFrames=full).
// Hidden columns are rendered as spaces so every frame keeps logoHeight lines.
func logoFrame(n int) string {
	if n > logoFrames {
		n = logoFrames
	}
	reveal := n * logoRuneCols / logoFrames // rune columns shown this frame
	var rows []string
	for i, row := range logoArt {
		runes := []rune(row)
		shown := make([]rune, len(runes))
		for j, r := range runes {
			if j < reveal {
				shown[j] = r
			} else {
				shown[j] = ' '
			}
		}
		color := gradientColors[i%len(gradientColors)]
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
		rows = append(rows, style.Render(string(shown)))
	}
	return strings.Join(rows, "\n")
}
