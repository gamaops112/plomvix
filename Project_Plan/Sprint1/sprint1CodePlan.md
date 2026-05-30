# Plomvix — Sprint 1 Code Plan
### For: DeepSeek V4 Coding Agent
### Language: Go 1.22 | Module: github.com/plomvix/plomvix

> Execute tasks in exact order. Each task is atomic — one file or one concern.
> Do not skip ahead. Each task depends on the previous.
> Every task has a Verify step — do not proceed until it passes.

---

## CONTEXT

You are building **Plomvix** — a unified observability database in Go.
Sprint 1 goal: a Go binary that boots, loads config, starts an HTTP server, and shuts down gracefully.
No database logic yet. Foundation only.

---

## TASK 01 — Initialize Go module

**Action:** Run in the project root:
```bash
go mod init github.com/plomvix/plomvix
```

**Verify:** `cat go.mod` shows module `github.com/plomvix/plomvix` and `go 1.22` or higher.

---

## TASK 02 — Create folder structure, stub main.go, and git init

> NOTE: Folder structure and stub main.go must exist BEFORE installing
> dependencies — `go get` requires at least one `.go` file in the module.
> `git init` must happen here so TASK 04's verify commands work.

**Action — Part A:** Create all directories:
```bash
mkdir -p cmd/plomvix
mkdir -p internal/config
mkdir -p internal/logger
mkdir -p internal/server
mkdir -p internal/auth
mkdir -p internal/ingestion
mkdir -p internal/schema
mkdir -p internal/query
mkdir -p internal/storage/wal
mkdir -p internal/storage/hot
mkdir -p internal/storage/cold
mkdir -p internal/admin
mkdir -p pkg/utils
mkdir -p data/wal
mkdir -p data/hot
mkdir -p data/cold/logs
mkdir -p data/cold/metrics
mkdir -p data/cold/json
mkdir -p data/cold/kv
```

**Action — Part B:** Create `.gitkeep` files in all placeholder directories:
```bash
touch internal/auth/.gitkeep
touch internal/ingestion/.gitkeep
touch internal/schema/.gitkeep
touch internal/query/.gitkeep
touch internal/storage/wal/.gitkeep
touch internal/storage/hot/.gitkeep
touch internal/storage/cold/.gitkeep
touch internal/admin/.gitkeep
touch data/wal/.gitkeep
touch data/hot/.gitkeep
touch data/cold/logs/.gitkeep
touch data/cold/metrics/.gitkeep
touch data/cold/json/.gitkeep
touch data/cold/kv/.gitkeep
```

**Action — Part C:** Create stub `cmd/plomvix/main.go`:
```go
package main

func main() {}
```
This stub is replaced completely in TASK 14. It exists only so `go get` works.

**Action — Part D:** Initialize git repository:
```bash
git init
```

**Verify:**
- `find . -name ".gitkeep" | wc -l` outputs `14`
- `cat cmd/plomvix/main.go` shows the stub
- `ls .git` confirms git is initialized

---

## TASK 03 — Install dependencies

**Action:**
```bash
go get github.com/spf13/viper
go get github.com/go-chi/chi/v5
go get go.uber.org/zap
go get github.com/common-nighthawk/go-figure
go get github.com/google/uuid
go mod tidy
```

**Verify:** All 5 packages appear in `go.mod`. `go mod tidy` exits with no errors.

---

## TASK 04 — Create .gitignore

**Action:** Create `.gitignore` at project root:

```gitignore
# Binaries
plomvix
plomvixctl
*.exe

# Data directory contents
# Using ** glob so nested subdirectories are handled correctly.
# Subdirectory folders themselves are un-ignored so git tracks the structure.
data/wal/**
data/hot/**
data/cold/**
!data/wal/.gitkeep
!data/hot/.gitkeep
!data/cold/logs/
!data/cold/metrics/
!data/cold/json/
!data/cold/kv/
!data/cold/logs/.gitkeep
!data/cold/metrics/.gitkeep
!data/cold/json/.gitkeep
!data/cold/kv/.gitkeep

# Config secrets
.env
*.local.yaml

# Go
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

# Test scripts
smoke_test.sh
```

**Verify:**
```bash
git add .
git check-ignore -v data/wal/somefile.log      # must be ignored
git check-ignore -v data/cold/logs/.gitkeep    # must NOT be ignored (empty output)
git ls-files data/                              # must show all 14 .gitkeep files
```

---

## TASK 05 — Create config.yaml

**Action:** Create `config.yaml` at project root:
```yaml
# Plomvix Configuration

# Environment mode: development | production
# In production mode, unsafe defaults are rejected at startup.
env: development

server:
  host: 0.0.0.0
  port: 8080
  read_timeout: 30    # seconds
  write_timeout: 30   # seconds
  idle_timeout: 60    # seconds

storage:
  data_dir: ./data
  wal_flush_threshold: 67108864    # 64MB in bytes
  hot_tier_max_size: 10737418240   # 10GB in bytes
  retention_days: 30

compression:
  hot_tier: snappy    # options: snappy | lz4 | none
  cold_tier: zstd     # options: zstd | snappy | none

indexing:
  auto_index_timestamp: true

auth:
  default_admin_username: admin
  default_admin_password: changeme           # CHANGE THIS before production use
  jwt_secret: plomvix-change-in-prod         # CHANGE THIS before production use
  jwt_expiry_seconds: 3600
  api_key_length: 32

logging:
  level: info      # options: debug | info | warn | error
  format: pretty   # options: json (production) | pretty (development)
```

