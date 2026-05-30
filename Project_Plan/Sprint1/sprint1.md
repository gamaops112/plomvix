# Plomvix — Sprint 1

> **Plomvix** is an Indian-built, open-source, unified observability and general-purpose database
> supporting logs, metrics, telemetry, key-value, and JSON data.
> Built in Go. Production grade. Resource friendly.

---

## Architecture Layers Overview

```
Layer 1  →  Project Skeleton + Config + Logging        ← Sprint 1
Layer 2  →  Auth System (JWT + API Key)
Layer 3  →  Write Ahead Log (WAL)
Layer 4  →  Hot Tier (RocksDB)
Layer 5  →  Ingestion API + Schema Inference Engine
Layer 6  →  SQL Query Engine (Hot Tier)
Layer 7  →  Cold Tier (Parquet) + Tiering Policy
Layer 8  →  Multi-Format Parsers
Layer 9  →  Admin APIs + Swagger Docs
Layer 10 →  Polish + Testing + Documentation
```

---

## Sprint 1 Goal

> By the end of Sprint 1, Plomvix should boot as a real Go binary.
> It should load its config, initialize its logger, create all required data directories,
> print a startup banner, start an HTTP server, and shut down gracefully.
> No business logic yet — but the skeleton is so clean and well structured
> that every future sprint simply fills in the blanks.

---

&nbsp;

## Feature 1 — Go Module Initialization

> Set up the Go module so the project has a proper identity,
> dependency management, and builds cleanly from a fresh clone.

---

### Story 1.1 — Initialize the Go Module

**What:**
Run `go mod init` to create the Go module with the correct module path.

**Module path:**
```
github.com/plomvix/plomvix
```

**Acceptance Criteria:**
- `go.mod` exists at the root of the project
- Module name is exactly `github.com/plomvix/plomvix`
- Go version is set to `1.22` or higher
- `go.sum` is created after first dependency is added
- Running `go build ./...` from root produces no errors

**Files produced:**
```
go.mod
go.sum
```

---

### Story 1.2 — Add Core Dependencies

**What:**
Add all third party Go libraries that Sprint 1 needs.
Do not add libraries for future sprints — only what is needed right now.

**Dependencies to add:**

| Library | Purpose |
|---|---|
| `github.com/spf13/viper` | Config loading from YAML + env vars |
| `github.com/go-chi/chi/v5` | HTTP router |
| `go.uber.org/zap` | Structured logger |
| `github.com/common-nighthawk/go-figure` | ASCII banner on startup |
| `github.com/google/uuid` | Request ID generation |

**How to add:**
```bash
go get github.com/spf13/viper
go get github.com/go-chi/chi/v5
go get go.uber.org/zap
go get github.com/common-nighthawk/go-figure
go get github.com/google/uuid
```

**Acceptance Criteria:**
- All five libraries present in `go.mod`
- `go mod tidy` runs without errors or warnings
- No unused dependencies in `go.mod`

---

&nbsp;

## Feature 2 — Folder Structure

> Establish the complete directory layout for the entire project upfront.
> Every future sprint will place its code inside this structure.
> Getting this right now prevents messy reorganization later.

---

### Story 2.1 — Create the Full Directory Layout

**What:**
Create all folders and placeholder files so the project structure is complete
and navigable from day one.

**Full directory tree:**
```
plomvix/
│
├── cmd/
│   └── plomvix/
│       └── main.go                  ← entry point
│
├── internal/
│   ├── config/
│   │   └── config.go                ← config loader
│   │
│   ├── logger/
│   │   └── logger.go                ← structured logger
│   │
│   ├── server/
│   │   └── server.go                ← HTTP server setup
│   │
│   ├── auth/
│   │   └── .gitkeep                 ← placeholder for Sprint 2
│   │
│   ├── ingestion/
│   │   └── .gitkeep                 ← placeholder for Sprint 5
│   │
│   ├── schema/
│   │   └── .gitkeep                 ← placeholder for Sprint 5
│   │
│   ├── query/
│   │   └── .gitkeep                 ← placeholder for Sprint 6
│   │
│   ├── storage/
│   │   ├── wal/
│   │   │   └── .gitkeep             ← placeholder for Sprint 3
│   │   ├── hot/
│   │   │   └── .gitkeep             ← placeholder for Sprint 4
│   │   └── cold/
│   │       └── .gitkeep             ← placeholder for Sprint 7
│   │
│   └── admin/
│       └── .gitkeep                 ← placeholder for Sprint 9
│
├── pkg/
│   └── utils/
│       ├── utils.go                 ← shared utility functions
│       ├── response.go              ← standard API response envelope
│       └── utils_test.go            ← unit tests
│
├── data/
│   ├── wal/                         ← WAL segments on disk
│   ├── hot/                         ← RocksDB files on disk
│   └── cold/
│       ├── logs/                    ← cold Parquet files for logs
│       ├── metrics/                 ← cold Parquet files for metrics
│       ├── json/                    ← cold Parquet files for JSON
│       └── kv/                      ← cold Parquet files for KV
│
├── config.yaml                      ← main config file
├── Makefile                         ← build, run, test, lint commands
├── .golangci.yml                    ← linter configuration
├── .gitignore                       ← ignore data/, binaries, .env
├── README.md                        ← project readme
└── sprint1.md                       ← this file
```

