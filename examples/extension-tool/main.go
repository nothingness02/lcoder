// Package main implements a custom Lcoder tool extension.
// It registers a "weather" tool that returns fake weather data.
package main

import (
	"context"
	"fmt"

	"github.com/lcoder/lcoder/pkg/models"
	"github.com/lcoder/lcoder/pkg/tools"
)

func init() {
	tools.DefaultFactories.Register("weather", newWeatherTool)
}

func main() {
	// Placeholder so `go build` succeeds for this importable extension.
}

type weatherTool struct{}

func newWeatherTool(cwd string) tools.Executable {
	return &weatherTool{}
}

func (w *weatherTool) Definition() models.ToolDefinition {
	return models.ToolDefinition{
		Name:        "weather",
		Description: "Get the current weather for a city",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"city": map[string]any{
					"type":        "string",
					"description": "City name",
				},
			},
			"required": []string{"city"},
		},
	}
}

func (w *weatherTool) Execute(ctx context.Context, callID string, args map[string]any) (models.ToolResult, error) {
	city, _ := args["city"].(string)
	if city == "" {
		return models.NewToolResultError("city is required"), nil
	}
	return models.ToolResult{
		Content: []models.ContentPart{
			models.TextContent{Text: fmt.Sprintf("The weather in %s is sunny, 24°C.", city)},
		},
	}, nil
}