**Verify:** `cat config.yaml` shows all sections. File is valid YAML.

---

## TASK 06 — Create internal/config/config.go

**Action:** Create `internal/config/config.go`.

**Imports required:**
```go
import (
    "fmt"
    "strings"
    "sync"

    "github.com/spf13/viper"
)
```

**Structs:**
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

**Singleton storage:**
```go
var (
    instance *Config
    once     sync.Once
    loadErr  error
)
```

**`Load(path string) (*Config, error)`:**
```go
func Load(path string) (*Config, error) {
    once.Do(func() {
        v := viper.New()
        v.SetConfigFile(path)
        v.SetConfigType("yaml")

        // SetEnvKeyReplacer ensures nested keys map to env vars correctly:
        // server.port → PLOMVIX_SERVER_PORT
        v.SetEnvPrefix("PLOMVIX")
        v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
        v.AutomaticEnv()

        if err := v.ReadInConfig(); err != nil {
            loadErr = fmt.Errorf("failed to read config file %q: %w", path, err)
            return
        }

        var cfg Config
        if err := v.Unmarshal(&cfg); err != nil {
            loadErr = fmt.Errorf("failed to parse config: %w", err)
            return
        }

        if err := validate(&cfg); err != nil {
            loadErr = err
            return
        }

        instance = &cfg
    })

    return instance, loadErr
}
```

> **TESTING NOTE:** `sync.Once` means `Load()` executes its body exactly once per
> process lifetime. In tests, never call `config.Load()` — construct `*Config`
> structs directly instead:
> ```go
> cfg := &config.Config{
>     Env: "development",
>     Server: config.ServerConfig{Host: "0.0.0.0", Port: 8080, ...},
>     ...
> }
> ```

**`Get() *Config`:**
```go
// Get returns the loaded Config singleton.
// Must only be called after Load() has returned successfully.
// Panics with a clear message if called before Load().
func Get() *Config {
    if instance == nil {
        panic("plomvix: config not loaded — call config.Load() first")
    }
    return instance
}
```

> **CONCURRENCY NOTE:** `Get()` is safe to call from multiple goroutines only
> after `Load()` has returned successfully, because `instance` is written once
> inside `sync.Once` before any goroutine can call `Get()` in normal boot flow.
> Do not call `Get()` concurrently with `Load()`.

**Helper methods:**
```go
func (c *Config) IsDevelopment() bool { return c.Env == "development" }
func (c *Config) IsProduction() bool  { return c.Env == "production" }
```

**`validate(c *Config) error`:**
```go
func validate(c *Config) error {
    var errs []string

    // validate each field per the rules table below
    // for each violation: errs = append(errs, "field: rule message")

    if len(errs) > 0 {
        return fmt.Errorf("plomvix config validation failed:\n  - %s",
            strings.Join(errs, "\n  - "))
    }
    return nil
}
```

**Validation rules — implement ALL of these inside `validate()`:**

| Field | Rule | Error message |
|---|---|---|
| `Env` | must be `development` or `production` | `env must be "development" or "production", got: "<value>"` |
| `Server.Port` | between 1 and 65535 | `server.port must be between 1 and 65535, got: <value>` |
| `Server.Host` | not empty | `server.host must not be empty` |
| `Server.ReadTimeout` | greater than 0 | `server.read_timeout must be greater than 0` |
| `Server.WriteTimeout` | greater than 0 | `server.write_timeout must be greater than 0` |
| `Server.IdleTimeout` | greater than 0 | `server.idle_timeout must be greater than 0` |
| `Storage.DataDir` | not empty | `storage.data_dir must not be empty` |
| `Storage.WALFlushThreshold` | greater than 0 | `storage.wal_flush_threshold must be greater than 0` |
| `Storage.HotTierMaxSize` | greater than 0 | `storage.hot_tier_max_size must be greater than 0` |
| `Storage.RetentionDays` | greater than 0 | `storage.retention_days must be greater than 0` |
| `Compression.HotTier` | one of: `snappy`, `lz4`, `none` | `compression.hot_tier must be one of [snappy lz4 none], got: "<value>"` |
| `Compression.ColdTier` | one of: `zstd`, `snappy`, `none` | `compression.cold_tier must be one of [zstd snappy none], got: "<value>"` |
| `Auth.DefaultAdminUsername` | not empty | `auth.default_admin_username must not be empty` |
| `Auth.DefaultAdminPassword` | not empty | `auth.default_admin_password must not be empty` |
| `Auth.JWTSecret` | not empty | `auth.jwt_secret must not be empty` |
| `Auth.JWTSecret` | if `Env == "production"`: not equal to `plomvix-change-in-prod` | `auth.jwt_secret must be changed from default in production mode` |
| `Auth.DefaultAdminPassword` | if `Env == "production"`: not equal to `changeme` | `auth.default_admin_password must be changed from default in production mode` |
| `Auth.JWTExpirySeconds` | greater than 0 | `auth.jwt_expiry_seconds must be greater than 0` |
| `Auth.APIKeyLength` | between 16 and 64 | `auth.api_key_length must be between 16 and 64, got: <value>` |
| `Logging.Level` | one of: `debug`, `info`, `warn`, `error` | `logging.level must be one of [debug info warn error], got: "<value>"` |
| `Logging.Format` | one of: `json`, `pretty` | `logging.format must be one of [json pretty], got: "<value>"` |

**Verify:** `go build ./internal/config/` compiles with no errors.

---

## TASK 07 — Create pkg/utils/utils.go

