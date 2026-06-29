package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lcoder/lcoder/pkg/events"
	"github.com/lcoder/lcoder/pkg/llm"
	"github.com/lcoder/lcoder/pkg/models"
)

func (a *Agent) streamAssistant(ctx context.Context, turn int) (models.AgentMessage, error) {
	streamCtx, streamCancel := context.WithCancel(ctx)
	a.mu.Lock()
	a.streamAbort = streamCancel
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.streamAbort = nil
		a.mu.Unlock()
		streamCancel()
	}()

	systemPrompt, tools, modelRef, execMode := a.applyMode()

	var req models.TurnRequest
	var err error
	if a.cfg.TransformContext != nil {
		messages := a.allMessages()
		transformed, terr := a.cfg.TransformContext(ctx, messages)
		if terr != nil {
			return models.AgentMessage{}, fmt.Errorf("transform context: %w", terr)
		}
		req = models.TurnRequest{
			Model:        modelRef,
			SystemPrompt: systemPrompt,
			Messages:     transformed,
			Tools:        tools,
			Generation: models.GenerationConfig{
				Temperature: 0.2,
				MaxTokens:   4096,
			},
			Cache: "auto",
		}
	} else {
		req, err = a.mgr.BuildTurnRequest(modelRef, tools)
		if err != nil {
			return models.AgentMessage{}, fmt.Errorf("build turn request: %w", err)
		}
	}

	_ = execMode

	turnStartTime := time.Now()
	stream, err := a.llm.StreamTurnRetry(ctx, req, llm.DefaultRetryConfig())
	if err != nil {
		return models.AgentMessage{}, err
	}
	defer stream.Close()

	var partial models.AgentMessage
	started := false
	ttftRecorded := false

	for {
		select {
		case <-streamCtx.Done():
			return partial, streamCtx.Err()
		default:
		}

		ev, ok, err := stream.Next(streamCtx)
		if err != nil {
			return partial, err
		}
		if !ok {
			break
		}

		switch ev.Type() {
		case "start":
			partial = models.NewAgentMessage(models.RoleAssistant)
			if !started {
				started = true
				_ = a.bus.Emit(ctx, events.MessageStartEvent{
					Base:    events.Base{Type: events.MessageStart, Turn: turn},
					Message: partial,
				})
			}
		case "text_delta", "thinking_delta":
			if !started {
				started = true
				partial = models.NewAgentMessage(models.RoleAssistant)
				_ = a.bus.Emit(ctx, events.MessageStartEvent{
					Base:    events.Base{Type: events.MessageStart, Turn: turn},
					Message: partial,
				})
			}
			if !ttftRecorded && a.obsCollector != nil {
				ttftRecorded = true
				_ = a.obsCollector.RecordTTFT(turn, time.Since(turnStartTime).Milliseconds())
			}
			delta, _ := ev.Payload["delta"].(string)
			if ev.Type() == "text_delta" {
				partial = updateText(partial, delta)
			} else {
				partial = updateThinking(partial, delta)
			}
			_ = a.bus.Emit(ctx, events.MessageUpdateEvent{
				Base:    events.Base{Type: events.MessageUpdate, Turn: turn},
				Delta:   delta,
				Message: partial,
			})
		case "toolcall_start", "toolcall_delta":
			if !started {
				started = true
				partial = models.NewAgentMessage(models.RoleAssistant)
				_ = a.bus.Emit(ctx, events.MessageStartEvent{
					Base:    events.Base{Type: events.MessageStart, Turn: turn},
					Message: partial,
				})
			}
			call, _ := ev.Payload["tool_call"].(map[string]any)
			partial = updateToolCall(partial, call, false)
			_ = a.bus.Emit(ctx, events.MessageUpdateEvent{
				Base:    events.Base{Type: events.MessageUpdate, Turn: turn},
				Delta:   formatToolCallDelta(call),
				Message: partial,
			})
		case "toolcall_end":
			if !started {
				started = true
			}
			msg, _ := ev.FinalMessage()
			partial = msg
		case "done":
			msg, err := ev.FinalMessage()
			if err != nil {
				return partial, err
			}
			if !started {
				_ = a.bus.Emit(ctx, events.MessageStartEvent{
					Base:    events.Base{Type: events.MessageStart, Turn: turn},
					Message: msg,
				})
			}
			_ = a.bus.Emit(ctx, events.MessageEndEvent{
				Base:    events.Base{Type: events.MessageEnd, Turn: turn},
				Message: msg,
			})
			if usage, ok := ev.Usage(); ok {
				usage.Provider = a.cfg.Model.Provider
				usage.Model = a.cfg.Model.ID
				_ = a.obsCollector.RecordLLMUsage(usage)
			}
			a.appendMessage(msg)
			return msg, nil
		case "error":
			if ge, ok := ev.Error(); ok {
				return models.AgentMessage{}, ge
			}
			return models.AgentMessage{}, fmt.Errorf("unknown engine error")
		}
	}

	// Stream ended without a done event: use accumulated partial.
	if !started {
		_ = a.bus.Emit(ctx, events.MessageStartEvent{
			Base:    events.Base{Type: events.MessageStart, Turn: turn},
			Message: partial,
		})
	}
	_ = a.bus.Emit(ctx, events.MessageEndEvent{
		Base:    events.Base{Type: events.MessageEnd, Turn: turn},
		Message: partial,
	})
	a.appendMessage(partial)
	return partial, nil
}

func updateText(msg models.AgentMessage, delta string) models.AgentMessage {
	if len(msg.Content) == 0 {
		msg.Content = []models.ContentPart{models.TextContent{Text: delta}}
		return msg
	}
	if text, ok := msg.Content[0].(models.TextContent); ok {
		text.Text += delta
		msg.Content[0] = text
		return msg
	}
	msg.Content = append([]models.ContentPart{models.TextContent{Text: delta}}, msg.Content...)
	return msg
}

func updateThinking(msg models.AgentMessage, delta string) models.AgentMessage {
	if len(msg.Content) == 0 {
		msg.Content = []models.ContentPart{models.ThinkingContent{Text: delta}}
		return msg
	}
	if thinking, ok := msg.Content[0].(models.ThinkingContent); ok {
		thinking.Text += delta
		msg.Content[0] = thinking
		return msg
	}
	msg.Content = append([]models.ContentPart{models.ThinkingContent{Text: delta}}, msg.Content...)
	return msg
}

func updateToolCall(msg models.AgentMessage, call map[string]any, final bool) models.AgentMessage {
	if call == nil {
		return msg
	}
	id, _ := call["id"].(string)
	name, _ := call["name"].(string)
	args, _ := call["arguments"].(map[string]any)

	for i, part := range msg.Content {
		if tc, ok := part.(models.ToolCallContent); ok && tc.ID == id {
			if name != "" {
				tc.Name = name
			}
			if tc.Arguments == nil {
				tc.Arguments = make(map[string]any)
			}
			for k, v := range args {
				tc.Arguments[k] = v
			}
			msg.Content[i] = tc
			return msg
		}
	}

	if args == nil {
		args = make(map[string]any)
	}
	msg.Content = append(msg.Content, models.ToolCallContent{
		ID:        id,
		Name:      name,
		Arguments: args,
	})
	return msg
}

func formatToolCallDelta(call map[string]any) string {
	if call == nil {
		return ""
	}
	b, _ := json.Marshal(call)
	return string(b)
}
