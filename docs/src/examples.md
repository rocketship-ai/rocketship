# Examples

Rocketship comes with example test suites that demonstrate different features and use cases. Each example includes detailed explanations and ready-to-use test specifications.

## Available Examples

### HTTP Testing

- [Request Chaining & Delays](examples/request-chaining.md) - Learn how to chain HTTP requests, handle responses, and use delays in your test suites

### Configuration & Variables

- [Configuration Variables](examples/config-variables.md) - Learn how to parameterize tests with configuration variables, CLI overrides, and variable files

## Running the Examples

The examples use the hosted test server at `tryme.rocketship.sh`. This server provides a simple HTTP API that you can use to experiment with Rocketship's features. Details:

- Test CRUD operations for a resource type
- Resources are isolated based off your IP
- FYI: Resource cleanup is done hourly (every :00)

Then, run an example:

```bash
# Run the request chaining example
rocketship run -af examples/request-chaining/rocketship.yaml

# Run the configuration variables example
rocketship run -af examples/config-variables/rocketship.yaml

# Run with variable overrides
rocketship run -af examples/config-variables/rocketship.yaml --var environment=production
```

You can find the test server's source code in the `for-contributors/test-server` directory.
