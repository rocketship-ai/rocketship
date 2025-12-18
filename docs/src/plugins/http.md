# HTTP Plugin

Test web APIs (like REST services) by sending HTTP requests and checking the responses. The HTTP plugin lets you:
- Send requests (GET, POST, PUT, DELETE, etc.)
- Include headers and request bodies
- Validate responses automatically
- Chain requests together (use data from one request in the next)
- Validate against OpenAPI specifications

## Quick Start

```yaml
tests:
  - name: "Create user"
    steps:
      - name: "POST request"
        plugin: http
        config:
          method: POST
          url: "https://api.example.com/users"
          headers:
            Content-Type: "application/json"
          body: |
            {
              "name": "Test User",
              "email": "test@example.com"
            }
        assertions:
          - type: status_code
            expected: 201
        save:
          - json_path: ".id"
            as: "user_id"
```

## Configuration

### Required Fields

| Field | Description | Example |
|-------|-------------|---------|
| `method` | HTTP method | `GET`, `POST`, `PUT`, `DELETE`, `PATCH` |
| `url` | Request URL | `https://api.example.com/users` |

### Optional Fields

| Field | Description | Example |
|-------|-------------|---------|
| `headers` | HTTP headers | `{"Authorization": "Bearer token"}` |
| `body` | Request body (string) | `{"key": "value"}` |
| `form` | URL-encoded form data | `{"username": "test"}` |
| `openapi` | OpenAPI validation config | See [OpenAPI Validation](#openapi-validation) |

## Request Chaining

One of the most powerful features: **pass data between requests**. 

**Example scenario:** 
1. Create a new user (the API returns a user ID)
2. Use that ID to fetch the user's details
3. Update the user's information

Each step can save values from the response and use them in the next step:

```yaml
steps:
  # Step 1: Create resource
  - name: "Create car"
    plugin: http
    config:
      method: POST
      url: "https://api.example.com/cars"
      body: |
        {
          "make": "Toyota",
          "model": "Corolla"
        }
    save:
      - json_path: ".id"
        as: "car_id"
      - header: "server"
        as: "server_info"

  # Step 2: Use saved values
  - name: "Get car"
    plugin: http
    config:
      method: GET
      url: "https://api.example.com/cars/{{ car_id }}"
```

## Assertions

Assertions let you **automatically check if the response is correct**. Instead of manually reading the response, you tell Rocketship what to look for, and it fails the test if expectations aren't met.

**Types of assertions:**

### Status Code

```yaml
assertions:
  - type: status_code
    expected: 200
```

### JSON Path

```yaml
assertions:
  - type: json_path
    path: ".user.email"
    expected: "test@example.com"
  - type: json_path
    path: ".items | length"
    expected: 3
```

### Headers

```yaml
assertions:
  - type: header
    name: "content-type"
    expected: "application/json"
```

## Save Fields

Extract values from responses for use in later steps:

### From JSON Response

```yaml
save:
  - json_path: ".id"
    as: "resource_id"
  - json_path: ".data.token"
    as: "auth_token"
```

### From Headers

```yaml
save:
  - header: "x-request-id"
    as: "request_id"
  - header: "content-type"
    as: "response_type"
```

## OpenAPI Validation

Validate requests and responses against OpenAPI v3 contracts.

### Suite-Level Configuration

Apply OpenAPI validation to all HTTP steps:

```yaml
openapi:
  spec: "./contracts/api.yaml"  # Local file or HTTP(S) URL
  cache_ttl: "30m"              # Contract cache duration
  validate_request: true        # Validate outgoing requests
  validate_response: true       # Validate incoming responses

tests:
  - name: "API test"
    steps:
      - plugin: http
        config:
          method: POST
          url: "https://api.example.com/users"
          body: '{"name": "Test"}'
        # Inherits suite-level OpenAPI validation
```

### Step-Level Overrides

Override validation for specific steps:

```yaml
- name: "Test malformed payload"
  plugin: http
  config:
    method: POST
    url: "https://api.example.com/users"
    body: '{"invalid": "data"}'
    openapi:
      validate_request: false  # Skip request validation for this step
```

### OpenAPI Options

| Field | Description | Example |
|-------|-------------|---------|
| `spec` | Path or URL to OpenAPI document | `./api.yaml` or `https://api.example.com/spec` |
| `cache_ttl` | Contract cache duration | `30m` (default) |
| `version` | Version identifier to force cache refresh | `"2024-03-15"` |
| `validate_request` | Enable request validation | `true` |
| `validate_response` | Enable response validation | `true` |
| `operation_id` | Require specific operation | `"createUser"` |

## Form Data

Submit URL-encoded forms:

```yaml
- name: "Login form"
  plugin: http
  config:
    method: POST
    url: "https://app.example.com/login"
    form:
      username: "test@example.com"
      password: "testpass"
```

Note: If both `form` and `body` are provided, `form` takes precedence.

## Common Patterns

### Authentication

```yaml
steps:
  - name: "Login"
    plugin: http
    config:
      method: POST
      url: "{{ .env.API_URL }}/auth/login"
      body: |
        {
          "email": "{{ .env.TEST_EMAIL }}",
          "password": "{{ .env.TEST_PASSWORD }}"
        }
    save:
      - json_path: ".token"
        as: "auth_token"

  - name: "Authenticated request"
    plugin: http
    config:
      method: GET
      url: "{{ .env.API_URL }}/me"
      headers:
        Authorization: "Bearer {{ auth_token }}"
```

### Query Parameters

```yaml
- name: "Search with filters"
  plugin: http
  config:
    method: GET
    url: "{{ .vars.api_url }}/search?q=test&limit=10&offset=0"
```

## See Also

- [Variables](../features/variables.md) - Using environment, config, and runtime variables
- [Retry Policies](../features/retry-policies.md) - Retrying failed HTTP requests