**Action:** Create `pkg/utils/utils.go`.

**Imports required:**
```go
import (
    "fmt"
    "os"
    "path/filepath"
    "runtime"

    "github.com/google/uuid"
)
```

**Implement all 7 functions:**

```go
package utils

// DirExists returns true if path exists and is a directory.
func DirExists(path string) bool {
    info, err := os.Stat(path)
    if err != nil {
        return false
    }
    return info.IsDir()
}

// EnsureDir creates path and all parent directories if they do not exist.
// It is idempotent — calling it on an existing directory returns nil.
func EnsureDir(path string) error {
    return os.MkdirAll(path, 0755)
}

// IsWritable returns true if the process can write to the given directory.
// It tests by creating and immediately deleting a temporary file inside path.
// No temporary files are left behind regardless of outcome.
func IsWritable(path string) bool {
    tmp := filepath.Join(path, ".plomvix_write_check")
    f, err := os.Create(tmp)
    if err != nil {
        return false
    }
    f.Close()
    os.Remove(tmp)
    return true
}

// BytesToHuman converts a byte count to a human-readable string with one decimal place.
// Examples: 512 → "512 B", 1536 → "1.5 KB", 67108864 → "64.0 MB"
//
// NOTE: parameter is named b to avoid shadowing the built-in identifier bytes.
func BytesToHuman(b int64) string {
    const unit = 1024
    if b < unit {
        return fmt.Sprintf("%d B", b)
    }
    div, exp := int64(unit), 0
    for n := b / unit; n >= unit; n /= unit {
        div *= unit
        exp++
    }
    labels := []string{"KB", "MB", "GB", "TB", "PB"}
    return fmt.Sprintf("%.1f %s", float64(b)/float64(div), labels[exp])
}

// GetGoVersion returns the current Go runtime version string.
// Example: "go1.22.0"
func GetGoVersion() string {
    return runtime.Version()
}

// GetOSArch returns the current operating system and architecture string.
// Example: "linux/amd64"
func GetOSArch() string {
    return fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
}

// NewRequestID returns a new UUID v4 string suitable for use as a request identifier.
// Example: "550e8400-e29b-41d4-a716-446655440000"
func NewRequestID() string {
    return uuid.New().String()
}
```

**Verify:** `go build ./pkg/utils/` compiles with no errors.

---

## TASK 08 — Create pkg/utils/utils_test.go

**Action:** Create `pkg/utils/utils_test.go`.

**Imports required:**
```go
import (
    "os"
    "path/filepath"
    "strings" // needed only if using strings.Split for UUID validation in TestNewRequestID
    "testing"
)
```
> NOTE: Only include `"strings"` if you use `strings.Split` in `TestNewRequestID`.
> If you count hyphens a different way, omit it — unused imports are a compile error in Go.

**Implement tests for all 7 functions:**

```
TestDirExists:
  - DirExists(os.TempDir()) → must return true
  - DirExists("/nonexistent/path/abc123xyz") → must return false
  - create a temp file via os.CreateTemp, call DirExists on the file path → must return false

TestEnsureDir:
  - EnsureDir(filepath.Join(os.TempDir(), "plomvix_test_ensure")) → must return nil
  - call EnsureDir on same path again → must return nil (idempotent)
  - defer os.RemoveAll to clean up

TestIsWritable:
  - IsWritable(os.TempDir()) → must return true
  - verify ".plomvix_write_check" does NOT exist in os.TempDir() after the call
  - readonly dir sub-test: skip if os.Getuid() == 0 (root bypasses chmod restrictions),
    otherwise create a chmod 000 dir and assert IsWritable returns false
    (see cross-platform pattern below)

TestBytesToHuman:
  - BytesToHuman(0)           → "0 B"
  - BytesToHuman(512)         → "512 B"
  - BytesToHuman(1024)        → "1.0 KB"
  - BytesToHuman(1536)        → "1.5 KB"
  - BytesToHuman(1048576)     → "1.0 MB"
  - BytesToHuman(67108864)    → "64.0 MB"
  - BytesToHuman(10737418240) → "10.0 GB"

TestGetGoVersion:
  - result must start with "go"

TestGetOSArch:
  - result must not be empty
  - result must contain exactly one "/"

TestNewRequestID:
  - result must have length 36
  - result must contain exactly 4 hyphens
  - split by "-", part lengths must be [8, 4, 4, 4, 12]
  - two consecutive calls must return different values
```

**Cross-platform safe readonly dir pattern for TestIsWritable:**
```go
// Root users bypass filesystem permissions — skip this sub-test when running as root
if os.Getuid() == 0 {
    t.Skip("skipping readonly dir test: chmod restrictions do not apply to root")
}
readonlyDir := filepath.Join(os.TempDir(), "plomvix_test_readonly")
if err := os.MkdirAll(readonlyDir, 0755); err != nil {
    t.Fatalf("setup failed: %v", err)
}
defer func() {
    _ = os.Chmod(readonlyDir, 0755)
    _ = os.RemoveAll(readonlyDir)
}()
if err := os.Chmod(readonlyDir, 0000); err != nil {
    t.Fatalf("chmod failed: %v", err)
}
if IsWritable(readonlyDir) {
    t.Error("expected IsWritable to return false for readonly dir")
}
```

**Verify:** `go test ./pkg/utils/` passes with zero failures.
Skips are acceptable when running as root (the readonly dir sub-test in TestIsWritable
skips itself under root because chmod restrictions do not apply to root users).

---

