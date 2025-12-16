# Browser Use Plugin

!!! warning "Poor Performance - Use Agent Plugin Instead"
    The Browser Use plugin has poor performance and is not recommended. Use the [Agent plugin](agent.md) with browser capability instead for better performance and reliability.

AI-driven browser automation using natural language tasks with GPT-4o or Claude.

## Quick Start

```yaml
- name: "Start browser"
  plugin: playwright
  config:
    role: start
    session_id: "test-{{ .run.id }}"

- name: "Verify page content"
  plugin: browser_use
  config:
    session_id: "test-{{ .run.id }}"
    task: "Verify the page has a heading 'Example Domain'"
    max_steps: 3
    use_vision: true
    llm:
      provider: "openai"
      model: "gpt-4o"
```

## Prerequisites

```bash
# Install dependencies
pip install playwright browser-use langchain-openai langchain-anthropic
playwright install chromium

# Set API key
export OPENAI_API_KEY=sk-your-key-here
# or
export ANTHROPIC_API_KEY=sk-ant-your-key-here
```

## Configuration

### Required Fields

| Field          | Description           | Example                                    |
| -------------- | --------------------- | ------------------------------------------ |
| `session_id`   | Browser session ID    | `"test-{{ .run.id }}"` (must match Playwright session) |
| `task`         | Natural language task | `"Click login button and verify redirect"` |
| `llm.provider` | LLM provider          | `"openai"` or `"anthropic"`                |

### Optional Fields

| Field         | Description                  | Default                |
| ------------- | ---------------------------- | ---------------------- |
| `max_steps`   | Maximum agent steps          | `10`                   |
| `use_vision`  | Enable vision capabilities   | `false`                |
| `temperature` | LLM temperature (0.0-2.0)    | Not set (model default)|
| `timeout`     | Task execution timeout       | `5m`                   |
| `llm.model`   | LLM model name               | `gpt-4o` (OpenAI)      |
| `llm.config`  | LLM configuration (API keys) | Auto-detected from env |

## LLM Configuration

### OpenAI

```yaml
llm:
  provider: "openai"
  model: "gpt-4o"
  config:
    OPENAI_API_KEY: "{{ .env.OPENAI_API_KEY }}"
```

Or auto-detect from environment:

```bash
export OPENAI_API_KEY=sk-your-key-here
```

```yaml
llm:
  provider: "openai" # Automatically uses OPENAI_API_KEY from env
```

### Anthropic

```yaml
llm:
  provider: "anthropic"
  config:
    ANTHROPIC_API_KEY: "{{ .env.ANTHROPIC_API_KEY }}"
```

## Common Patterns

### Login Flow

```yaml
- name: "Start browser"
  plugin: playwright
  config:
    role: start
    session_id: "test-{{ .run.id }}"

- name: "Complete login"
  plugin: browser_use
  config:
    session_id: "test-{{ .run.id }}"
    task: |
      Navigate to {{ .env.FRONTEND_URL }}/login and:
      - Fill email: {{ .env.TEST_EMAIL }}
      - Fill password: {{ .env.TEST_PASSWORD }}
      - Click login
      - Verify dashboard appears
    max_steps: 5
    llm:
      provider: "openai"
```

### Data Extraction

```yaml
- name: "Start browser"
  plugin: playwright
  config:
    role: start
    session_id: "test-{{ .run.id }}"

- name: "Extract pricing"
  plugin: browser_use
  config:
    session_id: "test-{{ .run.id }}"
    task: "Extract all plan names and monthly prices"
    use_vision: true
    max_steps: 3
    llm:
      provider: "openai"
      model: "gpt-4o"
  save:
    - json_path: ".result"
      as: "pricing_data"
```

### Form Validation

```yaml
- name: "Start browser"
  plugin: playwright
  config:
    role: start
    session_id: "test-{{ .run.id }}"

- name: "Verify success message"
  plugin: browser_use
  config:
    session_id: "test-{{ .run.id }}"
    task: "Check if success message appears after form submission"
    max_steps: 2
    llm:
      provider: "openai"
```

## Combining with Other Plugins

### With Playwright

```yaml
- name: "Navigate with Playwright"
  plugin: playwright
  config:
    script: |
      page.goto("https://example.com")
      page.fill("#api-key", "test-key-123")

- name: "Verify with AI"
  plugin: browser_use
  config:
    session_id: "test-{{ .run.id }}"
    task: "Verify the API key field shows 'test-key-123'"
    llm:
      provider: "openai"

### With HTTP

```yaml
- name: "Get test data from API"
  plugin: http
  config:
    method: GET
    url: "{{ .env.API_URL }}/test-data"
  save:
    - json_path: ".user_email"
      as: "test_email"

- name: "Use in browser"
  plugin: browser_use
  config:
    task: "Login with email {{ test_email }}"
```

## Response Format

browser_use returns structured results:

```json
{
  "status": "pass",
  "message": "Task completed successfully",
  "result": "Found heading 'Example Domain'"
}
```

## Vision Mode

Enable for visual elements like charts, images, or complex layouts:

```yaml
use_vision: true
```

## Best Practices

- **Be specific**: `"Find product under $50 and add to cart"` not `"Check website"`
- **Set max_steps**: Limit agent iterations (2-5 steps for most tasks)
- **Use vision selectively**: Only enable for visual content (slower, more tokens)
- **Simplify tasks**: Keep tasks focused and achievable
- **Combine with playwright**: Use playwright for setup, browser_use for verification

## Troubleshooting

| Issue               | Solution                                                 |
| ------------------- | -------------------------------------------------------- |
| Timeout errors      | Increase `timeout` or reduce `max_steps`                 |
| API errors          | Check API key and quota limits                           |
| Browser won't start | Run `playwright install chromium`                        |
| Task fails          | Simplify the task, add more specific instructions        |
| High cost           | Reduce `max_steps`, disable `use_vision` when not needed |

## Agent Plugin vs browser_use

| Feature          | browser_use               | agent (MCP)                       |
| ---------------- | ------------------------- | --------------------------------- |
| **Model**        | GPT-4o or Claude          | Claude Sonnet 4.5                 |
| **Tools**        | Browser only              | Browser + Files + APIs + Database |
| **Architecture** | Integrated library        | MCP server protocol               |
| **Use Case**     | Simple browser automation | Complex multi-tool workflows      |

**Recommendation**: Use the [Agent plugin](agent.md) for new projects. It offers better integration, multi-tool support, and Claude Sonnet 4.5's advanced capabilities.

## See Also

- [Agent Plugin](agent.md) - Recommended alternative with Claude Sonnet 4.5
- [Playwright Plugin](playwright.md) - Deterministic browser automation
- [Variables](../features/variables.md) - Passing data to tasks
