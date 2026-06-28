package hooks

import (
	"context"
	"fmt"

	"github.com/lcoder/lcoder/pkg/agent"
)

// CompositeBeforeToolCall runs multiple hooks in order.
// The first non-nil blocking result wins; errors short-circuit.
func CompositeBeforeToolCall(hooks ...agent.BeforeToolCallHook) agent.BeforeToolCallHook {
	return func(ctx context.Context, info agent.ToolCallInfo) (*agent.BeforeToolCallResult, error) {
		for _, h := range hooks {
			if h == nil {
				continue
			}
			result, err := h(ctx, info)
			if err != nil {
				return nil, fmt.Errorf("hook error: %w", err)
			}
			if result != nil && result.Block {
				return result, nil
			}
		}
		return nil, nil
	}
}
