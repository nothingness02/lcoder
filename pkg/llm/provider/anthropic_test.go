// pkg/llm/provider/anthropic_test.go
package provider

import (
	"context"
	"testing"

	"github.com/lcoder/lcoder/pkg/models"
)

func TestAnthropicStreamTextThinkingUsage(t *testing.T) {
	body := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":10,\"output_tokens\":0}}}\n\n" +
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"thinking\"}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"hmm\"}}\n\n" +
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"text\"}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hi\"}}\n\n" +
		"event: message_delta\ndata: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":3}}\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	srv := sseServer(t, body)

	ad := Anthropic{}
	ch, err := ad.Stream(context.Background(),
		Conn{BaseURL: srv.URL, APIKey: "k", Route: "anthropic"},
		models.TurnRequest{Model: models.ModelRef{Provider: "anthropic", ID: "claude-sonnet-4"},
			Messages: []models.AgentMessage{models.UserMessage("hi")}})
	if err != nil {
		t.Fatal(err)
	}
	evs := collect(t, ch)

	var text, thinking string
	var done *Event
	for i := range evs {
		switch evs[i].Kind {
		case KindTextDelta:
			text += evs[i].Delta
		case KindThinkingDelta:
			thinking += evs[i].Delta
		case KindDone:
			done = &evs[i]
		}
	}
	if text != "Hi" || thinking != "hmm" {
		t.Errorf("text=%q thinking=%q", text, thinking)
	}
	if done == nil || done.Usage == nil || done.Usage.PromptTokens != 10 || done.Usage.CompletionTokens != 3 {
		t.Fatalf("usage wrong: %+v", done)
	}
	if done.Message.Text() != "Hi" || done.Message.Thinking() != "hmm" {
		t.Errorf("final message wrong: %+v", done.Message)
	}
}

func TestAnthropicStreamToolUse(t *testing.T) {
	body := "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"tu1\",\"name\":\"read\"}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"path\\\":\"}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"\\\"x\\\"}\"}}\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	srv := sseServer(t, body)

	ad := Anthropic{}
	ch, _ := ad.Stream(context.Background(),
		Conn{BaseURL: srv.URL, APIKey: "k", Route: "anthropic"},
		models.TurnRequest{Model: models.ModelRef{Provider: "anthropic", ID: "claude-sonnet-4"}})
	evs := collect(t, ch)

	var done *Event
	for i := range evs {
		if evs[i].Kind == KindDone {
			done = &evs[i]
		}
	}
	calls := done.Message.ToolCalls()
	if len(calls) != 1 || calls[0].ID != "tu1" || calls[0].Name != "read" {
		t.Fatalf("tool call meta wrong: %+v", calls)
	}
	if calls[0].Arguments["path"] != "x" {
		t.Errorf("args=%+v", calls[0].Arguments)
	}
}
