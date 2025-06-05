.PHONY: proto lint test build compose-up install clean prepare-embed dev-setup docs docs-serve docs-deps docs-clean install-workflowcheck

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

install-workflowcheck:
	@if ! command -v workflowcheck &> /dev/null; then \
		go install go.temporal.io/sdk/contrib/tools/workflowcheck@latest; \
	fi

# Run linting
lint: build-binaries install-workflowcheck lint-python
	@echo "Running Go linter..."
	golangci-lint run
	@echo "Checking workflows..."
	workflowcheck ./...

# Run Python linting
lint-python:
	@echo "Running Python linter..."
	@if command -v ruff &> /dev/null; then \
		find . -name "*.py" -type f ! -path "*/venv/*" ! -path "*/.venv/*" ! -path "*/browser-venv/*" ! -path "*/docs/*" -print0 | xargs -0 -r ruff check; \
	else \
		echo "Ruff not installed. Install with: pip install ruff"; \
		exit 1; \
	fi

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

# Install the CLI to $GOPATH/bin or $HOME/go/bin
install: build
	@echo "Installing CLI..."
	@rm -f $(shell which rocketship 2>/dev/null)
	go install ./cmd/rocketship

# Set up development environment
dev-setup: prepare-embed
	@echo "Setting up development environment..."
	@if [ ! -f .git/hooks/pre-commit ]; then \
		./for-contributors/setup-hooks.sh; \
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

# Generate and serve documentation
docs-deps:
	@echo "Setting up documentation environment..."
	@python3 -m venv docs/.venv
	@. docs/.venv/bin/activate && cd docs && python3 -m pip install -r requirements.txt

docs: docs-deps
	@echo "Generating documentation..."
	@go run ./cmd/docgen
	@. docs/.venv/bin/activate && cd docs && mkdocs build

docs-serve: docs-deps
	@echo "Starting documentation server..."
	@go run ./cmd/docgen
	@. docs/.venv/bin/activate && cd docs && mkdocs serve

docs-clean:
	@echo "Cleaning up documentation environment..."
	@rm -rf docs/.venv docs/site
