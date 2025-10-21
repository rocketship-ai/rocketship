# Agent Plugin

The agent plugin integrates the [Claude Agent SDK](https://github.com/anthropics/claude-agent-sdk-python) into Rocketship, enabling AI-powered test automation using Claude Code with full MCP (Model Context Protocol) server support.

## Overview

This plugin is a **complete rewrite** of the original agent plugin, designed specifically for **QA testing workflows** using the Claude Agent SDK:

- **MCP Server Support**: Connect to any MCP server (Playwright, filesystem, APIs, etc.)
- **Persistent Browser Sessions**: Seamlessly integrates with Rocketship's `playwright` plugin via CDP
- **Session Management**: Single, continue, or resume execution modes
- **Variable Passing**: Full support for saving and passing variables between steps
- **Test Assertions**: Return `{"ok": false}` to fail test steps
- **Non-Interactive**: Hardcoded to `bypassPermissions` mode - never asks for user input

**This plugin is intended to replace the `browser_use` plugin**, which has inferior performance compared to Claude Code with the Playwright MCP server.

### QA Testing Focus

The agent plugin is designed for **automated testing workflows**, not development. Key design decisions:

- **Permission mode is hardcoded** to `bypassPermissions` - the agent will never ask for user permission or pause for input
- **No file editing** - the agent can use MCP tools to interact with systems but won't modify code files
- **Consistent output schema** - every task always returns `{"ok": true/false, "result": "..." or "error": "..."}`
- **Non-blocking execution** - perfect for CI/CD pipelines and automated test suites

### Simple, Production-Ready Prompts

The plugin includes a default system prompt that ensures consistent output. Every task always returns the same JSON schema:

```json
{"ok": true, "result": "what the agent found/did"}
// or
{"ok": false, "error": "specific failure reason"}
```

**Just write your task naturally:**
```yaml
prompt: "Navigate to example.com and verify the page title is 'Example Domain'"
```

The agent will:
- Execute the task using available MCP servers
- Decide if it succeeded or failed
- Always return JSON with `ok: true/false`

You don't need to tell the agent how to format responses or what schema to use.

## Installation

Install the required Python dependencies:

```bash
pip install claude-agent-sdk
```

Ensure you have the `ANTHROPIC_API_KEY` environment variable set:

```bash
export ANTHROPIC_API_KEY=your_api_key_here
```

## Configuration

### Minimal Configuration

```yaml
- name: "Simple agent task"
  plugin: agent
  config:
    prompt: "Your task description here"

    # That's it! Everything else has sensible defaults:
    # - allowed_tools: ["*"] (wildcard - all MCP tools allowed)
    # - max_turns: unlimited
    # - timeout: unlimited
    # - cwd: where you ran 'rocketship run'
    # - permission_mode: bypassPermissions (hardcoded for QA testing)
```

### Full Configuration (All Optional Fields)

```yaml
- name: "Advanced agent task"
  plugin: agent
  config:
    # ===== REQUIRED =====
    # The task prompt
    prompt: |
      Navigate to https://example.com and extract the page title.
      Return the result as JSON: {"title": "page title"}

    # ===== OPTIONAL - Only specify if you need non-default behavior =====

    # Execution mode: single (default), continue, or resume
    mode: single

    # Session ID for continue/resume modes (required for resume)
    session_id: "my-session-{{ .run.id }}"

    # Maximum conversation turns (default: unlimited)
    # Only specify if you want to limit turns
    max_turns: 5

    # Timeout for agent execution (default: unlimited)
    # Only specify if you want a timeout
    timeout: 2m

    # System prompt prepended to conversation
    system_prompt: "You are a QA testing expert"

    # Working directory (default: where 'rocketship run' was executed)
    # Only specify if you need a different directory
    # cwd: /path/to/project

    # NOTE: Permission mode is hardcoded to 'bypassPermissions' for QA testing.
    # The agent will never ask for user permission or pause for input.
    # This ensures non-blocking execution in automated test pipelines.

    # MCP servers configuration
    mcp_servers:
      playwright:
        type: stdio  # or "sse" for HTTP/SSE servers
        command: npx
        args:
          - "@playwright/mcp@latest"
          # CDP endpoint is automatically added if session_id matches a playwright session
        env:
          DEBUG: "true"

      # Example SSE server
      remote_api:
        type: sse
        url: https://api.example.com/mcp
        headers:
          Authorization: "Bearer {{ .env.API_TOKEN }}"

    # Tool permissions (default: ["*"] wildcard)
    # Only specify if you want to restrict specific tools
    # allowed_tools:
    #   - mcp__playwright__browser_navigate
    #   - mcp__playwright__browser_click
    #   - mcp__playwright__browser_snapshot

  save:
    - json_path: ".result"
      as: "agent_response"
```

## Features

### 1. MCP Server Support

The agent plugin supports two types of MCP servers:

#### stdio Servers (subprocess-based)

```yaml
mcp_servers:
  playwright:
    type: stdio
    command: npx
    args: ["@playwright/mcp@latest"]
    env:
      DEBUG: "true"
```

#### SSE Servers (HTTP-based)

```yaml
mcp_servers:
  remote:
    type: sse
    url: https://api.example.com/mcp
    headers:
      Authorization: "Bearer token"
```

### 2. Persistent Browser Sessions with CDP

The agent plugin automatically integrates with the `playwright` plugin's browser sessions:

```yaml
steps:
  # Start a browser session with playwright plugin
  - name: "Start browser"
    plugin: playwright
    config:
      role: start
      session_id: "test-{{ .run.id }}"
      headless: false

  # Agent uses the same session via CDP
  - name: "Agent automates browser"
    plugin: agent
    config:
      prompt: "Navigate to example.com and click the login button"
      session_id: "test-{{ .run.id }}"  # Same session ID!

      mcp_servers:
        playwright:
          type: stdio
          command: npx
          args: ["@playwright/mcp@latest"]
          # CDP endpoint is automatically injected using session_id

      allowed_tools: ["*"]
```

**How it works:**
1. The `playwright` plugin starts a browser and saves the CDP WebSocket endpoint
2. The `agent` plugin reads the CDP endpoint using the same `session_id`
3. The agent automatically adds `--cdp-endpoint <ws_url>` to the Playwright MCP server args
4. Claude Code controls the existing browser via CDP

### 3. Execution Modes

#### Single Mode (Default)
One-off execution with no session persistence:

```yaml
config:
  prompt: "Generate a test user"
  mode: single
```

#### Continue Mode
Continues the most recent conversation:

```yaml
config:
  prompt: "Now add an email address to that user"
  mode: continue
```

#### Resume Mode
Resumes a specific session by ID:

```yaml
config:
  prompt: "Continue the previous task"
  mode: resume
  session_id: "my-session-123"
```

### 4. Tool Permissions

Control which MCP tools the agent can use:

```yaml
# Allow all tools (wildcard)
allowed_tools: ["*"]

# Or specify individual tools
allowed_tools:
  - mcp__playwright__browser_navigate
  - mcp__playwright__browser_click
  - mcp__playwright__browser_type
  - mcp__playwright__browser_snapshot
```

### 5. Permission Modes

Fine-grained control over agent capabilities:

- **`rejectEdits`** (default, **recommended for QA**): Agent can use tools (MCP servers) but cannot edit files. Perfect for testing/QA scenarios where the agent should interact with browsers, databases, APIs, etc., but not modify your codebase.
- **`acceptEdits`**: Agent can use tools AND edit files. Only use this if you explicitly want the agent to modify code.
- **`rejectAll`**: Agent cannot use any tools (text generation only). Rarely needed.

### 6. Variable Saving and Passing

Save agent responses and use them in subsequent steps:

```yaml
steps:
  - name: "Generate data"
    plugin: agent
    config:
      prompt: "Generate a test user as JSON"
    save:
      - json_path: ".result"
        as: "user_data"

  - name: "Use saved data"
    plugin: log
    config:
      message: "Generated user: {{ user_data }}"
```

## Examples

### Example 1: Simple Text Generation

```yaml
- name: "Generate test data"
  plugin: agent
  config:
    prompt: |
      Generate a fictional test user with:
      - first_name
      - last_name
      - email
      - age (between 25-45)

      Return ONLY a JSON object, no markdown.

    mode: single
    max_turns: 1
    timeout: 30s

  save:
    - json_path: ".result"
      as: "user_json"
```

### Example 2: Browser Automation with Playwright MCP

```yaml
- name: "Navigate and extract data"
  plugin: agent
  config:
    prompt: |
      Navigate to https://example.com.
      Find the main heading text and return it as JSON:
      {"heading": "text you found"}

    session_id: "browser-{{ .run.id }}"
    max_turns: 5
    timeout: 2m

    mcp_servers:
      playwright:
        type: stdio
        command: npx
        args: ["@playwright/mcp@latest"]

    allowed_tools: ["*"]

  save:
    - json_path: ".result"
      as: "page_data"
```

### Example 3: Interweaved Steps with Variable Passing

See `examples/agent-browser-testing/rocketship.yaml` for a complete example showing:
- Starting a persistent browser session
- Agent using Playwright MCP via CDP
- Variable extraction and passing between steps
- Multiple agent invocations on the same browser session
- Session cleanup

## Comparison to browser_use Plugin

| Feature | agent (new) | browser_use (deprecated) |
|---------|-------------|--------------------------|
| **AI Model** | Claude Code (latest Sonnet 4.5) | Various (GPT-4o, etc.) |
| **Browser Control** | Playwright MCP via CDP | browser-use Python library |
| **Accuracy** | Excellent | Poor |
| **MCP Support** | Full support for any MCP server | None |
| **Session Persistence** | Full CDP integration | Limited |
| **Tool Ecosystem** | Entire MCP ecosystem | browser-use only |
| **Variable Passing** | Full JSON path support | Limited |
| **Performance** | Fast with direct CDP | Slower with Python library |

**Recommendation**: Use the `agent` plugin with Playwright MCP instead of `browser_use` for all browser automation tasks.

## Troubleshooting

### Agent Executor Not Found

```
error: agent executor returned no output
```

**Solution**: Ensure claude-agent-sdk is installed:
```bash
pip install claude-agent-sdk
```

### ANTHROPIC_API_KEY Not Set

```
error: ANTHROPIC_API_KEY environment variable is required
```

**Solution**: Set your API key:
```bash
export ANTHROPIC_API_KEY=your_key_here
```

### Playwright MCP Not Found

```
error: command not found: npx
```

**Solution**: Install Node.js and the Playwright MCP server:
```bash
npm install -g @playwright/mcp
```

### CDP Connection Failed

```
warn: Failed to read session file for CDP connection
```

**Solution**: Ensure the `session_id` matches between `playwright` plugin (start) and `agent` plugin steps.

## Implementation Details

The agent plugin uses an embedded Python script (`agent_executor.py`) that:

1. Receives configuration as JSON from the Go plugin
2. Initializes the Claude Agent SDK with specified options
3. Configures MCP servers (with automatic CDP endpoint injection)
4. Executes the agent task
5. Returns results as JSON

The Python script is embedded in the binary using `go:embed` and extracted to a temporary directory on first use.

## API Reference

### Config Fields

- **`prompt`** (string, **required**): The task description for the agent
- **`mode`** (string): Execution mode - `single` (default), `continue`, or `resume`
- **`session_id`** (string): Session identifier (required for `resume` mode)
- **`max_turns`** (int): Maximum conversation turns (default: **unlimited**)
- **`timeout`** (string): Execution timeout (default: **unlimited**)
- **`system_prompt`** (string): System prompt prepended to conversation
- **`permission_mode`** (string): Permission level - `rejectEdits` (default), `acceptEdits`, or `rejectAll`
- **`cwd`** (string): Working directory for agent execution (default: where `rocketship run` was executed)
- **`mcp_servers`** (map): MCP server configurations (see MCP Server Config)
- **`allowed_tools`** ([]string): Tool permissions (default: **`["*"]`** wildcard for all tools)

### MCP Server Config

#### stdio Type
- **`type`**: "stdio"
- **`command`** (string, required): Command to execute
- **`args`** ([]string): Command arguments
- **`env`** (map[string]string): Environment variables

#### sse Type
- **`type`**: "sse"
- **`url`** (string, required): HTTP/SSE endpoint URL
- **`headers`** (map[string]string): HTTP headers

### Response Fields

- **`success`** (bool): Whether execution succeeded
- **`result`** (string): Agent's response text
- **`session_id`** (string): Session identifier (if applicable)
- **`mode`** (string): Execution mode used
- **`error`** (string): Error message (if failed)

## See Also

- [Claude Agent SDK Documentation](https://docs.claude.com/en/api/agent-sdk/python)
- [MCP Protocol](https://docs.claude.com/en/api/agent-sdk/mcp)
- [Playwright MCP Server](https://github.com/microsoft/playwright-mcp)
- [Rocketship Playwright Plugin](../playwright/README.md)
