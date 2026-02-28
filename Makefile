# Folio — Political Intelligence System
# Run `make help` to see all available targets.

.PHONY: dev down migrate seed build test frontend logs help

# Default database connection for local development
DB_URL ?= postgres://folio:folio_dev@localhost:5432/folio?sslmode=disable

## dev: Build and start all services with Docker Compose
dev:
	docker compose up --build -d

## down: Stop all services
down:
	docker compose down

## migrate: Run all SQL migrations against the local database
migrate:
	@echo "Running migrations against $(DB_URL)..."
	@for f in migrations/*.sql; do \
		echo "  Applying $$f..."; \
		psql "$(DB_URL)" -f "$$f" 2>&1 || true; \
	done
	@echo "Migrations complete."

## seed: Run the seed migration (002_seed_sources.sql) against the local database
seed:
	@echo "Seeding sources..."
	psql "$(DB_URL)" -f migrations/002_seed_sources.sql
	@echo "Seed complete."

## build: Compile the API and Worker Go binaries
build:
	go build -o api ./cmd/api
	go build -o worker ./cmd/worker

## test: Run all Go tests
test:
	go test ./...

## frontend: Build the frontend static assets
frontend:
	cd frontend && npm install && npm run build

## logs: Tail Docker Compose logs
logs:
	docker compose logs -f

## help: Show this help message
help:
	@echo "Folio Makefile targets:"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'
	@echo ""
