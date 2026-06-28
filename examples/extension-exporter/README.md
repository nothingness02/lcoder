# Extension Exporter Example

This example shows how to write a custom observability exporter for Lcoder.

## Usage

Import the package in your Lcoder entrypoint:

```go
import _ "github.com/lcoder/lcoder/examples/extension-exporter"
```

Then configure it:

```yaml
observability:
  exporter: stdout
```

## What it does

Writes every observability record as a JSONL line to stdout.
