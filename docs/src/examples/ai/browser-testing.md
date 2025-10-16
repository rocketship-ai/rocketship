# Browser Testing - AI-Powered Web Automation

Test web applications using persistent browser sessions with Playwright and browser-use. This approach combines deterministic scripting with AI-driven automation, all sharing a single Chromium instance.

## Overview

Rocketship provides two complementary plugins for browser testing:

- **`playwright`** - Launches and manages persistent Chromium sessions, runs deterministic Python scripts
- **`browser_use`** - Executes AI-driven tasks using natural language instructions

Both plugins share a single browser session via Chrome DevTools Protocol (CDP), enabling powerful workflows that mix scripted actions with intelligent automation.

## Prerequisites

```bash
# Install dependencies
pip install playwright browser-use langchain-openai langchain-anthropic
playwright install chromium

# Set API key for browser-use
export OPENAI_API_KEY=sk-your-key-here
# OR
export ANTHROPIC_API_KEY=sk-ant-your-key-here
```

## Basic Workflow

```yaml
name: "Basic Browser Test"
tests:
  - name: "Test website with AI"
    cleanup:
      always:
        - name: "cleanup browser"
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
          window_width: 1280
          window_height: 720

      # 2. Navigate with Playwright
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
            result = {"url": page.url, "title": page.title()}

      # 3. AI verification with browser_use
      - name: "verify content with AI"
        plugin: browser_use
        config:
          session_id: "test-{{ .run.id }}"
          task: "Verify the page has a heading 'Example Domain' and summarize the content"
          max_steps: 3
          use_vision: true
          timeout: "2m"
          llm:
            provider: "openai"
            model: "gpt-4o"
            config:
              OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
```

## Playwright Configuration

### Starting a Browser Session

```yaml
- name: "start browser"
  plugin: playwright
  config:
    role: start
    session_id: "my-session"           # Required: Unique session identifier
    headless: true                      # Optional: Run without visible browser (default: true)
    window_width: 1280                  # Optional: Browser width (default: 1280)
    window_height: 720                  # Optional: Browser height (default: 720)
    slow_mo_ms: 100                     # Optional: Slow motion for debugging (default: 0)
    launch_timeout_ms: 30000            # Optional: Browser launch timeout (default: 30000)
    launch_args:                        # Optional: Custom Chrome flags
      - "--disable-gpu"
```

### Running Scripts

```yaml
- name: "execute playwright script"
  plugin: playwright
  config:
    role: script
    session_id: "my-session"
    language: python                    # Only Python supported
    script: |
      from playwright.sync_api import expect

      # Full Playwright API available
      page.goto("https://example.com")
      page.click("text=More information")

      # Return data via result dict
      result = {
        "url": page.url,
        "title": page.title()
      }
    env:                                # Optional: Environment variables
      MY_VAR: "{{ .env.MY_VAR }}"
  save:
    - json_path: ".url"
      as: "current_url"
```

### Stopping a Session

```yaml
- name: "stop browser"
  plugin: playwright
  config:
    role: stop
    session_id: "my-session"
```

## browser_use Configuration

```yaml
- name: "AI-driven task"
  plugin: browser_use
  config:
    session_id: "my-session"            # Required: Must match playwright session
    task: |                              # Required: Natural language instruction
      Navigate to the products page and extract all product names and prices

    # LLM Configuration (Required)
    llm:
      provider: "openai"                 # "openai" or "anthropic"
      model: "gpt-4o"                    # Model name
      config:
        OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"

    # Optional settings
    max_steps: 10                        # Max AI steps (default: 10)
    use_vision: true                     # Enable visual analysis (default: false)
    timeout: "5m"                        # Max execution time (default: "5m")
    temperature: 0.7                     # LLM temperature (optional)
    allowed_domains:                     # Domain restrictions (optional)
      - "example.com"
```

## Common Patterns

### Login Flow Test

```yaml
- name: "Test login"
  steps:
    - name: "start browser"
      plugin: playwright
      config:
        role: start
        session_id: "login-test"
        headless: false  # Watch it work

    - name: "navigate to login"
      plugin: playwright
      config:
        role: script
        session_id: "login-test"
        script: |
          page.goto("https://app.example.com/login")
          result = {"login_page_loaded": True}

    - name: "perform login with AI"
      plugin: browser_use
      config:
        session_id: "login-test"
        task: |
          Fill in the login form:
          - Email: test@example.com
          - Password: testpass123
          Click the login button and verify you see the dashboard.
        max_steps: 5
        llm:
          provider: "openai"
          model: "gpt-4o"
          config:
            OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
```

