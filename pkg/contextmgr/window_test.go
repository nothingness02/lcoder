package contextmgr

import (
	"strings"
	"testing"

	"github.com/lcoder/lcoder/pkg/models"
)

// A compacted/truncated tail must never begin with a tool_result whose matching
// tool_use was cut off, since Anthropic rejects an orphan tool_result. The
// window policy strips such leading orphans.
func TestWindowStripsLeadingOrphanToolResult(t *testing.T) {
	orphan := models.NewAgentMessage(models.RoleToolResult,
		models.ToolResultContent{ToolCallID: "x", Content: []models.ContentPart{models.TextContent{Text: "orphan result"}}})

	msgs := []models.AgentMessage{
		orphan,
		orphan,
		models.UserMessage("real user turn"),
		models.AssistantMessage("reply"),
	}
	got := stripLeadingOrphanToolResults(msgs)
	if len(got) != 2 {
		t.Fatalf("expected 2 messages after stripping orphans, got %d", len(got))
	}
	if got[0].Role != models.RoleUser {
		t.Fatalf("expected tail to start with a user message, got %v", got[0].Role)
	}
}

// A paired tool_result (preceded by its assistant tool_use within the tail)
// must be preserved.
func TestWindowKeepsPairedToolResult(t *testing.T) {
	msgs := []models.AgentMessage{
		models.NewAgentMessage(models.RoleAssistant,
			models.ToolCallContent{ID: "x", Name: "read", Arguments: map[string]any{}}),
		models.NewAgentMessage(models.RoleToolResult,
			models.ToolResultContent{ToolCallID: "x", Content: []models.ContentPart{models.TextContent{Text: "ok"}}}),
	}
	got := stripLeadingOrphanToolResults(msgs)
	if len(got) != 2 || got[0].Role != models.RoleAssistant {
		t.Fatalf("paired tool_use/tool_result must be kept intact, got %v", got)
	}
}

// The window policy no longer summarizes; it only truncates to fit the hard
// limit. Even with a summarizer configured on the manager, BuildTurnRequest must
// inject no compacted summary and must keep the request within budget.
func TestWindowTruncatesWithoutSummarizing(t *testing.T) {
	mgr := NewManager(TokenBudget{
		MaxTotal:      2000,
		TargetTotal:   1500,
		ReserveOutput: 500,
	}, WithSummarizer(func(msgs []models.AgentMessage) (string, error) {
		return "should not be called by window policy", nil
	}), WithWindowPolicy(DefaultKeepRecentInBudget()))

	mgr.SetBlock(NewBlock(BlockSystem, "system", StabilityStatic, 100,
		models.NewAgentMessage(models.RoleSystem, models.TextContent{Text: strings.Repeat("a", 800)}),
	))

	var recent []models.AgentMessage
	for i := 0; i < 20; i++ {
		recent = append(recent, models.UserMessage(strings.Repeat("u", 200)))
		recent = append(recent, models.AssistantMessage(strings.Repeat("a", 200)))
	}
	mgr.SetBlock(NewBlock(BlockRecent, "recent", StabilityDynamic, 100, recent...))

	req, err := mgr.BuildTurnRequest(models.ModelRef{Provider: "openai", ID: "gpt-4o"}, nil)
	if err != nil {
		t.Fatalf("expected graceful truncation, got error: %v", err)
	}
	if len(req.Messages) == 0 {
		t.Fatal("expected truncated messages to remain")
	}
	for _, m := range req.Messages {
		if m.Role == models.RoleSystem {
			if v, ok := m.Metadata["compacted"].(bool); ok && v {
				t.Fatal("window policy must not inject a compacted summary")
			}
		}
	}
	var foundUser bool
	for _, m := range req.Messages {
		if m.Role == models.RoleUser {
			foundUser = true
			break
		}
	}
	if !foundUser {
		t.Fatal("expected at least one user message after truncation")
	}
}
