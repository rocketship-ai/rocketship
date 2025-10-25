# Config & Runtime Variables

Config variables parameterize tests with reusable configuration. Runtime variables chain test steps by saving and reusing values from responses.

## Config Variables

Config variables provide non-sensitive test parameters defined in YAML files.

**Syntax:** `{{ .vars.variable_name }}`

### Defining Config Variables

```yaml
vars:
  base_url: "https://api.staging.com"
  timeout: 30
  max_retries: 3
  auth:
    header_name: "X-API-Key"
    token: "test-key-123"

tests:
  - name: "API test"
    steps:
      - plugin: "http"
        config:
          url: "{{ .vars.base_url }}/resources"
          headers:
            "{{ .vars.auth.header_name }}": "{{ .vars.auth.token }}"
          timeout: "{{ .vars.timeout }}s"
```

### Nested Variables

Access nested values with dot notation:

```yaml
vars:
  api:
    base_url: "https://api.example.com"
    version: "v2"
    endpoints:
      users: "/users"
      posts: "/posts"

tests:
  - name: "Use nested config"
    steps:
      - plugin: "http"
        config:
          url: "{{ .vars.api.base_url }}/{{ .vars.api.version }}{{ .vars.api.endpoints.users }}"
```

### CLI Overrides

Override variables without modifying YAML:

```bash
# Override individual variables
rocketship run -af test.yaml \
  --var base_url=https://api.production.com \
  --var timeout=60

# Override nested variables
rocketship run -af test.yaml \
  --var api.base_url=https://prod-api.com \
  --var api.version=v3
```

### Variable Files

Use variable files for complex overrides:

**Create `prod-vars.yaml`:**

```yaml
base_url: "https://api.production.com"
environment: "production"
timeout: 60
max_retries: 5
```

**Load variable file:**

```bash
rocketship run -af test.yaml --var-file prod-vars.yaml

# Combine with individual overrides
rocketship run -af test.yaml \
  --var-file prod-vars.yaml \
  --var timeout=120
```

## Runtime Variables

Runtime variables capture values during test execution for step chaining.

**Syntax:** `{{ variable_name }}`

### Save from Responses

```yaml
- name: "Create user"
  plugin: "http"
  config:
    method: "POST"
    url: "{{ .vars.base_url }}/users"
    body: |
      {
        "name": "Test User",
        "email": "test@example.com"
      }
  save:
    - json_path: ".id"
      as: "user_id"
    - json_path: ".email"
      as: "user_email"

- name: "Get user by ID"
  plugin: "http"
  config:
    url: "{{ .vars.base_url }}/users/{{ user_id }}"
```

### Chaining Multiple Steps

```yaml
- name: "Create company"
  plugin: "supabase"
  config:
    url: "{{ .vars.supabase_url }}"
    key: "{{ .env.SUPABASE_SERVICE_KEY }}"
    operation: "insert"
    table: "companies"
    insert:
      data:
        name: "Test Company"
  save:
    - json_path: ".[0].id"
      as: "company_id"

- name: "Create user for company"
  plugin: "supabase"
  config:
    url: "{{ .vars.supabase_url }}"
    key: "{{ .env.SUPABASE_SERVICE_KEY }}"
    operation: "insert"
    table: "users"
    insert:
      data:
        company_id: "{{ company_id }}"
        name: "Company Admin"
  save:
    - json_path: ".[0].id"
      as: "user_id"

- name: "Verify relationship"
  plugin: "supabase"
  config:
    url: "{{ .vars.supabase_url }}"
    key: "{{ .env.SUPABASE_SERVICE_KEY }}"
    operation: "select"
    table: "users"
    select:
      filters:
        - column: "id"
          operator: "eq"
          value: "{{ user_id }}"
  assertions:
    - type: "json_path"
      path: ".[0].company_id"
      expected: "{{ company_id }}"
```

### Optional vs Required Saves

By default, saves are **required** and fail if the value doesn't exist:

