# ── Build Stage ─────────────────────────────────────────────────────────────
FROM golang:alpine AS builder

WORKDIR /src

# Download dependencies first for optimal Docker layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build trimmed statically linked binary
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/server ./cmd/server

# ── Final Stage ─────────────────────────────────────────────────────────────
FROM alpine:latest

# Install CA certificates for outgoing HTTPS requests (e.g. Gemini, Weather APIs)
RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -S appgroup && \
    adduser -S appuser -G appgroup

WORKDIR /app

# Copy compiled binary from builder stage
COPY --from=builder --chown=appuser:appgroup /bin/server /app/server

# Run as non-root user
USER appuser:appgroup

EXPOSE 8080

ENTRYPOINT ["/app/server"]
