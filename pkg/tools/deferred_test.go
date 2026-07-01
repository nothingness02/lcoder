package tools

import (
	"context"
	"testing"

	"github.com/lcoder/lcoder/pkg/models"
)

// fakeTool is a minimal Executable for deferred-loading tests.
type fakeTool struct {
	def models.ToolDefinition
}

func (f fakeTool) Definition() models.ToolDefinition { return f.def }
func (f fakeTool) Execute(context.Context, string, map[string]any) (models.ToolExecutionResult, error) {
	return models.ToolExecutionResult{}, nil
}

func fullDef(name, desc string) models.ToolDefinition {
	return models.ToolDefinition{
		Name:        name,
		Description: desc,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"path": map[string]any{"type": "string"}},
			"required":   []any{"path"},
		},
	}
}

func newTestRegistry(t *testing.T) *Registry {
	t.Helper()
	r := NewRegistry(".")
	r.Register("read", fakeTool{fullDef("read", "Read the contents of a file. Supports text and images.")})
	r.Register("bash", fakeTool{fullDef("bash", "Execute a shell command. Returns stdout and stderr.")})
	r.Register("edit", fakeTool{fullDef("edit", "Edit a single file using exact text replacement. Unique match required.")})
	r.Register("grep", fakeTool{fullDef("grep", "Search for a literal string in files under a directory.")})
	return r
}

func TestDeferredDefinitions_SplitsCoreAndDeferred(t *testing.T) {
	r := newTestRegistry(t)
	active, deferred := r.DeferredDefinitions("read", "bash")

	// Active = core tools (full schema) + tool_search, sorted core then search.
	if len(active) != 3 {
		t.Fatalf("expected 3 active defs (2 core + tool_search), got %d", len(active))
	}
	if active[len(active)-1].Name != ToolSearchName {
		t.Fatalf("expected tool_search to be the last active def, got %q", active[len(active)-1].Name)
	}
	// Core tools must keep their full parameter schema.
	for _, d := range active[:2] {
		if d.Parameters == nil {
			t.Fatalf("core tool %q lost its parameter schema under deferral", d.Name)
		}
	}
	// Deferred = the remaining tools, as name-only stubs (no parameters).
	if len(deferred) != 2 {
		t.Fatalf("expected 2 deferred stubs, got %d", len(deferred))
	}
	for _, d := range deferred {
		if d.Parameters != nil {
			t.Fatalf("deferred stub %q must not carry a parameter schema", d.Name)
		}
		if d.Description[:10] != "(deferred)" {
			t.Fatalf("deferred stub %q must be labeled (deferred), got %q", d.Name, d.Description)
		}
	}
	// Deterministic order.
	if deferred[0].Name != "edit" || deferred[1].Name != "grep" {
		t.Fatalf("expected deferred sorted [edit grep], got [%s %s]", deferred[0].Name, deferred[1].Name)
	}
}

func TestSearchTools_ResolvesFullSchema(t *testing.T) {
	r := newTestRegistry(t)
	// A deferred tool's full schema is recovered by keyword.
	hits := r.SearchTools("edit")
	if len(hits) != 1 || hits[0].Name != "edit" {
		t.Fatalf("expected to resolve [edit], got %v", names(hits))
	}
	if hits[0].Parameters == nil {
		t.Fatalf("tool_search must return the FULL schema, but parameters were nil")
	}
	// Description match works too ("file" appears in read & edit descriptions).
	if got := names(r.SearchTools("file")); len(got) < 2 {
		t.Fatalf("expected description match to find multiple tools, got %v", got)
	}
}

func names(defs []models.ToolDefinition) []string {
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Name
	}
	return out
}
