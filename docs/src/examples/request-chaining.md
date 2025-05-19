# Request Chaining & Delays

This example demonstrates how to chain HTTP requests and use delays in your test suites. It uses our hosted test server at `tryme.rocketship.sh` to show real-world API testing scenarios.

## Test Specification

```yaml
name: "Request Chaining & Delays Example"
description: "A test suite demonstrating request chaining and delays with the test server"
version: "v1.0.0"
tests:
  - name: "User Management Flow"
    steps:
      - name: "Create first user"
        plugin: "http"
        config:
          method: "POST"
          url: "https://tryme.rocketship.sh/users"
          body: |
            {
              "name": "John Doe",
              "email": "john@example.com",
              "role": "admin"
            }
        assertions:
          - type: "status_code"
            expected: 200
          - type: "json_path"
            path: "$.name"
            expected: "John Doe"
        save:
          - json_path: "$.id"
            as: "first_user_id"
          - json_path: "$.email"
            as: "first_user_email"

      - name: "Wait for system processing"
        plugin: "delay"
        config:
          duration: "1s"

      - name: "Create second user"
        plugin: "http"
        config:
          method: "POST"
          url: "https://tryme.rocketship.sh/users"
          body: |
            {
              "name": "Jane Smith",
              "email": "jane@example.com",
              "role": "user"
            }
        assertions:
          - type: "status_code"
            expected: 200
        save:
          - json_path: "$.id"
            as: "second_user_id"

      - name: "Short delay for consistency"
        plugin: "delay"
        config:
          duration: "500ms"

      - name: "List all users"
        plugin: "http"
        config:
          method: "GET"
          url: "https://tryme.rocketship.sh/users"
        assertions:
          - type: "status_code"
            expected: 200
          - type: "json_path"
            path: "$.users_0.name"
            expected: "John Doe"
          - type: "json_path"
            path: "$.users_1.name"
            expected: "Jane Smith"
```

## Key Features Demonstrated

**Request Chaining**:

1. Creating multiple users
2. Saving response values for later use
3. Using saved values in subsequent requests
4. Verifying changes across requests

**Delays**:

1. Using delays between operations
2. Different delay durations (1s, 500ms)
3. Strategic placement for system consistency

**Assertions**:

1. Status code validation
2. JSON response validation using JSONPath
3. Response content validation

## Running the Example

Run the test suite:

```bash
rocketship run -af examples/request-chaining/rocketship.yaml
```

## Understanding the Flow

The example demonstrates a complete user management workflow:

1. Create first user and save their ID and email
2. Wait for 1 second to simulate system processing
3. Create second user and save their ID
4. Add a short 500ms delay for system consistency
5. Get all users and verify both exist

Each step builds on the previous ones, showing how to:

- Chain requests together
- Save and use response data
- Verify state changes
- Handle different HTTP methods
- Work with multiple resources
- Use strategic delays for system consistency

The delays in this example are for demonstration purposes. In real-world scenarios, you might use delays when:

- Waiting for asynchronous operations to complete
- Ensuring system consistency in distributed systems
- Rate limiting your API requests
- Testing timeout scenarios
