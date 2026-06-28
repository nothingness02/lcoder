package tools

import (
	"context"
	"fmt"
	"sync"

	"github.com/lcoder/lcoder/pkg/models"
)

// Registry holds all available tools.
type Registry struct {
	mu     sync.RWMutex
	tools  map[string]Executable
	cwd    string
}

// NewRegistry creates an empty registry bound to a working directory.
func NewRegistry(cwd string) *Registry {
	return &Registry{
		tools: make(map[string]Executable),
		cwd:   cwd,
	}
}

// Register adds a tool to the registry.
func (r *Registry) Register(name string, exec Executable) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[name] = exec
}

// RegisterBuiltin adds a built-in tool factory.
func (r *Registry) RegisterBuiltin(factory Factory) {
	exec := factory(r.cwd)
	r.Register(exec.Definition().Name, exec)
}

// Get returns a tool by name.
func (r *Registry) Get(name string) (Executable, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	exec, ok := r.tools[name]
	return exec, ok
}

// Definitions returns tool definitions for the LLM.
func (r *Registry) Definitions() []models.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]models.ToolDefinition, 0, len(r.tools))
	for _, exec := range r.tools {
		defs = append(defs, exec.Definition())
	}
	return defs
}

// Has reports whether a tool is registered.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.tools[name]
	return ok
}

// Execute runs a tool by name. It returns the tool result and a flag indicating
// whether the result represents an error.
func (r *Registry) Execute(ctx context.Context, callID string, name string, args map[string]any) (models.ToolResult, bool) {
	exec, ok := r.Get(name)
	if !ok {
		return models.NewToolResultError(fmt.Sprintf("Unknown tool: %s", name)), true
	}
	result, err := exec.Execute(ctx, callID, args)
	if err != nil {
		return models.NewToolResultError(err.Error()), true
	}
	return result, false
}
