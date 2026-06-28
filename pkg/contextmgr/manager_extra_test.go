package contextmgr

import (
	"testing"

	"github.com/lcoder/lcoder/pkg/models"
)

func TestManagerAllMessages(t *testing.T) {
	m := NewManager(TokenBudget{MaxTotal: 1000})
	m.SetSystemPrompt("you are an agent")
	m.AppendRecent(models.NewAgentMessage(models.RoleUser, models.TextContent{Text: "hello"}))
	m.AppendRecent(models.NewAgentMessage(models.RoleAssistant, models.TextContent{Text: "hi"}))

	msgs := m.AllMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != models.RoleUser {
		t.Fatalf("expected user first, got %s", msgs[0].Role)
	}
}

func TestManagerSetMessages(t *testing.T) {
	m := NewManager(TokenBudget{MaxTotal: 1000})
	m.SetMessages([]models.AgentMessage{
		models.NewAgentMessage(models.RoleSystem, models.TextContent{Text: "sys"}),
		models.NewAgentMessage(models.RoleUser, models.TextContent{Text: "u"}),
		models.NewAgentMessage(models.RoleAssistant, models.TextContent{Text: "a"}),
	})

	if m.SystemPrompt() != "sys" {
		t.Fatalf("unexpected system prompt: %s", m.SystemPrompt())
	}
	if len(m.AllMessages()) != 2 {
		t.Fatalf("expected 2 non-system messages, got %d", len(m.AllMessages()))
	}
}

func TestManagerClone(t *testing.T) {
	m := NewManager(TokenBudget{MaxTotal: 1000})
	m.AppendRecent(models.NewAgentMessage(models.RoleUser, models.TextContent{Text: "hello"}))

	other := m.Clone()
	other.AppendRecent(models.NewAgentMessage(models.RoleAssistant, models.TextContent{Text: "world"}))

	if len(m.AllMessages()) != 1 {
		t.Fatalf("original should have 1 message, got %d", len(m.AllMessages()))
	}
	if len(other.AllMessages()) != 2 {
		t.Fatalf("clone should have 2 messages, got %d", len(other.AllMessages()))
	}
}
