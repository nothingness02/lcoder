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

func TestCachePolicyNone_NoBreakpoints(t *testing.T) {
	mgr := NewManager(TokenBudget{MaxTotal: 10000, ReserveOutput: 0}, WithCacheHintPolicy(CachePolicyNone))
	mgr.SetSystemPrompt(strings.Repeat("sys ", 300)) // > 256 tokens
	mgr.ReplaceRecent([]models.AgentMessage{
		models.NewAgentMessage(models.RoleUser, models.TextContent{Text: "hi"}),
	})
	req, err := mgr.BuildTurnRequest(models.ModelRef{ID: "test"}, nil)
	if err != nil {
		t.Fatalf("BuildTurnRequest: %v", err)
	}
	if len(req.CacheBreakpoints) != 0 {
		t.Fatalf("expected no breakpoints with none policy, got %v", req.CacheBreakpoints)
	}
}

func TestCachePolicyAggressive_PrefixAlwaysAnchored(t *testing.T) {
	small := "sys " // < 256 tokens
	mgrDef := NewManager(TokenBudget{MaxTotal: 10000, ReserveOutput: 0})
	mgrDef.SetSystemPrompt(small)
	mgrDef.ReplaceRecent([]models.AgentMessage{
		models.NewAgentMessage(models.RoleAssistant, models.TextContent{Text: "hello"}),
		models.NewAgentMessage(models.RoleUser, models.TextContent{Text: "hi"}),
	})
	reqDef, _ := mgrDef.BuildTurnRequest(models.ModelRef{ID: "test"}, nil)
	if containsBreakpoint(reqDef.CacheBreakpoints, 0) {
		t.Fatalf("default policy should not breakpoint tiny prefix at 0, got %v", reqDef.CacheBreakpoints)
	}

	mgrAgg := NewManager(TokenBudget{MaxTotal: 10000, ReserveOutput: 0}, WithCacheHintPolicy(CachePolicyAggressive))
	mgrAgg.SetSystemPrompt(small)
	mgrAgg.ReplaceRecent([]models.AgentMessage{
		models.NewAgentMessage(models.RoleAssistant, models.TextContent{Text: "hello"}),
		models.NewAgentMessage(models.RoleUser, models.TextContent{Text: "hi"}),
	})
	reqAgg, _ := mgrAgg.BuildTurnRequest(models.ModelRef{ID: "test"}, nil)
	if !containsBreakpoint(reqAgg.CacheBreakpoints, 0) {
		t.Fatalf("aggressive policy should breakpoint any non-empty prefix at 0, got %v", reqAgg.CacheBreakpoints)
	}
}

func TestCacheHintSkip_NoBreakpointOnBlock(t *testing.T) {
	mgr := NewManager(TokenBudget{MaxTotal: 10000, ReserveOutput: 0})
	mgr.SetSystemPrompt(strings.Repeat("sys ", 300))
	retrieval := NewBlock(BlockRetrieval, "retrieval", StabilityStable, 50,
		models.NewAgentMessage(models.RoleSystem, models.TextContent{Text: "rag result"}))
	retrieval.CacheHint = CacheHintSkip
	mgr.SetBlock(retrieval)
	mgr.ReplaceRecent([]models.AgentMessage{
		models.NewAgentMessage(models.RoleUser, models.TextContent{Text: "hi"}),
	})
	req, _ := mgr.BuildTurnRequest(models.ModelRef{ID: "test"}, nil)
	for _, bp := range req.CacheBreakpoints {
		// retrieval block messages start at index 0 because it is not a system block,
		// but with CacheHintSkip it must not be anchored.
		if bp == 0 {
			t.Fatalf("CacheHintSkip block got breakpoint at 0: %v", req.CacheBreakpoints)
		}
	}
}

func containsBreakpoint(bps []int, idx int) bool {
	for _, b := range bps {
		if b == idx {
			return true
		}
	}
	return false
}
