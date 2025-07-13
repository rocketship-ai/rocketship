# MCP Server

Rocketship includes an MCP (Model Context Protocol) server that enables AI coding agents to write better Rocketship tests by providing examples, patterns, and guidance.

## Philosophy

The Rocketship MCP server is designed as a **knowledgeable assistant** that helps coding agents understand Rocketship testing patterns and best practices. Unlike traditional code generators, this MCP server:

- **Provides guidance, not files**: Shows examples and patterns for agents to adapt
- **Maintains agent control**: The coding agent creates all files and makes all decisions
- **Emphasizes learning**: Helps agents understand Rocketship concepts deeply
- **Focuses on quality**: Promotes E2E customer journey testing and best practices

## Installation

The MCP server is published as an npm package and can be used with zero installation:

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

The MCP server provides six assistant tools that provide guidance rather than generating files:

### 1. get_rocketship_examples

Get real examples and best practices for specific Rocketship features from the current codebase.

**Available Plugins:**

- `http` - API endpoint testing with authentication and validation
- `delay` - Timing control and wait operations
- `script` - Custom JavaScript logic execution
- `sql` - Database operations and validation
- `log` - Structured logging and debugging
- `agent` - AI-powered validation and analysis
- `browser` - UI automation and testing
- `supabase` - Direct Supabase API operations

**Example Request:**

```
"Show me examples of HTTP testing with step chaining"
```

**What You Get:**

- Real YAML code examples from the codebase
- Best practices for the specific plugin
- Implementation guidance and patterns
- Variable usage examples

### 2. suggest_test_structure

Suggests proper file structure and test organization based on current project configuration.

**Project Types:**

- `frontend` - Browser-based testing with user journeys
- `backend` - API endpoint testing
- `fullstack` - Combined frontend and backend testing
- `api` - Pure API testing focus
- `mobile` - Mobile application testing

**Example Request:**

```
"I need a test structure for an e-commerce frontend project"
```

**What You Get:**

- Recommended directory structure
- File organization patterns
- Plugin recommendations for your project type
- Test flow suggestions

### 3. get_schema_info

Provides current schema information for validation and proper syntax.

**Schema Sections:**

- `plugins` - Available plugins and their configurations
- `assertions` - Validation patterns and types
- `save` - Data extraction and variable storage
- `structure` - Overall YAML test structure
- `full` - Complete schema documentation

**Example Request:**

```
"Show me the schema for assertions and save operations"
```

**What You Get:**

- Current schema validation rules
- Required and optional fields
- Examples of proper syntax
- Compatibility information

### 4. get_cli_guidance

Provides current CLI usage patterns and commands from introspection.

**Command Types:**

- `run` - Execute tests with various options
- `validate` - Syntax and schema validation
- `structure` - File structure and organization guidance

**Example Request:**

```
"How do I run tests with custom variables?"
```

**What You Get:**

- Current CLI command examples
- Flag explanations and usage
- Common usage patterns
- Version-specific information

### 5. get_rocketship_cli_installation_instructions

Get step-by-step instructions for installing the Rocketship CLI on different platforms.

**Platform Support:**

- `auto` - Auto-detect platform (default)
- `macos-arm64` - macOS with Apple Silicon
- `macos-intel` - macOS with Intel processors
- `linux` - Linux distributions
- `windows` - Windows systems

**Example Request:**

```
"How do I install Rocketship on macOS?"
```

**What You Get:**

- Platform-specific installation commands
- Available vs NOT available installation methods
- Post-installation verification steps
- Troubleshooting guidance
- Prerequisites and dependencies

### 6. analyze_codebase_for_testing

Analyzes a codebase to suggest meaningful test scenarios based on available plugins.

**Focus Areas:**

- `user_journeys` - End-to-end customer workflows
- `api_endpoints` - API testing strategies
- `critical_paths` - Business-critical functionality
- `integration_points` - Service integration testing

**Example Request:**

```
"Analyze my React e-commerce app for testing opportunities"
```

**What You Get:**

- Suggested test scenarios for your codebase
- Plugin recommendations based on project type
- Critical flow identification
- Testing strategy recommendations

## Integration Examples

### With Claude Code

Add to your `.mcp.json` file in your project root:

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

Then ask Claude for help:

```
"I need to install Rocketship and create API tests for my Express.js authentication endpoints."
```

Claude will use the MCP server to provide installation instructions and relevant testing examples to help you get started.

### With Cursor