**Acceptance Criteria:**
- All folders exist
- Every `internal/` package that has no code yet has a `.gitkeep` file
- `data/` subdirectories all exist and are gitignored (data itself, not the folders)
- Project opens cleanly in any Go IDE with no missing package errors

---

### Story 2.2 — Create .gitignore

**What:**
Set up `.gitignore` so build artifacts, data files, and secrets
are never accidentally committed.

**Contents:**
```gitignore
# Binaries
plomvix
plomvixctl
*.exe

# Data directory contents (keep folder structure, ignore data files)
data/wal/*
data/hot/*
data/cold/*
!data/wal/.gitkeep
!data/hot/.gitkeep
!data/cold/logs/.gitkeep
!data/cold/metrics/.gitkeep
!data/cold/json/.gitkeep
!data/cold/kv/.gitkeep

# Config secrets
.env
*.local.yaml

# Go build cache
vendor/

# IDE
.idea/
.vscode/
*.swp

# Test coverage
coverage.out
coverage.html

# OS
.DS_Store
Thumbs.db
```

**Acceptance Criteria:**
- `.gitignore` is at the project root
- Running `git status` after creating test files in `data/` shows them as ignored
- `config.yaml` is NOT ignored — it is checked in with safe defaults
- `coverage.out` and build artifacts are ignored

---

&nbsp;

## Feature 3 — Configuration System

> Plomvix is fully config driven. Every tunable behaviour lives in `config.yaml`.
> The config system must load this file, validate it, support environment variable
> overrides, and make the config available to every part of the application
> through a single shared instance.

---

### Story 3.1 — Define config.yaml

**What:**
Create the master `config.yaml` file at the project root with all v1 configuration keys,
sensible defaults, and inline comments explaining each value.

**Full config.yaml:**
```yaml
# ─────────────────────────────────────────────
# Plomvix Configuration
# ─────────────────────────────────────────────

# Environment: development | production
env: development

server:
  host: 0.0.0.0         # Interface to bind to
  port: 8080            # Port to listen on
  read_timeout: 30      # HTTP read timeout in seconds
  write_timeout: 30     # HTTP write timeout in seconds
  idle_timeout: 60      # HTTP idle timeout in seconds

storage:
  data_dir: ./data                # Root directory for all data files
  wal_flush_threshold: 67108864   # WAL flush size in bytes (default: 64MB)
  hot_tier_max_size: 10737418240  # Max hot tier size in bytes (default: 10GB)
  retention_days: 30              # How many days to retain data globally

compression:
  hot_tier: snappy      # Compression for RocksDB hot tier (snappy / lz4 / none)
  cold_tier: zstd       # Compression for Parquet cold tier (zstd / snappy / none)

indexing:
  auto_index_timestamp: true   # Always index timestamp field automatically

auth:
  default_admin_username: admin
  default_admin_password: changeme        # CHANGE THIS before production use
  jwt_secret: plomvix-change-in-prod      # CHANGE THIS before production use
  jwt_expiry_seconds: 3600                # JWT token lifetime (default: 1 hour)
  api_key_length: 32                      # Length of generated API keys in bytes

logging:
  level: info           # Log level: debug / info / warn / error
  format: pretty        # Log format: json (production) / pretty (development)
```

**Acceptance Criteria:**
- File exists at project root
- All keys have inline comments
- Values are safe defaults — nothing here would break a fresh install
- Secrets have clear "CHANGE THIS before production" comments
- `env: development` is the default — production requires explicit override

---

### Story 3.2 — Write the Config Loader (config.go)

**What:**
Write `internal/config/config.go` that defines a `Config` struct matching
every key in `config.yaml`, loads it via Viper, supports env var overrides,
and exposes a shared instance across the application.

