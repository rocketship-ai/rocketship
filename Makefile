.PHONY: build
test:
	go test ./...
build:
	go vet ./...
	go test ./...
	go build -o bin/cli ./cmd/cli
	go build -o bin/engine     ./cmd/engine
	go build -o bin/worker      ./cmd/worker

.PHONY: compose-up
compose-up:
	docker compose -f .docker/docker-compose.yaml up -d

.PHONY: lint
lint:
	golangci-lint run

.PHONY: proto
proto:
	protoc \
	  --proto_path=proto \
	  --go_out=paths=source_relative:internal/api/generated \
	  --go-grpc_out=paths=source_relative:internal/api/generated \
	  proto/engine.proto
