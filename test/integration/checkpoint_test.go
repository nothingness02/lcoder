//go:build integration

package integration

import (
	"testing"

	"github.com/lcoder/lcoder/pkg/checkpoint"
)

func TestAgentCheckpointSaveRestore(t *testing.T) {
	dir := t.TempDir()
	store := checkpoint.NewFileStore(dir)

	// TODO: construct an agent as in TestAgentRealRun, run a scripted
	// prompt, save a checkpoint, create a fresh agent, restore state,
	// and continue from where the first agent left off.
	_ = store

	t.Skip("fill in with scripted LLM agent construction once wiring is stable")
}
