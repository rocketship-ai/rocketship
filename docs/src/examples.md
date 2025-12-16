# Examples

Rocketship comes with example test suites that demonstrate different features and use cases. Each example includes detailed explanations and ready-to-use test specifications.

## Available Examples

### HTTP Testing

- [Simple HTTP](https://github.com/rocketship-ai/rocketship/blob/main/examples/simple-http/rocketship.yaml) - Basic HTTP operations (GET, POST, DELETE) with assertions
- [Request Chaining](examples/http/request-chaining.md) - Chain HTTP requests and pass data between steps
- [Form Data](https://github.com/rocketship-ai/rocketship/blob/main/examples/http-form-encoded/rocketship.yaml) - Submit URL-encoded form data
- [OpenAPI Validation](examples/http/openapi-validation.md) - Validate API responses against OpenAPI schemas
- [Complex HTTP](https://github.com/rocketship-ai/rocketship/blob/main/examples/complex-http/rocketship.yaml) - Advanced HTTP workflows with multiple resources

### Browser Testing

- [AI Browser Testing](examples/ai/browser-testing.md) - Use AI-powered testing with the Agent plugin
- [Agent Testing](examples/ai/agent-testing.md) - Advanced AI agent workflows with MCP servers
- [Browser Basics](https://github.com/rocketship-ai/rocketship/blob/main/examples/browser/rocketship.yaml) - Combining Playwright and browser_use plugins
- [Persistent Sessions](plugins/browser/persistent-sessions.md) - Maintain browser state across steps

### Database Testing

- [SQL Testing](examples/sql-testing.md) - Test database operations with PostgreSQL, MySQL, SQLite, and SQL Server
- [Supabase Testing](examples/supabase-testing.md) - Full-stack Supabase testing (database, auth, storage)

### Variables & Configuration

- [Variables Overview](examples/variables/variables.md) - Comparison of environment, config, and runtime variables
- [Environment Variables](examples/variables/environment-variables.md) - Secrets and environment-specific configuration
- [Config Variables](examples/variables/config-variables.md) - Test parameters and runtime values

### Scripting & Utilities

- [Custom Scripting](examples/scripting/custom-scripting.md) - Add custom JavaScript or shell scripts
- [Shell Scripting](examples/scripting/shell-scripting.md) - Execute shell commands in tests
- [Shell Testing](https://github.com/rocketship-ai/rocketship/blob/main/examples/shell-testing/rocketship.yaml) - Comprehensive shell script testing examples

### Reliability & Control Flow

- [Retry Policies](examples/reliability/retry-policies.md) - Configure automatic retries with backoff
- [Retry Policy Example](https://github.com/rocketship-ai/rocketship/blob/main/examples/retry-policy/rocketship.yaml) - Working example demonstrating retry configuration
- [Delays](examples/reliability/delays.md) - Add deterministic pauses between steps
- [Simple Delay](https://github.com/rocketship-ai/rocketship/blob/main/examples/simple-delay/rocketship.yaml) - Basic delay plugin usage
- [Lifecycle Hooks](https://github.com/rocketship-ai/rocketship/tree/main/examples/lifecycle-hooks/) - Setup and teardown with suite and test-level hooks
  - [Suite-Level Hooks](https://github.com/rocketship-ai/rocketship/blob/main/examples/lifecycle-hooks/suite-level-hooks/rocketship.yaml) - Shared setup/cleanup for all tests
  - [Test-Level Hooks](https://github.com/rocketship-ai/rocketship/blob/main/examples/lifecycle-hooks/test-level-hooks/rocketship.yaml) - Setup/cleanup per test
  - [Combined Hooks](https://github.com/rocketship-ai/rocketship/blob/main/examples/lifecycle-hooks/combined-hooks/rocketship.yaml) - Mix suite and test-level hooks

### Debugging & Logging

- [Log Plugin](examples/logging/log-plugin.md) - Add custom logging messages for debugging
- [Simple Log](https://github.com/rocketship-ai/rocketship/blob/main/examples/simple-log/rocketship.yaml) - Basic logging examples

## Running the Examples

The examples use the hosted test server at `tryme.rocketship.sh`. This server provides a simple HTTP API that you can use to experiment with Rocketship's features. Details:

- Test CRUD operations for a resource type
- Resources are isolated based off a session header
- FYI: Resource cleanup is done hourly (every :00)

Then, run an example:

```bash
# Basic HTTP example
rocketship run -af examples/simple-http/rocketship.yaml

# Request chaining
rocketship run -af examples/request-chaining/rocketship.yaml

# Configuration variables
rocketship run -af examples/config-variables/rocketship.yaml --var environment=production

# Custom scripting
rocketship run -af examples/custom-scripting/rocketship.yaml

# Log plugin
rocketship run -af examples/simple-log/rocketship.yaml

# Delay plugin
rocketship run -af examples/simple-delay/rocketship.yaml

# Retry policies
rocketship run -af examples/retry-policy/rocketship.yaml

# Shell scripting
rocketship run -af examples/shell-testing/rocketship.yaml

# Lifecycle hooks
rocketship run -af examples/lifecycle-hooks/suite-level-hooks/rocketship.yaml
```

### Database Examples

You have two options for running the SQL examples:

1. **Minikube stack** – run `scripts/setup-local-dev.sh` (one-time setup), then `scripts/start-dev.sh`, port-forward the engine, then execute `rocketship run -af examples/sql-testing/rocketship.yaml`.
2. **Standalone Docker containers** – start databases locally:
   ```bash
   docker run --rm -d --name rocketship-postgres      -e POSTGRES_PASSWORD=testpass      -e POSTGRES_DB=testdb      -p 5433:5432      postgres:13

   docker run --rm -d --name rocketship-mysql      -e MYSQL_ROOT_PASSWORD=testpass      -e MYSQL_DATABASE=testdb      -p 3306:3306      mysql:8.0
   ```
   Then configure the DSNs in `examples/sql-testing/rocketship.yaml` to point to the exposed ports. Stop the containers when you're done:
   ```bash
   docker stop rocketship-postgres rocketship-mysql
   ```

You can find the test server's source code in the `for-contributors/test-server` directory.
