.DEFAULT_GOAL := help

.PHONY: help bootstrap infra-up infra-down app-up app-down migrate river-migrate frontend backend-api backend-worker backend-realtime generate check frontend-check go-test go-race compose-check

help:
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_-]+:.*##/ {printf "%-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

bootstrap: ## Install frontend dependencies and download Go modules
	npm ci
	cd backend && go mod download

infra-up: ## Start PostgreSQL, Redis, S3Mock, Mailpit and ClamAV
	docker compose up -d postgres redis s3mock mailpit clamav

infra-down: ## Stop the local stack without deleting volumes
	docker compose down

app-up: ## Build and start API, worker and realtime with local infrastructure
	docker compose --profile app up --build

app-down: ## Stop the complete local stack without deleting volumes
	docker compose --profile app down

migrate: ## Apply application and River queue migrations to the local database
	docker compose --profile app run --rm migrate
	docker compose --profile app run --rm river-migrate up

river-migrate: ## Apply River queue migrations through a direct database connection
	docker compose --profile app run --rm river-migrate up

frontend: ## Run Next.js on port 3000
	npm run dev

backend-api: ## Run the Go API on port 8080
	@set -a; . ./.env; set +a; cd backend; exec go run ./cmd/api

backend-worker: ## Run the background worker
	@set -a; . ./.env; set +a; cd backend; exec go run ./cmd/worker

backend-realtime: ## Run the realtime gateway on port 8081
	@set -a; . ./.env; set +a; cd backend; exec go run ./cmd/realtime

generate: ## Regenerate checked-in API/database code
	cd backend && go generate ./...
	npm run generate:api

frontend-check: ## Lint, type-check and test the frontend
	npm run check

go-test: ## Run Go unit tests
	cd backend && go test ./...

go-race: ## Run Go tests with race detection
	cd backend && go test -race ./...

compose-check: ## Validate the Compose model
	docker compose config --quiet

check: frontend-check go-race compose-check ## Run the local verification suite
