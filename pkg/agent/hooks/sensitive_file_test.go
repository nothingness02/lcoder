package hooks

import (
	"context"
	"testing"

	"github.com/lcoder/lcoder/pkg/agent"
	"github.com/lcoder/lcoder/pkg/models"
)

func TestSensitiveFileCheck(t *testing.T) {
	h := SensitiveFileCheck([]string{"*.env", "*.key"})
	info := agent.ToolCallInfo{
		ToolCall: models.ToolCallContent{Name: "read", ID: "1"},
		Args:     map[string]any{"path": "config.env"},
	}
	result, err := h(context.Background(), info)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || !result.Block {
		t.Fatal("expected block")
	}
}

func TestSensitiveFileCheckNonTool(t *testing.T) {
	h := SensitiveFileCheck([]string{"*.env"})
	info := agent.ToolCallInfo{
		ToolCall: models.ToolCallContent{Name: "bash", ID: "1"},
		Args:     map[string]any{"command": "ls"},
	}
	result, err := h(context.Background(), info)
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Fatal("expected nil for bash")
	}
}
