package agent

import (
	"testing"

	"github.com/lcoder/lcoder/pkg/models"
)

func TestUnresolvedTodosReminder(t *testing.T) {
	msg := models.NewAgentMessage(models.RoleAssistant, models.ToolCallContent{
		Name: "todo_write",
		Arguments: map[string]any{
			"todos": []any{
				map[string]any{"text": "A", "status": "done"},
				map[string]any{"text": "B", "status": "in_progress"},
				map[string]any{"text": "C", "status": "pending"},
			},
		},
	})

	reminders := UnresolvedTodosReminder([]models.AgentMessage{msg})
	if len(reminders) != 1 {
		t.Fatalf("expected 1 reminder, got %d", len(reminders))
	}
	if reminders[0] == "" {
		t.Fatalf("expected non-empty reminder")
	}

	allDone := models.NewAgentMessage(models.RoleAssistant, models.ToolCallContent{
		Name: "todo_write",
		Arguments: map[string]any{
			"todos": []any{
				map[string]any{"text": "A", "status": "done"},
			},
		},
	})
	if got := UnresolvedTodosReminder([]models.AgentMessage{allDone}); got != nil {
		t.Fatalf("expected nil when all done, got %v", got)
	}

	if got := UnresolvedTodosReminder(nil); got != nil {
		t.Fatalf("expected nil with no messages, got %v", got)
	}
}

func TestLatestTodos_FindsMostRecent(t *testing.T) {
	old := models.NewAgentMessage(models.RoleAssistant, models.ToolCallContent{
		Name: "todo_write",
		Arguments: map[string]any{
			"todos": []any{map[string]any{"text": "Old", "status": "done"}},
		},
	})
	recent := models.NewAgentMessage(models.RoleAssistant, models.ToolCallContent{
		Name: "todo_write",
		Arguments: map[string]any{
			"todos": []any{
				map[string]any{"text": "New", "status": "pending"},
			},
		},
	})

	tasks := latestTodos([]models.AgentMessage{old, recent})
	if len(tasks) != 1 || tasks[0].Text != "New" {
		t.Fatalf("expected most recent todo list, got %+v", tasks)
	}
}
