# Agent Plugin - AI-Powered Testing with MCP Servers

Use Claude with MCP (Model Context Protocol) servers to perform intelligent browser testing, file analysis, and multi-tool workflows in your tests.

> 💡 **New Architecture**: This plugin uses the Claude Agent SDK with MCP server support, enabling AI-driven testing with access to browser control, filesystem, APIs, and more.

## Prerequisites

```bash
# Set Anthropic API key
export ANTHROPIC_API_KEY=sk-ant-your-key-here

# Install MCP servers (if using)
npm install -g @playwright/mcp@0.0.43
```

## Quick Start: Browser Testing

```yaml
tests:
  - name: "Login Test"
    cleanup:
      always:
        - name: "Stop browser"
          plugin: playwright
          config:
            role: stop
            session_id: "login-{{ .run.id }}"
    steps:
      - name: "Start browser"
        plugin: playwright
        config:
          role: start
          session_id: "login-{{ .run.id }}"
          headless: false

      - name: "Agent: Login and verify"
        plugin: agent
        config:
          prompt: |
            Navigate to {{ .env.FRONTEND_URL }}/login and login with:
            - Email: {{ .env.TEST_EMAIL }}
            - Password: {{ .env.TEST_PASSWORD }}

            Verify you land on the dashboard page.
          session_id: "login-{{ .run.id }}"
          mcp_servers:
            playwright:
              type: stdio
              command: npx
              args: ["@playwright/mcp@0.0.43"]
```

## Core Concepts

### MCP Servers

The agent plugin connects to MCP servers to access tools. Common servers:

- **Playwright MCP** (`@playwright/mcp`): Browser control, screenshots, page navigation
- **Filesystem MCP**: Read/write files, search directories
- **Custom MCPs**: Build your own for API integrations, database access, etc.

### Browser Session Handoff

The agent can control browsers started by the `playwright` plugin via CDP:

```yaml
- name: "Start browser (Playwright)"
  plugin: playwright
  config:
    role: start
    session_id: "test-{{ .run.id }}"

- name: "Test with AI (Agent)"
  plugin: agent
  config:
    session_id: "test-{{ .run.id }}"  # Same session!
    prompt: "Click the login button"
    mcp_servers:
      playwright:
        type: stdio
        command: npx
        args: ["@playwright/mcp@0.0.43"]
```

## Configuration

### Required Fields

```yaml
config:
  prompt: "Your natural language task"  # Required
  mcp_servers:                          # Required (unless using default)
    <server_name>:
      type: stdio | sse
      command: "npx"                    # For stdio servers
      args: ["@playwright/mcp"]
```

### Optional Fields

```yaml
config:
  session_id: "browser-session"   # For browser testing (matches playwright)
  mode: "single"                  # "single", "continue", or "resume"
  max_turns: 10                   # Max agent loop iterations (default: unlimited)
  timeout: "2m"                   # Max execution time (default: 5m)
  system_prompt: "Custom instructions for Claude"
  allowed_tools: ["*"]            # Tool filter (default: all)
```

## Common Use Cases

### 1. Multi-Step Browser Workflows

```yaml
- name: "Agent: Complete checkout flow"
  plugin: agent
  config:
    prompt: |
      Complete a checkout flow:
      1. Add product ID {{ product_id }} to cart
      2. Navigate to cart
      3. Click checkout
      4. Fill shipping form with test data
      5. Verify order summary shows {{ product_id }}
    session_id: "checkout-{{ .run.id }}"
    max_turns: 15
    mcp_servers:
      playwright:
        type: stdio
        command: npx
        args: ["@playwright/mcp@0.0.43"]
  save:
    - json_path: ".result"
      as: "checkout_result"
```

### 2. Data Extraction

```yaml
- name: "Agent: Extract pricing data"
  plugin: agent
  config:
    prompt: |
      Go to /pricing and extract all plan names and prices.
      Return as JSON array: [{"name": "...", "price": "..."}]
    session_id: "scraper"
    mcp_servers:
      playwright:
        type: stdio
        command: npx
        args: ["@playwright/mcp@0.0.43"]
  save:
    - json_path: ".result"
      as: "pricing_plans"
  assertions:
    - type: json_path
      path: ".ok"
      expected: true
```

### 3. Multi-Tool Workflows (Browser + Database)

```yaml
- name: "Get expected vehicle count"
  plugin: supabase
  config:
    url: "{{ .env.SUPABASE_URL }}"
    key: "{{ .env.SUPABASE_SERVICE_KEY }}"
    operation: "select"
    table: "vehicles"
    select:
      columns: ["id"]
      filters:
        - column: "company_id"
          operator: "eq"
          value: "{{ company_id }}"
  save:
    - json_path: ". | length"
      as: "vehicle_count"

- name: "Agent: Verify fleet map"
  plugin: agent
  config:
    prompt: |
      Navigate to /fleet and verify:
      - Map displays {{ vehicle_count }} vehicle markers (may be clustered)
      - Vehicle cards at bottom show {{ vehicle_count }} vehicles
    session_id: "fleet-test"
    mcp_servers:
      playwright:
        type: stdio
        command: npx
        args: ["@playwright/mcp@0.0.43"]
```

## Advanced Features

**Continue Mode**: Chain prompts with shared context
```yaml
mode: "continue"  # Continues previous conversation
```

**System Prompts**: Customize agent behavior
```yaml
system_prompt: "You are a QA agent. Always return JSON: {\"ok\": bool, \"result\": string}"
```

**Tool Filtering**: Restrict available tools
```yaml
allowed_tools: ["browser_*", "screenshot"]  # Specific tools only
```

## Comparison with browser_use Plugin

| Feature | agent (MCP) | browser_use |
|---------|-------------|-------------|
| **Speed** | ✅ Fast (single inference) | ❌ Slow (multi-step loop) |
| **Model** | Claude Sonnet 4.5 (256k context) | GPT-4o or Claude |
| **Tools** | Browser + Files + APIs + Custom | Browser only |
| **Reliability** | ✅ High (fewer deps) | ❌ Lower (complex stack) |
| **Cost** | ✅ Lower (1 API call) | ❌ Higher (multiple calls) |

**Recommendation**: Use the agent plugin for all browser testing unless you specifically need `browser_use` for legacy compatibility.

## Best Practices

- **Be specific**: `"Click 'Add to Cart' for product 123"` not `"Add something to cart"`
- **Set timeouts**: 30s (simple), 2m (multi-step), 5m (complex/default)
- **Always cleanup**: Use `cleanup.always` to stop browsers
- **Handle dynamic content**: `"Wait for spinner to disappear, then verify..."`

## Troubleshooting

| Issue | Solution |
|-------|----------|
| MCP connection error | `npm list -g @playwright/mcp` |
| Session not found | Verify `session_id` matches playwright step |
| Agent timeout | Increase `timeout` or reduce `max_turns` |
| Tool execution fails | Run with `--debug` to see available tools |

## See Also

- [Playwright Plugin](../../plugins/browser/persistent-sessions.md) - Browser session management
- [browser_use Plugin](browser-testing.md) - Alternative (not recommended)
- [MCP Servers](https://modelcontextprotocol.io/) - Build custom servers
