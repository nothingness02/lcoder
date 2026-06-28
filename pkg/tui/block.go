package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type blockKind int

const (
	blockUser blockKind = iota
	blockAssistant
	blockTool
	blockSystem
)

// block is one rendered unit of conversation history.
type block struct {
	kind blockKind
	id   string // message ID or tool-call ID (for in-place updates)
	raw  string // user text / assistant markdown / tool result content

	// user extras
	attachments []string // @file mention basenames shown under the bar

	// assistant extras
	thinking string
	usage    *blockUsage

	// tool extras
	toolName string
	toolArgs string
	toolErr  bool
	elapsed  time.Duration
}

type blockUsage struct {
	inputTokens  int
	outputTokens int
	totalTokens  int
	cost         float64
}

// render returns the styled string for this block at the given width. expanded
// only affects tool blocks (Ctrl+O view).
func (b block) render(width int, expanded bool) string {
	switch b.kind {
	case blockUser:
		bar := lipgloss.NewStyle().
			Background(colorUserBar).
			Foreground(colorSecondary).
			Width(width).
			Padding(0, 1)
		var sb strings.Builder
		sb.WriteString(bar.Render("› " + b.raw))
		if len(b.attachments) > 0 {
			sb.WriteString("\n")
			seg := "↳ " + strings.Join(b.attachments, ", ")
			sb.WriteString(styleDim().Render(seg))
		}
		return sb.String()
	case blockAssistant:
		var sb strings.Builder
		if b.thinking != "" {
			sb.WriteString(styleDim().Italic(true).Render("🧠 " + truncate(b.thinking, 200)))
			sb.WriteString("\n\n")
		}
		sb.WriteString(renderMarkdownCached(b.raw, width))
		if b.usage != nil {
			sb.WriteString("\n")
			sb.WriteString(styleDim().Render(fmt.Sprintf(" · %d tokens · $%.4f", b.usage.totalTokens, b.usage.cost)))
		}
		return sb.String()
	case blockTool:
		if expanded {
			return formatExpandedToolResult(b.toolName, b.toolArgs, b.toolErr, b.raw, b.elapsed)
		}
		return formatCompactToolResult(b.toolName, b.toolArgs, b.toolErr, b.raw, b.elapsed)
	default: // blockSystem
		return styleDim().Italic(true).Render(b.raw)
	}
}
