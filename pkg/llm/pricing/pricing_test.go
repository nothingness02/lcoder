// pkg/llm/pricing/pricing_test.go
package pricing

import (
	"math"
	"testing"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestEstimateCostKnownModel(t *testing.T) {
	c := EstimateCost(map[string]ModelPrice{
		"openai/gpt-4o": {Prompt: 2.50, Completion: 10.00, CacheRead: 1.25, CacheWrite: 2.50},
	}, "openai", "gpt-4o", 1_000_000, 500_000, 0, 0)
	if !approx(c.PromptCost, 2.50) || !approx(c.CompletionCost, 5.00) {
		t.Fatalf("costs wrong: %+v", c)
	}
	if !approx(c.TotalCost, 7.50) {
		t.Fatalf("total wrong: %v", c.TotalCost)
	}
}

func TestEstimateCostUnknownModelZero(t *testing.T) {
	c := EstimateCost(nil, "x", "y", 1000, 1000, 0, 0)
	if c.TotalCost != 0 {
		t.Fatalf("unknown model should cost 0, got %v", c.TotalCost)
	}
}
