package contextmgr

import (
	"sort"

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

// Apply selects blocks within the token budget. Compaction is now a committed
// operation (Manager.MaybeCompact) run at turn boundaries, so the window policy's
// sole remaining job is a truncation backstop: keep static/stable blocks and drop
// the head of dynamic blocks so the request never exceeds the hard input limit,
// even when compaction was skipped or failed.
func (p *KeepRecentInBudget) Apply(blocks []*Block, budget TokenBudget, mgr *Manager) ([]*Block, error) {
	return p.fitWithoutCompaction(blocks, budget, mgr)
}

func (p *KeepRecentInBudget) fitWithoutCompaction(blocks []*Block, budget TokenBudget, mgr *Manager) ([]*Block, error) {
	var result []*Block
	var tokens int
	cap := budget.DropLimit()

	for _, b := range blocks {
		bt := mgr.EstimateTokens(b.Messages)
		if tokens+bt <= cap {
			result = append(result, b)
			tokens += bt
			continue
		}
		if b.Stability == StabilityDynamic {
			// Try to keep the tail of a dynamic block.
			kept := p.keepTail(b, cap-tokens, mgr)
			if len(kept.Messages) > 0 {
				result = append(result, kept)
			}
		}
		// Static/stable blocks that don't fit are dropped (should be rare).
	}
	result = p.enforceStaticRatio(result, budget, mgr)
	return p.ensureLastUser(result, mgr)
}

// enforceStaticRatio drops the lowest-priority static/stable blocks when their
// combined token count exceeds StaticRatio% of the effective input window.
// BlockSystem/system is protected by its high priority and dropped only after
// all lower-priority stable blocks have been removed.
func (p *KeepRecentInBudget) enforceStaticRatio(blocks []*Block, budget TokenBudget, mgr *Manager) []*Block {
	staticCap := staticRatioCap(budget)
	if staticCap <= 0 {
		return blocks
	}

	staticIdx := make([]int, 0, len(blocks))
	staticTokens := 0
	for i, b := range blocks {
		if b.Stability == StabilityDynamic {
			continue
		}
		staticIdx = append(staticIdx, i)
		staticTokens += mgr.EstimateTokens(b.Messages)
	}
	if staticTokens <= staticCap {
		return blocks
	}

	// Sort by priority ascending so the lowest-priority blocks are dropped first.
	sort.Slice(staticIdx, func(i, j int) bool {
		return blocks[staticIdx[i]].Priority < blocks[staticIdx[j]].Priority
	})

	for _, idx := range staticIdx {
		if staticTokens <= staticCap {
			break
		}
		staticTokens -= mgr.EstimateTokens(blocks[idx].Messages)
		blocks[idx] = nil
	}

	compacted := make([]*Block, 0, len(blocks))
	for _, b := range blocks {
		if b != nil {
			compacted = append(compacted, b)
		}
	}
	return compacted
}

// staticRatioCap returns the maximum token budget for static/stable blocks.
// A value of zero or >=100 disables the cap.
func staticRatioCap(budget TokenBudget) int {
	if budget.StaticRatio <= 0 || budget.StaticRatio >= 100 {
		return 0
	}
	eff := budget.EffectiveInput()
	if eff <= 0 {
		return 0
	}
	return int(float64(eff) * float64(budget.StaticRatio) / 100.0)
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
