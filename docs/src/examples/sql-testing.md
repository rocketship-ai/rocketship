# SQL Testing

The SQL plugin enables database operations and testing within Rocketship workflows. It supports multiple database engines and provides comprehensive assertion capabilities for validating query results.

## Supported Databases

- **PostgreSQL** - `driver: postgres`
- **MySQL** - `driver: mysql`
- **SQLite** - `driver: sqlite`
- **SQL Server** - `driver: sqlserver`

## Configuration

### Basic Configuration

```yaml
- name: "Query users"
  plugin: sql
  config:
    driver: postgres
    dsn: "postgres://user:password@localhost:5432/database?sslmode=disable"
    commands:
      - "SELECT id, name, email FROM users WHERE active = true;"
```

### Configuration with Variables

```yaml
vars:
  db_host: "localhost:5432"
  db_user: "testuser"
  db_password: "testpass"
  db_name: "testdb"

steps:
  - name: "Create user"
    plugin: sql
    config:
      driver: postgres
      dsn: "postgres://{{ .vars.db_user }}:{{ .vars.db_password }}@{{ .vars.db_host }}/{{ .vars.db_name }}?sslmode=disable"
      commands:
        - "INSERT INTO users (name, email) VALUES ('{{ .vars.user_name }}', '{{ .vars.user_email }}') RETURNING id;"
```

### External SQL Files

```yaml
- name: "Run migration"
  plugin: sql
  config:
    driver: postgres
    dsn: "{{ .vars.postgres_dsn }}"
    file: "./migrations/001_create_tables.sql"
    timeout: "60s"
```

## Database Connection Strings (DSN)

### PostgreSQL

```
postgres://username:password@host:port/database?sslmode=disable
```

### MySQL

```
username:password@tcp(host:port)/database
```

### SQLite

```
./path/to/database.db
```

### SQL Server

```
sqlserver://username:password@host:port?database=dbname
```

## Assertions

The SQL plugin supports several assertion types for validating query results:

### Row Count Assertion

Validates the number of rows returned by a specific query:

```yaml
assertions:
  - type: row_count
    query_index: 0
    expected: 5
```

### Query Count Assertion

Validates the total number of queries executed:

```yaml
assertions:
  - type: query_count
    expected: 3
```

### Success Count Assertion

Validates the number of successful queries:

```yaml
assertions:
  - type: success_count
    expected: 2
```

### Column Value Assertion

Validates specific column values in query results:

```yaml
assertions:
  - type: column_value
    query_index: 0
    row_index: 0
    column: "status"
    expected: "active"
```

## Saving Query Results

Extract values from query results for use in subsequent steps:

```yaml
save:
  - sql_result: ".queries[0].rows[0].id"
    as: "user_id"
  - sql_result: ".queries[0].rows_affected"
    as: "affected_count"
  - sql_result: ".stats.success_count"
    as: "successful_queries"
```

### Save Path Syntax

- `.queries[0].rows[0].column_name` - Extract column value from first query, first row
- `.queries[0].rows_affected` - Number of rows affected by first query
- `.stats.success_count` - Total number of successful queries
- `.stats.total_queries` - Total number of queries executed

## Handlebars Escaping in SQL Queries

When your SQL queries contain literal handlebars syntax (e.g., for stored procedures or database functions that use `{{ }}` syntax), you can escape them using backslashes:

```yaml
- name: "Query with escaped handlebars"
  plugin: sql
  config:
    driver: postgres
    dsn: "{{ .vars.db_dsn }}"
    commands:
      - "SELECT 'Normal: {{ .vars.test_user_name }}, Escaped: \\{{ placeholder }}' as mixed_example;"
```

In this example:

- `{{ .vars.test_user_name }}` will be replaced with the actual variable value
- `\\{{ placeholder }}` will render as literal `{{ placeholder }}` in the SQL query

For multiple levels of escaping:

- `\\{{ }}` → `{{ }}` (literal handlebars)
- `\\\\{{ }}` → `\\{{ }}` (escaped backslash + handlebars variable)
- `\\\\\\{{ }}` → `\\{{ }}` (literal escaped handlebars)

See the Handlebars Escaping section in (variables.md) for complete details and advanced usage.

## Complete Example

```yaml
name: "User Management Test"
version: "v1.0.0"

vars:
  db_dsn: "postgres://testuser:testpass@localhost:5433/testdb?sslmode=disable"
  test_email: "test@example.com"

tests:
  - name: "User CRUD Operations"
    steps:
      - name: "Create user"
        plugin: sql
        config:
          driver: postgres
          dsn: "{{ .vars.db_dsn }}"
          commands:
            - "INSERT INTO users (name, email, active) VALUES ('Test User', '{{ .vars.test_email }}', true) RETURNING id;"
        assertions:
          - type: row_count
            query_index: 0
            expected: 1
        save:
          - sql_result: ".queries[0].rows[0].id"
            as: "user_id"

      - name: "Verify user exists"
        plugin: sql
        config:
          driver: postgres
          dsn: "{{ .vars.db_dsn }}"
          commands:
            - "SELECT id, name, email, active FROM users WHERE id = {{ user_id }};"
        assertions:
          - type: row_count
            query_index: 0
            expected: 1
          - type: column_value
            query_index: 0
            row_index: 0
            column: "email"
            expected: "{{ .vars.test_email }}"
          - type: column_value
            query_index: 0
            row_index: 0
            column: "active"
            expected: true

      - name: "Update user status"
        plugin: sql
        config:
          driver: postgres
          dsn: "{{ .vars.db_dsn }}"
          commands:
            - "UPDATE users SET active = false WHERE id = {{ user_id }};"
        assertions:
          - type: success_count
            expected: 1

      - name: "Delete user"
        plugin: sql
        config:
          driver: postgres
          dsn: "{{ .vars.db_dsn }}"
          commands:
            - "DELETE FROM users WHERE id = {{ user_id }};"
        assertions:
          - type: success_count
            expected: 1
```

## Testing with Docker

For local testing, use the provided Docker Compose setup:

```bash
# Start test databases
cd .docker && docker-compose up postgres-test mysql-test -d

# Run SQL tests
rocketship run -af examples/sql-testing/rocketship.yaml
```

The test databases include:

- **PostgreSQL**: `localhost:5433` with sample data
- **MySQL**: `localhost:3307` with sample data

## Best Practices

### Security

- Use variables for connection strings to avoid hardcoding credentials
- Use least-privilege database users for testing
- Never commit real database credentials to version control

### Performance

- Set appropriate timeouts for long-running queries
- Use connection pooling (handled automatically by the plugin)
- Test with realistic data volumes

### Testing Strategy

- Test both successful and error scenarios
- Validate data integrity with assertions
- Use transactions when testing modifications
- Clean up test data to maintain test isolation

### Error Handling

```yaml
- name: "Handle expected errors"
  plugin: sql
  config:
    driver: postgres
    dsn: "{{ .vars.db_dsn }}"
    commands:
      - "SELECT * FROM nonexistent_table;"
  # This step will fail, which might be expected for negative testing
```

## Troubleshooting

### Connection Issues

- Verify database service is running
- Check connection string format for your database type
- Ensure network connectivity and firewall settings
- Validate credentials and database permissions

### Query Errors

- Check SQL syntax for your specific database
- Verify table and column names exist
- Ensure proper data types in INSERT/UPDATE operations
- Review database logs for detailed error messages

### Assertion Failures

- Verify expected values match actual query results
- Check query indices and row indices in assertions
- Ensure column names are spelled correctly
- Review query results in logs for debugging
