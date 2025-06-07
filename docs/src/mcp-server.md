# MCP Server

Rocketship includes an MCP (Model Context Protocol) server that enables AI coding assistants to automatically generate, run, and maintain your test suites. This integration allows tools like Claude, Cursor, Windsurf, and other MCP-compatible assistants to understand your codebase and create comprehensive test scenarios.

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

The MCP server provides five powerful tools for test automation:

### 1. scan_and_generate_test_suite

Analyzes your codebase context and generates a comprehensive test suite structure.

**Capabilities:**

- Creates organized test directories (api-tests, database-tests, integration-tests)
- Generates environment-specific configuration files
- Produces ready-to-run Rocketship YAML test files
- Automatically configures authentication and variables

**Example Usage:**

```
"Analyze my codebase and create a complete test suite for staging and production environments"
```

### 2. generate_test_from_prompt

Creates test files from natural language descriptions.

**Capabilities:**

- Converts plain English into valid Rocketship YAML
- Automatically selects appropriate plugins (HTTP, SQL, Supabase)
- Includes relevant assertions and error handling
- Generates proper variable references

**Example Usage:**

```
"Create a test that verifies user login, checks their profile, and validates their subscription status"
```

### 3. validate_test_file

Validates Rocketship test files for syntax and schema compliance.

**Capabilities:**

- Checks YAML syntax
- Validates against Rocketship schema
- Reports specific errors with line numbers
- Ensures all required fields are present

**Example Usage:**

```
"Validate all test files in the .rocketship directory"
```

### 4. run_and_analyze_tests

Executes tests and provides intelligent failure analysis.

**Capabilities:**

- Runs tests with specified environment variables
- Analyzes failure patterns
- Suggests fixes for common issues
- Provides detailed error context

**Example Usage:**

```
"Run my API tests against staging and explain any failures"
```

### 5. analyze_git_diff

Examines code changes and suggests test updates.

**Capabilities:**

- Compares branches to identify changes
- Suggests new tests for added endpoints
- Recommends updates for modified APIs
- Highlights tests that may need attention

**Example Usage:**

```
"What tests should I update based on my latest PR?"
```

## Integration Examples

### With Claude Code

1. Add the configuration to your `.mcp.json` file in your project:

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

2. Ask Claude to help with your tests!

```
"Generate a comprehensive rocketship test suite for all of my Express.js API endpoints?"
```

### With Cursor

1. Copy and paste either the Rocketship Cursor Web Link or Deep Link into your browser:

```
https://cursor.com/install-mcp?name=rocketship&config=eyJjb21tYW5kIjoibnB4IC15IEByb2NrZXRzaGlwYWkvbWNwLXNlcnZlckBsYXRlc3QifQ%3D%3D
```

```
cursor://anysphere.cursor-deeplink/mcp/install?name=rocketship&config=eyJjb21tYW5kIjoibnB4IC15IEByb2NrZXRzaGlwYWkvbWNwLXNlcnZlckBsYXRlc3QifQ==
```

2. Ask Cursor to create generate a test based off your prompt!

```
"Can you write a rocketship test for my FastAPI API that includes user authentication?"
```

### With Windsurf

1. Add the configuration to your Windsurf cascade configuration `~/.codeium/windsurf/mcp_config.json`:

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

2. Ask Windsurf to write a new test based off your branch!

```
"Can you write a new rocketship test for this feature branch?"
```

### With Other MCP Clients

Any MCP-compatible client can use the Rocketship server. The server communicates via JSON-RPC over stdio, making it compatible with various AI assistants and development tools.

## Best Practices

### 1. Provide Context

When asking for test generation, provide context about:

- Your API structure
- Authentication methods
- Database schemas
- External services you integrate with

### 2. Review Generated Tests

While the MCP server generates high-quality tests, always review:

- Environment variables and secrets
- API endpoints and URLs
- Expected response values
- Test data assumptions

### 3. Iterate and Refine

Use the MCP server iteratively:

1. Generate initial tests
2. Run and analyze failures
3. Ask for specific improvements
4. Update based on code changes

## Example Workflow

Here's a complete workflow using the MCP server:

```
User: "I have a new Node.js API with user management and payment processing.
      Generate a complete test suite."

AI: *Uses scan_and_generate_test_suite to create:*
    - .rocketship/api-tests/rocketship.yaml
    - .rocketship/integration-tests/rocketship.yaml
    - .rocketship/staging-vars.yaml
    - .rocketship/prod-vars.yaml

User: "Now run the tests against my staging environment"

AI: *Uses run_and_analyze_tests*
    "3 tests failed due to missing API_KEY environment variable.
     Please set your Stripe API key in staging-vars.yaml"

User: "I've updated the keys. Also, I just added a new webhook endpoint
      for payment confirmations"

AI: *Uses analyze_git_diff and generate_test_from_prompt*
    "I've detected your new /webhooks/stripe endpoint and generated
     tests for signature validation and payment confirmation flows"
```

## Environment Variables

The MCP server respects these environment variables:

- `ROCKETSHIP_LOG`: Set log level (DEBUG, INFO, ERROR)
- `ROCKETSHIP_ENGINE_ADDR`: Override default engine address

## Troubleshooting

### MCP Server Not Found

If Claude can't find the MCP server:

1. Ensure you have Node.js 18+ installed
2. Check your claude_desktop_config.json syntax
3. Restart Claude after configuration changes

### Test Generation Issues

If generated tests aren't working:

1. Provide more context about your API structure
2. Specify authentication methods explicitly
3. Include example requests/responses

### Validation Failures

Common validation issues:

- Missing `version` field (required, format: "v1.0.0")
- Invalid plugin names
- Malformed assertions

## Security Considerations

The MCP server:

- Never stores credentials or sensitive data
- Operates in read-only mode for analysis
- Executes tests only when explicitly requested
- Uses your local Rocketship installation

## Future Enhancements

Planned improvements include:

- Visual test builder interface
- Test coverage analysis
- Performance benchmarking
- Automatic test maintenance based on API changes
