//go:build windows

package sandbox

import "os/exec"

// applyLimits is a documented degradation on Windows: process-tree (Job Object)
// isolation and numeric rlimits are not yet implemented. Timeout and output
// capping from the cross-platform path still apply (spec §10). Job Object support
// is a follow-up.
func applyLimits(_ *exec.Cmd, _ ResourceLimits) {}

// killGroup terminates the direct process. Backgrounded grandchildren may survive
// until Job Object support lands; this is the explicit Windows degradation.
func killGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
