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
lint: build-binaries install-workflowcheck lint-python lint-typescript
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

# Run TypeScript linting
lint-typescript:
	@echo "Running TypeScript linter..."
	@if [ -d "mcp-server" ]; then \
		cd mcp-server && \
		if [ -f "package.json" ]; then \
			if command -v npm &> /dev/null; then \
				echo "Building TypeScript project..."; \
				npm run build || (echo "❌ TypeScript compilation failed" && exit 1); \
				echo "✅ TypeScript compilation successful"; \
			else \
				echo "npm not found. Please install Node.js and npm"; \
				exit 1; \
			fi; \
		else \
			echo "No package.json found in mcp-server directory"; \
		fi; \
	else \
		echo "No mcp-server directory found, skipping TypeScript linting"; \
	fi

# Build MCP server with embedded knowledge
build-mcp:
	@echo "Building MCP server with embedded knowledge..."
	@if [ -d "mcp-server" ]; then \
		cd mcp-server && \
		if [ -f "package.json" ]; then \
			if command -v npm &> /dev/null; then \
				npm run build || (echo "MCP server build failed" && exit 1); \
				echo "MCP server build successful"; \
			else \
				echo "npm not found. Please install Node.js and npm"; \
				exit 1; \
			fi; \
		else \
			echo "No package.json found in mcp-server directory"; \
		fi; \
	else \
		echo "No mcp-server directory found"; \
	fi

# Run tests
test: build-binaries test-go test-python test-typescript

# Run Go tests
test-go:
	@echo "Running Go tests..."
	go test ./...

# Run Python tests  
test-python:
	@echo "Running Python tests..."
	@if command -v python3 &> /dev/null; then \
		find . -name "*_test.py" -type f ! -path "*/venv/*" ! -path "*/.venv/*" ! -path "*/browser-venv/*" ! -path "*/docs/*" | while read -r file; do \
			echo "Running Python test: $$file"; \
			python3 "$$file" || exit 1; \
		done; \
		echo "All Python tests passed"; \
	else \
		echo "Python3 not found, skipping Python tests"; \
	fi

# Run TypeScript tests
test-typescript:
	@echo "Running TypeScript tests..."
	@if [ -d "mcp-server" ]; then \
		cd mcp-server && \
		if [ -f "package.json" ]; then \
			if command -v npm &> /dev/null; then \
				echo "Running TypeScript tests..."; \
				npm test || (echo "❌ TypeScript tests failed" && exit 1); \
				echo "✅ TypeScript tests successful"; \
			else \
				echo "npm not found, skipping TypeScript tests"; \
			fi; \
		else \
			echo "No package.json found in mcp-server directory, skipping TypeScript tests"; \
		fi; \
	else \
		echo "No mcp-server directory found, skipping TypeScript tests"; \
	fi

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
	@echo "Generating plugin reference..."
	@cd docs && python3 src/yaml-reference/generate-plugin-reference.py
	@. docs/.venv/bin/activate && cd docs && mkdocs build

docs-serve: docs-deps
	@echo "Starting documentation server..."
	@go run ./cmd/docgen
	@echo "Generating plugin reference..."
	@cd docs && python3 src/yaml-reference/generate-plugin-reference.py
	@. docs/.venv/bin/activate && cd docs && mkdocs serve

docs-clean:
	@echo "Cleaning up documentation environment..."
	@rm -rf docs/.venv docs/site
