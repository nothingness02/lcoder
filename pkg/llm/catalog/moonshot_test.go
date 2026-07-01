// pkg/llm/catalog/moonshot_test.go
package catalog

import "testing"

// k2Entry mirrors the K2.7 Code record models.dev files under the "moonshotai"
// provider key (context/output limits and USD-per-1M pricing).
func k2Entry() Entry {
	e := Entry{
		ID:            "kimi-k2.7-code",
		Name:          "Kimi K2.7 Code",
		Provider:      "moonshotai",
		ContextWindow: 262144,
		MaxOutput:     262144,
		Capabilities:  []string{"tools", "reasoning"},
	}
	e.Cost.Prompt = 0.95
	e.Cost.Completion = 4
	return e
}

// Lcoder surfaces the provider as "moonshot", but models.dev keys the same model
// as "moonshotai". A lookup by the canonical picker name must resolve the
// upstream record through the provider alias.
func TestMoonshotAliasResolvesK2(t *testing.T) {
	c := New(Options{Refresh: false, Overrides: []Entry{k2Entry()}})

	if w := c.Window("moonshot", "kimi-k2.7-code"); w != 262144 {
		t.Errorf("K2.7 window via moonshot alias = %d, want 262144", w)
	}
	if o := c.MaxOutput("moonshot", "kimi-k2.7-code"); o != 262144 {
		t.Errorf("K2.7 max output via moonshot alias = %d, want 262144", o)
	}

	// The upstream (models.dev) provider name keeps resolving directly.
	if w := c.Window("moonshotai", "kimi-k2.7-code"); w != 262144 {
		t.Errorf("K2.7 window via moonshotai = %d, want 262144", w)
	}
}

// Cost lookups keyed by the picker name ("moonshot/...") must find the mirrored
// models.dev pricing so cost estimation works for the running agent.
func TestMoonshotAliasPricing(t *testing.T) {
	c := New(Options{Refresh: false, Overrides: []Entry{k2Entry()}})
	pt := c.PriceTable()

	p, ok := pt["moonshot/kimi-k2.7-code"]
	if !ok {
		t.Fatalf("price table missing mirrored moonshot/kimi-k2.7-code")
	}
	if p.Prompt != 0.95 || p.Completion != 4 {
		t.Errorf("K2.7 mirrored price = %+v, want prompt=0.95 completion=4", p)
	}
	// The upstream key is present too.
	if _, ok := pt["moonshotai/kimi-k2.7-code"]; !ok {
		t.Fatalf("price table missing moonshotai/kimi-k2.7-code")
	}
}
