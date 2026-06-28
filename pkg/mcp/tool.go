package mcp

import (
	"context"
	"fmt"

	"github.com/lcoder/lcoder/pkg/models"
)

// Executable wraps an MCP tool as an Lcoder tool.Executable.
type Executable struct {
	client *Client
	tool   Tool
}

// NewExecutable creates an executable wrapper for an MCP tool.
func NewExecutable(client *Client, tool Tool) *Executable {
	return &Executable{client: client, tool: tool}
}

// Definition returns the tool schema exposed to the LLM.
func (e *Executable) Definition() models.ToolDefinition {
	return models.ToolDefinition{
		Name:          fmt.Sprintf("%s_%s", e.client.Name(), e.tool.Name),
		Description:   fmt.Sprintf("[%s] %s", e.client.Name(), e.tool.Description),
		Parameters:    e.tool.InputSchema,
		ExecutionMode: models.ExecutionParallel,
	}
}

// Execute invokes the MCP tool.
func (e *Executable) Execute(ctx context.Context, callID string, args map[string]any) (models.ToolResult, error) {
	result, err := e.client.CallTool(ctx, e.tool.Name, args)
	if err != nil {
		return models.NewToolResultError(err.Error()), nil
	}

	content := make([]models.ContentPart, 0, len(result.Content))
	for _, item := range result.Content {
		switch item.Type {
		case "image":
			content = append(content, models.ImageContent{Data: item.Data, MimeType: item.MimeType})
		default:
			content = append(content, models.TextContent{Text: item.Text})
		}
	}

	if result.IsError {
		return models.NewToolResultError(result.ContentText()), nil
	}
	return models.ToolResult{Content: content}, nil
}

// ContentText is a helper to extract text from CallToolResult content.
func (r *CallToolResult) ContentText() string {
	var out string
	for _, item := range r.Content {
		out += item.Text
	}
	return out
}