## TASK 09 — Create pkg/utils/response.go

**Action:** Create `pkg/utils/response.go`.

**Imports required:**
```go
import (
    "encoding/json"
    "net/http"
)
```

**Structs:**
```go
// Response is the standard success envelope for all Plomvix API responses.
type Response struct {
    Status    string      `json:"status"`
    Data      interface{} `json:"data,omitempty"`
    RequestID string      `json:"request_id,omitempty"`
}

// ErrorResponse is the standard error envelope for all Plomvix API error responses.
type ErrorResponse struct {
    Status    string    `json:"status"`
    Error     ErrorBody `json:"error"`
    RequestID string    `json:"request_id,omitempty"`
}

// ErrorBody contains the machine-readable code and human-readable message.
type ErrorBody struct {
    Code    string   `json:"code"`
    Message string   `json:"message"`
    Details []string `json:"details,omitempty"`
}
```

**Standard error code constants — define these at package level:**
```go
const (
    CodeValidationFailed  = "VALIDATION_FAILED"
    CodeUnauthorized      = "UNAUTHORIZED"
    CodeForbidden         = "FORBIDDEN"
    CodeNotFound          = "NOT_FOUND"
    CodeInternalError     = "INTERNAL_ERROR"
    CodeHealthCheckFailed = "HEALTH_CHECK_FAILED"
    CodeServiceUnavail    = "SERVICE_UNAVAILABLE"
)
```

**Internal write helper — implement this first, all public helpers call it:**
```go
func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(payload)
}
```

**Public helper functions:**

Every function must:
1. Read `requestID := r.Header.Get("X-Request-ID")`
2. Include `RequestID: requestID` in the response struct
3. Call `writeJSON(w, httpStatusCode, responseStruct)`

```go
func OK(w http.ResponseWriter, r *http.Request, data interface{})
// HTTP 200 — Response{Status:"ok", Data:data, RequestID:...}

func Created(w http.ResponseWriter, r *http.Request, data interface{})
// HTTP 201 — Response{Status:"ok", Data:data, RequestID:...}

func BadRequest(w http.ResponseWriter, r *http.Request, code, message string, details ...string)
// HTTP 400 — ErrorResponse{Status:"error", Error:{Code:code, Message:message, Details:details}, RequestID:...}

func Unauthorized(w http.ResponseWriter, r *http.Request, message string)
// HTTP 401 — ErrorResponse with Code: CodeUnauthorized

func Forbidden(w http.ResponseWriter, r *http.Request, message string)
// HTTP 403 — ErrorResponse with Code: CodeForbidden

func NotFound(w http.ResponseWriter, r *http.Request, message string)
// HTTP 404 — ErrorResponse with Code: CodeNotFound

func InternalError(w http.ResponseWriter, r *http.Request, message string)
// HTTP 500 — ErrorResponse with Code: CodeInternalError

func ServiceUnavailable(w http.ResponseWriter, r *http.Request, code, message string, details ...string)
// HTTP 503 — ErrorResponse{Status:"error", Error:{Code:code, Message:message, Details:details}, RequestID:...}
```

**Verify:** `go build ./pkg/utils/` compiles with no errors.

---

## TASK 10 — Create internal/logger/logger.go

**Action:** Create `internal/logger/logger.go`.

**Imports required:**
```go
import (
    "fmt"

    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)
```

**IMPORTANT — naming rules:**
- Do NOT name the package-level variable `log` — conflicts with stdlib `log` package.
- Do NOT name it `logger` — same name as the package, confusing to read.
- Name it `globalLogger`:
```go
var globalLogger *zap.Logger
```

**Field convention comment block — place immediately after `package logger`:**
```go
// Structured logging field conventions for Plomvix:
//
// Server startup:      zap.Int("port"), zap.String("host"), zap.String("version"), zap.Int("pid"), zap.String("env")
// HTTP request:        zap.String("method"), zap.String("path"), zap.Int("status"), zap.Int64("latency_ms"), zap.String("request_id")
// Storage operation:   zap.String("operation"), zap.String("data_type"), zap.Int64("bytes"), zap.Int64("duration_ms")
// Directory operation: zap.String("path"), zap.String("operation")
// Graceful shutdown:   zap.Int("timeout_seconds"), zap.String("reason")
//
// NEVER log: password, jwt_secret, api_key, or any field containing "secret" or "token".
```

**`Init(level, format string) error`:**
```go
func Init(level, format string) error {
    var zapLevel zapcore.Level
    if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
        return fmt.Errorf("invalid log level %q: %w", level, err)
    }

    var cfg zap.Config
    if format == "json" {
        cfg = zap.NewProductionConfig()
    } else {
        cfg = zap.NewDevelopmentConfig()
    }
    cfg.Level = zap.NewAtomicLevelAt(zapLevel)

    l, err := cfg.Build()
    if err != nil {
        return fmt.Errorf("failed to build logger: %w", err)
    }

    globalLogger = l
    return nil
}
```

**Guard function:**
```go
func must() {
    if globalLogger == nil {
        panic("plomvix: logger not initialized — call logger.Init() first")
    }
}
```

**Public functions:**
```go
func Debug(msg string, fields ...zap.Field) { must(); globalLogger.Debug(msg, fields...) }
func Info(msg string, fields ...zap.Field)  { must(); globalLogger.Info(msg, fields...) }
func Warn(msg string, fields ...zap.Field)  { must(); globalLogger.Warn(msg, fields...) }
func Error(msg string, fields ...zap.Field) { must(); globalLogger.Error(msg, fields...) }
func Fatal(msg string, fields ...zap.Field) { must(); globalLogger.Fatal(msg, fields...) }
func Sync()                                 { must(); _ = globalLogger.Sync() }
```

