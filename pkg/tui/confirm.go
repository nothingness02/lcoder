package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lcoder/lcoder/pkg/agent"
)

// confirmResult is returned to the blocked tool call goroutine.
type confirmResult struct {
	allow bool
	err   error
}

// confirmRequest carries a pending confirmation into the Bubble Tea loop.
type confirmRequest struct {
	info agent.ToolCallInfo
	resp chan confirmResult
}

// confirmRequestMsg asks the UI to show a permission prompt.
type confirmRequestMsg struct {
	req confirmRequest
}

// confirmResponseMsg carries the user's y/n decision back into the loop.
type confirmResponseMsg struct {
	allow bool
}

// programSender matches the part of *tea.Program that tuiConfirm needs.
type programSender interface {
	Send(tea.Msg)
}

// tuiConfirm implements agent.UserConfirmation by delegating to the Bubble Tea
// event loop. It blocks the tool-call goroutine until the user responds.
type tuiConfirm struct {
	program programSender
}

func (c *tuiConfirm) Confirm(ctx context.Context, info agent.ToolCallInfo) (bool, error) {
	req := confirmRequest{info: info, resp: make(chan confirmResult)}
	c.program.Send(confirmRequestMsg{req: req})
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case r := <-req.resp:
		return r.allow, r.err
	}
}

// confirmPanel renders an interactive permission prompt.
type confirmPanel struct {
	visible bool
	info    agent.ToolCallInfo
	resp    chan confirmResult
}

func (p *confirmPanel) show(info agent.ToolCallInfo, resp chan confirmResult) {
	p.visible = true
	p.info = info
	p.resp = resp
}

func (p *confirmPanel) hide() {
	p.visible = false
	p.info = agent.ToolCallInfo{}
	p.resp = nil
}

func (p *confirmPanel) View(width, height int) string {
	if !p.visible {
		return ""
	}
	prompt := fmt.Sprintf("Permission request: %s", p.info.ToolCall.Name)
	if args := formatArgs(p.info.Args); args != "" {
		prompt += fmt.Sprintf("(%s)", args)
	}
	prompt += "\n\nAllow? [y/N]"

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorError).
		Padding(1, 2).
		Width(60)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box.Render(prompt))
}

func formatArgs(args map[string]any) string {
	if len(args) == 0 {
		return ""
	}
	var parts []string
	for k, v := range args {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return strings.Join(parts, ", ")
}
