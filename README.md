# Plomvix

> The Indian-built unified observability and general-purpose database.
> Supports logs, metrics, telemetry, key-value, and JSON.
> Built in Go. Open source. MIT licensed.

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

---

## What is Plomvix?

Plomvix is a unified observability database that also functions as a general-purpose
data store. It supports:

- **Logs** — structured and unstructured log storage
- **Metrics** — time-series metric storage and querying
- **Telemetry** — distributed tracing and span storage
- **Key-Value** — general key-value store
- **JSON** — document-style JSON storage with indexing

### Why Plomvix?

- **ClickHouse** requires manual schema definition before ingestion
- **Loki** struggles at high scale with complex queries
- **Zero Indian-built open-source observability databases exist**

Plomvix bridges these gaps — auto-inferred schemas, tiered storage, SQL querying,
and production-grade reliability — all in a single binary.

Made in India. Built for the world.

---

## Current Status

**v0.1.0 — Sprint 1 Complete**

- Boots as a real Go binary
- Loads configuration from YAML + environment variables
- Structured JSON/pretty logging
- HTTP server with middleware chain
- Health check endpoint (`GET /health`)
- Graceful shutdown on SIGINT/SIGTERM

**Coming next:** Sprint 2 — Auth system (JWT + API keys)

---

## Prerequisites

