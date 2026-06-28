package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lcoder/lcoder/pkg/models"
	"github.com/lcoder/lcoder/pkg/tools"
)

// Edit performs exact-text replacements in a file.
type Edit struct {
	cwd string
}

// NewEdit creates an edit tool.
func NewEdit(cwd string) tools.Executable {
	return &Edit{cwd: cwd}
}

func (e *Edit) Definition() models.ToolDefinition {
	return models.ToolDefinition{
		Name:        "edit",
		Description: "Edit a single file using exact text replacement. Each oldText must match a unique, non-overlapping region.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file to edit",
				},
				"edits": map[string]any{
					"type":        "array",
					"description": "One or more targeted replacements",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"oldText": map[string]any{
								"type":        "string",
								"description": "Exact text to replace",
							},
							"newText": map[string]any{
								"type":        "string",
								"description": "Replacement text",
							},
						},
						"required": []string{"oldText", "newText"},
					},
				},
			},
			"required": []string{"path", "edits"},
		},
		ExecutionMode: models.ExecutionSequential,
	}
}

func (e *Edit) Execute(ctx context.Context, callID string, args map[string]any) (models.ToolResult, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return models.ToolResult{}, fmt.Errorf("missing path")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(e.cwd, path)
	}
	path = filepath.Clean(path)

	data, err := os.ReadFile(path)
	if err != nil {
		return models.ToolResult{}, err
	}
	text := string(data)

	editsRaw, ok := args["edits"].([]any)
	if !ok || len(editsRaw) == 0 {
		return models.ToolResult{}, fmt.Errorf("missing edits")
	}

	for _, raw := range editsRaw {
		edit, ok := raw.(map[string]any)
		if !ok {
			return models.ToolResult{}, fmt.Errorf("invalid edit entry")
		}
		oldText, ok := edit["oldText"].(string)
		if !ok {
			return models.ToolResult{}, fmt.Errorf("edit missing oldText")
		}
		newText, ok := edit["newText"].(string)
		if !ok {
			return models.ToolResult{}, fmt.Errorf("edit missing newText")
		}
		if !containsOnce(text, oldText) {
			return models.ToolResult{}, fmt.Errorf("oldText not found or not unique in %s", path)
		}
		text = replaceOnce(text, oldText, newText)
	}

	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return models.ToolResult{}, err
	}

	return models.ToolResult{
		Content: []models.ContentPart{
			models.TextContent{Text: fmt.Sprintf("Applied %d edit(s) to %s", len(editsRaw), path)},
		},
		Details: map[string]any{"path": path, "edits": len(editsRaw)},
	}, nil
}

func containsOnce(s, substr string) bool {
	idx := findFirst(s, substr)
	if idx == -1 {
		return false
	}
	return findFirst(s[idx+len(substr):], substr) == -1
}

func replaceOnce(s, old, new string) string {
	idx := findFirst(s, old)
	if idx == -1 {
		return s
	}
	return s[:idx] + new + s[idx+len(old):]
}

func findFirst(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

var _ tools.Executable = (*Edit)(nil)
