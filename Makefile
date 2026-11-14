# Folio — Political Intelligence System
# Run `make help` to see all available targets.

.PHONY: migrate seed build build-linux test frontend mac run-app help

# Default database connection for local development
DB_URL ?= postgres://folio:folio_dev@localhost:5432/folio?sslmode=disable

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

## build: Compile the single folio binary (frontend/dist must be built first)
build:
	go build -o bin/folio ./cmd/folio

## build-linux: Cross-compile folio for a Linux amd64 server
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/folio-linux-amd64 ./cmd/folio

## test: Run all Go tests
test:
	go test ./...

## frontend: Build the frontend static assets
frontend:
	cd frontend && npm install && npm run build

## mac: Build build/Folio.app + build/Folio.zip (Apple Silicon, Postgres embebido)
mac:
	bash scripts/build-macos.sh

## run-app: Run the built app in the foreground without opening the browser
run-app:
	FOLIO_NO_BROWSER=1 build/Folio.app/Contents/MacOS/Folio app

## help: Show this help message
help:
	@echo "Folio Makefile targets:"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'
	@echo ""
