package tools

import (
	"sort"
	"strings"

	"github.com/lcoder/lcoder/pkg/models"
)

// ToolSearchName is the meta-tool the model calls to load a deferred tool's full
// schema before invoking it.
const ToolSearchName = "tool_search"

// ToolSearchDefinition returns the tool_search meta-tool. Under deferred tool
// loading, only a small core set ships with full JSON schemas; every other tool
// appears as a name-only stub. The model calls tool_search with a keyword to
// pull the full schema of a deferred tool on demand, keeping the per-turn tool
// payload (and its token cost) small.
func ToolSearchDefinition() models.ToolDefinition {
	return models.ToolDefinition{
		Name: ToolSearchName,
		Description: "Search for and load the full schema of a deferred tool by keyword. " +
			"Tools listed as \"(deferred)\" expose only their name until you load them here; " +
			"call tool_search with a keyword (tool name or purpose) to retrieve the full schema, then call the tool.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Keyword to match against tool names and descriptions.",
				},
			},
			"required": []any{"query"},
		},
	}
}

// stubDefinition reduces a full tool definition to a deferred stub: its name plus
// a one-line description and no parameter schema, so it costs almost nothing
// until tool_search loads the real schema.
func stubDefinition(def models.ToolDefinition) models.ToolDefinition {
	desc := def.Description
	if i := strings.IndexByte(desc, '.'); i > 0 {
		desc = desc[:i+1] // keep the first sentence only
	}
	return models.ToolDefinition{
		Name:        def.Name,
		Description: "(deferred) " + desc,
	}
}

// DeferredDefinitions splits the registry's tools into an active set and a
// deferred set. The active set carries full JSON schemas for the named core
// tools plus the tool_search meta-tool; the deferred set carries name-only stubs
// for everything else. Both sets are sorted by name for deterministic output.
func (r *Registry) DeferredDefinitions(coreNames ...string) (active, deferred []models.ToolDefinition) {
	core := make(map[string]bool, len(coreNames))
	for _, n := range coreNames {
		core[n] = true
	}
	for _, def := range r.Definitions() {
		if core[def.Name] {
			active = append(active, def)
		} else {
			deferred = append(deferred, stubDefinition(def))
		}
	}
	sort.Slice(active, func(i, j int) bool { return active[i].Name < active[j].Name })
	sort.Slice(deferred, func(i, j int) bool { return deferred[i].Name < deferred[j].Name })
	active = append(active, ToolSearchDefinition())
	return active, deferred
}

// SearchTools returns the FULL schema of every registered tool whose name or
// description contains the query (case-insensitive) — the resolution that a
// tool_search call performs at runtime. Results are sorted by name.
func (r *Registry) SearchTools(query string) []models.ToolDefinition {
	q := strings.ToLower(strings.TrimSpace(query))
	var out []models.ToolDefinition
	for _, def := range r.Definitions() {
		if q == "" ||
			strings.Contains(strings.ToLower(def.Name), q) ||
			strings.Contains(strings.ToLower(def.Description), q) {
			out = append(out, def)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
