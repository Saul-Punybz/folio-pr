#!/bin/bash
# First-time Folio setup script
# Run this once after cloning the repo

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "=== Folio First-Time Setup ==="
echo "Project: $PROJECT_DIR"
echo ""

# 1. Create .env from example if it doesn't exist
if [ ! -f "$PROJECT_DIR/.env" ]; then
    cp "$PROJECT_DIR/.env.example" "$PROJECT_DIR/.env"
    echo "[1/5] Created .env from .env.example"
    echo "  >>> EDIT .env with your Oracle S3 credentials before deploying to production <<<"
else
    echo "[1/5] .env already exists, skipping"
fi

# 2. Build frontend
echo "[2/5] Building frontend..."
cd "$PROJECT_DIR/frontend"
if [ -f "package.json" ]; then
    npm install
    npm run build
    echo "  Frontend built to frontend/dist/"
else
    echo "  No package.json found, skipping frontend build"
    mkdir -p dist
fi

# 3. Start Docker Compose
echo "[3/5] Starting Docker Compose..."
cd "$PROJECT_DIR"
docker compose up -d --build

# 4. Wait for Postgres and run migrations
echo "[4/5] Waiting for Postgres..."
sleep 5
until docker compose exec -T postgres pg_isready -U folio > /dev/null 2>&1; do
    sleep 2
done
echo "  Postgres is ready"

echo "  Running migrations..."
for f in "$PROJECT_DIR"/migrations/*.sql; do
    echo "    Applying $(basename "$f")..."
    docker compose exec -T postgres psql -U folio -d folio < "$f"
done
echo "  Migrations complete"

# 5. Pull Ollama models
echo "[5/5] Setting up AI models..."
bash "$SCRIPT_DIR/setup-ollama.sh"

echo ""
echo "======================================="
echo "  Folio is running!"
echo "  API:     http://localhost:8080"
echo "  Login:   admin@folio.local / admin"
echo "  Ollama:  http://localhost:11434"
echo "======================================="
echo ""
echo "Next steps:"
echo "  1. Change the admin password"
echo "  2. Configure your .env for production (S3 creds, domain)"
echo "  3. For production: set DOMAIN=your.duckdns.org in .env"
