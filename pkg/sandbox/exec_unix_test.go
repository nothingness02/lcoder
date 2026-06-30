//go:build !windows

package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A backgrounded grandchild must be killed when the parent times out, otherwise
// it would create the sentinel after we return. Process-group kill prevents this.
func TestSoftLimitKillsOrphans(t *testing.T) {
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "SENTINEL")
	sb := newSoftLimit()
	_, _ = sb.Exec(context.Background(), ExecSpec{
		Command: "(sleep 1 && touch " + sentinel + ") & echo started",
		Cwd:     dir,
		Env:     []string{"PATH=" + pathEnv()},
		Timeout: 150 * time.Millisecond,
	})
	// Wait past the grandchild's sleep; if the group was killed it never fires.
	time.Sleep(1500 * time.Millisecond)
	if _, err := os.Stat(sentinel); err == nil {
		t.Fatal("orphan grandchild survived: sentinel was created")
	}
}
