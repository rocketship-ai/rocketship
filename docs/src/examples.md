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

You have two options for running the SQL examples:

1. **Minikube stack** – run `scripts/install-minikube.sh`, port-forward the engine, then execute `rocketship run -af examples/sql-testing/rocketship.yaml`.
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
