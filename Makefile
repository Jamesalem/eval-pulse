.PHONY: help build test run-api run-worker docker-up docker-down docker-build lint regression-check

help:
	@echo "EvalPulse Development & Operations CLI"
	@echo "Usage:"
	@echo "  make build             - Build API and Worker binaries"
	@echo "  make test              - Run all Go unit tests with race detector"
	@echo "  make run-api           - Run API gateway locally"
	@echo "  make run-worker        - Run distributed worker node locally"
	@echo "  make docker-up         - Start complete stack with Docker Compose"
	@echo "  make docker-down       - Stop Docker Compose stack"
	@echo "  make docker-build      - Build all Docker images"
	@echo "  make regression-check  - Execute CI/CD regression exit gate test"

build:
	go build -o bin/api ./cmd/api
	go build -o bin/worker ./cmd/worker

test:
	go test -v -race -cover ./...

run-api:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-build:
	docker compose build

lint:
	go vet ./...

regression-check:
	@echo "Evaluating regression threshold..."
	@curl -s -f http://localhost:8080/api/v1/metrics/regression-check?threshold_percent=5.0 || exit 1
