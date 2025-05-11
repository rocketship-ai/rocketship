.PHONY: proto lint test build compose-up install clean prepare-embed

prepare-embed:
	@mkdir -p internal/embedded/bin
	@touch internal/embedded/bin/.gitkeep

# Build the binaries
build-binaries: prepare-embed
	go build -o internal/embedded/bin/worker cmd/worker/main.go
	go build -o internal/embedded/bin/engine cmd/engine/main.go

# Build the CLI with embedded binaries
build: build-binaries
	go vet ./...
	go test ./...
	go build -o bin/rocketship cmd/rocketship/main.go

lint: prepare-embed
	golangci-lint run

test: prepare-embed
	go test ./...

proto:
	protoc \
	  --proto_path=proto \
	  --go_out=paths=source_relative:internal/api/generated \
	  --go-grpc_out=paths=source_relative:internal/api/generated \
	  proto/engine.proto

# Install the CLI to /usr/local/bin
install: build
	cp bin/rocketship /usr/local/bin/

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
	rm -rf bin/
	rm -rf internal/embedded/bin/
