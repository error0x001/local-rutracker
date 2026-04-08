.PHONY: help build build-migrator build-server run-server run-migrator docker-pg docker-down docker-migrate docker-logs docker-rebuild clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: build-migrator build-server ## Build Go binaries

build-migrator: ## Build migrator binary
	go build -o bin/migrator ./cmd/migrator

build-server: ## Build server binary
	go build -o bin/server ./cmd/server

run-server: ## Run server locally
	DB_HOST=localhost DB_PORT=5433 DB_USER=rutracker DB_PASSWORD=rutracker DB_NAME=rutracker DB_SSLMODE=disable \
	go run ./cmd/server

run-migrator: ## Run migrator locally (requires archive file)
	DB_HOST=localhost DB_PORT=5433 DB_USER=rutracker DB_PASSWORD=rutracker DB_NAME=rutracker DB_SSLMODE=disable \
	go run ./cmd/migrator -file=rutracker-20260329.xml.xz

docker-pg: ## Start PostgreSQL only
	docker compose up -d postgres

docker-migrate: ## Run migration in Docker (starts migrator service)
	docker compose up migrator

docker-server: ## Run server in Docker
	docker compose up server -d

docker-all: ## Start all services
	docker compose up -d

docker-down: ## Stop all services
	docker compose down

docker-down-v: ## Stop all services and delete volumes
	docker compose down -v

docker-logs: ## Show logs from all services
	docker compose logs -f

docker-logs-migrator: ## Show migrator logs
	docker compose logs -f migrator

docker-rebuild: docker-down ## Rebuild Docker images
	docker compose build --no-cache

clean: ## Remove build artifacts
	rm -rf bin/
