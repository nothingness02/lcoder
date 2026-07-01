// pkg/llm/client.go
package llm

import (
	"context"
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

// mapEvent translates a provider.Event into a GatewayEvent.
func mapEvent(ev provider.Event) GatewayEvent {
	return GatewayEvent{Event: ev}
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

// ModelMaxOutput returns the catalog single-response output ceiling for
// provider/model (0 if unknown).
func (c *Client) ModelMaxOutput(ctx context.Context, prov, model string) (int, error) {
	return c.engine.ModelMaxOutput(prov, model), nil
}

// Health reports in-process readiness.
func (c *Client) Health(ctx context.Context) (map[string]string, error) {
	return map[string]string{"status": "ok"}, nil
}

// GatewayEvent is a normalized event from the engine. It wraps the strongly-typed
// provider.Event so consumers can read typed fields directly instead of using
// map[string]any payloads.
type GatewayEvent struct {
	provider.Event
}

// Type returns the event type string (start, text_delta, etc.).
func (e GatewayEvent) Type() string {
	return e.Kind.String()
}

// Usage extracts LLM usage from a "done" event if present.
func (e GatewayEvent) Usage() (models.LLMUsage, bool) {
	if e.Kind != provider.KindDone || e.Event.Usage == nil {
		return models.LLMUsage{}, false
	}
	return *e.Event.Usage, true
}

// FinalMessage extracts the final assistant message from a "done" event.
func (e GatewayEvent) FinalMessage() (models.AgentMessage, error) {
	if e.Kind != provider.KindDone {
		return models.AgentMessage{}, fmt.Errorf("final message only available on done events")
	}
	return e.Event.Message, nil
}

// Error extracts a GatewayError from an "error" event.
func (e GatewayEvent) Error() (GatewayError, bool) {
	if e.Kind != provider.KindError || e.Event.Err == nil {
		return GatewayError{}, false
	}
	return GatewayError{
		Code:          e.Event.Err.Code,
		Message:       e.Event.Err.Message,
		ProviderError: e.Event.Err.ProviderError,
	}, true
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
