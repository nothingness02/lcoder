package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSandboxConfigParses(t *testing.T) {
	data := []byte(`
sandbox:
  backend: soft-limit
  env_allowlist: [PATH, HOME]
  network:
    default: deny
    allow: ["api.github.com:443"]
  filesystem:
    writable: ["."]
    readable: ["."]
  limits:
    max_memory_mb: 256
    max_cpu_seconds: 30
    max_output_bytes: 1048576
`)
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.Sandbox.Backend != "soft-limit" {
		t.Fatalf("backend = %q", cfg.Sandbox.Backend)
	}
	if cfg.Sandbox.Network.Default != "deny" {
		t.Fatalf("network.default = %q", cfg.Sandbox.Network.Default)
	}
	if len(cfg.Sandbox.Network.Allow) != 1 || cfg.Sandbox.Network.Allow[0] != "api.github.com:443" {
		t.Fatalf("network.allow = %v", cfg.Sandbox.Network.Allow)
	}
	if len(cfg.Sandbox.EnvAllowlist) != 2 {
		t.Fatalf("env_allowlist = %v", cfg.Sandbox.EnvAllowlist)
	}
	if cfg.Sandbox.Limits.MaxMemoryMB != 256 {
		t.Fatalf("limits.max_memory_mb = %d", cfg.Sandbox.Limits.MaxMemoryMB)
	}
}

func TestSandboxConfigDefaultsEmpty(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Sandbox.Backend != "" {
		t.Fatalf("expected empty backend (passthrough), got %q", cfg.Sandbox.Backend)
	}
}
