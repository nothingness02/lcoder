package tools

import (
	"net/http"
	"testing"

	"github.com/lcoder/lcoder/pkg/sandbox"
)

func TestHTTPUseSandboxSetsTransport(t *testing.T) {
	h := NewHTTPExecutable(HTTPConfig{Name: "x"})
	h.UseSandbox(sandbox.NewFakeSandbox())

	tr, ok := h.client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", h.client.Transport)
	}
	if tr.DialContext == nil {
		t.Fatal("expected DialContext to be set from sandbox network policy")
	}
}
