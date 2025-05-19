# Examples

Rocketship comes with example test suites that demonstrate different features and use cases. Each example includes detailed explanations and ready-to-use test specifications.

## Available Examples

### HTTP Testing

- [Request Chaining & Delays](examples/request-chaining.md) - Learn how to chain HTTP requests, handle responses, and use delays in your test suites

## Running the Examples

To run any example, first start the test server (included in the repository):

```bash
# Start the test server
go run for-contributors/test-server/main.go
```

Then in another terminal, run the example:

```bash
# Run the example test suite
rocketship run -af examples/request-chaining/rocketship.yaml
```

The test server provides a simple HTTP API that you can use to experiment with Rocketship's features. It includes:

- Full CRUD operations for any resource type
- JSON request/response handling
- Automatic ID generation
- In-memory storage for the session

You can find the test server's source code in the `for-contributors/test-server` directory.
