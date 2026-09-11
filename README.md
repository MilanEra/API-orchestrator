# 🌐 API Orchestrator

A lightweight, config-driven API gateway and orchestrator written in **pure Go** (standard library only, zero external dependencies).

It aggregates data from multiple external APIs in parallel, applies production-grade resilience patterns, caches responses, normalizes output, and serves a built-in web UI — all from a single static binary.

> 💡 Think of it as a mini **KrakenD** or **Kong** that you can fully understand, extend, and deploy anywhere.

---

## ✨ Key Features

- **Config-driven sources** — add, remove, or modify API sources by editing a JSON file. No recompilation required.
- **Parallel multi-source aggregation** — query multiple sources in one request using goroutines.
- **Resilience triad**:
  - 🔁 **Retry** with exponential backoff and configurable retryable HTTP status codes.
  - 🛡️ **Circuit Breaker** (Closed → Open → Half-Open) to protect against cascading failures.
  - ⏱️ **Token Bucket Rate Limiter** — no busy-waiting, blocks gracefully via channels.
- **In-memory TTL cache** — interface-based design (ready for Redis swap), deterministic cache keys.
- **Response mapping** — extract nested fields via dot-path notation (`"current.temperature_2m"`), with automatic passthrough when no mapping is defined.
- **Graceful shutdown** — handles `SIGINT`/`SIGTERM`, drains in-flight requests before exit.
- **Embedded Web UI** — single-file HTML served via `//go:embed`, no static file server needed.
- **Structured logging** via `log/slog`.
- **Zero external dependencies** — built entirely on Go's standard library.
- **Table-driven unit tests** for critical modules (`circuitbreaker`, `parser`).

---

## 🏗 Architecture & Modules

```
cmd/server/              → HTTP server, routing, graceful shutdown, embedded UI
internal/
├── cache/               → Cache interface + in-memory TTL implementation
├── circuitbreaker/      → State machine: Closed / Open / Half-Open
├── config/              → JSON configuration loader and SourceConfig model
├── fetcher/             → Source interface + HTTPSource (retry, ratelimit, headers)
├── orchestrator/        → Parallel execution, aggregation, cache integration
├── parser/              → Dot-path response mapping with passthrough fallback
├── ratelimit/           → Token bucket limiter (channel-based, no busy-wait)
├── registry/            → Source registry with dynamic registration
└── retry/               → Exponential backoff with status-code filtering
configs/
└── sources.json         → Declarative source definitions
```

---

## 🚀 Quick Start

### Prerequisites
- Go 1.21+ installed
- Internet connection (for external APIs)

### Run from source
```bash
go run ./cmd/server
```

### Build a single binary
```bash
go build -o api-orchestrator ./cmd/server
./api-orchestrator
```

Then open **http://localhost:8080** for the web UI, or hit the API directly:

```bash
# Single source
curl "http://localhost:8080/fetch?source=weather"

# Multiple sources in parallel
curl "http://localhost:8080/fetch?sources=weather,currency,github&base=EUR"
```

---

## 🔌 API Endpoints

| Method | Path                | Description                                      |
|--------|---------------------|--------------------------------------------------|
| GET    | `/`                 | Embedded Web UI                                  |
| GET    | `/ping`             | Health check                                     |
| GET    | `/fetch`            | Fetch data from one or multiple sources          |
| GET    | `/sources/list`     | List all registered sources with their metadata  |
| GET    | `/sources/status`   | Circuit breaker states per source                |
| GET    | `/cache/stats`      | Cache statistics (active / expired / total)        |
| POST   | `/cache/clear`      | Flush the entire cache                           |

### Query Parameters for `/fetch`
- `source=weather` — single source
- `sources=weather,currency,github` — multiple sources (comma-separated)
- Any additional parameters (e.g. `base=EUR`, `latitude=51.5`) are forwarded to the matching sources based on their config.

---

## ⚙️ Configuration

Sources are defined in `configs/sources.json`. Adding a new API requires **zero code changes** — just append a new entry:

