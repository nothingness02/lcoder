package builtin

import (
	"context"
	"strings"
	"testing"

	"github.com/lcoder/lcoder/pkg/models"
	"github.com/lcoder/lcoder/pkg/task"
)

func TestTodoWriteDefinition(t *testing.T) {
	def := NewTodoWrite("").Definition()
	if def.Name != task.ToolName {
		t.Fatalf("name = %q, want %q", def.Name, task.ToolName)
	}
	if def.Parameters["type"] != "object" {
		t.Fatalf("parameters must be a JSON schema object: %+v", def.Parameters)
	}
}

func TestTodoWriteExecuteSummary(t *testing.T) {
	tool := NewTodoWrite("")
	args := map[string]any{"todos": []any{
		map[string]any{"text": "a", "status": "done"},
		map[string]any{"text": "b", "status": "in_progress"},
		map[string]any{"text": "c", "status": "pending"},
	}}
	res, err := tool.Execute(context.Background(), "call-1", args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := toolText(res)
	if !strings.Contains(got, "3 tasks") || !strings.Contains(got, "1 done") {
		t.Fatalf("summary wrong: %q", got)
	}
}

func TestTodoWriteExecuteRejectsBad(t *testing.T) {
	tool := NewTodoWrite("")
	args := map[string]any{"todos": []any{
		map[string]any{"text": "a", "status": "nope"},
	}}
	if _, err := tool.Execute(context.Background(), "call-1", args); err == nil {
		t.Fatal("expected error for invalid status")
	}
}

// toolText extracts the concatenated text content of a ToolExecutionResult.
func toolText(res models.ToolExecutionResult) string {
	var out string
	for _, part := range res.Content {
		if tc, ok := part.(models.TextContent); ok {
			out += tc.Text
		}
	}
	return out
}
