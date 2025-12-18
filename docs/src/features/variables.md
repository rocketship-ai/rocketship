# Variables

Variables let you make your tests flexible and reusable. Instead of hardcoding values like URLs or passwords, you can use variables that change based on where or how you run the test.

**Why use variables?**
- Run the same test in different environments (development, staging, production)
- Keep secrets (like passwords) separate from your test code
- Reuse values across multiple steps in a test

## Variable Types

Rocketship supports four types of variables. Each serves a different purpose:

| Type            | Syntax             | When to Use                                    | Example                |
| --------------- | ------------------ | ---------------------------------------------- | ---------------------- |
| **Built-in**    | `{{ .run.id }}`    | Unique IDs that Rocketship generates           | `{{ .run.id }}`        |
| **Environment** | `{{ .env.VAR }}`   | Secrets, passwords, and sensitive data         | `{{ .env.API_KEY }}`   |
| **Config**      | `{{ .vars.name }}` | Settings that change between runs              | `{{ .vars.base_url }}` |
| **Runtime**     | `{{ variable }}`   | Values saved from one step to use in the next  | `{{ user_id }}`        |

## Built-in Variables

Rocketship automatically provides these variables for every test run:

| Variable        | Description                                  | Example    |
| --------------- | -------------------------------------------- | ---------- |
| `{{ .run.id }}` | A unique ID for each test run (useful for creating unique test data) | `a3f2e91c` |

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

Use environment variables for **secrets and sensitive information** like API keys, passwords, or tokens. These should never be stored in your test files.

**Why?** Security best practice - keep secrets out of code that might be shared or committed to version control.

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

**Which value is used?** If the same variable is set in multiple places, Rocketship uses this priority (first match wins):
1. System environment variables (highest priority)
2. Values from `--env-file`
3. Default values in your YAML file (lowest priority)

## Config Variables

Use config variables for **test settings that change between runs** but aren't secrets. Examples: API versions, timeout values, feature flags, or which environment to test.

**Perfect for:** Making one test file work in multiple scenarios without changing the code.

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

**Which value is used?** Rocketship checks in this order:
1. Values passed with `--var` flags on the command line (highest priority)
2. Values from `--var-file`
3. Values defined in the YAML `vars` section (lowest priority)

## Runtime Variables

Runtime variables let you **pass data from one step to the next**. For example, when you create a user and get back an ID, you can save that ID and use it in later steps to update or delete that user.

**How it works:** In one step, you save a value (like a user ID). In later steps, you reference that saved value.

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

## Using Literal Curly Braces

Sometimes you need to include `{{ }}` in your text without Rocketship treating it as a variable. Escape them with backslashes:

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
