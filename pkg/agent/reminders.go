package agent

import (
	"fmt"

	"github.com/lcoder/lcoder/pkg/models"
	"github.com/lcoder/lcoder/pkg/task"
)

// UnresolvedTodosReminder nudges the model to keep working while the most recent
// todo_write declares unfinished items. Returns nil when there is no todo list or
// everything is done — so it injects nothing on those turns.
func UnresolvedTodosReminder(messages []models.AgentMessage) []string {
	tasks := latestTodos(messages)
	if len(tasks) == 0 {
		return nil
	}
	done, inProgress, pending := task.Counts(tasks)
	remaining := inProgress + pending
	if remaining == 0 {
		return nil
	}
	return []string{fmt.Sprintf(
		"You have %d unfinished todo item(s) (%d done). Continue working toward them; do not stop until they are complete or you report a blocker.",
		remaining, done)}
}

func latestTodos(messages []models.AgentMessage) []task.Task {
	for i := len(messages) - 1; i >= 0; i-- {
		for _, tc := range messages[i].ToolCalls() {
			if tc.Name == task.ToolName {
				if ts, err := task.Parse(tc.Arguments["todos"]); err == nil {
					return ts
				}
			}
		}
	}
	return nil
}
