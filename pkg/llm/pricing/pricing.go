// pkg/llm/pricing/pricing.go
package pricing

// ModelPrice holds per-1M-token USD prices for one model.
type ModelPrice struct {
	Prompt     float64
	Completion float64
	CacheRead  float64
	CacheWrite float64
}

// CostBreakdown is the per-turn cost split.
type CostBreakdown struct {
	PromptCost     float64
	CompletionCost float64
	CacheReadCost  float64
	CacheWriteCost float64
	TotalCost      float64
}

// EstimateCost ports estimate_cost: tokens * price_per_1M / 1e6 across four
// tiers. Unknown models (no table entry) cost 0, matching current behavior.
func EstimateCost(table map[string]ModelPrice, provider, modelID string,
	promptTokens, completionTokens, cacheReadTokens, cacheWriteTokens int) CostBreakdown {
	key := provider + "/" + modelID
	p, ok := table[key]
	if !ok {
		return CostBreakdown{}
	}
	cost := func(tokens int, per1M float64) float64 {
		return float64(tokens) * per1M / 1_000_000
	}
	b := CostBreakdown{
		PromptCost:     cost(promptTokens, p.Prompt),
		CompletionCost: cost(completionTokens, p.Completion),
		CacheReadCost:  cost(cacheReadTokens, p.CacheRead),
		CacheWriteCost: cost(cacheWriteTokens, p.CacheWrite),
	}
	b.TotalCost = b.PromptCost + b.CompletionCost + b.CacheReadCost + b.CacheWriteCost
	return b
}

// DefaultPricing mirrors the built-in PRICING table from pricing.py. The catalog
// overlay can add or override entries.
func DefaultPricing() map[string]ModelPrice {
	return map[string]ModelPrice{
		"openai/gpt-4o":                      {Prompt: 2.50, Completion: 10.00, CacheRead: 1.25, CacheWrite: 2.50},
		"openai/gpt-4o-mini":                 {Prompt: 0.15, Completion: 0.60, CacheRead: 0.075, CacheWrite: 0.15},
		"anthropic/claude-sonnet-4-20250514": {Prompt: 3.00, Completion: 15.00, CacheRead: 0.30, CacheWrite: 3.75},
		"deepseek/deepseek-chat":             {Prompt: 0.27, Completion: 1.10, CacheRead: 0.10, CacheWrite: 0.27},
		"deepseek/deepseek-reasoner":         {Prompt: 0.55, Completion: 2.19, CacheRead: 0.14, CacheWrite: 0.55},
	}
}
