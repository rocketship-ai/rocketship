# Request Chaining & Delays

This example demonstrates how to chain HTTP requests and use delays in your test suites. It uses the test server included in the repository to show real-world API testing scenarios.

## Test Specification

```yaml
name: "Request Chaining & Delays Example"
description: "A test suite demonstrating request chaining and delays with the test server"
version: "v1.0.0"
tests:
  - name: "Create and Verify User"
    steps:
      - name: "Create a new user"
        plugin: "http"
        config:
          method: "POST"
          url: "http://localhost:8080/users"
          headers:
            Content-Type: "application/json"
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
            path: ".name"
            expected: "John Doe"
        save:
          - json_path: ".id"
            as: "user_id"

      - name: "Wait for system processing"
        plugin: "delay"
        config:
          duration: "1s"

      - name: "Verify user creation"
        plugin: "http"
        config:
          method: "GET"
          url: "http://localhost:8080/users/{{ user_id }}"
        assertions:
          - type: "status_code"
            expected: 200
          - type: "json_path"
            path: ".email"
            expected: "john@example.com"

  - name: "Update and List Users"
    steps:
      - name: "Update user information"
        plugin: "http"
        config:
          method: "PUT"
          url: "http://localhost:8080/users/{{ user_id }}"
          headers:
            Content-Type: "application/json"
          body: |
            {
              "name": "John Doe Jr",
              "email": "john.jr@example.com",
              "role": "user"
            }
        assertions:
          - type: "status_code"
            expected: 200
          - type: "json_path"
            path: ".name"
            expected: "John Doe Jr"

      - name: "Short delay for consistency"
        plugin: "delay"
        config:
          duration: "500ms"

      - name: "List all users"
        plugin: "http"
        config:
          method: "GET"
          url: "http://localhost:8080/users"
        assertions:
          - type: "status_code"
            expected: 200
          - type: "json_path"
            path: "to_entries | length"
            expected: 1
```

## Key Features Demonstrated

**Request Chaining**:

1. Creating a user and saving its ID
2. Using the saved ID in subsequent requests
3. Verifying changes across requests

**Delays**:

1. Using delays between operations
2. Different delay durations (1s, 500ms)
3. Strategic placement for system consistency

**HTTP Operations**:

1. POST: Creating resources
2. GET: Retrieving resources
3. PUT: Updating resources

**Assertions**:

1. Status code validation
2. JSON response validation using JSONPath
3. Response count validation

## Running the Example

1. Start the test server (included in the repository):

```bash
go run for-contributors/test-server/main.go
```

2. Run the test suite:

```bash
rocketship run -af examples/request-chaining/rocketship.yaml
```

## Understanding the Flow

1. First Test ("Create and Verify User"):

   - Creates a new user
   - Waits for 1 second to simulate system processing
   - Verifies the user was created correctly

2. Second Test ("Update and List Users"):
   - Updates the user's information
   - Adds a short delay for system consistency
   - Lists all users to verify the update

The delays in this example are for demonstration purposes. In real-world scenarios, you might use delays when:

- Waiting for asynchronous operations to complete
- Ensuring system consistency in distributed systems
- Rate limiting your API requests
- Testing timeout scenarios
