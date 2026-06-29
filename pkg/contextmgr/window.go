package contextmgr

import (
	"fmt"

	"github.com/lcoder/lcoder/pkg/models"
)

// WindowPolicy decides which blocks to keep, compact, or drop.
type WindowPolicy interface {
	Apply(blocks []*Block, budget TokenBudget, mgr *Manager) ([]*Block, error)
}

// KeepRecentInBudget keeps all static/stable blocks and compacts/drops dynamic blocks
// to fit within the token budget. It guarantees the last user message is retained.
type KeepRecentInBudget struct {
	MinRecent int // Minimum number of recent messages to keep
}

// NewKeepRecentInBudget creates a window policy.
func NewKeepRecentInBudget(minRecent int) *KeepRecentInBudget {
	if minRecent < 1 {
		minRecent = 1
	}
	return &KeepRecentInBudget{MinRecent: minRecent}
}

// DefaultKeepRecentInBudget keeps at least 10 recent messages.
func DefaultKeepRecentInBudget() *KeepRecentInBudget {
	return NewKeepRecentInBudget(10)
}

// Apply selects blocks within the token budget.
func (p *KeepRecentInBudget) Apply(blocks []*Block, budget TokenBudget, mgr *Manager) ([]*Block, error) {
	total := 0
	for _, b := range blocks {
		total += mgr.EstimateTokens(b.Messages)
	}

	// Eager compaction: if total exceeds compact threshold, compact before hard limit.
	if total > budget.CompactLimit() && mgr.summarizer != nil {
		return p.fitWithCompaction(blocks, budget, mgr)
	}

	if mgr.summarizer == nil {
		// Without a summarizer we can only truncate/drop dynamic blocks.
		return p.fitWithoutCompaction(blocks, budget, mgr)
	}
	return p.fitWithCompaction(blocks, budget, mgr)
}

func (p *KeepRecentInBudget) fitWithoutCompaction(blocks []*Block, budget TokenBudget, mgr *Manager) ([]*Block, error) {
	var result []*Block
	var tokens int

	for _, b := range blocks {
		bt := mgr.EstimateTokens(b.Messages)
		if tokens+bt <= budget.EffectiveInput() {
			result = append(result, b)
			tokens += bt
			continue
		}
		if b.Stability == StabilityDynamic {
			// Try to keep the tail of a dynamic block.
			kept := p.keepTail(b, budget.EffectiveInput()-tokens, mgr)
			if len(kept.Messages) > 0 {
				result = append(result, kept)
			}
		}
		// Static/stable blocks that don't fit are dropped (should be rare).
	}
	return p.ensureLastUser(result, mgr)
}

func (p *KeepRecentInBudget) fitWithCompaction(blocks []*Block, budget TokenBudget, mgr *Manager) ([]*Block, error) {
	// Separate static/stable from dynamic.
	var staticStable []*Block
	var dynamic []*Block
	for _, b := range blocks {
		if b.Stability == StabilityDynamic {
			dynamic = append(dynamic, b)
		} else {
			staticStable = append(staticStable, b)
		}
	}

	used := 0
	for _, b := range staticStable {
		used += mgr.EstimateTokens(b.Messages)
	}

	available := budget.EffectiveInput() - used
	if available < 0 {
		// Hard limit exceeded even by static blocks; fall back to truncation.
		return p.fitWithoutCompaction(blocks, budget, mgr)
	}

	// Fit dynamic blocks in priority order (higher first), recent messages last.
	var keptDynamic []*Block
	for i := len(dynamic) - 1; i >= 0; i-- {
		b := dynamic[i]
		tokens := mgr.EstimateTokens(b.Messages)
		if tokens <= available {
			keptDynamic = append([]*Block{b}, keptDynamic...)
			available -= tokens
			continue
		}
		// If this is the recent block, compact older messages into summary.
		if b.Kind == BlockRecent {
			compacted, err := p.compactRecent(b, available, mgr)
			if err != nil {
				return nil, fmt.Errorf("compact recent: %w", err)
			}
			keptDynamic = append([]*Block{compacted}, keptDynamic...)
			available -= mgr.EstimateTokens(compacted.Messages)
		}
		// Other dynamic blocks that don't fit are dropped.
	}

	result := append(staticStable, keptDynamic...)
	return p.ensureLastUser(result, mgr)
}

