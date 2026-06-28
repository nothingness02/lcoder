// pkg/llm/provider/cachepolicy_test.go
package provider

import "testing"

func TestCacheMarksDisabledWhenNone(t *testing.T) {
	marks := ComputeCacheMarks("none", []int{1}, 3, true)
	if marks.System || len(marks.MessageIdx) != 0 || marks.LastTool {
		t.Fatalf("expected no marks when cache=none, got %+v", marks)
	}
}

func TestCacheMarksDisabledWhenNotAnthropic(t *testing.T) {
	marks := ComputeCacheMarks("auto", []int{1}, 3, false)
	if marks.System || len(marks.MessageIdx) != 0 {
		t.Fatalf("expected no marks for non-anthropic, got %+v", marks)
	}
}

func TestCacheMarksUsesBreakpoints(t *testing.T) {
	marks := ComputeCacheMarks("auto", []int{0, 2}, 3, true)
	if !marks.System || !marks.LastTool {
		t.Fatalf("expected system+lastTool marks, got %+v", marks)
	}
	if len(marks.MessageIdx) != 2 || marks.MessageIdx[0] != 0 || marks.MessageIdx[1] != 2 {
		t.Fatalf("breakpoints wrong: %+v", marks.MessageIdx)
	}
}

func TestCacheMarksFallbackLastMsg(t *testing.T) {
	marks := ComputeCacheMarks("auto", nil, 4, true)
	if len(marks.MessageIdx) != 1 || marks.MessageIdx[0] != 3 {
		t.Fatalf("expected fallback to last index 3, got %+v", marks.MessageIdx)
	}
}
