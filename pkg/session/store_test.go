package session

import (
	"os"
	"testing"

	"github.com/lcoder/lcoder/pkg/models"
)

func TestCreateAppendLoad(t *testing.T) {
	dir, err := os.MkdirTemp("", "lcoder-session-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	store := NewStore(dir)
	sess, err := store.Create("/project")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	msg := models.UserMessage("hello")
	if err := sess.Append(msg); err != nil {
		t.Fatalf("append: %v", err)
	}

	loaded, err := store.Load(sess.Path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(loaded.Messages))
	}
	if loaded.Messages[0].Text() != "hello" {
		t.Fatalf("expected hello, got %s", loaded.Messages[0].Text())
	}
	if len(loaded.ActiveBranch) != 1 {
		t.Fatalf("expected active branch length 1, got %d", len(loaded.ActiveBranch))
	}
}
