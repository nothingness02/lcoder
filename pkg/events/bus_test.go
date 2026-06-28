package events

import (
	"context"
	"testing"
)

func TestBusOrderAndUnsubscribe(t *testing.T) {
	bus := New()

	var order []string
	h1 := bus.Subscribe(func(ctx context.Context, ev Event) error {
		order = append(order, "h1:"+string(ev.EventType()))
		return nil
	})
	_ = bus.Subscribe(func(ctx context.Context, ev Event) error {
		order = append(order, "h2:"+string(ev.EventType()))
		return nil
	})

	ctx := context.Background()
	if err := bus.Emit(ctx, AgentStartEvent{Base: Base{Type: AgentStart}}); err != nil {
		t.Fatalf("emit: %v", err)
	}

	if len(order) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(order))
	}
	if order[0] != "h1:agent_start" || order[1] != "h2:agent_start" {
		t.Fatalf("unexpected order: %v", order)
	}

	h1()
	order = nil
	if err := bus.Emit(ctx, AgentStartEvent{Base: Base{Type: AgentStart}}); err != nil {
		t.Fatalf("emit after unsubscribe: %v", err)
	}
	if len(order) != 1 || order[0] != "h2:agent_start" {
		t.Fatalf("expected only h2, got %v", order)
	}
}
