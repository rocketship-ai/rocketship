# Environment Variables

Access system environment variables in your tests using `{{ .env.VARIABLE_NAME }}` syntax.

## Basic Usage

```yaml
- name: "API request with environment variables"
  plugin: "http"
  config:
    method: "POST"
    url: "{{ .env.API_BASE_URL }}/users"
    headers:
      "Authorization": "Bearer {{ .env.API_TOKEN }}"
      "X-User": "{{ .env.USER }}"
    body: |-
      {
        "username": "{{ .env.USER }}",
        "api_key": "{{ .env.API_KEY }}"
      }
```

## SQL Connections

```yaml
- name: "Database query"
  plugin: "sql"
  config:
    driver: "postgres"
    dsn: "postgres://{{ .env.DB_USER }}:{{ .env.DB_PASSWORD }}@{{ .env.DB_HOST }}/{{ .env.DB_NAME }}"
    commands:
      - "SELECT * FROM users WHERE created_by = '{{ .env.USER }}';"
```

## Mixed with Other Variables

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

## Escaping

Environment variables support handlebars escaping:

```yaml
body: |-
  {
    "user": "{{ .env.USER }}",
    "docs": "Set \\{{ .env.API_KEY }} to configure"
  }
```

## Setting Environment Variables

```bash
# Command line
API_TOKEN=your_token rocketship run -af test.yaml

# Export for session
export API_TOKEN=your_token
export DB_URL=postgres://user:pass@localhost/db
rocketship run -af test.yaml
```

## Common Variables

```yaml
"{{ .env.USER }}"         # Current username
"{{ .env.HOME }}"         # Home directory
"{{ .env.API_KEY }}"      # API key
"{{ .env.API_TOKEN }}"    # Bearer token
"{{ .env.DATABASE_URL }}" # Database connection
"{{ .env.NODE_ENV }}"     # Environment name
```

Missing environment variables are treated as empty strings.