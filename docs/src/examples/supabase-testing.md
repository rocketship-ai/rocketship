# Supabase Plugin - Full-Stack Database Testing

Test your entire Supabase stack—database CRUD, authentication, storage, and RPC functions—with Rocketship's Supabase plugin.

## Quick Start

```yaml
- name: "Query users"
  plugin: supabase
  config:
    url: "{{ .env.SUPABASE_URL }}"
    key: "{{ .env.SUPABASE_ANON_KEY }}"
    operation: "select"
    table: "users"
    select:
      columns: ["id", "name", "email"]
      filters:
        - column: "status"
          operator: "eq"
          value: "active"
```

## Operations

### CRUD Operations

**SELECT** - Query with filtering, ordering, pagination:
```yaml
select:
  columns: ["id", "name", "email"]
  filters:
    - column: "status"
      operator: "eq"
      value: "active"
    - column: "created_at"
      operator: "gte"
      value: "2024-01-01"
  order:
    - column: "created_at"
      ascending: false
  limit: 10
```

**INSERT** - Create records:
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

**UPDATE** - Modify records:
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

**DELETE** - Remove records (filters required):
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

### Sign In & Token Extraction

Extract tokens for authenticated requests:

```yaml
operation: "auth_sign_in"
auth:
  email: "{{ test_email }}"
  password: "{{ test_password }}"
save:
  - json_path: ".session.access_token"
    as: "access_token"
  - json_path: ".session.refresh_token"
    as: "refresh_token"
```

Use extracted tokens in HTTP requests:
```yaml
- name: "Call protected endpoint"
  plugin: http
  config:
    url: "{{ .env.API_URL }}/api/user/profile"
    headers:
      Authorization: "Bearer {{ access_token }}"
```

### Admin Operations (Service Role)

```yaml
# Create user with auto-confirmation
operation: "auth_create_user"
auth:
  email: "admin-created@example.com"
  password: "AdminPass123!"
  email_confirm: true  # Auto-confirm email
  user_metadata:
    role: "staff"

# Delete user
operation: "auth_delete_user"
auth:
  user_id: "{{ user_id }}"
```

## Storage Operations

**Create bucket:**
```yaml
operation: "storage_create_bucket"
storage:
  bucket: "uploads"
  public: true
```

**Upload file:**
```yaml
operation: "storage_upload"
storage:
  bucket: "uploads"
  path: "documents/test.txt"
  file_content: "Test content"
  content_type: "text/plain"
```

**Download file:**
```yaml
operation: "storage_download"
storage:
  bucket: "uploads"
  path: "documents/test.txt"
```

## Assertions

**Check existence** - Use `save` instead of assertions (fails if path doesn't exist):
```yaml
save:
  - json_path: ".user.id"
    as: "user_id"  # Automatically validates existence
```

**Exact value match:**
```yaml
assertions:
  - type: json_path
    path: ".user.email"
    expected: "test@example.com"
  - type: json_path
    path: ".session.token_type"
    expected: "bearer"
```

**Check array length:**
```yaml
assertions:
  - type: json_path
    path: "length"
    expected: 5
```

## Complete E2E Example

```yaml
name: "User Authentication Flow"

tests:
  - name: "Complete user journey"
    steps:
      # Generate unique credentials
      - name: "Generate credentials"
        plugin: script
        config:
          language: javascript
          script: |
            const suffix = Date.now();
            save("test_email", `test-${suffix}@example.com`);
            save("test_password", `Pass${suffix}!`);

      # Create user
      - name: "Create user"
        plugin: supabase
        config:
          url: "{{ .env.SUPABASE_URL }}"
          key: "{{ .env.SUPABASE_SERVICE_KEY }}"
          operation: "auth_create_user"
          auth:
            email: "{{ test_email }}"
            password: "{{ test_password }}"
            email_confirm: true
        save:
          - json_path: ".user.id"
            as: "user_id"

      # Sign in
      - name: "Authenticate"
        plugin: supabase
        config:
          url: "{{ .env.SUPABASE_URL }}"
          key: "{{ .env.SUPABASE_ANON_KEY }}"
          operation: "auth_sign_in"
          auth:
            email: "{{ test_email }}"
            password: "{{ test_password }}"
        save:
          - json_path: ".session.access_token"
            as: "access_token"

      # Use token in HTTP request
      - name: "Access protected endpoint"
        plugin: http
        config:
          method: "GET"
          url: "{{ .env.API_URL }}/api/user/profile"
          headers:
            Authorization: "Bearer {{ access_token }}"
        assertions:
          - type: status_code
            expected: 200

      # Cleanup
      - name: "Delete user"
        plugin: supabase
        config:
          url: "{{ .env.SUPABASE_URL }}"
          key: "{{ .env.SUPABASE_SERVICE_KEY }}"
          operation: "auth_delete_user"
          auth:
            user_id: "{{ user_id }}"
```

## Best Practices

1. **Use environment variables for credentials:**
   ```yaml
   vars:
     supabase_url: "{{ .env.SUPABASE_URL }}"
     supabase_anon_key: "{{ .env.SUPABASE_ANON_KEY }}"
     supabase_service_key: "{{ .env.SUPABASE_SERVICE_KEY }}"
   ```

2. **Always clean up test data:**
   ```yaml
   - name: "Cleanup"
     plugin: supabase
     config:
       operation: "delete"
       table: "test_data"
       delete:
         filters:
           - column: "created_by"
             operator: "eq"
             value: "integration_test"
   ```

3. **Generate unique test data:**
   ```yaml
   script: |
     const suffix = Date.now();
     save("email", `test-${suffix}@example.com`);
   ```

4. **Test RLS policies** by using different keys (anon vs service role)

## Troubleshooting

Enable debug logging for detailed operation information:
```bash
ROCKETSHIP_LOG=DEBUG rocketship run -af your-test.yaml
```

**Common Issues:**

- **Invalid credentials**: Verify email/password, check if email confirmation is required
- **Permission errors**: Check RLS policies, use service role key for admin operations
- **Table not found**: Verify table exists and you have proper permissions
- **Email already registered**: Use unique emails with timestamps for each test run

See the [full example](../../examples/supabase-testing/rocketship.yaml) for comprehensive test coverage.
