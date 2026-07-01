package contextmgr

import "github.com/lcoder/lcoder/pkg/models"

// RealUsage captures the provider-reported prompt-token accounting for the last
// turn. The "real" prompt size is input + cache_read + cache_creation, matching
// how Anthropic-style providers bill a turn — distinct from the char-based
// DefaultEstimator, which only approximates size before any turn has run.
type RealUsage struct {
	InputTokens         int
	CacheReadTokens     int
	CacheCreationTokens int
}

// PromptTokens returns the total real prompt size: fresh input plus cached reads
// plus cache writes. This is the number that actually counts against the model
// context window on the wire.
func (u RealUsage) PromptTokens() int {
	return u.InputTokens + u.CacheReadTokens + u.CacheCreationTokens
}

// Valid reports whether the usage carries a usable (non-zero) prompt size.
func (u RealUsage) Valid() bool { return u.PromptTokens() > 0 }

// RecordRealUsage folds a provider usage report into the manager so subsequent
// budget and compaction decisions use real token counts instead of the
// heuristic estimate. CacheWriteTokens maps to cache_creation.
func (m *Manager) RecordRealUsage(u models.LLMUsage) {
	m.lastUsage = RealUsage{
		InputTokens:         u.PromptTokens,
		CacheReadTokens:     u.CacheReadTokens,
		CacheCreationTokens: u.CacheWriteTokens,
	}
	m.hasUsage = m.lastUsage.Valid()
}

// RealPromptTokens returns the last real prompt-token total and whether a usage
// report has been recorded.
func (m *Manager) RealPromptTokens() (int, bool) {
	if !m.hasUsage {
		return 0, false
	}
	return m.lastUsage.PromptTokens(), true
}

// LastRealUsage returns the last recorded provider usage and whether one exists.
func (m *Manager) LastRealUsage() (RealUsage, bool) {
	return m.lastUsage, m.hasUsage
}

// totalTokens sums the heuristic estimate across all blocks.
func (m *Manager) totalTokens() int {
	total := 0
	for _, b := range m.blocks {
		total += m.EstimateTokens(b.Messages)
	}
	return total
}

// currentTotalTokens returns the best available prompt-token total: the real
// provider count when a turn has run, otherwise the heuristic estimate.
func (m *Manager) currentTotalTokens() int {
	if rt, ok := m.RealPromptTokens(); ok {
		return rt
	}
	return m.totalTokens()
}
