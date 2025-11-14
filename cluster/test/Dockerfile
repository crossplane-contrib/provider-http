# Build stage
FROM golang:1.21-alpine AS builder

# Install ca-certificates
RUN apk --no-cache add ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod files and vendor directory
COPY go.mod go.sum ./
COPY vendor ./vendor

# Copy source code
COPY . .

# Build the application using vendored dependencies
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -a -installsuffix cgo -o server .

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN adduser -D -s /bin/sh appuser

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/server .

# Change ownership to appuser
RUN chown appuser:appuser server

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 5000

# Run the server
CMD ["./server"]