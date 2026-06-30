package sandbox

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"time"
)

// passthrough performs no isolation; it mirrors the historical bash behavior
// (sh -c, inherited environment) and an allow-all filesystem policy.
type passthrough struct {
	network *passthroughNetwork
}

func (p *passthrough) Name() string                 { return "passthrough" }
func (p *passthrough) Network() NetworkPolicy       { return p.network }
func (p *passthrough) Filesystem() FilesystemPolicy { return allowAllFS{} }

func (p *passthrough) Exec(ctx context.Context, spec ExecSpec) (ExecResult, error) {
	timeout := spec.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	name, args := shellArgs(spec.Command)
	cmd := exec.CommandContext(runCtx, name, args...)
	cmd.Dir = spec.Cwd
	env := spec.Env
	if env == nil {
		env = os.Environ()
	}
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	timedOut := runCtx.Err() == context.DeadlineExceeded
	res := ExecResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode(err),
		TimedOut: timedOut,
	}
	if timedOut {
		return res, nil
	}
	return res, err
}

// shellArgs resolves the shell invocation, honoring $SHELL with an "sh" fallback.
func shellArgs(command string) (string, []string) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "sh"
	}
	return shell, []string{"-c", command}
}

// exitCode extracts a process exit code from a run error (-1 for non-exit errors).
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return -1
}

var _ Sandbox = (*passthrough)(nil)
