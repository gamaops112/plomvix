# Plomvix — Sprint 10 Task Plan
### For: DeepSeek V4 Pro Coding Agent
### Language: Go 1.22 | Module: github.com/plomvix/plomvix

> Execute tasks in exact order. Each task is atomic — one file or one concern.
> Do not skip ahead. Each task depends on the previous.
> Every task has a Verify step — do not proceed until it passes.

---

## CONTEXT

Sprints 1–9 are complete. Sprint 10 is the **final sprint** — Polish, Testing,
and Documentation. No new features are added. The goal is a production-ready,
well-tested, well-documented codebase that a new contributor can clone and
understand within 30 minutes.

**What Sprint 10 delivers:**
- Full integration test suite — end-to-end tests covering the complete data path
- `README.md` — complete, accurate, works on a fresh machine
- Linter cleanup — `make lint` passes with zero issues across all packages
- Makefile polish — new `make integration-test` and `make check` targets
- `.gitignore` audit — ensure all generated files and data dirs are ignored
- `config.yaml` production safety documentation
- Build verification — ldflags version injection confirmed working
- Final `make test` + `make lint` + `make build` all pass clean

**What Sprint 10 does NOT do:**
- No new HTTP endpoints
- No new storage layers
- No Docker or Kubernetes manifests — infrastructure is out of scope
- No performance benchmarks — deferred to post-1.0
- No OTLP, Prometheus exposition format, SQL parser, RBAC expansion, or distributed mode
- No automatic reconciliation tooling for Sprint 7 partial hot/cold flush failures

---

## INTEGRATION TEST DESIGN — READ BEFORE WRITING ANY CODE

Integration tests live in `tests/integration/` and are skipped by default
when running `go test -short ./...`. They boot a real server with real storage
(temp directories) and test the complete data path.

**Separation from unit tests:**
- `make test` → `go test -short -race ./...` → unit tests only; integration tests self-skip in short mode
- `make integration-test` → `go test -race ./tests/integration/...` → full E2E

**How integration tests work:**
- Each test function starts a real `server.Server` with temp directories
- Tests call real HTTP endpoints using `net/http` against the running server
- Tests use `t.TempDir()` for all storage — cleaned up automatically
- Each test is fully self-contained — no shared state between tests
- Tests run the full path: auth → ingest → WAL write → hot tier → query → tier flush → cold query

**Test helper pattern:**
```go
// testServer starts a real Plomvix server in a temp directory.
// Returns the server base URL and a cleanup function.
func testServer(t *testing.T) (baseURL string, cleanup func())
```

The `testServer` helper:
1. Creates temp dirs for WAL, hot, cold, and system auth DB
2. Constructs a minimal config directly, without using `config.Load()`
3. Boots WAL, hot tier, cold tier, auth store, blacklist, and server
4. Bootstraps the default admin user before the server starts
5. Starts the server in a goroutine on port 0
6. Returns `http://127.0.0.1:{port}` as the base URL
7. Returns a cleanup func that shuts down server, blacklist, stores, and logger

**NOTE on port 0:** Go's `net.Listen("tcp", "127.0.0.1:0")` assigns a random
available port. `net.Listener.Addr().String()` returns the actual bound address.
The server must expose its listener address for tests to discover it.
This requires a small change to `internal/server/server.go` — see TASK 02.

**NOTE on timestamps:** Integration tests must use `int64` Unix nanoseconds, not
`float64`. Sprint 8 fixed parser-side timestamp precision, and Sprint 10 tests
must not reintroduce precision loss by creating float64 nanosecond timestamps.

---

## TASK 01 — Linter cleanup

**Action:** Run `make lint` and fix every issue reported.

Common issues to fix proactively before running lint:

**1. Unused imports** — check every file added in Sprints 5–9 for imports that
were added during development but are no longer used.

**2. `godot` — comments must end with a period.** Check all exported function
comments. Every `// FunctionName does X.` comment must end with a period.

**3. `errcheck` — unhandled errors.** Common locations:
- `w.Write([]byte(...))` in HTTP handlers — wrap or explicitly ignore:
  ```go
  _, _ = w.Write([]byte(docsHTML))
  ```
- `json.NewDecoder(...).Decode(...)` in tests — check the error unless the
  response body is intentionally ignored.

**4. `revive` exported symbols** — every exported type, function, and constant
must have a godoc comment.

**5. `misspell`** — check all comments and string literals for common typos.

**6. `gofmt`** — `gofmt` expects file paths, not Go package patterns. Use `find` to pass Go files.

**Action sequence:**
```bash
gofmt -w $(find . -name '*.go' -not -path './vendor/*')
CGO_ENABLED=1 make lint
```

Fix every reported issue. Re-run until output is clean.

**Verify:** `CGO_ENABLED=1 make lint` exits with code 0 and zero issues.

---

## TASK 02 — Expose listener address from server.Server

**Action:** Small targeted change to `internal/server/server.go` needed
by integration tests.

**Change — Add race-safe `Addr()` method to Server:**

After `httpServer.ListenAndServe()` binds, the actual address is available via
the net.Listener. To support port 0 in tests, refactor `Start()` to use
`net.Listen` explicitly before passing to `http.Server.Serve`.

Because integration tests call `srv.Addr()` from one goroutine while `Start()`
sets the address in another, the address field must be protected by a mutex.
Do not use an unguarded string field; `go test -race` will flag it.

```go
// Add fields to Server struct:
type Server struct {
    // ...existing fields...
    addrMu sync.RWMutex
    addr   string // actual bound address, set after Start() is called
}

// Addr returns the network address the server is listening on.
// Returns empty string if the server has not started yet.
func (s *Server) Addr() string {
    s.addrMu.RLock()
    defer s.addrMu.RUnlock()
    return s.addr
}

// setAddr records the listener address in a race-safe way.
func (s *Server) setAddr(addr string) {
    s.addrMu.Lock()
    defer s.addrMu.Unlock()
    s.addr = addr
}
```

**Updated `Start()` method:**
```go
func (s *Server) Start() error {
    addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)
    ln, err := net.Listen("tcp", addr)
    if err != nil {
        return fmt.Errorf("failed to listen on %s: %w", addr, err)
    }

    s.setAddr(ln.Addr().String())

    logger.Info("Plomvix ready",
        zap.String("addr", s.Addr()),
        zap.String("env", s.cfg.Env),
    )

    if err := s.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
        return err
    }
    return nil
}
```

