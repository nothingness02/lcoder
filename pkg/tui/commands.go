package tui

import (
	"fmt"
	"strings"
)

// SlashCommand is a parsed leading slash command (name + remaining args).
type SlashCommand struct {
	Name string
	Args string
}

// parseSlashCommand parses a leading slash command from text. Relocated from the
// old slash_commands.go during the Phase 10 model rewrite.
func parseSlashCommand(text string) (SlashCommand, bool) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/") {
		return SlashCommand{}, false
	}
	body := strings.TrimSpace(strings.TrimPrefix(text, "/"))
	parts := strings.SplitN(body, " ", 2)
	cmd := SlashCommand{Name: strings.ToLower(parts[0])}
	if len(parts) > 1 {
		cmd.Args = strings.TrimSpace(parts[1])
	}
	return cmd, true
}

// parseModeCommand parses a /mode command and returns the requested mode name.
// Relocated from the old mode_commands.go during the Phase 10 model rewrite.
func parseModeCommand(text string) (string, bool) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "/mode ") {
		return strings.TrimSpace(strings.TrimPrefix(text, "/mode ")), true
	}
	if text == "/mode" {
		return "", true
	}
	return "", false
}

// matches reports whether e matches name as primary or alias.
func (e commandEntry) matches(name string) bool {
	if e.Name == name {
		return true
	}
	for _, a := range e.Aliases {
		if a == name {
			return true
		}
	}
	return false
}

// findCommand resolves a name (primary or alias) against the registry.
func findCommand(name string) (commandEntry, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, e := range commandRegistry {
		if e.matches(name) {
			return e, true
		}
	}
	return commandEntry{}, false
}

// formatCommandHelp renders the command palette grouped by category.
func formatCommandHelp() string {
	byCategory := map[string][]commandEntry{}
	var categories []string
	for _, c := range commandRegistry {
		if _, ok := byCategory[c.Category]; !ok {
			categories = append(categories, c.Category)
		}
		byCategory[c.Category] = append(byCategory[c.Category], c)
	}
	var lines []string
	for _, cat := range categories {
		lines = append(lines, cat+":")
		for _, c := range byCategory[cat] {
			line := fmt.Sprintf("  /%-12s %s", c.Name, c.Description)
			if len(c.Aliases) > 0 {
				al := make([]string, len(c.Aliases))
				for i, a := range c.Aliases {
					al[i] = "/" + a
				}
				line += fmt.Sprintf(" (%s)", strings.Join(al, ", "))
			}
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}
