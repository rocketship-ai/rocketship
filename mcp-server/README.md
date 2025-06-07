# Rocketship MCP Server

A Model Context Protocol (MCP) server that enables AI coding agents to generate, maintain, and execute Rocketship tests directly from codebase context.

## Features

- **AI-Powered Test Generation**: Generate complete test suites from natural language prompts
- **Codebase-Aware Scanning**: Analyze existing code to suggest test structures and configurations
- **Environment Variable Detection**: Automatically generate environment-specific configuration files
- **Git Integration**: Analyze code changes to suggest test updates and maintenance
- **CLI Integration**: Execute Rocketship validation and test runs with intelligent reporting

## Quick Start

1. **Install the MCP server**:
   ```bash
   cd mcp-server
   pip install -e .
   ```

2. **Add to your MCP client configuration**:
   ```json
   {
     "mcpServers": {
       "rocketship": {
         "command": "rocketship-mcp",
         "args": []
       }
     }
   }
   ```

3. **Start using with your coding agent**:
   ```
   Generate API tests for my user authentication endpoints
   ```

## Available Tools

### 🎯 Test Generation
- `scan_and_generate_test_suite` - Analyze codebase and generate organized test directory structure
- `generate_test_from_prompt` - Create specific tests from natural language descriptions

### 🔧 Test Management  
- `validate_test_file` - Validate Rocketship YAML files using the CLI
- `run_and_analyze_tests` - Execute tests and provide intelligent failure analysis

### 🔀 Git Integration
- `analyze_git_diff` - Compare branches and suggest test updates for code changes

## Privacy & Security

- **No code transmission**: The MCP server never receives actual source code content
- **Context-only analysis**: Works with agent's existing codebase knowledge
- **Local execution**: All Rocketship CLI operations run locally

## Configuration

The server automatically detects:
- API endpoints and database schemas
- Environment configurations
- Existing test patterns
- Service dependencies

And generates appropriate:
- Test suite directory structures
- Environment variable files (`staging-vars.yaml`, `prod-vars.yaml`)
- Plugin-specific test configurations

## Examples

See the `examples/` directory for sample MCP tool usage and generated test structures.