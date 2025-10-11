# Variables
BINARY_NAME=server
DOCKER_IMAGE=ghcr.io/crossplane-contrib/provider-http-server
VERSION?=latest
PLATFORMS=linux/amd64,linux/arm64

# Go commands
.PHONY: build
build:
	go build -o ${BINARY_NAME} .

.PHONY: run
run: build
	./${BINARY_NAME}

.PHONY: clean
clean:
	go clean
	rm -f ${BINARY_NAME}

.PHONY: test
test:
	go test -v ./...

.PHONY: deps
deps:
	go mod download
	go mod tidy

# Docker commands
.PHONY: docker-build
docker-build:
	docker build -t ${DOCKER_IMAGE}:${VERSION} .

.PHONY: docker-run
docker-run: docker-build
	docker run -p 5000:5000 ${DOCKER_IMAGE}:${VERSION}

.PHONY: docker-push
docker-push: docker-build
	docker push ${DOCKER_IMAGE}:${VERSION}

# Multi-platform build (requires buildx)
.PHONY: docker-buildx
docker-buildx:
	docker buildx build --platform ${PLATFORMS} -t ${DOCKER_IMAGE}:${VERSION} --push .

# Development workflow
.PHONY: dev
dev: deps build run

.PHONY: dev-docker
dev-docker: docker-run

# Help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build         - Build the Go binary"
	@echo "  run           - Run the server locally"
	@echo "  clean         - Clean build artifacts"
	@echo "  test          - Run tests"
	@echo "  deps          - Download and tidy dependencies"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Build and run Docker container"
	@echo "  docker-push   - Push Docker image to registry"
	@echo "  docker-buildx - Build multi-platform and push"
	@echo "  dev           - Quick development (deps + build + run)"
	@echo "  dev-docker    - Quick development with Docker"
	@echo "  help          - Show this help message"