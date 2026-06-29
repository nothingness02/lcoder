package tools

import (
	"encoding/json"
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

func TestValidateArgs_TypeMismatch(t *testing.T) {
	def := defWith(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string"},
			"line": map[string]any{"type": "number"},
		},
		"required": []string{"path"},
	})
	err := ValidateArgs(def, map[string]any{"path": "main.go", "line": "42"})
	if err == nil {
		t.Fatal("expected error: line should be number, got string")
	}
	if !contains(err.Error(), "line") {
		t.Fatalf("error should name the offending field, got %q", err.Error())
	}
}

func TestValidateArgs_LenientNumbers(t *testing.T) {
	def := defWith(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"line": map[string]any{"type": "number"},
		},
	})
	for _, v := range []any{float64(42), int(42), int64(42), json.Number("42")} {
		if err := ValidateArgs(def, map[string]any{"line": v}); err != nil {
			t.Fatalf("number type should accept %T, got error %v", v, err)
		}
	}
}

func TestValidateArgs_Valid(t *testing.T) {
	def := defWith(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":  map[string]any{"type": "string"},
			"edits": map[string]any{"type": "array"},
		},
		"required": []string{"path", "edits"},
	})
	args := map[string]any{"path": "main.go", "edits": []any{map[string]any{}}}
	if err := ValidateArgs(def, args); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}