func (p *KeepRecentInBudget) compactRecent(b *Block, budget int, mgr *Manager) (*Block, error) {
	if len(b.Messages) == 0 {
		return b, nil
	}

	// Ensure at least MinRecent messages plus the last user message remain.
	keep := p.MinRecent
	if keep > len(b.Messages) {
		keep = len(b.Messages)
	}

	// Find last user message and make sure it is in the kept tail.
	lastUserIdx := -1
	for i := len(b.Messages) - 1; i >= 0; i-- {
		if b.Messages[i].Role == models.RoleUser {
			lastUserIdx = i
			break
		}
	}
	if lastUserIdx >= 0 && lastUserIdx < len(b.Messages)-keep {
		keep = len(b.Messages) - lastUserIdx
	}

	older := b.Messages[:len(b.Messages)-keep]
	recent := stripLeadingOrphanToolResults(b.Messages[len(b.Messages)-keep:])

	if len(older) == 0 {
		return NewBlock(BlockRecent, b.Name, StabilityDynamic, b.Priority, recent...), nil
	}

	summaryText, err := mgr.summarizer(older)
	if err != nil {
		// Graceful fallback: a summarizer failure (network error, circuit
		// breaker open, etc.) must not crash the turn. Drop the older messages
		// and keep only the recent tail, equivalent to truncation.
		return NewBlock(BlockRecent, b.Name, StabilityDynamic, b.Priority, recent...), nil
	}
	summaryMsg := models.NewAgentMessage(models.RoleSystem, models.TextContent{
		Text: "[Summary of earlier conversation]\n\n" + summaryText,
	}).WithMetadata("compacted", true)

	return NewBlock(BlockRecent, b.Name, StabilityDynamic, b.Priority,
		append([]models.AgentMessage{summaryMsg}, recent...)...), nil
}

func (p *KeepRecentInBudget) keepTail(b *Block, budget int, mgr *Manager) *Block {
	if budget <= 0 || len(b.Messages) == 0 {
		return NewBlock(b.Kind, b.Name, b.Stability, b.Priority)
	}

	// Keep messages from the end until budget exhausted.
	start := len(b.Messages)
	used := 0
	for i := len(b.Messages) - 1; i >= 0; i-- {
		t := mgr.EstimateTokens([]models.AgentMessage{b.Messages[i]})
		if used+t > budget {
			break
		}
		used += t
		start = i
	}
	if start >= len(b.Messages) {
		return NewBlock(b.Kind, b.Name, b.Stability, b.Priority)
	}
	kept := stripLeadingOrphanToolResults(b.Messages[start:])
	return NewBlock(b.Kind, b.Name, b.Stability, b.Priority, kept...)
}

// stripLeadingOrphanToolResults removes tool_result messages at the very front
// of a truncated/compacted tail. Their matching tool_use lives in the messages
// that were cut off ahead of the tail, so they would arrive at the provider as
// orphan tool_results — which Anthropic rejects with a 400. Any leading run of
// tool_result messages is necessarily orphaned (a paired tool_result is always
// preceded by its assistant tool_use, which would also be in the tail).
func stripLeadingOrphanToolResults(msgs []models.AgentMessage) []models.AgentMessage {
	start := 0
	for start < len(msgs) && msgs[start].Role == models.RoleToolResult {
		start++
	}
	if start == 0 {
		return msgs
	}
	return msgs[start:]
}

func (p *KeepRecentInBudget) ensureLastUser(blocks []*Block, mgr *Manager) ([]*Block, error) {
	var recent *Block
	var recentIdx int
	for i, b := range blocks {
		if b.Kind == BlockRecent {
			recent = b
			recentIdx = i
			break
		}
	}
	if recent == nil {
		// No recent block; nothing to ensure.
		return blocks, nil
	}

	lastUserIdx := -1
	for i := len(recent.Messages) - 1; i >= 0; i-- {
		if recent.Messages[i].Role == models.RoleUser {
			lastUserIdx = i
			break
		}
	}
	if lastUserIdx >= 0 {
		return blocks, nil
	}

	// Find a user message in any dynamic block and append it.
	for _, b := range blocks {
		if b.Stability != StabilityDynamic {
			continue
		}
		for i := len(b.Messages) - 1; i >= 0; i-- {
			if b.Messages[i].Role == models.RoleUser {
				recent.Messages = append(recent.Messages, b.Messages[i])
				blocks[recentIdx] = recent
				return blocks, nil
			}
		}
	}
	return blocks, nil
}
