// pkg/llm/catalog/refresh_test.go
package catalog

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

const fakeModelsDev = `{
  "openai": {"models": {
    "gpt-4o": {"name":"GPT-4o","limit":{"context":111111},
      "cost":{"input":2.5,"output":10,"cache_read":1.25,"cache_write":2.5},
      "tool_call":true}
  }}
}`

func TestRefreshMergesModelsDev(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(fakeModelsDev))
	}))
	defer ts.Close()

	cache := filepath.Join(t.TempDir(), "models.json")
	c := New(Options{Refresh: false, SourceURL: ts.URL})
	c.refresh(cache) // synchronous in test
	if w := c.Window("openai", "gpt-4o"); w != 111111 {
		t.Fatalf("refresh did not override window: got %d", w)
	}
}

func TestRefreshFailureKeepsSnapshot(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "models.json")
	c := New(Options{Refresh: false, SourceURL: "http://127.0.0.1:1"}) // nothing listening
	c.refresh(cache)
	if w := c.Window("openai", "gpt-4o"); w != 128000 {
		t.Fatalf("snapshot window lost after failed refresh: got %d", w)
	}
}

func TestRefreshPreservesOverrides(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(fakeModelsDev))
	}))
	defer ts.Close()
	cache := filepath.Join(t.TempDir(), "models.json")
	c := New(Options{Refresh: false, SourceURL: ts.URL, Overrides: []Entry{
		{ID: "gpt-4o", Provider: "openai", ContextWindow: 999},
	}})
	c.refresh(cache)
	if w := c.Window("openai", "gpt-4o"); w != 999 {
		t.Fatalf("override lost after refresh: got %d", w)
	}
}