- **Go** 1.22 or higher ([install](https://go.dev/dl/))
- **make** (for Makefile commands)
- **git**
- **golangci-lint** (optional, for `make lint`)

---

## Getting Started

```bash
# Clone the repository
git clone https://github.com/plomvix/plomvix
cd plomvix

# Install dependencies
go mod tidy

# Review and edit configuration
# vi config.yaml

# Run Plomvix
make run

# Test the health endpoint
curl http://localhost:8080/health
```

### Expected health response

```json
{
  "status": "ok",
  "data": {
    "version": "dev",
    "env": "development",
    "uptime_seconds": 3,
    "pid": 12345,
    "go_version": "go1.22.0",
    "os_arch": "linux/amd64"
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

---

## Web UI

Plomvix includes a React-based web UI served from `/app/*`.

```bash
make ui-install   # install UI dependencies (first time only)
make dev          # start Go + Vite together for development
make build        # build both Go binary and React app for production
```

See [docs/ui.md](docs/ui.md) for details.

### UI Authentication

The Plomvix browser UI is available at `/login` and `/app/explore`.

- Default credentials: configured via `auth.default_admin_username` and `auth.default_admin_password` in `config.yaml`
- Login sets an httpOnly `plomvix_token` cookie (not accessible to JavaScript)
- API clients can continue using JWT bearer tokens (`Authorization: Bearer <token>`) or API keys (`X-API-Key`)
- **Do not** store JWTs in browser `localStorage` — the httpOnly cookie handles browser sessions automatically
- `make dev` starts both the Go server and Vite dev server for local development

---

## Theme Engine

Plomvix includes a design-token theme engine backed by `theme.json`.

- **Light/dark mode** — toggle from the UI header
- **Developer Design Panel** — accessible at `/dev/design` when `dev_panel` is `true` in `theme.json`
- **Live CSS variable injection** — tokens map to `--plx-*` CSS custom properties
- **Admin APIs** — `PUT /api/theme`, `POST /api/theme/reset`, `GET /api/theme/export`

See [docs/api/theme.md](docs/api/theme.md) for the full theme API reference.

To disable the design panel in production, set `"dev_panel": false` in `theme.json`.

---

## Make Commands

| Command | Description |
|---|---|
| `make run` | Run Plomvix without building a binary |
| `make build` | Build the Plomvix binary with version injected |
| `make test` | Run all tests with race detector and coverage |
| `make test-verbose` | Run all tests with verbose output |
| `make vet` | Run go vet static analysis |
| `make lint` | Run golangci-lint |
| `make tidy` | Tidy go modules |
| `make clean` | Remove binary and coverage output |
| `make coverage` | Generate HTML coverage report |
| `make help` | Show available make commands |

---

## Configuration Reference

### Environment

| Key | Type | Default | Description |
|---|---|---|---|
| `env` | string | `development` | Environment mode: `development` or `production` |

### Server

| Key | Type | Default | Description |
|---|---|---|---|
| `server.host` | string | `0.0.0.0` | Interface to bind to |
| `server.port` | int | `8080` | Port to listen on |
| `server.read_timeout` | int | `30` | HTTP read timeout (seconds) |
| `server.write_timeout` | int | `30` | HTTP write timeout (seconds) |
| `server.idle_timeout` | int | `60` | HTTP idle timeout (seconds) |

### Storage

| Key | Type | Default | Description |
|---|---|---|---|
| `storage.data_dir` | string | `./data` | Root data directory |
| `storage.wal_flush_threshold` | int | `67108864` | WAL flush size in bytes (64MB) |
| `storage.hot_tier_max_size` | int | `10737418240` | Max hot tier size in bytes (10GB) |
| `storage.retention_days` | int | `30` | Global data retention in days |

### Compression

| Key | Type | Default | Description |
|---|---|---|---|
| `compression.hot_tier` | string | `snappy` | Hot tier compression: `snappy`, `lz4`, `none` |
| `compression.cold_tier` | string | `zstd` | Cold tier compression: `zstd`, `snappy`, `none` |

### Indexing

| Key | Type | Default | Description |
|---|---|---|---|
| `indexing.auto_index_timestamp` | bool | `true` | Auto-index timestamp fields |

### Auth

| Key | Type | Default | Description |
|---|---|---|---|
| `auth.default_admin_username` | string | `admin` | Default admin username |
| `auth.default_admin_password` | string | `changeme` | Default admin password (CHANGE in production) |
| `auth.jwt_secret` | string | `plomvix-change-in-prod` | JWT signing secret (CHANGE in production) |
| `auth.jwt_expiry_seconds` | int | `3600` | JWT token lifetime (seconds) |
| `auth.api_key_length` | int | `32` | Generated API key length (bytes) |

### Logging

| Key | Type | Default | Description |
|---|---|---|---|
| `logging.level` | string | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `logging.format` | string | `pretty` | Log format: `json` (production) or `pretty` (development) |

### Environment Variable Overrides

Set any config key with environment variables using the `PLOMVIX_` prefix.
Example: `PLOMVIX_SERVER_PORT=9090` overrides `server.port`.

---

## Project Structure

```
plomvix/
├── cmd/plomvix/main.go           # Entry point
├── internal/
│   ├── config/config.go          # Config loader + validation
│   ├── logger/logger.go          # Structured logger
│   ├── server/server.go          # HTTP server + middleware + health
│   ├── auth/                     # Placeholder — Sprint 2
│   ├── ingestion/                # Placeholder — Sprint 5
│   ├── schema/                   # Placeholder — Sprint 5
│   ├── query/                    # Placeholder — Sprint 6
│   ├── storage/
│   │   ├── wal/                  # Placeholder — Sprint 3
│   │   ├── hot/                  # Placeholder — Sprint 4
│   │   └── cold/                 # Placeholder — Sprint 7
│   └── admin/                    # Placeholder — Sprint 9
├── pkg/utils/
│   ├── utils.go                  # Shared utilities
│   ├── utils_test.go             # Unit tests
│   └── response.go               # Standard API response envelope
├── data/                         # Runtime data (gitignored)
├── config.yaml                   # Main configuration
├── Makefile                      # Build, test, lint commands
├── .golangci.yml                 # Linter configuration
└── .gitignore
```

---

## Roadmap

- [x] Sprint 1 — Project skeleton (config, logger, HTTP server, health check)
- [x] Sprint 2 — Auth system (JWT + API key)
- [x] Sprint 3 — Write Ahead Log (WAL)
- [x] Sprint 4 — Hot tier (RocksDB)
- [ ] Sprint 5 — Ingestion API + Schema inference engine
- [ ] Sprint 6 — SQL query engine (hot tier)
- [ ] Sprint 7 — Cold tier (Parquet) + Tiering policy
- [ ] Sprint 8 — Multi-format parsers
- [ ] Sprint 9 — Admin APIs + Swagger docs
- [ ] Sprint 10 — Polish + Testing + Documentation

---

## Contributing

1. **Issues** — Report bugs and request features on [GitHub Issues](https://github.com/plomvix/plomvix/issues)
2. **Pull Requests** — Fork → branch → PR
3. **Code Quality** — Run `make vet` and `make lint` before submitting

---

## License

MIT — see [LICENSE](LICENSE)

---

*Plomvix — Built in India. Built for the world.*
