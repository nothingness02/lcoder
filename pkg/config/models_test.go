package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadModelCatalogFrom(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "models.yaml")
	yaml := `models:
  - id: gpt-4o-mini
    provider: openai
    aliases: [4o-mini]
    capabilities: [tools, vision, streaming]
    context_window: 128000
    budget:
      target: 120000
      reserve_output: 8192
    pricing:
      prompt: 0.15
      completion: 0.60
  - id: claude-sonnet-4-20250514
    provider: anthropic
    aliases: [sonnet]
    capabilities: [tools, streaming]
    context_window: 200000
`
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	cat, err := LoadModelCatalogFrom(path)
	if err != nil {
		t.Fatalf("LoadModelCatalogFrom: %v", err)
	}
	if len(cat.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(cat.Models))
	}

	// Lookup by exact id.
	m, ok := cat.Lookup("gpt-4o-mini")
	if !ok || m.ContextWindow != 128000 || m.Budget.Target != 120000 {
		t.Fatalf("lookup by id failed: %+v ok=%v", m, ok)
	}
	if !m.HasCapability("vision") || m.HasCapability("reasoning") {
		t.Fatalf("capability check wrong: %v", m.Capabilities)
	}

	// Lookup by alias.
	if _, ok := cat.Lookup("sonnet"); !ok {
		t.Fatal("lookup by alias failed")
	}

	// Lookup by prefix (dated variant).
	if _, ok := cat.Lookup("claude-sonnet-4"); !ok {
		t.Fatal("lookup by prefix failed")
	}

	// Missing.
	if _, ok := cat.Lookup("no-such-model"); ok {
		t.Fatal("expected miss")
	}
}

func TestLoadModelCatalogEnvOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.yaml")
	if err := os.WriteFile(path, []byte("models:\n  - id: m1\n    provider: p1\n    context_window: 1000\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LCODER_MODELS_CONFIG", path)

	cat, resolved, ok := LoadModelCatalog()
	if !ok {
		t.Fatal("expected catalog found via env")
	}
	if resolved != path {
		t.Fatalf("expected resolved path %q, got %q", path, resolved)
	}
	if len(cat.Models) != 1 || cat.Models[0].ID != "m1" {
		t.Fatalf("unexpected catalog: %+v", cat.Models)
	}
}

// The catalog feeds context budgets: a model's context_window/budget drives
// ResolveContextBudget directly via Catalog.Lookup.
func TestCatalogDrivesContextBudget(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Model = "claude-sonnet-4-20250514"
	cfg.Catalog = ModelCatalog{Models: []ModelMeta{{
		ID:            "claude-sonnet-4-20250514",
		Provider:      "anthropic",
		ContextWindow: 200000,
		Budget:        ModelBudget{Target: 180000, ReserveOutput: 8192},
	}}}

	b, source := cfg.ResolveContextBudget(0, 0)
	if source != "catalog" {
		t.Fatalf("expected source catalog, got %q", source)
	}
	if b.MaxTotal != 200000 || b.TargetTotal != 180000 {
		t.Fatalf("expected budget from catalog, got %+v", b)
	}
}

func TestConfigModelMeta(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Catalog = ModelCatalog{Models: []ModelMeta{
		{ID: "gpt-4o", Provider: "openai", Capabilities: []string{"tools", "vision"}},
		{ID: "deepseek-reasoner", Provider: "deepseek", Capabilities: []string{"streaming"}},
	}}

	cfg.Model = "gpt-4o"
	meta, ok := cfg.ModelMeta()
	if !ok || !meta.HasCapability("tools") {
		t.Fatalf("expected tools-capable meta for gpt-4o, got %+v ok=%v", meta, ok)
	}
	if cfg.ModelLacksTools() {
		t.Fatal("gpt-4o should not be flagged as lacking tools")
	}

	cfg.Model = "deepseek-reasoner"
	if !cfg.ModelLacksTools() {
		t.Fatal("deepseek-reasoner declares no tools capability and should be flagged")
	}

	// Unknown model: no meta, and we do not flag (avoid false warnings).
	cfg.Model = "mystery"
	if _, ok := cfg.ModelMeta(); ok {
		t.Fatal("expected miss for unknown model")
	}
	if cfg.ModelLacksTools() {
		t.Fatal("unknown model should not be flagged as lacking tools")
	}
}
