# Playwright Plugin

Deterministic browser automation using Python scripts for precise DOM manipulation and fast assertions.

## Quick Start

```yaml
- name: "Navigate and verify"
  plugin: playwright
  config:
    role: script
    script: |
      from playwright.sync_api import expect

      page.goto("https://example.com")
      expect(page).to_have_url("https://example.com/")
      expect(page.locator("h1")).to_have_text("Example Domain")

      result = {"title": page.title()}
```

## Configuration

### Common Fields

| Field | Description | Example |
|-------|-------------|---------|
| `role` | Operation: `start`, `script`, or `stop` | `script` |
| `session_id` | Session identifier for persistent sessions | `"checkout-{{ .run.id }}"` |

### Fields for `start` Role

| Field | Description | Default | Example |
|-------|-------------|---------|---------|
| `session_id` | **Required.** Unique session identifier | - | `"checkout-{{ .run.id }}"` |
| `headless` | Run browser in headless mode | `false` | `true` |
| `window_width` | Browser window width (pixels) | `1280` | `1920` |
| `window_height` | Browser window height (pixels) | `720` | `1080` |
| `slow_mo_ms` | Delay between actions (milliseconds) | `0` | `100` |
| `launch_args` | Additional Chromium launch arguments | `[]` | `["--disable-blink-features=AutomationControlled"]` |
| `launch_timeout_ms` | Browser launch timeout (milliseconds) | `45000` | `60000` |

**Note:** Default `headless: false` shows the browser window for local testing. Set `headless: true` for CI/CD environments.

### Fields for `script` Role

| Field | Description | Example |
|-------|-------------|---------|
| `session_id` | **Required.** Session identifier - must match an active session from `role: start` | `"checkout-{{ .run.id }}"` |
| `script` | **Required.** Python Playwright code | See examples below |
| `language` | Script language | `python` (only supported value) |
| `env` | Environment variables available to script | `{"API_KEY": "{{ .env.API_KEY }}"}` |

## Roles

### Using `script` Role

The `script` role executes Python Playwright code against an existing browser session. You must start a session first with `role: start`:

### `start` / `script` / `stop` (Persistent Sessions)

For sharing browser state across multiple steps or with other plugins (like `browser_use`), manage the session explicitly:

```yaml
# Start browser session
- name: "Start browser"
  plugin: playwright
  config:
    role: start
    session_id: "checkout-{{ .run.id }}"
    headless: true

# Use the session
- name: "Navigate"
  plugin: playwright
  config:
    role: script
    session_id: "checkout-{{ .run.id }}"
    script: |
      page.goto("https://example.com")

# Stop when done
- name: "Stop browser"
  plugin: playwright
  config:
    role: stop
    session_id: "checkout-{{ .run.id }}"
```

See [Persistent Browser Sessions](browser/persistent-sessions.md) for details on sharing sessions with `browser_use` and `agent` plugins.

## Common Patterns

### Navigation & Interaction

```python
# Navigate
page.goto("https://example.com")
page.wait_for_load_state("networkidle")

# Click & fill
page.click("button#submit")
page.fill("input[name='email']", "test@example.com")
page.select_option("select#country", "US")

# Wait for elements
page.wait_for_selector("#content")
```

### Assertions

```python
from playwright.sync_api import expect

# Page assertions
expect(page).to_have_url("https://example.com/")
expect(page).to_have_title("Example Domain")

# Element assertions
expect(page.locator("h1")).to_have_text("Welcome")
expect(page.locator("#login")).to_be_visible()
expect(page.locator(".item")).to_have_count(5)
```

### Saving Data

```python
# Save to Rocketship state
result = {
  "title": page.title(),
  "url": page.url,
  "user_id": page.locator("#user-id").inner_text()
}
```

```yaml
save:
  - json_path: ".user_id"
    as: "user_id"
```

## Complete Example

```yaml
- name: "Login flow"
  plugin: playwright
  config:
    env:
      TEST_EMAIL: "{{ .env.TEST_EMAIL }}"
      TEST_PASSWORD: "{{ .env.TEST_PASSWORD }}"
    script: |
      import os
      from playwright.sync_api import expect

      page.goto("https://app.example.com/login")
      page.fill("input[name='email']", os.environ["TEST_EMAIL"])
      page.fill("input[name='password']", os.environ["TEST_PASSWORD"])
      page.click("button[type='submit']")

      expect(page).to_have_url("https://app.example.com/dashboard")
```

## Combining with Other Plugins

```yaml
# Get auth token from API
- name: "Get token"
  plugin: http
  config:
    method: POST
    url: "{{ .env.API_URL }}/auth/token"
  save:
    - json_path: ".token"
      as: "auth_token"

# Use in browser
- name: "Set token"
  plugin: playwright
  config:
    env:
      AUTH_TOKEN: "{{ auth_token }}"
    script: |
      import os
      page.goto("https://app.example.com")
      page.evaluate(f"localStorage.setItem('token', '{os.environ['AUTH_TOKEN']}')")
      page.reload()
```

## Best Practices

- **Use specific selectors**: Prefer IDs and data attributes over classes
- **Wait for elements**: Use `wait_for_selector` for dynamic content
- **Use expect**: Playwright's expect provides automatic retries
- **Pass secrets via env**: Never hardcode credentials in scripts

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Browser won't start | Run `playwright install chromium` |
| Element not found | Add `wait_for_selector` before interacting |
| Timeout errors | Increase timeout in wait methods |
| Flaky tests | Use `expect` assertions with built-in retries |

## See Also

- [Agent Plugin](agent.md) - AI-powered browser testing
- [Browser Use Plugin](browser-use.md) - AI-driven browser automation
- [Variables](../features/variables.md) - Using environment variables
