# Rocketship Agent Quickstart

This is a comprehensive reference for coding agents (Claude Code, Cursor, Windsurf, etc.) to write Rocketship tests effectively.

## What is Rocketship?

Rocketship is an open-source testing framework for browser and API testing with durable execution via Temporal. Tests are defined in YAML and execute steps sequentially with automatic retries, state management, and comprehensive assertions.

## Installation

```bash
# macOS
brew tap rocketship-ai/tap && brew install rocketship

# Linux/macOS (portable)
curl -fsSL https://raw.githubusercontent.com/rocketship-ai/rocketship/main/scripts/install.sh | bash

# Prerequisites (for local engine)
brew install temporal  # macOS
```

## Basic Test Structure

```yaml
name: "Test Suite Name"
description: "Optional description"
vars:
  api_url: "https://api.example.com"

tests:
  - name: "Test Case Name"
    steps:
      - name: "Step Name"
        plugin: "http"
        config:
          method: "GET"
          url: "{{ .vars.api_url }}/endpoint"
        assertions:
          - type: "status_code"
            expected: 200
        save:
          - json_path: ".id"
            as: "resource_id"
```

## Running Tests

```bash
rocketship run -af test.yaml              # Auto-start local engine
rocketship run -f test.yaml               # Use existing engine
rocketship run -af test.yaml --debug      # Debug logging
rocketship validate test.yaml             # Validate syntax
```

## Variables

### 1. Built-in Variables
```yaml
{{ .run.id }}  # Unique test run ID
```

### 2. Environment Variables
```yaml
{{ .env.API_KEY }}        # From system environment
{{ .env.DATABASE_URL }}   # From .env file
```

Load: `rocketship run -af test.yaml --env-file .env`

### 3. Config Variables
```yaml
vars:
  base_url: "https://api.example.com"
  timeout: 30

steps:
  - config:
      url: "{{ .vars.base_url }}/users"
```

Override: `rocketship run -af test.yaml --var base_url=https://staging.api.example.com`

### 4. Runtime Variables (Saved During Execution)
```yaml
- plugin: http
  config:
    method: POST
    url: "{{ .vars.api_url }}/users"
  save:
    - json_path: ".id"
      as: "user_id"

- plugin: http
  config:
    url: "{{ .vars.api_url }}/users/{{ user_id }}"  # Use saved variable
```

## Plugins

### HTTP Plugin
```yaml
- plugin: http
  config:
    method: "POST"
    url: "{{ .vars.api_url }}/users"
    headers:
      Authorization: "Bearer {{ .env.API_TOKEN }}"
      Content-Type: "application/json"
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
    - type: "header"
      name: "Content-Type"
      expected: "application/json"
  save:
    - json_path: ".id"
      as: "user_id"
    - json_path: ".email"
      as: "user_email"
    - header: "X-Request-ID"
      as: "request_id"
```

### SQL Plugin
```yaml
- plugin: sql
  config:
    driver: "postgres"  # postgres, mysql, sqlite, sqlserver
    dsn: "{{ .env.DATABASE_URL }}"
    commands:
      - "INSERT INTO users (email) VALUES ('test@example.com') RETURNING id;"
      - "SELECT * FROM users WHERE email = 'test@example.com';"
  assertions:
    - type: "query_count"
      expected: 2
    - type: "row_count"
      query_index: 1
      expected: 1
    - type: "column_value"
      query_index: 1
      row_index: 0
      column: "email"
      expected: "test@example.com"
  save:
    - sql_result: ".queries[0].rows[0].id"
      as: "db_user_id"
```

### Supabase Plugin
```yaml
# Insert
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

# Select with filters
- plugin: supabase
  config:
    operation: "select"
    table: "users"
    select:
      columns: ["id", "email", "name"]
      filters:
        - column: "email"
          operator: "eq"
          value: "test@example.com"
      order:
        - column: "created_at"
          ascending: false
      limit: 10
  assertions:
    - type: "supabase_count"
      expected: 1

# Update
- plugin: supabase
  config:
    operation: "update"
    table: "users"
    update:
      data:
        name: "Updated Name"
      filters:
        - column: "id"
          operator: "eq"
          value: "{{ user_id }}"

# Delete
- plugin: supabase
  config:
    operation: "delete"
    table: "users"
    delete:
      filters:
        - column: "id"
          operator: "eq"
          value: "{{ user_id }}"

# Auth
- plugin: supabase
  config:
    operation: "auth_sign_up"
    auth:
      email: "user@example.com"
      password: "password123"

# RPC
- plugin: supabase
  config:
    operation: "rpc"
    rpc:
      function: "get_user_stats"
      params:
        user_id: "{{ user_id }}"
```

