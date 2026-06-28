package observability

import "testing"

func TestComputeStats(t *testing.T) {
	records := []Record{
		{Type: "span_start", Span: &Span{SpanID: "root", Name: "agent_run", StartTime: 0}},
		{Type: "span_start", Span: &Span{SpanID: "llm1", Name: "llm_response", ParentID: "turn1"}},
		{Type: "span_end", Span: &Span{SpanID: "llm1", Name: "llm_response", EndTime: 100}},
		{Type: "metric", Metric: &Metric{Name: "llm_total_tokens", Value: 150}},
		{Type: "metric", Metric: &Metric{Name: "llm_cost_usd", Value: 0.002}},
		{Type: "span_start", Span: &Span{SpanID: "tool1", Name: "tool_ls", ParentID: "turn1"}},
		{Type: "span_end", Span: &Span{SpanID: "tool1", Name: "tool_ls", EndTime: 50, Status: SpanOK}},
		{Type: "span_end", Span: &Span{SpanID: "root", Name: "agent_run", EndTime: 1000}},
	}

	stats := ComputeStats(records)
	if stats.LLMCalls != 1 {
		t.Fatalf("expected 1 llm call, got %d", stats.LLMCalls)
	}
	if stats.ToolCalls != 1 {
		t.Fatalf("expected 1 tool call, got %d", stats.ToolCalls)
	}
	if stats.TotalTokens != 150 {
		t.Fatalf("expected 150 tokens, got %d", stats.TotalTokens)
	}
	if stats.TotalCost != 0.002 {
		t.Fatalf("expected cost 0.002, got %f", stats.TotalCost)
	}
	if stats.TotalDurationMs != 1000 {
		t.Fatalf("expected duration 1000ms, got %d", stats.TotalDurationMs)
	}
}
