// Package main implements a custom Lcoder hook extension.
// It blocks all write/edit operations to files named README.md.
package main

import (
	"context"
	"fmt"

	"github.com/lcoder/lcoder/pkg/agent"
)

// ReadmeProtector returns a BeforeToolCallHook that blocks modifications to README.md.
func ReadmeProtector() agent.BeforeToolCallHook {
	return func(ctx context.Context, info agent.ToolCallInfo) (*agent.BeforeToolCallResult, error) {
		if info.ToolCall.Name != "write" && info.ToolCall.Name != "edit" {
			return nil, nil
		}
		path, _ := info.Args["path"].(string)
		if path == "README.md" {
			return &agent.BeforeToolCallResult{
				Block:  true,
				Reason: "README.md is protected by the readme-protector hook",
			}, nil
		}
		return nil, nil
	}
}

func main() {
	// This main is a placeholder so `go build` works.
	// Real usage: import this package and use ReadmeProtector() when building the agent.
	fmt.Println("readme-protector hook loaded")
}
