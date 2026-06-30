package tools

import (
	"context"
	"testing"

	"github.com/lcoder/lcoder/pkg/models"
	"github.com/lcoder/lcoder/pkg/sandbox"
)

type fakeAwareTool struct{ got sandbox.Sandbox }

func (f *fakeAwareTool) Definition() models.ToolDefinition {
	return models.ToolDefinition{Name: "aware"}
}
func (f *fakeAwareTool) Execute(ctx context.Context, callID string, args map[string]any) (models.ToolResult, error) {
	return models.ToolResult{}, nil
}
func (f *fakeAwareTool) UseSandbox(sb sandbox.Sandbox) { f.got = sb }

type plainTool struct{}

func (plainTool) Definition() models.ToolDefinition {
	return models.ToolDefinition{Name: "plain"}
}
func (plainTool) Execute(ctx context.Context, callID string, args map[string]any) (models.ToolResult, error) {
	return models.ToolResult{}, nil
}

func TestRegisterInjectsSandboxIntoAwareTool(t *testing.T) {
	r := NewRegistry(".")
	sb := sandbox.NewFakeSandbox()
	r.SetSandbox(sb)
	tool := &fakeAwareTool{}
	r.Register("aware", tool)
	if tool.got != sb {
		t.Fatalf("expected sandbox injected, got %v", tool.got)
	}
}

func TestRegisterSkipsPlainTool(t *testing.T) {
	r := NewRegistry(".")
	r.SetSandbox(sandbox.NewFakeSandbox())
	r.Register("plain", plainTool{}) // must not panic
}

func TestRegisterNilSandboxNoPanic(t *testing.T) {
	r := NewRegistry(".")
	r.Register("aware", &fakeAwareTool{}) // sb nil, must not panic
}
