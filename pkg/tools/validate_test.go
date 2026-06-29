package tools

import (
	"testing"

	"github.com/lcoder/lcoder/pkg/models"
)

func defWith(params map[string]any) models.ToolDefinition {
	return models.ToolDefinition{Name: "demo", Parameters: params}
}

func TestValidateArgs_MissingRequired(t *testing.T) {
	def := defWith(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":  map[string]any{"type": "string"},
			"edits": map[string]any{"type": "array"},
		},
		"required": []string{"path", "edits"},
	})
	err := ValidateArgs(def, map[string]any{"path": "main.go"})
	if err == nil {
		t.Fatal("expected error for missing required field 'edits'")
	}
	if !contains(err.Error(), "edits") {
		t.Fatalf("error should name the missing field, got %q", err.Error())
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
