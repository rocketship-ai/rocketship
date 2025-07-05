# Using Environment Files

Load environment variables from `.env` files for secure, environment-specific configuration.

## Overview

The `--env-file` flag allows you to load environment variables from a file instead of setting them in your shell. This is particularly useful for:

- Managing secrets without exposing them in shell history
- Switching between different environments easily
- Keeping sensitive data out of version control
- Maintaining consistency across team members

## Basic Usage

### 1. Create an .env file

```bash
# .env
API_BASE_URL=https://api.staging.com
API_KEY=sk-staging-key-123
DATABASE_URL=postgres://user:pass@localhost/db
ENVIRONMENT=staging
```

### 2. Run tests with --env-file

```bash
rocketship run -af test.yaml --env-file .env
```

### 3. Access in your tests

```yaml
- name: "API request"
  plugin: "http"
  config:
    url: "{{ .env.API_BASE_URL }}/users"
    headers:
      "Authorization": "Bearer {{ .env.API_KEY }}"
```

## File Format

The `.env` file follows standard environment file format:

```bash
# Comments start with #
KEY=value

# Empty lines are ignored

# Quotes are optional but useful for values with spaces
MESSAGE="Hello World"
SINGLE_QUOTED='Also works'

# Empty values are supported
EMPTY_VALUE=

# No spaces around the equals sign
CORRECT=value
WRONG = value  # This will fail
```

## Multi-Environment Setup

### Directory Structure

```
project/
├── .env.example      # Template (commit this)
├── .env             # Local development (gitignore)
├── .env.staging     # Staging values (gitignore)
├── .env.production  # Production values (gitignore)
└── tests/
    └── api.yaml
```

### .env.example (commit to git)

```bash
# API Configuration
API_BASE_URL=http://localhost:3000
API_KEY=your-api-key-here

# Database
DATABASE_URL=postgres://user:pass@localhost/db

# Feature Flags
ENABLE_NEW_FEATURE=false
```

### Usage

```bash
# Local development
rocketship run -af tests/api.yaml --env-file .env

# Staging
rocketship run -af tests/api.yaml --env-file .env.staging

# Production
rocketship run -af tests/api.yaml --env-file .env.production
```

## CI/CD Integration

### GitHub Actions

```yaml
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      # Option 1: Use GitHub secrets directly (recommended)
      - name: Run tests with secrets
        env:
          API_KEY: ${{ secrets.API_KEY }}
          DATABASE_URL: ${{ secrets.DATABASE_URL }}
        run: |
          rocketship run -af tests/api.yaml
      
      # Option 2: Create .env file from secrets
      - name: Create env file
        run: |
          echo "API_KEY=${{ secrets.API_KEY }}" > .env
          echo "DATABASE_URL=${{ secrets.DATABASE_URL }}" >> .env
          
      - name: Run tests with env file
        run: |
          rocketship run -af tests/api.yaml --env-file .env
```

### GitLab CI

```yaml
test:
  script:
    # GitLab CI variables are already in environment
    - rocketship run -af tests/api.yaml
  variables:
    API_KEY: $CI_API_KEY
    DATABASE_URL: $CI_DATABASE_URL
```

## Best Practices

### 1. Security

```bash
# .gitignore
.env
.env.*
!.env.example
```

### 2. Required Variables Check

```yaml
- name: "Validate environment"
  plugin: "script"
  config:
    language: "javascript"
    script: |
      // Check required env vars
      const required = ['API_KEY', 'DATABASE_URL', 'API_BASE_URL'];
      const missing = [];
      
      for (const key of required) {
        if (!process.env[key]) {
          missing.push(key);
        }
      }
      
      if (missing.length > 0) {
        throw new Error(`Missing required env vars: ${missing.join(', ')}`);
      }
      
      console.log("✅ All required environment variables are set");
```

### 3. Default Values

Use defaults in your test files for optional variables:

```yaml
config:
  timeout: "{{ .env.TIMEOUT_SECONDS | default '30' }}s"
  retries: "{{ .env.MAX_RETRIES | default '3' }}"
```

## Precedence

Environment variables follow this precedence order:

1. **System environment variables** (already set in shell) - **HIGHEST PRIORITY**
2. **Variables from `--env-file`** - only set if not already in environment
3. **Default values in test files** - used if variable not found

This means:
- System env vars are NEVER overridden by file values
- You can use `--env-file` for defaults while allowing overrides
- CI/CD secrets work seamlessly since they're set as system env vars
- Local developers can override file values by exporting variables

Example:
```bash
# .env file has API_KEY=default-key
# But you can override for this session:
export API_KEY=my-special-key
rocketship run -af test.yaml --env-file .env
# Uses API_KEY=my-special-key (system wins)
```

## Complete Example

### .env.staging

```bash
# Staging Environment Configuration
API_BASE_URL=https://api.staging.example.com
API_KEY=sk-staging-abc123
DATABASE_URL=postgres://user:pass@staging.db.com/myapp

# Feature Flags
ENABLE_NEW_FEATURE=true
DEBUG_MODE=true

# Timeouts
REQUEST_TIMEOUT=10
MAX_RETRIES=5
```

### test-suite.yaml

```yaml
name: "API Integration Tests"
version: "v1.0.0"

vars:
  # Non-sensitive defaults
  api_version: "v1"

tests:
  - name: "User API Tests"
    steps:
      - name: "Create user"
        plugin: "http"
        config:
          method: "POST"
          url: "{{ .env.API_BASE_URL }}/{{ .vars.api_version }}/users"
          headers:
            "Authorization": "Bearer {{ .env.API_KEY }}"
            "X-Debug": "{{ .env.DEBUG_MODE }}"
          body: |
            {
              "email": "test@example.com",
              "environment": "{{ .env.ENVIRONMENT | default 'unknown' }}"
            }
          timeout: "{{ .env.REQUEST_TIMEOUT }}s"
          retries: "{{ .env.MAX_RETRIES }}"
```

### Running

```bash
# Load staging environment
rocketship run -af test-suite.yaml --env-file .env.staging
```

## Migration from --var-file

If you're currently using `--var-file` for secrets:

### Before (not recommended for secrets)
```yaml
# vars.yaml
api_key: "sk-secret-key"
database_url: "postgres://..."
```

```bash
rocketship run -af test.yaml --var-file vars.yaml
```

### After (recommended)
```bash
# .env
API_KEY=sk-secret-key
DATABASE_URL=postgres://...
```

```bash
rocketship run -af test.yaml --env-file .env
```

Update your test files:
- Change `{{ .vars.api_key }}` to `{{ .env.API_KEY }}`
- Use UPPER_CASE for environment variables (convention)