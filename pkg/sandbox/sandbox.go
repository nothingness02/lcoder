// Package sandbox abstracts isolation of sensitive environment interactions
// (command execution and network access) behind a swappable Sandbox interface.
// Backends range from a no-op Passthrough to an OS-level SoftLimit to reserved
// Container/Remote backends. See docs/superpowers/specs/2026-06-30-sandbox-design.md.
package sandbox

import (
	"context"
	"net"
	"time"
)

// Sandbox isolates sensitive environment interactions. Implementations are
// selected via New and consumed by tools without knowledge of the backend.
type Sandbox interface {
	// Exec runs a command under the backend's isolation policy (subprocess plane).
	Exec(ctx context.Context, spec ExecSpec) (ExecResult, error)
	// Network serves the in-process plane (DialContext) and the subprocess plane
	// (SubprocessConfig).
	Network() NetworkPolicy
	// Filesystem serves the in-process plane (Check) and the subprocess plane
	// (SubprocessMounts).
	Filesystem() FilesystemPolicy
	// Name identifies the backend for logging and telemetry.
	Name() string
}

// ExecSpec describes one controlled command execution.
type ExecSpec struct {
	Command string        // command line passed to "sh -c"
	Cwd     string        // working directory
	Env     []string      // KEY=VALUE entries; backend may filter to an allowlist
	Timeout time.Duration // 0 means the backend default (60s)
	Limits  ResourceLimits
}

// ResourceLimits bounds a sandboxed process. Enforcement is backend- and
// platform-dependent; unsupported limits degrade explicitly (see spec §10).
type ResourceLimits struct {
	MaxMemoryMB    int
	MaxCPUSeconds  int
	MaxOutputBytes int
}

// ExecResult separates stdout and stderr so callers can distinguish normal
// output from error streams. Combined reproduces a merged-output contract.
type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	TimedOut bool
}

// Combined merges stdout and stderr, joining with a newline only when both are
// non-empty.
func (r ExecResult) Combined() string {
	switch {
	case r.Stderr == "":
		return r.Stdout
	case r.Stdout == "":
		return r.Stderr
	default:
		return r.Stdout + "\n" + r.Stderr
	}
}

// FSOp is a filesystem access mode checked by FilesystemPolicy.
type FSOp int

const (
	FSRead FSOp = iota
	FSWrite
)

// Mount describes a subprocess-plane filesystem binding for container backends.
type Mount struct {
	Source   string
	Target   string
	ReadOnly bool
}

// SubprocessNetConfig carries subprocess-plane network wiring. Empty fields mean
// the backend does not constrain subprocess network (e.g. Passthrough).
type SubprocessNetConfig struct {
	ProxyEnv         []string // KEY=VALUE proxy hints (best-effort, bypassable)
	ContainerNetwork string   // --network value for container backends
}

// NetworkPolicy decides reachability for both planes.
type NetworkPolicy interface {
	// DialContext is injected into in-process consumers (http/MCP). Truly enforced.
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
	// SubprocessConfig returns subprocess-plane wiring derived from this policy.
	SubprocessConfig() SubprocessNetConfig
}

// FilesystemPolicy decides file access for both planes.
type FilesystemPolicy interface {
	// Check is called by in-process file tools before access. Truly enforced.
	Check(path string, op FSOp) error
	// SubprocessMounts returns subprocess-plane mounts for container backends.
	SubprocessMounts() []Mount
}
