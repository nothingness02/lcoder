// Package main implements a custom Lcoder observability exporter.
// It writes metrics to stdout as JSONL.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/lcoder/lcoder/pkg/observability"
)

// StdoutExporter writes observability records to stdout.
type StdoutExporter struct{}

// NewStdoutExporter creates a new stdout exporter.
func NewStdoutExporter() *StdoutExporter {
	return &StdoutExporter{}
}

// Export writes a record to stdout.
func (e *StdoutExporter) Export(record observability.Record) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// Close is a no-op.
func (e *StdoutExporter) Close() error { return nil }

func main() {
	// Register the exporter factory so it can be selected via config.
	observability.DefaultRegistry().Register("stdout", func(cfg map[string]any, output string) (observability.Exporter, error) {
		return NewStdoutExporter(), nil
	})
	fmt.Fprintln(os.Stderr, "stdout exporter registered")
}
