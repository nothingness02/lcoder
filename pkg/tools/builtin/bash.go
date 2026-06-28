package builtin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lcoder/lcoder/pkg/models"
	"github.com/lcoder/lcoder/pkg/tools"
)

// Bash executes shell commands.
type Bash struct {
	cwd string
}

// NewBash creates a bash tool.
func NewBash(cwd string) tools.Executable {
	return &Bash{cwd: cwd}
}

func (b *Bash) Definition() models.ToolDefinition {
	return models.ToolDefinition{
		Name:        "bash",
		Description: "Execute a shell command in the project working directory.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "Shell command to execute",
				},
				"timeout": map[string]any{
					"type":        "integer",
					"description": "Timeout in seconds (default 60)",
				},
			},
			"required": []string{"command"},
		},
		ExecutionMode: models.ExecutionSequential,
	}
}

func (b *Bash) Execute(ctx context.Context, callID string, args map[string]any) (models.ToolResult, error) {
	command, ok := args["command"].(string)
	if !ok || command == "" {
		return models.ToolResult{}, fmt.Errorf("missing command")
	}

	timeout := 60
	if v, ok := args["timeout"].(float64); ok {
		timeout = int(v)
	}

	cwd := b.cwd
	if !filepath.IsAbs(cwd) {
		abs, err := filepath.Abs(cwd)
		if err == nil {
			cwd = abs
		}
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "sh"
	}

	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, shell, "-c", command)
	cmd.Dir = cwd
	cmd.Env = os.Environ()

	out, err := cmd.CombinedOutput()
	output := string(out)
	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			output += "\n[command timed out]"
		}
		return models.ToolResult{
			Content: []models.ContentPart{models.TextContent{Text: strings.TrimSpace(output)}},
			Details: map[string]any{"command": command, "cwd": cwd},
		}, fmt.Errorf("command failed: %w", err)
	}

	return models.ToolResult{
		Content: []models.ContentPart{models.TextContent{Text: strings.TrimSpace(output)}},
		Details: map[string]any{"command": command, "cwd": cwd},
	}, nil
}

var _ tools.Executable = (*Bash)(nil)
