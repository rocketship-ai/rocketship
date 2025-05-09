.PHONY: proto lint test build compose-up install clean

proto:
	protoc \
	  --proto_path=proto \
	  --go_out=paths=source_relative:internal/api/generated \
	  --go-grpc_out=paths=source_relative:internal/api/generated \
	  proto/engine.proto

lint:
	golangci-lint run

test:
	go test ./...

# Build the CLI
build:
	go vet ./...
	go test ./...
	go build -o bin/rocketship cmd/cli/main.go
	go build -o bin/engine     ./cmd/engine
	go build -o bin/worker      ./cmd/worker

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
