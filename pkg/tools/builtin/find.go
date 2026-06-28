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

// Find searches for files by name pattern.
type Find struct {
	cwd string
}

// NewFind creates a find tool.
func NewFind(cwd string) tools.Executable {
	return &Find{cwd: cwd}
}

func (f *Find) Definition() models.ToolDefinition {
	return models.ToolDefinition{
		Name:        "find",
		Description: "Find files by name pattern under a directory.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "Glob pattern to match file names, e.g. '*.go'",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Directory to search (default cwd)",
				},
			},
			"required": []string{"pattern"},
		},
		ExecutionMode: models.ExecutionParallel,
	}
}

func (f *Find) Execute(ctx context.Context, callID string, args map[string]any) (models.ToolResult, error) {
	pattern, ok := args["pattern"].(string)
	if !ok || pattern == "" {
		return models.ToolResult{}, fmt.Errorf("missing pattern")
	}

	path := f.cwd
	if v, ok := args["path"].(string); ok && v != "" {
		path = v
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(f.cwd, path)
	}
	path = filepath.Clean(path)

	var matches []string
	err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		matched, _ := filepath.Match(pattern, filepath.Base(p))
		if matched {
			rel, _ := filepath.Rel(f.cwd, p)
			matches = append(matches, rel)
		}
		return nil
	})
	if err != nil {
		return models.ToolResult{}, err
	}

	return models.ToolResult{
		Content: []models.ContentPart{models.TextContent{Text: strings.Join(matches, "\n")}},
		Details: map[string]any{"path": path, "matches": len(matches)},
	}, nil
}

var _ tools.Executable = (*Find)(nil)
