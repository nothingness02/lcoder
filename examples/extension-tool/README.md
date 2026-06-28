# Extension Tool Example

This example shows how to write a custom Go tool extension for Lcoder.

## Usage

Build the extension and place it in your `~/.lcoder/extensions/` directory, or reference it in `lcoder.yaml`:

```yaml
extensions:
  - name: weather
    source: ./examples/extension-tool
```

Then start Lcoder and ask:

```
What is the weather in Beijing?
```

## How it works

The extension registers a `weather` factory into `tools.DefaultFactories` during `init()`. When Lcoder loads the extension, the tool becomes available to the agent.
