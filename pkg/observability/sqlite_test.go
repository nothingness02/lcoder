package observability

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSQLiteExporter(t *testing.T) {
	dir, err := os.MkdirTemp("", "lcoder-obs-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "obs.db")
	exp, err := NewSQLiteExporter(path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer exp.Close()

	record := Record{
		Type: "metric",
		Metric: &Metric{
			Timestamp: 1,
			Name:      "llm_total_tokens",
			Value:     150,
			Labels:    map[string]string{"model": "gpt-4o"},
		},
	}
	if err := exp.Export(record); err != nil {
		t.Fatalf("export: %v", err)
	}

	summary, err := exp.QuerySummary()
	if err != nil {
		t.Fatalf("query summary: %v", err)
	}
	if summary["llm_total_tokens"] != 150 {
		t.Fatalf("expected 150, got %v", summary)
	}
}
