# MCP Server

Rocketship includes an MCP (Model Context Protocol) server that assists AI coding agents in writing better Rocketship tests by providing examples, patterns, and guidance rather than generating complete test files.

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

Get examples and best practices for specific Rocketship features.

**Features:**
- `api_testing` - HTTP endpoint testing with authentication and validation
- `step_chaining` - Using data from previous steps in workflows
- `assertions` - Comprehensive validation patterns
- `plugins` - Configuration examples for all plugins
- `environments` - Multi-stage configuration patterns
- `customer_journeys` - E2E workflow testing examples

**Example Request:**
```
"Show me API testing examples with authentication"
```

**What You Get:**
- Real YAML code examples
- Best practices for the feature
- Implementation guidance
- Next steps checklist

### 2. suggest_test_structure

Returns a test template with TODOs and implementation guidance.

**Test Types:**
- `api` - HTTP endpoint testing
- `browser` - UI automation testing
- `sql` - Database testing
- `integration` - Multi-service testing
- `e2e` - End-to-end customer journeys

**Example Request:**
```
"I need to test user registration and email verification flow"
```

**What You Get:**
- YAML template with TODO sections
- Implementation checklist
- Suggested structure for your specific use case

### 3. get_assertion_patterns

Shows assertion examples for different testing scenarios.

**Response Types:**
- `json` - JSON API response validation
- `xml` - XML response validation
- `text` - Plain text response validation
- `status` - HTTP status code patterns
- `headers` - HTTP header validation
- `sql` - Database result validation
- `browser` - UI element validation

**Example Request:**
```
"What assertions should I use for user profile API responses?"
```

**What You Get:**
- Comprehensive assertion examples
- JSONPath and XPath patterns
- Validation tips and best practices

### 4. get_plugin_config

Provides configuration examples for Rocketship plugins.

**Available Plugins:**
- `http` - API testing with retry logic and authentication
- `sql` - Database operations with transaction support
- `browser` - UI automation with screenshots and interactions
- `agent` - AI-powered validation and analysis
- `supabase` - Direct Supabase API operations
- `delay` - Timing control with jitter
- `script` - Custom JavaScript logic
- `log` - Structured logging and debugging

**Example Request:**
```
"How do I configure the SQL plugin for PostgreSQL testing?"
```

**What You Get:**
- Basic and advanced configuration examples
- Feature descriptions and capabilities
- Plugin-specific tips and best practices

### 5. validate_and_suggest

Reviews your Rocketship YAML content and suggests improvements.

**Improvement Focus Areas:**
- `performance` - Timeout and retry optimizations
- `assertions` - Better validation patterns
- `structure` - YAML organization improvements
- `coverage` - Test scenario completeness
- `best_practices` - General Rocketship recommendations

**Example Request:**
```
"Review my test file and suggest improvements"
```

**What You Get:**
- Issue identification and fixes
- Specific improvement suggestions
- Best practice recommendations
- Next steps for enhancement

### 6. get_cli_commands

Provides CLI command examples and usage patterns.

**Command Categories:**
- `run` - Execute tests with various options
- `validate` - Syntax and schema validation
- `start` - Start Rocketship engine server
- `stop` - Stop engine server
- `general` - Help, version, and configuration

**Example Request:**
```
"How do I run tests with custom variables in CI/CD?"
```

**What You Get:**
- Command examples for different scenarios
- Flag explanations and usage
- Workflow patterns and best practices

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
"I need to create API tests for my Express.js authentication endpoints. Show me some patterns I can follow."
```

Claude will use the MCP server to get relevant examples and help you create your own test files.

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
"What's the best way to structure a Rocketship test for user login with database validation?"
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
"Help me understand how to use step chaining in Rocketship for a complete e-commerce checkout flow"
```

### With Other MCP Clients

Any MCP-compatible client can use the Rocketship server. The server communicates via JSON-RPC over stdio, making it compatible with various AI assistants and development tools.

## Best Practices

### 1. Ask for Specific Guidance

Instead of asking for complete test generation, ask for guidance on specific aspects:

**Good:**
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
User: "I need to test a user registration API that creates a user, 
       sends an email, and requires email verification"

Agent: *Uses get_rocketship_examples with feature="customer_journeys"*

MCP Server: *Returns E2E customer journey examples including:*
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