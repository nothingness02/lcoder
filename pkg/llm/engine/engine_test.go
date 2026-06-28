// pkg/llm/engine/engine_test.go
package engine

import (
	"context"
	"testing"

	"github.com/lcoder/lcoder/pkg/llm/catalog"
	"github.com/lcoder/lcoder/pkg/llm/provider"
	"github.com/lcoder/lcoder/pkg/models"
)

// fakeAdapter emits a fixed event script.
type fakeAdapter struct{ events []provider.Event }

func (f fakeAdapter) Stream(ctx context.Context, conn provider.Conn, req models.TurnRequest) (<-chan provider.Event, error) {
	ch := make(chan provider.Event, len(f.events))
	for _, e := range f.events {
		ch <- e
	}
	close(ch)
	return ch, nil
}

func TestEngineFillsCostOnDone(t *testing.T) {
	cat := catalog.New(catalog.Options{Refresh: false}) // gpt-4o priced 2.5/10 in snapshot
	eng := New(cat)
	eng.SetAdapterFactory(func(route string, marks provider.CacheMarks) provider.Adapter {
		return fakeAdapter{events: []provider.Event{
			{Kind: provider.KindTextDelta, Delta: "hi"},
			{Kind: provider.KindDone,
				Message: models.AgentMessage{Role: models.RoleAssistant},
				Usage:   &models.LLMUsage{PromptTokens: 1_000_000, CompletionTokens: 500_000}},
		}}
	})
	eng.RegisterProvider("openai", provider.Conn{Route: "openai"})

	ch, err := eng.StreamTurn(context.Background(), models.TurnRequest{
		Model: models.ModelRef{Provider: "openai", ID: "gpt-4o"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got *models.LLMUsage
	for ev := range ch {
		if ev.Kind == provider.KindDone {
			got = ev.Usage
		}
	}
	if got == nil {
		t.Fatal("no done event")
	}
	if got.TotalCost != 7.5 {
		t.Fatalf("cost not computed: got %v, want 7.5", got.TotalCost)
	}
	if got.Provider != "openai" || got.Model != "gpt-4o" {
		t.Fatalf("usage provider/model not stamped: %+v", got)
	}
}

func TestEngineRoutesAnthropicCacheMarks(t *testing.T) {
	cat := catalog.New(catalog.Options{Refresh: false})
	eng := New(cat)
	var gotMarks provider.CacheMarks
	eng.SetAdapterFactory(func(route string, marks provider.CacheMarks) provider.Adapter {
		gotMarks = marks
		return fakeAdapter{events: []provider.Event{{Kind: provider.KindDone,
			Message: models.AgentMessage{Role: models.RoleAssistant}}}}
	})
	eng.RegisterProvider("anthropic", provider.Conn{Route: "anthropic"})
	ch, _ := eng.StreamTurn(context.Background(), models.TurnRequest{
		Model:    models.ModelRef{Provider: "anthropic", ID: "claude-sonnet-4-20250514"},
		Messages: []models.AgentMessage{models.UserMessage("hi")},
	})
	for range ch {
	}
	if !gotMarks.System || len(gotMarks.MessageIdx) != 1 {
		t.Fatalf("anthropic cache marks not computed: %+v", gotMarks)
	}
}