1. Go to Cursor Settings > Features > Enable Model Context Protocol
2. Add to your MCP configuration:

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

3. Ask Cursor for guidance:

```
"How do I install Rocketship and structure a test for user login with database validation?"
```

### With Windsurf

Add to your Windsurf MCP configuration (`~/.codeium/windsurf/mcp_config.json`):

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

Then ask for assistance:

```
"Help me install Rocketship and understand how to use step chaining for a complete e-commerce checkout flow"
```

### With Other MCP Clients

Any MCP-compatible client can use the Rocketship server. The server communicates via JSON-RPC over stdio, making it compatible with various AI assistants and development tools.

## Best Practices

### 1. Ask for Specific Guidance

Instead of asking for complete test generation, ask for guidance on specific aspects:

**Good:**

- "How do I install Rocketship on my platform?"
- "Show me examples of API authentication testing"
- "What assertions work best for user profile endpoints?"
- "How should I structure an E2E checkout flow test?"

**Less Effective:**

- "Generate all my tests"
- "Create a complete test suite"

### 2. Learn the Patterns

Use the MCP server to understand Rocketship concepts:

- Study the examples provided
- Understand the reasoning behind best practices
- Adapt patterns to your specific use case
- Build your own expertise over time

### 3. Focus on Customer Journeys

The MCP server emphasizes E2E customer journey testing:

- Think about complete user workflows
- Test realistic user scenarios
- Validate data consistency across steps
- Include error and edge cases

### 4. Iterate and Improve

Use the validation tool to continuously improve:

1. Get initial structure guidance
2. Create your test implementation
3. Validate and get improvement suggestions
4. Refine based on feedback
5. Learn from the process

## Example Workflow

Here's how a typical interaction works:

```
User: "I need to install Rocketship and test a user registration API 
       that creates a user, sends an email, and requires email verification"

Agent: *Uses get_rocketship_cli_installation_instructions*

MCP Server: *Returns platform-specific installation instructions with:*
- Installation commands for the user's platform
- Post-installation verification steps
- Troubleshooting guidance

Agent: "First, let's get Rocketship installed..."
       *Provides installation guidance*

User: "Great! Now I need help with the test structure"

Agent: *Uses get_rocketship_examples with feature_type="http"*

MCP Server: *Returns HTTP testing examples including:*
- Multi-step workflow patterns
- Email verification testing approaches
- Data validation between steps
- Best practices for user onboarding flows

Agent: "Based on these examples, let me help you create a test..."
       *Creates test file incorporating the patterns*

User: "Now I want to add database validation to ensure the user was created correctly"

Agent: *Uses get_plugin_config with plugin="sql"*

MCP Server: *Returns SQL plugin configuration examples*

Agent: *Helps add SQL validation step to the existing test*

User: "Can you review my test and suggest improvements?"

Agent: *Uses validate_and_suggest with the YAML content*

MCP Server: *Returns specific suggestions for improvement*

Agent: *Helps implement the suggested improvements*
```

## Environment Variables

The MCP server respects these environment variables:

- `ROCKETSHIP_LOG`: Set log level (DEBUG, INFO, ERROR)
- `NODE_ENV`: Development/production mode

## Troubleshooting

### MCP Server Not Found

If your AI client can't find the MCP server:

1. Ensure Node.js 18+ is installed
2. Check your MCP configuration syntax
3. Restart your AI client after configuration changes
4. Verify the npm package is accessible

### No Guidance Provided

If the MCP server isn't providing helpful guidance:

1. Be more specific in your requests
2. Provide context about what you're trying to test
3. Ask for specific features or patterns
4. Try different tool combinations

### Examples Don't Match Your Use Case

If the provided examples don't fit your scenario:

1. Ask for multiple feature examples to combine
2. Request specific plugin configurations
3. Use the validation tool to refine your approach
4. Adapt the patterns to your specific needs

## Security Considerations

The MCP server:

- **Never generates files**: Only provides guidance and examples
- **No code access**: Works with agent's existing knowledge
- **Read-only operation**: Cannot modify your project
- **Local execution**: All operations run in your environment
- **No data storage**: Doesn't store or transmit your code

## Future Enhancements

Planned improvements include:

- **Enhanced pattern library**: More examples for complex scenarios
- **Interactive tutorials**: Step-by-step guidance for common workflows
- **Context-aware suggestions**: Better understanding of project structure
- **Performance optimization**: Faster response times and better caching
