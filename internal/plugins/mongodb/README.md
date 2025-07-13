# MongoDB Plugin

The MongoDB plugin provides comprehensive support for MongoDB database operations including document management, collection administration, indexing, and aggregation.

## Supported Operations

### Document Operations
- `insert` - Insert a single document
- `insert_many` - Insert multiple documents  
- `find` - Find multiple documents with filtering, sorting, and pagination
- `find_one` - Find a single document
- `update` - Update a single document
- `update_many` - Update multiple documents
- `delete` - Delete a single document
- `delete_many` - Delete multiple documents
- `count` - Count documents matching a filter

### Aggregation
- `aggregate` - Execute aggregation pipelines

### Index Management
- `create_index` - Create database indexes
- `drop_index` - Drop database indexes
- `list_indexes` - List all indexes for a collection

### Collection Management
- `create_collection` - Create a new collection
- `drop_collection` - Drop an existing collection
- `list_collections` - List all collections in a database

## Configuration

### Basic Configuration
```yaml
plugin: mongodb
config:
  uri: "mongodb://localhost:27017"  # MongoDB connection URI
  database: "my_database"           # Database name
  collection: "my_collection"       # Collection name (required for most operations)
  operation: "insert"               # Operation to perform
  timeout: "30s"                    # Operation timeout (optional)
```

### Connection URI Examples
```yaml
# Local MongoDB
uri: "mongodb://localhost:27017"

# MongoDB with authentication
uri: "mongodb://username:password@localhost:27017"

# MongoDB Atlas
uri: "mongodb+srv://username:password@cluster.mongodb.net"

# MongoDB with replica set
uri: "mongodb://host1:27017,host2:27017,host3:27017/database?replicaSet=rs0"
```

## Operation Examples

### Insert Operations

#### Insert Single Document
```yaml
- name: "Create user"
  plugin: mongodb
  config:
    uri: "{{ .vars.mongodb_uri }}"
    database: "app_db"
    collection: "users"
    operation: "insert"
    insert:
      document:
        name: "John Doe"
        email: "john@example.com"
        age: 30
        active: true
        tags: ["developer", "mongodb"]
```

#### Insert Multiple Documents
```yaml
- name: "Create multiple users"
  plugin: mongodb
  config:
    uri: "{{ .vars.mongodb_uri }}"
    database: "app_db"
    collection: "users"
    operation: "insert_many"
    insert:
      documents:
        - name: "Alice"
          email: "alice@example.com"
          age: 28
        - name: "Bob"
          email: "bob@example.com"
          age: 32
      ordered: true  # Process documents in order
```

### Find Operations

#### Find Multiple Documents
```yaml
- name: "Find active users"
  plugin: mongodb
  config:
    uri: "{{ .vars.mongodb_uri }}"
    database: "app_db"
    collection: "users"
    operation: "find"
    find:
      filter:
        active: true
        age: { $gte: 25 }
      projection:
        name: 1
        email: 1
        age: 1
      sort:
        age: -1  # Sort by age descending
      limit: 10
      skip: 0
```

#### Find Single Document
```yaml
- name: "Find user by email"
  plugin: mongodb
  config:
    uri: "{{ .vars.mongodb_uri }}"
    database: "app_db"
    collection: "users"
    operation: "find_one"
    find:
      filter:
        email: "john@example.com"
      projection:
        password: 0  # Exclude password field
```

### Update Operations

#### Update Single Document
```yaml
- name: "Update user age"
  plugin: mongodb
  config:
    uri: "{{ .vars.mongodb_uri }}"
    database: "app_db"
    collection: "users"
    operation: "update"
    update:
      filter:
        email: "john@example.com"
      update:
        $set:
          age: 31
          updated_at: "2024-01-15T10:00:00Z"
        $inc:
          login_count: 1
      upsert: false
```

#### Update Multiple Documents
```yaml
- name: "Activate all developers"
  plugin: mongodb
  config:
    uri: "{{ .vars.mongodb_uri }}"
    database: "app_db"
    collection: "users"
    operation: "update_many"
    update:
      filter:
        tags: "developer"
      update:
        $set:
          active: true
          last_updated: "2024-01-15T10:00:00Z"
```

### Delete Operations

#### Delete Single Document
```yaml
- name: "Delete inactive user"
  plugin: mongodb
  config:
    uri: "{{ .vars.mongodb_uri }}"
    database: "app_db"
    collection: "users"
    operation: "delete"
    delete:
      filter:
        active: false
        last_login: { $lt: "2023-01-01T00:00:00Z" }
```

#### Delete Multiple Documents
```yaml
- name: "Delete all inactive users"
  plugin: mongodb
  config:
    uri: "{{ .vars.mongodb_uri }}"
    database: "app_db"
    collection: "users"
    operation: "delete_many"
    delete:
      filter:
        active: false
```

### Count Operations

```yaml
- name: "Count active users"
  plugin: mongodb
  config:
    uri: "{{ .vars.mongodb_uri }}"
    database: "app_db"
    collection: "users"
    operation: "count"
    count:
      filter:
        active: true
```

### Aggregation Operations

```yaml
- name: "User statistics by age group"
  plugin: mongodb
  config:
    uri: "{{ .vars.mongodb_uri }}"
    database: "app_db"
    collection: "users"
    operation: "aggregate"
    aggregate:
      pipeline:
        - $match:
            active: true
        - $group:
            _id:
              $cond:
                if: { $gte: ["$age", 30] }
                then: "30+"
                else: "under_30"
            count: { $sum: 1 }
            avg_age: { $avg: "$age" }
        - $sort:
            count: -1
```

