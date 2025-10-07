# Managing Delays in HTTP Test Suites

Most real-world APIs need a short breather between operations. Rocketship’s `delay` plugin gives you deterministic pauses so eventual consistency and background work do not break your flows.

## Delay Plugin Basics

```yaml
- name: "Wait for system processing"
  plugin: "delay"
  config:
    duration: "1s"
```

- `duration` accepts Go-style intervals (`500ms`, `2s`, `1m`).
- Combine multiple delays with different values to smooth out spikes or let async jobs finish.

## Where Delays Fit in the Request-Chaining Example

The [HTTP Request Chaining](../http/request-chaining.md) suite pauses twice:

1. **After creating the first car** – a one-second delay so the remote service persists state before the next POST.
2. **Before reading aggregated data** – a shorter pause ensures both creations are visible when the GET call runs.

Feel free to tweak the durations and observe how the tryme service responds. When tests run inside the Docker stack, a bit of extra buffer keeps shared infrastructure happy.

## Best Practices

- Prefer the shortest delay that stabilises the external dependency.
- Use descriptive step names (`"Wait for search index"`) so reviewers know why the pause exists.
- Keep delays close to the operations they protect.
- Combine with assertions that notice stale state; if the API is still catching up, consider retry policies.

## Related Reading

- [HTTP Request Chaining](../http/request-chaining.md) – full end-to-end workflow using saved state and cleanup steps.
- [Contract Validation](../http/openapi-validation.md) – ensure responses conform to the OpenAPI contract even after delayed operations.
