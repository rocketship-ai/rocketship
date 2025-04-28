.PHONY: build
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
