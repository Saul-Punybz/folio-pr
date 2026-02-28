# Folio -- Build Instructions

## Quick Start (Local Dev)
```bash
# First time
bash scripts/setup.sh

# Or manually:
docker compose up -d --build
bash scripts/setup-ollama.sh
```

## Build
```bash
# Go backend
go build -o bin/api ./cmd/api
go build -o bin/worker ./cmd/worker

# Frontend
cd frontend && npm install && npm run build

# Docker
docker compose build
```

## Test
```bash
go test ./...
```

## Key Architecture
- Go Chi API server (cmd/api) -- serves REST API + static frontend
- Go Worker (cmd/worker) -- RSS/scraper ingestion + AI processing via Ollama
- Postgres + pgvector -- articles, users, sessions, vector similarity
- Ollama -- llama3.2:3b (summaries/classification) + nomic-embed-text (embeddings)
- Oracle Object Storage -- evidence archive (S3-compatible)
- Caddy -- reverse proxy + auto HTTPS
- Astro + React -- static frontend with interactive islands

## Environment Variables
See .env.example for all required variables.

## Database
Migrations in /migrations/ -- applied in order by filename.
Default admin: admin@folio.local / admin (CHANGE IN PRODUCTION)

## Deployment
```bash
# Oracle Free Tier ARM
bash scripts/deploy-oracle.sh
```

## Project Structure
```
cmd/api/         -- HTTP server entrypoint
cmd/worker/      -- Scraper + AI worker entrypoint
internal/        -- All Go packages
  auth/          -- Session auth middleware
  config/        -- Environment config
  db/            -- Database connection
  handlers/      -- HTTP handlers
  models/        -- DB models + queries
  scraper/       -- RSS parser + Colly scraper
  ai/            -- Ollama client
  storage/       -- S3 object storage client
  middleware/    -- HTTP middleware
migrations/      -- SQL migration files
frontend/        -- Astro + React frontend
scripts/         -- Setup and deploy scripts
configs/         -- Caddyfile
```
