package sandbox

import (
	"context"
	"net"
)

// FakeSandbox records Exec calls and returns programmed results. Intended for
// consumer (tool) tests so they never touch real processes or the network.
type FakeSandbox struct {
	Calls     []ExecSpec
	Result    ExecResult
	Err       error
	NetPolicy NetworkPolicy
	FSPolicy  FilesystemPolicy
}

// NewFakeSandbox returns a FakeSandbox with allow-all policies by default.
func NewFakeSandbox() *FakeSandbox {
	return &FakeSandbox{
		NetPolicy: &passthroughNetwork{dialer: &net.Dialer{}},
		FSPolicy:  allowAllFS{},
	}
}

func (f *FakeSandbox) Exec(_ context.Context, spec ExecSpec) (ExecResult, error) {
	f.Calls = append(f.Calls, spec)
	return f.Result, f.Err
}

func (f *FakeSandbox) Network() NetworkPolicy       { return f.NetPolicy }
func (f *FakeSandbox) Filesystem() FilesystemPolicy { return f.FSPolicy }
func (f *FakeSandbox) Name() string                 { return "fake" }

var _ Sandbox = (*FakeSandbox)(nil)
