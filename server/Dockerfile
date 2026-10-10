# syntax=docker/dockerfile:1

# ==============================================================================
# Build Stage
# ==============================================================================
FROM golang:1.27.1-alpine AS builder

WORKDIR /src

# Install required build tools and certificates
RUN apk add --no-cache git ca-certificates tzdata

# Create unprivileged user for security
RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid 10001 \
    appuser

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source tree
COPY . .

# Compile static binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-w -s -extldflags '-static'" \
    -o /bin/server \
    ./cmd/server

# ==============================================================================
# Production Runtime Stage (Minimal Alpine)
# ==============================================================================
FROM alpine:3.21

# Install runtime certificates and timezone data
RUN apk add --no-cache ca-certificates tzdata

# Copy user definition from builder
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

# Copy compiled binary and migrations from builder
COPY --from=builder /bin/server /bin/server
COPY --from=builder /src/migrations /migrations

# Default environment configurations
ENV PORT=8080 \
    APP_DOMAIN=localhost \
    ENVIRONMENT=production \
    LOG_LEVEL=info \
    MIGRATIONS_PATH=/migrations

# Use non-root user
USER appuser:appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://127.0.0.1:${PORT}/healthz || exit 1

ENTRYPOINT ["/bin/server"]
