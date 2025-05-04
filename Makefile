.PHONY: proto lint test build compose-up 

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

build:
	go vet ./...
	go test ./...
	go build -o bin/cli ./cmd/cli
	go build -o bin/engine     ./cmd/engine
	go build -o bin/worker      ./cmd/worker

compose-up:
	docker compose -f .docker/docker-compose.yaml up -d
