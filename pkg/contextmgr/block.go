// Package contextmgr provides structured, windowed, cache-friendly context
// management for the agent conversation.
package contextmgr

import (
	"strings"

	"github.com/lcoder/lcoder/pkg/models"
)

// BlockKind classifies a context block by source and stability.
type BlockKind string

const (
	BlockSystem      BlockKind = "system"       // Top-level system prompt
	BlockMode        BlockKind = "mode"         // Mode-specific system prompt
	BlockSkills      BlockKind = "skills"       // Activated skills
	BlockProjectDocs BlockKind = "project_docs" // AGENTS.md / CLAUDE.md
	BlockToolDefs    BlockKind = "tool_defs"    // Tool JSON schemas
	BlockSummary     BlockKind = "summary"      // Summarized older messages
	BlockRecent      BlockKind = "recent"       // Recent full messages
	BlockRetrieval   BlockKind = "retrieval"    // RAG / code index results
)

// Stability indicates how likely a block is to change between turns.
type Stability string

const (
	StabilityStatic  Stability = "static"  // Unchanged for the whole session
	StabilityStable  Stability = "stable"  // Rarely changes within a session
	StabilityDynamic Stability = "dynamic" // May change every turn
)

// CacheHint gives cache-placement advice to the LLM engine.
type CacheHint string

const (
	CacheHintBreakpoint CacheHint = "breakpoint" // Good place for a cache breakpoint
	CacheHintSkip       CacheHint = "skip"       // Not worth caching
)

// Block is a unit of context with metadata for budgeting and caching.
type Block struct {
	Kind      BlockKind
	Name      string
	Priority  int
	Stability Stability
	Messages  []models.AgentMessage
	Metadata  map[string]any
	CacheHint CacheHint
	// LastModifiedTurn tracks the last turn this block's content changed.
	// Used to decide cache refresh frequency.
	LastModifiedTurn int
}

// NewBlock creates a block with the given kind and messages.
func NewBlock(kind BlockKind, name string, stability Stability, priority int, msgs ...models.AgentMessage) *Block {
	return &Block{
		Kind:      kind,
		Name:      name,
		Stability: stability,
		Priority:  priority,
		Messages:  msgs,
		Metadata:  make(map[string]any),
	}
}

// Text returns the concatenated text of all messages in the block.
func (b *Block) Text() string {
	var parts []string
	for _, m := range b.Messages {
		parts = append(parts, m.Text())
	}
	return strings.Join(parts, "\n")
}

// IsSystemBlock reports whether the block should be merged into the system prompt.
func IsSystemBlock(b *Block) bool {
	switch b.Kind {
	case BlockSystem, BlockMode, BlockSkills, BlockProjectDocs:
		return true
	}
	return false
}

// DefaultBlockOrder returns the canonical order of block kinds for cache friendliness.
// Stable blocks come first; dynamic blocks come last.
func DefaultBlockOrder() []BlockKind {
	return []BlockKind{
		BlockSystem,
		BlockMode,
		BlockSkills,
		BlockProjectDocs,
		BlockSummary,
		BlockRetrieval,
		BlockRecent,
	}
}
