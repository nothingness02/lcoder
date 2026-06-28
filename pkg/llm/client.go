// pkg/llm/client.go
package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lcoder/lcoder/pkg/config"
	"github.com/lcoder/lcoder/pkg/llm/engine"
	"github.com/lcoder/lcoder/pkg/llm/provider"
	"github.com/lcoder/lcoder/pkg/models"
)

// Client is the in-process LLM client. It keeps the method surface the agent
// loop and TUI depend on, delegating to an in-process engine.
type Client struct {
	engine *engine.Engine
}

// NewClient creates a client over an in-process engine.
func NewClient(eng *engine.Engine) *Client {
	return &Client{engine: eng}
}

// StreamTurn starts a provider turn and returns a channel-backed event stream.
func (c *Client) StreamTurn(ctx context.Context, req models.TurnRequest) (*TurnStream, error) {
	src, err := c.engine.StreamTurn(ctx, req)
	if err != nil {
		return nil, err
	}
	out := make(chan GatewayEvent)
	go func() {
		defer close(out)
		for ev := range src {
			out <- mapEvent(ev)
		}
	}()
	return &TurnStream{ch: out}, nil
}

// mapEvent translates a provider.Event into a GatewayEvent with the exact
// payload shape the agent loop consumes (see Critical Contracts).
func mapEvent(ev provider.Event) GatewayEvent {
	switch ev.Kind {
	case provider.KindStart:
		return GatewayEvent{Name: "start", Payload: map[string]any{"type": "start"}}
	case provider.KindTextDelta:
		return GatewayEvent{Name: "text_delta", Payload: map[string]any{"type": "text_delta", "delta": ev.Delta}}
	case provider.KindThinkingDelta:
		return GatewayEvent{Name: "thinking_delta", Payload: map[string]any{"type": "thinking_delta", "delta": ev.Delta}}
	case provider.KindToolCallDelta:
		return GatewayEvent{Name: "toolcall_delta", Payload: map[string]any{
			"type":            "toolcall_delta",
			"tool_call_index": ev.ToolCallIndex,
			"arguments_json":  ev.ArgumentsJSON,
		}}
	case provider.KindDone:
		payload := map[string]any{"type": "done"}
		// Message is a value type; on done it is always the finalized assistant
		// message, so emit it unconditionally.
		payload["message"] = jsonRoundTrip(ev.Message)
		if ev.Usage != nil {
			payload["usage"] = jsonRoundTrip(ev.Usage)
		}
		return GatewayEvent{Name: "done", Payload: payload}
	case provider.KindError:
		ge := GatewayError{Code: "internal", Message: "unknown error"}
		if ev.Err != nil {
			ge = GatewayError{Code: ev.Err.Code, Message: ev.Err.Message, ProviderError: ev.Err.ProviderError}
		}
		return GatewayEvent{Name: "error", Payload: map[string]any{"type": "error", "error": jsonRoundTrip(ge)}}
	default:
		return GatewayEvent{Name: "", Payload: map[string]any{}}
	}
}

// jsonRoundTrip converts a typed value into the map/any shape that
// GatewayEvent.FinalMessage/Usage/Error re-decode, so AgentMessage's custom
// MarshalJSON is honored.
func jsonRoundTrip(v any) any {
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out any
	_ = json.Unmarshal(data, &out)
	return out
}

// RegisterProvider stores a provider connection on the engine (in-process).
func (c *Client) RegisterProvider(ctx context.Context, name string, conn config.ProviderConn) error {
	c.engine.RegisterProvider(name, provider.Conn{
		BaseURL: conn.BaseURL,
		APIKey:  conn.APIKey,
		Route:   conn.Route,
		Headers: conn.Headers,
	})
	return nil
}

// ListModels returns the available models from the catalog.
func (c *Client) ListModels(ctx context.Context) ([]models.ModelInfo, error) {
	return c.engine.ListModels(), nil
}

// ModelWindow returns the catalog context window for provider/model (0 if unknown).
func (c *Client) ModelWindow(ctx context.Context, prov, model string) (int, error) {
	return c.engine.ModelWindow(prov, model), nil
}

// Health reports in-process readiness.
func (c *Client) Health(ctx context.Context) (map[string]string, error) {
	return map[string]string{"status": "ok"}, nil
}

// GatewayEvent is a normalized event from the engine.
type GatewayEvent struct {
	Name    string
	Raw     string
	Payload map[string]any
}

// Type returns the payload type field if present.
func (e GatewayEvent) Type() string {
	if t, ok := e.Payload["type"].(string); ok {
		return t
	}
	return ""
}

// Usage extracts LLM usage from a "done" event if present.
func (e GatewayEvent) Usage() (models.LLMUsage, bool) {
	usageAny, ok := e.Payload["usage"]
	if !ok {
		return models.LLMUsage{}, false
	}
	data, err := json.Marshal(usageAny)
	if err != nil {
		return models.LLMUsage{}, false
	}
	var usage models.LLMUsage
	if err := json.Unmarshal(data, &usage); err != nil {
		return models.LLMUsage{}, false
	}
	return usage, true
}

// FinalMessage extracts the final assistant message from a "done" event.
func (e GatewayEvent) FinalMessage() (models.AgentMessage, error) {
	msgAny, ok := e.Payload["message"]
	if !ok {
		return models.AgentMessage{}, fmt.Errorf("done event missing message")
	}
	data, err := json.Marshal(msgAny)
	if err != nil {
		return models.AgentMessage{}, err
	}
	var msg models.AgentMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return models.AgentMessage{}, err
	}
	return msg, nil
}

// Error extracts a GatewayError from an "error" event.
func (e GatewayEvent) Error() (GatewayError, bool) {
	errAny, ok := e.Payload["error"]
	if !ok {
		return GatewayError{}, false
	}
	data, err := json.Marshal(errAny)
	if err != nil {
		return GatewayError{}, false
	}
	var ge GatewayError
	if err := json.Unmarshal(data, &ge); err != nil {
		return GatewayError{}, false
	}
	return ge, true
}

// GatewayError is returned by the engine on failure.
type GatewayError struct {
	Code          string         `json:"code"`
	Message       string         `json:"message"`
	ProviderError map[string]any `json:"provider_error,omitempty"`
}

func (e GatewayError) Error() string {
	return fmt.Sprintf("engine error %s: %s", e.Code, e.Message)
}
