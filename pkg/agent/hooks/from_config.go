package hooks

import (
	"github.com/lcoder/lcoder/pkg/agent"
	"github.com/lcoder/lcoder/pkg/config"
)

// FromConfig builds a composite BeforeToolCallHook from declarative config.
func FromConfig(cfg config.HookConfig) agent.BeforeToolCallHook {
	var hooks []agent.BeforeToolCallHook
	if cfg.SensitiveFileCheck.Enabled && len(cfg.SensitiveFileCheck.Patterns) > 0 {
		hooks = append(hooks, SensitiveFileCheck(cfg.SensitiveFileCheck.Patterns))
	}
	if cfg.BashDenylist.Enabled && len(cfg.BashDenylist.Patterns) > 0 {
		hooks = append(hooks, BashDenylist(cfg.BashDenylist.Patterns))
	}
	return CompositeBeforeToolCall(hooks...)
}
