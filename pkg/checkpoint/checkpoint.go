// Package checkpoint defines a portable snapshot DTO and the interfaces used to
// save and restore agent runtime state.
package checkpoint

import (
	"encoding/json"
	"time"

	"github.com/lcoder/lcoder/pkg/contextmgr"
	"github.com/lcoder/lcoder/pkg/models"
)

// CurrentVersion is the only checkpoint format version accepted by UnmarshalJSON.
const CurrentVersion = 1

// Checkpoint is a portable, serializable snapshot of an agent session.
type Checkpoint struct {
	Version   int               `json:"version"`
	CreatedAt time.Time         `json:"created_at"`
	Mode      string            `json:"mode"`
	Model     models.ModelRef   `json:"model"`
	Context   *ContextSnapshot  `json:"context"`
	Runtime   *RuntimeSnapshot  `json:"runtime"`
}

// MarshalJSON sets default Version and CreatedAt before serialization.
func (cp Checkpoint) MarshalJSON() ([]byte, error) {
	if cp.Version == 0 {
		cp.Version = CurrentVersion
	}
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = time.Now().UTC()
	}
	type alias Checkpoint
	return json.Marshal((*alias)(&cp))
}

// UnmarshalJSON validates the checkpoint version before finishing deserialization.
func (cp *Checkpoint) UnmarshalJSON(data []byte) error {
	type alias Checkpoint
	aux := (*alias)(cp)
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.Version != CurrentVersion {
		return ErrVersionMismatch
	}
	return nil
}

// ContextSnapshot captures the state of a contextmgr.Manager.
type ContextSnapshot struct {
	Budget             contextmgr.TokenBudget  `json:"budget"`
	Blocks             []BlockSnapshot         `json:"blocks"`
	EphemeralReminders []string                `json:"ephemeral_reminders,omitempty"`
	LastUsage          *contextmgr.RealUsage   `json:"last_usage,omitempty"`
	CachePolicy        string                  `json:"cache_policy,omitempty"`
}

// BlockSnapshot mirrors contextmgr.Block with serializable fields.
type BlockSnapshot struct {
	Kind             string                `json:"kind"`
	Name             string                `json:"name"`
	Priority         int                   `json:"priority"`
	Stability        string                `json:"stability"`
	Messages         []models.AgentMessage `json:"messages,omitempty"`
	Metadata         map[string]any        `json:"metadata,omitempty"`
	CacheHint        string                `json:"cache_hint,omitempty"`
	LastModifiedTurn int                   `json:"last_modified_turn,omitempty"`
}

// RuntimeSnapshot captures the agent runtime state.
type RuntimeSnapshot struct {
	State          int                   `json:"state"`
	SteeringQueue  []models.AgentMessage `json:"steering_queue,omitempty"`
	FollowUpQueue  []models.AgentMessage `json:"follow_up_queue,omitempty"`
	ActiveDeferred map[string]bool       `json:"active_deferred,omitempty"`
}

// Source produces a Checkpoint representing the current state.
type Source interface {
	Checkpoint() (*Checkpoint, error)
}

// Target restores state from a Checkpoint.
type Target interface {
	Restore(*Checkpoint) error
}

// Store persists and retrieves checkpoints by identifier.
type Store interface {
	Save(id string, cp *Checkpoint) error
	Load(id string) (*Checkpoint, error)
	List() ([]string, error)
	Delete(id string) error
}