**Config struct definition:**
```go
type Config struct {
    Env         string            `mapstructure:"env"`
    Server      ServerConfig      `mapstructure:"server"`
    Storage     StorageConfig     `mapstructure:"storage"`
    Compression CompressionConfig `mapstructure:"compression"`
    Indexing    IndexingConfig    `mapstructure:"indexing"`
    Auth        AuthConfig        `mapstructure:"auth"`
    Logging     LoggingConfig     `mapstructure:"logging"`
}

type ServerConfig struct {
    Host         string `mapstructure:"host"`
    Port         int    `mapstructure:"port"`
    ReadTimeout  int    `mapstructure:"read_timeout"`
    WriteTimeout int    `mapstructure:"write_timeout"`
    IdleTimeout  int    `mapstructure:"idle_timeout"`
}

type StorageConfig struct {
    DataDir           string `mapstructure:"data_dir"`
    WALFlushThreshold int64  `mapstructure:"wal_flush_threshold"`
    HotTierMaxSize    int64  `mapstructure:"hot_tier_max_size"`
    RetentionDays     int    `mapstructure:"retention_days"`
}

type CompressionConfig struct {
    HotTier  string `mapstructure:"hot_tier"`
    ColdTier string `mapstructure:"cold_tier"`
}

type IndexingConfig struct {
    AutoIndexTimestamp bool `mapstructure:"auto_index_timestamp"`
}

type AuthConfig struct {
    DefaultAdminUsername string `mapstructure:"default_admin_username"`
    DefaultAdminPassword string `mapstructure:"default_admin_password"`
    JWTSecret            string `mapstructure:"jwt_secret"`
    JWTExpirySeconds     int    `mapstructure:"jwt_expiry_seconds"`
    APIKeyLength         int    `mapstructure:"api_key_length"`
}

type LoggingConfig struct {
    Level  string `mapstructure:"level"`
    Format string `mapstructure:"format"`
}
```

**Public API of config package:**
```go
// Load reads config from the given path, applies env overrides, validates, and stores instance
func Load(path string) (*Config, error)

// Get returns the loaded config instance — panics if Load() was never called
func Get() *Config

// IsDevelopment returns true if env is "development"
func (c *Config) IsDevelopment() bool

// IsProduction returns true if env is "production"
func (c *Config) IsProduction() bool
```

**Environment variable override format:**
```
PLOMVIX_ENV=production
PLOMVIX_SERVER_PORT=9090
PLOMVIX_AUTH_JWT_SECRET=mysecret
PLOMVIX_LOGGING_LEVEL=debug
PLOMVIX_LOGGING_FORMAT=json
```
Prefix is `PLOMVIX_`, nested keys joined with `_`, all uppercase.

**Acceptance Criteria:**
- `config.Load("config.yaml")` returns a fully populated `*Config` with no error on valid file
- `config.Load()` returns a descriptive error if the file is missing or malformed
- Environment variables override file values correctly
- `config.Get()` returns the same instance anywhere in the app after `Load()` is called once
- `config.Get()` panics with message `"plomvix: config not loaded — call config.Load() first"` if called before `Load()`

---

### Story 3.3 — Config Validation

**What:**
After loading, validate that all fields have acceptable values.
Plomvix must refuse to start with a broken config — fail fast with clear messages.

**Validation rules:**

| Field | Rule |
|---|---|
| `env` | Must be one of: `development`, `production` |
| `server.port` | Must be between 1 and 65535 |
| `server.host` | Must not be empty |
| `server.read_timeout` | Must be greater than 0 |
| `server.write_timeout` | Must be greater than 0 |
| `server.idle_timeout` | Must be greater than 0 |
| `storage.data_dir` | Must not be empty |
| `storage.wal_flush_threshold` | Must be greater than 0 |
| `storage.hot_tier_max_size` | Must be greater than 0 |
| `storage.retention_days` | Must be greater than 0 |
| `compression.hot_tier` | Must be one of: `snappy`, `lz4`, `none` |
| `compression.cold_tier` | Must be one of: `zstd`, `snappy`, `none` |
| `auth.default_admin_username` | Must not be empty |
| `auth.default_admin_password` | Must not be empty |
| `auth.jwt_secret` | Must not be empty |
| `auth.jwt_secret` | In `production` env — must not equal `plomvix-change-in-prod` |
| `auth.default_admin_password` | In `production` env — must not equal `changeme` |
| `auth.jwt_expiry_seconds` | Must be greater than 0 |
| `auth.api_key_length` | Must be between 16 and 64 |
| `logging.level` | Must be one of: `debug`, `info`, `warn`, `error` |
| `logging.format` | Must be one of: `json`, `pretty` |

**Error output format:**
```
Plomvix config validation failed:
  - server.port must be between 1 and 65535, got: 0
  - compression.hot_tier must be one of [snappy lz4 none], got: "brotli"
  - auth.jwt_secret must be changed from default in production mode
```

