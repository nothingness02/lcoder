package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lcoder/lcoder/pkg/models"
	"github.com/lcoder/lcoder/pkg/tools"
)

// Read reads files with optional offset/limit.
type Read struct {
	cwd string
}

// NewRead creates a read tool.
func NewRead(cwd string) tools.Executable {
	return &Read{cwd: cwd}
}

func (r *Read) Definition() models.ToolDefinition {
	return models.ToolDefinition{
		Name:        "read",
		Description: "Read the contents of a file. Supports text files and images. For text files, output is truncated; use offset/limit for large files.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file to read (relative or absolute)",
				},
				"offset": map[string]any{
					"type":        "integer",
					"description": "Line number to start from (1-indexed)",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum number of lines to read",
				},
			},
			"required": []string{"path"},
		},
		ExecutionMode: models.ExecutionParallel,
	}
}

func (r *Read) Execute(ctx context.Context, callID string, args map[string]any) (models.ToolResult, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return models.ToolResult{}, fmt.Errorf("missing path")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(r.cwd, path)
	}
	path = filepath.Clean(path)

	info, err := os.Stat(path)
	if err != nil {
		return models.ToolResult{}, err
	}
	if info.IsDir() {
		return models.ToolResult{}, fmt.Errorf("path is a directory: %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return models.ToolResult{}, err
	}

	text := string(data)
	lines := strings.Split(text, "\n")

	offset := 1
	if v, ok := args["offset"].(float64); ok {
		offset = int(v)
	}
	limit := 0
	if v, ok := args["limit"].(float64); ok {
		limit = int(v)
	}

	start := offset - 1
	if start < 0 {
		start = 0
	}
	if start > len(lines) {
		start = len(lines)
	}
	end := len(lines)
	if limit > 0 {
		end = start + limit
		if end > len(lines) {
			end = len(lines)
		}
	}

	selected := strings.Join(lines[start:end], "\n")
	return models.ToolResult{
		Content: []models.ContentPart{
			models.TextContent{Text: selected},
		},
		Details: map[string]any{
			"path":  path,
			"lines": fmt.Sprintf("%d-%d", start+1, end),
		},
	}, nil
}

// Ensure Read implements Executable.
var _ tools.Executable = (*Read)(nil)
