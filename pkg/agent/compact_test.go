package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/lcoder/lcoder/pkg/contextmgr"
	"github.com/lcoder/lcoder/pkg/events"
	"github.com/lcoder/lcoder/pkg/models"
)

// agent 在每轮前调用 mgr.MaybeCompact;提交时发 CompactionCommitted 事件。
func TestAgentEmitsCompactionCommitted(t *testing.T) {
	mgr := contextmgr.NewManager(
		contextmgr.TokenBudget{MaxTotal: 2000, TargetTotal: 100, ReserveOutput: 0},
		contextmgr.WithSummarizer(func(msgs []models.AgentMessage) (string, error) { return "s", nil }),
		contextmgr.WithMinRecent(2),
	)
	var recent []models.AgentMessage
	for i := 0; i < 20; i++ {
		recent = append(recent, models.UserMessage(strings.Repeat("u", 200)))
		recent = append(recent, models.AssistantMessage(strings.Repeat("a", 200)))
	}
	mgr.SetBlock(contextmgr.NewBlock(contextmgr.BlockRecent, "recent", contextmgr.StabilityDynamic, 100, recent...))

	a := &Agent{mgr: mgr, bus: events.New()}
	var got bool
	unsub := a.bus.Subscribe(func(ctx context.Context, ev events.Event) error {
		if _, ok := ev.(events.CompactionCommittedEvent); ok {
			got = true
		}
		return nil
	})
	defer unsub()

	a.maybeCompact(context.Background(), 1)
	if !got {
		t.Fatal("expected CompactionCommitted event to be emitted")
	}
}
