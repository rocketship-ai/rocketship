# Simple HTTP Tests

This example demonstrates basic HTTP endpoint testing with request chaining, variable saving, and assertions. It simulates a user management API workflow.

## Test Specification

```yaml
name: "Request Chaining Test Suite"
description: "A test suite demonstrating request chaining with the test server"
version: "v1.0.0"
tests:
  - name: "Create and Chain User Operations"
    steps:
      - name: "Create first user"
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
          - type: "json_path"
            path: ".email"
            expected: "john@example.com"
          - type: "header"
            name: "Content-Type"
            expected: "application/json"
        save:
          - json_path: ".id"
            as: "first_user_id"
          - json_path: ".email"
            as: "first_user_email"

      # ... More steps in the full example ...
```

## Key Features Demonstrated

1. **HTTP Methods**: The example shows various HTTP methods:

   - POST: Creating users
   - GET: Retrieving user information
   - PUT: Updating user data
   - DELETE: Removing users

2. **Assertions**:

   - Status code validation
   - JSON response validation using JSONPath
   - Header validation

3. **Variable Management**:

   - Saving response values using `save`
   - Using saved variables in subsequent requests with `{{ variable_name }}`

4. **Request Chaining**:
   - Creating dependent requests
   - Verifying changes from previous operations

## Running the Example

1. Start the test server (included in the repository):

```bash
go run for-contributors/test-server/main.go
```

2. Run the test suite:

```bash
rocketship run -af examples/simple-http/rocketship.yaml
```

## Full Example

The complete example includes additional steps for:

- Creating multiple users
- Retrieving user lists
- Updating user information
- Deleting users
- Verifying operations

You can find the full example in the [repository](https://github.com/rocketship-ai/rocketship/blob/main/examples/simple-http/rocketship.yaml).
