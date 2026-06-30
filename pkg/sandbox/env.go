package sandbox

import (
	"runtime"
	"strings"
)

// scrubEnv filters env (KEY=VALUE entries) to the allowlist using OS-appropriate
// name matching. Credential-bearing vars are excluded by virtue of the allowlist
// (default-deny). On Windows env names are case-insensitive, so matching folds case.
func scrubEnv(env, allowlist []string) []string {
	return scrubEnvFold(env, allowlist, runtime.GOOS == "windows")
}

// scrubEnvFold is the testable core; fold controls case-insensitive name matching.
func scrubEnvFold(env, allowlist []string, fold bool) []string {
	allow := make(map[string]bool, len(allowlist))
	for _, name := range allowlist {
		allow[foldName(name, fold)] = true
	}
	var out []string
	for _, kv := range env {
		eq := strings.IndexByte(kv, '=')
		if eq < 0 {
			continue
		}
		if allow[foldName(kv[:eq], fold)] {
			out = append(out, kv)
		}
	}
	return out
}

func foldName(name string, fold bool) string {
	if fold {
		return strings.ToUpper(name)
	}
	return name
}
