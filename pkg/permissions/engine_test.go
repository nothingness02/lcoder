package permissions

import (
	"testing"
)

func TestDefaultAllow(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	if engine.Evaluate(Request{Tool: "read"}) != Allow {
		t.Fatal("expected allow by default")
	}
}

func TestSpecificity(t *testing.T) {
	cfg := Config{
		Rules: map[string]RuleTable{
			"bash": {
				"*":     Ask,
				"git *": Allow,
			},
		},
	}
	engine := NewEngine(cfg)

	if engine.Evaluate(Request{Tool: "bash", Command: "rm -rf /"}) != Ask {
		t.Fatal("expected ask for generic bash")
	}
	if engine.Evaluate(Request{Tool: "bash", Command: "git status"}) != Allow {
		t.Fatal("expected allow for git command")
	}
}

func TestDeny(t *testing.T) {
	cfg := Config{
		Rules: map[string]RuleTable{
			"read": {
				"*.env": Deny,
				"*":     Allow,
			},
		},
	}
	engine := NewEngine(cfg)

	if engine.Evaluate(Request{Tool: "read", Path: ".env"}) != Deny {
		t.Fatal("expected deny for .env")
	}
	if engine.Evaluate(Request{Tool: "read", Path: "main.go"}) != Allow {
		t.Fatal("expected allow for main.go")
	}
}
