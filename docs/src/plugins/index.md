# Plugins

Rocketship provides a plugin-based architecture for testing different protocols and services. Each plugin is designed to be simple, focused, and composable.

## Available Plugins

### API Testing

- **[HTTP](http.md)** - Test REST APIs with request chaining, assertions, and OpenAPI validation
- **[Supabase](supabase.md)** - Test Supabase database, authentication, and storage operations

### Database Testing

- **[SQL](sql.md)** - Execute queries and validate results across PostgreSQL, MySQL, SQLite, and SQL Server

### Browser Testing

- **[Agent](agent.md)** - AI-powered testing using Claude with MCP servers (recommended)
- **[Playwright](playwright.md)** - Deterministic browser automation with Python scripts
- **[Browser Use](browser-use.md)** - AI-driven browser automation with natural language tasks

### Scripting & Utilities

- **[Script](script.md)** - Execute custom JavaScript or shell scripts for data processing and validation
- **[Log](log.md)** - Output custom messages during test execution
- **[Delay](delay.md)** - Add deterministic pauses between test steps

## Plugin Architecture

All plugins follow a consistent interface:

```yaml
- name: "Step name"
  plugin: plugin_name
  config:
    # Plugin-specific configuration
  assertions:
    # Optional validation
  save:
    # Optional variable extraction
  retry:
    # Optional retry policy
```

## Common Features

All plugins support:

- **Variables**: Use `{{ .env.VAR }}`, `{{ .vars.name }}`, and `{{ runtime_var }}`
- **Assertions**: Validate responses and outputs
- **Save**: Extract values for use in later steps
- **Retry**: Configure automatic retries with backoff

## Plugin Selection Guide

| Use Case | Recommended Plugin | Alternative |
|----------|-------------------|-------------|
| REST API testing | [HTTP](http.md) | - |
| Database CRUD | [SQL](sql.md) | [Supabase](supabase.md) |
| Supabase full-stack | [Supabase](supabase.md) | [SQL](sql.md) + [HTTP](http.md) |
| AI browser testing | [Agent](agent.md) | [Browser Use](browser-use.md) |
| Deterministic browser | [Playwright](playwright.md) | - |
| Data processing | [Script](script.md) | - |
| Debugging/logging | [Log](log.md) | - |
| Timing control | [Delay](delay.md) | Retry policies |

## See Also

- [Features](../features/variables.md) - Variables, lifecycle hooks, retry policies
- [Command Reference](../reference/rocketship.md) - CLI commands
