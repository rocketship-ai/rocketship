# Rocketship MCP Server

A Model Context Protocol (MCP) server that assists AI coding agents in writing better Rocketship tests by providing examples, patterns, and guidance rather than generating complete test files.

## Philosophy

This MCP server is designed as a **knowledgeable assistant** that helps coding agents understand Rocketship testing patterns and best practices. The agent maintains full control over file creation while receiving expert guidance.

## Features

- **Pattern Library**: Comprehensive examples for API testing, step chaining, assertions, and customer journeys
- **Test Structure Guidance**: Suggests templates with TODOs for agents to implement
- **Plugin Configuration Help**: Shows configuration examples for all Rocketship plugins
- **Assertion Patterns**: Demonstrates validation techniques for different response types
- **CLI Command Reference**: Provides usage examples and best practices
- **YAML Validation**: Reviews test files and suggests improvements

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
Get examples and best practices for specific Rocketship features.

**Features:** `api_testing`, `step_chaining`, `assertions`, `plugins`, `environments`, `customer_journeys`

### 2. `suggest_test_structure`
Returns a test template with TODOs for you to implement.

**Types:** `api`, `browser`, `sql`, `integration`, `e2e`

### 3. `get_assertion_patterns`
Shows assertion examples for different testing scenarios.

**Response Types:** `json`, `xml`, `text`, `status`, `headers`, `sql`, `browser`

### 4. `get_plugin_config`
Provides configuration examples for Rocketship plugins.

**Plugins:** `http`, `sql`, `browser`, `agent`, `supabase`, `delay`, `script`, `log`

### 5. `validate_and_suggest`
Reviews your Rocketship YAML and suggests improvements.

**Focus Areas:** `performance`, `assertions`, `structure`, `coverage`, `best_practices`

### 6. `get_cli_commands`
Provides CLI command examples and usage patterns.

**Commands:** `run`, `validate`, `start`, `stop`, `general`

## Usage Examples

### Learning API Testing Patterns

```
"Show me examples of API testing with authentication and response validation"

Agent asks for: get_rocketship_examples(feature="api_testing")
You get: Examples, best practices, and implementation guidance
You create: Your own test file based on the patterns
```

### Creating Test Structure

```
"I need to create an E2E test for user registration flow"

Agent asks for: suggest_test_structure(test_name="User Registration", test_type="e2e", customer_journey="User signs up and verifies email")
You get: Template with TODOs and implementation checklist
You create: Complete test by filling in the TODOs
```

### Understanding Assertions

```
"What assertions should I use for JSON API responses?"

Agent asks for: get_assertion_patterns(response_type="json", test_scenario="User profile API")
You get: Comprehensive assertion examples and tips
You implement: Appropriate assertions in your test
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