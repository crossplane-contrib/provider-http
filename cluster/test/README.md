# Test Server

A simple HTTP server written in Go for testing HTTP provider functionality.

## Features

- Authorization middleware (Bearer/Basic token authentication)
- Custom headers (`X-Secret-Header`)
- User CRUD operations
- Login endpoint
- Notification endpoint
- In-memory data storage

## API Endpoints

All endpoints require `Authorization: Bearer my-secret-value` header.

### User Management
- `GET /v1/users/{user_id}` - Get user by ID
- `POST /v1/users` - Create user
- `PUT /v1/users/{user_id}` - Update user
- `DELETE /v1/users/{user_id}` - Delete user

### Authentication
- `POST /v1/login` - Get authentication token

### Notifications
- `POST /v1/notify` - Send notification

## Development

### Local Development
```bash
# Build and run
make dev

# Or manually
go build -o server .
./server
```

### Docker Development
```bash
# Build and run with Docker
make dev-docker

# Or manually
docker build -t test-server .
docker run -p 5000:5000 test-server
```

### Available Make Targets
- `make build` - Build the binary
- `make run` - Run the server locally
- `make test` - Run tests
- `make docker-build` - Build Docker image
- `make docker-run` - Build and run Docker container
- `make help` - Show all available targets

## Deployment

The server is automatically built and deployed to GitHub Container Registry when changes are pushed to the `cluster/test/` directory.

Image: `ghcr.io/crossplane-contrib/provider-http-server:latest`