```json
{
  "name": "weather",
  "url": "https://api.open-meteo.com/v1/forecast?latitude={latitude}&longitude={longitude}&current=temperature_2m,weather_code",
  "method": "GET",
  "headers": { "Accept": "application/json" },
  "default_params": {
    "latitude": "55.7558",
    "longitude": "37.6173"
  },
  "timeout_seconds": 10,
  "cache_ttl_seconds": 60,
  "retry": {
    "max_attempts": 3,
    "initial_delay_ms": 500,
    "max_delay_ms": 5000,
    "retryable_status_codes": [408, 429, 500, 502, 503, 504]
  },
  "rate_limit": { "requests_per_second": 2 },
  "circuit_breaker": {
    "failure_threshold": 3,
    "success_threshold": 2,
    "recovery_timeout_seconds": 30
  },
  "response_mapping": {
    "temperature":  "current.temperature_2m",
    "weather_code": "current.weather_code",
    "time":         "current.time"
  }
}
```

> 📌 If `response_mapping` is omitted or empty, the full upstream response is returned as-is (**passthrough mode**).

### Currently bundled sources
- **weather** → [Open-Meteo](https://open-meteo.com/) (no API key required)
- **currency** → [ExchangeRate-API](https://open.er-api.com/) (free tier)
- **github** → GitHub REST API (public repos, `User-Agent` header auto-injected)

---

## 🧪 Testing

The project uses Go's standard `testing` package with **table-driven tests** for critical modules.

```bash
go test ./...
```

### Covered modules
- **`internal/circuitbreaker`** — state transitions (Closed → Open → Half-Open → Closed), error thresholds, recovery timeout.
- **`internal/parser`** — dot-path mapping, nested fields, type handling, invalid JSON, missing paths, passthrough behavior.

> 🛠 Tests for `retry`, `ratelimit`, `cache`, and `orchestrator` are planned for the next iteration using `httptest.Server` mocks.

---

## 📂 Project Structure

```
api-orchestrator/
├── cmd/server/
│   ├── main.go              # Entry point, routing, graceful shutdown
│   └── web/index.html       # Embedded UI (served via //go:embed)
├── configs/
│   └── sources.json         # Declarative source configuration
├── internal/
│   ├── cache/               # TTL cache (interface + memory impl)
│   ├── circuitbreaker/      # Circuit breaker pattern
│   ├── config/              # JSON config loader
│   ├── fetcher/             # HTTP source with retry + ratelimit
│   ├── orchestrator/        # Parallel aggregation engine
│   ├── parser/              # Response field mapping
│   ├── ratelimit/           # Token bucket limiter
│   ├── registry/            # Source registry
│   └── retry/               # Exponential backoff
├── go.mod
└── README.md
```

---

## 🗺 Roadmap

### ✅ Done
- Core orchestrator with parallel execution
- Resilience patterns (retry, circuit breaker, token bucket rate limiter)
- TTL cache with deterministic keys
- Config-driven source management
- Embedded web UI
- Graceful shutdown
- Table-driven tests for core modules

### 🔜 Planned
- [ ] **Hot-reload configuration** — CRUD API for sources, no restart required
- [ ] **Live mode** — WebSocket / SSE push updates to the UI
- [ ] **GitHub Webhooks** — event-driven source updates with HMAC signature verification
- [ ] **Distributed cache** — Redis backend implementation of the `Cache` interface
- [ ] **Observability** — Prometheus metrics (`/metrics`) + Grafana dashboard
- [ ] **Authentication** — API keys / JWT for `/fetch` endpoint
- [ ] **AI summarization source** — aggregate multiple sources via LLM API
- [ ] **Full test coverage** — `httptest`-based integration tests for all modules

---

## 🤝 Inspirations & References

This project draws inspiration from production-grade API gateways and orchestrators:
- [KrakenD](https://github.com/krakend/krakend-ce) — declarative API gateway
- [TrueTickets/api-aggregator](https://github.com/TrueTickets/api-aggregator) — Go aggregation patterns
- [PayRam/api-orchestrator-go](https://github.com/PayRam/api-orchestrator-go) — metadata-driven orchestration
- [ahadb/api-gateway](https://github.com/ahadb/api-gateway) — resilience pattern showcase

---

## 📄 License

MIT — free to use, modify, and showcase in your portfolio.

---

*Built with ❤️*