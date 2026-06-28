package tui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
)

// fileMenuMax caps how many file suggestions the @-picker shows at once.
const fileMenuMax = 10

// activeMention returns the partial path being typed after a trailing '@' word
// (the '@' preceded by start-of-input or whitespace, with no whitespace after
// it). It reports false when no such in-progress mention exists.
func activeMention(val string) (string, bool) {
	at := strings.LastIndex(val, "@")
	if at < 0 {
		return "", false
	}
	if at > 0 && !isMentionSpace(val[at-1]) {
		return "", false
	}
	partial := val[at+1:]
	if strings.ContainsAny(partial, " \t\n") {
		return "", false
	}
	return partial, true
}

// fileMatches lists up to fileMenuMax cwd-relative file paths fuzzy-matching
// partial. It skips .git, node_modules, and hidden directories.
func fileMatches(cwd, partial string) []string {
	var files []string
	_ = filepath.WalkDir(cwd, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path == cwd {
				return nil
			}
			name := d.Name()
			if name == ".git" || name == "node_modules" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(cwd, path)
		if err != nil {
			return nil
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})

	if partial == "" {
		if len(files) > fileMenuMax {
			files = files[:fileMenuMax]
		}
		return files
	}
	var out []string
	for _, m := range fuzzy.Find(partial, files) {
		out = append(out, files[m.Index])
		if len(out) >= fileMenuMax {
			break
		}
	}
	return out
}

// renderFileMenu draws the @-file suggestion dropdown.
func renderFileMenu(matches []string, selected, width int) string {
	if len(matches) == 0 {
		return ""
	}
	var lines []string
	for i, f := range matches {
		row := "@" + f
		if i == selected {
			row = lipgloss.NewStyle().Foreground(colorSelect).Render("› ") + row
		} else {
			row = "  " + row
		}
		lines = append(lines, truncateCells(row, width, "…"))
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorFaint)
	return box.Render(strings.Join(lines, "\n"))
}
