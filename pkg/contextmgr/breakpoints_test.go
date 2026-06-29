package contextmgr

import (
	"strings"
	"testing"

	"github.com/lcoder/lcoder/pkg/models"
)

func TestBuildTurnRequestCacheBreakpoints(t *testing.T) {
	m := NewManager(TokenBudget{MaxTotal: 2000, TargetTotal: 1800, ReserveOutput: 200})
	m.SetSystemPrompt(strings.Repeat("a", 1100))
	m.AppendRecent(models.NewAgentMessage(models.RoleUser, models.TextContent{Text: "hello"}))
	m.AppendRecent(models.NewAgentMessage(models.RoleAssistant, models.TextContent{Text: "hi"}))

	req, err := m.BuildTurnRequest(models.ModelRef{Provider: "anthropic", ID: "claude"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(req.CacheBreakpoints) == 0 {
		t.Fatalf("expected breakpoints, got %v", req.CacheBreakpoints)
	}
	foundFirst := false
	foundLastUser := false
	for _, bp := range req.CacheBreakpoints {
		if bp == 0 {
			foundFirst = true
		}
		if bp == 0 {
			foundLastUser = true
		}
	}
	if !foundFirst {
		t.Fatalf("expected breakpoint at first message, got %v", req.CacheBreakpoints)
	}
	if !foundLastUser {
		t.Fatalf("expected breakpoint at last user message, got %v", req.CacheBreakpoints)
	}
}

func TestExplicitCacheHintBreakpoint(t *testing.T) {
	m := NewManager(TokenBudget{MaxTotal: 2000, TargetTotal: 1800, ReserveOutput: 200})
	b := NewBlock(BlockRecent, "recent", StabilityDynamic, 100,
		models.NewAgentMessage(models.RoleUser, models.TextContent{Text: "hello"}))
	b.CacheHint = CacheHintBreakpoint
	m.SetBlock(b)

	req, err := m.BuildTurnRequest(models.ModelRef{Provider: "anthropic", ID: "claude"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, bp := range req.CacheBreakpoints {
		if bp == 0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected explicit breakpoint, got %v", req.CacheBreakpoints)
	}
}