**Verify:** `go build ./internal/logger/` compiles with no errors.

---

## TASK 11 — Create internal/server/server.go

**Action:** Create `internal/server/server.go`.

**Imports required:**
```go
import (
    "context"
    "fmt"
    "net/http"
    "os"
    "path/filepath"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "go.uber.org/zap"

    "github.com/plomvix/plomvix/internal/config"
    "github.com/plomvix/plomvix/internal/logger"
    "github.com/plomvix/plomvix/pkg/utils"
)
```

**Struct:**
```go
// Server holds the HTTP server and all its dependencies.
// NOTE: the field is named httpServer (not http) to avoid shadowing the net/http import.
type Server struct {
    router     *chi.Mux
    cfg        *config.Config
    httpServer *http.Server
    startTime  time.Time
    version    string
}
```

**`New(cfg *config.Config, version string) *Server`:**
```go
func New(cfg *config.Config, version string) *Server {
    s := &Server{
        router:    chi.NewRouter(),
        cfg:       cfg,
        startTime: time.Now(),
        version:   version,
    }
    s.httpServer = &http.Server{
        Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
        Handler:      s.router,
        ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
        WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
        IdleTimeout:  time.Duration(cfg.Server.IdleTimeout) * time.Second,
    }
    s.setupMiddleware()
    s.setupRoutes()
    return s
}
```

**`Start() error`:**
```go
func (s *Server) Start() error {
    logger.Info("http server listening", zap.String("addr", s.httpServer.Addr))
    if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        return err
    }
    return nil
}
```

**`Shutdown(ctx context.Context) error`:**
```go
func (s *Server) Shutdown(ctx context.Context) error {
    return s.httpServer.Shutdown(ctx)
}
```

**`Router() *chi.Mux`:**
```go
func (s *Server) Router() *chi.Mux {
    return s.router
}
```

**`setupMiddleware()` — attach in this EXACT order:**
```go
func (s *Server) setupMiddleware() {
    // 1. Custom Request ID — MUST use utils.NewRequestID() for UUID v4.
    //    Do NOT use chi's middleware.RequestID — it does not produce UUID v4 format.
    s.router.Use(func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            requestID := utils.NewRequestID()
            r.Header.Set("X-Request-ID", requestID)
            w.Header().Set("X-Request-ID", requestID)
            next.ServeHTTP(w, r)
        })
    })

    // 2. Request logger
    s.router.Use(func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
            start := time.Now()
            next.ServeHTTP(ww, r)
            logger.Info("request completed",
                zap.String("method", r.Method),
                zap.String("path", r.URL.Path),
                zap.Int("status", ww.Status()),
                zap.Int64("latency_ms", time.Since(start).Milliseconds()),
                zap.String("request_id", r.Header.Get("X-Request-ID")),
            )
        })
    })

    // 3. Panic recoverer
    s.router.Use(middleware.Recoverer)

    // 4. Request timeout
    s.router.Use(middleware.Timeout(
        time.Duration(s.cfg.Server.WriteTimeout) * time.Second,
    ))

    // 5. Content-Type
    s.router.Use(func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.Header().Set("Content-Type", "application/json")
            next.ServeHTTP(w, r)
        })
    })
}
```

**`setupRoutes()`:**
```go
func (s *Server) setupRoutes() {
    s.router.Get("/health", s.handleHealth)
}
```

**`handleHealth` — implement in the same file:**
```go
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
    // Use filepath.Join for all path construction — consistent and cross-platform.
    dataDirs := []string{
        filepath.Join(s.cfg.Storage.DataDir, "wal"),
        filepath.Join(s.cfg.Storage.DataDir, "hot"),
        filepath.Join(s.cfg.Storage.DataDir, "cold", "logs"),
        filepath.Join(s.cfg.Storage.DataDir, "cold", "metrics"),
        filepath.Join(s.cfg.Storage.DataDir, "cold", "json"),
        filepath.Join(s.cfg.Storage.DataDir, "cold", "kv"),
    }

    var failures []string
    for _, dir := range dataDirs {
        if !utils.IsWritable(dir) {
            failures = append(failures,
                fmt.Sprintf("data directory not writable: %s", dir))
        }
    }

    if len(failures) > 0 {
        utils.ServiceUnavailable(w, r,
            utils.CodeHealthCheckFailed,
            "One or more health checks failed",
            failures...,
        )
        return
    }

    utils.OK(w, r, map[string]interface{}{
        "version":        s.version,
        "env":            s.cfg.Env,
        "uptime_seconds": int64(time.Since(s.startTime).Seconds()),
        "pid":            os.Getpid(),
        "go_version":     utils.GetGoVersion(),
        "os_arch":        utils.GetOSArch(),
    })
}
```

> NOTE: TASK 11 and TASK 12 are now merged into one file. There is no separate
> TASK 12 — `handleHealth` is implemented here as part of server.go.

**Verify:** `go build ./internal/server/` compiles with no errors.

---

## TASK 12 — Create LICENSE file

**Action:** Create `LICENSE` at project root:
```
MIT License

Copyright (c) 2024 Plomvix Contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

**Verify:** `cat LICENSE` shows full MIT text.

---

## TASK 13 — Create Makefile

**Action:** Create `Makefile` at project root.

**CRITICAL — use TABS for indentation, not spaces.**
Verify with `cat -A Makefile` — recipe lines must start with `^I` (tab character).

**Use `:=` for immediately-evaluated variables (BUILD_TIME, GIT_COMMIT) and `?=` / `=` for others:**

```makefile
.PHONY: run build test test-verbose vet lint tidy clean coverage help