**Acceptance Criteria:**
- All validation errors are collected and returned together — not one at a time
- Each error message names the exact field and explains the rule
- Valid config passes with no errors
- Production-unsafe defaults are caught only when `env: production`
- Validation is called automatically inside `config.Load()`

---

&nbsp;

## Feature 4 — Internal Logger

> Every component of Plomvix needs structured, levelled logging.
> The logger must be initialized once at startup and be usable
> from every internal package without passing it around explicitly.

---

### Story 4.1 — Initialize Zap Logger

**What:**
Write `internal/logger/logger.go` using `go.uber.org/zap` that provides
a global structured logger configurable from the config system.

**Public API:**
```go
// Init initializes the global logger from config — must be called before any logging
func Init(cfg config.LoggingConfig) error

// Debug logs at debug level with structured fields
func Debug(msg string, fields ...zap.Field)

// Info logs at info level with structured fields
func Info(msg string, fields ...zap.Field)

// Warn logs at warn level with structured fields
func Warn(msg string, fields ...zap.Field)

// Error logs at error level with structured fields
func Error(msg string, fields ...zap.Field)

// Fatal logs at fatal level then calls os.Exit(1)
func Fatal(msg string, fields ...zap.Field)

// Sync flushes any buffered log entries — call on shutdown
func Sync()
```

**JSON format output example (production):**
```json
{"level":"info","ts":"2024-01-15T10:30:00Z","caller":"server/server.go:42","msg":"server started","port":8080,"host":"0.0.0.0"}
```

**Pretty format output example (development):**
```
2024-01-15T10:30:00Z  INFO  server/server.go:42  server started  {"port": 8080, "host": "0.0.0.0"}
```

**Acceptance Criteria:**
- `logger.Init()` must be called before any other logger function
- Calling any logger function before `Init()` panics with: `"plomvix: logger not initialized — call logger.Init() first"`
- Log level from config is respected — debug logs do not appear when level is `info`
- Logger is goroutine safe
- `logger.Fatal()` logs the message then calls `os.Exit(1)`
- Caller information (file + line number) is included in every log entry
- `logger.Sync()` is called during graceful shutdown to flush buffered entries

---

### Story 4.2 — Logger Fields Convention

**What:**
Define and document the standard structured fields used across all of Plomvix.
Consistency ensures logs are machine parseable and searchable.

**Standard fields by scenario:**

| Scenario | Required Fields |
|---|---|
| Server startup | `port`, `host`, `version`, `pid`, `env` |
| HTTP request | `method`, `path`, `status`, `latency_ms`, `request_id` |
| Storage operation | `operation`, `data_type`, `bytes`, `duration_ms` |
| Config load | `path`, `env` — never log secret values |
| Directory operation | `path`, `operation` (created / exists / failed) |
| Graceful shutdown | `timeout_seconds`, `reason` |
| Error | `error` string — `stack` only in development mode |

**Fields that must NEVER be logged:**
- `password` or `default_admin_password`
- `jwt_secret`
- `api_key`
- Any field containing the word `secret` or `token`

**Accepted field usage:**
```go
// Correct — structured fields
logger.Info("server started", zap.Int("port", 8080), zap.String("host", "0.0.0.0"))

// Wrong — string formatting
logger.Info(fmt.Sprintf("server started on port %d", 8080))
```

**Acceptance Criteria:**
- Field conventions are documented in a comment block at the top of `logger.go`
- No package in the project logs any of the forbidden fields
- All log calls across Sprint 1 use structured fields, not string formatting

---

&nbsp;

## Feature 5 — HTTP Server

> Plomvix's API layer starts here. Sprint 1 does not implement any
> business endpoints yet — but the HTTP server must be production grade
> from day one: correct timeouts, middleware chain, request ID tracking,
> and a health check that actually means something.

---

### Story 5.1 — Initialize Chi Router and Server Struct

**What:**
Write `internal/server/server.go` that creates and configures the HTTP server.

**Server struct and API:**
```go
type Server struct {
    router     *chi.Mux
    cfg        *config.Config
    httpServer *http.Server
    startTime  time.Time
}

// New creates a new Server instance with router and middleware configured
func New(cfg *config.Config) *Server

// Start begins listening — blocks until server stops
func (s *Server) Start() error

// Shutdown performs graceful shutdown waiting up to 30 seconds
func (s *Server) Shutdown(ctx context.Context) error

// Router returns the chi router for registering routes from other packages
func (s *Server) Router() *chi.Mux
```

**Acceptance Criteria:**
- `Server.Start()` begins listening on `host:port` from config
- `Server.Shutdown()` waits for in-flight requests to complete before stopping
- Read, write, and idle timeouts are applied from config values
- All server lifecycle events are logged using the field conventions from Story 4.2
- `startTime` is recorded so uptime can be calculated for the health endpoint

