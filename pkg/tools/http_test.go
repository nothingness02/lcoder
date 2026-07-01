package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lcoder/lcoder/pkg/models"
)

func TestHTTPExecutable(t *testing.T) {
	var received map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"content":[{"type":"text","text":"deployed"}],"details":{"version":"v1"},"terminate":false}`)
	}))
	defer ts.Close()

	exec := NewHTTPExecutable(HTTPConfig{
		Name:        "deploy",
		Endpoint:    ts.URL,
		Description: "Deploy service",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{"type": "string"},
			},
		},
	})

	result, err := exec.Execute(context.Background(), "call_1", map[string]any{"service": "api"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if received["name"] != "deploy" {
		t.Fatalf("expected deploy, got %v", received["name"])
	}
	if result.Content[0].(models.TextContent).Text != "deployed" {
		t.Fatalf("unexpected result text: %v", result.Content)
	}
	if result.Details["version"] != "v1" {
		t.Fatalf("unexpected details: %v", result.Details)
	}
}

func TestHTTPExecutableError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"service missing"}`)
	}))
	defer ts.Close()

	exec := NewHTTPExecutable(HTTPConfig{Name: "deploy", Endpoint: ts.URL})
	result, err := exec.Execute(context.Background(), "call_1", map[string]any{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !isErrorResult(result) {
		t.Fatal("expected error result")
	}
	if result.Content[0].(models.TextContent).Text != "service missing" {
		t.Fatalf("unexpected error text: %v", result.Content)
	}
}

func isErrorResult(result models.ToolExecutionResult) bool {
	if len(result.Content) == 0 {
		return false
	}
	if tr, ok := result.Content[0].(models.ToolResultContent); ok {
		return tr.IsError
	}
	// HTTP tool returns plain text content for errors.
	return strings.HasPrefix(result.Content[0].(models.TextContent).Text, "service missing")
}
