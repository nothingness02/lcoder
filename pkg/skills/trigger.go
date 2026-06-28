package skills

import (
	"fmt"
	"strings"

	"github.com/lcoder/lcoder/pkg/models"
)

// FindByName looks up a skill by name in a loaded skill list.
func FindByName(skills []Skill, name string) (Skill, bool) {
	for _, s := range skills {
		if s.Name == name {
			return s, true
		}
	}
	return Skill{}, false
}

// ParseManualTrigger checks if text is a manual skill trigger like "/skill:name".
// It returns the skill name and the remaining user text (if any).
func ParseManualTrigger(text string) (name string, rest string, ok bool) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/skill:") {
		return "", text, false
	}
	after := strings.TrimPrefix(text, "/skill:")
	parts := strings.SplitN(after, " ", 2)
	name = strings.TrimSpace(parts[0])
	if name == "" {
		return "", text, false
	}
	if len(parts) > 1 {
		rest = strings.TrimSpace(parts[1])
	}
	return name, rest, true
}

// ExpandManualTrigger returns system/user messages that activate a skill.
func ExpandManualTrigger(skill Skill, originalText string) []models.AgentMessage {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("You are now using the %s skill.\n\n", skill.Name))
	if skill.WhenToUse != "" {
		b.WriteString("Purpose: ")
		b.WriteString(skill.WhenToUse)
		b.WriteString("\n\n")
	}
	if len(skill.Steps) > 0 {
		b.WriteString("Steps:\n")
		for _, step := range skill.Steps {
			b.WriteString("- ")
			b.WriteString(step)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	if skill.OutputFormat != "" {
		b.WriteString("Output format: ")
		b.WriteString(skill.OutputFormat)
		b.WriteString("\n\n")
	}
	b.WriteString("Follow the above instructions for the user's request.")

	system := models.NewAgentMessage(models.RoleSystem, models.TextContent{Text: b.String()})
	user := models.NewAgentMessage(models.RoleUser, models.TextContent{Text: originalText})
	return []models.AgentMessage{system, user}
}
