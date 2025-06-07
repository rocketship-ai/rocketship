# Supabase Plugin

The Supabase plugin enables comprehensive testing of Supabase applications, including database operations, RPC function calls, authentication, and storage operations.

## Overview

This plugin provides a unified interface for testing all aspects of a Supabase backend:

- **Database Operations**: SELECT, INSERT, UPDATE, DELETE queries with advanced filtering
- **RPC Functions**: Call custom PostgreSQL functions with parameters
- **Authentication**: User management operations (create, update, delete users)
- **Storage**: Bucket and file operations (create, upload, download, delete)

## Configuration

### Basic Configuration

```yaml
- name: "Test step"
  plugin: "supabase"
  config:
    url: "{{ SUPABASE_URL }}"                    # Your Supabase project URL
    anon_key: "{{ SUPABASE_ANON_KEY }}"          # Anonymous/public API key
    service_key: "{{ SUPABASE_SERVICE_KEY }}"    # Service role key (for admin operations)
    operation: "select"                          # Operation type
    table: "users"                               # Table name (for database operations)
```

### Environment Variables

Set these environment variables or use them in your test configuration:

- `SUPABASE_URL`: Your Supabase project URL (e.g., `https://abc123.supabase.co`)
- `SUPABASE_ANON_KEY`: Your project's anonymous/public API key
- `SUPABASE_SERVICE_KEY`: Your project's service role key (for admin operations)

## Operations

### Database Operations

#### SELECT

```yaml
- name: "Get users"
  plugin: "supabase"
  config:
    url: "{{ SUPABASE_URL }}"
    anon_key: "{{ SUPABASE_ANON_KEY }}"
    operation: "select"
    table: "users"
    columns: "id,name,email"              # Columns to select (default: *)
    filters:                              # Optional filters
      - column: "status"
        operator: "eq"
        value: "active"
      - column: "age"
        operator: "gte"
        value: 18
    order:                                # Optional ordering
      - column: "created_at"
        ascending: false
    limit: 10                             # Optional limit
    offset: 0                             # Optional offset
    count: "exact"                        # Optional: exact, planned, estimated
    single: true                          # Return single object instead of array
```

**Available Filter Operators:**
- `eq` - Equal to
- `neq` - Not equal to
- `gt` - Greater than
- `gte` - Greater than or equal to
- `lt` - Less than
- `lte` - Less than or equal to
- `like` - Pattern matching (case sensitive)
- `ilike` - Pattern matching (case insensitive)
- `is` - Checking for exact values (including null)
- `in` - Matches any of the values
- `contains` - Contains (for arrays/JSON)
- `contained_by` - Contained by (for arrays/JSON)

#### INSERT

```yaml
- name: "Create user"
  plugin: "supabase"
  config:
    url: "{{ SUPABASE_URL }}"
    anon_key: "{{ SUPABASE_ANON_KEY }}"
    operation: "insert"
    table: "users"
    data: |
      {
        "name": "John Doe",
        "email": "john@example.com",
        "age": 30
      }
    upsert: true                          # Optional: enable upsert mode
    on_conflict: "email"                  # Required for upsert: conflict resolution column
```

**Bulk Insert:**
```yaml
data: |
  [
    {"name": "John", "email": "john@example.com"},
    {"name": "Jane", "email": "jane@example.com"}
  ]
```

#### UPDATE

```yaml
- name: "Update user"
  plugin: "supabase"
  config:
    url: "{{ SUPABASE_URL }}"
    anon_key: "{{ SUPABASE_ANON_KEY }}"
    operation: "update"
    table: "users"
    data: |
      {
        "name": "John Smith",
        "updated_at": "now()"
      }
    filters:                              # Required: specify which rows to update
      - column: "id"
        operator: "eq"
        value: "{{ user_id }}"
```

#### DELETE

```yaml
- name: "Delete user"
  plugin: "supabase"
  config:
    url: "{{ SUPABASE_URL }}"
    anon_key: "{{ SUPABASE_ANON_KEY }}"
    operation: "delete"
    table: "users"
    filters:                              # Required: specify which rows to delete
      - column: "id"
        operator: "eq"
        value: "{{ user_id }}"
```

### RPC Functions

Call custom PostgreSQL functions:

```yaml
- name: "Call RPC function"
  plugin: "supabase"
  config:
    url: "{{ SUPABASE_URL }}"
    anon_key: "{{ SUPABASE_ANON_KEY }}"
    operation: "rpc"
    rpc:
      function: "get_user_stats"
      params:
        user_id: 123
        start_date: "2024-01-01"
      get: false                          # Optional: suppress data return
      head: false                         # Optional: read-only access mode
```

### Authentication Operations

Authentication operations require the `service_key`:

#### Create User

```yaml
- name: "Create auth user"
  plugin: "supabase"
  config:
    url: "{{ SUPABASE_URL }}"
    service_key: "{{ SUPABASE_SERVICE_KEY }}"
    operation: "auth"
    auth:
      action: "create_user"
      email: "user@example.com"
      password: "password123"
      phone: "+1234567890"               # Optional
      email_confirm: true                # Auto-confirm email
      phone_confirm: false               # Auto-confirm phone
      user_metadata:                     # Optional metadata
        first_name: "John"
        last_name: "Doe"
```

#### Update User

```yaml
- name: "Update auth user"
  plugin: "supabase"
  config:
    url: "{{ SUPABASE_URL }}"
    service_key: "{{ SUPABASE_SERVICE_KEY }}"
    operation: "auth"
    auth:
      action: "update_user"
      user_id: "{{ auth_user_id }}"
      email: "newemail@example.com"      # Optional
      password: "newpassword"            # Optional
      user_metadata:                     # Optional
        department: "Engineering"
```

