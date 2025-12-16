# Browser Testing - AI-Powered Web Automation

> ℹ️ **Alternative Available**: The `agent` plugin provides browser testing via Playwright MCP and can combine browser automation with other tools (filesystem, APIs, database). See [Agent Plugin Examples](agent-testing.md) for the MCP-based approach.

Test web applications using persistent browser sessions that combine deterministic scripting (Playwright) with AI-driven automation (browser_use).

## Quick Start

```bash
# Install dependencies
pip install playwright browser-use langchain-openai
playwright install chromium

# Set API key
export OPENAI_API_KEY=sk-your-key-here
```

## Basic Workflow

```yaml
name: "Browser Test"
tests:
  - name: "Test with AI"
    cleanup:
      always:
        - name: "stop browser"
          plugin: playwright
          config:
            role: stop
            session_id: "test-{{ .run.id }}"
    steps:
      # 1. Start browser session
      - name: "start browser"
        plugin: playwright
        config:
          role: start
          session_id: "test-{{ .run.id }}"
          headless: true

      # 2. Navigate with Playwright (deterministic)
      - name: "navigate to site"
        plugin: playwright
        config:
          role: script
          session_id: "test-{{ .run.id }}"
          language: python
          script: |
            from playwright.sync_api import expect
            page.goto("https://example.com")
            expect(page).to_have_url("https://example.com/")

      # 3. Verify with AI (intelligent)
      - name: "verify content"
        plugin: browser_use
        config:
          session_id: "test-{{ .run.id }}"
          task: "Verify the page has a heading 'Example Domain'"
          max_steps: 3
          use_vision: true
          llm:
            provider: "openai"
            model: "gpt-4o"
            config:
              OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
```

## Key Concepts

### Session Sharing

Both plugins share a single Chromium instance via Chrome DevTools Protocol (CDP):

- **Playwright** - Deterministic scripts, precise DOM manipulation, fast assertions
- **browser_use** - AI-driven tasks, natural language instructions, visual understanding

### Structured Output

browser_use returns structured test results (pass/fail) for reliable QA automation:

```json
{
  "status": "pass",
  "message": "Verified heading 'Example Domain' exists",
  "error": null
}
```

## Common Patterns

### Login Flow

```yaml
steps:
  - name: "start browser"
    plugin: playwright
    config:
      role: start
      session_id: "login-test"

  - name: "navigate to login"
    plugin: playwright
    config:
      role: script
      session_id: "login-test"
      script: |
        page.goto("https://app.example.com/login")

  - name: "perform login with AI"
    plugin: browser_use
    config:
      session_id: "login-test"
      task: |
        Fill in the login form:
        - Email: test@example.com
        - Password: testpass123
        Click login and verify dashboard appears.
      max_steps: 5
      llm:
        provider: "openai"
        model: "gpt-4o"
        config:
          OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
```

### Data Extraction

```yaml
steps:
  - name: "navigate to pricing"
    plugin: playwright
    config:
      role: script
      session_id: "scraper"
      script: |
        page.goto("https://example.com/pricing")

  - name: "extract prices"
    plugin: browser_use
    config:
      session_id: "scraper"
      task: "Extract all plan names and monthly prices"
      use_vision: true
      max_steps: 3
      llm:
        provider: "openai"
        model: "gpt-4o"
        config:
          OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
    save:
      - json_path: ".result"
        as: "pricing_data"
```

### Form Validation

```yaml
steps:
  - name: "submit form"
    plugin: playwright
    config:
      role: script
      session_id: "form-test"
      script: |
        page.fill('input[name="name"]', 'Test User')
        page.click('button[type="submit"]')

  - name: "verify success"
    plugin: browser_use
    config:
      session_id: "form-test"
      task: "Check if success message appears"
      llm:
        provider: "openai"
        model: "gpt-4o"
```

## Configuration Reference

### Playwright Options

```yaml
- plugin: playwright
  config:
    role: start | script | stop
    session_id: "unique-id"          # Required
    headless: true                   # Optional (default: true)
    window_width: 1280               # Optional (default: 1280)
    window_height: 720               # Optional (default: 720)
    script: |                        # For role: script
      # Python code with `page` object available
      result = {"key": "value"}
```

### browser_use Options

```yaml
- plugin: browser_use
  config:
    session_id: "unique-id"          # Required (must match playwright)
    task: "Natural language task"    # Required
    max_steps: 10                    # Optional (default: 10)
    use_vision: true                 # Optional (default: false)
    timeout: "5m"                    # Optional (default: "5m")
    llm:                             # Required
      provider: "openai"             # "openai" or "anthropic"
      model: "gpt-4o"                # Optional (uses provider default)
      config:                        # Optional (see API Key Configuration below)
        OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
```

**API Key Configuration:**

The plugin needs access to your LLM provider's API key. You have two options:

1. **Environment variable** (recommended): Export `OPENAI_API_KEY` or `ANTHROPIC_API_KEY` in your shell, and the plugin will automatically use it:
   ```bash
   export OPENAI_API_KEY=sk-your-key-here
   ```
   ```yaml
   llm:
     provider: "openai"  # No config block needed
   ```

2. **Explicit config**: Pass the key explicitly via the `llm.config` block:
   ```yaml
   llm:
     provider: "openai"
     config:
       OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"  # Read from env
       # or
       OPENAI_API_KEY: "sk-your-key-here"  # Hardcoded (not recommended)
   ```

The `{{ .env.VARIABLE_NAME }}` template syntax reads from environment variables, which is useful when you want to explicitly document which variables the test requires.

## Best Practices

**Always use cleanup blocks** to stop browsers:
```yaml
cleanup:
  always:
    - name: "stop browser"
      plugin: playwright
      config:
        role: stop
        session_id: "my-session"
```

**Be specific in AI tasks**:
```yaml
# ✅ Good
task: "Find the first product under $50 and click 'Add to Cart'"

# ❌ Vague
task: "Check the website"
```

**Set appropriate timeouts**:
```yaml
timeout: "30s"  # Simple tasks
timeout: "2m"   # Multi-step workflows
timeout: "5m"   # Complex AI tasks
```

**Enable vision for visual elements**:
```yaml
use_vision: true  # Better for charts, images, complex layouts
```

## Troubleshooting
**Session not found**: Ensure `session_id` matches between steps
**Timeout errors**: Increase `timeout` or reduce `max_steps`
**Browser won't start**: Run `playwright install chromium`

## See Also
- [Persistent Sessions Guide](../../plugins/browser/persistent-sessions.md)
- YAML Reference for full plugin documentation
