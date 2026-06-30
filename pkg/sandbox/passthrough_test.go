package sandbox

import (
	"context"
	"strings"
	"testing"
)

func TestPassthroughExecRunsCommand(t *testing.T) {
	sb := &passthrough{network: &passthroughNetwork{dialer: nil}}
	res, err := sb.Exec(context.Background(), ExecSpec{Command: "go version", Cwd: "."})
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if !strings.HasPrefix(res.Stdout, "go version") {
		t.Fatalf("expected go version on stdout, got %q / stderr %q", res.Stdout, res.Stderr)
	}
	if res.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", res.ExitCode)
	}
}

func TestPassthroughExecCapturesExitCode(t *testing.T) {
	sb := &passthrough{network: &passthroughNetwork{dialer: nil}}
	res, _ := sb.Exec(context.Background(), ExecSpec{Command: "exit 3", Cwd: "."})
	if res.ExitCode != 3 {
		t.Fatalf("expected exit 3, got %d", res.ExitCode)
	}
}

func TestPassthroughMetadata(t *testing.T) {
	sb := &passthrough{network: &passthroughNetwork{dialer: nil}}
	if sb.Name() != "passthrough" {
		t.Fatalf("name = %q", sb.Name())
	}
	if err := sb.Filesystem().Check("/anything", FSWrite); err != nil {
		t.Fatalf("passthrough fs should allow, got %v", err)
	}
}
