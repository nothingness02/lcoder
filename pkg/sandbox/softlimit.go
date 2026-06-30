package sandbox

import (
	"bytes"
	"context"
	"os/exec"
	"time"
)

// softLimit applies best-effort isolation with no external dependencies: env
// scrubbing, timeout, output capping, and (on Unix) process-group orphan
// cleanup. It enforces the in-process network/filesystem policies truly, but the
// subprocess plane is best-effort only — it is NOT a security boundary (spec §4).
type softLimit struct {
	network  *allowlistNetwork
	fs       *restrictedFS
	envAllow []string
}

func (s *softLimit) Name() string                 { return "soft-limit" }
func (s *softLimit) Network() NetworkPolicy       { return s.network }
func (s *softLimit) Filesystem() FilesystemPolicy { return s.fs }

func (s *softLimit) Exec(ctx context.Context, spec ExecSpec) (ExecResult, error) {
	timeout := spec.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	name, args := shellArgs(spec.Command)
	// We manage the kill ourselves (not CommandContext) so we can signal the
	// whole process group and reap orphaned grandchildren.
	cmd := exec.Command(name, args...)
	cmd.Dir = spec.Cwd
	cmd.Env = scrubEnv(spec.Env, s.envAllow)
	// After we kill the process, a backgrounded grandchild may still hold the
	// stdout/stderr pipe open, which would block Wait indefinitely. WaitDelay
	// bounds that wait: once the process is gone, Wait aborts the I/O copy
	// shortly after rather than hanging on the orphan's pipe handle.
	cmd.WaitDelay = 100 * time.Millisecond

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	applyLimits(cmd, spec.Limits) // platform-specific (Setpgid on Unix; no-op on Windows)

	if err := cmd.Start(); err != nil {
		return ExecResult{}, err
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	var runErr error
	timedOut := false
	select {
	case <-runCtx.Done():
		timedOut = runCtx.Err() == context.DeadlineExceeded
		killGroup(cmd)
		<-done // reap
	case runErr = <-done:
	}

	res := ExecResult{
		Stdout:   capOutput(stdout.String(), spec.Limits.MaxOutputBytes),
		Stderr:   capOutput(stderr.String(), spec.Limits.MaxOutputBytes),
		ExitCode: exitCode(runErr),
		TimedOut: timedOut,
	}
	if timedOut {
		return res, nil
	}
	return res, runErr
}

// capOutput truncates s to max bytes (0 = unlimited), appending a marker.
func capOutput(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "\n[output truncated]"
}

var _ Sandbox = (*softLimit)(nil)
