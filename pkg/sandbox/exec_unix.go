//go:build !windows

package sandbox

import (
	"os/exec"
	"syscall"
)

// applyLimits places the child in its own process group so the whole tree can be
// signaled on timeout. Numeric rlimits (RLIMIT_AS/RLIMIT_CPU) are intentionally
// deferred — they carry cross-platform and OOM-killer hazards (spec §10) and are
// a follow-up; orphan safety is the guarantee provided here.
func applyLimits(cmd *exec.Cmd, _ ResourceLimits) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killGroup sends SIGKILL to the entire process group (negative PID), reaping
// backgrounded grandchildren that a leader-only kill would orphan.
func killGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
