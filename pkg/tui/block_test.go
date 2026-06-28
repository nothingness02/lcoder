package tui

import (
	"strings"
	"testing"
)

func TestRenderUserBlock(t *testing.T) {
	b := block{kind: blockUser, raw: "hello world"}
	out := b.render(80, false)
	if !strings.Contains(out, "hello world") {
		t.Fatalf("user block missing text: %q", out)
	}
}

func TestRenderAssistantBlockMarkdown(t *testing.T) {
	b := block{kind: blockAssistant, raw: "# Hi\n\ntext"}
	out := b.render(80, false)
	if strings.Contains(out, "# Hi") {
		t.Fatal("assistant block did not render markdown")
	}
}

func TestRenderToolBlockCompactVsExpanded(t *testing.T) {
	b := block{kind: blockTool, toolName: "bash", toolArgs: `{"command":"ls"}`, raw: "file1\nfile2"}
	compact := b.render(80, false)
	expanded := b.render(80, true)
	if compact == expanded {
		t.Fatal("expanded should differ from compact")
	}
}

func TestRenderSystemBlock(t *testing.T) {
	b := block{kind: blockSystem, raw: "switched mode"}
	if out := b.render(80, false); !strings.Contains(out, "switched mode") {
		t.Fatalf("system block missing text: %q", out)
	}
}
