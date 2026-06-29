package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lcoder/lcoder/pkg/agent"
	"github.com/lcoder/lcoder/pkg/agent/hooks"
	"github.com/lcoder/lcoder/pkg/compaction"
	"github.com/lcoder/lcoder/pkg/config"
	"github.com/lcoder/lcoder/pkg/events"
	"github.com/lcoder/lcoder/pkg/llm/catalog"
	"github.com/lcoder/lcoder/pkg/llm/engine"
	llmprovider "github.com/lcoder/lcoder/pkg/llm/provider"
	"github.com/lcoder/lcoder/pkg/models"
	"github.com/lcoder/lcoder/pkg/permissions"
)

// buildEngine constructs the in-process LLM engine: a model catalog (snapshot +
// background refresh + models.yaml overrides) plus every configured provider
// connection. The returned engine is passed to llm.NewClient.
func buildEngine(cfg config.Config) *engine.Engine {
	cachePath := ""
	if home, err := os.UserHomeDir(); err == nil {
		cachePath = filepath.Join(home, ".lcoder", "cache", "models.json")
	}
	cat := catalog.New(catalog.Options{
		Refresh:   true,
		CachePath: cachePath,
		Overrides: catalogOverridesFromConfig(cfg),
	})
	eng := engine.New(cat)
	for name, conn := range cfg.Providers {
		eng.RegisterProvider(name, llmprovider.Conn{
			BaseURL: conn.BaseURL,
			APIKey:  conn.APIKey,
			Route:   conn.Route,
			Headers: conn.Headers,
		})
	}
	return eng
}

// catalogOverridesFromConfig maps the user's models.yaml catalog entries into
// explicit catalog overrides so locally-declared models take priority over the
// snapshot/models.dev data.
func catalogOverridesFromConfig(cfg config.Config) []catalog.Entry {
	out := make([]catalog.Entry, 0, len(cfg.Catalog.Models))
	for _, m := range cfg.Catalog.Models {
		out = append(out, catalog.Entry{
			ID:            m.ID,
			Provider:      m.Provider,
			ContextWindow: m.ContextWindow,
			Capabilities:  m.Capabilities,
		})
	}
	return out
}

// todo:根据系统优化内置的系统提示词
func makeTransformContext(keep int) agent.TransformContext {
	strategy := compaction.NewKeepLastStrategy(keep)
	return func(ctx context.Context, messages []models.AgentMessage) ([]models.AgentMessage, error) {
		if len(messages) <= keep+1 {
			return messages, nil
		}
		if len(messages)%compactionInterval == 0 {
			return strategy.Compact(messages, compaction.SimpleSummarize)
		}
		return messages, nil
	}
}

func makeBeforeToolCall(bus *events.Bus, engine *permissions.Engine, hookCfg config.HookConfig) agent.BeforeToolCallHook {
	declarative := hooks.FromConfig(hookCfg)
	permissionHook := func(ctx context.Context, info agent.ToolCallInfo) (*agent.BeforeToolCallResult, error) {
		decision := engine.Decide(info.ToolCall.Name, info.Args)

		// Emit audit event for every permission decision.
		var blocked bool
		var blockReason string
		allowed := decision == permissions.Allow
		if decision == permissions.Deny {
			blocked = true
			blockReason = "denied by policy"
		}

		_ = bus.Emit(ctx, events.AuditEvent{
			Base:        events.Base{Type: events.Audit},
			ToolCallID:  info.ToolCall.ID,
			ToolName:    info.ToolCall.Name,
			Args:        info.Args,
			Decision:    string(decision),
			Allowed:     allowed,
			Blocked:     blocked,
			BlockReason: blockReason,
		})

		switch decision {
		case permissions.Allow:
			return nil, nil
		case permissions.Deny:
			return &agent.BeforeToolCallResult{Block: true, Reason: blockReason}, nil
		case permissions.Ask:
			result, err := askUser(ctx, info)
			if err != nil {
				return nil, err
			}
			// Emit follow-up audit event reflecting the interactive decision.
			askBlockReason := ""
			if result != nil {
				askBlockReason = result.Reason
			}
			_ = bus.Emit(ctx, events.AuditEvent{
				Base:        events.Base{Type: events.Audit},
				ToolCallID:  info.ToolCall.ID,
				ToolName:    info.ToolCall.Name,
				Args:        info.Args,
				Decision:    "ask",
				Allowed:     result == nil || !result.Block,
				Blocked:     result != nil && result.Block,
				BlockReason: askBlockReason,
			})
			return result, nil
		default:
			return nil, nil
		}
	}

	return hooks.CompositeBeforeToolCall(declarative, permissionHook)
}

func askUser(ctx context.Context, info agent.ToolCallInfo) (*agent.BeforeToolCallResult, error) {
	fmt.Fprintf(os.Stderr, "\nPermission request: %s(%s)\nAllow? [y/N] ", info.ToolCall.Name, formatArgs(info.Args))
	var line string
	fmt.Fscanln(os.Stdin, &line)
	if strings.EqualFold(strings.TrimSpace(line), "y") {
		return nil, nil
	}
	return &agent.BeforeToolCallResult{Block: true, Reason: "user denied"}, nil
}

func formatArgs(args map[string]any) string {
	if len(args) == 0 {
		return ""
	}
	var parts []string
	for k, v := range args {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return strings.Join(parts, ", ")
}

func parsePermissionConfig(pc config.PermissionConfig) []permissions.Rule {
	var rules []permissions.Rule
	for tool, patterns := range pc.Rules {
		for pattern, decision := range patterns {
			rules = append(rules, permissions.Rule{
				Tool:     tool,
				Pattern:  pattern,
				Decision: permissions.Decision(decision),
			})
		}
	}
	return rules
}
