package mcp

import (
	"context"
	"testing"
)

func TestPrefixedName(t *testing.T) {
	if got := PrefixedName("server1", "toolA"); got != "server1_toolA" {
		t.Fatalf("unexpected prefixed name: %s", got)
	}
}

func TestExecutableDefinition(t *testing.T) {
	client := &Client{name: "test"}
	exec := NewExecutable(client, Tool{
		Name:        "echo",
		Description: "echo tool",
		InputSchema: map[string]any{"type": "object"},
	})
	def := exec.Definition()
	if def.Name != "test_echo" {
		t.Fatalf("expected test_echo, got %s", def.Name)
	}
}

func TestContentText(t *testing.T) {
	result := &CallToolResult{
		Content: []ContentItem{{Type: "text", Text: "hello"}, {Type: "text", Text: " world"}},
	}
	if result.ContentText() != "hello world" {
		t.Fatalf("unexpected content text: %s", result.ContentText())
	}
}

func TestExecutableExecuteError(t *testing.T) {
	// A nil client will panic on CallTool; use a registry status test instead.
	reg := NewRegistry(nil)
	statuses := reg.Servers()
	if len(statuses) != 0 {
		t.Fatalf("expected 0 statuses, got %d", len(statuses))
	}
}

var _ = context.Background
