# Folio — Political Intelligence System
# Multi-stage Docker build for API and Worker binaries

# ── Stage 1: Build ────────────────────────────────────────────
FROM golang:1.22-alpine AS build

RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Cache dependency downloads
COPY go.mod go.sum ./
RUN go mod download

# Copy all source code
COPY . .

# Build the API binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/api ./cmd/api

# Build the Worker binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/worker ./cmd/worker

# ── Stage 2: Runtime ─────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy compiled binaries
COPY --from=build /out/api /app/api
COPY --from=build /out/worker /app/worker

# Copy frontend static assets (built externally via `make frontend`)
COPY frontend/dist /app/frontend/dist

# Copy database migrations
COPY migrations/ /app/migrations/

EXPOSE 8080

CMD ["/app/api"]
