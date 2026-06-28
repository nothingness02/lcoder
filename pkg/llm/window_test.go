package llm

import (
	"context"
	"testing"
)

func TestModelWindowExactMatch(t *testing.T) {
	c := newTestClient()
	w, err := c.ModelWindow(context.Background(), "openai", "gpt-4o")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w != 128000 {
		t.Fatalf("expected 128000, got %d", w)
	}
}

func TestModelWindowPrefixMatch(t *testing.T) {
	c := newTestClient()
	// Snapshot id is "claude-sonnet-4-20250514"; a shorter query prefix-matches it.
	w, _ := c.ModelWindow(context.Background(), "anthropic", "claude-sonnet-4")
	if w != 200000 {
		t.Fatalf("expected 200000 via prefix, got %d", w)
	}
}

func TestModelWindowProviderMismatch(t *testing.T) {
	c := newTestClient()
	w, _ := c.ModelWindow(context.Background(), "azure", "gpt-4o")
	if w != 0 {
		t.Fatalf("expected 0 for provider mismatch, got %d", w)
	}
}

func TestModelWindowUnknownModel(t *testing.T) {
	c := newTestClient()
	w, err := c.ModelWindow(context.Background(), "openai", "no-such-model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w != 0 {
		t.Fatalf("expected 0 for unknown model, got %d", w)
	}
}
