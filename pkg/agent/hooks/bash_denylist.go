package hooks

import (
	"context"
	"fmt"
	"strings"

	"github.com/lcoder/lcoder/pkg/agent"
)

// BashDenylist blocks bash commands matching dangerous patterns.
func BashDenylist(patterns []string) agent.BeforeToolCallHook {
	return func(ctx context.Context, info agent.ToolCallInfo) (*agent.BeforeToolCallResult, error) {
		if info.ToolCall.Name != "bash" {
			return nil, nil
		}
		cmd, _ := info.Args["command"].(string)
		cmd = strings.ToLower(cmd)
		for _, pattern := range patterns {
			pattern = strings.ToLower(pattern)
			if strings.Contains(cmd, pattern) {
				return &agent.BeforeToolCallResult{
					Block:  true,
					Reason: fmt.Sprintf("bash command matches denylist pattern: %q", pattern),
				}, nil
			}
		}
		return nil, nil
	}
}
