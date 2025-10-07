# Rocketship Test Server

A simple HTTP test server for Rocketship CLI examples and testing.

## Features

- RESTful API endpoints for testing
- In-memory data store
- CORS enabled for cross-origin requests
- Rate limiting (100 requests per minute)
- Automatic HTTPS with Fly.io
- Request/response logging

## API Endpoints

### Resource CRUD Operations

- `GET /{resource}` - List all resources of a type
- `GET /{resource}/{id}` - Get a specific resource
- `POST /{resource}` - Create a new resource
- `PUT /{resource}/{id}` - Update a resource
- `DELETE /{resource}/{id}` - Delete a resource
- `POST /_clear` - Clear all data

### Utility Endpoints

- `GET /status/{code}` - Returns the specified HTTP status code (100-599)
- `GET /delay/{seconds}` - Delays response by N seconds (max 10)
- `GET /echo` - Echoes back request headers and query parameters
- `POST /echo` - Echoes back request headers, body, and form data
- `GET /uuid` - Generates and returns a new UUID v4
- `GET /json` - Returns sample JSON data for testing

## Local Development

1. Run the server:

   ```bash
   go run .
   ```

2. The server will start on port 8080 by default. You can change this with the `-port` flag:
   ```bash
   go run . -port 3000
   ```

## Example Usage

```bash
# Create a user
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d '{"name": "John Doe", "email": "john@example.com"}'

# Get all users
curl http://localhost:8080/users

# Get a specific user
curl http://localhost:8080/users/user_0

# Update a user
curl -X PUT http://localhost:8080/users/user_0 \
  -H "Content-Type: application/json" \
  -d '{"name": "John Updated", "email": "john@example.com"}'

# Delete a user
curl -X DELETE http://localhost:8080/users/user_0

# Clear all data
curl -X POST http://localhost:8080/_clear
```

### Utility Endpoints Examples

```bash
# Test status codes
curl http://localhost:8080/status/200  # Success
curl http://localhost:8080/status/404  # Not Found
curl http://localhost:8080/status/503  # Service Unavailable

# Test delays (useful for retry/timeout testing)
curl http://localhost:8080/delay/0  # No delay
curl http://localhost:8080/delay/2  # 2 second delay

# Echo request details (GET)
curl "http://localhost:8080/echo?foo=bar&test=value"

# Echo form data (POST)
curl -X POST http://localhost:8080/echo \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "name=John&email=john@example.com"

# Echo JSON data (POST)
curl -X POST http://localhost:8080/echo \
  -H "Content-Type: application/json" \
  -d '{"name": "John", "email": "john@example.com"}'

# Generate UUID
curl http://localhost:8080/uuid

# Get sample JSON data
curl http://localhost:8080/json
```

## Notes

- Data is stored in memory and will be lost when the server is stopped
- Resource IDs are automatically generated if not provided
- All responses are in JSON format
- The server is intended for development and testing purposes only
