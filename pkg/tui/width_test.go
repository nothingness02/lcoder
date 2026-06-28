package tui

import "testing"

func TestDisplayWidth(t *testing.T) {
	if got := displayWidth("abc"); got != 3 {
		t.Fatalf("ascii width = %d, want 3", got)
	}
	if got := displayWidth("你好"); got != 4 {
		t.Fatalf("cjk width = %d, want 4", got)
	}
}

func TestTruncateCells(t *testing.T) {
	if got := truncateCells("hello", 10, "…"); got != "hello" {
		t.Fatalf("no-trunc = %q, want hello", got)
	}
	got := truncateCells("你好世界", 5, "…")
	if displayWidth(got) > 5 {
		t.Fatalf("truncated width = %d, want <= 5", displayWidth(got))
	}
}

func TestTruncateCellsSafe(t *testing.T) {
	got := truncateCellsSafe("你好abc", 4)
	if displayWidth(got) > 4 {
		t.Fatalf("safe width = %d, want <= 4", displayWidth(got))
	}
}
