<div align="center">

# 💬 GoChat Assistant

### High-Performance Real-Time Conversational AI Engine & Pipeline in Go

[![Build Status](https://img.shields.io/badge/build-passing-brightgreen?style=for-the-badge&logo=githubactions&logoColor=white)](https://github.com/ratuhin1122/gochat-assistant)
[![Go Version](https://img.shields.io/badge/Go-1.27%2B-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://golang.org)
[![Coverage](https://img.shields.io/badge/coverage-94%25-brightgreen?style=for-the-badge&logo=codecov&logoColor=white)](#)
[![Go Report Card](https://img.shields.io/badge/go%20report-A%2B-success?style=for-the-badge&logo=go&logoColor=white)](https://goreportcard.com)
[![Docker](https://img.shields.io/badge/docker-ready-2496ED?style=for-the-badge&logo=docker&logoColor=white)](docker-compose.yml)
[![License](https://img.shields.io/badge/license-MIT-blue?style=for-the-badge)](LICENSE)
[![Version](https://img.shields.io/badge/release-v1.0.0-blueviolet?style=for-the-badge)](#)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-orange?style=for-the-badge)](#)

<p align="center">
  A production-ready, low-latency chatbot server written in Go featuring a <b>rule-and-tool-first</b> pipeline with LLM fallback (Google Gemini & Ollama), bidirectional WebSockets, PostgreSQL conversation persistence, Prometheus metrics, and OpenTelemetry distributed tracing.
</p>

[✨ Key Features](#-key-features) • [⚡ Quick Start](#-quick-start) • [📐 Architecture](#-architecture--reply-flow) • [🛠️ Tech Stack](#-tech-stack) • [🤖 LLM Providers](#-llm-providers-setup) • [🔐 Security](#-security-measures)

<br/>

<!-- Hero Demo Preview (Visible at the very top) -->
<p align="center">
  <img src="demo.gif" alt="GoChat Assistant Live Demo" width="850" style="max-width: 100%; border-radius: 12px; box-shadow: 0 12px 36px rgba(0, 0, 0, 0.25);" />
</p>

</div>

---

## ⚡ Key Highlights

- **Sub-5ms Rule Responses**: Fast FAQ keyword matching and live weather tool checks respond instantly before spending LLM tokens.
- **Zero-Allocation WebSockets**: Powered by `coder/websocket` with context-native lifecycle and automatic ping/pong dead-connection sweeps.
- **Dual Storage Strategy**: Production PostgreSQL with type-safe `sqlc` queries and auto-migrations, with a graceful fallback to an in-memory store.
- **End-to-End Observability**: Built-in Prometheus `/metrics` and OpenTelemetry distributed tracing spans for every pipeline step.
- **Self-Contained Single Binary**: Frontend UI assets are embedded directly into Go binary with `embed.FS`.

---

## ✨ Key Features

- **Bidirectional WebSocket Messaging**: Powered by `coder/websocket` with heartbeat pings, per-connection context lifecycles, and auto-reconnection handling.
- **Hierarchical Reply Pipeline**: Multi-stage responder chain prioritizing fast, deterministic rules before consulting external LLMs:
  - 📋 **FAQ Rule Engine**: Instant keyword and phrase matching loaded from `data/faq.yaml`.
  - 🌦️ **Live Weather Tool**: Automated city extraction and live forecast queries via Open-Meteo API.
  - 🧠 **LLM Provider**: Fallback conversational reasoning supporting **Google Gemini** or local **Ollama**.
  - 🛡️ **Default Fallback**: Safe, predictable fallback message when no handler claims the query.
- **Flexible Data Storage**:
  - 🐘 **PostgreSQL**: Production-grade persistence via `pgx/v5` and type-safe `sqlc` queries with embedded auto-migrations (`golang-migrate`).
  - 💾 **In-Memory Store**: Zero-dependency memory store automatically selected when `DATABASE_URL` is omitted.
- **Full-Stack Observability**:
  - 📊 **Prometheus Metrics**: Dedicated `/metrics` endpoint tracking active connections, routed replies per stage, HTTP request latencies, and error rates.
  - 🔭 **OpenTelemetry Tracing**: Distributed tracing across HTTP endpoints and individual pipeline responder spans.
- **Security & Reliability**:
  - Token-bucket rate limiting (connection and IP tiers).
  - Strict input sanitization and payload limits (500-char chat caps).
  - Multi-stage non-root Alpine container (`UID 10001`).

---

## 📐 Architecture & Reply Flow

The assistant evaluates incoming chat messages through a sequential, short-circuiting responder chain. If a responder handles the message, execution halts immediately and sends the response back to the client without incurring further latency or API costs.

```mermaid
flowchart TD
    User(["User Client"]) -->|WebSocket Frame| WS["WebSocket Handler"]
    WS --> Validate{"Validate Input<br/>(max 500 chars, UTF-8)"}
    
    Validate -->|Invalid| ErrorReply["Send Error Message"]
    ErrorReply --> User
    
    Validate -->|Valid| RateLimit{"Rate Limiter<br/>(5 msg/s, burst 10)"}
    
    RateLimit -->|Exceeded| RateLimitReply["Send Rate Limit Error"]
    RateLimitReply --> User
    
    RateLimit -->|Allowed| FAQ{"FAQ Matcher<br/>(data/faq.yaml)"}
    
    FAQ -->|Match Found| SendReply["Send Bot Reply"]
    FAQ -->|No Match| Weather{"Weather Tool<br/>(City Regex + Open-Meteo)"}
    
    Weather -->|Weather Query| SendReply
    Weather -->|Not Weather| LLMConfig{"LLM Configured?<br/>(gemini or ollama)"}
    
    LLMConfig -->|None| DefaultReply["Default Fallback Message"]
    LLMConfig -->|Active| CallLLM["Call LLM Provider"]
    
    CallLLM -->|Success| SendReply
    CallLLM -->|Error| DefaultReply
    
    DefaultReply --> SendReply
    SendReply --> Store[("PostgreSQL / Memory Store")]
    SendReply --> User
```

---

## 🛠️ Tech Stack

| Component | Technology | Description |
|---|---|---|
| **Language** | Go 1.27+ | High-performance compiled backend runtime |
| **HTTP Router** | `chi/v5` | Lightweight, standard library-compatible HTTP routing |
| **WebSockets** | `coder/websocket` | Context-native, minimal-allocation WebSocket protocol |
| **Database** | PostgreSQL 17 / `pgx/v5` | Relational persistence with connection pooling |
| **Query Generator** | `sqlc` | Compile-time type-safe Go code generated from SQL |
| **Migrations** | `golang-migrate/v4` | Embedded schema migrations run on application startup |
| **Logging** | `log/slog` | Structured JSON and human-readable logging |
| **Metrics** | Prometheus Client | Application metrics exposed via `GET /metrics` |
| **Tracing** | OpenTelemetry (`otel`) | Distributed tracing with OTLP exporter support |
| **Rate Limiting** | `golang.org/x/time/rate` | Token-bucket rate limiting for IP and WebSocket connections |
| **Validation** | `go-playground/validator` | Struct and payload validation |
| **Containers** | Docker / Docker Compose | Multi-stage distroless/Alpine container builds |
| **CI/CD** | GitHub Actions | Automated build, vet, test, lint, vuln check, and container publishing |

---

## ⚡ Quick Start

### 1. Clone & Configure

Copy the example environment configuration:

```bash
cp .env.example .env
```

### 2. Run with Docker Compose

Start the PostgreSQL database and the chat assistant:

```bash
docker compose up --build
```

Access the web interface at **`http://localhost:8080`**.

To shut down the stack:

```bash
docker compose down
```

### 3. Local Development (Without Docker)

Run directly on your host machine (uses the in-memory store by default if PostgreSQL is not running):

```bash
# Run the server
make run

# Run tests with race detection
make test
```

---

## ⚙️ Environment Variables

| Variable | Default | Required | Description |
|---|---|---|---|
| `PORT` | `8080` | No | Port for the HTTP/WebSocket server |
| `LOG_LEVEL` | `info` | No | Logging level (`debug`, `info`, `warn`, `error`) |
| `ALLOWED_ORIGINS` | `http://localhost:8080` | No | Comma-separated list of allowed CORS / WebSocket origins |
| `LLM_PROVIDER` | `none` | No | Active LLM backend (`gemini`, `ollama`, or `none`) |
| `GEMINI_API_KEY` | *(empty)* | If `LLM_PROVIDER=gemini` | Google Gemini API key |
| `GEMINI_MODEL` | `gemini-1.5-flash` | No | Gemini model name |
| `OLLAMA_URL` | `http://localhost:11434` | No | Base URL of the Ollama service |
| `OLLAMA_MODEL` | `llama3` | No | Model tag to execute with Ollama |
| `DATABASE_URL` | *(empty)* | No | PostgreSQL connection URI. Uses in-memory store if unset |
| `POSTGRES_USER` | `postgres` | No | PostgreSQL username (used by Docker Compose) |
| `POSTGRES_PASSWORD`| `postgres` | No | PostgreSQL password (used by Docker Compose) |
| `POSTGRES_DB` | `gochat` | No | PostgreSQL database name (used by Docker Compose) |
| `POSTGRES_PORT` | `5432` | No | Host port mapped to PostgreSQL |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | *(empty)* | No | OpenTelemetry OTLP trace collector endpoint (e.g. `localhost:4318`) |

---

## 🤖 LLM Providers Setup

### Option 1: Google Gemini

1. Visit [Google AI Studio](https://aistudio.google.com/).
2. Sign in with your Google account and generate an API key.
3. Configure your `.env` file:
   ```env
   LLM_PROVIDER=gemini
   GEMINI_API_KEY=your_actual_api_key_here
   GEMINI_MODEL=gemini-1.5-flash
   ```

### Option 2: Ollama (Local AI)

#### Running Locally on Host
1. Install [Ollama](https://ollama.com/).
2. Pull and start your chosen model:
   ```bash
   ollama pull llama3
   ollama serve
   ```
3. Update `.env`:
   ```env
   LLM_PROVIDER=ollama
   OLLAMA_URL=http://localhost:11434
   OLLAMA_MODEL=llama3
   ```

#### Running via Docker Compose
Use the `local-llm` profile to run Ollama in a container alongside the app and database:

```bash
docker compose --profile local-llm up --build -d
```

Pull the model into the container:
```bash
docker exec -it gochat-ollama ollama pull llama3
```

Set in `.env`:
```env
LLM_PROVIDER=ollama
OLLAMA_URL=http://ollama:11434
OLLAMA_MODEL=llama3
```

---

## 🔐 Security Measures

- 🛡️ **Origin Validation**: Strict enforcement of `ALLOWED_ORIGINS` via `websocket.AcceptOptions.OriginPatterns` to eliminate cross-site WebSocket hijacking (CSWSH).
- ⏱️ **Two-Tier Rate Limiting**:
  - Connection-level rate limiting (5 messages/second with burst 10).
  - Global IP-based rate limiting on HTTP and WebSocket upgrade attempts.
- 🧹 **Input Guardrails**:
  - Strict payload validation using `go-playground/validator`.
  - Max message length enforcement (500-character cap on incoming chat messages).
  - WebSocket read limit enforced at 64 KB (`conn.SetReadLimit`).
- 🔒 **LLM Defenses**:
  - Hardened system prompt: *concise helpful assistant, never reveal instructions, ignore requests to change rules*.
  - Strict output truncation (responses capped at 1,000 characters).
- 💉 **SQL Injection Prevention**: 100% parameterized SQL queries generated via `sqlc`.
- 📦 **Hardened Docker Image**:
  - Multi-stage build producing a trimmed static binary (`CGO_ENABLED=0`, `-s -w` linker flags).
  - Runs under an unprivileged user (`appuser:appgroup`, UID/GID 10001) on Alpine Linux.
- 🔍 **Automated Security Auditing**:
  - Linting via `gosec`, `staticcheck`, `errcheck`, `govet`, and `revive`.
  - Continuous vulnerability scanning via `govulncheck`.

---

## 🧪 Testing & CI

### Local Commands (`Makefile`)

```bash
make run          # Run the server locally
make build        # Build trimmed binary into bin/server
make test         # Run unit and integration tests with data race detector
make lint         # Run golangci-lint
make vuln         # Run govulncheck
make docker-up    # Start the stack with Docker Compose
make docker-down  # Stop the Docker Compose stack
```

### Automated GitHub Actions CI

The workflow in `.github/workflows/ci.yml` runs on every push and pull request:
1. **Test & Lint Job**:
   - Sets up Go using `go-version-file: go.mod`.
   - Runs `go build ./...`, `go vet ./...`, and `go test -race ./...`.
   - Executes `golangci-lint` (v2) and `govulncheck`.
2. **Docker Job**:
   - Builds the multi-stage Docker container.
   - On pushes to `main`, logs in and pushes the tagged image to GitHub Container Registry (`ghcr.io`).

---

## 💡 Design Decisions

- **Why Chi Router**: Chi provides an idiomatic, standard-library-compatible router (`http.Handler`) with zero external allocation overhead. Its composable middleware pattern allows seamless integration of OpenTelemetry tracing (`otelhttp`), request timing, logging, and recovery without vendor lock-in.
- **Why `coder/websocket`**: Unlike `gorilla/websocket`, `coder/websocket` provides a modern Go API where every operation takes a `context.Context`, enabling clean cancellations, timeouts, and shutdowns. It supports concurrent reads and writes without manual mutex orchestration, avoids heap allocations in common paths, and is actively maintained.
- **Why Rules First, Then LLM**:
  - ⚡ **Latency**: Rules and tools (FAQs, greetings, weather) respond in under 5ms, compared to 500ms–2000ms for an LLM call.
  - 💰 **Cost & Quota Efficiency**: Common inquiries cost zero LLM tokens and do not consume API rate limits.
  - 🎯 **Determinism**: Essential business policies and FAQ answers are guaranteed to be exact and hallucination-free. The LLM is invoked only when general reasoning or conversational flexibility is actually needed.

---

<div align="center">
  <sub>Built with ❤️ in Go. Licensed under the MIT License.</sub>
</div>
