package contextmgr

import (
	"fmt"
	"strings"

	"github.com/lcoder/lcoder/pkg/models"
)

// TokenBudget defines hard and soft limits for context sizing.
type TokenBudget struct {
	MaxTotal      int // Hard upper bound (model context window)
	TargetTotal   int // Desired upper bound
	ReserveOutput int // Tokens reserved for model output
	// CompactThreshold is the ratio of TargetTotal at which compaction starts.
	// Zero defaults to 1.0 (compact only when exceeding target).
	CompactThreshold float64
	// DropThreshold is the ratio of MaxTotal at which old messages are dropped.
	// Zero defaults to 1.0 (drop only when exceeding max).
	DropThreshold float64
}

// EffectiveInput returns the budget left for input after reserving output.
func (b TokenBudget) EffectiveInput() int {
	return b.MaxTotal - b.ReserveOutput
}

// CompactLimit returns the token count at which compaction should start.
func (b TokenBudget) CompactLimit() int {
	thr := b.CompactThreshold
	if thr <= 0 {
		thr = 1.0
	}
	return int(float64(b.TargetTotal) * thr)
}

// DropLimit returns the token count at which old messages should be dropped.
func (b TokenBudget) DropLimit() int {
	thr := b.DropThreshold
	if thr <= 0 {
		thr = 1.0
	}
	return int(float64(b.MaxTotal-b.ReserveOutput) * thr)
}

// TokenEstimator estimates token count for a slice of messages.
type TokenEstimator func(messages []models.AgentMessage) int

// SummarizeFunc generates a summary from messages.
type SummarizeFunc func(messages []models.AgentMessage) (string, error)

// Manager manages structured context blocks within a token budget.
type Manager struct {
	budget     TokenBudget
	blocks     []*Block
	estimator  TokenEstimator
	summarizer SummarizeFunc
	policy     WindowPolicy
}

// Option configures a Manager.
type Option func(*Manager)

// WithEstimator sets a custom token estimator.
func WithEstimator(e TokenEstimator) Option {
	return func(m *Manager) { m.estimator = e }
}

// WithSummarizer sets the summarizer used for compaction.
func WithSummarizer(s SummarizeFunc) Option {
	return func(m *Manager) { m.summarizer = s }
}

// WithWindowPolicy sets the window policy.
func WithWindowPolicy(p WindowPolicy) Option {
	return func(m *Manager) { m.policy = p }
}

