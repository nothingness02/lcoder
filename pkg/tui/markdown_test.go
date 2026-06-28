package tui

import (
	"strings"
	"testing"
)

func TestRenderMarkdownBasic(t *testing.T) {
	out := renderMarkdown("# Title\n\nsome **bold** text", 80)
	if out == "" {
		t.Fatal("empty render")
	}
	if strings.Contains(out, "# Title") {
		t.Fatal("heading markdown not transformed")
	}
}

func TestRenderMarkdownCacheHit(t *testing.T) {
	a := renderMarkdownCached("hello `code`", 80)
	b := renderMarkdownCached("hello `code`", 80)
	if a != b {
		t.Fatal("cache returned different output for same input")
	}
}

func TestRenderMarkdownEmpty(t *testing.T) {
	if out := renderMarkdown("", 80); out != "" {
		t.Fatalf("empty input render = %q, want empty", out)
	}
}
