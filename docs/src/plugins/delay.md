# Delay Plugin

Add deterministic pauses between test steps for eventual consistency and background processing.

## Quick Start

```yaml
- name: "Wait for processing"
  plugin: delay
  config:
    duration: "2s"
```

## Configuration

| Field | Description | Example |
|-------|-------------|---------|
| `duration` | Wait duration | `500ms`, `2s`, `1m`, `5m` |

## Common Use Cases

```yaml
# After creating resources
- name: "Create user"
  plugin: http
  config:
    method: POST
    url: "{{ .vars.api_url }}/users"

- name: "Wait for indexing"
  plugin: delay
  config:
    duration: "2s"

# Between API calls
- name: "Create item 1"
  plugin: http
  config:
    method: POST
    url: "{{ .vars.api_url }}/items"

- name: "Wait for system processing"
  plugin: delay
  config:
    duration: "1s"

- name: "Create item 2"
  plugin: http
  config:
    method: POST
    url: "{{ .vars.api_url }}/items"
```

## Best Practices

- **Use shortest effective duration**: Start with shorter delays and increase if needed
- **Descriptive names**: Explain why the delay is needed (`"Wait for search index"`)
- **Consider alternatives**: Use [retry policies](../features/retry-policies.md) for non-deterministic operations

## See Also

- [Retry Policies](../features/retry-policies.md) - Better alternative for flaky operations
