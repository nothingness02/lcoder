// Package config defines Lcoder configuration types and loading.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	koanf "github.com/knadh/koanf/v2"
)

// HTTPToolConfig describes an external HTTP tool.
type HTTPToolConfig struct {
	Name          string            `yaml:"name"`
	Endpoint      string            `yaml:"endpoint"`
	Description   string            `yaml:"description"`
	Parameters    map[string]any    `yaml:"parameters"`
	ExecutionMode string            `yaml:"execution_mode"`
	Headers       map[string]string `yaml:"headers"`
}

// MCPServerConfig describes a stdio MCP server.
type MCPServerConfig struct {
	Name    string            `yaml:"name"`
	Command []string          `yaml:"command"`
	Env     map[string]string `yaml:"env"`
}

// PermissionConfig holds permission rules per tool.
type PermissionConfig struct {
	Rules map[string]map[string]string `yaml:"rules"`
}

// SandboxConfig configures the sandbox backend wiring tools at startup.
type SandboxConfig struct {
	Backend      string                  `yaml:"backend"` // "" -> passthrough
	EnvAllowlist []string                `yaml:"env_allowlist"`
	Network      SandboxNetworkConfig    `yaml:"network"`
	Filesystem   SandboxFilesystemConfig `yaml:"filesystem"`
	Limits       SandboxLimitsConfig     `yaml:"limits"`
}

// SandboxNetworkConfig is the yaml form of the network allowlist.
type SandboxNetworkConfig struct {
	Default string   `yaml:"default"` // "deny" | "allow"
	Allow   []string `yaml:"allow"`
}

// SandboxFilesystemConfig lists allowed roots (relative to project root).
type SandboxFilesystemConfig struct {
	Readable []string `yaml:"readable"`
	Writable []string `yaml:"writable"`
}

// SandboxLimitsConfig is the yaml form of resource limits.
type SandboxLimitsConfig struct {
	MaxMemoryMB    int `yaml:"max_memory_mb"`
	MaxCPUSeconds  int `yaml:"max_cpu_seconds"`
	MaxOutputBytes int `yaml:"max_output_bytes"`
}

// TUIConfig holds TUI-specific settings.
type TUIConfig struct {
	Theme string `yaml:"theme"`
}

// ContextConfig controls structured context manager behavior.
type ContextConfig struct {
	Mode             string  `yaml:"mode"`              // "auto", "manual", "off"
	MaxTokens        int     `yaml:"max_tokens"`        // hard context budget
	TargetTokens     int     `yaml:"target_tokens"`     // soft target budget
	ReserveOutput    int     `yaml:"reserve_output"`    // output reservation
	StaticRatio      int     `yaml:"static_ratio"`      // ratio percentage for static/stable blocks
	MinRecent        int     `yaml:"min_recent"`        // minimum recent messages to keep
	AutoCompact      bool    `yaml:"auto_compact"`      // enable automatic compaction
	CompactThreshold float64 `yaml:"compact_threshold"` // ratio of target at which compaction starts
	CacheHintPolicy  string  `yaml:"cache_hint_policy"` // "default", "aggressive", "none"
	DeferredTools    bool    `yaml:"deferred_tools"`    // ship only core tools + tool_search
	CoreTools        []string `yaml:"core_tools"`       // tools kept full under deferral
	DropThreshold    float64 `yaml:"drop_threshold"`    // ratio of effective input at which old msgs drop
}

// Config is the full Lcoder configuration.
type Config struct {
	Provider    string                  `yaml:"provider"`
	Model       string                  `yaml:"model"`
	TUI         TUIConfig               `yaml:"tui"`
	Context     ContextConfig           `yaml:"context"`
	Permissions PermissionConfig        `yaml:"permissions"`
	HTTPTools   []HTTPToolConfig        `yaml:"http_tools"`
	MCPServers  []MCPServerConfig       `yaml:"mcp_servers"`
	Hooks       HookConfig              `yaml:"hooks"`
	Extensions  []ExtensionConfig       `yaml:"extensions"`
	Packages    []PackageConfig         `yaml:"packages"`
	Providers   map[string]ProviderConn `yaml:"providers"`
	Sandbox     SandboxConfig           `yaml:"sandbox"`

	// Catalog is the shared model metadata loaded from models.yaml (not parsed
	// from the main config file). ModelsConfigPath is its resolved location.
	Catalog          ModelCatalog `yaml:"-"`
	ModelsConfigPath string       `yaml:"-"`
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Provider: "openai",
		Model:    "gpt-4o-mini",
		TUI:      TUIConfig{Theme: "dark"},
		Context: ContextConfig{
			Mode:             "auto",
			MaxTokens:        0, // 0 = unset; resolved from catalog/engine at runtime
			TargetTokens:     0, // 0 = unset; derived from MaxTotal when missing
			ReserveOutput:    0, // 0 = unset; falls back to defaultReserveOutput
			StaticRatio:      60,
			MinRecent:        10,
			AutoCompact:      true,
			CompactThreshold: 0.9,
			CacheHintPolicy:  "default",
			DeferredTools:    false,
			CoreTools:        nil,
			DropThreshold:    1.0,
		},
		Permissions: PermissionConfig{
			Rules: map[string]map[string]string{
				"read":  {"*": "allow"},
				"write": {"*": "allow"},
				"edit":  {"*": "allow"},
				"ls":    {"*": "allow"},
				"grep":  {"*": "allow"},
				"find":  {"*": "allow"},
				"bash": {
					"*":         "ask",
					"git *":     "allow",
					"go test *": "allow",
				},
			},
		},
	}
}

