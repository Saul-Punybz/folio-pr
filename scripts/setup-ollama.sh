#!/bin/bash
# Setup script to pull required Ollama models after container starts
# Run this after `docker compose up` on first deployment

set -euo pipefail

OLLAMA_HOST="${OLLAMA_HOST:-http://localhost:11434}"

echo "=== Folio AI Setup ==="
echo "Ollama host: $OLLAMA_HOST"

# Wait for Ollama to be ready
echo "Waiting for Ollama..."
until curl -sf "$OLLAMA_HOST/api/tags" > /dev/null 2>&1; do
    sleep 2
    echo "  Still waiting..."
done
echo "Ollama is ready!"

# Pull instruct model for summaries, classification, entity extraction
echo ""
echo "Pulling llama3.2:3b (instruct model)..."
curl -sf "$OLLAMA_HOST/api/pull" -d '{"name": "llama3.2:3b"}' | while read -r line; do
    status=$(echo "$line" | grep -o '"status":"[^"]*"' | head -1)
    if [ -n "$status" ]; then
        echo "  $status"
    fi
done
echo "llama3.2:3b ready!"

# Pull embedding model for vector similarity
echo ""
echo "Pulling nomic-embed-text (embedding model)..."
curl -sf "$OLLAMA_HOST/api/pull" -d '{"name": "nomic-embed-text"}' | while read -r line; do
    status=$(echo "$line" | grep -o '"status":"[^"]*"' | head -1)
    if [ -n "$status" ]; then
        echo "  $status"
    fi
done
echo "nomic-embed-text ready!"

# Verify models are available
echo ""
echo "=== Installed Models ==="
curl -sf "$OLLAMA_HOST/api/tags" | python3 -m json.tool 2>/dev/null || curl -sf "$OLLAMA_HOST/api/tags"

echo ""
echo "=== Folio AI setup complete! ==="