### Data Extraction

```yaml
- name: "Scrape pricing data"
  steps:
    - name: "navigate to pricing"
      plugin: playwright
      config:
        role: script
        session_id: "scraper"
        script: |
          page.goto("https://example.com/pricing")
          result = {"page_ready": True}

    - name: "extract prices with AI"
      plugin: browser_use
      config:
        session_id: "scraper"
        task: "Extract all plan names and their monthly prices from the pricing table"
        use_vision: true  # Better for visual elements
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

### Form Submission with Validation

```yaml
- name: "Submit contact form"
  steps:
    - name: "fill form with Playwright"
      plugin: playwright
      config:
        role: script
        session_id: "form-test"
        script: |
          page.goto("https://example.com/contact")
          page.fill('input[name="name"]', 'Test User')
          page.fill('input[name="email"]', 'test@example.com')
          page.fill('textarea[name="message"]', 'Test message')
          page.click('button[type="submit"]')
          result = {"form_submitted": True}

    - name: "verify success with AI"
      plugin: browser_use
      config:
        session_id: "form-test"
        task: "Check if a success message appears confirming the form was submitted"
        max_steps: 2
        llm:
          provider: "openai"
          model: "gpt-4o"
          config:
            OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
```

## Parallel Browser Sessions

Run multiple independent browser sessions simultaneously:

```yaml
tests:
  - name: "Mobile test"
    steps:
      - name: "start mobile browser"
        plugin: playwright
        config:
          role: start
          session_id: "mobile-{{ .run.id }}"
          window_width: 375
          window_height: 667

  - name: "Desktop test"
    steps:
      - name: "start desktop browser"
        plugin: playwright
        config:
          role: start
          session_id: "desktop-{{ .run.id }}"
          window_width: 1920
          window_height: 1080
```

Each test runs independently with its own browser instance.

## Variable Passing

Pass data between steps using save/assertions:

```yaml
- name: "get product URL"
  plugin: playwright
  config:
    role: script
    session_id: "shop"
    script: |
      result = {"product_url": "https://example.com/product/123"}
  save:
    - json_path: ".product_url"
      as: "product_link"

- name: "navigate to product"
  plugin: browser_use
  config:
    session_id: "shop"
    task: "Go to {{ product_link }} and add the item to cart"
    llm:
      provider: "openai"
      model: "gpt-4o"
      config:
        OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
```

## Best Practices

### Session Management

Always use cleanup blocks to ensure browsers are stopped:

```yaml
cleanup:
  always:
    - name: "stop browser"
      plugin: playwright
      config:
        role: stop
        session_id: "my-session"
```

### Task Clarity

Be specific in browser_use tasks:

```yaml
# ❌ Vague
task: "Check the website"

# ✅ Specific
task: "Navigate to /products, find the first product with price under $50, and click its 'Add to Cart' button"
```

### Timeout Configuration

Set appropriate timeouts for complex tasks:

```yaml
# Simple tasks: 30s-1m
timeout: "30s"

# Multi-step workflows: 2-5m
timeout: "3m"

# Complex AI tasks: 5m+
timeout: "5m"
```

### Headless vs Headful

```yaml
# CI/CD and production: use headless
headless: true

# Local debugging: watch the browser
headless: false
```

### Vision Mode

Enable vision for visual elements:

```yaml
use_vision: true  # Better accuracy for charts, images, complex layouts
```

## Troubleshooting

**Session not found**: Ensure session_id matches exactly between start/script/stop

**Timeout errors**: Increase timeout or reduce max_steps
```yaml
timeout: "10m"
max_steps: 5
```

**Browser won't start**: Install Playwright browsers
```bash
playwright install chromium
```

**Script errors**: Check Python syntax and Playwright API usage
```yaml
script: |
  from playwright.sync_api import expect
  # Your code here
```

## Running Examples

```bash
# Run persistent session example
rocketship run -af examples/browser/persistent-session/checkout.yaml

# Run parallel sessions test
rocketship run -af test-parallel-browser-sessions.yaml

# With debug logging
ROCKETSHIP_LOG=DEBUG rocketship run -af your-test.yaml
```

## See Also

- [Persistent Sessions Guide](../../plugins/browser/persistent-sessions.md) - Detailed technical documentation
- [Playwright Plugin](../../plugins/playwright.md) - Full API reference
- [browser_use Plugin](../../plugins/browser_use.md) - AI automation reference
