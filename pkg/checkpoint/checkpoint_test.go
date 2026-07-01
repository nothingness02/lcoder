package checkpoint_test

import (
	"encoding/json"
	"testing"

	"github.com/lcoder/lcoder/pkg/checkpoint"
	"github.com/lcoder/lcoder/pkg/contextmgr"
	"github.com/lcoder/lcoder/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestCheckpointRoundTrip(t *testing.T) {
	cp := &checkpoint.Checkpoint{
		Mode: "test",
		Model: models.ModelRef{
			Provider: "anthropic",
			ID:       "claude-test",
		},
		Context: &checkpoint.ContextSnapshot{
			Budget: contextmgr.TokenBudget{
				MaxTotal:    128000,
				TargetTotal: 96000,
			},
			Blocks: []checkpoint.BlockSnapshot{
				{
					Kind:      string(contextmgr.BlockRecent),
					Name:      "recent",
					Priority:  100,
					Stability: string(contextmgr.StabilityDynamic),
					Messages:  []models.AgentMessage{models.UserMessage("hello")},
					CacheHint: string(contextmgr.CacheHintBreakpoint),
				},
			},
			EphemeralReminders: []string{"reminder"},
			LastUsage: &contextmgr.RealUsage{
				InputTokens:         10,
				CacheReadTokens:     20,
				CacheCreationTokens: 30,
			},
			CachePolicy: string(contextmgr.CachePolicyDefault),
		},
		Runtime: &checkpoint.RuntimeSnapshot{
			State:          1,
			SteeringQueue:  []models.AgentMessage{models.UserMessage("steer")},
			FollowUpQueue:  []models.AgentMessage{models.UserMessage("follow")},
			ActiveDeferred: map[string]bool{"edit": true},
		},
	}

	data, err := json.Marshal(cp)
	require.NoError(t, err)

	var got checkpoint.Checkpoint
	require.NoError(t, json.Unmarshal(data, &got))

	require.Equal(t, checkpoint.CurrentVersion, got.Version)
	require.False(t, got.CreatedAt.IsZero())
	require.Equal(t, cp.Mode, got.Mode)
	require.Equal(t, cp.Model, got.Model)
	require.Len(t, got.Context.Blocks, 1)
	require.Equal(t, string(contextmgr.BlockRecent), got.Context.Blocks[0].Kind)
	require.Equal(t, "hello", got.Context.Blocks[0].Messages[0].Text())
	require.Equal(t, []string{"reminder"}, got.Context.EphemeralReminders)
	require.NotNil(t, got.Context.LastUsage)
	require.Equal(t, 60, got.Context.LastUsage.PromptTokens())
	require.Equal(t, string(contextmgr.CachePolicyDefault), got.Context.CachePolicy)
	require.Equal(t, 1, got.Runtime.State)
	require.Len(t, got.Runtime.SteeringQueue, 1)
	require.Equal(t, "steer", got.Runtime.SteeringQueue[0].Text())
	require.Len(t, got.Runtime.FollowUpQueue, 1)
	require.Equal(t, "follow", got.Runtime.FollowUpQueue[0].Text())
	require.Equal(t, map[string]bool{"edit": true}, got.Runtime.ActiveDeferred)
}

func TestCheckpointVersionMismatch(t *testing.T) {
	cp := &checkpoint.Checkpoint{Mode: "test"}
	data, err := json.Marshal(cp)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	raw["version"] = 99

	mutated, err := json.Marshal(raw)
	require.NoError(t, err)

	var got checkpoint.Checkpoint
	err = json.Unmarshal(mutated, &got)
	require.ErrorIs(t, err, checkpoint.ErrVersionMismatch)
}
