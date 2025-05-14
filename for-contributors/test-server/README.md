# Test Server

The test server is a simple HTTP server that can be used during development and testing of the Rocketship CLI. It provides detailed request/response logging and an in-memory store for data persistence during the session.

## Features

- Supports GET, POST, PUT, DELETE HTTP methods
- In-memory data store for session persistence
- Detailed request and response logging
- JSON-based API
- Automatic ID generation for resources
- Thread-safe operations

## Usage

### Starting the server

```bash
# Start on default port 8080
go run .

# Start on custom port
go run . -port 3000
```

### API Endpoints

The server supports RESTful operations on any resource type. Resources are stored in memory and will persist until the server is stopped.

#### Examples

1. Create a resource (POST):

```bash
curl -X POST http://localhost:8080/users -H "Content-Type: application/json" -d '{"name": "John Doe", "email": "john@example.com"}'
```

2. Get all resources of a type (GET):

```bash
curl http://localhost:8080/users
```

3. Get a specific resource (GET):

```bash
curl http://localhost:8080/users/user_0
```

4. Update a resource (PUT):

```bash
curl -X PUT http://localhost:8080/users/user_0 -H "Content-Type: application/json" -d '{"name": "John Updated", "email": "john.updated@example.com"}'
```

5. Delete a resource (DELETE):

```bash
curl -X DELETE http://localhost:8080/users/user_0
```

### Server Output

The server provides detailed logging of all requests and responses:

- 📥 Incoming requests (headers, body, method, path)
- 📤 Outgoing responses (status, body)
- 💾 Store operations
- ❌ Error messages (if any)

## Notes

- Data is stored in memory and will be lost when the server is stopped
- Resource IDs are automatically generated if not provided
- All responses are in JSON format
- The server is intended for development and testing purposes only
