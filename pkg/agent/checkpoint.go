package agent

import (
	"github.com/lcoder/lcoder/pkg/checkpoint"
	"github.com/lcoder/lcoder/pkg/contextmgr"
)

// Checkpoint captures the agent's current mode, model, context manager state,
// and runtime state into a portable snapshot.
func (a *Agent) Checkpoint() (*checkpoint.Checkpoint, error) {
	mgrState, err := a.mgr.Snapshot()
	if err != nil {
		return nil, err
	}

	stateSnap := a.loopState.snapshot()
	execSnap := a.executor.snapshot()

	cp := &checkpoint.Checkpoint{
		Version: 0, // MarshalJSON will set CurrentVersion.
		Mode:    a.cfg.Mode,
		Model:   a.cfg.Model,
		Context: &checkpoint.ContextSnapshot{
			Budget:             mgrState.Budget,
			EphemeralReminders: mgrState.EphemeralReminders,
			CachePolicy:        mgrState.CachePolicy,
			LastUsage:          mgrState.LastUsage,
			Blocks:             make([]checkpoint.BlockSnapshot, 0, len(mgrState.Blocks)),
		},
		Runtime: &checkpoint.RuntimeSnapshot{
			State:          int(stateSnap.State),
			SteeringQueue:  stateSnap.SteeringQueue,
			FollowUpQueue:  stateSnap.FollowUpQueue,
			ActiveDeferred: execSnap.ActiveDeferred,
		},
	}

	for _, b := range mgrState.Blocks {
		cp.Context.Blocks = append(cp.Context.Blocks, checkpoint.BlockSnapshot{
			Kind:             string(b.Kind),
			Name:             b.Name,
			Priority:         b.Priority,
			Stability:        string(b.Stability),
			Messages:         b.Messages,
			Metadata:         b.Metadata,
			CacheHint:        string(b.CacheHint),
			LastModifiedTurn: b.LastModifiedTurn,
		})
	}

	return cp, nil
}

// Restore replaces the agent's mode, model, context manager state, and runtime
// state from a checkpoint.
//
// This is not a thread-safe hot-restore: it must be called at a safe boundary
// (e.g., while the agent is idle and waiting for user input) when no run loop,
// streamer, or executor is active.
func (a *Agent) Restore(cp *checkpoint.Checkpoint) error {
	a.cfg.Mode = cp.Mode
	a.cfg.Model = cp.Model

	mgrState := &contextmgr.ManagerState{
		Budget:             cp.Context.Budget,
		EphemeralReminders: cp.Context.EphemeralReminders,
		CachePolicy:        cp.Context.CachePolicy,
		LastUsage:          cp.Context.LastUsage,
		Blocks:             make([]contextmgr.BlockState, 0, len(cp.Context.Blocks)),
	}

	for _, b := range cp.Context.Blocks {
		mgrState.Blocks = append(mgrState.Blocks, contextmgr.BlockState{
			Kind:             contextmgr.BlockKind(b.Kind),
			Name:             b.Name,
			Priority:         b.Priority,
			Stability:        contextmgr.Stability(b.Stability),
			Messages:         b.Messages,
			Metadata:         b.Metadata,
			CacheHint:        contextmgr.CacheHint(b.CacheHint),
			LastModifiedTurn: b.LastModifiedTurn,
		})
	}

	if err := a.mgr.Restore(mgrState); err != nil {
		return err
	}

	a.loopState.restore(RuntimeState{
		State:         State(cp.Runtime.State),
		SteeringQueue: cp.Runtime.SteeringQueue,
		FollowUpQueue: cp.Runtime.FollowUpQueue,
	})

	a.executor.restore(RuntimeState{
		ActiveDeferred: cp.Runtime.ActiveDeferred,
	})

	return nil
}
