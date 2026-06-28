package observability

import (
	"fmt"
	"strings"
)

// TraceFormatter renders a simple text trace from observability records.
type TraceFormatter struct{}

// NewTraceFormatter creates a trace formatter.
func NewTraceFormatter() *TraceFormatter {
	return &TraceFormatter{}
}

// Render returns a human-readable trace.
func (f *TraceFormatter) Render(records []Record) string {
	var b strings.Builder
	indent := 0
	for _, r := range records {
		switch r.Type {
		case "span_start":
			if r.Span == nil {
				continue
			}
			prefix := strings.Repeat("  ", indent)
			b.WriteString(fmt.Sprintf("%s→ %s (%s)\n", prefix, r.Span.Name, ms(r.Span.StartTime)))
			indent++
		case "span_end":
			if r.Span == nil {
				continue
			}
			indent--
			if indent < 0 {
				indent = 0
			}
			prefix := strings.Repeat("  ", indent)
			duration := ""
			if r.Span.EndTime > 0 && r.Span.StartTime > 0 {
				duration = fmt.Sprintf(" %dms", r.Span.EndTime-r.Span.StartTime)
			}
			b.WriteString(fmt.Sprintf("%s← %s [%s]%s\n", prefix, r.Span.Name, r.Span.Status, duration))
		case "metric":
			if r.Metric == nil {
				continue
			}
			labels := ""
			if len(r.Metric.Labels) > 0 {
				labels = " " + formatLabels(r.Metric.Labels)
			}
			b.WriteString(fmt.Sprintf("  📊 %s = %g%s\n", r.Metric.Name, r.Metric.Value, labels))
		}
	}
	return b.String()
}

func ms(ts int64) string {
	return fmt.Sprintf("%dms", ts)
}

// FormatTrace reads observability records from a file and renders a trace.
func FormatTrace(path string) (string, error) {
	records, err := LoadFile(path)
	if err != nil {
		return "", err
	}
	formatter := NewTraceFormatter()
	return formatter.Render(records), nil
}
