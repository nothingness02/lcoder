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

func TestValidateArgs_DegradesWithoutProperties(t *testing.T) {
	// No "properties" key — e.g. an unusual MCP/extension schema.
	def := defWith(map[string]any{"type": "object"})
	if err := ValidateArgs(def, map[string]any{"anything": 1}); err != nil {
		t.Fatalf("schema without properties should pass through, got %v", err)
	}
	// Entirely empty parameters.
	if err := ValidateArgs(defWith(map[string]any{}), map[string]any{}); err != nil {
		t.Fatalf("empty schema should pass through, got %v", err)
	}
}

func TestValidateArgs_NoRequiredOnlyTypes(t *testing.T) {
	def := defWith(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string"},
		},
	})
	// Missing optional field is fine.
	if err := ValidateArgs(def, map[string]any{}); err != nil {
		t.Fatalf("missing optional field should pass, got %v", err)
	}
	// But a present field with wrong type still fails.
	if err := ValidateArgs(def, map[string]any{"path": 123}); err == nil {
		t.Fatal("present field with wrong type should fail")
	}
}

func TestValidateArgs_NestedNotInspected(t *testing.T) {
	def := defWith(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"edits": map[string]any{"type": "array"},
		},
		"required": []string{"edits"},
	})
	// Array items are malformed, but top-level type (array) is correct, so the
	// top-level validator accepts it — nesting is out of scope by design.
	args := map[string]any{"edits": []any{"not-an-object"}}
	if err := ValidateArgs(def, args); err != nil {
		t.Fatalf("nested item errors must not be caught at top level, got %v", err)
	}
}

func TestValidateArgs_RequiredAsAnySlice(t *testing.T) {
	// JSON unmarshaling yields []any for "required", not []string.
	def := defWith(map[string]any{
		"type":       "object",
		"properties": map[string]any{"path": map[string]any{"type": "string"}},
		"required":   []any{"path"},
	})
	if err := ValidateArgs(def, map[string]any{}); err == nil {
		t.Fatal("required given as []any should still be enforced")
	}
}
