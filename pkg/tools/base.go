package tools

import (
	"context"

	"github.com/lcoder/lcoder/pkg/models"
	"github.com/lcoder/lcoder/pkg/sandbox"
)

// Executable is the interface implemented by every tool available to the agent.
type Executable interface {
	Definition() models.ToolDefinition
	Execute(ctx context.Context, callID string, args map[string]any) (models.ToolExecutionResult, error)
}

// Factory creates a tool instance bound to a working directory.
type Factory func(cwd string) Executable

// SandboxAware is optionally implemented by tools that need a sandbox. The
// Registry detects it at registration time and injects the active sandbox.
// Tools that do not implement it (e.g. third-party extensions) are unaffected.
type SandboxAware interface {
	UseSandbox(sb sandbox.Sandbox)
}
