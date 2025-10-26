# MCP Server

!!! warning "Early Beta - Not Recommended for Production Use"
    The Rocketship MCP server is in early beta and not recommended for general usage yet. Features and APIs may change without notice.

    **Tip**: Instead of using the MCP server, copy and paste [ROCKETSHIP_QUICKSTART.md](https://raw.githubusercontent.com/rocketship-ai/rocketship/main/ROCKETSHIP_QUICKSTART.md) into your coding agent's context window for a comprehensive reference.

Rocketship includes an MCP (Model Context Protocol) server that enables AI coding agents to access Rocketship examples, patterns, and guidance.

## Installation

Add to your MCP configuration (e.g., `.mcp.json` for Claude Code):

```json
{
  "mcpServers": {
    "rocketship": {
      "command": "npx",
      "args": ["-y", "@rocketshipai/mcp-server@latest"]
    }
  }
}
```

Restart your AI client after adding the configuration.

## Available Tools

### get_rocketship_examples

Get real examples for specific Rocketship features from the codebase.

**Supported plugins**: `http`, `delay`, `script`, `sql`, `log`, `agent`, `browser`, `supabase`

### suggest_test_structure

Suggests file structure and test organization based on project type.

**Project types**: `frontend`, `backend`, `fullstack`, `api`, `mobile`

### get_schema_info

Provides current schema information for validation and syntax.

**Sections**: `plugins`, `assertions`, `save`, `structure`, `full`

### get_cli_guidance

Provides CLI usage patterns and commands.

**Command types**: `run`, `validate`, `structure`

### get_rocketship_cli_installation_instructions

Get platform-specific installation instructions.

**Platforms**: `auto`, `macos-arm64`, `macos-intel`, `linux`, `windows`

### analyze_codebase_for_testing

Analyzes a codebase to suggest meaningful test scenarios.

**Focus areas**: `user_journeys`, `api_endpoints`, `critical_paths`, `integration_points`

## Integration Example

### Claude Code

Add to `.mcp.json`:

```json
{
  "mcpServers": {
    "rocketship": {
      "command": "npx",
      "args": ["-y", "@rocketshipai/mcp-server@latest"]
    }
  }
}
```

Then ask Claude:

```
"I need to install Rocketship and create API tests for my Express.js authentication endpoints."
```

Claude will use the MCP server to provide installation instructions and relevant examples.

### Other Editors

**Cursor**: Settings > Features > Enable Model Context Protocol, then add the same config

**Windsurf**: Add to `~/.codeium/windsurf/mcp_config.json`

Any MCP-compatible client can use the server via JSON-RPC over stdio.

## Usage Tips

Ask for specific guidance rather than complete generation:

- "How do I install Rocketship on my platform?"
- "Show me examples of API authentication testing"
- "What assertions work best for user profile endpoints?"
- "How should I structure an E2E checkout flow test?"

## Troubleshooting

### MCP Server Not Found

1. Ensure Node.js 18+ is installed
2. Check MCP configuration syntax
3. Restart your AI client
4. Verify npm package accessibility

### No Guidance Provided

1. Be more specific in requests
2. Provide context about what you're testing
3. Ask for specific features or patterns

## Environment Variables

- `ROCKETSHIP_LOG`: Set log level (DEBUG, INFO, ERROR)
- `NODE_ENV`: Development/production mode
