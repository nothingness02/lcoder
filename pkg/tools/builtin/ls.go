package builtin

import (
	"context"
	"os"
	"sort"
	"strings"

	"github.com/lcoder/lcoder/pkg/models"
	"github.com/lcoder/lcoder/pkg/sandbox"
	"github.com/lcoder/lcoder/pkg/tools"
)

// Ls lists directory contents.
type Ls struct {
	cwd string
	sb  sandbox.Sandbox
}

// UseSandbox injects the sandbox used to enforce filesystem checks.
func (l *Ls) UseSandbox(sb sandbox.Sandbox) { l.sb = sb }

// NewLs creates an ls tool.
func NewLs(cwd string) tools.Executable {
	return &Ls{cwd: cwd}
}

func (l *Ls) Definition() models.ToolDefinition {
	return models.ToolDefinition{
		Name:        "ls",
		Description: "List files and directories. Defaults to the current working directory.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Directory path to list",
				},
			},
		},
		ExecutionMode: models.ExecutionParallel,
	}
}

func (l *Ls) Execute(ctx context.Context, callID string, args map[string]any) (models.ToolExecutionResult, error) {
	path := l.cwd
	if v, ok := args["path"].(string); ok && v != "" {
		path = v
	}
	path, err := resolveAndCheck(l.cwd, l.sb, path, sandbox.FSRead)
	if err != nil {
		return models.ToolExecutionResult{}, err
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return models.ToolExecutionResult{}, err
	}

	var lines []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		lines = append(lines, name)
	}
	sort.Strings(lines)

	return models.ToolExecutionResult{
		Content: []models.ContentPart{models.TextContent{Text: strings.Join(lines, "\n")}},
		Details: map[string]any{"path": path, "count": len(lines)},
	}, nil
}

var _ tools.Executable = (*Ls)(nil)
