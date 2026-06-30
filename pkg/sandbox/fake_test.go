package sandbox

import (
	"context"
	"errors"
	"testing"
)

func TestFakeSandboxRecordsAndReturns(t *testing.T) {
	f := NewFakeSandbox()
	f.Result = ExecResult{Stdout: "canned", ExitCode: 0}

	res, err := f.Exec(context.Background(), ExecSpec{Command: "anything", Cwd: "/tmp"})
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if res.Stdout != "canned" {
		t.Fatalf("expected canned result, got %q", res.Stdout)
	}
	if len(f.Calls) != 1 || f.Calls[0].Command != "anything" {
		t.Fatalf("expected recorded call, got %+v", f.Calls)
	}
}

func TestFakeSandboxReturnsErr(t *testing.T) {
	f := NewFakeSandbox()
	f.Err = errors.New("boom")
	_, err := f.Exec(context.Background(), ExecSpec{})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected boom, got %v", err)
	}
}

func TestFakeSandboxSatisfiesInterface(t *testing.T) {
	var _ Sandbox = NewFakeSandbox()
}
