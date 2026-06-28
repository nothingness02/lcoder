# Extension HTTP Tool Example

This example shows how to define a custom HTTP tool via `lcoder.yaml` without writing Go code.

## Usage

Reference the tool definition in your `lcoder.yaml`:

```yaml
http_tools:
  - name: random-user
    endpoint: https://randomuser.me/api
    description: Fetch a random user profile
    parameters:
      type: object
      properties:
        gender:
          type: string
          enum: [male, female]
      required: []
```

Then ask Lcoder:

```
Get me a random user
```
