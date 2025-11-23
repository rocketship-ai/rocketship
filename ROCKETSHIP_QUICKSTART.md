# Rocketship Agent Quickstart

Short reference for coding agents (Cursor, Claude Code, Windsurf, etc.) to write and run Rocketship tests.

## What is Rocketship?

Rocketship is a testing framework for browser and API testing. A Rocketship test suite is a YAML file (typically kept in a `.rocketship` directory) made up of one or more tests. Each test has steps, and each step calls a plugin (http, supabase, playwright, agent, etc.).

You can run a single file with `-f` or an entire `.rocketship` directory with `-d`. Use `-a` to automatically start and stop the local server.

## Installation

```bash
# macOS
brew tap rocketship-ai/tap && brew install rocketship

# Linux/macOS (portable)
curl -fsSL https://raw.githubusercontent.com/rocketship-ai/rocketship/main/scripts/install.sh | bash

# ----- PREREQS -----
# required for local runs
brew install temporal
# required for browser testing
pip install playwright
playwright install chromium
# required for agent plugin steps
pip install claude-agent-sdk
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
rocketship run -af test.yaml      # Run a test file with auto start/stop of local server
rocketship run -ad .rocketship    # starts the local engine, runs the tests, shuts the engine down
```

The -a flag is an extremely important flag to use if you're running tests locally (not connecting to the remote cloud). It will automatically start and stop the local server for you after the tests run, so you don't have to manually start and stop the server. If you really wanted to start and stop the server manually, do:

```bash
rocketship start server -b # starts the local server in the background
rocketship run test.yaml # runs the tests against the local server
rocketship stop server # stops the local server
```

I highly recommend you just use the -a flag and let Rocketship handle the server for you.

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

**Environment variables**:

1. System: `export API_KEY=abc && rocketship run -af test.yaml`
2. .env file: `rocketship run -af test.yaml --env-file .env`
3. Custom .env path: `rocketship run -af test.yaml --env-file config/.env.staging`

**Config variables**:

1. In YAML: `vars: { base_url: "https://api.example.com" }`
2. Override via CLI: `rocketship run -af test.yaml --var base_url=https://staging.api.example.com`
3. From var file: `rocketship run -af test.yaml --var-file vars.yaml`

**Precedence**:

- Environment: System env > `--env-file`
- Config: `--var` CLI flags > `--var-file` > YAML `vars`

## Core Plugins (Quick Examples)

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
  save:
    - json_path: ".id"
      as: "user_id"
```

### Supabase Plugin

```yaml
- plugin: supabase
  config:
    url: "{{ .env.SUPABASE_URL }}"
    key: "{{ .env.SUPABASE_KEY }}"
    operation: "auth_sign_up"
    auth:
      email: "test-{{ .run.id }}@example.com"
      password: "password123"
  save:
    - json_path: ".user.id"
      as: "user_id"
```

More operations: `select`, `insert`, `update`, `delete`, `rpc`, `auth_sign_in`, `auth_create_user`, `auth_delete_user`, `storage_*`. See docs for full examples.

### Log Plugin

```yaml
- plugin: log
  config:
    message: "Starting auth flow for run {{ .run.id }}"
    level: "INFO"
```

### Playwright Plugin (Scripted Browser)

```yaml
- plugin: playwright
  config:
    role: script
    script: |
      from playwright.sync_api import expect

      page.goto("{{ .env.FRONTEND_URL }}/login")
      page.locator("#email").fill("test@example.com")
      page.locator("#password").fill("password123")
      page.locator("button[type='submit']").click()

      expect(page).to_have_url("{{ .env.FRONTEND_URL }}/dashboard")
```

### Agent Plugin (AI-Driven)

```yaml
- plugin: agent
  config:
    prompt: |
      In the current browser session, verify:
      - Login form has email and password fields
      - Submit button is present and enabled
      - Successful login shows "Hello {{ login_email }}" somewhere on the page
    capabilities: ["browser"]
```

**Capabilities**: `["browser"]` lets the agent hook into a browser session created by Rocketship (e.g., via the Playwright plugin).

### Script Plugin

```yaml
- plugin: script
  config:
    language: javascript
    script: |
      const email = state.user_email || "test@example.com";
      save("normalized_email", email.toLowerCase());
```

## Plugin Docs

| Plugin        | Description                                                            | Docs URL                                        |
| ------------- | ---------------------------------------------------------------------- | ----------------------------------------------- |
| `http`        | HTTP/API testing                                                       | https://docs.rocketship.sh/plugins/http/        |
| `supabase`    | Supabase DB/auth/storage                                               | https://docs.rocketship.sh/plugins/supabase/    |
| `sql`         | SQL databases                                                          | https://docs.rocketship.sh/plugins/sql/         |
| `agent`       | AI-driven testing with browser tools                                   | https://docs.rocketship.sh/plugins/agent/       |
| `playwright`  | Scripted browser automation                                            | https://docs.rocketship.sh/plugins/playwright/  |
| `browser_use` | Cheaper AI for browser automation but way less performant than `agent` | https://docs.rocketship.sh/plugins/browser-use/ |
| `script`      | JS/shell scripting                                                     | https://docs.rocketship.sh/plugins/script/      |
| `log`         | Logging within tests                                                   | https://docs.rocketship.sh/plugins/log/         |
| `delay`       | Fixed delays between steps                                             | https://docs.rocketship.sh/plugins/delay/       |

## Advanced Features

- **Lifecycle Hooks**: `init`, `cleanup.always`, `cleanup.on_failure` for setup/teardown.
  Docs: https://docs.rocketship.sh/features/lifecycle-hooks/
- **Retry Policies**: `retry` block on steps for backoff and retries.
  Docs: https://docs.rocketship.sh/features/retry-policies/
- **Variable Passing**: Built-in, Environment, Config, and Runtime.
  Docs: https://docs.rocketship.sh/features/variables/
- **Assertions & Save**: Multiple assertion types (`status_code`, `json_path`, `supabase_count`, etc.) and save targets (`json_path`, `header`, `sql_result`, etc.).
  Docs: https://docs.rocketship.sh/yaml-reference/plugin-reference

## Reference

- Full docs: https://docs.rocketship.sh
- Plugin reference: https://docs.rocketship.sh/yaml-reference/plugin-reference
- Command reference: https://docs.rocketship.sh/reference/rocketship
- GitHub: https://github.com/rocketship-ai/rocketship