VERSION      ?= 0.1.0
BINARY        = plomvix
BUILD_TIME   := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LD_FLAGS_INNER = -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)
LDFLAGS       = -ldflags "$(LD_FLAGS_INNER)"

## run: Run Plomvix without building a binary
run:
	go run $(LDFLAGS) ./cmd/plomvix

## build: Build the Plomvix binary with version injected
build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/plomvix

## test: Run all tests with race detector and coverage
test:
	go test -race -cover ./...

## test-verbose: Run all tests with verbose output
test-verbose:
	go test -race -cover -v ./...

## vet: Run go vet static analysis
vet:
	go vet ./...

## lint: Run golangci-lint (install: https://golangci-lint.run/usage/install)
lint:
	golangci-lint run ./...

## tidy: Tidy go modules
tidy:
	go mod tidy

## clean: Remove binary and coverage output
clean:
	rm -f $(BINARY) coverage.out coverage.html

## coverage: Generate HTML coverage report (open coverage.html manually)
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report written to coverage.html"

## help: Show available make commands
help:
	@grep -E '^## ' Makefile | sed 's/## //'
```

**Verify:**
- `make build` produces binary with correct version
- `make help` lists all commands
- `make vet` passes with no errors
- `cat -A Makefile` shows `^I` on recipe lines

---

## TASK 14 — Create cmd/plomvix/main.go

**Action:** Replace the stub `cmd/plomvix/main.go` with the full implementation.

**Imports required:**
```go
import (
    "context"
    "flag"
    "fmt"
    "os"
    "os/signal"
    "path/filepath"
    "syscall"
    "time"

    figure "github.com/common-nighthawk/go-figure"
    "go.uber.org/zap"

    "github.com/plomvix/plomvix/internal/config"
    "github.com/plomvix/plomvix/internal/logger"
    "github.com/plomvix/plomvix/internal/server"
    "github.com/plomvix/plomvix/pkg/utils"
)
```

**Build-time variables at package level:**
```go
var (
    Version   = "dev"
    BuildTime = "unknown"
    GitCommit = "unknown"
)
```

**`main()` boot sequence — implement in this EXACT order:**

```
1.  Define CLI flags:
      versionFlag := flag.Bool("version", false, "print version and exit")
      configPath  := flag.String("config", "./config.yaml", "path to config file")
    flag.Parse()

2.  Handle --version flag:
      if *versionFlag {
          fmt.Printf("Plomvix %s (built: %s, commit: %s, %s)\n",
              Version, BuildTime, GitCommit, utils.GetOSArch())
          return // use return, not os.Exit(0) — consistent with Step 16 and avoids defer bypass
      }

3.  Print ASCII banner (before config loads, always visible):
      fig := figure.NewFigure("Plomvix", "", true)
      fig.Print()
      fmt.Println()
      fmt.Println("  The Indian-built observability database.")
      fmt.Printf("  Version: %s\n\n", Version)

4.  Load config:
      cfg, err := config.Load(*configPath)
      if err != nil {
          fmt.Fprintln(os.Stderr, err)
          os.Exit(1)
      }

5.  Initialize logger:
      if err := logger.Init(cfg.Logging.Level, cfg.Logging.Format); err != nil {
          fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
          os.Exit(1)
      }

6.  Log startup:
      logger.Info("plomvix starting",
          zap.String("version",    Version),
          zap.Int("pid",           os.Getpid()),
          zap.String("env",        cfg.Env),
          zap.String("build_time", BuildTime),
          zap.String("git_commit", GitCommit),
      )

7.  Bootstrap data directories:
      if err := bootstrapDataDirs(cfg); err != nil {
          logger.Error("failed to bootstrap data directories", zap.Error(err))
          os.Exit(1)
      }

8.  Create server:
      srv := server.New(cfg, Version)

9.  OS signal channel:
      quit := make(chan os.Signal, 1)
      signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

10. Start server in goroutine:
      serverErr := make(chan error, 1)
      go func() {
          serverErr <- srv.Start()
      }()

11. Brief pause then log ready:
      time.Sleep(100 * time.Millisecond)
      logger.Info("plomvix ready",
          zap.String("host", cfg.Server.Host),
          zap.Int("port",    cfg.Server.Port),
          zap.String("env",  cfg.Env),
      )

12. Block on signal or server error:
      select {
      case sig := <-quit:
          logger.Info("shutdown signal received",
              zap.String("signal", sig.String()))
      case err := <-serverErr:
          if err != nil {
              logger.Error("server error", zap.Error(err))
          }
      }

13. Graceful shutdown — explicit cancel, NOT defer:
      logger.Info("shutting down plomvix", zap.Int("timeout_seconds", 30))
      ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
      if err := srv.Shutdown(ctx); err != nil {
          logger.Error("shutdown error", zap.Error(err))
      }
      cancel() // called explicitly before exit, not deferred

14. Log stopped BEFORE Sync so it is not lost:
      logger.Info("plomvix stopped cleanly")

15. Flush logger:
      logger.Sync()

16. Return from main() — do NOT call os.Exit(0) at the end of normal flow.
      Returning from main naturally exits with code 0.

