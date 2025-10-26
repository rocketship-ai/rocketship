# Rocketship Agent Quickstart

Comprehensive reference for coding agents, like you, to write Rocketship tests.

## What is Rocketship?

Testing framework for browser and API testing. A rocketship.yaml file is a test suite for Rocketship. Made up of 1 or more tests. Each test is made up of 1 or more steps. Each step is a plugin that is executed. You can run a single file with the -f flag, or an entire .rocketship directory with the -d flag.

If you are wanting to run tests locally, use the -a flag to automatically start and stop the local server.

## Installation

```bash
# macOS
brew tap rocketship-ai/tap && brew install rocketship

# Linux/macOS (portable)
curl -fsSL https://raw.githubusercontent.com/rocketship-ai/rocketship/main/scripts/install.sh | bash

# Prerequisites (if you want to run tests locally)
brew install temporal
```

## Basic Test Structure

```yaml
name: "Test Suite Name"
tests:
  - name: "Test Case Name"
    steps:
      - name: "Step Name"
        plugin: "http"
        config:
          method: "GET"
          url: "https://api.example.com/endpoint"
        assertions:
          - type: "status_code"
            expected: 200
        save:
          - json_path: ".id"
            as: "resource_id"
```

## Running Tests

```bash
rocketship run -af test.yaml      # Run a test file with auto start and stop the local server
rocketship run -ad .rocketship --debug     # Run an entire .rocketship directory, also with debug logging
```

## Variables

```yaml
# Built-in
{{ .run.id }}  # Unique test run ID

# Environment (from .env file or system env variables)
{{ .env.API_KEY }}
{{ .env.DATABASE_URL }}

# Config (from vars section in YAML or var file)
{{ .vars.base_url }}
{{ .vars.timeout }}

# Runtime (saved during execution)
{{ user_id }}  # From previous save
```

### Passing Variables and Environment

**Environment variables** (3 ways):

1. From system: `export API_KEY=abc && rocketship run -af test.yaml`
2. From .env file: `rocketship run -af test.yaml --env-file .env`
3. From .env file with custom path: `rocketship run -af test.yaml --env-file config/.env.staging`

**Config variables** (3 ways):

1. In YAML: `vars: { base_url: "https://api.example.com" }`
2. Override via CLI: `rocketship run -af test.yaml --var base_url=https://staging.api.example.com`
3. From var file: `rocketship run -af test.yaml --var-file vars.yaml`

**Precedence**:

- Environment: System env > `--env-file`
- Config: `--var` CLI flags > `--var-file` > YAML `vars`

## Core Plugins

### HTTP Plugin

```yaml
- plugin: http
  config:
    method: "POST"
    url: "{{ .vars.api_url }}/users"
    headers:
      Authorization: "Bearer {{ .env.API_TOKEN }}"
    body: |
      {
        "email": "test-{{ .run.id }}@example.com",
        "name": "Test User"
      }
  assertions:
    - type: "status_code"
      expected: 201
    - type: "json_path"
      path: ".email"
      expected: "test-{{ .run.id }}@example.com"
  save:
    - json_path: ".id"
      as: "user_id"
    - header: "X-Request-ID"
      as: "request_id"
```

### Supabase Plugin

```yaml
- plugin: supabase
  config:
    url: "{{ .env.SUPABASE_URL }}"
    key: "{{ .env.SUPABASE_KEY }}"
    operation: "insert"
    table: "users"
    insert:
      data:
        email: "test@example.com"
        name: "Test User"
  save:
    - json_path: ".data[0].id"
      as: "user_id"
```

**Supported operations**: `select`, `insert`, `update`, `delete`, `rpc`, `auth_sign_up`, `auth_sign_in`, `auth_create_user`, `auth_delete_user`, `storage_upload`, `storage_download`, `storage_delete`, `storage_create_bucket`, `storage_delete_bucket`