// NewManager creates a context manager with the given budget.
func NewManager(budget TokenBudget, opts ...Option) *Manager {
	m := &Manager{
		budget:     budget,
		estimator:  DefaultEstimator,
		summarizer: nil,
		policy:     &KeepRecentInBudget{},
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// SetBudget replaces the manager's token budget in place. Blocks and history are
// untouched, so a live model switch can re-size the budget without losing the
// conversation.
func (m *Manager) SetBudget(b TokenBudget) {
	m.budget = b
}

// Budget returns the manager's current token budget.
func (m *Manager) Budget() TokenBudget {
	return m.budget
}

// SetBlock replaces an existing block of the same kind and name, or appends it.
func (m *Manager) SetBlock(block *Block) {
	for i, b := range m.blocks {
		if b.Kind == block.Kind && b.Name == block.Name {
			m.blocks[i] = block
			return
		}
	}
	m.blocks = append(m.blocks, block)
}

// GetBlock returns the first block matching kind and name.
func (m *Manager) GetBlock(kind BlockKind, name string) (*Block, bool) {
	for _, b := range m.blocks {
		if b.Kind == kind && b.Name == name {
			return b, true
		}
	}
	return nil, false
}

// AppendRecent appends a message to the recent messages block.
func (m *Manager) AppendRecent(msg models.AgentMessage) {
	b, ok := m.GetBlock(BlockRecent, "recent")
	if !ok {
		b = NewBlock(BlockRecent, "recent", StabilityDynamic, 100)
		m.SetBlock(b)
	}
	b.Messages = append(b.Messages, msg)
}

// SetBlockWithTurn replaces a block and records the current turn for cache decisions.
func (m *Manager) SetBlockWithTurn(block *Block, turn int) {
	block.LastModifiedTurn = turn
	m.SetBlock(block)
}

// Blocks returns blocks in canonical order.
func (m *Manager) Blocks() []*Block {
	order := DefaultBlockOrder()
	orderIndex := make(map[BlockKind]int, len(order))
	for i, k := range order {
		orderIndex[k] = i
	}

	ordered := make([]*Block, len(m.blocks))
	copy(ordered, m.blocks)
	// Stable sort by canonical order, then by priority descending.
	for i := 0; i < len(ordered); i++ {
		for j := i + 1; j < len(ordered); j++ {
			ai := orderIndex[ordered[i].Kind]
			aj := orderIndex[ordered[j].Kind]
			if ai > aj || (ai == aj && ordered[i].Priority < ordered[j].Priority) {
				ordered[i], ordered[j] = ordered[j], ordered[i]
			}
		}
	}
	return ordered
}

// EstimateTokens returns the estimated token count for messages.
func (m *Manager) EstimateTokens(messages []models.AgentMessage) int {
	return m.estimator(messages)
}

// BuildTurnRequest selects blocks within budget and builds a TurnRequest.
// It also computes cache breakpoints based on block boundaries and stability.
func (m *Manager) BuildTurnRequest(model models.ModelRef, tools []models.ToolDefinition) (models.TurnRequest, error) {
	blocks, err := m.policy.Apply(m.Blocks(), m.budget, m)
	if err != nil {
		return models.TurnRequest{}, fmt.Errorf("apply window policy: %w", err)
	}

	var systemParts []string
	var messages []models.AgentMessage
	var breakpoints []int
	var messageIdx int
	var stableTokens int

	for _, b := range blocks {
		if IsSystemBlock(b) {
			systemParts = append(systemParts, b.Text())
			stableTokens += m.EstimateTokens(b.Messages)
			continue
		}
		if len(b.Messages) == 0 {
			continue
		}
		// Place a cache breakpoint at the first non-system message when the
		// prefix (system/static/stable blocks) is large enough to be worth caching.
		if messageIdx == 0 && stableTokens >= 256 {
			breakpoints = append(breakpoints, messageIdx)
		}
		// Explicit block-level hints also produce breakpoints.
		if b.CacheHint == CacheHintBreakpoint {
			breakpoints = append(breakpoints, messageIdx)
		}
		messages = append(messages, b.Messages...)
		messageIdx += len(b.Messages)
	}

	// Always mark the last user message as a cache breakpoint if available.
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == models.RoleUser {
			breakpoints = append(breakpoints, i)
			break
		}
	}

	return models.TurnRequest{
		Model:            model,
		SystemPrompt:     strings.Join(systemParts, "\n\n"),
		Messages:         messages,
		Tools:            tools,
		Cache:            "auto",
		CacheBreakpoints: breakpoints,
	}, nil
}

// AllMessages returns all messages across all blocks in canonical order.
func (m *Manager) AllMessages() []models.AgentMessage {
	var messages []models.AgentMessage
	for _, b := range m.Blocks() {
		if !IsSystemBlock(b) {
			messages = append(messages, b.Messages...)
		}
	}
	return messages
}

// ReplaceRecent replaces the recent messages block with the given messages.
func (m *Manager) ReplaceRecent(msgs []models.AgentMessage) {
	m.SetBlock(NewBlock(BlockRecent, "recent", StabilityDynamic, 100, msgs...))
}

// ClearRecent removes all messages from the recent block.
func (m *Manager) ClearRecent() {
	m.ReplaceRecent(nil)
}

// SystemPrompt returns the merged system prompt from all system blocks.
func (m *Manager) SystemPrompt() string {
	var parts []string
	for _, b := range m.Blocks() {
		if IsSystemBlock(b) {
			if text := b.Text(); text != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "\n\n")
}

// SetSystemPrompt sets the primary system prompt block.
func (m *Manager) SetSystemPrompt(text string) {
	m.SetBlock(NewBlock(BlockSystem, "system", StabilityStatic, 100,
		models.NewAgentMessage(models.RoleSystem, models.TextContent{Text: text})))
}

// SetMessages rebuilds the conversation from a flat message list.
// System messages become system blocks; everything else goes into recent.
func (m *Manager) SetMessages(msgs []models.AgentMessage) {
	// Preserve existing system blocks if no system messages provided.
	var hasSystem bool
	var nonSystem []models.AgentMessage
	for _, msg := range msgs {
		if msg.Role == models.RoleSystem {
			m.SetSystemPrompt(msg.Text())
			hasSystem = true
		} else {
			nonSystem = append(nonSystem, msg)
		}
	}
	if !hasSystem {
		m.SetBlock(NewBlock(BlockSystem, "system", StabilityStatic, 100))
	}
	m.ReplaceRecent(nonSystem)
}

// Clone returns a deep copy of the manager with independent blocks.
func (m *Manager) Clone() *Manager {
	other := NewManager(m.budget, WithEstimator(m.estimator), WithSummarizer(m.summarizer), WithWindowPolicy(m.policy))
	for _, b := range m.blocks {
		copied := NewBlock(b.Kind, b.Name, b.Stability, b.Priority)
		copied.Messages = append([]models.AgentMessage(nil), b.Messages...)
		copied.Metadata = make(map[string]any)
		for k, v := range b.Metadata {
			copied.Metadata[k] = v
		}
		copied.CacheHint = b.CacheHint
		copied.LastModifiedTurn = b.LastModifiedTurn
		other.SetBlock(copied)
	}
	return other
}

// Stats returns token usage per block and total.
func (m *Manager) Stats() map[string]int {
	stats := make(map[string]int)
	total := 0
	for _, b := range m.Blocks() {
		tokens := m.EstimateTokens(b.Messages)
		stats[string(b.Kind)+":"+b.Name] = tokens
		total += tokens
	}
	stats["total"] = total
	stats["budget_max"] = m.budget.MaxTotal
	stats["budget_target"] = m.budget.TargetTotal
	stats["budget_output_reserve"] = m.budget.ReserveOutput
	stats["compact_limit"] = m.budget.CompactLimit()
	stats["drop_limit"] = m.budget.DropLimit()
	return stats
}

// DefaultEstimator uses a rough 4-char-per-token heuristic.
func DefaultEstimator(messages []models.AgentMessage) int {
	total := 0
	for _, m := range messages {
		for _, part := range m.Content {
			if t, ok := part.(models.TextContent); ok {
				total += len(t.Text)
			}
		}
	}
	return total / 4
}