// Budget resolution fallbacks, used only when no explicit user/catalog/discovered
// value is available for the configured model.
const (
	fallbackMaxTokens    = 128000
	defaultReserveOutput = 8192
	defaultTargetRatio   = 0.9
)

// ResolveContextBudget returns the effective context budget for the configured
// model, plus the source that determined MaxTotal ("user", "catalog",
// "discovered", or "default"). discoveredWindow is the window looked up from the
// LLM engine/catalog at startup (0 if unknown); pass 0 for fully offline resolution.
//
// Priority per field:
//
//	MaxTotal:      user context.max_tokens > catalog context_window > discovered window > fallback
//	ReserveOutput: user context.reserve_output > catalog budget.reserve_output > default
//	TargetTotal:   user context.target_tokens > catalog budget.target > MaxTotal * ratio
func (c Config) ResolveContextBudget(discoveredWindow int) (TokenBudget, string) {
	cfg := c.Context
	meta, hasMeta := c.Catalog.Lookup(c.Model)

	// MaxTotal.
	maxTotal := cfg.MaxTokens
	source := "user"
	if maxTotal <= 0 && hasMeta && meta.ContextWindow > 0 {
		maxTotal = meta.ContextWindow
		source = "catalog"
	}
	if maxTotal <= 0 && discoveredWindow > 0 {
		maxTotal = discoveredWindow
		source = "discovered"
	}
	if maxTotal <= 0 {
		maxTotal = fallbackMaxTokens
		source = "default"
	}

	// ReserveOutput.
	reserve := cfg.ReserveOutput
	if reserve <= 0 && hasMeta && meta.Budget.ReserveOutput > 0 {
		reserve = meta.Budget.ReserveOutput
	}
	if reserve <= 0 {
		reserve = defaultReserveOutput
	}

	// TargetTotal.
	target := cfg.TargetTokens
	if target <= 0 && hasMeta && meta.Budget.Target > 0 {
		target = meta.Budget.Target
	}
	if target <= 0 || target > maxTotal {
		target = int(float64(maxTotal) * defaultTargetRatio)
	}

	threshold := cfg.CompactThreshold
	if threshold <= 0 || threshold > 1 {
		threshold = 0.9
	}

	dropThreshold := cfg.DropThreshold
	if dropThreshold <= 0 || dropThreshold > 1 {
		dropThreshold = 1.0
	}

	return TokenBudget{
		MaxTotal:         maxTotal,
		TargetTotal:      target,
		ReserveOutput:    reserve,
		CompactThreshold: threshold,
		DropThreshold:    dropThreshold,
	}, source
}

// TokenBudget is the resolved context window budget.
type TokenBudget struct {
	MaxTotal         int
	TargetTotal      int
	ReserveOutput    int
	CompactThreshold float64
	DropThreshold    float64
}

// Load reads configuration from standard locations.
func Load() (Config, error) {
	k := koanf.NewWithConf(koanf.Conf{
		Delim:       ".",
		StrictMerge: false,
	})

	cfg := DefaultConfig()
	_ = k.Load(confmap.Provider(map[string]any{
		"provider":  cfg.Provider,
		"model":     cfg.Model,
		"tui.theme": cfg.TUI.Theme,
		"context": map[string]any{
			"mode":              cfg.Context.Mode,
			"max_tokens":        cfg.Context.MaxTokens,
			"target_tokens":     cfg.Context.TargetTokens,
			"reserve_output":    cfg.Context.ReserveOutput,
			"static_ratio":      cfg.Context.StaticRatio,
			"min_recent":        cfg.Context.MinRecent,
			"auto_compact":      cfg.Context.AutoCompact,
			"compact_threshold": cfg.Context.CompactThreshold,
			"cache_hint_policy": cfg.Context.CacheHintPolicy,
			"deferred_tools":    cfg.Context.DeferredTools,
			"core_tools":        cfg.Context.CoreTools,
			"drop_threshold":    cfg.Context.DropThreshold,
		},
	}, "."), nil)

	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}

	paths := []string{
		filepath.Join(home, ".lcoder", "config.yaml"),
		filepath.Join(home, ".lcoder", "config.yml"),
		"lcoder.yaml",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			if err := k.Load(file.Provider(p), yaml.Parser()); err != nil {
				return cfg, fmt.Errorf("load config %s: %w", p, err)
			}
		}
	}

	_ = k.Load(env.Provider("LCODER_", ".", func(s string) string {
		return s[len("LCODER_"):]
	}), nil)

	if err := k.Unmarshal("", &cfg); err != nil {
		return cfg, fmt.Errorf("unmarshal config: %w", err)
	}

	// Fold TUI-managed credentials (~/.lcoder/credentials.yaml) into providers,
	// without overriding hand-written config.providers fields.
	if credPath := resolveCredentialsPath(); credPath != "" {
		if creds, err := LoadCredentials(credPath); err == nil {
			cfg.Providers = mergeCredentials(cfg.Providers, creds)
		} else {
			fmt.Fprintf(os.Stderr, "warning: 读取 credentials 失败,已忽略: %v\n", err)
		}
	}

	// Expand {env:VAR} references in provider connection settings.
	cfg.Providers = resolveProviders(cfg.Providers)

	// Fold the shared model catalog (models.yaml) into the config when present,
	// so context budgets and capabilities come from a single source of truth.
	// ResolveContextBudget reads catalog windows directly via Catalog.Lookup.
	if cat, path, ok := LoadModelCatalog(); ok {
		cfg.Catalog = cat
		cfg.ModelsConfigPath = path
	}
	return cfg, nil
}
