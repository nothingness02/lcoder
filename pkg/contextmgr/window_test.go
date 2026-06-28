package contextmgr

import (
	"errors"
	"strings"
	"testing"

	"github.com/lcoder/lcoder/pkg/models"
)

// When the summarizer fails, compaction must degrade to truncation rather than
// propagating a fatal error, so the turn can still be built.
func TestWindowCompactionFallsBackOnSummarizerError(t *testing.T) {
	mgr := NewManager(TokenBudget{
		MaxTotal:      2000,
		TargetTotal:   1500,
		ReserveOutput: 500,
	}, WithSummarizer(func(msgs []models.AgentMessage) (string, error) {
		return "", errors.New("summarizer down")
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
		t.Fatalf("expected graceful fallback, got error: %v", err)
	}
	if len(req.Messages) == 0 {
		t.Fatal("expected truncated messages to remain")
	}

	// No compacted summary should be injected on the failure path.
	for _, m := range req.Messages {
		if m.Role == models.RoleSystem {
			if v, ok := m.Metadata["compacted"].(bool); ok && v {
				t.Fatal("did not expect a compacted summary when summarizer fails")
			}
		}
	}

	// A user message must still be retained.
	var foundUser bool
	for _, m := range req.Messages {
		if m.Role == models.RoleUser {
			foundUser = true
			break
		}
	}
	if !foundUser {
		t.Fatal("expected at least one user message to be retained after truncation")
	}
}