**Imports to add to server.go:**
```go
import (
    "net"
    "sync"
)
```

Keep existing imports like `fmt`, `net/http`, and `go.uber.org/zap` if already
present. `s.httpServer` must already be configured before `Start()` is called.
Only the `ListenAndServe` → `Listen` + `Serve` refactor is needed.

**Verify:**
```bash
CGO_ENABLED=1 go build ./internal/server/
CGO_ENABLED=1 go test -race ./internal/server/...
```
Existing smoke tests still pass — `make build && ./plomvix` starts correctly.

---

## TASK 03 — Create tests/integration/ directory and helpers

**Action — Part A:** Create directory:
```bash
mkdir -p tests/integration
```

**Action — Part B:** Create `tests/integration/helpers_test.go`.

**Package declaration:** `package integration`

**Full file content:**
```go
package integration

import (
    "bytes"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "net/http"
    "os"
    "path/filepath"
    "sync"
    "testing"
    "time"

    "github.com/plomvix/plomvix/internal/auth"
    "github.com/plomvix/plomvix/internal/config"
    "github.com/plomvix/plomvix/internal/logger"
    "github.com/plomvix/plomvix/internal/server"
    coldstore "github.com/plomvix/plomvix/internal/storage/cold"
    hotstore "github.com/plomvix/plomvix/internal/storage/hot"
    walstore "github.com/plomvix/plomvix/internal/storage/wal"
)

var loggerOnce sync.Once
var loggerErr error

// testServer starts a real Plomvix server with temp storage.
// Returns the base URL and a cleanup function.
func testServer(t *testing.T) (string, func()) {
    t.Helper()
    if testing.Short() {
        t.Skip("integration test skipped in short mode")
    }

    dir := t.TempDir()

    cfg := &config.Config{
        Env: "development",
        Server: config.ServerConfig{
            Host:         "127.0.0.1",
            Port:         0,
            ReadTimeout:  30,
            WriteTimeout: 30,
            IdleTimeout:  60,
        },
        Storage: config.StorageConfig{
            DataDir:           dir,
            WALFlushThreshold: 64 * 1024 * 1024,
            HotTierMaxSize:    1024 * 1024 * 1024,
            RetentionDays:     1,
        },
        Compression: config.CompressionConfig{
            HotTier:  "snappy",
            ColdTier: "zstd",
        },
        Indexing: config.IndexingConfig{
            AutoIndexTimestamp: true,
        },
        Auth: config.AuthConfig{
            DefaultAdminUsername: "admin",
            DefaultAdminPassword: "testpass",
            JWTSecret:            "integration-test-secret",
            JWTExpirySeconds:     3600,
            APIKeyLength:         32,
        },
        Logging: config.LoggingConfig{
            Level:  "error",
            Format: "json",
        },
    }

    loggerOnce.Do(func() {
        loggerErr = logger.Init(cfg.Logging)
    })
    if loggerErr != nil {
        t.Fatalf("logger.Init failed: %v", loggerErr)
    }

    walDir := filepath.Join(dir, "wal")
    wal, err := walstore.Open(walDir, cfg)
    if err != nil {
        t.Fatalf("wal.Open failed: %v", err)
    }

    entries, err := wal.Recover()
    if err != nil {
        _ = wal.Close()
        t.Fatalf("wal.Recover failed: %v", err)
    }

    hotDir := filepath.Join(dir, "hot")
    hot, err := hotstore.Open(hotDir, cfg)
    if err != nil {
        _ = wal.Close()
        t.Fatalf("hotstore.Open failed: %v", err)
    }

    if _, err := hot.ReplayWAL(entries); err != nil {
        hot.Close()
        _ = wal.Close()
        t.Fatalf("hot.ReplayWAL failed: %v", err)
    }

    coldDir := filepath.Join(dir, "cold")
    cold, err := coldstore.NewStore(coldDir)
    if err != nil {
        hot.Close()
        _ = wal.Close()
        t.Fatalf("coldstore.NewStore failed: %v", err)
    }

    tierEngine := coldstore.NewTieringEngine(hot, cold, cfg)
    // Do not start the hourly background goroutine in tests. Manual flush
    // endpoints call tierEngine.Flush() directly and are deterministic.

    authDB := filepath.Join(dir, "system", "auth.db")
    if err := os.MkdirAll(filepath.Dir(authDB), 0755); err != nil {
        hot.Close()
        _ = wal.Close()
        t.Fatalf("failed to create auth dir: %v", err)
    }

    authStore, err := auth.NewStore(authDB)
    if err != nil {
        hot.Close()
        _ = wal.Close()
        t.Fatalf("auth.NewStore failed: %v", err)
    }

    if err := auth.BootstrapAdminUser(authStore, cfg); err != nil {
        _ = authStore.Close()
        hot.Close()
        _ = wal.Close()
        t.Fatalf("BootstrapAdminUser failed: %v", err)
    }

    blacklist := auth.NewBlacklist()

    srv := server.New(cfg, "test", "2024-01-01", "test-commit",
        authStore, blacklist, wal, hot, cold, tierEngine)

    errCh := make(chan error, 1)
    go func() {
        if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
            errCh <- err
            return
        }
        errCh <- nil
    }()

    var baseURL string
    for i := 0; i < 100; i++ {
        select {
        case err := <-errCh:
            if err != nil {
                t.Fatalf("server failed to start: %v", err)
            }
        default:
        }

        if addr := srv.Addr(); addr != "" {
            baseURL = "http://" + addr
            break
        }
        time.Sleep(20 * time.Millisecond)
    }
    if baseURL == "" {
        t.Fatal("server did not start within 2 seconds")
    }

    cleanup := func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        _ = srv.Shutdown(ctx)
        blacklist.Stop()
        hot.Close()
        _ = wal.Close()
        _ = authStore.Close()
        _ = logger.Sync()
    }

    return baseURL, cleanup
}

// adminToken logs in as admin and returns the JWT token.
func adminToken(t *testing.T, baseURL string) string {
    t.Helper()
    resp := mustPost(t, baseURL+"/auth/login", "", map[string]string{
        "username": "admin",
        "password": "testpass",
    })
    data, ok := resp["data"].(map[string]interface{})
    if !ok {
        t.Fatalf("login response missing data: %v", resp)
    }
    token, ok := data["token"].(string)
    if !ok || token == "" {
        t.Fatalf("login did not return token: %v", resp)
    }
    return token
}

// mustPost sends a POST request and returns the decoded JSON response body.
func mustPost(t *testing.T, url, token string, body interface{}) map[string]interface{} {
    t.Helper()
    return doRequest(t, http.MethodPost, url, token, "application/json", body, 0)
}

// mustPostRaw sends a POST request with raw body bytes and a custom Content-Type.
func mustPostRaw(t *testing.T, url, token, contentType string, body []byte) map[string]interface{} {
    t.Helper()
    return doRawRequest(t, http.MethodPost, url, token, contentType, body, 0)
}

// mustGet sends a GET request and returns the decoded JSON response body.
func mustGet(t *testing.T, url, token string) map[string]interface{} {
    t.Helper()
    return doRequest(t, http.MethodGet, url, token, "", nil, 0)
}

// expectStatus sends a request and asserts the response status code.
func expectStatus(t *testing.T, method, url, token string, body interface{}, wantStatus int) {
    t.Helper()
    doRequest(t, method, url, token, "application/json", body, wantStatus)
}

// doRequest is the JSON request helper.
func doRequest(t *testing.T, method, url, token, contentType string, body interface{}, expectCode int) map[string]interface{} {
    t.Helper()

    var raw []byte
    if body != nil {
        b, err := json.Marshal(body)
        if err != nil {
            t.Fatalf("failed to marshal request body: %v", err)
        }
        raw = b
    }
    return doRawRequest(t, method, url, token, contentType, raw, expectCode)
}

// doRawRequest is the underlying HTTP helper.
func doRawRequest(t *testing.T, method, url, token, contentType string, body []byte, expectCode int) map[string]interface{} {
    t.Helper()

    var reqBody io.Reader
    if body != nil {
        reqBody = bytes.NewReader(body)
    }

    req, err := http.NewRequest(method, url, reqBody)
    if err != nil {
        t.Fatalf("failed to create request: %v", err)
    }
    if contentType != "" {
        req.Header.Set("Content-Type", contentType)
    }
    if token != "" {
        req.Header.Set("Authorization", "Bearer "+token)
    }

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        t.Fatalf("request failed: %v", err)
    }
    defer func() {
        _ = resp.Body.Close()
    }()

    if expectCode != 0 {
        if resp.StatusCode != expectCode {
            b, _ := io.ReadAll(resp.Body)
            t.Fatalf("expected status %d, got %d: %s", expectCode, resp.StatusCode, string(b))
        }
        return nil
    }

    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        b, _ := io.ReadAll(resp.Body)
        t.Fatalf("unexpected status %d: %s", resp.StatusCode, string(b))
    }

    b, err := io.ReadAll(resp.Body)
    if err != nil {
        t.Fatalf("failed to read response body: %v", err)
    }
    if len(bytes.TrimSpace(b)) == 0 {
        return map[string]interface{}{}
    }

    var result map[string]interface{}
    if err := json.Unmarshal(b, &result); err != nil {
        t.Fatalf("failed to decode JSON response %q: %v", string(b), err)
    }
    return result
}

// ingestLogs sends a batch of log records to /ingest/logs.
func ingestLogs(t *testing.T, baseURL, token string, records []map[string]interface{}) {
    t.Helper()
    mustPost(t, baseURL+"/ingest/logs", token, map[string]interface{}{
        "records": records,
    })
}

// queryLogs queries /query/logs and returns the total count.
func queryLogs(t *testing.T, baseURL, token string) int {
    t.Helper()
    resp := mustGet(t, baseURL+"/query/logs", token)
    data, ok := resp["data"].(map[string]interface{})
    if !ok {
        t.Fatalf("query response missing data: %v", resp)
    }
    total, ok := data["total"].(float64)
    if !ok {
        t.Fatalf("query response missing total: %v", resp)
    }
    return int(total)
}

// oldTimestamp returns a timestamp older than the default integration retention period.
func oldTimestamp() int64 {
    return time.Now().Add(-48 * time.Hour).UnixNano()
}

// nowTimestamp returns a current Unix nanosecond timestamp.
func nowTimestamp() int64 {
    return time.Now().UnixNano()
}

// requireOK checks the standard response envelope status field.
func requireOK(t *testing.T, resp map[string]interface{}) {
    t.Helper()
    if got := fmt.Sprintf("%v", resp["status"]); got != "ok" {
        t.Fatalf("status = %q, want ok; response=%v", got, resp)
    }
}
```

