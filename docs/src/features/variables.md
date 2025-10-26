# Variables

Parameterize tests with environment variables, config variables, runtime variables, and built-in variables.

## Variable Types

| Type            | Syntax             | Use Case               | Example                |
| --------------- | ------------------ | ---------------------- | ---------------------- |
| **Built-in**    | `{{ .run.id }}`    | System metadata        | `{{ .run.id }}`        |
| **Environment** | `{{ .env.VAR }}`   | Secrets, API keys      | `{{ .env.API_KEY }}`   |
| **Config**      | `{{ .vars.name }}` | Test parameters        | `{{ .vars.base_url }}` |
| **Runtime**     | `{{ variable }}`   | Saved during execution | `{{ user_id }}`        |

## Built-in Variables

| Variable        | Description        | Example    |
| --------------- | ------------------ | ---------- |
| `{{ .run.id }}` | Unique test run ID | `a3f2e91c` |

```yaml
# Unique test data per run
- name: "Create user"
  plugin: http
  config:
    method: POST
    url: "{{ .env.API_URL }}/users"
    body: |
      {
        "email": "test-{{ .run.id }}@example.com"
      }
```

## Environment Variables

Use for secrets and environment-specific config.

```yaml
steps:
  - plugin: http
    config:
      url: "{{ .env.API_BASE_URL }}/users"
      headers:
        Authorization: "Bearer {{ .env.API_TOKEN }}"
```

```bash
# Load from .env file
rocketship run -af test.yaml --env-file .env

# Or from system environment
export API_TOKEN=sk-your-token
rocketship run -af test.yaml
```

**Precedence**: System environment > `--env-file` > YAML defaults

## Config Variables

Use for test parameters and non-sensitive configuration.

```yaml
vars:
  api_version: "v2"
  timeout: 30

tests:
  - steps:
      - plugin: http
        config:
          url: "{{ .vars.base_url }}/{{ .vars.api_version }}/users"
```

```bash
# Override via CLI
rocketship run -af test.yaml --var api_version=v3
```

**Precedence**: `--var` CLI flags > `--var-file` > YAML `vars`

## Runtime Variables

Save values during test execution for use in later steps.

```yaml
- name: "Create user"
  plugin: http
  config:
    method: POST
    url: "{{ .vars.base_url }}/users"
  save:
    - json_path: ".id"
      as: "user_id"

- name: "Get user"
  plugin: http
  config:
    url: "{{ .vars.base_url }}/users/{{ user_id }}"
```

## Handlebars Escaping

Escape literal `{{ }}` with backslashes:

```yaml
# Processed variable
"api_url": "{{ .env.API_BASE_URL }}/users"

# Literal handlebars
"template": "Use \\{{ user_id }} in the API"
# Result: "Use {{ user_id }} in the API"
```

## Best Practices

- **Environment**: Use `.gitignore` for `.env` files, never commit secrets
- **Config**: Use descriptive names, put defaults in YAML, override via `--var`
- **Runtime**: Use descriptive names, clean up test data in lifecycle hooks
- **Security**: Store secrets in CI/CD platform, not in env files

## See Also

- [HTTP Plugin](../plugins/http.md) - Using variables in HTTP requests
- [Lifecycle Hooks](lifecycle-hooks.md) - Setting up and tearing down test data
