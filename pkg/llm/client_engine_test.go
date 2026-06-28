// pkg/llm/client_engine_test.go
package llm_test

import (
	"context"
	"testing"

	"github.com/lcoder/lcoder/pkg/llm"
	"github.com/lcoder/lcoder/pkg/llm/catalog"
	"github.com/lcoder/lcoder/pkg/llm/engine"
	"github.com/lcoder/lcoder/pkg/llm/provider"
	"github.com/lcoder/lcoder/pkg/models"
)

func TestClientStreamMapsEvents(t *testing.T) {
	cat := catalog.New(catalog.Options{Refresh: false})
	eng := engine.New(cat)
	eng.SetAdapterFactory(func(route string, marks provider.CacheMarks) provider.Adapter {
		return scriptAdapter{[]provider.Event{
			{Kind: provider.KindStart},
			{Kind: provider.KindTextDelta, Delta: "hello"},
			{Kind: provider.KindDone, Message: models.AgentMessage{
				Role: models.RoleAssistant, Content: []models.ContentPart{models.TextContent{Text: "hello"}}}},
		}}
	})
	eng.RegisterProvider("openai", provider.Conn{Route: "openai"})
	c := llm.NewClient(eng)

	stream, err := c.StreamTurn(context.Background(), models.TurnRequest{
		Model: models.ModelRef{Provider: "openai", ID: "gpt-4o"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	var gotText string
	var sawDone bool
	for {
		ev, ok, err := stream.Next(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
		switch ev.Type() {
		case "text_delta":
			gotText += ev.Payload["delta"].(string)
		case "done":
			sawDone = true
			msg, err := ev.FinalMessage()
			if err != nil || msg.Text() != "hello" {
				t.Fatalf("final message wrong: %v / %q", err, msg.Text())
			}
		}
	}
	if gotText != "hello" || !sawDone {
		t.Fatalf("stream mapping wrong: text=%q done=%v", gotText, sawDone)
	}
}

// scriptAdapter is a local fake. (Mirrors engine.fakeAdapter; kept local to avoid exporting.)
type scriptAdapter struct{ events []provider.Event }

func (s scriptAdapter) Stream(ctx context.Context, conn provider.Conn, req models.TurnRequest) (<-chan provider.Event, error) {
	ch := make(chan provider.Event, len(s.events))
	for _, e := range s.events {
		ch <- e
	}
	close(ch)
	return ch, nil
}
