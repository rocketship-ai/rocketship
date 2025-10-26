# Supabase Plugin

Test your entire Supabase stack—database, authentication, and storage.

## Quick Start

```yaml
- name: "Query users"
  plugin: supabase
  config:
    url: "{{ .env.SUPABASE_URL }}"
    key: "{{ .env.SUPABASE_SERVICE_KEY }}"
    operation: "select"
    table: "users"
    select:
      columns: ["id", "name", "email"]
      filters:
        - column: "status"
          operator: "eq"
          value: "active"
```

## Database Operations

### SELECT

```yaml
operation: "select"
table: "users"
select:
  columns: ["id", "name", "email"]
  filters:
    - column: "status"
      operator: "eq"
      value: "active"
  order:
    - column: "created_at"
      ascending: false
  limit: 10
  offset: 0
```

### INSERT

```yaml
operation: "insert"
table: "users"
insert:
  data:
    name: "John Doe"
    email: "john@example.com"
save:
  - json_path: ".[0].id"
    as: "user_id"
```

### UPDATE

```yaml
operation: "update"
table: "users"
update:
  data:
    status: "verified"
  filters:
    - column: "id"
      operator: "eq"
      value: "{{ user_id }}"
```

### DELETE

```yaml
operation: "delete"
table: "users"
delete:
  filters:
    - column: "id"
      operator: "eq"
      value: "{{ user_id }}"
```

### Filter Operators

`eq`, `neq`, `gt`, `gte`, `lt`, `lte`, `like`, `ilike`, `is`, `in`

```yaml
filters:
  - column: "status"
    operator: "in"
    value: ["active", "premium"]
  - column: "name"
    operator: "ilike"
    value: "%smith%"
```

### RPC Functions

```yaml
operation: "rpc"
rpc:
  function: "get_user_count"
  params:
    min_age: 18
save:
  - json_path: "."
    as: "user_count"
```

## Authentication

### Sign Up

```yaml
operation: "auth_sign_up"
auth:
  email: "user@example.com"
  password: "SecurePass123!"
  user_metadata:
    name: "John Doe"
save:
  - json_path: ".user.id"
    as: "user_id"
```

### Sign In

```yaml
operation: "auth_sign_in"
auth:
  email: "{{ test_email }}"
  password: "{{ test_password }}"
save:
  - json_path: ".session.access_token"
    as: "access_token"
```

Use extracted tokens in HTTP requests:

```yaml
- name: "Call protected endpoint"
  plugin: http
  config:
    url: "{{ .env.API_URL }}/api/profile"
    headers:
      Authorization: "Bearer {{ access_token }}"
```

### Admin Operations (Service Role)

```yaml
# Create user with auto-confirmation
operation: "auth_create_user"
auth:
  email: "admin@example.com"
  password: "AdminPass123!"
  email_confirm: true
  user_metadata:
    role: "staff"

# Delete user
operation: "auth_delete_user"
auth:
  user_id: "{{ user_id }}"
```

## Storage

### Create Bucket

```yaml
operation: "storage_create_bucket"
storage:
  bucket: "uploads"
  public: true
```

### Upload File

```yaml
operation: "storage_upload"
storage:
  bucket: "uploads"
  path: "documents/test.txt"
  file_content: "Test content"
  content_type: "text/plain"
```

### Download File

```yaml
operation: "storage_download"
storage:
  bucket: "uploads"
  path: "documents/test.txt"
```

## Best Practices

- **Credentials**: Use environment variables for URL and keys
- **Cleanup**: Always delete test data in cleanup hooks
- **Uniqueness**: Generate unique emails with timestamps
- **Keys**: Use service role key for admin operations, anon key for client operations

## See Also

- [Variables](../features/variables.md) - Managing credentials with environment variables
- [Lifecycle Hooks](../features/lifecycle-hooks.md) - Setting up and cleaning up test data
