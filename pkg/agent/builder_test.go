package agent

import (
	"testing"

	"github.com/lcoder/lcoder/pkg/events"
	"github.com/lcoder/lcoder/pkg/llm/llmtest"
	"github.com/lcoder/lcoder/pkg/models"
	"github.com/lcoder/lcoder/pkg/permissions"
	"github.com/lcoder/lcoder/pkg/tools"
)

func TestBuilderRequiresGatewayClient(t *testing.T) {
	_, err := NewBuilder().Build()
	if err == nil {
		t.Fatal("expected error for missing gateway client")
	}
}

func TestBuilderBuildsAgent(t *testing.T) {
	bus := events.New()
	registry := tools.NewRegistry(".")
	client := llmtest.Client(llmtest.Turn(llmtest.Done(models.AssistantMessage(""), nil)))
	ag, err := NewBuilder().
		WithGatewayClient(client).
		WithRegistry(registry).
		WithEventBus(bus).
		WithPermissions(permissions.NewEngineFromRules(nil)).
		WithSystemPrompt("test prompt").
		WithModel("openai", "gpt-4o-mini").
		WithMaxTurns(10).
		WithToolExecutionMode(models.ExecutionSequential).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if ag.cfg.SystemPrompt != "test prompt" {
		t.Fatalf("unexpected system prompt: %s", ag.cfg.SystemPrompt)
	}
	if ag.cfg.MaxTurns != 10 {
		t.Fatalf("unexpected max turns: %d", ag.cfg.MaxTurns)
	}
	if ag.cfg.ToolExecutionMode != models.ExecutionSequential {
		t.Fatalf("unexpected execution mode: %v", ag.cfg.ToolExecutionMode)
	}
}
