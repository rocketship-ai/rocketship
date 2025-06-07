# Rocketship MCP Server Installation Guide

## Prerequisites

- **Python 3.8+** - The MCP server is built with Python
- **Rocketship CLI** - Must be installed and accessible in PATH
- **Git** - Required for git diff analysis features

## Installation

### 1. Install the MCP Server

```bash
cd mcp-server
pip install -e .
```

### 2. Verify Installation

```bash
# Test the installation
python test_server.py

# Verify CLI access
rocketship-mcp --help
```

### 3. Configure Your MCP Client

Add the Rocketship MCP server to your MCP client configuration:

#### Claude Desktop
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

#### Cursor/VSCode with MCP Extension
```json
{
  "mcp.servers": {
    "rocketship": {
      "command": "rocketship-mcp"
    }
  }
}
```

### 4. Test the Connection

Start your MCP client and try these commands with your coding agent:

```
Generate a comprehensive test suite for my API endpoints
```

```
Create a test that validates user authentication with error cases
```

```
Analyze my git changes and suggest test updates
```

## Development Setup

### Install Development Dependencies

```bash
cd mcp-server
pip install -e ".[dev]"
```

### Run Tests

```bash
# Run basic functionality tests
python test_server.py

# Run with pytest (if available)
pytest tests/
```

### Code Formatting

```bash
# Format code
black src/
isort src/

# Type checking
mypy src/
```

## Configuration

### Environment Variables

The MCP server respects these environment variables:

```bash
# Optional: Custom Rocketship CLI path
export ROCKETSHIP_CLI_PATH="/path/to/rocketship"

# Optional: Default project root
export ROCKETSHIP_PROJECT_ROOT="/path/to/your/project"

# Optional: Default environments
export ROCKETSHIP_DEFAULT_ENVIRONMENTS="staging,prod,dev"
```

### Rocketship CLI Requirements

Ensure the Rocketship CLI is properly installed:

```bash
# Install Rocketship CLI (if not already installed)
# Follow instructions at: https://docs.rocketship.sh/installation/

# Verify installation
rocketship version
rocketship validate --help
```

## Troubleshooting

### Common Issues

1. **"rocketship-mcp command not found"**
   ```bash
   # Reinstall with pip
   pip install -e .
   
   # Check installation
   which rocketship-mcp
   ```

2. **"rocketship command not found" during execution**
   ```bash
   # Install Rocketship CLI
   # See: https://docs.rocketship.sh/installation/
   
   # Or set custom path
   export ROCKETSHIP_CLI_PATH="/path/to/rocketship"
   ```

3. **Import errors**
   ```bash
   # Check Python version
   python --version  # Should be 3.8+
   
   # Reinstall dependencies
   pip install -r requirements.txt
   ```

4. **MCP client connection issues**
   - Verify MCP client configuration JSON syntax
   - Check that the `rocketship-mcp` command is in PATH
   - Restart your MCP client after configuration changes

### Debug Mode

Run the server with debug logging:

```bash
# Set debug environment
export ROCKETSHIP_MCP_DEBUG=1
rocketship-mcp
```

### Logs

Check MCP server logs in your client's output or console for detailed error messages.

## Next Steps

Once installed, check out:

- [Usage Examples](examples/mcp_usage_examples.md) - Practical examples of using the MCP server
- [Generated Test Structure](examples/generated_test_structure/) - Example generated test files
- [Rocketship Documentation](https://docs.rocketship.sh/) - Learn more about Rocketship testing

## Support

If you encounter issues:

1. Check the [troubleshooting section](#troubleshooting) above
2. Review the [examples](examples/) directory
3. File an issue in the Rocketship repository