---

### Story 5.2 — Middleware Chain

**What:**
Set up the global middleware chain that applies to every request in order.

**Middleware stack in execution order:**

| Order | Middleware | What it does |
|---|---|---|
| 1 | Request ID | Generates UUID, attaches as `X-Request-ID` header on request and response |
| 2 | Logger | Logs every completed request with method, path, status, latency, request ID |
| 3 | Recoverer | Catches any handler panic, returns `500`, logs stack trace |
| 4 | Timeout | Cancels request context if handler exceeds `write_timeout` from config |
| 5 | Content-Type | Sets `Content-Type: application/json` on all responses |

**Request log line (one per request):**
```json
{
  "level": "info",
  "msg": "request completed",
  "method": "GET",
  "path": "/health",
  "status": 200,
  "latency_ms": 2,
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Acceptance Criteria:**
- Every response includes `X-Request-ID` header with a valid UUID v4
- Every request produces exactly one log line with all required fields
- A panicking handler returns `500` JSON error and does not crash the server
- Requests exceeding write timeout receive `503 Service Unavailable` in the standard error envelope
- Middleware runs in the exact order listed above

---

### Story 5.3 — Health Check Endpoint

**What:**
Implement `GET /health` — the only business endpoint in Sprint 1.

**Response when healthy — HTTP 200:**
```json
{
  "status": "ok",
  "data": {
    "version": "0.1.0",
    "env": "development",
    "uptime_seconds": 3600,
    "pid": 12345,
    "go_version": "go1.22.0",
    "os_arch": "linux/amd64"
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Response when degraded — HTTP 503:**
```json
{
  "status": "error",
  "error": {
    "code": "HEALTH_CHECK_FAILED",
    "message": "One or more health checks failed",
    "details": [
      "data directory not writable: ./data/wal",
      "data directory not writable: ./data/hot"
    ]
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Checks performed inside the handler:**
- Each `data/` subdirectory exists and is writable — tested by attempting to create and delete a temp file

**Acceptance Criteria:**
- Returns `200` with full data payload when all checks pass
- Returns `503` with list of specific failed checks when any check fails
- Response time is under 10ms — no heavy computation inside handler
- Does not require authentication — health must be checkable without a token
- Uses the standard response envelope from Story 7.4
- `uptime_seconds` is accurate — calculated from server `startTime`

---

&nbsp;

## Feature 6 — Main Entry Point

> `main.go` is the orchestrator. It wires everything together in the correct
> order, handles startup failures gracefully, and manages the process lifecycle.

---

### Story 6.1 — Wire Up main.go

**What:**
Write `cmd/plomvix/main.go` that boots Plomvix in a strict, correct sequence.

**Boot sequence — in this exact order:**
```
1.  Parse CLI flags (--version, --config)
2.  If --version flag → print version string and exit 0
3.  Print ASCII banner to stdout
4.  Load config from path (default: ./config.yaml)
5.  Validate config → if invalid: print all errors to stderr and exit 1
6.  Initialize logger from config
7.  Log "Plomvix starting..." with version, pid, env
8.  Bootstrap data directories → if any fail: log error and exit 1
9.  Initialize HTTP server
10. Register all routes (health only in Sprint 1)
11. Start listening for OS signals (SIGINT, SIGTERM) in background goroutine
12. Log "Plomvix ready" with host, port, env
13. Start HTTP server — blocks here
14. On signal received: log "shutting down...", call Server.Shutdown() with 30s timeout
15. Call logger.Sync() to flush buffered logs
16. Log "Plomvix stopped cleanly" and exit 0
```

**ASCII banner:**
```
 ____  _       __  __      _
|  _ \| | ___ |  \/  |_  _(_)_  __
| |_) | |/ _ \| |\/| \ \/ / \ \/ /
|  __/| | (_) | |  | |>  <| |>  <
|_|   |_|\___/|_|  |_/_/\_\_/_/\_\

The Indian-built observability database.
Version: 0.1.0  |  env: development
```

**Acceptance Criteria:**
- Banner prints before any log output
- Config path is configurable via `--config` flag, defaults to `./config.yaml`
- Config load failure → all errors printed to stderr → exit code `1`
- Port already in use → clear error logged → exit code `1`
- `CTRL+C` or `SIGTERM` → graceful shutdown → exit code `0`
- Shutdown waits up to 30 seconds for in-flight requests
- `logger.Sync()` is always called before process exits

---

### Story 6.2 — Data Directory Bootstrap

**What:**
On every startup, before the server starts, verify the full `data/` directory
structure exists. Create any missing directories automatically.

**Directories to verify and create if missing:**
```
{data_dir}/wal/
{data_dir}/hot/
{data_dir}/cold/logs/
{data_dir}/cold/metrics/
{data_dir}/cold/json/
{data_dir}/cold/kv/
```

`data_dir` comes from `storage.data_dir` in config.

**Log output per directory:**
```json
{"level":"debug","msg":"data directory ready","path":"./data/wal","operation":"exists"}
{"level":"debug","msg":"data directory ready","path":"./data/hot","operation":"created"}
```

**Acceptance Criteria:**
- On a completely fresh install — all subdirectories are created automatically
- If all directories already exist — silent at `info` level, visible only at `debug`
- If a directory cannot be created (permissions) → log at `error` level → exit code `1`
- Uses `utils.EnsureDir()` from `pkg/utils`

---

### Story 6.3 — Version Flag + Build-Time Version Injection

**What:**
Support `--version` CLI flag. Version string must be injected at build time
via ldflags so it reflects the actual release, not a hardcoded string.

**Version flag output:**
```bash
$ ./plomvix --version
Plomvix v0.1.0 (go1.22.0, linux/amd64)
```

**Build-time variables in main.go:**
```go
var (
    Version   = "dev"               // overridden by ldflags at build
    BuildTime = "unknown"           // overridden by ldflags at build
    GitCommit = "unknown"           // overridden by ldflags at build
)
```

**Makefile build command (Story 7.1) injects these automatically:**
```bash
go build \
  -ldflags "-X main.Version=0.1.0 \
             -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
             -X main.GitCommit=$(git rev-parse --short HEAD)" \
  -o plomvix ./cmd/plomvix
```

**Environment mode:**
- `env` from config determines behaviour differences:
  - `development` → pretty logs, unsafe defaults allowed, stack traces in errors
  - `production` → JSON logs, unsafe defaults rejected by validator, no stack traces

**Acceptance Criteria:**
- `./plomvix --version` prints version string and exits `0` without starting server
- When built with `make build`, version reflects actual ldflags values
- When run with `go run`, version shows `dev` — expected and acceptable
- `env` from config is logged on startup and included in banner
- `BuildTime` and `GitCommit` are logged at startup at `debug` level

---

&nbsp;

## Feature 7 — Shared Utilities

> Small, reusable helper functions that multiple packages will need.
> Defined once in `pkg/utils/` to avoid duplication across the codebase.

---

### Story 7.1 — Makefile

**What:**
Write a `Makefile` at the project root with all common developer commands.
Every developer action should be one `make` command — no one should have to
remember long Go commands.

**Makefile targets:**

```makefile
.PHONY: run build test lint tidy clean help

VERSION   ?= 0.1.0
BINARY     = plomvix
BUILD_TIME = $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT = $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS    = -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

## run: Run Plomvix without building a binary
run:
	go run $(LDFLAGS) ./cmd/plomvix

## build: Build the Plomvix binary
build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/plomvix

## test: Run all tests with race detector
test:
	go test -race -cover ./...

## test-verbose: Run all tests with verbose output
test-verbose:
	go test -race -cover -v ./...

## lint: Run golangci-lint
lint:
	golangci-lint run ./...

## tidy: Tidy go modules
tidy:
	go mod tidy

## clean: Remove built binary and coverage output
clean:
	rm -f $(BINARY) coverage.out coverage.html

## coverage: Run tests and open HTML coverage report
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	open coverage.html

## help: Show this help message
help:
	@grep -E '^## ' Makefile | sed 's/## //'
```

**Acceptance Criteria:**
- `make run` starts Plomvix correctly
- `make build` produces a `plomvix` binary with correct version injected
- `make test` runs all tests with race detector enabled
- `make lint` runs linter (requires `golangci-lint` installed)
- `make clean` removes binary and coverage files
- `make help` prints all available commands with descriptions
- Makefile works on Linux and macOS

---

### Story 7.2 — Linter Configuration (.golangci.yml)

**What:**
Write `.golangci.yml` at the project root to configure `golangci-lint`
with a sensible set of linters that enforce code quality from day one.

**Configuration:**
```yaml
run:
  timeout: 5m
  go: "1.22"

linters:
  enable:
    - errcheck        # check that errors are handled
    - gosimple        # simplification suggestions
    - govet           # report suspicious constructs
    - ineffassign     # detect unused variable assignments
    - staticcheck     # static analysis
    - unused          # find unused code
    - gofmt           # enforce gofmt formatting
    - goimports       # enforce import grouping
    - misspell        # catch common spelling mistakes
    - godot           # check that comments end with a period
    - revive          # fast, configurable linter

linters-settings:
  revive:
    rules:
      - name: exported
        severity: warning

issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - errcheck    # allow unhandled errors in tests
```

**Acceptance Criteria:**
- `make lint` runs without configuration errors
- `errcheck` catches unhandled errors across the codebase
- `gofmt` enforces consistent formatting
- Test files are excluded from `errcheck` to reduce noise
- Linter passes cleanly on Sprint 1 code before moving to Sprint 2

---

### Story 7.3 — Write pkg/utils/utils.go

**What:**
Implement shared utility functions used across Sprint 1 and referenced in future sprints.

**Functions to implement:**

```go
// DirExists returns true if the path exists and is a directory
func DirExists(path string) bool

// EnsureDir creates the directory and all parents if they do not exist
// Returns nil if already exists, error if creation fails
func EnsureDir(path string) error

// IsWritable returns true if the process can write to the given directory
// Tests by attempting to create and immediately delete a temp file
func IsWritable(path string) bool

// BytesToHuman converts bytes to a human readable string
// 1024 → "1.0 KB", 67108864 → "64.0 MB", 10737418240 → "10.0 GB"
func BytesToHuman(bytes int64) string

// GetGoVersion returns the current Go runtime version string
// Example: "go1.22.0"
func GetGoVersion() string

// GetOSArch returns the current OS and architecture string
// Example: "linux/amd64"
func GetOSArch() string

// NewRequestID generates a new UUID v4 string for use as a request identifier
func NewRequestID() string
```

**Acceptance Criteria:**
- Every function has a godoc comment
- Every function has unit tests in `pkg/utils/utils_test.go`
- `EnsureDir` is idempotent — calling it twice on the same path returns nil both times
- `BytesToHuman` handles B, KB, MB, GB, TB correctly with one decimal place
- `IsWritable` cleans up its temp file — no files left behind after the check
- `NewRequestID` always returns a valid UUID v4 format string

---

### Story 7.4 — Standard API Response Envelope (response.go)

**What:**
Write `pkg/utils/response.go` defining the standard JSON response shape
used by every endpoint in Plomvix — Sprint 1 through Sprint 9.
Defined once here so all future sprints import and use it consistently.

**Response structs:**
```go
// Success response envelope
type Response struct {
    Status    string      `json:"status"`               // always "ok"
    Data      interface{} `json:"data,omitempty"`        // response payload
    RequestID string      `json:"request_id,omitempty"` // from X-Request-ID header
}

// Error response envelope
type ErrorResponse struct {
    Status    string     `json:"status"`               // always "error"
    Error     ErrorBody  `json:"error"`
    RequestID string     `json:"request_id,omitempty"`
}

type ErrorBody struct {
    Code    string   `json:"code"`              // machine readable error code e.g. "VALIDATION_FAILED"
    Message string   `json:"message"`           // human readable description
    Details []string `json:"details,omitempty"` // optional list of specific issues
}
```

**Helper functions:**
```go
// OK writes a 200 JSON success response
func OK(w http.ResponseWriter, r *http.Request, data interface{})

// Created writes a 201 JSON success response
func Created(w http.ResponseWriter, r *http.Request, data interface{})

// BadRequest writes a 400 JSON error response
func BadRequest(w http.ResponseWriter, r *http.Request, code, message string, details ...string)

// Unauthorized writes a 401 JSON error response
func Unauthorized(w http.ResponseWriter, r *http.Request, message string)

// Forbidden writes a 403 JSON error response
func Forbidden(w http.ResponseWriter, r *http.Request, message string)

// NotFound writes a 404 JSON error response
func NotFound(w http.ResponseWriter, r *http.Request, message string)

// InternalError writes a 500 JSON error response
func InternalError(w http.ResponseWriter, r *http.Request, message string)

// ServiceUnavailable writes a 503 JSON error response
func ServiceUnavailable(w http.ResponseWriter, r *http.Request, code, message string, details ...string)
```

**Standard error codes:**
```
VALIDATION_FAILED
UNAUTHORIZED
FORBIDDEN
NOT_FOUND
INTERNAL_ERROR
HEALTH_CHECK_FAILED
SERVICE_UNAVAILABLE
```

**Acceptance Criteria:**
- Every helper function reads `X-Request-ID` from the request and includes it in the response
- All responses set `Content-Type: application/json`
- All helpers are usable from any package via `utils.OK(w, r, data)`
- Response structs match the examples in Story 5.3 exactly
- No handler in Sprint 1 writes raw JSON manually — all use these helpers

---

&nbsp;

## Feature 8 — README

> README is not an afterthought. It is the first thing any contributor,
> user, or evaluator reads. Sprint 1 README should be honest about
> what Plomvix is, what it is not yet, and how to run it.

---

### Story 8.1 — Write README.md

**What:**
Write a complete `README.md` — not a placeholder. Someone should be able
to clone the repo, read this, and have Plomvix running in under 5 minutes.

**Sections:**

```
1. What is Plomvix
   - What it is, what problem it solves
   - Why it exists — the gap in the market
   - Made in India, open source, MIT licensed

2. Current Status
   - Sprint 1 complete — skeleton, config, logger, HTTP server, health check
   - What works right now
   - What is coming (link to sprint plan)

3. Prerequisites
   - Go 1.22 or higher
   - make (for Makefile commands)
   - git

4. Getting Started
   - git clone
   - go mod tidy
   - Edit config.yaml (explain the key fields)
   - make run
   - curl GET /health → show expected response

5. Available Make Commands
   - Table of all make targets and what they do

6. Configuration Reference
   - Table: key | type | default | description
   - Cover all keys in config.yaml

7. Project Structure
   - Annotated directory tree matching Story 2.1

8. Roadmap
   - Sprint 1 through Sprint 9 listed as checklist
   - Sprint 1 checked off

9. Contributing
   - How to raise issues
   - How to submit PRs
   - Code style: run make lint before submitting

10. License
    - MIT License
```

**Acceptance Criteria:**
- README renders correctly on GitHub — all code blocks, tables, and headers display properly
- Getting Started section is tested — a developer on a fresh machine can follow it and get a `200` from `/health`
- No placeholder text like "TODO" or "coming soon" without a Sprint reference
- Config reference table covers every key in `config.yaml`
- Plomvix name and Indian identity are stated clearly in section 1

---

&nbsp;

## Sprint 1 — Definition of Done

Sprint 1 is complete when **all of the following are true:**

- [ ] `go mod tidy` runs with zero errors
- [ ] `make build` produces the `plomvix` binary with version injected via ldflags
- [ ] `./plomvix --version` prints version string and exits cleanly
- [ ] `make run` boots Plomvix — banner prints, config loads, server starts
- [ ] `GET /health` returns `200 OK` with correct JSON response envelope
- [ ] `GET /health` returns `503` with issue details when data directory is not writable
- [ ] All config validation rules enforced — bad config exits with all errors listed
- [ ] `PLOMVIX_SERVER_PORT=9090` env var overrides port correctly
- [ ] Production unsafe defaults (`changeme`, `plomvix-change-in-prod`) rejected when `env: production`
- [ ] Logs are JSON format when `logging.format: json`
- [ ] Logs are pretty format when `logging.format: pretty`
- [ ] Log level from config is respected — debug logs invisible at `info` level
- [ ] Every HTTP response includes `X-Request-ID` header
- [ ] Every HTTP request produces exactly one structured log line
- [ ] A panicking handler returns `500` and does not crash the server
- [ ] `CTRL+C` triggers graceful shutdown — log confirms clean stop
- [ ] All `data/` subdirectories auto-created on first boot
- [ ] Permission failure on data dir creation exits with code `1` and clear error
- [ ] `make test` passes with zero failures and race detector enabled
- [ ] `make lint` passes with zero issues
- [ ] All utility functions in `pkg/utils` have passing unit tests
- [ ] README Getting Started section works end to end on a fresh machine

---

&nbsp;

## Sprint 1 — Story Summary

| Feature | Story | Description |
|---|---|---|
| 1 — Go Module | 1.1 | Initialize Go module |
| 1 — Go Module | 1.2 | Add core dependencies |
| 2 — Folder Structure | 2.1 | Create full directory layout |
| 2 — Folder Structure | 2.2 | Create .gitignore |
| 3 — Config System | 3.1 | Define config.yaml |
| 3 — Config System | 3.2 | Write config loader |
| 3 — Config System | 3.3 | Config validation |
| 4 — Logger | 4.1 | Initialize Zap logger |
| 4 — Logger | 4.2 | Logger fields convention |
| 5 — HTTP Server | 5.1 | Initialize Chi router and server struct |
| 5 — HTTP Server | 5.2 | Middleware chain |
| 5 — HTTP Server | 5.3 | Health check endpoint |
| 6 — Main Entry | 6.1 | Wire up main.go |
| 6 — Main Entry | 6.2 | Data directory bootstrap |
| 6 — Main Entry | 6.3 | Version flag + build-time injection + env mode |
| 7 — Utilities | 7.1 | Makefile |
| 7 — Utilities | 7.2 | Linter configuration (.golangci.yml) |
| 7 — Utilities | 7.3 | Write pkg/utils/utils.go |
| 7 — Utilities | 7.4 | Standard API response envelope (response.go) |
| 8 — README | 8.1 | Write README.md |
| **Total** | **20 stories** | |

---

&nbsp;

*Plomvix — Built in India. Built for the world.*