### Index Operations

#### Create Index
```yaml
- name: "Create email index"
  plugin: mongodb
  config:
    uri: "{{ .vars.mongodb_uri }}"
    database: "app_db"
    collection: "users"
    operation: "create_index"
    index:
      keys:
        email: 1
      options:
        unique: true
        name: "email_unique_idx"
        background: true
```

#### Drop Index
```yaml
- name: "Drop email index"
  plugin: mongodb
  config:
    uri: "{{ .vars.mongodb_uri }}"
    database: "app_db"
    collection: "users"
    operation: "drop_index"
    index:
      name: "email_unique_idx"
```

#### List Indexes
```yaml
- name: "List all indexes"
  plugin: mongodb
  config:
    uri: "{{ .vars.mongodb_uri }}"
    database: "app_db"
    collection: "users"
    operation: "list_indexes"
```

### Collection Management

#### Create Collection
```yaml
- name: "Create users collection"
  plugin: mongodb
  config:
    uri: "{{ .vars.mongodb_uri }}"
    database: "app_db"
    operation: "create_collection"
    admin:
      action: "create_collection"
      collection: "users"
```

#### Drop Collection
```yaml
- name: "Drop users collection"
  plugin: mongodb
  config:
    uri: "{{ .vars.mongodb_uri }}"
    database: "app_db"
    operation: "drop_collection"
    admin:
      action: "drop_collection"
      collection: "users"
```

#### List Collections
```yaml
- name: "List all collections"
  plugin: mongodb
  config:
    uri: "{{ .vars.mongodb_uri }}"
    database: "app_db"
    operation: "list_collections"
```

## Variable Replacement

The MongoDB plugin supports the central DSL template system for variable replacement:

### Config Variables (processed by CLI)
```yaml
config:
  uri: "{{ .vars.mongodb_uri }}"
  database: "{{ .vars.database_name }}"
```

### Runtime Variables (processed by plugin)
```yaml
find:
  filter:
    user_id: "{{ user_id }}"  # From previous step's saved variables
    status: "{{ status }}"
```

### Environment Variables
```yaml
config:
  uri: "{{ .env.MONGODB_CONNECTION_STRING }}"
```

## Assertions

### Document Existence
```yaml
assertions:
  - type: "document_exists"
    expected: true
```

### Count Validation
```yaml
assertions:
  - type: "count_equals"
    expected: 5
```

### Field Value Matching
```yaml
assertions:
  - type: "field_matches"
    field: "name"
    expected: "John Doe"
```

### JSON Path Assertions
```yaml
assertions:
  - type: "json_path"
    path: ".inserted_ids[0]"
  - type: "json_path"
    path: ".modified_count"
    expected: "1"
```

### Result Content Validation
```yaml
assertions:
  - type: "result_contains"
    expected: "john@example.com"
```

### Error Validation
```yaml
assertions:
  - type: "error_exists"
    expected: false
```

## Saving Results

### Save Inserted ID
```yaml
save:
  - as: "user_id"
    json_path: ".inserted_ids[0]"
```

### Save Query Results
```yaml
save:
  - as: "user_count"
    json_path: ".count"
  - as: "first_user_name"
    json_path: ".data[0].name"
```

### Save Metadata
```yaml
save:
  - as: "operation_duration"
    json_path: ".metadata.duration"
```

## Error Handling

The plugin handles various error conditions:

- **Connection Errors**: Invalid URI, network issues, authentication failures
- **Database Errors**: Invalid database or collection names
- **Operation Errors**: Invalid queries, constraint violations, timeout errors
- **Validation Errors**: Missing required fields, invalid data types

Errors are returned in the response's `error` field:

```json
{
  "error": {
    "code": 11000,
    "message": "E11000 duplicate key error",
    "details": "duplicate key error collection: users index: email_1"
  }
}
```

## Response Format

### Successful Operations
```json
{
  "data": [...],           // Query results or operation data
  "inserted_ids": [...],   // IDs of inserted documents
  "modified_count": 2,     // Number of modified documents
  "deleted_count": 1,      // Number of deleted documents
  "matched_count": 3,      // Number of matched documents
  "upserted_id": "...",    // ID of upserted document
  "count": 42,             // Document count
  "metadata": {
    "operation": "find",
    "database": "app_db",
    "collection": "users",
    "duration": "15.2ms"
  }
}
```

### Error Operations
```json
{
  "error": {
    "code": 11000,
    "message": "Error message",
    "details": "Detailed error information"
  },
  "metadata": {
    "operation": "insert",
    "database": "app_db",
    "collection": "users",
    "duration": "5.1ms"
  }
}
```

## Best Practices

1. **Use Specific Filters**: Always use specific filters for update and delete operations to avoid unintended modifications.

2. **Index Strategy**: Create indexes for frequently queried fields to improve performance.

3. **Batch Operations**: Use `insert_many`, `update_many`, and `delete_many` for bulk operations.

4. **Projections**: Use projections to limit returned data and improve performance.

5. **Connection Pooling**: The plugin automatically handles connection pooling for optimal performance.

6. **Error Handling**: Always include error assertions for operations that might fail.

7. **Variable Security**: Use environment variables for sensitive connection strings.

8. **Timeouts**: Set appropriate timeouts for operations that might take time.

9. **Aggregation Optimization**: Structure aggregation pipelines efficiently with `$match` early to reduce document flow.

10. **Index Maintenance**: Regularly review and optimize indexes based on query patterns.