**Verify:**
```bash
CGO_ENABLED=1 go test -race -run TestDoesNotExist ./tests/integration/
```
This compiles the integration test package without running the full E2E suite.
Do not use `go build` against `tests/integration`; the package only contains `_test.go`
files, so `go test` is the correct verification command.

---

## TASK 04 — Create tests/integration/ingest_query_test.go

**Action:** Create `tests/integration/ingest_query_test.go`.

**Package declaration:** `package integration`

**Full file content:**
```go
package integration

import (
    "fmt"
    "net/http"
    "testing"
    "time"
)

func TestHealthCheck(t *testing.T) {
    baseURL, cleanup := testServer(t)
    defer cleanup()

    resp := mustGet(t, baseURL+"/health", "")
    requireOK(t, resp)

    data := resp["data"].(map[string]interface{})
    if _, ok := data["version"]; !ok {
        t.Error("health response missing version")
    }
    if _, ok := data["wal"]; !ok {
        t.Error("health response missing wal block")
    }
    if _, ok := data["hot"]; !ok {
        t.Error("health response missing hot block")
    }
}

func TestAuthLoginLogout(t *testing.T) {
    baseURL, cleanup := testServer(t)
    defer cleanup()

    token := adminToken(t, baseURL)
    if token == "" {
        t.Fatal("expected non-empty token")
    }

    mustGet(t, baseURL+"/admin/stats", token)
    mustPost(t, baseURL+"/auth/logout", token, nil)
    expectStatus(t, http.MethodGet, baseURL+"/admin/stats", token, nil, http.StatusUnauthorized)
}

func TestIngestAndQueryLogs(t *testing.T) {
    baseURL, cleanup := testServer(t)
    defer cleanup()
    token := adminToken(t, baseURL)

    records := []map[string]interface{}{
        {"level": "info", "message": "first", "timestamp": time.Now().Add(-3 * time.Second).UnixNano()},
        {"level": "warn", "message": "second", "timestamp": time.Now().Add(-2 * time.Second).UnixNano()},
        {"level": "error", "message": "third", "timestamp": time.Now().Add(-1 * time.Second).UnixNano()},
    }
    ingestLogs(t, baseURL, token, records)

    total := queryLogs(t, baseURL, token)
    if total != 3 {
        t.Errorf("query total = %d, want 3", total)
    }
}

func TestIngestAndQueryMetrics(t *testing.T) {
    baseURL, cleanup := testServer(t)
    defer cleanup()
    token := adminToken(t, baseURL)

    mustPost(t, baseURL+"/ingest/metrics", token, map[string]interface{}{
        "records": []map[string]interface{}{
            {"name": "cpu.usage", "value": 55.5},
            {"name": "mem.usage", "value": 70.2},
        },
    })

    resp := mustGet(t, baseURL+"/query/metrics", token)
    data := resp["data"].(map[string]interface{})
    if int(data["total"].(float64)) != 2 {
        t.Errorf("metrics total = %v, want 2", data["total"])
    }
}

func TestIngestAndQueryJSON(t *testing.T) {
    baseURL, cleanup := testServer(t)
    defer cleanup()
    token := adminToken(t, baseURL)

    mustPost(t, baseURL+"/ingest/json", token, map[string]interface{}{
        "records": []map[string]interface{}{
            {"data": map[string]interface{}{"event": "order_placed", "amount": 299.99}},
        },
    })

    resp := mustGet(t, baseURL+"/query/json", token)
    data := resp["data"].(map[string]interface{})
    if int(data["total"].(float64)) < 1 {
        t.Errorf("json total = %v, want >= 1", data["total"])
    }
}

func TestIngestAndQueryKV(t *testing.T) {
    baseURL, cleanup := testServer(t)
    defer cleanup()
    token := adminToken(t, baseURL)

    mustPost(t, baseURL+"/ingest/kv", token, map[string]interface{}{
        "records": []map[string]interface{}{
            {"key": "user:alice", "value": "active"},
        },
    })

    resp := mustGet(t, fmt.Sprintf("%s/query/kv/user:alice", baseURL), token)
    data := resp["data"].(map[string]interface{})
    if int(data["count"].(float64)) != 1 {
        t.Errorf("kv count = %v, want 1", data["count"])
    }
}

func TestQueryKVNotFound(t *testing.T) {
    baseURL, cleanup := testServer(t)
    defer cleanup()
    token := adminToken(t, baseURL)

    resp := mustGet(t, baseURL+"/query/kv/does-not-exist", token)
    data := resp["data"].(map[string]interface{})
    if int(data["count"].(float64)) != 0 {
        t.Errorf("missing key count = %v, want 0", data["count"])
    }
}

func TestQueryWithFilter(t *testing.T) {
    baseURL, cleanup := testServer(t)
    defer cleanup()
    token := adminToken(t, baseURL)

    ingestLogs(t, baseURL, token, []map[string]interface{}{
        {"level": "info", "message": "a", "timestamp": time.Now().Add(-2 * time.Second).UnixNano()},
        {"level": "warn", "message": "b", "timestamp": time.Now().Add(-1 * time.Second).UnixNano()},
        {"level": "info", "message": "c", "timestamp": nowTimestamp()},
    })

    resp := mustGet(t, baseURL+"/query/logs?filter=level%3Dinfo", token)
    data := resp["data"].(map[string]interface{})
    if int(data["total"].(float64)) != 2 {
        t.Errorf("filter total = %v, want 2", data["total"])
    }
}

func TestQueryPagination(t *testing.T) {
    baseURL, cleanup := testServer(t)
    defer cleanup()
    token := adminToken(t, baseURL)

    for i := 0; i < 5; i++ {
        ingestLogs(t, baseURL, token, []map[string]interface{}{
            {
                "level":     "info",
                "message":   fmt.Sprintf("msg-%d", i),
                "timestamp": time.Now().Add(time.Duration(i) * time.Millisecond).UnixNano(),
            },
        })
    }

    resp := mustGet(t, baseURL+"/query/logs?limit=2&offset=0", token)
    data := resp["data"].(map[string]interface{})
    if int(data["total"].(float64)) != 5 {
        t.Errorf("total = %v, want 5", data["total"])
    }
    if int(data["count"].(float64)) != 2 {
        t.Errorf("count = %v, want 2", data["count"])
    }

    resp = mustGet(t, baseURL+"/query/logs?limit=2&offset=2", token)
    data = resp["data"].(map[string]interface{})
    if int(data["count"].(float64)) != 2 {
        t.Errorf("page 2 count = %v, want 2", data["count"])
    }
}

func TestSprint8MultiFormatLogIngest(t *testing.T) {
    baseURL, cleanup := testServer(t)
    defer cleanup()
    token := adminToken(t, baseURL)

    mustPostRaw(t, baseURL+"/ingest/logs", token, "text/csv", []byte("level,message,host\ninfo,csv test,web-01\nwarn,csv warn,web-02\n"))
    mustPostRaw(t, baseURL+"/ingest/logs", token, "text/x-logfmt", []byte("level=info msg=logfmt-test host=web-01"))
    mustPostRaw(t, baseURL+"/ingest/logs", token, "application/x-syslog", []byte("<34>1 2024-01-15T10:30:00Z web-01 app 123 ID47 - syslog test"))

    total := queryLogs(t, baseURL, token)
    if total < 4 {
        t.Errorf("multi-format total = %d, want >= 4", total)
    }
}

func TestIngestWithoutAuth(t *testing.T) {
    baseURL, cleanup := testServer(t)
    defer cleanup()

    expectStatus(t, http.MethodPost, baseURL+"/ingest/logs", "", map[string]interface{}{
        "records": []map[string]interface{}{{"level": "info", "message": "nope"}},
    }, http.StatusUnauthorized)
}

func TestQueryWithoutAuth(t *testing.T) {
    baseURL, cleanup := testServer(t)
    defer cleanup()

    expectStatus(t, http.MethodGet, baseURL+"/query/logs", "", nil, http.StatusUnauthorized)
}

func TestSchemaInference(t *testing.T) {
    baseURL, cleanup := testServer(t)
    defer cleanup()
    token := adminToken(t, baseURL)

    ingestLogs(t, baseURL, token, []map[string]interface{}{
        {"level": "info", "message": "schema test", "count": 42, "timestamp": nowTimestamp()},
    })

    resp := mustGet(t, baseURL+"/query/schema/logs", token)
    data := resp["data"].(map[string]interface{})
    fields, ok := data["fields"].(map[string]interface{})
    if !ok {
        t.Fatal("schema fields missing")
    }
    if fields["level"] != "string" {
        t.Errorf("level field type = %v, want string", fields["level"])
    }
    if fields["count"] != "int64" {
        t.Errorf("count field type = %v, want int64", fields["count"])
    }
}
```

