# Extension Hook Example

This example shows how to write a custom Lcoder hook extension.

## Usage

Import the hook when building your own Lcoder entrypoint:

```go
import (
    "github.com/lcoder/lcoder/pkg/agent"
    "github.com/lcoder/lcoder/examples/extension-hook"
)

ag, err := agent.NewBuilder().
    WithBeforeToolCall(extensionhook.ReadmeProtector()).
    Build()
```

## What it does

Blocks all `write` and `edit` tool calls targeting `README.md`.
