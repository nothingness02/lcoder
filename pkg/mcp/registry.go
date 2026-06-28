package mcp

import (
	"fmt"
	"sync"

	"github.com/lcoder/lcoder/pkg/tools"
)

// ServerConfig describes an MCP server to connect to.
type ServerConfig struct {
	Name    string            `json:"name"`
	Command []string          `json:"command"`
	Env     map[string]string `json:"env,omitempty"`
}

// Registry manages multiple MCP clients.
type Registry struct {
	mu      sync.RWMutex
	clients map[string]*Client
	configs []ServerConfig
	errors  map[string]error
}

// NewRegistry creates an MCP registry.
func NewRegistry(configs []ServerConfig) *Registry {
	return &Registry{
		clients: make(map[string]*Client),
		configs: configs,
		errors:  make(map[string]error),
	}
}

// Connect starts all configured MCP servers.
func (r *Registry) Connect() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, cfg := range r.configs {
		client, err := NewClient(cfg.Name, cfg.Command, cfg.Env)
		if err != nil {
			r.errors[cfg.Name] = err
			continue
		}
		r.clients[cfg.Name] = client
	}
	return nil
}

// Close shuts down all MCP clients.
func (r *Registry) Close() error {
	r.mu.Lock()
	clients := make([]*Client, 0, len(r.clients))
	for _, c := range r.clients {
		clients = append(clients, c)
	}
	r.mu.Unlock()

	var firstErr error
	for _, c := range clients {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// RegisterTools registers all MCP tools into the Lcoder tools registry.
func (r *Registry) RegisterTools(registry *tools.Registry) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, client := range r.clients {
		for _, tool := range client.Tools() {
			exec := NewExecutable(client, tool)
			registry.Register(exec.Definition().Name, exec)
		}
	}
}

// Servers returns status info for each configured server.
func (r *Registry) Servers() []ServerStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var statuses []ServerStatus
	for _, cfg := range r.configs {
		client, ok := r.clients[cfg.Name]
		status := ServerStatus{
			Name:    cfg.Name,
			Command: cfg.Command,
		}
		if ok && client.Healthy() {
			status.Connected = true
			status.ToolCount = len(client.Tools())
			status.Info = client.ServerInfo()
		} else if err, ok := r.errors[cfg.Name]; ok {
			status.Error = err.Error()
		} else {
			status.Error = "not connected"
		}
		statuses = append(statuses, status)
	}
	return statuses
}

// ServerStatus describes the status of one MCP server.
type ServerStatus struct {
	Name      string
	Command   []string
	Connected bool
	ToolCount int
	Info      Info
	Error     string
}

// PrefixedName returns a tool name with the server prefix.
func PrefixedName(serverName, toolName string) string {
	return fmt.Sprintf("%s_%s", serverName, toolName)
}