**Verify:** `CGO_ENABLED=1 go test -race -count=1 ./tests/integration/` — all tests pass.

---

## TASK 05 — Create tests/integration/tiering_test.go

**Action:** Create `tests/integration/tiering_test.go`.

**Package declaration:** `package integration`

**Full file content:**
```go
package integration

import (
    "fmt"
    "net/http"
    "testing"
    "time"
)

func TestTierFlushMovesData(t *testing.T) {
    baseURL, cleanup := testServer(t)
    defer cleanup()
    token := adminToken(t, baseURL)

    ingestLogs(t, baseURL, token, []map[string]interface{}{
        {"level": "info", "message": "tiering test", "timestamp": oldTimestamp()},
    })

    totalBefore := queryLogs(t, baseURL, token)
    if totalBefore < 1 {
        t.Fatalf("expected record before flush, got total=%d", totalBefore)
    }

    resp := mustPost(t, baseURL+"/admin/tier/flush", token, nil)
    requireOK(t, resp)

    data := resp["data"].(map[string]interface{})
    moved := int(data["records_moved"].(float64))
    if moved < 1 {
        t.Errorf("records_moved = %d, want >= 1", moved)
    }

    totalAfter := queryLogs(t, baseURL, token)
    if totalAfter < 1 {
        t.Errorf("after flush, expected records still queryable, got total=%d", totalAfter)
    }
}

func TestTierFlushDoesNotAffectNewRecords(t *testing.T) {
    baseURL, cleanup := testServer(t)
    defer cleanup()
    token := adminToken(t, baseURL)

    ingestLogs(t, baseURL, token, []map[string]interface{}{
        {"level": "info", "message": "new record", "timestamp": nowTimestamp()},
    })

    resp := mustPost(t, baseURL+"/admin/tier/flush", token, nil)
    requireOK(t, resp)

    total := queryLogs(t, baseURL, token)
    if total != 1 {
        t.Errorf("after flush, total = %d, want 1", total)
    }
}

func TestAdminStats(t *testing.T) {
    baseURL, cleanup := testServer(t)
    defer cleanup()
    token := adminToken(t, baseURL)

    resp := mustGet(t, baseURL+"/admin/stats", token)
    data := resp["data"].(map[string]interface{})

    for _, key := range []string{"wal", "hot", "cold", "runtime"} {
        if _, ok := data[key]; !ok {
            t.Errorf("admin/stats missing key %q", key)
        }
    }
}

func TestAdminWALRotate(t *testing.T) {
    baseURL, cleanup := testServer(t)
    defer cleanup()
    token := adminToken(t, baseURL)

    respBefore := mustGet(t, baseURL+"/admin/wal/stats", token)
    dataBefore := respBefore["data"].(map[string]interface{})
    segBefore := dataBefore["active_segment"].(float64)

    mustPost(t, baseURL+"/admin/wal/rotate", token, nil)

    respAfter := mustGet(t, baseURL+"/admin/wal/stats", token)
    dataAfter := respAfter["data"].(map[string]interface{})
    segAfter := dataAfter["active_segment"].(float64)

    if segAfter <= segBefore {
        t.Errorf("active segment after rotate = %v, want > %v", segAfter, segBefore)
    }
}

func TestAdminSchemaDeleteAndReset(t *testing.T) {
    baseURL, cleanup := testServer(t)
    defer cleanup()
    token := adminToken(t, baseURL)

    ingestLogs(t, baseURL, token, []map[string]interface{}{
        {"level": "info", "message": "schema test", "timestamp": nowTimestamp()},
    })

    expectStatus(t, http.MethodDelete, baseURL+"/admin/schema/logs", token, nil, http.StatusOK)

    resp := mustGet(t, baseURL+"/query/schema/logs", token)
    data := resp["data"].(map[string]interface{})
    count := int(data["record_count"].(float64))
    if count != 0 {
        t.Errorf("after schema delete, record_count = %d, want 0", count)
    }
}

func TestDocsEndpoints(t *testing.T) {
    baseURL, cleanup := testServer(t)
    defer cleanup()

    expectStatus(t, http.MethodGet, baseURL+"/openapi.json", "", nil, http.StatusOK)
    expectStatus(t, http.MethodGet, baseURL+"/docs", "", nil, http.StatusOK)
}

func TestBatchIngest(t *testing.T) {
    baseURL, cleanup := testServer(t)
    defer cleanup()
    token := adminToken(t, baseURL)

    records := make([]map[string]interface{}, 10)
    for i := 0; i < 10; i++ {
        records[i] = map[string]interface{}{
            "level":     "info",
            "message":   fmt.Sprintf("batch-%d", i),
            "timestamp": time.Now().Add(time.Duration(i) * time.Millisecond).UnixNano(),
        }
    }
    resp := mustPost(t, baseURL+"/ingest/logs", token, map[string]interface{}{"records": records})
    data := resp["data"].(map[string]interface{})
    if int(data["ingested"].(float64)) != 10 {
        t.Errorf("ingested = %v, want 10", data["ingested"])
    }

    total := queryLogs(t, baseURL, token)
    if total != 10 {
        t.Errorf("query total = %d, want 10", total)
    }
}
```

