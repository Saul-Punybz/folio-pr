#!/bin/bash
# Ollama initialization — used by docker compose to ensure models are ready
# Can be called by a docker compose service or as a post-start hook

OLLAMA_HOST="${OLLAMA_HOST:-http://ollama:11434}"
REQUIRED_MODELS=("llama3.2:3b" "nomic-embed-text")

wait_for_ollama() {
    local max_attempts=60
    local attempt=0
    while [ $attempt -lt $max_attempts ]; do
        if curl -sf "$OLLAMA_HOST/api/tags" > /dev/null 2>&1; then
            return 0
        fi
        sleep 5
        attempt=$((attempt + 1))
    done
    echo "ERROR: Ollama failed to start after $max_attempts attempts"
    return 1
}

pull_model() {
    local model=$1
    # Check if model already exists
    if curl -sf "$OLLAMA_HOST/api/tags" | grep -q "\"$model\""; then
        echo "Model $model already available"
        return 0
    fi
    echo "Pulling $model..."
    curl -sf "$OLLAMA_HOST/api/pull" -d "{\"name\": \"$model\"}" > /dev/null
    echo "Model $model pulled successfully"
}

echo "Waiting for Ollama..."
wait_for_ollama

for model in "${REQUIRED_MODELS[@]}"; do
    pull_model "$model"
done

echo "All models ready"
