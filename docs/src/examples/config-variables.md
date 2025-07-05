# Configuration Variables Example

This example demonstrates how to use configuration variables in Rocketship test suites. Configuration variables allow you to parameterize your tests, making them reusable across different environments and configurations.

## Key Features Demonstrated

- **Configuration Variables**: Define reusable variables in the `vars` section using `{{ .vars.variable_name }}`
- **Runtime Variables**: Use variables generated during test execution using `{{ variable_name }}`
- **Nested Variables**: Support for nested structures like `{{ .vars.auth.token }}`
- **Mixed Variables**: Combine config and runtime variables in the same test
- **CLI Overrides**: Override config variables from the command line
- **Variable Files**: Load variables from external files

For information about environment variables, see the [Environment Variables guide](environment-variables.md).

## When to Use Each Variable Type

- **Config Variables (`{{ .vars.* }}`)**: Non-sensitive configuration, test data, mock responses
- **Environment Variables (`{{ .env.* }}`)**: Secrets, API keys, environment-specific values
- **Runtime Variables (`{{ variable }}`)**: Values captured during test execution

### Configuration Variables Section

```yaml
vars:
  base_url: "https://tryme.rocketship.sh"
  environment: "staging"
  timeout: 2
  auth:
    header_name: "X-API-Key"
    token: "test-api-key-123"
  book:
    title: "The Go Programming Language"
    author: "Alan Donovan"
    isbn: "978-0134190440"
tests: ...
```

### Variable Usage Patterns

#### 1. Basic Config Variables

```yaml
- name: "Create book"
  plugin: "http"
  config:
    method: "POST"
    url: "{{ .vars.base_url }}/books"
    headers:
      "{{ .vars.auth.header_name }}": "{{ .vars.auth.token }}"
```

#### 2. Mixed Config and Runtime Variables

```yaml
- name: "Get book"
  plugin: "http"
  config:
    url: "{{ .vars.base_url }}/books/{{ book_id }}" # Config + Runtime
  assertions:
    - type: "json_path"
      path: ".environment"
      expected: "{{ .vars.environment }}" # Config variable
    - type: "json_path"
      path: ".id"
      expected: "{{ book_id }}" # Runtime variable (from save)
```

#### 3. Config Variables in Plugin Configuration

```yaml
- name: "Wait with config timeout"
  plugin: "delay"
  config:
    duration: "{{ .vars.timeout }}s"
```

## Running the Example

### Basic Usage

```bash
# Run with default variables
rocketship run -af examples/config-variables/rocketship.yaml
```

### CLI Variable Overrides

```bash
# Override single variables
rocketship run -af examples/config-variables/rocketship.yaml \
  --var base_url=https://api.production.com \
  --var environment=production

# Override nested variables
rocketship run -af examples/config-variables/rocketship.yaml \
  --var auth.token=prod-api-key-456 \
  --var book.title="Advanced Go Programming"
```

### Using Variable Files

Create a `prod-vars.yaml` file:

```yaml
base_url: "https://api.production.com"
environment: "production"
auth:
  token: "prod-api-key-456"
timeout: 60
```

Then run:

```bash
rocketship run -af examples/config-variables/rocketship.yaml --var-file prod-vars.yaml
```

## Variable Precedence

Variables are resolved in this order (highest to lowest precedence):

1. **CLI Variables** (`--var key=value`)
2. **Variable Files** (`--var-file vars.yaml`)
3. **YAML vars section** (built into test file)

## Best Practices

### 1. Clear Variable Naming

Use descriptive names that indicate purpose:

```yaml
vars:
  api_base_url: "https://api.staging.com"
  max_retry_count: 3
  test_user_email: "test@example.com"
```

### 2. Environment-Specific Configurations

Structure variables for easy environment switching:

```yaml
vars:
  environment: "staging"
  api:
    base_url: "https://api.staging.com"
    timeout: 30
  database:
    host: "db.staging.com"
    port: 5432
```

### 3. Separate Config from Runtime

- **Config variables**: Use `{{ .vars.* }}` for environment/configuration values
- **Runtime variables**: Use `{{ variable }}` for values captured during test execution

### 4. Variable Files for Environments

Create separate variable files for each environment:

- `vars/staging.yaml`
- `vars/production.yaml`
- `vars/development.yaml`

## Variable Types Supported

- **Strings**: `"value"`
- **Numbers**: `42`, `3.14`
- **Booleans**: `true`, `false`
- **Objects**: Nested key-value structures
- **Arrays**: Lists of values

## Integration with Test Flow

The configuration variables example demonstrates a complete CRUD flow:

1. **Create** a book with config variables for API endpoint and auth
2. **Read** the book using mixed config and runtime variables
3. **Update** the book with runtime data from previous steps
4. **Delete** the book for cleanup

This pattern shows how config variables work seamlessly with Rocketship's existing runtime variable system from `save` blocks.