**Verify:** `CGO_ENABLED=1 go test -race -count=1 ./tests/integration/` — all tests pass.

---

## TASK 06 — Update Makefile with integration-test, check, vet, and build metadata targets

**Action:** Update `Makefile` carefully. If a target already exists, update it in place
instead of creating a duplicate target.

**Required variables near the top of the Makefile:**
```makefile
APP_NAME ?= plomvix
VERSION ?= 0.1.0
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
CGO_ENABLED ?= 1
export CGO_ENABLED

LD_FLAGS := -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)
```

**Build target must inject metadata:**
```makefile
## build: Build the Plomvix binary with version metadata
build:
	CGO_ENABLED=1 go build -ldflags "$(LD_FLAGS)" -o $(APP_NAME) ./cmd/plomvix
```

**New or updated test targets:**
```makefile
## test: Run unit tests with race detector (integration tests self-skip in short mode)
test:
	CGO_ENABLED=1 go test -short -race -cover ./...

## test-verbose: Run unit tests with verbose output (integration tests self-skip in short mode)
test-verbose:
	CGO_ENABLED=1 go test -short -race -cover -v ./...

## integration-test: Run integration tests (boots real server, slower)
integration-test:
	CGO_ENABLED=1 go test -race -count=1 -v ./tests/integration/...

## vet: Run go vet
vet:
	CGO_ENABLED=1 go vet ./...

## check: Run vet, lint, and all tests (unit + integration)
check: vet lint test integration-test
	@echo "All checks passed."
```

