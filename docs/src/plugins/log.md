# Log Plugin

Output custom messages during test execution for debugging and progress tracking.

## Quick Start

```yaml
- name: "Log message"
  plugin: log
  config:
    message: "Starting test execution"
```

## Configuration

| Field | Description | Example |
|-------|-------------|---------|
| `message` | Message to log (supports variables) | `"Processing user {{ user_id }}"` |

## Using Variables

```yaml
# Runtime variables
- name: "Create user"
  plugin: http
  config:
    method: POST
    url: "{{ .vars.api_url }}/users"
  save:
    - json_path: ".id"
      as: "user_id"

- name: "Log user ID"
  plugin: log
  config:
    message: "Created user with ID: {{ user_id }}"

# Config variables
- plugin: log
  config:
    message: "Running tests in {{ .vars.environment }} environment"

# Environment variables
- plugin: log
  config:
    message: "Test running on {{ .env.HOSTNAME }}"
```

## Common Use Cases

```yaml
# Progress tracking
- plugin: log
  config:
    message: "🚀 Starting authentication flow"

- name: "Login"
  plugin: http
  config:
    method: POST
    url: "{{ .env.API_URL }}/auth/login"

- plugin: log
  config:
    message: "✅ Authentication completed"

# Debug information
- name: "Query database"
  plugin: sql
  config:
    driver: postgres
    dsn: "{{ .env.DATABASE_URL }}"
    commands:
      - "SELECT COUNT(*) as count FROM users;"
  save:
    - sql_result: ".queries[0].rows[0].count"
      as: "user_count"

- plugin: log
  config:
    message: "Found {{ user_count }} users in database"
```

## Best Practices

- **Use emojis**: Make logs more readable (`🚀`, `✅`, `⚠️`, `❌`)
- **Include context**: Add relevant variable values
- **Clear messages**: Write descriptive, actionable messages

## See Also

- [Variables](../features/variables.md) - Using variables in log messages