> CLARIFICATION on os.Exit usage:
> - os.Exit(1) in steps 4, 5, 7 is CORRECT — these are hard error paths before
>   anything is deferred, so defer-bypass is not a concern there.
> - os.Exit(0) at the END of main (step 16) is replaced with plain return —
>   because future sprints may add deferred cleanup (e.g. flushing storage),
>   and os.Exit(0) would bypass those silently.
> - Rule: always os.Exit(1) on errors, never os.Exit(0) at normal exit.
```

**`bootstrapDataDirs(cfg *config.Config) error`:**
```go
func bootstrapDataDirs(cfg *config.Config) error {
    dirs := []string{
        filepath.Join(cfg.Storage.DataDir, "wal"),
        filepath.Join(cfg.Storage.DataDir, "hot"),
        filepath.Join(cfg.Storage.DataDir, "cold", "logs"),
        filepath.Join(cfg.Storage.DataDir, "cold", "metrics"),
        filepath.Join(cfg.Storage.DataDir, "cold", "json"),
        filepath.Join(cfg.Storage.DataDir, "cold", "kv"),
    }
    for _, dir := range dirs {
        if err := utils.EnsureDir(dir); err != nil {
            return fmt.Errorf("failed to create data directory %q: %w", dir, err)
        }
        logger.Debug("data directory ready", zap.String("path", dir))
    }
    return nil
}
```

**Verify:** `go build ./cmd/plomvix/` compiles with no errors.

---

## TASK 15 — Create .golangci.yml

**Action:** Create `.golangci.yml` at project root:

```yaml
run:
  timeout: 5m
  go: "1.22"

linters:
  enable:
    - errcheck
    - gosimple
    - govet
    - ineffassign
    - staticcheck
    - gofmt
    - goimports
    - misspell
    - revive

linters-settings:
  revive:
    rules:
      - name: exported
        severity: warning

issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - errcheck
```

> NOTE: `unused` is intentionally NOT listed. It was merged into `staticcheck`
> in golangci-lint v1.57+. Listing both causes errors on modern versions.

**Verify:** `golangci-lint run ./...` runs without configuration errors.

---

## TASK 16 — Create README.md

**Action:** Create `README.md` at project root with these sections:

```
1. Header
   - Plomvix name + one-line description
   - Badges: build status, license (MIT), Go version

2. What is Plomvix
   - Unified observability + general-purpose database
   - Supports: logs, metrics, telemetry, key-value, JSON
   - Why it exists: ClickHouse = manual schema, Loki = slow at scale,
     zero Indian-built open-source observability databases exist
   - Made in India. Open source. MIT licensed.

3. Current Status
   - v0.1.0 — Sprint 1 complete
   - What works: boots, loads config, HTTP server, health check
   - Coming next: Sprint 2 — Auth system

4. Prerequisites
   - Go 1.22+
   - make
   - git
   - golangci-lint (optional — for make lint)

5. Getting Started
   git clone https://github.com/plomvix/plomvix
   cd plomvix
   go mod tidy
   make run
   curl http://localhost:8080/health

   Show expected 200 response:
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

6. Make Commands
   Table: command | description (all 9 targets)

7. Configuration Reference
   Table: key | type | default | description
   Cover every key in config.yaml

8. Project Structure
   Annotated directory tree

9. Roadmap
   - [ ] Sprint 1 ✅ — Project skeleton
   - [ ] Sprint 2 — Auth system
   - [ ] Sprint 3 — WAL
   - [ ] Sprint 4 — Hot tier (RocksDB)
   - [ ] Sprint 5 — Ingestion + Schema inference
   - [ ] Sprint 6 — SQL query engine
   - [ ] Sprint 7 — Cold tier + Tiering
   - [ ] Sprint 8 — Admin APIs + Swagger
   - [ ] Sprint 9 — Polish + Testing + CLI

10. Contributing
    - Issues on GitHub
    - Fork → branch → PR
    - Run make vet && make lint before submitting

11. License
    MIT — see LICENSE file
```

**Verify:** README renders on GitHub with no broken sections. Getting Started works end-to-end.

---

## TASK 17 — Full smoke test

**Action:** Save the following as `smoke_test.sh` in the project root, then run it.
`smoke_test.sh` is listed in `.gitignore` (added in TASK 04) — it is a local verification
script, not a committed deliverable.

```bash
#!/bin/bash
# Plomvix Sprint 1 Smoke Test
# Run from project root: bash smoke_test.sh

set -euo pipefail

SERVER_PID=""

cleanup() {
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        kill -SIGTERM "$SERVER_PID" 2>/dev/null || true
        wait "$SERVER_PID" 2>/dev/null || true
    fi
}
trap cleanup EXIT

echo "=== Step 1: Clean build ==="
go mod tidy
make vet
make build
echo "Binary: $(ls -lh plomvix)"

echo ""
echo "=== Step 2: Version flag ==="
./plomvix --version

echo ""
echo "=== Step 3: Boot and health check ==="
./plomvix > /tmp/plomvix_stdout.log 2>&1 &
SERVER_PID=$!
sleep 2

curl -sf http://localhost:8080/health | jq .
echo "Health check: PASSED"

echo ""
echo "=== Step 4: X-Request-ID header check ==="
REQUEST_ID=$(curl -sI http://localhost:8080/health \
    | grep -i "x-request-id" \
    | awk '{print $2}' \
    | tr -d '\r\n')
echo "X-Request-ID: ${REQUEST_ID}"
if [ "${#REQUEST_ID}" -ne 36 ]; then
    echo "FAIL: X-Request-ID length is ${#REQUEST_ID}, expected 36"
    exit 1