**Important Makefile rules:**
- Do not define duplicate `vet`, `test`, `build`, or `lint` targets.
- Keep existing `run`, `tidy`, `clean`, and `coverage` targets if present.
- Ensure `make build` still produces `./plomvix`.
- Ensure `./plomvix --version` shows non-empty Version, BuildTime, and GitCommit.

**Verify:**
```bash
make vet
make build
./plomvix --version
make test
make integration-test
make check
```

---

## TASK 07 — Audit and fix .gitignore

**Action:** Verify `/.gitignore` is correct and complete. It should already
exist from earlier sprints — this task audits and fills gaps added by Sprints 2–10.

**Required `.gitignore` content:**
```gitignore
# Binaries
plomvix
plomvixctl
*.exe

# Data directory contents
# Keep directory structure and .gitkeep files, ignore generated data.
data/wal/**
data/hot/**
data/cold/**
data/system/*
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
!data/system/.gitkeep

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

# Local smoke scripts/logs
smoke_test.sh
tmp/
*.log
```

**Also verify `.gitkeep` files exist in all required directories:**
```bash
mkdir -p data/wal data/hot data/cold/logs data/cold/metrics data/cold/json data/cold/kv data/system
touch data/wal/.gitkeep
touch data/hot/.gitkeep
touch data/cold/logs/.gitkeep
touch data/cold/metrics/.gitkeep
touch data/cold/json/.gitkeep
touch data/cold/kv/.gitkeep
touch data/system/.gitkeep
```

**Verify:**
```bash
git check-ignore -v data/wal/seg-000001.wal
git check-ignore -v data/hot/000123.sst
git check-ignore -v data/cold/logs/2024-01-01/part-000001.parquet
git check-ignore -v data/system/auth.db
! git check-ignore -q data/wal/.gitkeep
! git check-ignore -q data/hot/.gitkeep
! git check-ignore -q data/cold/logs/.gitkeep
! git check-ignore -q data/system/.gitkeep
```

Generated data files must be ignored. `.gitkeep` files must not be ignored.

---

## TASK 08 — Write README.md

**Action:** Write a complete `README.md` at the project root.

The README must be honest, accurate, and fully working on a fresh machine.
Someone should be able to clone the repo, read this, and have Plomvix running
in under 10 minutes.

**Full README.md content:**

```markdown
# Plomvix

**Indian-built, open-source, unified observability and general-purpose database.**

Plomvix supports logs, metrics, key-value records, and JSON data in a single
binary. Built in Go. Production grade. Resource friendly.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8.svg)](https://go.dev)

---

## What is Plomvix?

Plomvix is a unified observability database — a single server binary for teams
that need structured log storage, time-series metrics, JSON document storage,
and key-value access with minimal operational overhead.

**Why it exists:** most observability stacks require several separate systems,
each with its own operational burden. Plomvix consolidates the core storage,
ingest, query, and admin workflows into one Go service with no external runtime
services. RocksDB system libraries are required to build and run the hot tier.

---

## Current Status

Sprint 10 complete — v0.1.0 release candidate.

**What works:**
- Ingest logs, metrics, JSON documents, and KV pairs over HTTP
- Multi-format input for logs: JSON, CSV, logfmt, and syslog
- CSV input for JSON documents
- Automatic schema inference on writes
- Time-range queries with field filtering and pagination
- WAL-backed durability — every write is fsynced before acknowledgement
- Hot tier (RocksDB) and cold tier (Parquet) with manual/background tiering
- Admin APIs for WAL, cold tier, schema, and system stats
- Interactive API documentation at `/docs` powered by Stoplight Elements
- JWT and API key authentication

**Not included yet:**
- OTLP ingestion
- Prometheus exposition format
- Full SQL parser
- Multi-node/distributed mode
- RBAC beyond the current admin role

---

## Prerequisites

- Go 1.22 or higher
- `make`
- `git`
- `jq` for the copy-paste shell examples
- RocksDB system libraries for the hot tier

**Install RocksDB on Ubuntu/Debian:**
```bash
sudo apt-get update
sudo apt-get install -y librocksdb-dev libsnappy-dev liblz4-dev libzstd-dev libgflags-dev build-essential
```

**Install RocksDB on macOS:**
```bash
brew install rocksdb snappy lz4 zstd
```

---

## Getting Started

```bash
# Clone
git clone https://github.com/plomvix/plomvix.git
cd plomvix

# Install Go dependencies
go mod tidy

# Build
make build

# Run
./plomvix
```

The server starts on `http://localhost:8080`.

**Verify it is running:**
```bash
curl http://localhost:8080/health
```

**Login and ingest your first log:**
```bash
TOKEN=$(curl -sf -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"changeme"}' | jq -r '.data.token')

curl -X POST http://localhost:8080/ingest/logs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"records":[{"level":"info","message":"Hello Plomvix"}]}'

curl "http://localhost:8080/query/logs" \
  -H "Authorization: Bearer $TOKEN"
```

**View API documentation:**

Open `http://localhost:8080/docs` in your browser.

---

## Make Commands

| Command | Description |
|---|---|
| `make run` | Run without building a binary |
| `make build` | Build the `plomvix` binary with version metadata |
| `make test` | Run unit tests with race detector |
| `make integration-test` | Run end-to-end integration tests |
| `make check` | Run vet + lint + unit + integration tests |
| `make lint` | Run golangci-lint |
| `make tidy` | Tidy Go modules |
| `make clean` | Remove binary and coverage files |
| `make coverage` | Generate HTML coverage report |
| `make vet` | Run go vet |

---

## Configuration Reference

Edit `config.yaml` before running. All values can be overridden with environment
variables using the `PLOMVIX_` prefix.

