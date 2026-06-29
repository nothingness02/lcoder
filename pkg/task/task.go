// Package task defines the agent task-list schema shared by the todo_write tool
// and the TUI sidebar. It is the single source of truth for task shape and the
// tool's registered name.
package task

import "fmt"

// Status is a task's lifecycle state.
type Status string

const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
)

// ToolName is the registered name of the task-declaration tool.
const ToolName = "todo_write"

// Task is one item in the agent's declared plan.
type Task struct {
	Text   string
	Status Status
}

func validStatus(s Status) bool {
	return s == StatusPending || s == StatusInProgress || s == StatusDone
}

// Parse converts the decoded `todos` argument (as delivered in a tool call's
// args or a ToolExecutionStartEvent.Args) into a validated task slice. raw is
// expected to be a []any of map[string]any with non-empty "text" and a valid
// "status".
func Parse(raw any) ([]Task, error) {
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("todos must be an array, got %T", raw)
	}
	out := make([]Task, 0, len(items))
	for i, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("todos[%d] must be an object, got %T", i, it)
		}
		text, _ := m["text"].(string)
		if text == "" {
			return nil, fmt.Errorf("todos[%d].text must be a non-empty string", i)
		}
		statusStr, _ := m["status"].(string)
		st := Status(statusStr)
		if !validStatus(st) {
			return nil, fmt.Errorf("todos[%d].status %q invalid (want pending|in_progress|done)", i, statusStr)
		}
		out = append(out, Task{Text: text, Status: st})
	}
	return out, nil
}

// Counts tallies tasks by status.
func Counts(tasks []Task) (done, inProgress, pending int) {
	for _, t := range tasks {
		switch t.Status {
		case StatusDone:
			done++
		case StatusInProgress:
			inProgress++
		case StatusPending:
			pending++
		}
	}
	return
}
