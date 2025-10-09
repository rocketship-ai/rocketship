# Variables

Rocketship supports three types of variables for parameterizing your tests:

| Type | Syntax | Use Case | Example |
|------|--------|----------|---------|
| **Environment** | `{{ .env.VAR }}` | Secrets, API keys, environment-specific URLs | `{{ .env.API_KEY }}` |
| **Config** | `{{ .vars.name }}` | Test parameters, non-sensitive config | `{{ .vars.base_url }}` |
| **Runtime** | `{{ variable }}` | Values saved during test execution (including suite hooks) | `{{ user_id }}` |

> Suite-level and test-level hook saves both follow the runtime pattern: reference them as `{{ name }}` inside the relevant steps.

## Quick Decision Guide

**Use Environment Variables (`.env`) when:**
- ✅ The value is a secret (API key, password, token)
- ✅ The value changes per environment (staging URL vs production URL)
- ✅ The value should never be committed to git
- ✅ Different team members need different values

**Use Config Variables (`.vars`) when:**
- ✅ The value is test configuration (timeouts, limits)
- ✅ The value is safe to commit to git
- ✅ You want to override values via CLI (`--var`)
- ✅ The value is test data or parameters

**Use Runtime Variables when:**
- ✅ You need to save values from API responses
- ✅ You're chaining test steps together
- ✅ You need to pass data between steps
- ✅ The value is generated during test execution

## Basic Examples

### Environment Variables
```yaml
# Use for secrets
- name: "Authenticated API request"
  plugin: "http"
  config:
    url: "{{ .env.API_BASE_URL }}/users"
    headers:
      "Authorization": "Bearer {{ .env.API_TOKEN }}"
```

```bash
# Load from .env file
rocketship run -af test.yaml --env-file .env
```

### Config Variables
```yaml
# Define in YAML
vars:
  api_version: "v2"
  timeout: 30

tests:
  - name: "API test"
    steps:
      - plugin: "http"
        config:
          url: "{{ .vars.base_url }}/{{ .vars.api_version }}/users"
          timeout: "{{ .vars.timeout }}s"
```

```bash
# Override via CLI
rocketship run -af test.yaml --var api_version=v3 --var timeout=60
```

### Runtime Variables
```yaml
# Save from response
- name: "Create user"
  plugin: "http"
  config:
    method: "POST"
    url: "{{ .vars.base_url }}/users"
  save:
    - json_path: ".id"
      as: "user_id"

# Use in next step
- name: "Get user"
  plugin: "http"
  config:
    url: "{{ .vars.base_url }}/users/{{ user_id }}"
```

## Variable Precedence

When the same variable name exists in multiple places:

### Environment Variables
1. System environment (highest)
2. `--env-file` values
3. Default in test file

```bash
export API_KEY=system-key
rocketship run -af test.yaml --env-file .env  # Uses system-key, not .env
```

### Config Variables
1. `--var` CLI flags (highest)
2. `--var-file` values
3. YAML `vars` section

```bash
rocketship run -af test.yaml \
  --var-file overrides.yaml \
  --var timeout=120  # CLI wins
```

## Combining Variable Types

Use all three types together for maximum flexibility:

```yaml
vars:
  api_version: "v1"

tests:
  - name: "Mixed variable usage"
    steps:
      # Step 1: Use env + config
      - name: "Create resource"
        plugin: "http"
        config:
          url: "{{ .env.API_BASE_URL }}/{{ .vars.api_version }}/users"
          headers:
            "Authorization": "Bearer {{ .env.API_TOKEN }}"
        save:
          - json_path: ".id"
            as: "user_id"

      # Step 2: Use all three types
      - name: "Update resource"
        plugin: "http"
        config:
          url: "{{ .env.API_BASE_URL }}/{{ .vars.api_version }}/users/{{ user_id }}"
          headers:
            "Authorization": "Bearer {{ .env.API_TOKEN }}"
```

## Handlebars Escaping

When your API uses handlebars syntax `{{ }}`, escape them with backslashes:

```yaml
# Processed variable
"api_url": "{{ .env.API_BASE_URL }}/users"

# Literal handlebars (not processed)
"template": "Use \\{{ user_id }} in the API"
# Result: "Use {{ user_id }} in the API"
```

**Escape levels:**

| Input | Output | Description |
|-------|--------|-------------|
| `\\{{ var }}` | `{{ var }}` | Literal handlebars |
| `\\\\{{ .vars.x }}` | `\value` | Backslash + processed variable |
| `\\\\\\{{ var }}` | `\{{ var }}` | Backslash + literal handlebars |

**In JSON bodies:**
```yaml
body: |-
  {
    "api_url": "{{ .env.API_BASE_URL }}",
    "template": "Use \\{{ user_id }} for IDs",
    "docs": "Escape with \\{{ variable }} syntax"
  }
```

**Common use case - Supabase/PostgreSQL JSON:**
```yaml
# PostgreSQL JSONB query with literal handlebars
- name: "Query with JSON template"
  plugin: "sql"
  config:
    driver: "postgres"
    dsn: "{{ .env.DATABASE_URL }}"
    query: |
      SELECT metadata->>'template' as template
      FROM notifications
      WHERE metadata @> '{"type": "email"}'
```

If the `metadata` column contains `{"template": "Hello {{ name }}"}`, you need to escape it:
```yaml
# Insert with escaped handlebars
- name: "Insert notification template"
  plugin: "sql"
  config:
    driver: "postgres"
    dsn: "{{ .env.DATABASE_URL }}"
    query: |
      INSERT INTO notifications (metadata)
      VALUES ('{"template": "Hello \\{{ name }}"}'::jsonb)
```

## Best Practices

### Environment Variables
- ✅ Always use `.gitignore` for `.env` files
- ✅ Commit `.env.example` with dummy values
- ✅ Use different env files per environment (`.env.staging`, `.env.production`)
- ✅ Set secrets in CI/CD platform, not env files

### Config Variables
- ✅ Use descriptive names (`request_timeout` not `t`)
- ✅ Group related variables with nesting
- ✅ Put sensible defaults in YAML
- ✅ Override via `--var` for different test scenarios

### Runtime Variables
- ✅ Use `required: false` for optional saves
- ✅ Validate saved values in next step if critical
- ✅ Use descriptive names that indicate what was saved
- ✅ Clean up test data at the end of tests

### Security
```bash
# .gitignore
.env
.env.*
!.env.example
```

```yaml
# Validate required env vars
- name: "Check environment"
  plugin: "script"
  config:
    language: "javascript"
    script: |
      const required = ['API_KEY', 'DATABASE_URL'];
      const missing = required.filter(k => !process.env[k]);
      if (missing.length) {
        throw new Error(`Missing: ${missing.join(', ')}`);
      }
```

## Learn More

- **[Environment Variables](environment-variables.md)** - Detailed guide on `.env` usage, multi-environment setup, and CI/CD integration
- **[Config & Runtime Variables](config-variables.md)** - Detailed guide on `.vars`, `--var` overrides, and runtime variable chaining
