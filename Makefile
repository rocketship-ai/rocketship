.PHONY: build
build:
	go vet ./...
	go test ./...
	go build -o bin/rocketship ./cmd/rocketship
	go build -o bin/engine     ./cmd/engine
	go build -o bin/agent      ./cmd/worker

.PHONY: compose-up
compose-up:
	docker compose -f .docker/docker-compose.yaml up -d

.PHONY: lint
lint:
	golangci-lint run
