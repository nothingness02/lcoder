package llm

import (
	"context"
	"testing"
)

func TestHealth(t *testing.T) {
	client := newTestClient()
	resp, err := client.Health(context.Background())
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("unexpected status: %v", resp)
	}
}

func TestListModels(t *testing.T) {
	client := newTestClient()
	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("models: %v", err)
	}
	var found bool
	for _, m := range models {
		if m.ID == "gpt-4o" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected gpt-4o in catalog models, got %v", models)
	}
}
