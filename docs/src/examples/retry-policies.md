# Retry Policies

Configure automatic retries for any step to improve test reliability when dealing with flaky services or network issues.

## Basic Usage

Add a `retry` configuration to any step:

```yaml
- name: "HTTP request with retry"
  plugin: "http"
  config:
    method: "GET"
    url: "https://api.example.com/users"
  retry:
    maximum_attempts: 3
    initial_interval: "1s"
    backoff_coefficient: 2.0
```

## Configuration Options

| Option | Type | Description | Example |
|--------|------|-------------|---------|
| `maximum_attempts` | integer | Maximum retry attempts | `3` |
| `initial_interval` | string | Initial retry delay | `"1s"`, `"500ms"` |
| `maximum_interval` | string | Maximum retry delay | `"30s"`, `"5m"` |
| `backoff_coefficient` | number | Exponential backoff multiplier | `2.0` |
| `non_retryable_errors` | array | Error types to never retry | `["ValidationError"]` |

## Plugin Support

Retry works with **all plugins**:

```yaml
# HTTP requests
- name: "API call"
  plugin: "http"
  retry:
    maximum_attempts: 5
    initial_interval: "2s"

# Database queries  
- name: "Database check"
  plugin: "sql"
  retry:
    maximum_attempts: 3
    initial_interval: "1s"

# Even delays
- name: "Wait step"
  plugin: "delay"
  retry:
    maximum_attempts: 2
```

## Examples

### Exponential Backoff
```yaml
retry:
  maximum_attempts: 4
  initial_interval: "1s"
  maximum_interval: "16s"
  backoff_coefficient: 2.0
# Retries: 1s → 2s → 4s → 8s
```

### Linear Backoff
```yaml
retry:
  maximum_attempts: 3
  initial_interval: "5s"
  backoff_coefficient: 1.0
# Retries: 5s → 5s → 5s
```

### Skip Certain Errors
```yaml
retry:
  maximum_attempts: 3
  non_retryable_errors: ["AuthenticationError", "ValidationError"]
```

!!! note "Backward Compatibility"
    Steps without retry configuration use the default single-attempt behavior.