[Full docs with examples](https://docs.rocketship.sh/plugins/supabase/)

### Agent Plugin (AI-Driven)

```yaml
- plugin: agent
  config:
    prompt: |
      Navigate to {{ .env.FRONTEND_URL }}/login and verify:
      - Login form is visible with email and password fields
      - Submit button is present
      - Form validation works correctly
    max_turns: 10
    timeout: "5m"
    capabilities: ["browser"]
```

**Capabilities**: Agent can use tools based on configured capabilities. Browser capability (`browser`) is the only one currently available in the agent plugin config. Browser capability allows the agent to hook into a browser session that rocketship makes.

### Playwright Plugin (Scripted Browser)

```yaml
- plugin: playwright
  config:
    script: |
      from playwright.sync_api import expect

      page.goto("{{ .env.FRONTEND_URL }}/login")
      page.locator("#email").fill("test@example.com")
      page.locator("#password").fill("password123")
      page.locator("button[type='submit']").click()

      expect(page).to_have_url("{{ .env.FRONTEND_URL }}/dashboard")
```

### Other Plugins

For full examples, curl the docs at https://docs.rocketship.sh/plugins/

- **SQL**: Database operations (PostgreSQL, MySQL, SQLite, SQL Server) - [docs](https://docs.rocketship.sh/plugins/sql/)
- **Script**: JavaScript/shell execution - [docs](https://docs.rocketship.sh/plugins/script/)
- **Delay**: Fixed delays between steps - [docs](https://docs.rocketship.sh/plugins/delay/)
- **Log**: Output messages during execution - [docs](https://docs.rocketship.sh/plugins/log/)
- **Browser Use**: AI-driven browser (poor performance, use Agent instead) - [docs](https://docs.rocketship.sh/plugins/browser-use/)

## Lifecycle Hooks

```yaml
name: "API Test Suite"

init:
  - name: "Get auth token"
    plugin: http
    config:
      method: POST
      url: "{{ .env.API_URL }}/auth/token"
    save:
      - json_path: ".token"
        as: "api_token"

tests:
  - name: "Test with token"
    steps:
      - plugin: http
        config:
          headers:
            Authorization: "Bearer {{ api_token }}"

cleanup:
  always:
    - name: "Cleanup test data"
      plugin: http
      config:
        method: DELETE
        url: "{{ .env.API_URL }}/test-data/{{ .run.id }}"

  on_failure:
    - name: "Collect logs"
      plugin: log
      config:
        message: "Test failed for run {{ .run.id }}"
```

**Execution order**: `init` → `tests` → `cleanup.on_failure` (if failed) → `cleanup.always`

Test-level hooks work the same way. [Full docs](https://docs.rocketship.sh/features/lifecycle-hooks/)

## Retry Policies

```yaml
- plugin: http
  config:
    url: "{{ .vars.api_url }}/endpoint"
  retry:
    maximum_attempts: 5
    initial_interval: "1s"
    backoff_coefficient: 2.0 # 1s → 2s → 4s → 8s
    non_retryable_errors: ["ValidationError"]
```

[Full docs](https://docs.rocketship.sh/features/retry-policies/)

## Assertions

| Type                 | Example                                                                                                     |
| -------------------- | ----------------------------------------------------------------------------------------------------------- |
| `status_code`        | `type: "status_code"`, `expected: 200`                                                                      |
| `json_path`          | `type: "json_path"`, `path: ".user.email"`, `expected: "test@example.com"`                                  |
| `header`             | `type: "header"`, `name: "Content-Type"`, `expected: "application/json"`                                    |
| `row_count` (SQL)    | `type: "row_count"`, `query_index: 0`, `expected: 5`                                                        |
| `column_value` (SQL) | `type: "column_value"`, `query_index: 0`, `row_index: 0`, `column: "email"`, `expected: "test@example.com"` |
| `supabase_count`     | `type: "supabase_count"`, `expected: 1`                                                                     |

## Save Fields

| Type         | Example                                               |
| ------------ | ----------------------------------------------------- |
| `json_path`  | `json_path: ".id"`, `as: "user_id"`                   |
| `header`     | `header: "X-Request-ID"`, `as: "request_id"`          |
| `sql_result` | `sql_result: ".queries[0].rows[0].id"`, `as: "db_id"` |

## Example: E2E User Flow

```yaml
tests:
  - name: "User registration flow"
    steps:
      - name: "Register user"
        plugin: http
        config:
          method: POST
          url: "{{ .env.API_URL }}/auth/register"
          body: |
            {"email": "test-{{ .run.id }}@example.com", "password": "Test123!"}
        save:
          - json_path: ".user_id"
            as: "user_id"

      - name: "Login"
        plugin: http
        config:
          method: POST
          url: "{{ .env.API_URL }}/auth/login"
          body: |
            {"email": "test-{{ .run.id }}@example.com", "password": "Test123!"}
        save:
          - json_path: ".access_token"
            as: "token"

      - name: "Access protected resource"
        plugin: http
        config:
          method: GET
          url: "{{ .env.API_URL }}/users/{{ user_id }}"
          headers:
            Authorization: "Bearer {{ token }}"
        assertions:
          - type: "status_code"
            expected: 200
```

## Best Practices

1. Use `{{ .run.id }}` in test data for uniqueness
2. Clean up resources with `cleanup.always` hooks
3. Keep tests isolated and independent
4. Store secrets in `.env` files (never commit)
5. Add retry policies to flaky network operations
6. Use descriptive step names for debugging
7. Prefer Agent plugin over Browser Use for performance

## Reference

- Full docs: https://docs.rocketship.sh
- Plugin reference: https://docs.rocketship.sh/yaml-reference/plugin-reference
- Command reference: https://docs.rocketship.sh/reference/rocketship
- GitHub: https://github.com/rocketship-ai/rocketship
