package config

import (
	"testing"
)

// catalogFor builds a Config whose catalog declares one model with the given
// window/budget, useful for exercising the catalog layer of the priority chain.
func catalogFor(model string, window, target, reserve int) Config {
	cfg := DefaultConfig()
	cfg.Model = model
	cfg.Catalog = ModelCatalog{Models: []ModelMeta{{
		ID:            model,
		ContextWindow: window,
		Budget:        ModelBudget{Target: target, ReserveOutput: reserve},
	}}}
	return cfg
}

// With no user config, no catalog, and no litellm window, resolution falls back
// to the hard defaults and reports source "default".
func TestResolveContextBudgetDefaultFallback(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Model = "mystery-model"
	budget, source := cfg.ResolveContextBudget(0)

	if source != "default" {
		t.Fatalf("expected source default, got %q", source)
	}
	if budget.MaxTotal != fallbackMaxTokens {
		t.Fatalf("expected max %d, got %d", fallbackMaxTokens, budget.MaxTotal)
	}
	if budget.ReserveOutput != defaultReserveOutput {
		t.Fatalf("expected reserve %d, got %d", defaultReserveOutput, budget.ReserveOutput)
	}
	if budget.TargetTotal != int(float64(fallbackMaxTokens)*defaultTargetRatio) {
		t.Fatalf("expected derived target, got %d", budget.TargetTotal)
	}
}

// A litellm-discovered window is used when neither user nor catalog supplies one.
func TestResolveContextBudgetLitellmWindow(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Model = "mystery-model"
	budget, source := cfg.ResolveContextBudget(256000)

	if source != "litellm" {
		t.Fatalf("expected source litellm, got %q", source)
	}
	if budget.MaxTotal != 256000 {
		t.Fatalf("expected max 256000, got %d", budget.MaxTotal)
	}
	// target derived from the litellm window, reserve from default.
	if budget.TargetTotal != int(256000*defaultTargetRatio) {
		t.Fatalf("expected derived target, got %d", budget.TargetTotal)
	}
	if budget.ReserveOutput != defaultReserveOutput {
		t.Fatalf("expected reserve %d, got %d", defaultReserveOutput, budget.ReserveOutput)
	}
}

// The catalog window overrides litellm (explicit per-model override).
func TestResolveContextBudgetCatalogOverridesLitellm(t *testing.T) {
	cfg := catalogFor("claude-sonnet-4-20250514", 200000, 180000, 8192)
	budget, source := cfg.ResolveContextBudget(999999)

	if source != "catalog" {
		t.Fatalf("expected source catalog, got %q", source)
	}
	if budget.MaxTotal != 200000 || budget.TargetTotal != 180000 || budget.ReserveOutput != 8192 {
		t.Fatalf("expected catalog budget, got %+v", budget)
	}
}

// User global config wins over both catalog and litellm.
func TestResolveContextBudgetUserOverrides(t *testing.T) {
	cfg := catalogFor("claude-sonnet-4-20250514", 200000, 180000, 8192)
	cfg.Context.MaxTokens = 64000
	cfg.Context.TargetTokens = 60000
	cfg.Context.ReserveOutput = 4096

	budget, source := cfg.ResolveContextBudget(999999)
	if source != "user" {
		t.Fatalf("expected source user, got %q", source)
	}
	if budget.MaxTotal != 64000 || budget.TargetTotal != 60000 || budget.ReserveOutput != 4096 {
		t.Fatalf("expected user budget, got %+v", budget)
	}
}

// When only the catalog window is known (no budget target), the target is
// derived from the window and reserve falls back to the default.
func TestResolveContextBudgetCatalogWindowOnly(t *testing.T) {
	cfg := catalogFor("local-model", 100000, 0, 0)
	budget, source := cfg.ResolveContextBudget(0)

	if source != "catalog" {
		t.Fatalf("expected source catalog, got %q", source)
	}
	if budget.MaxTotal != 100000 {
		t.Fatalf("expected max 100000, got %d", budget.MaxTotal)
	}
	if budget.TargetTotal != int(100000*defaultTargetRatio) {
		t.Fatalf("expected derived target, got %d", budget.TargetTotal)
	}
	if budget.ReserveOutput != defaultReserveOutput {
		t.Fatalf("expected reserve %d, got %d", defaultReserveOutput, budget.ReserveOutput)
	}
}

// A user target larger than the resolved max is clamped to the derived ratio.
func TestResolveContextBudgetTargetClamped(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Context.MaxTokens = 100000
	cfg.Context.TargetTokens = 200000 // invalid: exceeds max
	budget, _ := cfg.ResolveContextBudget(0)

	if budget.TargetTotal >= budget.MaxTotal {
		t.Fatalf("expected target < max, got target=%d max=%d", budget.TargetTotal, budget.MaxTotal)
	}
}

func TestResolveContextBudgetCompactThresholdDefault(t *testing.T) {
	cfg := DefaultConfig()
	budget, _ := cfg.ResolveContextBudget(0)
	if budget.CompactThreshold != 0.9 {
		t.Fatalf("expected default compact threshold 0.9, got %v", budget.CompactThreshold)
	}
}

func TestResolveContextBudgetCompactThresholdOverride(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Context.CompactThreshold = 0.75
	budget, _ := cfg.ResolveContextBudget(0)
	if budget.CompactThreshold != 0.75 {
		t.Fatalf("expected compact threshold 0.75, got %v", budget.CompactThreshold)
	}
}

func TestResolveContextBudgetCompactThresholdInvalidFallback(t *testing.T) {
	for _, bad := range []float64{0, -0.5, 1.5} {
		cfg := DefaultConfig()
		cfg.Context.CompactThreshold = bad
		budget, _ := cfg.ResolveContextBudget(0)
		if budget.CompactThreshold != 0.9 {
			t.Fatalf("expected fallback 0.9 for invalid %v, got %v", bad, budget.CompactThreshold)
		}
	}
}