### Agent Plugin (AI-Driven with Claude)
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
```

**Browser capability** (automatic when any browser plugin is used):
```yaml
- plugin: agent
  config:
    prompt: |
      Using the browser:
      1. Navigate to {{ .env.FRONTEND_URL }}
      2. Click the "Sign Up" button
      3. Fill in the registration form
      4. Verify success message appears
```

### Playwright Plugin (Scripted Browser)
```yaml
- plugin: playwright
  config:
    script: |
      from playwright.sync_api import expect

      page.goto("{{ .env.FRONTEND_URL }}/login")
      expect(page).to_have_url("{{ .env.FRONTEND_URL }}/login")

      page.locator("#email").fill("test@example.com")
      page.locator("#password").fill("password123")
      page.locator("button[type='submit']").click()

      expect(page).to_have_url("{{ .env.FRONTEND_URL }}/dashboard")
    env:
      CUSTOM_VAR: "value"
```

### Browser Use Plugin (AI-Driven Browser - Poor Performance)
> **Warning**: Use the Agent plugin with browser capability instead for better performance.

```yaml
- plugin: browser_use
  config:
    task: "Navigate to the login page and verify the form is present"
    max_steps: 5
    use_vision: true
    llm:
      provider: "openai"
      model: "gpt-4o"
```

### Script Plugin
```yaml
# JavaScript
- plugin: script
  config:
    language: "javascript"
    script: |
      const data = JSON.parse(context.previousResponse);
      const total = data.items.reduce((sum, item) => sum + item.price, 0);

      save("total_price", total);
      assert(total > 0, "Total price must be positive");

# Shell
- plugin: script
  config:
    language: "shell"
    script: |
      #!/bin/bash
      curl -s {{ .vars.api_url }}/health | jq -r '.status'
    timeout: "30s"
```

### Delay Plugin
```yaml
- plugin: delay
  config:
    duration: "2s"  # 500ms, 2s, 1m, 5m
```

### Log Plugin
```yaml
- plugin: log
  config:
    message: "Created user with ID: {{ user_id }}"
```

## Lifecycle Hooks

### Suite-Level Hooks
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
          url: "{{ .env.API_URL }}/users"
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
      plugin: http
      config:
        method: GET
        url: "{{ .env.OPS_URL }}/logs?run={{ .run.id }}"
```

### Test-Level Hooks
```yaml
tests:
  - name: "User CRUD test"
    init:
      - name: "Create test user"
        plugin: http
        config:
          method: POST
          url: "{{ .env.API_URL }}/users"
        save:
          - json_path: ".id"
            as: "user_id"

    steps:
      - name: "Update user"
        plugin: http
        config:
          method: PATCH
          url: "{{ .env.API_URL }}/users/{{ user_id }}"

    cleanup:
      always:
        - name: "Delete user"
          plugin: http
          config:
            method: DELETE
            url: "{{ .env.API_URL }}/users/{{ user_id }}"
```

**Execution order**: `init` → `steps` → `cleanup.on_failure` (if failed) → `cleanup.always`

## Retry Policies

```yaml
- plugin: http
  config:
    method: GET
    url: "{{ .vars.api_url }}/eventually-consistent"
  retry:
    maximum_attempts: 5
    initial_interval: "1s"
    backoff_coefficient: 2.0      # Exponential: 1s → 2s → 4s → 8s
    maximum_interval: "30s"        # Cap at 30s
    non_retryable_errors:
      - "ValidationError"
      - "AuthenticationError"
```

## Assertions

| Type | Fields | Example |
|------|--------|---------|
| `status_code` | `expected` | `expected: 200` |
| `json_path` | `path`, `expected` | `path: ".user.email"`, `expected: "test@example.com"` |
| `header` | `name`, `expected` | `name: "Content-Type"`, `expected: "application/json"` |
| `row_count` | `query_index`, `expected` | `query_index: 0`, `expected: 5` |
| `query_count` | `expected` | `expected: 2` |
| `column_value` | `query_index`, `row_index`, `column`, `expected` | See SQL example |
| `supabase_count` | `expected` | `expected: 1` |
| `supabase_error` | `expected` | `expected: null` |

## Save Fields

| Type | Fields | Example |
|------|--------|---------|
| `json_path` | `as`, `required` | `json_path: ".id"`, `as: "user_id"` |
| `header` | `as`, `required` | `header: "X-Request-ID"`, `as: "request_id"` |
| `sql_result` | `as`, `required` | `sql_result: ".queries[0].rows[0].id"`, `as: "db_id"` |