| Key | Type | Default | Description |
|---|---|---|---|
| `env` | string | `development` | `development` or `production` |
| `server.host` | string | `0.0.0.0` | Interface to bind to |
| `server.port` | int | `8080` | Port to listen on |
| `server.read_timeout` | int | `30` | HTTP read timeout in seconds |
| `server.write_timeout` | int | `30` | HTTP write timeout in seconds |
| `server.idle_timeout` | int | `60` | HTTP idle timeout in seconds |
| `storage.data_dir` | string | `./data` | Root data directory |
| `storage.wal_flush_threshold` | int | `67108864` | WAL segment size in bytes |
| `storage.hot_tier_max_size` | int | `10737418240` | RocksDB max size in bytes |
| `storage.retention_days` | int | `30` | Days before data becomes eligible for cold tier |
| `compression.hot_tier` | string | `snappy` | RocksDB compression: `snappy`, `lz4`, `none` |
| `compression.cold_tier` | string | `zstd` | Parquet compression: `zstd`, `snappy`, `none` |
| `auth.default_admin_username` | string | `admin` | Initial admin username |
| `auth.default_admin_password` | string | `changeme` | **Change in production** |
| `auth.jwt_secret` | string | `plomvix-change-in-prod` | **Change in production** |
| `auth.jwt_expiry_seconds` | int | `3600` | JWT token lifetime |
| `auth.api_key_length` | int | `32` | API key length in bytes |
| `logging.level` | string | `info` | `debug`, `info`, `warn`, `error` |
| `logging.format` | string | `pretty` | `json` or `pretty` |

**Environment variable override examples:**
```bash
PLOMVIX_SERVER_PORT=9090 ./plomvix
PLOMVIX_LOGGING_FORMAT=json ./plomvix
PLOMVIX_AUTH_JWT_SECRET=mysecret ./plomvix
```

**Production mode** rejects unsafe defaults. Before running in production:
```yaml
env: production
auth:
  default_admin_password: your-strong-password
  jwt_secret: your-random-256-bit-secret
logging:
  format: json
```

---

## API Reference

Full interactive documentation is available at `http://localhost:8080/docs` when
the server is running. The raw OpenAPI 3.0 specification is available at
`http://localhost:8080/openapi.json`.

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/health` | None | Server health and stats |
| POST | `/auth/login` | None | Login, returns JWT |
| POST | `/auth/logout` | JWT | Invalidate token |
| POST | `/auth/refresh` | JWT | Refresh token |
| POST | `/ingest/logs` | JWT/APIKey | Ingest log records |
| POST | `/ingest/metrics` | JWT/APIKey | Ingest metric records |
| POST | `/ingest/json` | JWT/APIKey | Ingest JSON documents |
| POST | `/ingest/kv` | JWT/APIKey | Ingest key-value pairs |
| GET | `/query/logs` | JWT/APIKey | Query log records |
| GET | `/query/metrics` | JWT/APIKey | Query metric records |
| GET | `/query/json` | JWT/APIKey | Query JSON documents |
| GET | `/query/kv/{key}` | JWT/APIKey | Look up a KV record |
| GET | `/query/schema/{type}` | JWT/APIKey | Get inferred schema |
| POST | `/admin/tier/flush` | Admin | Trigger cold tier flush |
| GET | `/admin/stats` | Admin | Consolidated system stats |
| GET | `/admin/info` | Admin | Version and build info |
| GET | `/admin/wal/stats` | Admin | WAL statistics |
| POST | `/admin/wal/rotate` | Admin | Force WAL segment rotation |
| GET | `/admin/cold/stats` | Admin | Cold tier statistics |
| GET | `/admin/schema` | Admin | All inferred schemas |
| DELETE | `/admin/schema/{type}` | Admin | Reset schema for a type |
| GET | `/openapi.json` | None | OpenAPI 3.0 specification |
| GET | `/docs` | None | Interactive API documentation |

---

## Architecture

```text
                    HTTP Request
                         │
                    ┌────▼────┐
                    │  Router │  (Chi)
                    └────┬────┘
                         │
          ┌──────────────┼──────────────┐
          │              │              │
     ┌────▼────┐   ┌────▼────┐   ┌────▼────┐
     │  Auth   │   │ Ingest  │   │  Query  │
     └─────────┘   └────┬────┘   └────┬────┘
                        │              │
                   ┌────▼────┐        │
                   │   WAL   │        │
                   └────┬────┘        │
                        │              │
                   ┌────▼────┐   ┌────▼────┐
                   │Hot Tier │◄──┤  Merge  │◄── Cold Tier
                   │(RocksDB)│   └─────────┘   (Parquet)
                   └────┬────┘
                        │
                   ┌────▼────┐
                   │Tiering  │
                   │ Engine  │
                   └─────────┘
```

**Storage layers:**
- **WAL** — Write Ahead Log. Every write is fsynced here first.
- **Hot Tier** — RocksDB. Fast reads and writes for recent data.
- **Cold Tier** — Parquet files partitioned by date for older logs, metrics, and JSON.

---

## Project Structure

```text
plomvix/
├── api/                    ← OpenAPI specification
├── cmd/plomvix/main.go     ← entry point
├── config.yaml             ← configuration
├── docs/api/               ← endpoint-specific API docs
├── internal/
│   ├── admin/              ← admin API handlers
│   ├── auth/               ← JWT + API key authentication
│   ├── config/             ← config loader and validation
│   ├── ingestion/          ← ingest handlers + schema inference
│   ├── logger/             ← structured Zap logger
│   ├── parser/             ← multi-format parsers
│   ├── query/              ← query engine + filter parser
│   ├── server/             ← HTTP server + middleware
│   └── storage/
│       ├── cold/           ← Parquet cold tier
│       ├── hot/            ← RocksDB hot tier
│       └── wal/            ← Write Ahead Log
├── pkg/utils/              ← shared utilities + API response helpers
└── tests/integration/      ← end-to-end integration tests
```

---

## Roadmap

- [x] Sprint 1 — Project skeleton, config, logger, HTTP server
- [x] Sprint 2 — Auth system (JWT + API keys)
- [x] Sprint 3 — Write Ahead Log
- [x] Sprint 4 — Hot tier (RocksDB)
- [x] Sprint 5 — Ingestion API + schema inference
- [x] Sprint 6 — Query engine
- [x] Sprint 7 — Cold tier (Parquet) + tiering
- [x] Sprint 8 — Multi-format parsers (CSV, logfmt, syslog)
- [x] Sprint 9 — Admin APIs + Stoplight Elements docs
- [x] Sprint 10 — Polish, integration tests, documentation
- [ ] v0.2 — Prometheus metrics endpoint
- [ ] v0.2 — OTLP ingestion
- [ ] v0.3 — Distributed mode
- [ ] v0.3 — SQL query language

---

## Contributing

Contributions are welcome.

1. Open an issue to discuss what you want to change.
2. Fork the repo and create a branch.
3. Make your changes. Run `make check` before submitting.
4. Open a pull request.

**Code style:** `make lint` must pass with zero issues.

---

## License

MIT License. See [LICENSE](LICENSE) for details.

*Plomvix — Built in India. Built for the world.*
```

