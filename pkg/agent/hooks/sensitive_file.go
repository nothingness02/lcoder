package hooks

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lcoder/lcoder/pkg/agent"
)

// SensitiveFileCheck blocks or warns on read/write access to sensitive paths.
func SensitiveFileCheck(patterns []string) agent.BeforeToolCallHook {
	return func(ctx context.Context, info agent.ToolCallInfo) (*agent.BeforeToolCallResult, error) {
		if info.ToolCall.Name != "read" && info.ToolCall.Name != "write" && info.ToolCall.Name != "edit" {
			return nil, nil
		}
		pathArg, _ := info.Args["path"].(string)
		if pathArg == "" {
			return nil, nil
		}
		for _, pattern := range patterns {
			matched, err := matchPattern(pattern, pathArg)
			if err != nil {
				return nil, err
			}
			if matched {
				return &agent.BeforeToolCallResult{
					Block:  true,
					Reason: fmt.Sprintf("access to sensitive path blocked: %s matches %q", pathArg, pattern),
				}, nil
			}
		}
		return nil, nil
	}
}

func matchPattern(pattern, path string) (bool, error) {
	if strings.Contains(pattern, "*") {
		matched, err := filepath.Match(pattern, filepath.Base(path))
		if err != nil {
			return false, err
		}
		return matched, nil
	}
	return strings.Contains(path, pattern), nil
}
