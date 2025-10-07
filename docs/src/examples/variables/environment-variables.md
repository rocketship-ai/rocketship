# Environment Variables

Environment variables inject secrets and environment-specific configuration into tests without hardcoding sensitive values.

**Syntax:** `{{ .env.VARIABLE_NAME }}`

## Using --env-file

Create a `.env` file:

```bash
# .env
API_BASE_URL=https://api.staging.com
API_KEY=sk-staging-key-123
DATABASE_URL=postgres://user:pass@localhost/db
SUPABASE_ANON_KEY=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

Run tests:

```bash
rocketship run -af test.yaml --env-file .env
```

Use in tests:

```yaml
- name: "API request with auth"
  plugin: "http"
  config:
    url: "{{ .env.API_BASE_URL }}/users"
    headers:
      "Authorization": "Bearer {{ .env.API_KEY }}"
```

## Multi-Environment Setup

Organize files per environment:

```
project/
├── .env.example      # Template with dummy values (commit this)
├── .env              # Local development (gitignore)
├── .env.staging      # Staging values (gitignore)
├── .env.production   # Production values (gitignore)
└── tests/api-tests.yaml
```

**`.env.example`** (commit this):
```bash
# API Configuration
API_BASE_URL=https://api.example.com
API_KEY=your-api-key-here
DATABASE_URL=postgres://user:pass@host/db
```

**Usage:**
```bash
# Local
rocketship run -af test.yaml --env-file .env

# Staging
rocketship run -af test.yaml --env-file .env.staging

# Production
rocketship run -af test.yaml --env-file .env.production
```

## CI/CD Integration

### GitHub Actions

```yaml
# .github/workflows/test.yml
- name: Run tests
  env:
    API_KEY: ${{ secrets.API_KEY }}
    DATABASE_URL: ${{ secrets.DATABASE_URL }}
    SUPABASE_ANON_KEY: ${{ secrets.SUPABASE_ANON_KEY }}
  run: rocketship run -af tests/integration.yaml
```

Secrets are automatically available as environment variables—no `--env-file` needed.

### GitLab CI

```yaml
# .gitlab-ci.yml
test:
  script: rocketship run -af tests/integration.yaml
  variables:
    API_KEY: $CI_API_KEY
    DATABASE_URL: $CI_DATABASE_URL
```

### Environment Variables in CI

In CI, **don't use `--env-file`**:
1. Set secrets in your CI platform (GitHub Secrets, GitLab Variables)
2. Reference them in workflow configuration
3. Rocketship picks them up from system environment automatically

## Security Best Practices

### Gitignore Environment Files

```bash
# .gitignore
.env
.env.*
!.env.example
```

### Use .env.example as Template

```bash
# .env.example
API_KEY=your-api-key-here
DATABASE_URL=postgres://user:pass@localhost/db
OPENAI_API_KEY=sk-...
```

Team members copy and fill in values:
```bash
cp .env.example .env
# Edit .env with actual credentials
```

### Validate Required Variables

```yaml
- name: "Validate required env vars"
  plugin: "script"
  config:
    language: "javascript"
    script: |
      const required = ['API_KEY', 'DATABASE_URL', 'SUPABASE_ANON_KEY'];
      const missing = required.filter(key => !process.env[key]);
      if (missing.length > 0) {
        throw new Error(`Missing required env vars: ${missing.join(', ')}`);
      }
```

## Examples

### API Testing

```yaml
vars:
  api_version: "v1"

tests:
  - name: "User API with environment config"
    steps:
      - name: "Create user"
        plugin: "http"
        config:
          url: "{{ .env.API_BASE_URL }}/{{ .vars.api_version }}/users"
          method: "POST"
          headers:
            "Authorization": "Bearer {{ .env.API_TOKEN }}"
          body: |
            {
              "email": "test@example.com",
              "name": "Test User"
            }
```

### Database Testing

```yaml
tests:
  - name: "Database connection"
    steps:
      - name: "Query users"
        plugin: "sql"
        config:
          driver: "postgres"
          dsn: "{{ .env.DATABASE_URL }}"
          query: "SELECT * FROM users WHERE active = true LIMIT 10"
```

### Supabase Testing

```yaml
vars:
  supabase_url: "https://myproject.supabase.co"

tests:
  - name: "Supabase operations"
    steps:
      - name: "Query companies"
        plugin: "supabase"
        config:
          url: "{{ .vars.supabase_url }}"
          key: "{{ .env.SUPABASE_ANON_KEY }}"
          operation: "select"
          table: "companies"
          select:
            columns: ["id", "name"]
            limit: 10
```

### Multi-Environment

```yaml
tests:
  - name: "Environment-aware test"
    steps:
      - name: "Health check"
        plugin: "http"
        config:
          url: "{{ .env.API_BASE_URL }}/health"
        assertions:
          - type: "status_code"
            expected: 200

      - name: "Authenticated request"
        plugin: "http"
        config:
          url: "{{ .env.API_BASE_URL }}/api/data"
          headers:
            "Authorization": "Bearer {{ .env.API_TOKEN }}"
```

Run against different environments:
```bash
# Local
rocketship run -af test.yaml --env-file .env

# Staging
rocketship run -af test.yaml --env-file .env.staging

# Production
rocketship run -af test.yaml --env-file .env.production
```

## Common Patterns

### Build URLs from Parts

```yaml
config:
  url: "{{ .env.DB_PROTOCOL }}://{{ .env.DB_HOST }}:{{ .env.DB_PORT }}/{{ .env.DB_NAME }}"
```

### Environment-Specific Test Data

```yaml
- name: "Create test user"
  plugin: "http"
  config:
    url: "{{ .env.API_BASE_URL }}/users"
    method: "POST"
    body: |
      {
        "email": "{{ .env.TEST_USER_EMAIL }}",
        "environment": "{{ .env.ENVIRONMENT_NAME }}"
      }
```

```bash
# .env.staging
TEST_USER_EMAIL=staging-test@example.com
ENVIRONMENT_NAME=staging

# .env.production
TEST_USER_EMAIL=prod-test@example.com
ENVIRONMENT_NAME=production
```
