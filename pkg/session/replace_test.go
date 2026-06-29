package session

import (
	"os"
	"testing"

	"github.com/lcoder/lcoder/pkg/models"
)

// Replace 必须把磁盘记录整体重写为给定消息(丢弃原有更长的历史),
// 这是压缩提交时"重置 session 对应的对话记录"的落盘原语。
func TestReplaceRewritesSessionToGivenMessages(t *testing.T) {
	dir, err := os.MkdirTemp("", "lcoder-replace-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	store := NewStore(dir)
	sess, err := store.Create("/project")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	for i := 0; i < 5; i++ {
		if err := sess.Append(models.UserMessage("m")); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	summary := models.NewAgentMessage(models.RoleSystem, models.TextContent{Text: "[Summary] earlier"}).
		WithMetadata("compacted", true)
	tail := models.UserMessage("latest")
	if err := sess.Replace([]models.AgentMessage{summary, tail}); err != nil {
		t.Fatalf("replace: %v", err)
	}

	loaded, err := store.Load(sess.Path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Messages) != 2 {
		t.Fatalf("expected 2 messages after replace, got %d", len(loaded.Messages))
	}
	if loaded.Messages[0].Text() != "[Summary] earlier" || loaded.Messages[1].Text() != "latest" {
		t.Fatalf("unexpected messages after replace: %+v", loaded.Messages)
	}
}
