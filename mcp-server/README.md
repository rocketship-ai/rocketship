# Rocketship MCP Server

A Model Context Protocol (MCP) server that enables AI coding agents to generate, validate, and execute Rocketship tests directly from codebase context.

## Features

- **AI-Powered Test Generation**: Generate complete test suites from natural language prompts
- **Codebase-Aware Scanning**: Analyze existing code to suggest test structures and configurations
- **Environment Variable Detection**: Automatically generate environment-specific configuration files
- **Git Integration**: Analyze code changes to suggest test updates and maintenance
- **CLI Integration**: Execute Rocketship validation and test runs with intelligent reporting

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

### Claude Desktop
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

### Other MCP Clients
```json
{
  "servers": {
    "rocketship": {
      "command": "rocketship-mcp"
    }
  }
}
```

## Available Tools

- `scan_and_generate_test_suite` - Analyze codebase and generate organized test directory structure
- `generate_test_from_prompt` - Create specific tests from natural language descriptions
- `validate_test_file` - Validate Rocketship YAML files using the CLI
- `run_and_analyze_tests` - Execute tests and provide intelligent failure analysis
- `analyze_git_diff` - Compare branches and suggest test updates for code changes

## Usage Examples

### Generate Complete Test Suite

```
Generate comprehensive tests for my Node.js API with PostgreSQL database
```

### Create Specific Tests

```
Create a test that validates user authentication with error cases for invalid passwords
```

### Validate and Run Tests

```
Validate my Rocketship test file and run it against staging environment
```

### Git-Based Maintenance

```
Analyze my feature branch changes and suggest what tests need updating
```

## Requirements

- **Rocketship CLI** - Must be installed and accessible in PATH
- **Node.js 18+** - For running the MCP server
- **Git** - Required for git diff analysis features

## Privacy & Security

- **No code transmission**: The MCP server never receives actual source code content
- **Context-only analysis**: Works with agent's existing codebase knowledge  
- **Local execution**: All Rocketship CLI operations run locally

## Development

```bash
# Clone and install dependencies
git clone https://github.com/rocketship-ai/rocketship
cd rocketship/mcp-server
npm install

# Build
npm run build

# Run in development
npm run dev
```

## License

MIT