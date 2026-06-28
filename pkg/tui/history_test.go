package tui

import "testing"

func TestHistoryNavigation(t *testing.T) {
	h := newInputHistory()
	h.add("first")
	h.add("second")
	if got := h.prev(); got != "second" {
		t.Fatalf("prev = %q, want second", got)
	}
	if got := h.prev(); got != "first" {
		t.Fatalf("prev = %q, want first", got)
	}
	if got := h.next(); got != "second" {
		t.Fatalf("next = %q, want second", got)
	}
}

func TestHistoryResetOnAdd(t *testing.T) {
	h := newInputHistory()
	h.add("a")
	_ = h.prev()
	h.add("b")
	if got := h.prev(); got != "b" {
		t.Fatalf("prev after add = %q, want b", got)
	}
}
