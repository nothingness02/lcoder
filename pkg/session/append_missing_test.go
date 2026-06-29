package session

import (
	"os"
	"testing"

	"github.com/lcoder/lcoder/pkg/models"
)

// TestAppendMissingPersistsAgentOutput is the core of the persistence fix: a
// session only has the user message appended at submit time; the agent's
// assistant and tool_result messages live elsewhere and must be mirrored in by
// AppendMissing so they actually reach disk.
func TestAppendMissingPersistsAgentOutput(t *testing.T) {
	dir, err := os.MkdirTemp("", "lcoder-append-missing-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	store := NewStore(dir)
	sess, err := store.Create("/project")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// User message is appended at submit time, as the TUI does today.
	user := models.UserMessage("hi")
	if err := sess.Append(user); err != nil {
		t.Fatalf("append user: %v", err)
	}

	// The agent's full context window after a turn: user + assistant + tool.
	asst := models.NewAgentMessage(models.RoleAssistant, models.TextContent{Text: "hello"})
	tool := models.NewAgentMessage(models.RoleToolResult, models.ToolResultContent{
		ToolCallID: "c1", Name: "bash", Content: []models.ContentPart{models.TextContent{Text: "ok"}},
	})
	full := []models.AgentMessage{user, asst, tool}

	if err := sess.AppendMissing(full); err != nil {
		t.Fatalf("append missing: %v", err)
	}

	// Reload from disk to prove the agent output actually persisted.
	loaded, err := store.Load(sess.Path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Messages) != 3 {
		t.Fatalf("expected 3 persisted messages, got %d", len(loaded.Messages))
	}
	if loaded.Messages[1].Role != models.RoleAssistant || loaded.Messages[1].Text() != "hello" {
		t.Fatalf("assistant message not persisted in order: %+v", loaded.Messages[1])
	}
	if loaded.Messages[2].Role != models.RoleToolResult {
		t.Fatalf("tool_result message not persisted: %+v", loaded.Messages[2])
	}
}

// TestAppendMissingDedupesByID guards against double-appending the user message
// (already present) and is idempotent when called repeatedly with the same set.
func TestAppendMissingDedupesByID(t *testing.T) {
	dir, err := os.MkdirTemp("", "lcoder-append-missing-dedup-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	store := NewStore(dir)
	sess, _ := store.Create("/project")
	user := models.UserMessage("hi")
	_ = sess.Append(user)
	asst := models.AssistantMessage("hello")
	full := []models.AgentMessage{user, asst}

	if err := sess.AppendMissing(full); err != nil {
		t.Fatalf("append missing: %v", err)
	}
	if err := sess.AppendMissing(full); err != nil {
		t.Fatalf("append missing (2nd): %v", err)
	}
	if len(sess.Messages) != 2 {
		t.Fatalf("expected 2 messages after idempotent calls, got %d", len(sess.Messages))
	}
}
