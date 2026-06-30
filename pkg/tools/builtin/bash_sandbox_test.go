package builtin

import (
	"context"
	"testing"

	"github.com/lcoder/lcoder/pkg/models"
	"github.com/lcoder/lcoder/pkg/sandbox"
)

func TestBashUsesSandboxExec(t *testing.T) {
	b := NewBash("/tmp").(*Bash)
	fake := sandbox.NewFakeSandbox()
	fake.Result = sandbox.ExecResult{Stdout: "hello"}
	b.UseSandbox(fake)

	res, err := b.Execute(context.Background(), "c1", map[string]any{"command": "echo hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.Calls) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(fake.Calls))
	}
	if fake.Calls[0].Command != "echo hello" {
		t.Fatalf("command = %q", fake.Calls[0].Command)
	}
	txt := res.Content[0].(models.TextContent).Text
	if txt != "hello" {
		t.Fatalf("output = %q, want %q", txt, "hello")
	}
}

func TestBashSandboxNonZeroExitReturnsError(t *testing.T) {
	b := NewBash("/tmp").(*Bash)
	fake := sandbox.NewFakeSandbox()
	fake.Result = sandbox.ExecResult{Stderr: "boom", ExitCode: 1}
	b.UseSandbox(fake)

	_, err := b.Execute(context.Background(), "c1", map[string]any{"command": "false"})
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
}

func TestBashSandboxTimeoutMarksOutput(t *testing.T) {
	b := NewBash("/tmp").(*Bash)
	fake := sandbox.NewFakeSandbox()
	fake.Result = sandbox.ExecResult{Stdout: "partial", TimedOut: true}
	b.UseSandbox(fake)

	res, err := b.Execute(context.Background(), "c1", map[string]any{"command": "sleep 99"})
	if err == nil {
		t.Fatal("expected error on timeout")
	}
	txt := res.Content[0].(models.TextContent).Text
	if txt != "partial\n[command timed out]" {
		t.Fatalf("output = %q", txt)
	}
}
