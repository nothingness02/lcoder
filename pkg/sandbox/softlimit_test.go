package sandbox

import (
	"context"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

func pathEnv() string { return os.Getenv("PATH") }

func newSoftLimit() *softLimit {
	return &softLimit{
		network:  &allowlistNetwork{dialer: &net.Dialer{}},
		fs:       &restrictedFS{},
		envAllow: []string{"PATH", "HOME", "SHELL", "LANG", "SystemRoot", "ComSpec"},
	}
}

func TestSoftLimitScrubsEnv(t *testing.T) {
	sb := newSoftLimit()
	// SECRET is not in the allowlist, so the child must not see it.
	res, err := sb.Exec(context.Background(), ExecSpec{
		Command: `echo "[$SECRET]"`,
		Cwd:     ".",
		Env:     []string{"PATH=" + pathEnv(), "SECRET=topsecret"},
	})
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if strings.Contains(res.Stdout, "topsecret") {
		t.Fatalf("secret leaked into child env: %q", res.Stdout)
	}
}

func TestSoftLimitTimeout(t *testing.T) {
	sb := newSoftLimit()
	start := time.Now()
	res, _ := sb.Exec(context.Background(), ExecSpec{
		Command: "sleep 30",
		Cwd:     ".",
		Env:     []string{"PATH=" + pathEnv()},
		Timeout: 200 * time.Millisecond,
	})
	if !res.TimedOut {
		t.Fatal("expected TimedOut=true")
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("timeout did not kill promptly")
	}
}

func TestSoftLimitOutputCap(t *testing.T) {
	sb := newSoftLimit()
	res, err := sb.Exec(context.Background(), ExecSpec{
		Command: `printf 'aaaaaaaaaa'`, // 10 bytes
		Cwd:     ".",
		Env:     []string{"PATH=" + pathEnv()},
		Limits:  ResourceLimits{MaxOutputBytes: 4},
	})
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if !strings.HasPrefix(res.Stdout, "aaaa") || !strings.Contains(res.Stdout, "truncated") {
		t.Fatalf("expected truncated output, got %q", res.Stdout)
	}
}

func TestSoftLimitMetadata(t *testing.T) {
	sb := newSoftLimit()
	if sb.Name() != "soft-limit" {
		t.Fatalf("name = %q", sb.Name())
	}
}
