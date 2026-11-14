# Folio — Political Intelligence System
# Multi-stage Docker build for the single folio binary (frontend + migrations embedded)

# ── Stage 1: Build ────────────────────────────────────────────
FROM golang:1.23-alpine AS build

RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Cache dependency downloads
COPY go.mod go.sum ./
RUN go mod download

# Copy all source code
COPY . .

# Build the single binary (frontend/dist must exist: `make frontend` first)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/folio ./cmd/folio

# ── Stage 2: Runtime ─────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=build /out/folio /app/folio

EXPOSE 8080

CMD ["/app/folio"]
