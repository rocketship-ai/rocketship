# Retry Policies

Configure automatic retries for test steps with configurable backoff strategies.

## Quick Start

```yaml
- name: "API request with retry"
  plugin: http
  config:
    method: GET
    url: "{{ .vars.api_url }}/users"
  retry:
    maximum_attempts: 3
    initial_interval: "1s"
    backoff_coefficient: 2.0
```

## Configuration

| Option                  | Description                 | Example                  |
| ----------------------- | --------------------------- | ------------------------ |
| `maximum_attempts`      | Maximum retry attempts      | `3`                      |
| `initial_interval`      | Initial retry delay         | `"1s"`, `"500ms"`        |
| `maximum_interval`      | Maximum retry delay         | `"30s"`                  |
| `backoff_coefficient`   | Exponential backoff multiplier | `2.0`                    |
| `non_retryable_errors`  | Error types to never retry  | `["ValidationError"]`    |

## Backoff Strategies

### Exponential Backoff

```yaml
retry:
  maximum_attempts: 4
  initial_interval: "1s"
  backoff_coefficient: 2.0
# Retry delays: 1s → 2s → 4s → 8s
```

### Linear Backoff

```yaml
retry:
  maximum_attempts: 3
  initial_interval: "5s"
  backoff_coefficient: 1.0
# Retry delays: 5s → 5s → 5s
```

### Capped Exponential

```yaml
retry:
  maximum_attempts: 5
  initial_interval: "1s"
  maximum_interval: "10s"
  backoff_coefficient: 2.0
# Retry delays: 1s → 2s → 4s → 8s → 10s (capped)
```

## Common Patterns

```yaml
# Flaky HTTP endpoints
- name: "Call API"
  plugin: http
  config:
    method: GET
    url: "{{ .vars.api_url }}/status"
  retry:
    maximum_attempts: 5
    initial_interval: "1s"
    backoff_coefficient: 2.0

# Database queries
- name: "Query database"
  plugin: sql
  config:
    driver: postgres
    dsn: "{{ .env.DATABASE_URL }}"
    commands:
      - "SELECT COUNT(*) FROM users;"
  retry:
    maximum_attempts: 3
    initial_interval: "2s"

# Skip specific errors
- name: "Create user"
  plugin: http
  config:
    method: POST
    url: "{{ .vars.api_url }}/users"
  retry:
    maximum_attempts: 3
    non_retryable_errors:
      - "ValidationError"
      - "AuthenticationError"
```

## Best Practices

- **Use retries for**: Network timeouts, transient failures, flaky APIs, eventual consistency
- **Don't retry**: Validation errors, authentication failures, permanent errors (404, 401)
- **Start conservative**: Begin with 3 attempts, adjust based on flakiness
- **Use exponential backoff**: Better for rate-limited APIs
- **Set maximum_interval**: Prevent excessively long delays

## See Also

- [Delay Plugin](../plugins/delay.md) - Fixed delays between steps
- [HTTP Plugin](../plugins/http.md) - HTTP request retries
