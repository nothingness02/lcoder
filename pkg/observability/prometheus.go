package observability

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/lcoder/lcoder/pkg/models"
)

// PrometheusExporter aggregates metrics and exposes them via an HTTP handler.
type PrometheusExporter struct {
	mu      sync.RWMutex
	metrics map[string]*prometheusMetric
}

// Export writes a record to the Prometheus exporter in memory.
func (p *PrometheusExporter) Export(record Record) error {
	switch record.Type {
	case "metric":
		if record.Metric == nil {
			return nil
		}
		m := record.Metric
		switch m.Name {
		case "llm_prompt_tokens":
			labels := map[string]string{"provider": m.Labels["provider"], "model": m.Labels["model"]}
			p.counter("lcoder_llm_prompt_tokens_total", "Total prompt tokens", labels, m.Value)
		case "llm_completion_tokens":
			labels := map[string]string{"provider": m.Labels["provider"], "model": m.Labels["model"]}
			p.counter("lcoder_llm_completion_tokens_total", "Total completion tokens", labels, m.Value)
		case "llm_total_tokens":
			labels := map[string]string{"provider": m.Labels["provider"], "model": m.Labels["model"]}
			p.counter("lcoder_llm_total_tokens_total", "Total tokens", labels, m.Value)
		case "llm_cost_usd":
			labels := map[string]string{"provider": m.Labels["provider"], "model": m.Labels["model"]}
			p.counter("lcoder_llm_cost_usd_total", "Total LLM cost in USD", labels, m.Value)
		case "tool_execution_duration_ms":
			labels := map[string]string{"tool": m.Labels["tool"], "status": m.Labels["status"]}
			p.histogram("lcoder_tool_execution_duration_ms", "Tool execution duration in ms", labels, m.Value)
		case "agent_turn_duration_ms":
			labels := map[string]string{"turn": m.Labels["turn"]}
			p.histogram("lcoder_agent_turn_duration_ms", "Agent turn duration in ms", labels, m.Value)
		}
	}
	return nil
}

type prometheusMetric struct {
	type_   string
	help    string
	values  []promValue
}

type promValue struct {
	labels map[string]string
	value  float64
}

// NewPrometheusExporter creates a Prometheus metrics exporter.
func NewPrometheusExporter() *PrometheusExporter {
	return &PrometheusExporter{
		metrics: make(map[string]*prometheusMetric),
	}
}

// ObserveUsage records LLM usage metrics (convenience method).
func (p *PrometheusExporter) ObserveUsage(usage models.LLMUsage) {
	labels := map[string]string{
		"provider": usage.Provider,
		"model":    usage.Model,
	}
	p.counter("lcoder_llm_requests_total", "Total LLM requests", labels, 1)
	p.counter("lcoder_llm_prompt_tokens_total", "Total prompt tokens", labels, float64(usage.PromptTokens))
	p.counter("lcoder_llm_completion_tokens_total", "Total completion tokens", labels, float64(usage.CompletionTokens))
	p.counter("lcoder_llm_total_tokens_total", "Total tokens", labels, float64(usage.TotalTokens))
	p.counter("lcoder_llm_cache_read_tokens_total", "Cache read tokens", labels, float64(usage.CacheReadTokens))
	p.counter("lcoder_llm_cache_write_tokens_total", "Cache write tokens", labels, float64(usage.CacheWriteTokens))
	p.counter("lcoder_llm_cost_usd_total", "Total LLM cost in USD", labels, usage.TotalCost)
}

// ObserveTool records tool execution metrics.
func (p *PrometheusExporter) ObserveTool(toolName string, success bool, durationMs int64) {
	status := "success"
	if !success {
		status = "error"
	}
	labels := map[string]string{"tool": toolName, "status": status}
	p.counter("lcoder_tool_executions_total", "Total tool executions", labels, 1)
	p.histogram("lcoder_tool_execution_duration_ms", "Tool execution duration in ms", labels, float64(durationMs))
}

// ObserveTurn records turn duration.
func (p *PrometheusExporter) ObserveTurn(turn int, durationMs int64) {
	labels := map[string]string{"turn": fmt.Sprintf("%d", turn)}
	p.histogram("lcoder_agent_turn_duration_ms", "Agent turn duration in ms", labels, float64(durationMs))
}

func (p *PrometheusExporter) counter(name, help string, labels map[string]string, value float64) {
	p.record(name, "counter", help, labels, value)
}

func (p *PrometheusExporter) histogram(name, help string, labels map[string]string, value float64) {
	p.record(name, "histogram", help, labels, value)
}

func (p *PrometheusExporter) record(name, type_, help string, labels map[string]string, value float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	m, ok := p.metrics[name]
	if !ok {
		m = &prometheusMetric{type_: type_, help: help}
		p.metrics[name] = m
	}
	m.values = append(m.values, promValue{labels: labels, value: value})
}

// Handler returns an http.Handler that serves Prometheus metrics.
func (p *PrometheusExporter) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		data := p.Render()
		_, _ = w.Write([]byte(data))
	})
}

// Close is a no-op for the PrometheusExporter.
func (p *PrometheusExporter) Close() error { return nil }

// Render returns the Prometheus exposition format text.
func (p *PrometheusExporter) Render() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var b strings.Builder
	for name, m := range p.metrics {
		b.WriteString(fmt.Sprintf("# HELP %s %s\n", name, m.help))
		b.WriteString(fmt.Sprintf("# TYPE %s %s\n", name, m.type_))
		for _, v := range m.values {
			labelStr := formatLabels(v.labels)
			if labelStr != "" {
				b.WriteString(fmt.Sprintf("%s%s %g\n", name, labelStr, v.value))
			} else {
				b.WriteString(fmt.Sprintf("%s %g\n", name, v.value))
			}
		}
	}
	return b.String()
}

func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	var parts []string
	// Sort keys for deterministic output in tests.
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=\"%s\"", k, escapeLabel(labels[k])))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// ServeMetrics runs an HTTP server on the given port that exposes Prometheus metrics.
func ServeMetrics(port string) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", NewPrometheusExporter().Handler())
	return http.ListenAndServe(":"+port, mux)
}

func escapeLabel(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}
