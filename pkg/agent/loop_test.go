package agent

import (
	"context"
	"testing"
	"time"

	"github.com/lcoder/lcoder/pkg/events"
	"github.com/lcoder/lcoder/pkg/llm/llmtest"
	"github.com/lcoder/lcoder/pkg/models"
	"github.com/lcoder/lcoder/pkg/observability"
	"github.com/lcoder/lcoder/pkg/permissions"
	"github.com/lcoder/lcoder/pkg/tools"
	"github.com/lcoder/lcoder/pkg/tools/builtin"
)

func testRegistry(root string) *tools.Registry {
	registry := tools.NewRegistry(root)
	for _, f := range builtin.Factories() {
		registry.RegisterBuiltin(f)
	}
	return registry
}

func TestAgentOneTurn(t *testing.T) {
	client := llmtest.Client(llmtest.Turn(
		llmtest.Start(),
		llmtest.Text("Hello"),
		llmtest.Done(models.AssistantMessage("Hello"), nil),
	))

	bus := events.New()
	var got []string
	bus.Subscribe(func(ctx context.Context, ev events.Event) error {
		got = append(got, string(ev.EventType()))
		return nil
	})

	obs := observability.NewCollector(observability.NewMemoryExporter())
	ag := NewWithObservability(Config{
		SystemPrompt:      "You are helpful.",
		Model:             models.ModelRef{Provider: "openai", ID: "gpt-4o-mini"},
		MaxTurns:          5,
		ToolExecutionMode: models.ExecutionParallel,
	}, client, testRegistry("."), permissions.NewEngine(permissions.DefaultConfig()), bus, obs)

	if err := ag.Prompt(context.Background(), models.UserMessage("hi")); err != nil {
		t.Fatalf("prompt: %v", err)
	}

	if len(got) < 2 {
		t.Fatalf("expected events, got %v", got)
	}
	if got[0] != "agent_start" {
		t.Fatalf("expected agent_start, got %v", got)
	}
	if got[len(got)-1] != "agent_end" {
		t.Fatalf("expected agent_end last, got %v", got)
	}
}

func TestAgentToolCall(t *testing.T) {
	// First turn requests a tool; the finalized done message carries the call.
	toolMsg := models.NewAgentMessage(models.RoleAssistant, models.ToolCallContent{
		Type: "tool_call", ID: "call_1", Name: "ls", Arguments: map[string]any{},
	})
	client := llmtest.Client(llmtest.Turn(llmtest.Done(toolMsg, nil)))

	bus := events.New()
	var got []string
	bus.Subscribe(func(ctx context.Context, ev events.Event) error {
		got = append(got, string(ev.EventType()))
		return nil
	})

	obs := observability.NewCollector(observability.NewMemoryExporter())
	ag := NewWithObservability(Config{
		SystemPrompt:      "You are helpful.",
		Model:             models.ModelRef{Provider: "openai", ID: "gpt-4o-mini"},
		MaxTurns:          2,
		ToolExecutionMode: models.ExecutionParallel,
	}, client, testRegistry(t.TempDir()), permissions.NewEngine(permissions.DefaultConfig()), bus, obs)

	if err := ag.Prompt(context.Background(), models.UserMessage("list files")); err != nil {
		t.Fatalf("prompt: %v", err)
	}

	var hasToolStart, hasToolEnd bool
	for _, e := range got {
		if e == "tool_execution_start" {
			hasToolStart = true
		}
		if e == "tool_execution_end" {
			hasToolEnd = true
		}
	}
	if !hasToolStart || !hasToolEnd {
		t.Fatalf("expected tool events, got %v", got)
	}
}

func TestAgentMaxTurns(t *testing.T) {
	client, adapter := llmtest.NewScript(llmtest.Turn(
		llmtest.Done(models.AssistantMessage("ok"), nil),
	))

	obs := observability.NewCollector(observability.NewMemoryExporter())
	ag := NewWithObservability(Config{
		SystemPrompt:      "You are helpful.",
		Model:             models.ModelRef{Provider: "openai", ID: "gpt-4o-mini"},
		MaxTurns:          3,
		ToolExecutionMode: models.ExecutionParallel,
		ShouldStop: func(ctx context.Context, turn TurnSummary) (bool, error) {
			return false, nil
		},
	}, client, testRegistry("."), permissions.NewEngine(permissions.DefaultConfig()), events.New(), obs)

	_ = ag.Prompt(context.Background(), models.UserMessage("hi"))
	if adapter.CallCount() != 3 {
		t.Fatalf("expected 3 turns, got %d", adapter.CallCount())
	}
}