**Verify:**
- README renders correctly on GitHub — tables, code blocks, and headers display
- Getting Started section works on a fresh machine with RocksDB installed
- No placeholder text remains
- README does not claim unsupported features such as OTLP, Prometheus, SQL, RBAC, or distributed mode as complete

---

## TASK 09 — Final build and complete smoke test

**Action:**

```bash
#!/bin/bash
set -euo pipefail

echo "=== Clearing stale data ==="
rm -rf data/hot/ data/wal/ data/cold/
rm -f data/system/auth.db

echo ""
echo "=== Step 1: go mod tidy ==="
go mod tidy
git diff --exit-code go.mod go.sum \
    && echo "PASS: go.mod and go.sum are clean" \
    || { echo "FAIL: go.mod or go.sum has uncommitted changes"; exit 1; }

echo ""
echo "=== Step 2: gofmt ==="
UNFORMATTED=$(gofmt -l $(find . -name '*.go' -not -path './vendor/*'))
if [ -n "$UNFORMATTED" ]; then
    echo "FAIL: files not formatted with gofmt:"
    echo "$UNFORMATTED"
    exit 1
fi
echo "PASS: all files correctly formatted"

echo ""
echo "=== Step 3: go vet ==="
CGO_ENABLED=1 make vet
echo "PASS: go vet clean"

echo ""
echo "=== Step 4: Lint ==="
CGO_ENABLED=1 make lint
echo "PASS: lint clean"

echo ""
echo "=== Step 5: Unit tests ==="
CGO_ENABLED=1 make test
echo "PASS: unit tests passed"

echo ""
echo "=== Step 6: Integration tests ==="
CGO_ENABLED=1 make integration-test
echo "PASS: integration tests passed"

echo ""
echo "=== Step 7: Build with version injection ==="
CGO_ENABLED=1 make build
echo "PASS: binary built"

echo ""
echo "=== Step 8: Version flag ==="
./plomvix --version | grep -q "Plomvix" \
    && echo "PASS: --version output correct" \
    || { echo "FAIL: --version output wrong"; exit 1; }

echo ""
echo "=== Step 9: Server boots and health check passes ==="
SERVER_PID=""
cleanup() {
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        kill -SIGTERM "$SERVER_PID" 2>/dev/null || true
        wait "$SERVER_PID" 2>/dev/null || true
    fi
}
trap cleanup EXIT

./plomvix > /tmp/plomvix_s10.log 2>&1 &
SERVER_PID=$!
sleep 3

HEALTH=$(curl -sf http://localhost:8080/health)
echo "$HEALTH" | jq '.status' | grep -q "ok" \
    && echo "PASS: health check ok" \
    || { echo "FAIL: health check failed"; exit 1; }

echo ""
echo "=== Step 10: OpenAPI spec is valid JSON ==="
curl -sf http://localhost:8080/openapi.json | python3 -m json.tool > /dev/null \
    && echo "PASS: OpenAPI spec valid" \
    || { echo "FAIL: OpenAPI spec invalid"; exit 1; }

echo ""
echo "=== Step 11: Docs UI loads ==="
curl -sf http://localhost:8080/docs | grep -q "elements-api" \
    && echo "PASS: docs UI loads" \
    || { echo "FAIL: docs UI missing elements-api"; exit 1; }

echo ""
echo "=== Step 12: Environment variable override ==="
kill -SIGTERM "$SERVER_PID"
wait "$SERVER_PID" 2>/dev/null || true
SERVER_PID=""
sleep 1

PLOMVIX_SERVER_PORT=9090 ./plomvix > /tmp/plomvix_s10_9090.log 2>&1 &
SERVER_PID=$!
sleep 3

curl -sf http://localhost:9090/health | jq '.status' | grep -q "ok" \
    && echo "PASS: env var port override works" \
    || { echo "FAIL: env var port override failed"; exit 1; }

echo ""
echo "=== Step 13: Graceful shutdown ==="
kill -SIGTERM "$SERVER_PID"
wait "$SERVER_PID"
EXIT_CODE=$?
SERVER_PID=""
[ "$EXIT_CODE" -eq 0 ] \
    && echo "PASS: clean shutdown (exit code 0)" \
    || { echo "FAIL: exit code $EXIT_CODE"; exit 1; }

echo ""
echo "================================================"
echo "  ALL STEPS PASSED — Sprint 10 FINAL CHECK DONE"
echo "  Plomvix v0.1.0 is release-ready.             "
echo "================================================"
```

| Step | Verified | Expected |
|---|---|---|
| 1 | `go mod tidy` | Clean, no uncommitted changes |
| 2 | `gofmt` | All files formatted |
| 3 | `go vet` | Zero issues |
| 4 | Lint | Zero issues |
| 5 | Unit tests | All pass with race detector |
| 6 | Integration tests | All pass |
| 7 | Build | Binary produced with ldflags |
| 8 | `--version` | Correct output |
| 9 | Health check | Returns ok |
| 10 | OpenAPI spec | Valid JSON |
| 11 | Docs UI | `<elements-api>` present |
| 12 | Env var override | Port 9090 works |
| 13 | Graceful shutdown | Exit code 0 |

---

## EXECUTION ORDER SUMMARY

```
TASK 01  →  Linter cleanup (gofmt + make lint — zero issues)
TASK 02  →  internal/server/server.go (race-safe Addr() method, refactor Start())
TASK 03  →  tests/integration/helpers_test.go
TASK 04  →  tests/integration/ingest_query_test.go
TASK 05  →  tests/integration/tiering_test.go
TASK 06  →  Makefile (integration-test, check, vet, test -short, build ldflags)
TASK 07  →  .gitignore audit + .gitkeep verification
TASK 08  →  README.md (complete, accurate, working)
TASK 09  →  Final smoke test — all 13 steps must pass
```

---

*Sprint 10 complete when TASK 09 passes with zero failures.*
*Plomvix v0.1.0 — Built in India. Built for the world.*