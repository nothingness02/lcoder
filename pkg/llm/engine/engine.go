// pkg/llm/engine/engine.go
package engine

import (
	"context"

	"github.com/lcoder/lcoder/pkg/llm/catalog"
	"github.com/lcoder/lcoder/pkg/llm/pricing"
	"github.com/lcoder/lcoder/pkg/llm/provider"
	"github.com/lcoder/lcoder/pkg/models"
)

// AdapterFactory builds an adapter for a route, given precomputed cache marks.
type AdapterFactory func(route string, marks provider.CacheMarks) provider.Adapter

// Engine routes turns to provider adapters in-process.
type Engine struct {
	providers  map[string]provider.Conn
	catalog    *catalog.Catalog
	newAdapter AdapterFactory
}

// New builds an engine over a catalog with the default adapter factory.
func New(cat *catalog.Catalog) *Engine {
	return &Engine{
		providers:  map[string]provider.Conn{},
		catalog:    cat,
		newAdapter: defaultAdapterFactory,
	}
}

func defaultAdapterFactory(route string, marks provider.CacheMarks) provider.Adapter {
	if route == "anthropic" {
		return provider.Anthropic{Marks: marks}
	}
	return provider.OpenAICompat{}
}

// SetAdapterFactory overrides adapter construction (used by tests / llmtest).
func (e *Engine) SetAdapterFactory(f AdapterFactory) { e.newAdapter = f }

// RegisterProvider stores or replaces an in-memory provider connection.
func (e *Engine) RegisterProvider(name string, conn provider.Conn) {
	e.providers[name] = conn
}

// ListModels returns the catalog's model list.
func (e *Engine) ListModels() []models.ModelInfo { return e.catalog.List() }

// ModelWindow returns the catalog context window for provider/model (0 if unknown).
func (e *Engine) ModelWindow(prov, model string) int { return e.catalog.Window(prov, model) }

// ModelMaxOutput returns the catalog single-response output ceiling for
// provider/model (0 if unknown).
func (e *Engine) ModelMaxOutput(prov, model string) int { return e.catalog.MaxOutput(prov, model) }

func (e *Engine) resolveProvider(ref models.ModelRef) string {
	if ref.Provider != "" {
		return ref.Provider
	}
	for _, m := range e.catalog.List() {
		if m.ID == ref.ID {
			return m.Provider
		}
	}
	return ""
}

// StreamTurn selects an adapter, starts the provider stream, and returns a
// channel of normalized events with cost filled in on the done event.
func (e *Engine) StreamTurn(ctx context.Context, req models.TurnRequest) (<-chan provider.Event, error) {
	prov := e.resolveProvider(req.Model)
	conn := e.providers[prov]
	if conn.Route == "" {
		conn.Route = prov
	}
	anthropic := conn.Route == "anthropic"
	marks := provider.ComputeCacheMarks(req.Cache, req.CacheBreakpoints, len(req.Messages), anthropic)
	conn.BaseURL = provider.ResolveBaseURL(conn)

	adapter := e.newAdapter(conn.Route, marks)
	src, err := adapter.Stream(ctx, conn, req)
	if err != nil {
		return nil, err
	}
	out := make(chan provider.Event)
	go e.forward(prov, req.Model.ID, src, out)
	return out, nil
}

// forward copies events through, computing cost on the done event.
func (e *Engine) forward(prov, model string, src <-chan provider.Event, out chan<- provider.Event) {
	defer close(out)
	table := e.catalog.PriceTable()
	for ev := range src {
		if ev.Kind == provider.KindDone && ev.Usage != nil {
			u := ev.Usage
			u.Provider = prov
			u.Model = model
			cb := pricing.EstimateCost(table, prov, model,
				u.PromptTokens, u.CompletionTokens, u.CacheReadTokens, u.CacheWriteTokens)
			u.PromptCost = cb.PromptCost
			u.CompletionCost = cb.CompletionCost
			u.CacheReadCost = cb.CacheReadCost
			u.CacheWriteCost = cb.CacheWriteCost
			u.TotalCost = cb.TotalCost
		}
		out <- ev
	}
}
