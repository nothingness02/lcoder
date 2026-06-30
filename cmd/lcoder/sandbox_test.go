package main

import (
	"testing"

	"github.com/lcoder/lcoder/pkg/config"
)

func TestToSandboxConfigMapsAllFields(t *testing.T) {
	c := config.SandboxConfig{
		Backend:      "soft-limit",
		EnvAllowlist: []string{"PATH", "HOME"},
		Network: config.SandboxNetworkConfig{
			Default: "deny",
			Allow:   []string{"api.github.com:443"},
		},
		Filesystem: config.SandboxFilesystemConfig{
			Readable: []string{"."},
			Writable: []string{"src"},
		},
		Limits: config.SandboxLimitsConfig{
			MaxMemoryMB:    512,
			MaxCPUSeconds:  60,
			MaxOutputBytes: 1048576,
		},
	}

	got := toSandboxConfig(c, "/project/root")

	if got.Backend != "soft-limit" {
		t.Errorf("Backend = %q, want soft-limit", got.Backend)
	}
	if got.ProjectRoot != "/project/root" {
		t.Errorf("ProjectRoot = %q, want /project/root", got.ProjectRoot)
	}
	if len(got.EnvAllowlist) != 2 || got.EnvAllowlist[0] != "PATH" {
		t.Errorf("EnvAllowlist = %v, want [PATH HOME]", got.EnvAllowlist)
	}
	if got.Network.DefaultAllow {
		t.Error("Network.DefaultAllow = true, want false for default=deny")
	}
	if len(got.Network.Allow) != 1 || got.Network.Allow[0] != "api.github.com:443" {
		t.Errorf("Network.Allow = %v", got.Network.Allow)
	}
	if len(got.Filesystem.Readable) != 1 || got.Filesystem.Readable[0] != "." {
		t.Errorf("Filesystem.Readable = %v", got.Filesystem.Readable)
	}
	if len(got.Filesystem.Writable) != 1 || got.Filesystem.Writable[0] != "src" {
		t.Errorf("Filesystem.Writable = %v", got.Filesystem.Writable)
	}
	if got.Limits.MaxMemoryMB != 512 || got.Limits.MaxCPUSeconds != 60 || got.Limits.MaxOutputBytes != 1048576 {
		t.Errorf("Limits = %+v", got.Limits)
	}
}

func TestToSandboxConfigDefaultAllowMapping(t *testing.T) {
	got := toSandboxConfig(config.SandboxConfig{
		Network: config.SandboxNetworkConfig{Default: "allow"},
	}, "/r")
	if !got.Network.DefaultAllow {
		t.Error("default=allow must map to DefaultAllow=true")
	}
}

func TestToSandboxConfigEmptyDefaultIsDeny(t *testing.T) {
	got := toSandboxConfig(config.SandboxConfig{}, "/r")
	if got.Network.DefaultAllow {
		t.Error("empty default must map to DefaultAllow=false (default-deny)")
	}
	if got.Backend != "" {
		t.Errorf("empty backend must stay empty (passthrough), got %q", got.Backend)
	}
	if got.ProjectRoot != "/r" {
		t.Errorf("ProjectRoot = %q, want /r", got.ProjectRoot)
	}
}
