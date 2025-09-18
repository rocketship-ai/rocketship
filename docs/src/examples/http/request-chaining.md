# HTTP Request Chaining

Rocketship’s HTTP plugin lets you stitch multiple calls together by passing data from one response into the next. This example extends the original request-chaining suite so you can see how state flows across steps while the delay plugin keeps the external system stable. For timing specifics, check out the [Managing Delays](delays.md) guide.

## Full Test Specification

```yaml
name: "Request Chaining Example"
description: "Chaining POST/GET/DELETE calls against the tryme server"
openapi:
  spec: "./contracts/tryme.yaml"
  cache_ttl: "30m"
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

## Key Techniques

### Request Chaining

- Persist IDs (`save` block) to drive later requests.
- Reuse headers (`server_info`) captured from earlier responses.
- Mix HTTP verbs (POST, GET, DELETE) inside the same flow.

### Header and Body Operations

- Validate headers with `type: "header"` assertions.
- Save header values and inject them into future request bodies.
- Apply JSONPath assertions to ensure the API echoes chained state correctly.

### Assertions

- Combine status codes, headers, and JSONPath checks for comprehensive validation.
- Reference saved runtime values in assertions to guarantee chained data propagated.

## Running the Example

```bash
rocketship run -af examples/request-chaining/rocketship.yaml
```

The example YAML above matches the suite shipped in `examples/request-chaining/rocketship.yaml`, so you can experiment immediately.

## Understanding the Flow

1. **Create first car** – capture the new car ID, its model, and the upstream `server` header.
2. **Delay** – give the remote service a moment to settle before reusing data (details in [Managing Delays](delays.md)).
3. **Create second car** – reuse the header value inside the body and save another ID.
4. **Aggregate read** – verify both cars exist and the reused header value appears in the second payload.
5. **Cleanup** – delete both resources to keep the shared tryme environment tidy.

Next, dive into [Contract Validation](openapi-validation.md) to see how this same suite enforces the OpenAPI schema.
