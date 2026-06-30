package sandbox

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// passthroughNetwork allows all traffic via a plain dialer.
type passthroughNetwork struct{ dialer *net.Dialer }

func (p *passthroughNetwork) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return p.dialer.DialContext(ctx, network, addr)
}

func (p *passthroughNetwork) SubprocessConfig() SubprocessNetConfig { return SubprocessNetConfig{} }

// allowEntry is one host:port allow rule. host may be a "*.example.com" wildcard;
// port 0 means any port.
type allowEntry struct {
	host string
	port int
}

// allowlistNetwork enforces an allowlist for in-process dials.
type allowlistNetwork struct {
	defaultAllow bool
	entries      []allowEntry
	dialer       *net.Dialer
}

func (p *allowlistNetwork) allowed(host string, port int) bool {
	for _, e := range p.entries {
		if e.port != 0 && e.port != port {
			continue
		}
		if hostMatches(e.host, host) {
			return true
		}
	}
	return p.defaultAllow
}

// hostMatches reports whether host matches pattern. A "*." prefix matches any
// strict subdomain (not the bare apex).
func hostMatches(pattern, host string) bool {
	host = strings.ToLower(host)
	pattern = strings.ToLower(pattern)
	if pattern == host {
		return true
	}
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // ".example.com"
		return strings.HasSuffix(host, suffix) && len(host) > len(suffix)
	}
	return false
}

func (p *allowlistNetwork) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("sandbox: bad address %q: %w", addr, err)
	}
	port, _ := strconv.Atoi(portStr)
	if !p.allowed(host, port) {
		return nil, fmt.Errorf("sandbox: network access to %s denied by policy", addr)
	}
	return p.dialer.DialContext(ctx, network, addr)
}

// SubprocessConfig returns empty wiring; subprocess proxy hints are deferred to a
// later iteration (see spec §6 — best-effort, bypassable).
func (p *allowlistNetwork) SubprocessConfig() SubprocessNetConfig { return SubprocessNetConfig{} }
