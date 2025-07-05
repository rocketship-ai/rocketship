# Variables

Rocketship supports three types of variables for parameterizing your tests:

- **Environment Variables**: `{{ .env.VAR_NAME }}` - System env vars and secrets
- **Config Variables**: `{{ .vars.variable_name }}` - Non-sensitive configuration  
- **Runtime Variables**: `{{ variable_name }}` - Values captured during test execution

## Environment Variables

### Using --env-file (Recommended)

Create a `.env` file:
```bash
# .env
API_BASE_URL=https://api.staging.com
API_KEY=sk-staging-key-123
DATABASE_URL=postgres://user:pass@localhost/db
```

Run with env file:
```bash
rocketship run -af test.yaml --env-file .env
```

Access in tests:
```yaml
- name: "API request"
  plugin: "http"
  config:
    url: "{{ .env.API_BASE_URL }}/users"
    headers:
      "Authorization": "Bearer {{ .env.API_KEY }}"
```

### Multi-Environment Setup

```
project/
├── .env.example      # Template (commit this)
├── .env             # Local development (gitignore)
├── .env.staging     # Staging values (gitignore)
├── .env.production  # Production values (gitignore)
└── tests/api.yaml
```

Usage:
```bash
# Local
rocketship run -af test.yaml --env-file .env

# Staging  
rocketship run -af test.yaml --env-file .env.staging

# Production
rocketship run -af test.yaml --env-file .env.production
```

### CI/CD Integration

**GitHub Actions:**
```yaml
- name: Run tests
  env:
    API_KEY: ${{ secrets.API_KEY }}
    DATABASE_URL: ${{ secrets.DATABASE_URL }}
  run: rocketship run -af test.yaml
```

**GitLab CI:**
```yaml
test:
  script: rocketship run -af test.yaml
  variables:
    API_KEY: $CI_API_KEY
    DATABASE_URL: $CI_DATABASE_URL
```

### Precedence

Environment variables follow this precedence:

1. **System environment variables** (highest priority)
2. **Variables from `--env-file`**
3. **Default values in test files**

System env vars are never overridden by file values.

## Config Variables

Define reusable configuration in the `vars` section:

```yaml
vars:
  base_url: "https://api.staging.com"
  timeout: 30
  auth:
    header_name: "X-API-Key" 
    token: "test-key-123"

tests:
  - name: "API test"
    steps:
      - name: "Create resource"
        plugin: "http"
        config:
          url: "{{ .vars.base_url }}/resources"
          headers:
            "{{ .vars.auth.header_name }}": "{{ .vars.auth.token }}"
          timeout: "{{ .vars.timeout }}s"
```

### CLI Overrides

```bash
# Override variables
rocketship run -af test.yaml \
  --var base_url=https://api.production.com \
  --var auth.token=prod-key-456

# Use variable files
rocketship run -af test.yaml --var-file prod-vars.yaml
```

### Variable Files

Create `prod-vars.yaml`:
```yaml
base_url: "https://api.production.com"
environment: "production"
auth:
  token: "prod-key-456"
timeout: 60
```

### Precedence

Config variables follow this precedence:

1. **CLI Variables** (`--var key=value`)
2. **Variable Files** (`--var-file vars.yaml`)
3. **YAML vars section**

## Runtime Variables

Capture values during test execution:

```yaml
- name: "Create user"
  plugin: "http"
  config:
    method: "POST"
    url: "{{ .vars.base_url }}/users"
  save:
    - json_path: ".id"
      as: "user_id"

- name: "Get user"
  plugin: "http"  
  config:
    url: "{{ .vars.base_url }}/users/{{ user_id }}"
```

## Mixed Usage

Combine all variable types:

```yaml
vars:
  api_version: "v1"

tests:
  - name: "Mixed variables"
    steps:
      - name: "Create resource"
        plugin: "http"
        config:
          url: "{{ .env.API_BASE_URL }}/{{ .vars.api_version }}/resources"
          headers:
            "Authorization": "Bearer {{ .env.API_TOKEN }}"
        save:
          - json_path: ".id"
            as: "resource_id"

      - name: "Get resource"  
        plugin: "http"
        config:
          url: "{{ .env.API_BASE_URL }}/{{ .vars.api_version }}/resources/{{ resource_id }}"
```

## Handlebars Escaping

When APIs use handlebars syntax (`{{ }}`), escape with backslashes:

```yaml
# Normal variable processing
"message": "Hello {{ user_name }}"

# Escaped (literal handlebars)
"template": "Use \\{{ user_id }} in API"     # Outputs: Use {{ user_id }} in API
```

**Multiple escape levels:**
```yaml
# 1 backslash (odd) → literal handlebars
"docs": "Use \\{{ variable }}"              # → Use {{ variable }}

# 2 backslashes (even) → backslash + processed variable
"path": "\\\\{{ .vars.api_path }}"          # → \staging/api

# 3 backslashes (odd) → backslash + literal handlebars  
"example": "\\\\\\{{ variable }}"           # → \{{ variable }}
```

**In JSON contexts:**
```yaml
body: |-
  {
    "instructions": "Use \\{{ user_id }} in requests",
    "template": "Syntax: \\{{ variable_name }} for literals"
  }
```

## Best Practices

### Security
```bash
# .gitignore
.env
.env.*
!.env.example
```

### Environment Variables vs Config Variables

- **Environment Variables (`{{ .env.* }}`)**: API keys, secrets, environment-specific URLs
- **Config Variables (`{{ .vars.* }}`)**: Test data, timeouts, non-sensitive configuration
- **Runtime Variables (`{{ variable }}`)**: Values captured during test execution

### Variable Validation

```yaml
- name: "Validate environment"
  plugin: "script"
  config:
    language: "javascript"
    script: |
      const required = ['API_KEY', 'DATABASE_URL'];
      const missing = required.filter(key => !process.env[key]);
      
      if (missing.length > 0) {
        throw new Error(`Missing env vars: ${missing.join(', ')}`);
      }
```

### Default Values

```yaml
config:
  timeout: "{{ .env.TIMEOUT_SECONDS | default '30' }}s"
  retries: "{{ .env.MAX_RETRIES | default '3' }}"
```

## Examples

```bash
# Basic environment variables
rocketship run -af test.yaml --env-file .env

# Config variables with overrides
rocketship run -af test.yaml --var environment=production

# Mixed usage
rocketship run -af test.yaml --env-file .env --var-file prod-vars.yaml
```