```yaml
- name: "Get user profile"
  plugin: "http"
  config:
    url: "{{ .vars.base_url }}/users/{{ user_id }}"
  save:
    # Required (default) - fails if not present
    - json_path: ".id"
      as: "user_id"

    # Optional - continues if not present
    - json_path: ".profile.avatar_url"
      as: "avatar_url"
      required: false

    - json_path: ".profile.bio"
      as: "user_bio"
      required: false
```

### Save from Headers

```yaml
- name: "Login"
  plugin: "http"
  config:
    url: "{{ .vars.base_url }}/auth/login"
    method: "POST"
  save:
    - json_path: ".session.token"
      as: "auth_token"
    - header: "X-Session-ID"
      as: "session_id"

- name: "Use saved headers"
  plugin: "http"
  config:
    url: "{{ .vars.base_url }}/profile"
    headers:
      "Authorization": "Bearer {{ auth_token }}"
      "X-Session-ID": "{{ session_id }}"
```

## Examples

### Parameterized Test Suite

```yaml
vars:
  target_environment: "staging"
  test_user_count: 5
  cleanup_after_test: true

tests:
  - name: "Parameterized integration test"
    steps:
      - name: "Log configuration"
        plugin: "log"
        config:
          message: |
            Running test:
            - Environment: {{ .vars.target_environment }}
            - User count: {{ .vars.test_user_count }}
            - Cleanup: {{ .vars.cleanup_after_test }}
```

Run with different configurations:

```bash
# Staging with 5 users
rocketship run -af test.yaml \
  --var target_environment=staging \
  --var test_user_count=5

# Production with 100 users
rocketship run -af test.yaml \
  --var target_environment=production \
  --var test_user_count=100 \
  --var cleanup_after_test=false
```

### Dynamic Test Data

```yaml
vars:
  company_prefix: "test-company"

tests:
  - name: "Dynamic test data"
    steps:
      - name: "Generate unique name"
        plugin: "script"
        config:
          language: "javascript"
          script: |
            const timestamp = Date.now();
            const uniqueName = `${state.company_prefix}-${timestamp}`;
            save("company_name", uniqueName);

      - name: "Create with generated name"
        plugin: "http"
        config:
          url: "{{ .env.API_BASE_URL }}/companies"
          method: "POST"
          body: |
            {
              "name": "{{ company_name }}",
              "type": "test"
            }
```

### Conditional Logic

```yaml
vars:
  skip_cleanup: false
  debug_mode: true

tests:
  - name: "Conditional execution"
    steps:
      - name: "Create data"
        plugin: "http"
        config:
          url: "{{ .env.API_BASE_URL }}/data"
          method: "POST"
        save:
          - json_path: ".id"
            as: "data_id"

      - name: "Debug output"
        plugin: "script"
        config:
          language: "javascript"
          script: |
            if (state.debug_mode === 'true') {
              console.log(`Created data: ${state.data_id}`);
            }

      - name: "Cleanup"
        plugin: "script"
        config:
          language: "javascript"
          script: |
            if (state.skip_cleanup !== 'true') {
              console.log('Cleaning up...');
            } else {
              console.log('Skipping cleanup');
            }
```

## Best Practices

### Descriptive Variable Names

```yaml
# ❌ Bad
vars:
  u: "https://api.com"
  t: 30

# ✅ Good
vars:
  base_url: "https://api.com"
  request_timeout: 30
```

### Group Related Variables

```yaml
vars:
  api:
    base_url: "https://api.example.com"
    version: "v2"
    timeout: 30
  database:
    table_prefix: "test_"
    max_connections: 10
```

### Validate Runtime Variables

```yaml
- name: "Create user"
  plugin: "http"
  config:
    method: "POST"
    url: "{{ .vars.base_url }}/users"
  save:
    - json_path: ".id"
      as: "user_id"

- name: "Validate saved user_id"
  plugin: "script"
  config:
    language: "javascript"
    script: |
      if (!state.user_id) {
        throw new Error("user_id not saved");
      }
      if (typeof state.user_id !== 'string') {
        throw new Error(`user_id should be string, got ${typeof state.user_id}`);
      }
```
