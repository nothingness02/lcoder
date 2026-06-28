package models

import (
	"encoding/json"
	"testing"
)

func TestUserMessage(t *testing.T) {
	msg := UserMessage("hello")
	if msg.Role != RoleUser {
		t.Fatalf("expected role %s, got %s", RoleUser, msg.Role)
	}
	if msg.Text() != "hello" {
		t.Fatalf("expected text hello, got %s", msg.Text())
	}
}

func TestAgentMessageJSONRoundTrip(t *testing.T) {
	original := NewAgentMessage(RoleAssistant,
		TextContent{Text: "I'll list files."},
		ToolCallContent{ID: "call_1", Name: "ls", Arguments: map[string]any{"path": "."}},
	)

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded AgentMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Role != original.Role {
		t.Fatalf("role mismatch: %s vs %s", decoded.Role, original.Role)
	}
	if decoded.Text() != original.Text() {
		t.Fatalf("text mismatch: %s vs %s", decoded.Text(), original.Text())
	}

	calls := decoded.ToolCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Name != "ls" {
		t.Fatalf("expected tool ls, got %s", calls[0].Name)
	}
	if calls[0].Arguments["path"] != "." {
		t.Fatalf("expected path '.', got %v", calls[0].Arguments["path"])
	}
}

func TestToolResultContentText(t *testing.T) {
	tr := ToolResultContent{
		ToolCallID: "call_1",
		Name:       "read",
		Content:    []ContentPart{TextContent{Text: "file contents"}},
	}
	if tr.Text() != "file contents" {
		t.Fatalf("expected 'file contents', got %s", tr.Text())
	}
}

func TestModelInfoNameRoundTrips(t *testing.T) {
	in := ModelInfo{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", ContextWindow: 128000}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out ModelInfo
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if out.Name != "GPT-4o" {
		t.Fatalf("Name did not round-trip: %q", out.Name)
	}
}
