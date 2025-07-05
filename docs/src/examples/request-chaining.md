# Request Chaining & Delays

This example demonstrates how to chain HTTP requests and use delays in your test suites. It uses our hosted test server at `tryme.rocketship.sh` to show real-world API testing scenarios.

## Test Specification

```yaml
name: "Request Chaining & Delays Example"
description: "A test suite demonstrating request chaining and delays with the test server"
version: "v1.0.0"
tests:
  - name: "Car Management Flow"
    steps:
      - name: "Create first car"
        plugin: "http"
        config:
          method: "POST"
          url: "https://tryme.rocketship.sh/cars"
          body: |
            {
              "make": "Toyota",
              "model": "Corolla",
              "year": 2020
            }
        assertions:
          - type: "status_code"
            expected: 200
          - type: "header"
            name: "content-type"
            expected: "application/json"
          - type: "json_path"
            path: ".make"
            expected: "Toyota"
        save:
          - json_path: ".id"
            as: "first_car_id"
          - json_path: ".model"
            as: "first_car_model"
          - header: "server"
            as: "server_info"

      - name: "Wait for system processing"
        plugin: "delay"
        config:
          duration: "1s"

      - name: "Create second car"
        plugin: "http"
        config:
          method: "POST"
          url: "https://tryme.rocketship.sh/cars"
          body: |
            {
              "make": "Honda",
              "model": "Civic", 
              "year": 2022,
              "server_used": "{{ server_info }}"
            }
        assertions:
          - type: "status_code"
            expected: 200
        save:
          - json_path: ".id"
            as: "second_car_id"

      - name: "Short delay for consistency"
        plugin: "delay"
        config:
          duration: "500ms"

      - name: "List all cars"
        plugin: "http"
        config:
          method: "GET"
          url: "https://tryme.rocketship.sh/cars"
        assertions:
          - type: "status_code"
            expected: 200
          - type: "json_path"
            path: ".cars_0.make"
            expected: "Toyota"
          - type: "json_path"
            path: ".cars_1.make"
            expected: "Honda"
          - type: "json_path"
            path: ".cars_1.server_used"
            expected: "{{ server_info }}"

      - name: "Cleanup - Delete first car"
        plugin: "http"
        config:
          method: "DELETE"
          url: "https://tryme.rocketship.sh/cars/{{ first_car_id }}"
        assertions:
          - type: "status_code"
            expected: 204

      - name: "Cleanup - Delete second car"
        plugin: "http"
        config:
          method: "DELETE"
          url: "https://tryme.rocketship.sh/cars/{{ second_car_id }}"
        assertions:
          - type: "status_code"
            expected: 204
```

## Key Features Demonstrated

**Request Chaining**:

1. Creating multiple cars with different data
2. Saving response values (JSON and headers) for later use
3. Using saved header values in subsequent request bodies
4. Verifying changes across requests with variable substitution

**Header Operations**:

1. Header validation with `type: "header"` assertions
2. Header value extraction with `header: "server"` saves
3. Using saved header values in request body: `"server_used": "{{ server_info }}"`

**Delays**:

1. Using delays between operations
2. Different delay durations (1s, 500ms)
3. Strategic placement for system consistency

**Assertions**:

1. Status code validation
2. Header validation (content-type)
3. JSON response validation using JSONPath
4. Variable substitution validation

## Running the Example

Run the test suite:

```bash
rocketship run -af examples/request-chaining/rocketship.yaml
```

## Understanding the Flow

The example demonstrates a complete car management workflow with header operations:

1. **Create first car** - Save car ID, model (JSON) and server header value
2. **Wait for system processing** - 1 second delay
3. **Create second car** - Use saved header value in request body
4. **Short delay** - 500ms for consistency
5. **List all cars** - Verify both cars exist and header value was passed through
6. **Cleanup** - Delete both cars using saved IDs

Each step builds on the previous ones, showing how to:

- **Chain requests together** with variable substitution
- **Save and use response data** from both JSON and headers
- **Pass header values through request workflows**
- **Verify state changes** across multiple operations
- **Handle different HTTP methods** (POST, GET, DELETE)
- **Work with multiple resources** and lifecycle management
- **Use strategic delays** for system consistency

The delays in this example are for demonstration purposes. In real-world scenarios, you might use delays when:

- Waiting for asynchronous operations to complete
- Ensuring system consistency in distributed systems
- Rate limiting your API requests
- Testing timeout scenarios

## Handlebars Escaping in Request Bodies

When your APIs return or expect handlebars syntax (`{{ }}`), use backslash escaping to include literal handlebars:

```yaml
- name: "Send template documentation"
  plugin: "http"
  config:
    method: "POST"
    url: "https://tryme.rocketship.sh/docs"
    body: |-
      {
        "instructions": "Use \\{{ user_id }} in your API calls",
        "template_example": "Welcome \\{{ user_name }}!",
        "processed_value": "Current environment: {{ .vars.environment }}"
      }
  assertions:
    - type: "json_path"
      path: ".instructions"
      expected: "Use {{ user_id }} in your API calls"
```

The backslash (`\`) escapes the handlebars, making `\\{{ user_id }}` output literal `{{ user_id }}` instead of trying to process it as a variable.

See the Handlebars Escaping section in (variables.md) for complete details and advanced usage.
