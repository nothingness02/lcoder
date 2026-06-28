// pkg/llm/turnstream.go
package llm

import "context"

// TurnStream yields normalized gateway events from an in-process engine stream.
type TurnStream struct {
	ch   <-chan GatewayEvent
	done bool
}

// Next returns the next event, or ok=false when the stream is exhausted.
func (s *TurnStream) Next(ctx context.Context) (GatewayEvent, bool, error) {
	if s.done {
		return GatewayEvent{}, false, nil
	}
	select {
	case <-ctx.Done():
		return GatewayEvent{}, false, ctx.Err()
	case ev, ok := <-s.ch:
		if !ok {
			s.done = true
			return GatewayEvent{}, false, nil
		}
		if ev.Name == "done" || ev.Name == "error" {
			s.done = true
		}
		return ev, true, nil
	}
}

// Close is a no-op for channel-backed streams (kept for interface compatibility).
func (s *TurnStream) Close() error { return nil }
