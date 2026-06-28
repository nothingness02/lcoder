package session

import (
	"os"
	"testing"

	"github.com/lcoder/lcoder/pkg/models"
)

func TestFork(t *testing.T) {
	dir, _ := os.MkdirTemp("", "lcoder-session-*")
	defer os.RemoveAll(dir)

	store := NewStore(dir)
	sess, _ := store.Create("/project")
	_ = sess.Append(models.UserMessage("a"))
	_ = sess.Append(models.UserMessage("b"))
	_ = sess.Append(models.UserMessage("c"))

	forked, err := store.Fork("/project", sess, sess.Messages[1].ID)
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	if len(forked.Messages) != 2 {
		t.Fatalf("expected 2 messages in fork, got %d", len(forked.Messages))
	}
	if forked.Messages[0].Text() != "a" || forked.Messages[1].Text() != "b" {
		t.Fatalf("unexpected fork history: %v", forked.Messages)
	}
}

func TestClone(t *testing.T) {
	dir, _ := os.MkdirTemp("", "lcoder-session-*")
	defer os.RemoveAll(dir)

	store := NewStore(dir)
	sess, _ := store.Create("/project")
	_ = sess.Append(models.UserMessage("a"))

	cloned, err := store.Clone("/project", sess)
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	if len(cloned.Messages) != 1 {
		t.Fatalf("expected 1 message in clone, got %d", len(cloned.Messages))
	}
}