## Common Patterns

### E2E User Journey
```yaml
name: "E2E User Registration Flow"

tests:
  - name: "Complete user registration"
    steps:
      - name: "Register user"
        plugin: http
        config:
          method: POST
          url: "{{ .env.API_URL }}/auth/register"
          body: |
            {
              "email": "test-{{ .run.id }}@example.com",
              "password": "Test123!@#"
            }
        save:
          - json_path: ".user_id"
            as: "user_id"

      - name: "Verify user in database"
        plugin: sql
        config:
          driver: postgres
          dsn: "{{ .env.DATABASE_URL }}"
          commands:
            - "SELECT * FROM users WHERE id = {{ user_id }};"
        assertions:
          - type: "row_count"
            query_index: 0
            expected: 1

      - name: "Login with new user"
        plugin: http
        config:
          method: POST
          url: "{{ .env.API_URL }}/auth/login"
          body: |
            {
              "email": "test-{{ .run.id }}@example.com",
              "password": "Test123!@#"
            }
        assertions:
          - type: "status_code"
            expected: 200
        save:
          - json_path: ".access_token"
            as: "access_token"

      - name: "Access protected resource"
        plugin: http
        config:
          method: GET
          url: "{{ .env.API_URL }}/users/{{ user_id }}/profile"
          headers:
            Authorization: "Bearer {{ access_token }}"
        assertions:
          - type: "status_code"
            expected: 200
```

### CRUD with Cleanup
```yaml
tests:
  - name: "CRUD operations"
    init:
      - name: "Create resource"
        plugin: http
        config:
          method: POST
          url: "{{ .vars.api_url }}/resources"
        save:
          - json_path: ".id"
            as: "resource_id"

    steps:
      - name: "Read resource"
        plugin: http
        config:
          method: GET
          url: "{{ .vars.api_url }}/resources/{{ resource_id }}"

      - name: "Update resource"
        plugin: http
        config:
          method: PUT
          url: "{{ .vars.api_url }}/resources/{{ resource_id }}"

    cleanup:
      always:
        - name: "Delete resource"
          plugin: http
          config:
            method: DELETE
            url: "{{ .vars.api_url }}/resources/{{ resource_id }}"
```

### Browser + API Verification
```yaml
tests:
  - name: "UI action with backend verification"
    steps:
      - name: "Submit form via browser"
        plugin: agent
        config:
          prompt: |
            Navigate to {{ .env.FRONTEND_URL }}/users/create
            Fill in the form:
            - Name: "Test User"
            - Email: "test-{{ .run.id }}@example.com"
            Submit and verify success message

      - name: "Wait for async processing"
        plugin: delay
        config:
          duration: "2s"

      - name: "Verify user created in database"
        plugin: sql
        config:
          driver: postgres
          dsn: "{{ .env.DATABASE_URL }}"
          commands:
            - "SELECT * FROM users WHERE email = 'test-{{ .run.id }}@example.com';"
        assertions:
          - type: "row_count"
            query_index: 0
            expected: 1
```

## Best Practices for Coding Agents

1. **Use unique identifiers**: Always use `{{ .run.id }}` in test data to avoid collisions
2. **Clean up resources**: Use `cleanup.always` hooks to delete test data
3. **Test isolation**: Each test should be independent and not rely on other tests
4. **Error handling**: Use `cleanup.on_failure` to collect debugging info
5. **Retry flaky operations**: Add retry policies to network calls and eventual consistency checks
6. **Variable naming**: Use descriptive names for saved variables (`user_id`, not `id`)
7. **Environment variables**: Store secrets in `.env` files, never commit them
8. **Browser testing**: Prefer `agent` plugin with browser capability over `browser_use` for performance
9. **Assertions**: Add multiple assertions to verify different aspects of responses
10. **Documentation**: Add descriptive `name` fields to all steps for better debugging

## tryme Test Server

For testing without external dependencies, use the hosted test server at `tryme.rocketship.sh`:

```yaml
steps:
  - plugin: http
    config:
      url: "https://tryme.rocketship.sh/users"
      method: "POST"
      headers:
        X-Test-Session: "{{ .run.id }}"  # Session isolation
      body: |
        {
          "name": "Test User",
          "email": "test@example.com"
        }
```

**Session isolation**: Use `X-Test-Session` header to ensure concurrent test runs don't interfere.

## Reference Documentation

- Full docs: https://docs.rocketship.sh
- Plugin reference: https://docs.rocketship.sh/plugins/
- GitHub: https://github.com/rocketship-ai/rocketship