func TestAgentAbortIsReentrant(t *testing.T) {
	// A slow stream keeps the turn in-flight while Abort is called repeatedly.
	client, adapter := llmtest.NewScript(llmtest.Turn(
		append(llmtest.RepeatText("x", 100), llmtest.Done(models.AssistantMessage("done"), nil))...,
	))
	adapter.Delay = time.Millisecond

	obs := observability.NewCollector(observability.NewMemoryExporter())
	ag := NewWithObservability(Config{
		SystemPrompt:      "You are helpful.",
		Model:             models.ModelRef{Provider: "openai", ID: "gpt-4o-mini"},
		MaxTurns:          5,
		ToolExecutionMode: models.ExecutionParallel,
	}, client, testRegistry("."), permissions.NewEngine(permissions.DefaultConfig()), events.New(), obs)

	// Calling Abort multiple times must not panic.
	go func() {
		ag.Abort()
		ag.Abort()
		ag.Abort()
	}()

	_ = ag.Prompt(context.Background(), models.UserMessage("hi"))
}

func TestAgentAbortStopsStream(t *testing.T) {
	client, adapter := llmtest.NewScript(llmtest.Turn(
		append(llmtest.RepeatText("x", 50), llmtest.Done(models.AssistantMessage("long"), nil))...,
	))
	adapter.Delay = 5 * time.Millisecond

	obs := observability.NewCollector(observability.NewMemoryExporter())
	ag := NewWithObservability(Config{
		SystemPrompt:      "You are helpful.",
		Model:             models.ModelRef{Provider: "openai", ID: "gpt-4o-mini"},
		MaxTurns:          5,
		ToolExecutionMode: models.ExecutionParallel,
	}, client, testRegistry("."), permissions.NewEngine(permissions.DefaultConfig()), events.New(), obs)

	go func() {
		time.Sleep(25 * time.Millisecond)
		ag.Abort()
	}()

	err := ag.Prompt(context.Background(), models.UserMessage("hi"))
	// Aborting mid-stream returns context.Canceled. If the abort happens
	// before the stream starts, the full turn may complete. Both are valid
	// as long as the call does not panic.
	if err != nil {
		return
	}
	msgs := ag.AllMessages()
	if len(msgs) == 0 {
		t.Fatal("expected some messages after aborted prompt")
	}
	last := msgs[len(msgs)-1]
	if last.Role == models.RoleAssistant {
		// If the full turn completed, the assistant message should be short
		// (not the 50-delta "long" final message).
		if last.Text() == "long" {
			t.Fatal("expected stream to abort before producing final long message")
		}
	}
}

func TestAgentWithModeSnapshot(t *testing.T) {
	client := llmtest.Client(llmtest.Turn(llmtest.Done(models.AssistantMessage("ok"), nil)))

	mm := NewModeManager()
	mm.modes["code"] = ModeConfig{Name: "code", SystemPrompt: "code mode"}
	mm.modes["review"] = ModeConfig{Name: "review", SystemPrompt: "review mode"}

	obs := observability.NewCollector(observability.NewMemoryExporter())
	ag := NewWithObservability(Config{
		SystemPrompt:      "base",
		Model:             models.ModelRef{Provider: "openai", ID: "gpt-4o-mini"},
		MaxTurns:          1,
		ToolExecutionMode: models.ExecutionParallel,
		ModeManager:       mm,
		Mode:              "code",
	}, client, testRegistry("."), permissions.NewEngine(permissions.DefaultConfig()), events.New(), obs)

	reviewAg := ag.WithMode("review")
	if reviewAg.cfg.Mode != "review" {
		t.Fatalf("expected review mode, got %s", reviewAg.cfg.Mode)
	}
	// The snapshot should share the same base context system prompt, not
	// have accumulated mode prompts from the original agent.
	if reviewAg.mgr.SystemPrompt() != ag.mgr.SystemPrompt() {
		t.Fatalf("WithMode should snapshot base system prompt")
	}
}

func TestAgentTransformContext(t *testing.T) {
	client, adapter := llmtest.NewScript(llmtest.Turn(
		llmtest.Done(models.AssistantMessage("ok"), nil),
	))

	transformCalled := false
	obs := observability.NewCollector(observability.NewMemoryExporter())
	ag := NewWithObservability(Config{
		SystemPrompt:      "You are helpful.",
		Model:             models.ModelRef{Provider: "openai", ID: "gpt-4o-mini"},
		MaxTurns:          1,
		ToolExecutionMode: models.ExecutionParallel,
		TransformContext: func(ctx context.Context, msgs []models.AgentMessage) ([]models.AgentMessage, error) {
			transformCalled = true
			return msgs, nil
		},
	}, client, testRegistry("."), permissions.NewEngine(permissions.DefaultConfig()), events.New(), obs)

	_ = ag.Prompt(context.Background(), models.UserMessage("hi"))
	if !transformCalled {
		t.Fatal("expected TransformContext to be called")
	}
	// After the identity transform the request carries the compacted messages.
	if n := len(adapter.LastRequest().Messages); n > 3 {
		t.Fatalf("expected transformed messages, got %d", n)
	}
}
