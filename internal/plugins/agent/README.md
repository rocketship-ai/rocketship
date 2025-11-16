# Agent Plugin

The agent plugin integrates the [Claude Agent SDK](https://github.com/anthropics/claude-agent-sdk-python) into Rocketship, enabling AI-powered test automation using Claude Code with full MCP (Model Context Protocol) server support.

## Overview

AI-powered test automation using Claude Code with MCP server support. Designed for QA testing workflows:

- **MCP Server Support**: Connect to Playwright, filesystem, APIs, etc.
- **Browser Sessions**: Integrates with Rocketship's `playwright` plugin via CDP
- **Session Management**: Single, continue, or resume modes
- **Non-Interactive**: Hardcoded to `bypassPermissions` for CI/CD pipelines
- **Consistent Output**: Always returns `{"ok": true/false, "result/error": "..."}`

Just write your task naturally - the agent handles MCP tools and returns structured JSON automatically.

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

### Full Configuration

```yaml
- name: "Advanced agent task"
  plugin: agent
  config:
    prompt: "Navigate to example.com and extract the page title"
    mode: single  # or continue/resume
    session_id: "my-session-{{ .run.id }}"
    max_turns: 5
    timeout: 2m
    system_prompt: "You are a QA testing expert"

    mcp_servers:
      playwright:
        type: stdio
        command: npx
        args: ["@playwright/mcp@latest"]
      remote_api:
        type: sse
        url: https://api.example.com/mcp
        headers:
          Authorization: "Bearer {{ .env.API_TOKEN }}"

    allowed_tools: ["*"]  # Wildcard = all tools

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
      session_id: "test-{{ .run.id }}" # Same session ID!

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

> **Note:** The permission mode is currently hardcoded to `bypassPermissions` for non-interactive, automated QA workflows. Users cannot configure permission modes at this time. The agent will never ask for user input or pause for permission.

### 7. Variable Saving and Passing

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

## Example: Browser Automation

```yaml
- name: "Navigate and extract data"
  plugin: agent
  config:
    prompt: "Navigate to example.com and extract the main heading"
    session_id: "browser-{{ .run.id }}"
    timeout: 2m
    mcp_servers:
      playwright:
        type: stdio
        command: npx
        args: ["@playwright/mcp@latest"]
  save:
    - json_path: ".result"
      as: "page_data"
```

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
