package observability

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lcoder/lcoder/pkg/models"
)

func TestPrometheusExporter(t *testing.T) {
	exp := NewPrometheusExporter()
	exp.ObserveUsage(models.LLMUsage{
		Provider:         "openai",
		Model:            "gpt-4o",
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		TotalCost:        0.002,
	})
	exp.ObserveTool("read", true, 12)
	exp.ObserveTool("bash", false, 100)
	exp.ObserveTurn(0, 500)

	rendered := exp.Render()
	if !strings.Contains(rendered, "lcoder_llm_total_tokens_total") {
		t.Fatalf("missing token metric")
	}
	if !strings.Contains(rendered, `lcoder_tool_executions_total{status="success",tool="read"}`) {
		t.Fatalf("missing tool success metric")
	}
	if !strings.Contains(rendered, `lcoder_tool_executions_total{status="error",tool="bash"}`) {
		t.Fatalf("missing tool error metric")
	}

	rec := httptest.NewRecorder()
	exp.Handler().ServeHTTP(rec, nil)
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "lcoder_llm_cost_usd_total") {
		t.Fatalf("handler missing cost metric")
	}
}
