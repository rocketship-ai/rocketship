# Rocketship MCP Server

A Model Context Protocol (MCP) server that assists AI coding agents in writing better Rocketship tests by providing examples, patterns, and guidance rather than generating complete test files.

## Philosophy

This MCP server is designed as a **knowledgeable assistant** that helps coding agents understand Rocketship testing patterns and best practices. The agent maintains full control over file creation while receiving expert guidance.

## Features

- **Real Examples**: Provides actual examples from the current Rocketship codebase
- **Test Structure Guidance**: Suggests file organization and project structure
- **Schema Information**: Current schema validation rules and syntax
- **CLI Installation Help**: Step-by-step installation instructions for all platforms
- **CLI Command Reference**: Current usage patterns extracted from CLI introspection
- **Codebase Analysis**: Suggests test scenarios based on your project type

## Installation

### Using npx (Recommended)

No installation required! Just add to your MCP configuration:

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

### Manual Installation

```bash
npm install -g @rocketshipai/mcp-server
```

## Configuration

Add to your MCP client configuration file:

### Claude Code

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

### Cursor

Add to Cursor Settings > MCP Servers:

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

### Windsurf

Add to `~/.codeium/windsurf/mcp_config.json`:

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

## Available Tools

### 1. `get_rocketship_examples`

Get real examples and best practices for specific Rocketship features from the current codebase.

**Plugins:** `http`, `delay`, `script`, `sql`, `log`, `agent`, `playwright`, `browser_use`, `supabase`

### 2. `suggest_test_structure`

Suggests proper file structure and test organization based on current project configuration.

**Project Types:** `frontend`, `backend`, `fullstack`, `api`, `mobile`

### 3. `get_schema_info`

Provides current schema information for validation and proper syntax.

**Sections:** `plugins`, `assertions`, `save`, `structure`, `full`

### 4. `get_cli_guidance`

Provides current CLI usage patterns and commands from introspection.

**Command Types:** `run`, `validate`, `structure`

### 5. `get_rocketship_cli_installation_instructions`

Get step-by-step instructions for installing the Rocketship CLI on different platforms.

**Platforms:** `auto`, `macos-arm64`, `macos-intel`, `linux`, `windows`

### 6. `analyze_codebase_for_testing`

Analyzes a codebase to suggest meaningful test scenarios based on available plugins.

**Focus Areas:** `user_journeys`, `api_endpoints`, `critical_paths`, `integration_points`

## Usage Examples

### Installing Rocketship CLI

```
"How do I install Rocketship on my Mac?"

Agent asks for: get_rocketship_cli_installation_instructions(platform="macos-arm64")
You get: Platform-specific installation commands, verification steps, and troubleshooting
You run: The installation commands to get Rocketship set up
```

### Getting Real Examples

```
"Show me examples of HTTP testing with step chaining"

Agent asks for: get_rocketship_examples(feature_type="http", use_case="step chaining")
You get: Real YAML examples from the codebase, variable usage patterns
You create: Your own test file based on the actual patterns
```

### Structuring Tests for Projects

```
"I need a test structure for my React e-commerce frontend"

Agent asks for: suggest_test_structure(project_type="frontend", user_flows=["checkout", "authentication"])
You get: Directory structure recommendations, plugin suggestions
You create: Organized test structure following the guidance
```

### Analyzing Your Codebase

```
"What should I test in my Express.js API?"

Agent asks for: analyze_codebase_for_testing(codebase_info="Express.js API with authentication", focus_area="api_endpoints")
You get: Suggested test scenarios, critical path identification
You implement: Tests for the most important functionality
```

## Key Benefits

- **Educational**: Learn Rocketship patterns while building tests
- **Flexible**: Adapt examples to your specific needs
- **Reliable**: No file generation issues or directory conflicts
- **Comprehensive**: Covers all Rocketship features and plugins
- **Best Practices**: Emphasizes E2E customer journey testing

## How It Works

1. **Agent asks for guidance** using the MCP tools
2. **MCP server provides examples and patterns** specific to the request
3. **Agent creates the actual test files** based on the guidance
4. **Agent can validate and improve** tests using additional tools

## Requirements

- **Node.js 18+** - For running the MCP server
- **Rocketship CLI** - For validation and execution (when needed)

## Privacy & Security

- **No file generation**: MCP server never creates files in your project
- **Educational only**: Provides guidance and examples
- **Local execution**: All operations run in your environment

## Development

```bash
# Clone and install dependencies
git clone https://github.com/rocketship-ai/rocketship
cd rocketship/mcp-server
npm install

# Build
npm run build

# Test
npm test

# Run in development
npm run dev
```

## License

MIT
