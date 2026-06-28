package events

import (
	"context"
	"fmt"
	"sync"
)

// Handler processes agent events.
type Handler func(ctx context.Context, event Event) error

// Bus broadcasts events to registered handlers in order.
type Bus struct {
	handlers []Handler
	mu       sync.RWMutex
}

// New creates an event bus.
func New() *Bus {
	return &Bus{}
}

// Subscribe registers a handler. The returned function unsubscribes it.
func (b *Bus) Subscribe(handler Handler) func() {
	b.mu.Lock()
	defer b.mu.Unlock()
	idx := len(b.handlers)
	b.handlers = append(b.handlers, handler)
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if idx >= len(b.handlers) {
			return
		}
		b.handlers = append(b.handlers[:idx], b.handlers[idx+1:]...)
	}
}

// Emit synchronously dispatches an event to all handlers.
// Handlers are invoked in registration order. If a handler returns an error,
// subsequent handlers still run, and the first error is returned.
func (b *Bus) Emit(ctx context.Context, event Event) error {
	b.mu.RLock()
	handlers := make([]Handler, len(b.handlers))
	copy(handlers, b.handlers)
	b.mu.RUnlock()

	var firstErr error
	for _, h := range handlers {
		if err := h(ctx, event); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("event handler failed for %s: %w", event.EventType(), err)
		}
	}
	return firstErr
}