#### Delete User

```yaml
- name: "Delete auth user"
  plugin: "supabase"
  config:
    url: "{{ SUPABASE_URL }}"
    service_key: "{{ SUPABASE_SERVICE_KEY }}"
    operation: "auth"
    auth:
      action: "delete_user"
      user_id: "{{ auth_user_id }}"
      soft_delete: false                 # Optional: soft delete vs hard delete
```

### Storage Operations

#### Create Bucket

```yaml
- name: "Create storage bucket"
  plugin: "supabase"
  config:
    url: "{{ SUPABASE_URL }}"
    service_key: "{{ SUPABASE_SERVICE_KEY }}"
    operation: "storage"
    storage:
      action: "create_bucket"
      bucket: "avatars"
      public: false
      allowed_mime_types:
        - "image/jpeg"
        - "image/png"
      file_size_limit: 1048576           # 1MB in bytes
```

#### Upload File

```yaml
- name: "Upload file"
  plugin: "supabase"
  config:
    url: "{{ SUPABASE_URL }}"
    anon_key: "{{ SUPABASE_ANON_KEY }}"
    operation: "storage"
    storage:
      action: "upload"
      bucket: "avatars"
      path: "users/{{ user_id }}/avatar.jpg"
      file_content: "{{ base64_encoded_content }}"  # Direct content
      # OR
      file_path: "/path/to/local/file.jpg"          # Local file path (not yet implemented)
      cache_control: "3600"
      upsert: true                       # Overwrite if exists
```

#### List Files

```yaml
- name: "List files"
  plugin: "supabase"
  config:
    url: "{{ SUPABASE_URL }}"
    anon_key: "{{ SUPABASE_ANON_KEY }}"
    operation: "storage"
    storage:
      action: "list"
      bucket: "avatars"
      path: "users/"                     # Optional: folder path
      limit: 100                         # Optional
      offset: 0                          # Optional
```

#### Delete File

```yaml
- name: "Delete file"
  plugin: "supabase"
  config:
    url: "{{ SUPABASE_URL }}"
    anon_key: "{{ SUPABASE_ANON_KEY }}"
    operation: "storage"
    storage:
      action: "delete"
      bucket: "avatars"
      path: "users/123/avatar.jpg"
```

#### Other Storage Operations

```yaml
# Get bucket info
storage:
  action: "get_bucket"
  bucket: "avatars"

# Update bucket settings
storage:
  action: "update_bucket"
  bucket: "avatars"
  public: true
  file_size_limit: 2097152             # 2MB

# Empty bucket
storage:
  action: "empty_bucket"
  bucket: "avatars"

# Delete bucket
storage:
  action: "delete_bucket"
  bucket: "avatars"
```

## Assertions

### Row Count Assertions

```yaml
assertions:
  - type: "row_count"
    expected: 5
    operator: "eq"                      # eq, neq, gt, gte, lt, lte
```

### Field Assertions

For single-row responses or first row of multi-row responses:

```yaml
assertions:
  - type: "field"
    field: "name"
    expected: "John Doe"
    operator: "eq"                      # eq, neq, contains, exists
  - type: "field"
    field: "email"
    exists: true                        # Just check if field exists
```

### JSON Path Assertions

Use JQ expressions for complex data extraction:

```yaml
assertions:
  - type: "json_path"
    path: ".[] | select(.status == \"active\") | length"
    expected: 3
  - type: "json_path"
    path: ".[0].user.profile.name"
    expected: "{{ expected_name }}"
  - type: "json_path"
    path: ".metadata"
    exists: true                        # Check if path exists
```

## Save Configuration

### Save Field Values

```yaml
save:
  - field: "id"                         # Field name from response
    as: "user_id"                       # Variable name to save as
    required: true                      # Whether value is required (default: true)
```

### Save with JSON Path

```yaml
save:
  - json_path: ".[0].id"               # JQ expression
    as: "first_user_id"
  - json_path: ".users | length"
    as: "user_count"
    required: false                     # Optional value
```

## Variable Replacement

All string fields support variable replacement using `{{ variable_name }}` syntax:

```yaml
config:
  url: "https://{{ project_id }}.supabase.co"
  table: "users_{{ environment }}"
  filters:
    - column: "id"
      operator: "eq"
      value: "{{ user_id }}"
  data: |
    {
      "name": "{{ user_name }}",
      "organization": "{{ org_name }}"
    }
```

## Error Handling

The plugin automatically handles Supabase API errors and provides detailed error messages:

```yaml
# If an error occurs, the step will fail with details like:
# "Supabase operation failed: {"message": "Invalid API key", "code": "PGRST301"}"
```

## Complete Example

See `/examples/supabase-testing/rocketship.yaml` for a comprehensive example that demonstrates:

- Database CRUD operations
- RPC function calls
- Authentication management
- Storage operations
- Advanced filtering and assertions
- Variable chaining between steps

## Best Practices

1. **Use Environment Variables**: Store sensitive keys in environment variables
2. **Proper Cleanup**: Always clean up test data in your test suites
3. **Row-Level Security**: Ensure your RLS policies allow test operations
4. **Error Handling**: Use assertions to verify expected behavior
5. **Variable Chaining**: Save IDs from create operations to use in subsequent steps
6. **Service Key Security**: Only use service_key for admin operations and never expose it in client-side code

## Limitations

1. **File Uploads**: Currently only supports direct content upload, not local file paths
2. **Binary Data**: File content should be properly encoded for upload
3. **Authentication**: Only supports admin auth operations, not client-side auth flows
4. **Real-time**: Does not support Supabase real-time subscriptions
5. **Edge Functions**: Does not support Supabase Edge Functions (use HTTP plugin instead)