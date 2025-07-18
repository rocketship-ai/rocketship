# Async Operations with HTTP Polling

This example demonstrates how to use the HTTP plugin's async polling feature to handle long-running operations.

## Overview

The HTTP plugin now supports polling functionality that allows you to:
1. Make an initial HTTP request
2. Continuously poll the same or different endpoint until specific conditions are met
3. Handle timeouts and exponential backoff
4. Extract data from the final successful response

## Configuration

### Basic Polling Configuration

```yaml
config:
  method: "GET"
  url: "https://api.example.com/jobs/123"
  polling:
    interval: "2s"          # How often to poll
    timeout: "5m"           # Maximum time to wait
    max_attempts: 30        # Maximum polling attempts (optional)
    backoff_coefficient: 1.2 # Exponential backoff multiplier (optional)
    conditions:             # Conditions that must be met to stop polling
      - type: "status_code"
        expected: 200
      - type: "json_path"
        path: ".status"
        expected: "completed"
```

### Polling Conditions

The plugin supports three types of conditions:

#### 1. Status Code Conditions
```yaml
conditions:
  - type: "status_code"
    expected: 200
```

#### 2. JSON Path Conditions
```yaml
conditions:
  - type: "json_path"
    path: ".status"
    expected: "completed"
```

Check if a JSON path exists:
```yaml
conditions:
  - type: "json_path"
    path: ".job_id"
    exists: true
```

#### 3. Header Conditions
```yaml
conditions:
  - type: "header"
    name: "X-Job-Status"
    expected: "finished"
```

### Variable Replacement

You can use variables in polling conditions:

```yaml
conditions:
  - type: "json_path"
    path: ".status"
    expected: "{{ expected_status }}"
```

## Real-World Use Cases

### 1. Job Processing
```yaml
steps:
  - name: "Submit job"
    plugin: "http"
    config:
      method: "POST"
      url: "https://api.example.com/jobs"
      body: '{"data": "process this"}'
    save:
      - json_path: ".job_id"
        as: "job_id"

  - name: "Wait for job completion"
    plugin: "http"
    config:
      method: "GET"
      url: "https://api.example.com/jobs/{{ job_id }}"
      polling:
        interval: "5s"
        timeout: "10m"
        conditions:
          - type: "json_path"
            path: ".status"
            expected: "completed"
```

### 2. Deployment Status
```yaml
steps:
  - name: "Check deployment status"
    plugin: "http"
    config:
      method: "GET"
      url: "https://api.example.com/deployments/123"
      polling:
        interval: "10s"
        timeout: "30m"
        backoff_coefficient: 1.1
        conditions:
          - type: "status_code"
            expected: 200
          - type: "json_path"
            path: ".deployment.status"
            expected: "success"
```

### 3. File Processing
```yaml
steps:
  - name: "Wait for file processing"
    plugin: "http"
    config:
      method: "GET"
      url: "https://api.example.com/files/{{ file_id }}/status"
      polling:
        interval: "3s"
        timeout: "15m"
        max_attempts: 100
        conditions:
          - type: "json_path"
            path: ".processing_complete"
            expected: true
          - type: "json_path"
            path: ".errors"
            exists: false
```

## Configuration Options

| Option | Required | Type | Description |
|--------|----------|------|-------------|
| `interval` | Yes | string | Time between polling attempts (e.g., "1s", "30s", "2m") |
| `timeout` | Yes | string | Maximum time to wait before giving up (e.g., "5m", "1h") |
| `max_attempts` | No | int | Maximum number of polling attempts |
| `backoff_coefficient` | No | float | Multiplier for exponential backoff (default: 1.0) |
| `conditions` | Yes | array | List of conditions that must be met to stop polling |

## Error Handling

The polling mechanism handles various error scenarios:

- **Network errors**: Retries with backoff
- **HTTP errors**: Retries with backoff
- **Timeout**: Fails after the specified timeout
- **Max attempts**: Fails after exceeding maximum attempts
- **Condition failures**: Continues polling until conditions are met

## Best Practices

1. **Set reasonable intervals**: Don't poll too frequently to avoid overwhelming the server
2. **Use timeouts**: Always set a reasonable timeout to prevent infinite polling
3. **Use exponential backoff**: Helps reduce server load with `backoff_coefficient > 1.0`
4. **Combine conditions**: Use multiple conditions for more robust polling
5. **Handle failures**: Consider what happens if polling times out or fails

## Testing

You can test the async polling functionality using the provided example:

```bash
# Using Docker environment
./.docker/rocketship start
./.docker/rocketship run -f examples/async-operations/rocketship.yaml

# Using local environment
rocketship run -af examples/async-operations/rocketship.yaml
```

The example uses `httpbin.org` endpoints that simulate async operations with delays.