# SQL Plugin

Execute SQL queries and validate database state in your tests.

## Supported Databases

| Database | Driver | DSN Format |
|----------|--------|------------|
| PostgreSQL | `postgres` | `postgres://user:pass@host:port/db?sslmode=disable` |
| MySQL | `mysql` | `user:pass@tcp(host:port)/db` |
| SQLite | `sqlite` | `./path/to/database.db` |
| SQL Server | `sqlserver` | `sqlserver://user:pass@host:port?database=db` |

## Quick Start

```yaml
- name: "Query database"
  plugin: sql
  config:
    driver: postgres
    dsn: "{{ .env.DATABASE_URL }}"
    commands:
      - "SELECT id, name FROM users WHERE active = true;"
  assertions:
    - type: row_count
      query_index: 0
      expected: 5
  save:
    - sql_result: ".queries[0].rows[0].id"
      as: "user_id"
```

## Configuration

### Required Fields

| Field | Description | Example |
|-------|-------------|---------|
| `driver` | Database driver | `postgres`, `mysql`, `sqlite`, `sqlserver` |
| `dsn` | Database connection string | `postgres://user:pass@host:port/db` |

### Optional Fields

| Field | Description | Example |
|-------|-------------|---------|
| `commands` | Array of SQL statements | `["SELECT * FROM users;"]` |
| `file` | Path to SQL file | `./migrations/001_create.sql` |
| `timeout` | Query execution timeout | `60s` |

Note: Must provide either `commands` or `file`, not both.

## Assertions

### Row Count

Validate number of rows returned by a query:

```yaml
assertions:
  - type: row_count
    query_index: 0  # Index of the query in commands array
    expected: 3
```

### Query Count

Validate total number of queries executed:

```yaml
assertions:
  - type: query_count
    expected: 5
```

### Success Count

Validate number of successful queries:

```yaml
assertions:
  - type: success_count
    expected: 2
```

### Column Value

Validate specific column value:

```yaml
assertions:
  - type: column_value
    query_index: 0
    row_index: 0
    column: "status"
    expected: "active"
```

## Save Fields

Extract values from query results:

### From Rows

```yaml
save:
  - sql_result: ".queries[0].rows[0].id"
    as: "user_id"
  - sql_result: ".queries[0].rows[1].email"
    as: "user_email"
```

### From Statistics

```yaml
save:
  - sql_result: ".queries[0].rows_affected"
    as: "affected_count"
  - sql_result: ".stats.success_count"
    as: "successful_queries"
  - sql_result: ".stats.total_queries"
    as: "total_queries"
```

## Common Patterns

### CRUD Operations

```yaml
steps:
  # Create
  - name: "Insert user"
    plugin: sql
    config:
      driver: postgres
      dsn: "{{ .env.DATABASE_URL }}"
      commands:
        - "INSERT INTO users (name, email) VALUES ('Test', 'test@example.com') RETURNING id;"
    save:
      - sql_result: ".queries[0].rows[0].id"
        as: "user_id"

  # Read
  - name: "Query user"
    plugin: sql
    config:
      driver: postgres
      dsn: "{{ .env.DATABASE_URL }}"
      commands:
        - "SELECT * FROM users WHERE id = {{ user_id }};"
    assertions:
      - type: row_count
        query_index: 0
        expected: 1

  # Update
  - name: "Update user"
    plugin: sql
    config:
      driver: postgres
      dsn: "{{ .env.DATABASE_URL }}"
      commands:
        - "UPDATE users SET active = false WHERE id = {{ user_id }};"

  # Delete
  - name: "Delete user"
    plugin: sql
    config:
      driver: postgres
      dsn: "{{ .env.DATABASE_URL }}"
      commands:
        - "DELETE FROM users WHERE id = {{ user_id }};"
```

### Multiple Commands

Execute multiple SQL statements in one step:

```yaml
- name: "Setup test data"
  plugin: sql
  config:
    driver: postgres
    dsn: "{{ .env.DATABASE_URL }}"
    commands:
      - "DELETE FROM test_users WHERE email LIKE '%@test.com';"
      - "INSERT INTO test_users (name, email) VALUES ('User 1', 'user1@test.com');"
      - "INSERT INTO test_users (name, email) VALUES ('User 2', 'user2@test.com');"
  assertions:
    - type: query_count
      expected: 3
    - type: success_count
      expected: 3
```

### External SQL Files

Use external SQL files for complex queries or migrations:

```yaml
- name: "Run migration"
  plugin: sql
  config:
    driver: postgres
    dsn: "{{ .env.DATABASE_URL }}"
    file: "./migrations/001_create_tables.sql"
    timeout: "60s"
```

### Using Variables

```yaml
vars:
  table_name: "users"
  min_age: 18

steps:
  - name: "Dynamic query"
    plugin: sql
    config:
      driver: postgres
      dsn: "{{ .env.DATABASE_URL }}"
      commands:
        - "SELECT * FROM {{ .vars.table_name }} WHERE age >= {{ .vars.min_age }};"
```

## Best Practices

- **Security**: Store DSN in environment variables, never commit credentials
- **Performance**: Set appropriate timeouts for long queries
- **Isolation**: Clean up test data in cleanup hooks
- **Assertions**: Validate both success and error scenarios

## See Also

- [Variables](../features/variables.md) - Using environment variables for credentials
- [Lifecycle Hooks](../features/lifecycle-hooks.md) - Setting up and tearing down databases
