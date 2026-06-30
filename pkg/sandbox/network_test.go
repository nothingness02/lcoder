package sandbox

import (
	"context"
	"net"
	"strconv"
	"strings"
	"testing"
)

func TestHostMatches(t *testing.T) {
	cases := []struct {
		pattern, host string
		want          bool
	}{
		{"api.example.com", "api.example.com", true},
		{"api.example.com", "other.com", false},
		{"*.github.com", "api.github.com", true},
		{"*.github.com", "github.com", false}, // wildcard requires a subdomain
		{"*.github.com", "evilgithub.com", false},
	}
	for _, c := range cases {
		if got := hostMatches(c.pattern, c.host); got != c.want {
			t.Fatalf("hostMatches(%q,%q)=%v want %v", c.pattern, c.host, got, c.want)
		}
	}
}

func TestAllowlistDenies(t *testing.T) {
	p := &allowlistNetwork{
		defaultAllow: false,
		entries:      []allowEntry{{host: "allowed.com", port: 443}},
		dialer:       &net.Dialer{},
	}
	_, err := p.DialContext(context.Background(), "tcp", "blocked.com:443")
	if err == nil || !strings.Contains(err.Error(), "denied by policy") {
		t.Fatalf("expected deny error, got %v", err)
	}
}

func TestAllowlistAllowsAndDials(t *testing.T) {
	// Spin up a local listener and allow it; DialContext should connect.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	p := &allowlistNetwork{
		defaultAllow: false,
		entries:      []allowEntry{{host: "127.0.0.1", port: port}},
		dialer:       &net.Dialer{},
	}
	conn, err := p.DialContext(context.Background(), "tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("expected dial success, got %v", err)
	}
	conn.Close()
}

func TestPassthroughNetworkSubprocessConfigEmpty(t *testing.T) {
	p := &passthroughNetwork{dialer: &net.Dialer{}}
	if cfg := p.SubprocessConfig(); len(cfg.ProxyEnv) != 0 || cfg.ContainerNetwork != "" {
		t.Fatalf("expected empty subprocess config, got %+v", cfg)
	}
}

func TestAllowlistAnyPortEntry(t *testing.T) {
	// port 0 in an entry means "any port".
	p := &allowlistNetwork{
		defaultAllow: false,
		entries:      []allowEntry{{host: "anyport.com", port: 0}},
		dialer:       &net.Dialer{},
	}
	if !p.allowed("anyport.com", 8080) {
		t.Fatal("port-0 entry should allow any port")
	}
	if !p.allowed("anyport.com", 443) {
		t.Fatal("port-0 entry should allow any port")
	}
	if p.allowed("other.com", 443) {
		t.Fatal("non-matching host must still be denied")
	}
}

func TestAllowlistDefaultAllow(t *testing.T) {
	// defaultAllow=true permits hosts that match no entry.
	p := &allowlistNetwork{defaultAllow: true, dialer: &net.Dialer{}}
	if !p.allowed("anything.com", 443) {
		t.Fatal("defaultAllow=true should permit unmatched hosts")
	}
}
