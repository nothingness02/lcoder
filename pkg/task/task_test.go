package task

import "testing"

func argsTodos(items ...map[string]any) any {
	out := make([]any, len(items))
	for i, it := range items {
		out[i] = it
	}
	return out
}

func TestParseValid(t *testing.T) {
	raw := argsTodos(
		map[string]any{"text": "read auth", "status": "done"},
		map[string]any{"text": "split handler", "status": "in_progress"},
		map[string]any{"text": "write tests", "status": "pending"},
	)
	tasks, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("want 3 tasks, got %d", len(tasks))
	}
	if tasks[1].Text != "split handler" || tasks[1].Status != StatusInProgress {
		t.Fatalf("task[1] wrong: %+v", tasks[1])
	}
}

func TestParseRejectsBadStatus(t *testing.T) {
	_, err := Parse(argsTodos(map[string]any{"text": "x", "status": "blocked"}))
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestParseRejectsEmptyText(t *testing.T) {
	_, err := Parse(argsTodos(map[string]any{"text": "", "status": "pending"}))
	if err == nil {
		t.Fatal("expected error for empty text")
	}
}

func TestParseRejectsNonArray(t *testing.T) {
	if _, err := Parse("nope"); err == nil {
		t.Fatal("expected error for non-array todos")
	}
}

func TestCounts(t *testing.T) {
	tasks := []Task{
		{Text: "a", Status: StatusDone},
		{Text: "b", Status: StatusDone},
		{Text: "c", Status: StatusInProgress},
		{Text: "d", Status: StatusPending},
	}
	done, inProgress, pending := Counts(tasks)
	if done != 2 || inProgress != 1 || pending != 1 {
		t.Fatalf("counts wrong: %d/%d/%d", done, inProgress, pending)
	}
}
