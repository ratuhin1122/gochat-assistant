Project: gochat-assistant. A real-time chatbot server written in Go.

Stack: latest stable Go, chi router, coder/websocket, log/slog, pgx/v5 + sqlc, PostgreSQL, Prometheus client, OpenTelemetry, go-playground/validator, golang.org/x/time/rate, testify, gopkg.in/yaml.v3, Docker, Docker Compose, GitHub Actions.

Layout:
cmd/server/main.go
internal/config, internal/server, internal/chat, internal/nlu, internal/llm, internal/tools/weather, internal/store, internal/observability
web/ (embedded static UI), data/faq.yaml, migrations/

Reply flow: user message -> validate -> rate limit -> FAQ rules -> weather tool -> LLM provider (Gemini, or Ollama) -> default message. If no LLM is configured, the bot works with rules only.

Env vars: PORT, LOG_LEVEL, ALLOWED_ORIGINS, LLM_PROVIDER (gemini|ollama|none), GEMINI_API_KEY, GEMINI_MODEL, OLLAMA_URL, OLLAMA_MODEL, DATABASE_URL.

Rules:
- Do only the task requested. Do not edit unrelated files.
- Standard library first. Add a dependency only if the stack above lists it.
- Pass context.Context everywhere. Wrap errors with %w. No global mutable state.
- Keep interfaces small. Write table-driven tests for new logic.
- Never hardcode secrets. Never commit .env.
- WebSocket connections use a server ping every 30s with a 10s timeout; always cancel the connection context when the read loop exits.
- After each task run: go build ./... && go vet ./... && go test ./...
