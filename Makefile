.PHONY: proto lint test build compose-up install clean prepare-embed dev-setup

prepare-embed:
	@mkdir -p internal/embedded/bin
	@touch internal/embedded/bin/.gitkeep

# Build the embedded binaries
build-binaries: prepare-embed
	@echo "Building embedded binaries..."
	@go build -o internal/embedded/bin/worker cmd/worker/main.go
	@go build -o internal/embedded/bin/engine cmd/engine/main.go

# Build the CLI with embedded binaries
build: build-binaries
	@echo "Building CLI..."
	go vet ./...
	go test ./...
	go build -o bin/rocketship cmd/rocketship/main.go

# Run linting
lint: build-binaries
	@echo "Running linter..."
	golangci-lint run

# Run tests
test: build-binaries
	@echo "Running tests..."
	go test ./...

# Generate protobuf code
proto:
	@echo "Generating protobuf code..."
	protoc \
	  --proto_path=proto \
	  --go_out=paths=source_relative:internal/api/generated \
	  --go-grpc_out=paths=source_relative:internal/api/generated \
	  proto/engine.proto

# Install the CLI to /usr/local/bin
install: build
	@echo "Installing CLI..."
	cp bin/rocketship /usr/local/bin/

# Set up development environment
dev-setup: prepare-embed
	@echo "Setting up development environment..."
	@if [ ! -f .git/hooks/pre-commit ]; then \
		./for-maintainers/setup-hooks.sh; \
	fi
	@echo "Building initial binaries..."
	@$(MAKE) build-binaries
	@echo "Development environment setup complete!"

compose-up:
	@if ! command -v docker-compose &> /dev/null; then \
		echo "Error: docker-compose is not installed."; \
		exit 1; \
	fi
	docker-compose -f .docker/docker-compose.yaml up -d

compose-down:
	docker-compose -f .docker/docker-compose.yaml down

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -rf internal/embedded/bin/
