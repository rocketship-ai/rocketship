# Supabase Plugin - Full-Stack Database Testing

The Supabase plugin enables comprehensive testing of Supabase applications, providing coverage of database operations, authentication, storage, and PostgreSQL RPC functions. Test your entire Supabase stack from database CRUD operations to file storage and user authentication workflows.

## Key Features

- **CRUD Operations** - Create, read, update, and delete data with advanced filtering
- **PostgreSQL RPC Functions** - Call stored procedures and custom database functions
- **Authentication Testing** - User signup, signin, and session management
- **Storage Operations** - File upload, download, and bucket management
- **Advanced Filtering** - 15+ filter operators including like, in, range queries
- **JSON Path Extraction** - Save and chain data between test steps
- **Row Level Security** - Test RLS policies and permissions
- **Real-time Features** - Test subscriptions and live data updates

## Prerequisites

Before using the Supabase plugin, you need:

1. **Supabase Project** - Create a project at [supabase.com](https://supabase.com)
2. **Project Credentials** - Your project URL and API keys
3. **Database Schema** - Tables and functions set up for testing

```bash
# Get your project details from Supabase Dashboard
PROJECT_URL="https://your-project.supabase.co"
ANON_KEY="your-anon-key"
SERVICE_KEY="your-service-role-key"  # For admin operations
```

## Basic Configuration

```yaml
plugin: supabase
config:
  url: "https://your-project.supabase.co"
  key: "your-anon-key"
  operation: "select" # Required: CRUD or special operation
  table: "users" # Required for CRUD operations
```

## CRUD Operations

### SELECT - Reading Data

Query data with filtering, ordering, and pagination:

```yaml
- name: "Query active users"
  plugin: supabase
  config:
    url: "{{ .vars.supabase_url }}"
    key: "{{ .vars.supabase_anon_key }}"
    operation: "select"
    table: "users"
    select:
      columns: ["id", "name", "email", "created_at"]
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
      offset: 0
      count: "exact" # Get total count
```

### INSERT - Creating Data

Create new records with optional upsert handling:

```yaml
- name: "Create new user"
  plugin: supabase
  config:
    url: "{{ .vars.supabase_url }}"
    key: "{{ .vars.supabase_anon_key }}"
    operation: "insert"
    table: "users"
    insert:
      data:
        name: "John Doe"
        email: "john@example.com"
        metadata:
          role: "user"
          preferences: { "theme": "dark" }
      upsert: false # Set to true for upsert behavior
  save:
    - json_path: ".[0].id"
      as: "new_user_id"
```

### UPDATE - Modifying Data

Update existing records with conditional filters:

```yaml
- name: "Update user status"
  plugin: supabase
  config:
    url: "{{ .vars.supabase_url }}"
    key: "{{ .vars.supabase_anon_key }}"
    operation: "update"
    table: "users"
    update:
      data:
        status: "verified"
        updated_at: "2024-01-15T10:30:00Z"
      filters:
        - column: "id"
          operator: "eq"
          value: "{{ new_user_id }}"
  assertions:
    - type: json_path
      path: ".[0].status"
      expected: "verified"
```

### DELETE - Removing Data

Delete records with required safety filters:

```yaml
- name: "Delete test user"
  plugin: supabase
  config:
    url: "{{ .vars.supabase_url }}"
    key: "{{ .vars.supabase_anon_key }}"
    operation: "delete"
    table: "users"
    delete:
      filters: # Filters are required for safety
        - column: "id"
          operator: "eq"
          value: "{{ new_user_id }}"
```

## Filter Operators

The Supabase plugin supports comprehensive filtering options:

| Operator | Description           | Example                                    |
| -------- | --------------------- | ------------------------------------------ |
| `eq`     | Equal                 | `{"operator": "eq", "value": "active"}`    |
| `neq`    | Not equal             | `{"operator": "neq", "value": "deleted"}`  |
| `gt`     | Greater than          | `{"operator": "gt", "value": 100}`         |
| `gte`    | Greater than or equal | `{"operator": "gte", "value": 18}`         |
| `lt`     | Less than             | `{"operator": "lt", "value": 1000}`        |
| `lte`    | Less than or equal    | `{"operator": "lte", "value": 65}`         |
| `like`   | Pattern matching      | `{"operator": "like", "value": "%test%"}`  |
| `ilike`  | Case-insensitive like | `{"operator": "ilike", "value": "%TEST%"}` |
| `is`     | Null check            | `{"operator": "is", "value": null}`        |
| `in`     | Value in list         | `{"operator": "in", "value": ["a", "b"]}`  |

### Complex Filtering Example

```yaml
select:
  columns: ["id", "name", "email", "age", "status"]
  filters:
    - column: "status"
      operator: "in"
      value: ["active", "premium"]
    - column: "age"
      operator: "gte"
      value: 18
    - column: "name"
      operator: "ilike"
      value: "%smith%"
  order:
    - column: "created_at"
      ascending: false
    - column: "name"
      ascending: true
  limit: 50
```

## RPC Function Calls

Execute PostgreSQL functions and stored procedures:

```yaml
- name: "Call simple function"
  plugin: supabase
  config:
    url: "{{ .vars.supabase_url }}"
    key: "{{ .vars.supabase_anon_key }}"
    operation: "rpc"
    rpc:
      function: "get_user_count"
  save:
    - json_path: "."
      as: "total_users"

- name: "Call function with parameters"
  plugin: supabase
  config:
    url: "{{ .vars.supabase_url }}"
    key: "{{ .vars.supabase_anon_key }}"
    operation: "rpc"
    rpc:
      function: "create_user_profile"
      params:
        user_id: "{{ new_user_id }}"
        profile_data:
          bio: "Test user profile"
          preferences: { "notifications": true }
  assertions:
    - type: json_path
      path: ".success"
      expected: true
```

## Authentication Operations

### User Signup

Create new user accounts with metadata:

```yaml
- name: "User registration"
  plugin: supabase
  config:
    url: "{{ .vars.supabase_url }}"
    key: "{{ .vars.supabase_anon_key }}"
    operation: "auth_sign_up"
    auth:
      email: "newuser@example.com"
      password: "SecurePassword123!"
      user_metadata:
        first_name: "John"
        last_name: "Doe"
        role: "customer"
  save:
    - json_path: ".user.id"
      as: "auth_user_id"
  assertions:
    - type: json_path
      path: ".user.email"
      expected: "newuser@example.com"
```

### User Signin

Authenticate existing users:

```yaml
- name: "User authentication"
  plugin: supabase
  config:
    url: "{{ .vars.supabase_url }}"
    key: "{{ .vars.supabase_anon_key }}"
    operation: "auth_sign_in"
    auth:
      email: "newuser@example.com"
      password: "SecurePassword123!"
  save:
    - json_path: ".access_token"
      as: "user_token"
    - json_path: ".refresh_token"
      as: "refresh_token"
  assertions:
    - type: json_path
      path: ".access_token"
      expected: "exists"
```

### Admin User Operations

Manage users with service role permissions:

```yaml
- name: "Create user (admin)"
  plugin: supabase
  config:
    url: "{{ .vars.supabase_url }}"
    key: "{{ .vars.supabase_service_key }}" # Service role required
    operation: "auth_create_user"
    auth:
      email: "admin-created@example.com"
      password: "AdminPassword123!"
      user_metadata:
        created_by: "admin"
        role: "staff"

- name: "Delete user (admin)"
  plugin: supabase
  config:
    url: "{{ .vars.supabase_url }}"
    key: "{{ .vars.supabase_service_key }}"
    operation: "auth_delete_user"
    auth:
      user_id: "{{ auth_user_id }}"
```

## Storage Operations

### Create Storage Bucket

```yaml
- name: "Create storage bucket"
  plugin: supabase
  config:
    url: "{{ .vars.supabase_url }}"
    key: "{{ .vars.supabase_anon_key }}"
    operation: "storage_create_bucket"
    storage:
      bucket: "test-uploads"
      public: true
```

### File Upload

```yaml
- name: "Upload test file"
  plugin: supabase
  config:
    url: "{{ .vars.supabase_url }}"
    key: "{{ .vars.supabase_anon_key }}"
    operation: "storage_upload"
    storage:
      bucket: "test-uploads"
      path: "documents/test-file.txt"
      file_content: |
        This is test file content.
        Line 2 of the file.
        End of content.
      content_type: "text/plain"
      cache_control: "3600"
```

### File Download

```yaml
- name: "Download and verify file"
  plugin: supabase
  config:
    url: "{{ .vars.supabase_url }}"
    key: "{{ .vars.supabase_anon_key }}"
    operation: "storage_download"
    storage:
      bucket: "test-uploads"
      path: "documents/test-file.txt"
  assertions:
    - type: json_path
      path: "."
      expected: |
        This is test file content.
        Line 2 of the file.
        End of content.
```

### File Deletion

```yaml
- name: "Clean up test file"
  plugin: supabase
  config:
    url: "{{ .vars.supabase_url }}"
    key: "{{ .vars.supabase_anon_key }}"
    operation: "storage_delete"
    storage:
      bucket: "test-uploads"
      path: "documents/test-file.txt"
```

## Data Extraction and Chaining

### Save Operation Syntax

Extract data from responses for use in subsequent steps:

```yaml
save:
  # Extract from JSON response data
  - json_path: ".[0].id"
    as: "record_id"
  - json_path: ".user.email"
    as: "user_email"

  # Extract from response headers
  - header: "Content-Type"
    as: "response_type"

  # Optional extractions (won't fail if missing)
  - json_path: ".optional_field"
    as: "optional_data"
    required: false
```

### JSON Path Examples

| Path                    | Description               | Example Result |
| ----------------------- | ------------------------- | -------------- |
| `".[0].id"`             | First record's ID         | `123`          |
| `".count"`              | Total count from response | `45`           |
| `".user.metadata.role"` | Nested object property    | `"admin"`      |
| `"length"`              | Array length              | `10`           |

## Configuration with Variables

Use variables for reusable and secure configurations:

```yaml
vars:
  supabase_url: "https://your-project.supabase.co"
  supabase_anon_key: "{{ .env.SUPABASE_ANON_KEY }}"
  supabase_service_key: "{{ .env.SUPABASE_SERVICE_KEY }}"
  test_email: "test@example.com"

tests:
  - name: "User lifecycle test"
    steps:
      - name: "Create user"
        plugin: supabase
        config:
          url: "{{ .vars.supabase_url }}"
          key: "{{ .vars.supabase_anon_key }}"
          operation: "insert"
          table: "users"
          insert:
            data:
              email: "{{ .vars.test_email }}"
              status: "pending"
```

## Assertions

Validate Supabase operations with built-in assertions:

```yaml
assertions:
  # Standard JSON path assertions
  - type: json_path
    path: ".[0].status"
    expected: "active"

  # Check array lengths
  - type: json_path
    path: "length"
    expected: 5

  # Verify nested data
  - type: json_path
    path: ".user.metadata.role"
    expected: "admin"

  # Check for existence (any truthy value)
  - type: json_path
    path: ".access_token"
    expected: "exists"
```

## Complete Workflow Example

```yaml
name: "Supabase E2E User Journey"
version: "v1.0.0"

vars:
  supabase_url: "https://your-project.supabase.co"
  supabase_anon_key: "your-anon-key"
  test_email: "integration-test@example.com"
  test_password: "TestPassword123!"

tests:
  - name: "Complete user workflow"
    steps:
      # 1. User Registration
      - name: "User signup"
        plugin: supabase
        config:
          url: "{{ .vars.supabase_url }}"
          key: "{{ .vars.supabase_anon_key }}"
          operation: "auth_sign_up"
          auth:
            email: "{{ .vars.test_email }}"
            password: "{{ .vars.test_password }}"
            user_metadata:
              name: "Test User"
              source: "integration_test"
        save:
          - json_path: ".user.id"
            as: "user_id"

      # 2. Create User Profile
      - name: "Create profile"
        plugin: supabase
        config:
          url: "{{ .vars.supabase_url }}"
          key: "{{ .vars.supabase_anon_key }}"
          operation: "insert"
          table: "profiles"
          insert:
            data:
              user_id: "{{ user_id }}"
              display_name: "Test User"
              bio: "Integration test profile"
              preferences:
                theme: "dark"
                notifications: true
        save:
          - json_path: ".[0].id"
            as: "profile_id"

      # 3. User Authentication
      - name: "User signin"
        plugin: supabase
        config:
          url: "{{ .vars.supabase_url }}"
          key: "{{ .vars.supabase_anon_key }}"
          operation: "auth_sign_in"
          auth:
            email: "{{ .vars.test_email }}"
            password: "{{ .vars.test_password }}"
        save:
          - json_path: ".access_token"
            as: "auth_token"

      # 4. File Upload
      - name: "Upload profile picture"
        plugin: supabase
        config:
          url: "{{ .vars.supabase_url }}"
          key: "{{ .vars.supabase_anon_key }}"
          operation: "storage_upload"
          storage:
            bucket: "profiles"
            path: "avatars/{{ user_id }}.jpg"
            file_content: "fake-image-data-for-testing"
            content_type: "image/jpeg"

      # 5. Update Profile with Image
      - name: "Update profile with avatar"
        plugin: supabase
        config:
          url: "{{ .vars.supabase_url }}"
          key: "{{ .vars.supabase_anon_key }}"
          operation: "update"
          table: "profiles"
          update:
            data:
              avatar_url: "avatars/{{ user_id }}.jpg"
              updated_at: "{{ .now }}"
            filters:
              - column: "user_id"
                operator: "eq"
                value: "{{ user_id }}"

      # 6. Query Final State
      - name: "Verify complete profile"
        plugin: supabase
        config:
          url: "{{ .vars.supabase_url }}"
          key: "{{ .vars.supabase_anon_key }}"
          operation: "select"
          table: "profiles"
          select:
            columns: ["*"]
            filters:
              - column: "user_id"
                operator: "eq"
                value: "{{ user_id }}"
        assertions:
          - type: json_path
            path: ".[0].display_name"
            expected: "Test User"
          - type: json_path
            path: ".[0].avatar_url"
            expected: "avatars/{{ user_id }}.jpg"

      # 7. Cleanup
      - name: "Delete test data"
        plugin: supabase
        config:
          url: "{{ .vars.supabase_url }}"
          key: "{{ .vars.supabase_anon_key }}"
          operation: "delete"
          table: "profiles"
          delete:
            filters:
              - column: "user_id"
                operator: "eq"
                value: "{{ user_id }}"
```

## Testing with Row Level Security

Test RLS policies by using different authentication contexts:

```yaml
- name: "Test RLS with user context"
  plugin: supabase
  config:
    url: "{{ .vars.supabase_url }}"
    key: "{{ user_auth_token }}" # User's JWT token
    operation: "select"
    table: "private_data"
    select:
      columns: ["id", "user_id", "data"]
  assertions:
    # Should only return data owned by this user
    - type: json_path
      path: "length"
      expected: 1

- name: "Test RLS with anon context"
  plugin: supabase
  config:
    url: "{{ .vars.supabase_url }}"
    key: "{{ .vars.supabase_anon_key }}" # Anonymous access
    operation: "select"
    table: "private_data"
    select:
      columns: ["id", "data"]
  assertions:
    # Should return no data (RLS blocks anonymous access)
    - type: json_path
      path: "length"
      expected: 0
```

## Running Tests

```bash
# Run basic Supabase tests
rocketship run -af examples/supabase-testing/rocketship.yaml

# Run with environment variables
SUPABASE_URL=your-url SUPABASE_ANON_KEY=your-key rocketship run -af your-test.yaml

# Run against different environments
rocketship run -af tests/supabase/staging.yaml
rocketship run -af tests/supabase/production.yaml
```

## Best Practices

### 1. Use Environment Variables for Credentials

```yaml
vars:
  supabase_url: "{{ .env.SUPABASE_URL }}"
  supabase_anon_key: "{{ .env.SUPABASE_ANON_KEY }}"
  supabase_service_key: "{{ .env.SUPABASE_SERVICE_KEY }}"
```

### 2. Clean Up Test Data

Always clean up test data to maintain test isolation:

```yaml
# At the end of your test
- name: "Cleanup test data"
  plugin: supabase
  config:
    url: "{{ .vars.supabase_url }}"
    key: "{{ .vars.supabase_anon_key }}"
    operation: "delete"
    table: "test_data"
    delete:
      filters:
        - column: "created_by"
          operator: "eq"
          value: "integration_test"
```

### 3. Test Both Success and Error Cases

```yaml
# Test successful operations
- name: "Valid user creation"
  plugin: supabase
  config:
    url: "{{ .vars.supabase_url }}"
    key: "{{ .vars.supabase_anon_key }}"
    operation: "insert"
    table: "users"
    insert:
      data:
        email: "valid@example.com"
        name: "Valid User"

# Test error conditions
- name: "Duplicate email should fail"
  plugin: supabase
  config:
    url: "{{ .vars.supabase_url }}"
    key: "{{ .vars.supabase_anon_key }}"
    operation: "insert"
    table: "users"
    insert:
      data:
        email: "valid@example.com" # Same email
        name: "Duplicate User"
  # This step should fail due to unique constraint
```

### 4. Use Meaningful Test Data

```yaml
insert:
  data:
    email: "integration-test-{{ .timestamp }}@example.com"
    name: "Test User {{ .timestamp }}"
    metadata:
      test_run_id: "{{ .run_id }}"
      created_by: "rocketship_integration"
```

### 5. Test Database Functions

```yaml
- name: "Test business logic function"
  plugin: supabase
  config:
    url: "{{ .vars.supabase_url }}"
    key: "{{ .vars.supabase_anon_key }}"
    operation: "rpc"
    rpc:
      function: "calculate_user_score"
      params:
        user_id: "{{ user_id }}"
        include_bonus: true
  assertions:
    - type: json_path
      path: ".score"
      expected: 850
    - type: json_path
      path: ".bonus_applied"
      expected: true
```

## Troubleshooting

### Connection Issues

**"Invalid project URL"**

- Verify your Supabase project URL format: `https://project-id.supabase.co`
- Check that your project is active and not paused

**"Invalid API key"**

- Verify you're using the correct key for your operation:
  - `anon` key for most operations
  - `service_role` key for admin operations
- Check key hasn't expired or been regenerated

### Permission Errors

**"Insufficient privileges"**

- Check Row Level Security policies on your tables
- Verify user authentication context
- Use service role key for admin operations

**"Table not found"**

- Verify table exists in your database
- Check table name spelling and case sensitivity
- Ensure you have proper permissions

### Authentication Issues

**"Email already registered"**

- Use unique emails for each test run
- Clean up test users after testing
- Use email templates with timestamps

**"Invalid credentials"**

- Verify email/password combination
- Check if email confirmation is required
- Ensure user account isn't disabled

### Storage Issues

**"Bucket not found"**

- Create storage bucket first using `storage_create_bucket`
- Verify bucket name spelling
- Check bucket permissions

**"File upload failed"**

- Verify bucket exists and is accessible
- Check file path format
- Ensure proper permissions for upload

### General Debugging

Enable debug logging for detailed operation information:

```bash
ROCKETSHIP_LOG=DEBUG rocketship run -af your-test.yaml
```

Check Supabase Dashboard logs for server-side errors and detailed operation results.

The Supabase plugin provides comprehensive testing capabilities for modern full-stack applications, enabling you to validate your entire Supabase stack from database operations to user authentication and file storage.
