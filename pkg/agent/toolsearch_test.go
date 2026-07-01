package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/lcoder/lcoder/pkg/events"
	"github.com/lcoder/lcoder/pkg/models"
	"github.com/lcoder/lcoder/pkg/tools"
)

func TestHandleToolSearch_ResolvesDeferredTool(t *testing.T) {
	r := tools.NewRegistry(".")
	r.Register("edit", fakeTool{fullDef("edit", "Edit a single file using exact text replacement.")})

	bus := events.New()
	var end events.ToolExecutionEndEvent
	bus.Subscribe(func(ctx context.Context, ev events.Event) error {
		if e, ok := ev.(events.ToolExecutionEndEvent); ok {
			end = e
		}
		return nil
	})

	ag := &Agent{
		cfg:      Config{},
		registry: r,
		bus:      bus,
	}
	ag.executor = &executor{cfg: &ag.cfg, registry: r, emitter: &eventEmitter{bus: bus}}

	call := models.ToolCallContent{
		ID:   "call_1",
		Name: tools.ToolSearchName,
		Arguments: map[string]any{
			"query": "edit",
		},
	}
	msg := ag.executor.handleToolSearch(context.Background(), 0, models.AgentMessage{}, call)

	if end.ToolName != tools.ToolSearchName {
		t.Fatalf("expected ToolExecutionEndEvent for tool_search, got %q", end.ToolName)
	}
	if msg.Role != models.RoleToolResult {
		t.Fatalf("expected tool_result message, got %s", msg.Role)
	}
	trc, ok := msg.Content[0].(models.ToolResultContent)
	if !ok {
		t.Fatalf("expected ToolResultContent, got %T", msg.Content[0])
	}
	if !strings.Contains(trc.Text(), "edit") {
		t.Fatalf("expected result to mention edit, got %q", trc.Text())
	}
	if !strings.Contains(trc.Text(), `"parameters"`) {
		t.Fatalf("expected result to contain full schema, got %q", trc.Text())
	}
}

func TestHandleToolSearch_NoMatch(t *testing.T) {
	r := tools.NewRegistry(".")
	r.Register("edit", fakeTool{fullDef("edit", "Edit.")})

	ag := &Agent{cfg: Config{}, registry: r, bus: events.New()}
	ag.executor = &executor{cfg: &ag.cfg, registry: r, emitter: &eventEmitter{bus: ag.bus}}
	call := models.ToolCallContent{
		ID:        "call_2",
		Name:      tools.ToolSearchName,
		Arguments: map[string]any{"query": "nonexistent"},
	}
	msg := ag.executor.handleToolSearch(context.Background(), 0, models.AgentMessage{}, call)
	trc := msg.Content[0].(models.ToolResultContent)
	if !strings.Contains(trc.Text(), "No tools matched") {
		t.Fatalf("expected no-match text, got %q", trc.Text())
	}
}