fi
echo "UUID format: VALID"

echo ""
echo "=== Step 5: JSON log format check ==="
# Use port 8081 to avoid conflict with the server already running on 8080 from Step 3
PLOMVIX_LOGGING_FORMAT=json PLOMVIX_SERVER_PORT=8081 ./plomvix > /tmp/plomvix_json.log 2>&1 &
JSON_PID=$!
sleep 1
kill -SIGTERM "$JSON_PID" 2>/dev/null || true
wait "$JSON_PID" 2>/dev/null || true
# The ASCII banner prints before logger init — skip non-JSON lines, find first JSON line
JSON_LINE=$(grep -m1 '^{' /tmp/plomvix_json.log || true)
if [ -z "$JSON_LINE" ]; then
    echo "FAIL: no JSON log lines found in output"
    exit 1
fi
echo "$JSON_LINE" | jq . > /dev/null 2>&1 \
    && echo "JSON log format: VALID" \
    || { echo "FAIL: log line is not valid JSON: $JSON_LINE"; exit 1; }

echo ""
echo "=== Step 6: Graceful shutdown ==="
kill -SIGTERM "$SERVER_PID"
wait "$SERVER_PID"
SHUTDOWN_CODE=$?
SERVER_PID=""
# Plomvix catches SIGTERM via signal.Notify and returns from main() cleanly.
# Exit code must be 0 — if it is anything else, shutdown did not complete cleanly.
if [ "$SHUTDOWN_CODE" -ne 0 ]; then
    echo "FAIL: server did not exit cleanly (exit code: $SHUTDOWN_CODE, expected: 0)"
    exit 1
fi
echo "Graceful shutdown: PASSED (exit code: $SHUTDOWN_CODE)"

echo ""
echo "=== Step 7: Run tests ==="
make test

echo ""
echo "=== Step 8: go vet ==="
make vet
echo "go vet: PASSED"

echo ""
echo "=== Step 9: Env var port override ==="
PLOMVIX_SERVER_PORT=9090 ./plomvix > /dev/null 2>&1 &
SERVER_PID=$!
sleep 2
curl -sf http://localhost:9090/health | jq .
echo "Env var override: PASSED"
kill -SIGTERM "$SERVER_PID"
wait "$SERVER_PID" || true
SERVER_PID=""

echo ""
echo "=== Step 10: Bad config rejection ==="
cat > /tmp/plomvix_bad.yaml << 'BADCFG'
env: development
server:
  host: ""
  port: 0
  read_timeout: 30
  write_timeout: 30
  idle_timeout: 60
storage:
  data_dir: ./data
  wal_flush_threshold: 0
  hot_tier_max_size: 10737418240
  retention_days: 30
compression:
  hot_tier: snappy
  cold_tier: zstd
indexing:
  auto_index_timestamp: true
auth:
  default_admin_username: admin
  default_admin_password: changeme
  jwt_secret: plomvix-change-in-prod
  jwt_expiry_seconds: 3600
  api_key_length: 32
logging:
  level: info
  format: pretty
BADCFG

# Temporarily disable set -e for the expected-failure command
set +e
./plomvix --config /tmp/plomvix_bad.yaml
BAD_EXIT=$?
set -e

if [ "$BAD_EXIT" -eq 0 ]; then
    echo "FAIL: bad config should have been rejected but server exited 0"
    exit 1
fi
echo "Bad config rejection: PASSED (exit code: $BAD_EXIT)"

echo ""
echo "================================================"
echo "  ALL STEPS PASSED — Sprint 1 smoke test DONE  "
echo "================================================"
```

**Expected result per step:**

| Step | What is verified | Expected |
|---|---|---|
| 1 | Clean build + vet | Binary produced, no errors |
| 2 | `--version` flag | Prints version + os/arch, exits 0 |
| 3 | Health endpoint | `{"status":"ok","data":{...},"request_id":"..."}` |
| 4 | X-Request-ID header | Present, exactly 36 chars (UUID v4) |
| 5 | JSON log format | Log output is valid JSON when `format=json` |
| 6 | Graceful shutdown | Exits with code 0 (signal caught, main returns cleanly) |
| 7 | Unit tests | All pass with race detector |
| 8 | go vet | No issues |
| 9 | Env var override | Server binds to port 9090 |
| 10 | Bad config | Server exits non-zero with validation errors |

**If any step fails:** fix the relevant task before proceeding to Sprint 2.

---

## EXECUTION ORDER SUMMARY

```
TASK 01  →  go mod init
TASK 02  →  folder structure + stub main.go + git init   ← before TASK 03
TASK 03  →  install dependencies                          ← needs .go file
TASK 04  →  .gitignore
TASK 05  →  config.yaml
TASK 06  →  internal/config/config.go
TASK 07  →  pkg/utils/utils.go
TASK 08  →  pkg/utils/utils_test.go
TASK 09  →  pkg/utils/response.go
TASK 10  →  internal/logger/logger.go
TASK 11  →  internal/server/server.go  (struct + middleware + routes + handleHealth)
TASK 12  →  LICENSE
TASK 13  →  Makefile
TASK 14  →  cmd/plomvix/main.go        ← replaces stub from TASK 02
TASK 15  →  .golangci.yml
TASK 16  →  README.md
TASK 17  →  smoke_test.sh — all 10 steps must pass
```

---

*Sprint 1 complete when TASK 17 passes with zero failures.*
*Plomvix — Built in India. Built for the world.*