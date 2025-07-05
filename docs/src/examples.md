# Examples

Rocketship comes with example test suites that demonstrate different features and use cases. Each example includes detailed explanations and ready-to-use test specifications.

## Available Examples

### HTTP Testing

- [Request Chaining & Delays](examples/request-chaining.md) - Learn how to chain HTTP requests, handle responses, and use delays in your test suites

### Variables

- [Variables](examples/variables.md) - Learn how to parameterize tests with configuration variables, CLI overrides, and variable files

### Database Testing

- [SQL Testing](examples/sql-testing.md) - Learn how to test database operations with PostgreSQL, MySQL, SQLite, and SQL Server

### Scripting & Custom Logic

- [Custom Scripting](examples/custom-scripting.md) - Learn how to add custom JavaScript logic to your test suites

### Debugging & Logging

- [Log Plugin](examples/log-plugin.md) - Learn how to add custom logging messages to your test suites for debugging and monitoring

## Running the Examples

The examples use the hosted test server at `tryme.rocketship.sh`. This server provides a simple HTTP API that you can use to experiment with Rocketship's features. Details:

- Test CRUD operations for a resource type
- Resources are isolated based off a session header
- FYI: Resource cleanup is done hourly (every :00)

Then, run an example:

```bash
# Run the request chaining example
rocketship run -af examples/request-chaining/rocketship.yaml

# Run the configuration variables example
rocketship run -af examples/config-variables/rocketship.yaml

# Run with variable overrides
rocketship run -af examples/config-variables/rocketship.yaml --var environment=production

# Run the custom scripting example
rocketship run -af examples/custom-scripting/rocketship.yaml

# Run the log plugin example
rocketship run -af examples/simple-log/rocketship.yaml
```

### Database Examples

For SQL testing examples, you'll need to start the test databases first:

```bash
# Start test databases with Docker Compose
cd .docker && docker-compose up postgres-test mysql-test -d

# Wait for databases to be ready, then run SQL tests
rocketship run -f examples/sql-testing/rocketship.yaml -e localhost:7700
```

The SQL examples use local test databases with pre-populated sample data:

- **PostgreSQL**: Available at `localhost:5433`
- **MySQL**: Available at `localhost:3307`

You can find the test server's source code in the `for-contributors/test-server` directory.
