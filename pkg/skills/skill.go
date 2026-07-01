package skills

import (
	"fmt"
	"strings"
)

// Skill is a reusable capability package described in Markdown.
type Skill struct {
	Name         string
	WhenToUse    string
	Steps        []string
	Examples     []string
	OutputFormat string
	Source       string
}

// ToSystemPromptBlock renders a list of skills for the system prompt.
func ToSystemPromptBlock(skills []Skill) string {
	if len(skills) == 0 {
		return ""
	}
	var b stringsBuilder
	b.Write("You have access to the following skills. Use them when appropriate:\n\n")
	for _, s := range skills {
		b.Writef("- %s: %s\n", s.Name, s.WhenToUse)
		for _, step := range s.Steps {
			b.Writef("  - Step: %s\n", step)
		}
		for _, ex := range s.Examples {
			b.Writef("  - Example: %s\n", ex)
		}
		if s.OutputFormat != "" {
			b.Writef("  - Output format: %s\n", s.OutputFormat)
		}
	}
	return b.String()
}

type stringsBuilder struct {
	parts []string
}

func (sb *stringsBuilder) Write(s string) {
	sb.parts = append(sb.parts, s)
}

func (sb *stringsBuilder) Writef(format string, args ...any) {
	sb.parts = append(sb.parts, fmt.Sprintf(format, args...))
}

func (sb *stringsBuilder) String() string {
	return strings.Join(sb.parts, "")
}
