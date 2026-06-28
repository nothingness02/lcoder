# Extension Mode Package Example

This example shows how to package and distribute a custom agent mode.

## Structure

```
extension-mode/
  lcoder-package.yaml
  agents/
    review.yaml
```

## Install

```bash
lcoder install --local ./examples/extension-mode
```

Then run:

```bash
lcoder --mode review "review pkg/agent/loop.go"
```
