# Plomvix — Sprint 9 Task Plan
### For: DeepSeek V4 Pro Coding Agent
### Language: Go 1.22 | Module: github.com/plomvix/plomvix

> Execute tasks in exact order. Each task is atomic — one file or one concern.
> Do not skip ahead. Each task depends on the previous.
> Every task has a Verify step — do not proceed until it passes.

---

## CONTEXT

Sprints 1–8 are complete. Sprint 9 adds **Admin APIs** and **API Documentation**
powered by Stoplight Elements.

**What Sprint 9 delivers:**
- `GET /admin/stats` — consolidated system stats (WAL + hot + cold + runtime)
- `GET /admin/info` — version, build time, git commit, Go version, uptime
- `GET /admin/wal/stats` — detailed WAL statistics
- `POST /admin/wal/rotate` — force WAL segment rotation
- `GET /admin/cold/stats` — detailed cold tier statistics
- `GET /admin/schema` — list all inferred schemas for all data types
- `DELETE /admin/schema/{type}` — reset the schema for a data type
- `GET /openapi.json` — serves the complete OpenAPI 3.0 specification (public, no auth)
- `GET /docs` — serves the Stoplight Elements interactive API documentation UI (public, no auth)
- All new admin endpoints require admin authentication
- Full test coverage for new admin handlers
- OpenAPI coverage for Sprint 8 multi-format ingest support
- No new Go dependencies — Stoplight Elements loads via CDN in the served HTML

**What Sprint 9 does NOT do:**
- No OTLP — deferred
- No code-annotation-based spec generation — the OpenAPI spec is handwritten
  and lives at `api/openapi.json`. Handwritten specs are more accurate and easier
  to maintain for a project of this size.
- No authentication on `/docs` or `/openapi.json` — docs are public by design
- No reconciliation tooling for Sprint 7 partial cold-tier flush failures — deferred to a future sprint
- No new parser formats beyond Sprint 8 — Sprint 9 only documents and administers existing features

---

## STOPLIGHT ELEMENTS INTEGRATION — READ BEFORE WRITING ANY CODE

Stoplight Elements is a Web Component loaded via CDN. The Go server needs to
serve exactly two things:

1. **`GET /docs`** — an HTML page that loads Elements via CDN and points it at `/openapi.json`
2. **`GET /openapi.json`** — the OpenAPI 3.0 spec as JSON

**No npm, no build step, no new Go dependencies.** The HTML page is a Go template
or a raw string embedded in the binary.

**HTML page template (exact):**
```html
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Plomvix API Docs</title>
  <script src="https://unpkg.com/@stoplight/elements/web-components.min.js"></script>
  <link rel="stylesheet" href="https://unpkg.com/@stoplight/elements/styles.min.css">
  <style>
    html, body { margin: 0; padding: 0; height: 100%; }
    elements-api { display: block; height: 100%; }
  </style>
</head>
<body>
  <elements-api
    apiDescriptionUrl="/openapi.json"
    router="hash"
    layout="sidebar"
  />
</body>
</html>
```

**Key attributes:**
- `apiDescriptionUrl="/openapi.json"` — points to the spec served by the same server
- `router="hash"` — uses URL hash routing, works without server-side routing
- `layout="sidebar"` — three-column Stripe-style layout

**Compatibility note:** Stoplight Elements supports loading via script/CSS CDN and an
`<elements-api>` web component. Configuration attributes include `apiDescriptionUrl`
and `layout`; keep the HTML minimal and do not add a frontend build step.

---

## OPENAPI SPEC DESIGN — READ BEFORE WRITING ANY CODE

The OpenAPI spec lives at `api/openapi.json`. It covers ALL endpoints from Sprints 1–9.

**Spec structure:**
```json
{
  "openapi": "3.0.3",
  "info": {
    "title": "Plomvix API",
    "version": "0.1.0",
    "description": "Indian-built unified observability database"
  },
  "servers": [{"url": "http://localhost:8080"}],
  "components": {
    "securitySchemes": {
      "BearerAuth": {"type": "http", "scheme": "bearer"},
      "APIKeyAuth": {"type": "apiKey", "in": "header", "name": "X-API-Key"}
    },
    "schemas": { ... }
  },
  "security": [],
  "paths": { ... }
}
```

**Endpoints to document (all sprints):**

| Sprint | Endpoints |
|---|---|
| 1 | `GET /health` |
| 2 | `POST /auth/login`, `POST /auth/logout`, `POST /auth/refresh`, `POST /admin/users`, `GET /admin/users`, `GET /admin/users/{id}`, `PATCH /admin/users/{id}`, `DELETE /admin/users/{id}`, `POST /admin/users/{id}/apikey`, `DELETE /admin/users/{id}/apikey`, `GET /admin/users/{id}/apikey/status` |
| 5 | `POST /ingest/logs`, `POST /ingest/metrics`, `POST /ingest/json`, `POST /ingest/kv` |
| 6 | `GET /query/logs`, `GET /query/metrics`, `GET /query/json`, `GET /query/kv/{key}`, `GET /query/schema/{type}` |
| 7 | `POST /admin/tier/flush` |
| 9 | `GET /admin/stats`, `GET /admin/info`, `GET /admin/wal/stats`, `POST /admin/wal/rotate`, `GET /admin/cold/stats`, `GET /admin/schema`, `DELETE /admin/schema/{type}`, `GET /openapi.json`, `GET /docs` |

**Security on all endpoints:**
- Public (no auth): `GET /health`, `POST /auth/login`, `GET /openapi.json`, `GET /docs`
- JWT or API key: all ingest and query endpoints, `POST /auth/logout`, `POST /auth/refresh`
- Admin only: all `/admin/*` endpoints

**Sprint 8 format support to document:**
- `/ingest/logs` accepts `application/json`, `text/csv`, `text/x-logfmt`, and `application/x-syslog`
- `/ingest/json` accepts `application/json` and `text/csv`
- `/ingest/metrics` and `/ingest/kv` remain JSON-only
- Unknown or absent `Content-Type` falls back to JSON for backward compatibility

**Standard response schemas to define once in `components/schemas`:**
- `SuccessResponse` — `{status: "ok", data: {}, request_id: ""}`
- `ErrorResponse` — `{status: "error", error: {code, message, details[]}, request_id: ""}`
- `UserResponse` — user object
- `IngestResponse` — `{ingested: int, request_id: ""}`
- `QueryResult` — `{records: [], count, total, limit, offset, query_ms, data_type}`
- `Schema` — `{data_type, fields: {}, updated_at, record_count}`

---

## TASK 01 — Create api/ directory and openapi.json

**Action — Part A:** Create the directory:
```bash
mkdir -p api
```

**Action — Part B:** Create `api/openapi.json`.

This is a large file. Write it in full — no placeholders. Every endpoint
from Sprints 1–9 must be documented with correct request/response schemas.

**CRITICAL:** The sample structure below is illustrative only. Do NOT leave literal
`...`, `{ ... }`, empty path objects, or TODO placeholders in `api/openapi.json`.
The validation task later fails if any placeholders remain.

**Full file structure** (agent must fill in all paths completely):

```json
{
  "openapi": "3.0.3",
  "info": {
    "title": "Plomvix API",
    "version": "0.1.0",
    "description": "Plomvix — Indian-built, open-source unified observability and general-purpose database. Supports logs, metrics, telemetry, key-value, and JSON data.",
    "contact": {
      "name": "Plomvix",
      "url": "https://github.com/plomvix/plomvix"
    },
    "license": {
      "name": "MIT",
      "url": "https://opensource.org/licenses/MIT"
    }
  },
  "servers": [
    {
      "url": "http://localhost:8080",
      "description": "Local development server"
    }
  ],
  "components": {
    "securitySchemes": {
      "BearerAuth": {
        "type": "http",
        "scheme": "bearer",
        "bearerFormat": "JWT",
        "description": "JWT token obtained from POST /auth/login"
      },
      "APIKeyAuth": {
        "type": "apiKey",
        "in": "header",
        "name": "X-API-Key",
        "description": "API key generated via POST /admin/users/{id}/apikey"
      }
    },
    "schemas": {
      "ErrorBody": {
        "type": "object",
        "required": ["code", "message"],
        "properties": {
          "code":    { "type": "string", "example": "VALIDATION_FAILED" },
          "message": { "type": "string", "example": "username and password are required" },
          "details": { "type": "array", "items": { "type": "string" } }
        }
      },
      "ErrorResponse": {
        "type": "object",
        "required": ["status", "error"],
        "properties": {
          "status":     { "type": "string", "example": "error" },
          "error":      { "$ref": "#/components/schemas/ErrorBody" },
          "request_id": { "type": "string", "format": "uuid" }
        }
      },
      "SuccessResponse": {
        "type": "object",
        "required": ["status"],
        "properties": {
          "status":     { "type": "string", "example": "ok" },
          "data":       { "type": "object" },
          "request_id": { "type": "string", "format": "uuid" }
        }
      },
      "UserResponse": {
        "type": "object",
        "properties": {
          "id":         { "type": "string", "format": "uuid" },
          "username":   { "type": "string" },
          "role":       { "type": "string", "example": "admin" },
          "created_at": { "type": "string", "format": "date-time" },
          "updated_at": { "type": "string", "format": "date-time" }
        }
      },
      "IngestResponse": {
        "type": "object",
        "properties": {
          "ingested":   { "type": "integer", "example": 3 },
          "request_id": { "type": "string", "format": "uuid" }
        }
      },
      "QueryResult": {
        "type": "object",
        "properties": {
          "records":   { "type": "array", "items": { "type": "object" } },
          "count":     { "type": "integer" },
          "total":     { "type": "integer" },
          "limit":     { "type": "integer" },
          "offset":    { "type": "integer" },
          "query_ms":  { "type": "integer" },
          "data_type": { "type": "string" }
        }
      },
      "Schema": {
        "type": "object",
        "properties": {
          "data_type":    { "type": "string" },
          "fields":       { "type": "object", "additionalProperties": { "type": "string" } },
          "updated_at":   { "type": "string", "format": "date-time" },
          "record_count": { "type": "integer" }
        }
      }
    }
  },
  "paths": {
    "/health": { ... },
    "/auth/login": { ... },
    "/auth/logout": { ... },
    "/auth/refresh": { ... },
    "/admin/users": { ... },
    "/admin/users/{id}": { ... },
    "/admin/users/{id}/apikey": { ... },
    "/admin/users/{id}/apikey/status": { ... },
    "/ingest/logs": { ... },
    "/ingest/metrics": { ... },
    "/ingest/json": { ... },
    "/ingest/kv": { ... },
    "/query/logs": { ... },
    "/query/metrics": { ... },
    "/query/json": { ... },
    "/query/kv/{key}": { ... },
    "/query/schema/{type}": { ... },
    "/admin/tier/flush": { ... },
    "/admin/stats": { ... },
    "/admin/info": { ... },
    "/admin/wal/stats": { ... },
    "/admin/wal/rotate": { ... },
    "/admin/cold/stats": { ... },
    "/admin/schema": { ... },
    "/admin/schema/{type}": { ... },
    "/openapi.json": { ... },
    "/docs": { ... }
  }
}
```

**Each path entry must include:**
- `summary` — one-line description
- `description` — full description
- `tags` — group endpoints (Health, Auth, Users, APIKeys, Ingest, Query, Admin, Docs)
- `security` — either `[]` for public endpoints or `[{"BearerAuth":[]},{"APIKeyAuth":[]}]` for protected/admin endpoints
- `parameters` — path and query params with types and descriptions
- `requestBody` — for POST/PATCH/PATCH-like methods with `content`, `schema`, and `example`
- `responses` — include success plus relevant `400`, `401`, `403`, `404`, `409`, and `500` responses
- `operationId` — unique camelCase identifier (e.g. `loginUser`, `ingestLogs`)

**Admin endpoint security in OpenAPI:** OpenAPI security schemes express JWT/API-key
authentication, not the admin role itself. For every `/admin/*` operation, set
`security` to `[{"BearerAuth":[]},{"APIKeyAuth":[]}]` and state `Admin role required`
in the operation description.

**Tags to define at top level:**
```json
"tags": [
  {"name": "Health",  "description": "Server health and system stats"},
  {"name": "Auth",    "description": "Authentication — login, logout, token refresh"},
  {"name": "Users",   "description": "User account management (admin only)"},
  {"name": "APIKeys", "description": "API key management (admin only)"},
  {"name": "Ingest",  "description": "Data ingestion — logs, metrics, JSON, KV"},
  {"name": "Query",   "description": "Data queries — time-range scans and point lookups"},
  {"name": "Admin",   "description": "System administration — WAL, cold tier, schema, stats"},
  {"name": "Docs",    "description": "API documentation"}
]
```

**Verify:**
```bash
cat api/openapi.json | python3 -m json.tool > /dev/null
! grep -R "\.\.\.\|TODO\|PLACEHOLDER" api/openapi.json
```
Valid JSON, no placeholders, no literal ellipsis.

---

## TASK 02 — Create internal/admin/ directory and handler.go

**Action — Part A:**
```bash
mkdir -p internal/admin
rm -f internal/admin/.gitkeep
```

**Action — Part B:** Create `internal/admin/handler.go`.

**Imports required:**
```go
import (
    "net/http"
    "runtime"
    "time"

    "github.com/go-chi/chi/v5"
    "go.uber.org/zap"

    "github.com/plomvix/plomvix/internal/ingestion"
    "github.com/plomvix/plomvix/internal/logger"
    "github.com/plomvix/plomvix/internal/storage/cold"
    hotstore "github.com/plomvix/plomvix/internal/storage/hot"
    walstore "github.com/plomvix/plomvix/internal/storage/wal"
    "github.com/plomvix/plomvix/pkg/utils"
)
```

**Handler struct:**
```go
// Handler handles all Sprint 9 admin endpoints.
type Handler struct {
    wal       *walstore.Manager
    hot       *hotstore.Manager
    cold      *cold.Store
    version   string
    buildTime string
    gitCommit string
    startTime time.Time
}

// NewHandler creates a new admin Handler.
func NewHandler(
    wal *walstore.Manager,
    hot *hotstore.Manager,
    cold *cold.Store,
    version, buildTime, gitCommit string,
    startTime time.Time,
) *Handler {
    return &Handler{
        wal:       wal,
        hot:       hot,
        cold:      cold,
        version:   version,
        buildTime: buildTime,
        gitCommit: gitCommit,
        startTime: startTime,
    }
}
```

---

**`GET /admin/stats`**

Returns a consolidated view of all system statistics.

```go
// Stats handles GET /admin/stats.
//
// GET /admin/stats
// Auth: admin only
//
// Responses:
//   200 OK — consolidated system stats
func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
    walStats := h.wal.Stats()
    hotStats := h.hot.Stats()

    var coldStats cold.TierStats
    if h.cold != nil {
        coldStats = h.cold.Stats()
    }

    var memStats runtime.MemStats
    runtime.ReadMemStats(&memStats)

    utils.OK(w, r, map[string]interface{}{
        "wal": map[string]interface{}{
            "segment_count":    walStats.SegmentCount,
            "active_segment":   walStats.ActiveSegment,
            "active_size_bytes": walStats.ActiveSizeBytes,
            "total_entries":    walStats.TotalEntries,
        },
        "hot": map[string]interface{}{
            "total_writes":      hotStats.TotalWrites,
            "total_data_writes": hotStats.TotalDataWrites,
            "data_dir":          hotStats.DataDir,
        },
        "cold": map[string]interface{}{
            "parquet_files":   coldStats.TotalParquetFiles,
            "records_moved":   coldStats.TotalRecordsMoved,
            "last_flush_at":   coldStats.LastFlushAt,
        },
        "runtime": map[string]interface{}{
            "goroutines":    runtime.NumGoroutine(),
            "alloc_bytes":   memStats.Alloc,
            "sys_bytes":     memStats.Sys,
            "gc_cycles":     memStats.NumGC,
        },
    })
}
```

---

**`GET /admin/info`**

Returns server version and build information.

```go
// Info handles GET /admin/info.
//
// GET /admin/info
// Auth: admin only
//
// Responses:
//   200 OK — version and build info
func (h *Handler) Info(w http.ResponseWriter, r *http.Request) {
    utils.OK(w, r, map[string]interface{}{
        "version":        h.version,
        "build_time":     h.buildTime,
        "git_commit":     h.gitCommit,
        "go_version":     runtime.Version(),
        "os_arch":        runtime.GOOS + "/" + runtime.GOARCH,
        "uptime_seconds": int64(time.Since(h.startTime).Seconds()),
    })
}
```

---

**`GET /admin/wal/stats`**

Returns detailed WAL statistics.

```go
// WALStats handles GET /admin/wal/stats.
//
// GET /admin/wal/stats
// Auth: admin only
//
// Responses:
//   200 OK — WAL stats
func (h *Handler) WALStats(w http.ResponseWriter, r *http.Request) {
    stats := h.wal.Stats()
    utils.OK(w, r, map[string]interface{}{
        "segment_count":     stats.SegmentCount,
        "active_segment":    stats.ActiveSegment,
        "active_size_bytes": stats.ActiveSizeBytes,
        "total_entries":     stats.TotalEntries,
    })
}
```

---

**`POST /admin/wal/rotate`**

Forces the WAL to rotate to a new segment immediately, regardless of size.

```go
// WALRotate handles POST /admin/wal/rotate.
//
// POST /admin/wal/rotate
// Auth: admin only
//
// Responses:
//   200 OK       — rotation complete, returns new active segment index
//   500 Internal — INTERNAL_ERROR: rotation failed
func (h *Handler) WALRotate(w http.ResponseWriter, r *http.Request) {
    if err := h.wal.Rotate(); err != nil {
        logger.Error("WAL rotation failed", zap.Error(err))
        utils.InternalError(w, r, "WAL rotation failed")
        return
    }
    stats := h.wal.Stats()
    utils.OK(w, r, map[string]interface{}{
        "message":        "WAL segment rotated",
        "active_segment": stats.ActiveSegment,
    })
}
```

**NOTE:** `wal.Rotate()` does not exist yet. Add it in TASK 03.

---

**`GET /admin/cold/stats`**

Returns detailed cold tier statistics.

```go
// ColdStats handles GET /admin/cold/stats.
//
// GET /admin/cold/stats
// Auth: admin only
//
// Responses:
//   200 OK — cold tier stats
func (h *Handler) ColdStats(w http.ResponseWriter, r *http.Request) {
    if h.cold == nil {
        utils.InternalError(w, r, "cold tier not available")
        return
    }
    stats := h.cold.Stats()
    utils.OK(w, r, map[string]interface{}{
        "parquet_files":       stats.TotalParquetFiles,
        "records_moved":       stats.TotalRecordsMoved,
        "last_flush_at":       stats.LastFlushAt,
        "last_flush_duration": stats.LastFlushDuration.String(),
    })
}
```

---

**`GET /admin/schema`**

Returns the inferred schemas for all data types (`logs`, `metrics`, `json`, `kv`).
KV is not cold-tiered in Sprint 7, but it still has ingestion/schema metadata.

```go
// SchemaList handles GET /admin/schema.
//
// GET /admin/schema
// Auth: admin only
//
// Responses:
//   200 OK — map of data type → schema
func (h *Handler) SchemaList(w http.ResponseWriter, r *http.Request) {
    // Import ingestion package for LoadSchema
    // schemas are loaded from the hot tier _meta CF
    types := []string{"logs", "metrics", "json", "kv"}
    result := make(map[string]interface{}, len(types))
    for _, dt := range types {
        schema, err := ingestion.LoadSchema(h.hot, dt)
        if err != nil {
            utils.InternalError(w, r, "failed to load schema for "+dt)
            return
        }
        result[dt] = schema
    }
    utils.OK(w, r, result)
}
```


---

**`DELETE /admin/schema/{type}`**

Resets (deletes) the inferred schema for a data type.

```go
// SchemaDelete handles DELETE /admin/schema/{type}.
//
// DELETE /admin/schema/{type}
// Auth: admin only
//
// Path param: type — one of: logs, metrics, json, kv
//
// Responses:
//   200 OK          — schema reset
//   400 Bad Request — VALIDATION_FAILED: unknown data type
//   500 Internal    — INTERNAL_ERROR
func (h *Handler) SchemaDelete(w http.ResponseWriter, r *http.Request) {
    dataType := chi.URLParam(r, "type")
    valid := map[string]bool{"logs": true, "metrics": true, "json": true, "kv": true}
    if !valid[dataType] {
        utils.BadRequest(w, r, utils.CodeValidationFailed,
            "type must be one of: logs, metrics, json, kv")
        return
    }
    if err := ingestion.DeleteSchema(h.hot, dataType); err != nil {
        utils.InternalError(w, r, "failed to delete schema")
        return
    }
    utils.OK(w, r, map[string]interface{}{
        "message":   "schema reset",
        "data_type": dataType,
    })
}
```


**NOTE:** `ingestion.DeleteSchema` does not exist yet. Add it in TASK 04.

**Verify:** `go build ./internal/admin/` compiles after TASKs 03 and 04.

**Task ordering note:** This handler intentionally references `wal.Rotate()` and
`ingestion.DeleteSchema()` before those methods exist. Continue immediately to
TASK 03 and TASK 04, then run the package build verify.

---

## TASK 03 — Add Rotate() to internal/storage/wal/manager.go

**Action:** Add the `Rotate()` method to `internal/storage/wal/manager.go`.

```go
// Rotate forces the WAL to close the current segment and open a new one,
// regardless of the current segment size.
//
// Important: Writer.rotate() requires the caller to hold writer.mu.
// Manager.Rotate is the public safe wrapper used by admin APIs.
func (m *Manager) Rotate() error {
    if m == nil || m.writer == nil {
        return fmt.Errorf("WAL writer not initialized")
    }

    m.writer.mu.Lock()
    defer m.writer.mu.Unlock()

    if m.writer.currentFile == nil {
        return fmt.Errorf("WAL writer is closed")
    }
    return m.writer.rotate()
}
```

**Import check:** `manager.go` already imports `fmt` in Sprint 3. If it has been
removed, add it back.

**Why this matters:** `Writer.rotate()` explicitly says the caller must hold the
mutex. Calling it directly without locking can race with concurrent writes.

**Verify:** `CGO_ENABLED=1 go build ./internal/storage/wal/` compiles with no errors.

---

## TASK 04 — Add DeleteSchema to internal/ingestion/schema.go

**Action — Part A:** Add `DeleteMeta` to `internal/storage/hot/manager.go`.

```go
// DeleteMeta removes a key from the _meta column family.
func (m *Manager) DeleteMeta(key []byte) error {
    return m.store.Delete(CFMeta, key)
}
```

**Action — Part B:** Add `DeleteSchema` to `internal/ingestion/schema.go`.

```go
// DeleteSchema removes the stored schema for a data type from the _meta CF.
// After deletion, schema inference starts fresh on the next ingest.
func DeleteSchema(store *hot.Manager, dataType string) error {
    return store.DeleteMeta(schemaKey(dataType))
}
```

**Do NOT use this incorrect implementation:**
```go
return store.PutMeta(schemaKey(dataType), nil)
```
Passing `nil` to RocksDB `PutCF` is not valid for deleting metadata. Use
`DeleteMeta` so the key is actually removed from `_meta`.

**Verify:**
```bash
CGO_ENABLED=1 go build ./internal/storage/hot/
go build ./internal/ingestion/
CGO_ENABLED=1 go build ./internal/admin/
```

---

## TASK 05 — Register admin routes and docs endpoints in internal/server/server.go

**Action:** Five targeted changes to `internal/server/server.go` and one call-site
change in `cmd/plomvix/main.go`.

**Change 1 — Add admin handler field to Server struct:**
```go
type Server struct {
    // ...existing fields from Sprint 7...
    adminHandler *adminpkg.Handler // ← ADD
}
```

**Import aliases to add:**
```go
adminpkg "github.com/plomvix/plomvix/internal/admin"
```

Sprint 7 already added these fields/imports to `Server`; keep them unchanged:
```go
cold       *coldstore.Store
tierEngine *coldstore.TieringEngine
```

**Change 2 — Update `New()` signature to pass build metadata explicitly:**

`BuildTime` and `GitCommit` are package-level variables in `cmd/plomvix/main.go`,
not config fields. Do not access `cfg.BuildTime` or `cfg.GitCommit`.

```go
func New(cfg *config.Config, version, buildTime, gitCommit string,
    store *auth.Store, blacklist *auth.Blacklist,
    wal *walmanager.Manager, hotTier *hotmanager.Manager,
    cold *coldstore.Store, tierEngine *coldstore.TieringEngine) *Server {

    s := &Server{
        router:      chi.NewRouter(),
        cfg:         cfg,
        startTime:   time.Now(),
        version:     version,
        store:       store,
        blacklist:   blacklist,
        wal:         wal,
        hotTier:     hotTier,
        cold:        cold,
        tierEngine:  tierEngine,
        adminHandler: adminpkg.NewHandler(
            wal, hotTier, cold,
            version, buildTime, gitCommit,
            time.Now(),
        ),
    }
    // ...httpServer setup, middleware, routes unchanged...
    return s
}
```

**Change 3 — Update `cmd/plomvix/main.go` call site:**
```go
// Change:
srv := server.New(cfg, Version, store, blacklist, wal, hotTier, coldTier, tierEngine)

// To:
srv := server.New(cfg, Version, BuildTime, GitCommit,
    store, blacklist, wal, hotTier, coldTier, tierEngine)
```

**Change 4 — Add Sprint 9 admin routes inside the existing admin-only route group.**

Place these inside the same group that already uses both:
```go
r.Use(auth.Middleware(s.store, s.blacklist, s.cfg))
r.Use(auth.RequireAdmin())
```

Routes to add:
```go
r.Get("/admin/stats",            s.adminHandler.Stats)
r.Get("/admin/info",             s.adminHandler.Info)
r.Get("/admin/wal/stats",        s.adminHandler.WALStats)
r.Post("/admin/wal/rotate",      s.adminHandler.WALRotate)
r.Get("/admin/cold/stats",       s.adminHandler.ColdStats)
r.Get("/admin/schema",           s.adminHandler.SchemaList)
r.Delete("/admin/schema/{type}", s.adminHandler.SchemaDelete)
```

Keep the Sprint 7 manual tier flush route in the same admin-only group:
```go
r.Post("/admin/tier/flush", s.handleTierFlush)
```

**Change 5 — Add public docs and spec endpoints (no auth required):**

Add these in the public section alongside `/health` and `/auth/login`:
```go
r.Get("/openapi.json", s.handleOpenAPISpec)
r.Get("/docs",         s.handleDocs)
```

Add the two handlers to `server.go`:

```go
// handleOpenAPISpec serves the OpenAPI 3.0 JSON specification.
//
// GET /openapi.json
// Auth: none (public)
func (s *Server) handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Cache-Control", "public, max-age=3600")
    http.ServeFile(w, r, "api/openapi.json")
}

// handleDocs serves the Stoplight Elements interactive API documentation UI.
//
// GET /docs
// Auth: none (public)
func (s *Server) handleDocs(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    w.WriteHeader(http.StatusOK)
    _, _ = w.Write([]byte(docsHTML))
}
```

Add `docsHTML` constant to `server.go`:

```go
// docsHTML is the Stoplight Elements API documentation page.
// Elements is loaded via CDN — no build step required.
const docsHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Plomvix API Docs</title>
  <script src="https://unpkg.com/@stoplight/elements/web-components.min.js"></script>
  <link rel="stylesheet" href="https://unpkg.com/@stoplight/elements/styles.min.css">
  <style>
    html, body { margin: 0; padding: 0; height: 100%; }
    elements-api { display: block; height: 100%; }
  </style>
</head>
<body>
  <elements-api
    apiDescriptionUrl="/openapi.json"
    router="hash"
    layout="sidebar"
  />
</body>
</html>`
```

**Verify:**
```bash
CGO_ENABLED=1 go build ./internal/server/
CGO_ENABLED=1 go build ./cmd/plomvix/
```

---

## TASK 06 — Create internal/admin/handler_test.go

**Action:** Create `internal/admin/handler_test.go`.

**Package declaration:** `package admin`

**Imports:**
```go
import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "path/filepath"
    "testing"
    "time"

    "github.com/go-chi/chi/v5"

    "github.com/plomvix/plomvix/internal/config"
    "github.com/plomvix/plomvix/internal/storage/cold"
    hotstore "github.com/plomvix/plomvix/internal/storage/hot"
    walstore "github.com/plomvix/plomvix/internal/storage/wal"
)
```

**Test helper:**
```go
func newTestHandler(t *testing.T) *Handler {
    t.Helper()
    dir := t.TempDir()

    walDir := filepath.Join(dir, "wal")
    walCfg := &config.Config{Storage: config.StorageConfig{
        DataDir: dir, WALFlushThreshold: 64 * 1024 * 1024,
    }}
    wal, err := walstore.Open(walDir, walCfg)
    if err != nil {
        t.Fatalf("wal.Open failed: %v", err)
    }
    t.Cleanup(func() { _ = wal.Close() })

    hotDir := filepath.Join(dir, "hot")
    hotCfg := &config.Config{Storage: config.StorageConfig{DataDir: hotDir}}
    hot, err := hotstore.Open(hotDir, hotCfg)
    if err != nil {
        t.Fatalf("hot.Open failed: %v", err)
    }
    t.Cleanup(func() { hot.Close() })

    coldDir := filepath.Join(dir, "cold")
    cs, err := cold.NewStore(coldDir)
    if err != nil {
        t.Fatalf("cold.NewStore failed: %v", err)
    }

    return NewHandler(wal, hot, cs, "test", "2024-01-01", "abc1234", time.Now())
}

func getJSON(t *testing.T, handler http.HandlerFunc, path string) map[string]interface{} {
    t.Helper()
    req := httptest.NewRequest(http.MethodGet, path, nil)
    w := httptest.NewRecorder()
    handler(w, req)
    if w.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
    }
    var resp map[string]interface{}
    json.NewDecoder(w.Body).Decode(&resp)
    return resp
}
```

**Tests:**
```go
func TestStats(t *testing.T) {
    h := newTestHandler(t)
    resp := getJSON(t, h.Stats, "/admin/stats")
    data := resp["data"].(map[string]interface{})
    if _, ok := data["wal"]; !ok {
        t.Error("stats response missing 'wal' key")
    }
    if _, ok := data["hot"]; !ok {
        t.Error("stats response missing 'hot' key")
    }
    if _, ok := data["cold"]; !ok {
        t.Error("stats response missing 'cold' key")
    }
    if _, ok := data["runtime"]; !ok {
        t.Error("stats response missing 'runtime' key")
    }
}

func TestInfo(t *testing.T) {
    h := newTestHandler(t)
    resp := getJSON(t, h.Info, "/admin/info")
    data := resp["data"].(map[string]interface{})
    if data["version"] != "test" {
        t.Errorf("version = %v, want test", data["version"])
    }
    if data["git_commit"] != "abc1234" {
        t.Errorf("git_commit = %v, want abc1234", data["git_commit"])
    }
    if _, ok := data["uptime_seconds"]; !ok {
        t.Error("info response missing 'uptime_seconds'")
    }
}

func TestWALStats(t *testing.T) {
    h := newTestHandler(t)
    resp := getJSON(t, h.WALStats, "/admin/wal/stats")
    data := resp["data"].(map[string]interface{})
    if _, ok := data["segment_count"]; !ok {
        t.Error("wal stats missing 'segment_count'")
    }
}

func TestWALRotate(t *testing.T) {
    h := newTestHandler(t)
    req := httptest.NewRequest(http.MethodPost, "/admin/wal/rotate", nil)
    w := httptest.NewRecorder()
    h.WALRotate(w, req)
    if w.Code != http.StatusOK {
        t.Errorf("WALRotate status = %d, want 200", w.Code)
    }
}

func TestColdStats(t *testing.T) {
    h := newTestHandler(t)
    resp := getJSON(t, h.ColdStats, "/admin/cold/stats")
    data := resp["data"].(map[string]interface{})
    if _, ok := data["parquet_files"]; !ok {
        t.Error("cold stats missing 'parquet_files'")
    }
}

func TestSchemaList(t *testing.T) {
    h := newTestHandler(t)
    resp := getJSON(t, h.SchemaList, "/admin/schema")
    data := resp["data"].(map[string]interface{})
    // All four types should be present even if empty
    for _, dt := range []string{"logs", "metrics", "json", "kv"} {
        if _, ok := data[dt]; !ok {
            t.Errorf("schema list missing data type %q", dt)
        }
    }
}

func TestSchemaDelete(t *testing.T) {
    h := newTestHandler(t)

    // Use chi router to handle URL param
    r := chi.NewRouter()
    r.Delete("/admin/schema/{type}", h.SchemaDelete)

    req := httptest.NewRequest(http.MethodDelete, "/admin/schema/logs", nil)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    if w.Code != http.StatusOK {
        t.Errorf("SchemaDelete status = %d, want 200: %s", w.Code, w.Body.String())
    }
}

func TestSchemaDeleteUnknownType(t *testing.T) {
    h := newTestHandler(t)

    r := chi.NewRouter()
    r.Delete("/admin/schema/{type}", h.SchemaDelete)

    req := httptest.NewRequest(http.MethodDelete, "/admin/schema/unknown", nil)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    if w.Code != http.StatusBadRequest {
        t.Errorf("SchemaDelete unknown type status = %d, want 400", w.Code)
    }
}
```

**Verify:** `CGO_ENABLED=1 go test -race ./internal/admin/` — all tests pass.

**Notes:**
- These tests open RocksDB through the hot tier, so CGO and RocksDB system libraries
  from Sprint 4 must be available.
- The success path must not require `logger.Init()`; only error paths should log.

---

## TASK 07 — Validate openapi.json completeness

**Action:** Run the following validation checks:

```bash
# 1. Valid JSON
cat api/openapi.json | python3 -m json.tool > /dev/null \
    && echo "PASS: valid JSON" \
    || { echo "FAIL: invalid JSON"; exit 1; }

# 2. No placeholders or literal ellipsis
! grep -R "\.\.\.\|TODO\|PLACEHOLDER" api/openapi.json \
    && echo "PASS: no placeholders" \
    || { echo "FAIL: placeholders remain in openapi.json"; exit 1; }

# 3. Count paths — Sprint 9 should document at least 26 concrete paths
PATH_COUNT=$(cat api/openapi.json | python3 -c "
import sys, json
spec = json.load(sys.stdin)
print(len(spec.get('paths', {})))
")
[ "$PATH_COUNT" -ge 26 ] \
    && echo "PASS: $PATH_COUNT paths in spec" \
    || { echo "FAIL: only $PATH_COUNT paths, expected >= 26"; exit 1; }

# 4. All required top-level fields present
python3 -c "
import sys, json
spec = json.load(open('api/openapi.json'))
required = ['openapi', 'info', 'paths', 'components', 'tags']
missing = [f for f in required if f not in spec]
if missing:
    print('FAIL: missing fields:', missing)
    sys.exit(1)
print('PASS: all required top-level fields present')
"

# 5. Security schemes defined
python3 -c "
import sys, json
spec = json.load(open('api/openapi.json'))
schemes = spec.get('components', {}).get('securitySchemes', {})
if 'BearerAuth' not in schemes or 'APIKeyAuth' not in schemes:
    print('FAIL: missing security schemes')
    sys.exit(1)
print('PASS: security schemes present')
"

# 6. Required paths exist exactly
python3 -c "
import sys, json
spec = json.load(open('api/openapi.json'))
paths = set(spec.get('paths', {}).keys())
required = {
  '/health', '/auth/login', '/auth/logout', '/auth/refresh',
  '/admin/users', '/admin/users/{id}', '/admin/users/{id}/apikey',
  '/admin/users/{id}/apikey/status',
  '/ingest/logs', '/ingest/metrics', '/ingest/json', '/ingest/kv',
  '/query/logs', '/query/metrics', '/query/json', '/query/kv/{key}',
  '/query/schema/{type}', '/admin/tier/flush',
  '/admin/stats', '/admin/info', '/admin/wal/stats', '/admin/wal/rotate',
  '/admin/cold/stats', '/admin/schema', '/admin/schema/{type}',
  '/openapi.json', '/docs',
}
missing = sorted(required - paths)
if missing:
    print('FAIL: missing required paths:', missing)
    sys.exit(1)
print('PASS: all required paths present')
"

# 7. Public endpoints have explicit empty security
python3 -c "
import json, sys
spec = json.load(open('api/openapi.json'))
public = {('/health','get'),('/auth/login','post'),('/openapi.json','get'),('/docs','get')}
for path, method in public:
    op = spec['paths'][path][method]
    if op.get('security') != []:
        print(f'FAIL: {method.upper()} {path} must have security: []')
        sys.exit(1)
print('PASS: public endpoint security is explicit')
"
```

**Verify:** All seven checks pass.

---

## TASK 08 — Full build and smoke test

**Action:**

```bash
#!/bin/bash
set -euo pipefail

echo "=== Clearing stale data ==="
rm -rf data/hot/ data/wal/ data/cold/
rm -f data/system/auth.db

SERVER_PID=""
cleanup() {
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        kill -SIGTERM "$SERVER_PID" 2>/dev/null || true
        wait "$SERVER_PID" 2>/dev/null || true
    fi
}
trap cleanup EXIT

echo "=== Step 1: Build ==="
CGO_ENABLED=1 make vet
CGO_ENABLED=1 make build

echo ""
echo "=== Step 2: Run all tests ==="
CGO_ENABLED=1 make test

echo ""
echo "=== Step 3: Boot server ==="
./plomvix > /tmp/plomvix_s9.log 2>&1 &
SERVER_PID=$!
sleep 3

echo ""
echo "=== Step 4: OpenAPI spec is publicly accessible ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/openapi.json)
[ "$STATUS" -eq 200 ] && echo "PASS: /openapi.json → 200" \
    || { echo "FAIL: /openapi.json → $STATUS"; exit 1; }

echo ""
echo "=== Step 5: OpenAPI spec is valid JSON ==="
curl -sf http://localhost:8080/openapi.json | python3 -m json.tool > /dev/null \
    && echo "PASS: spec is valid JSON" \
    || { echo "FAIL: spec is not valid JSON"; exit 1; }

echo ""
echo "=== Step 6: Docs UI is publicly accessible ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/docs)
[ "$STATUS" -eq 200 ] && echo "PASS: /docs → 200" \
    || { echo "FAIL: /docs → $STATUS"; exit 1; }

echo ""
echo "=== Step 7: Docs page contains Elements web component ==="
curl -sf http://localhost:8080/docs | grep -q "elements-api" \
    && echo "PASS: docs page contains <elements-api>" \
    || { echo "FAIL: docs page missing <elements-api>"; exit 1; }

echo ""
echo "=== Step 8: Login ==="
TOKEN=$(curl -sf -X POST http://localhost:8080/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"changeme"}' | jq -r '.data.token')
echo "Token acquired"

echo ""
echo "=== Step 9: GET /admin/stats ==="
RESP=$(curl -sf http://localhost:8080/admin/stats \
    -H "Authorization: Bearer $TOKEN")
echo "$RESP" | jq '.data.wal' | grep -q "segment_count" \
    && echo "PASS: /admin/stats returns wal stats" \
    || { echo "FAIL: /admin/stats missing wal block"; exit 1; }

echo ""
echo "=== Step 10: GET /admin/info ==="
RESP=$(curl -sf http://localhost:8080/admin/info \
    -H "Authorization: Bearer $TOKEN")
echo "$RESP" | jq '.data.version' | grep -qv "null" \
    && echo "PASS: /admin/info returns version" \
    || { echo "FAIL: /admin/info missing version"; exit 1; }

echo ""
echo "=== Step 11: POST /admin/wal/rotate ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/admin/wal/rotate \
    -H "Authorization: Bearer $TOKEN")
[ "$STATUS" -eq 200 ] && echo "PASS: /admin/wal/rotate → 200" \
    || { echo "FAIL: /admin/wal/rotate → $STATUS"; exit 1; }

echo ""
echo "=== Step 12: GET /admin/schema ==="
RESP=$(curl -sf http://localhost:8080/admin/schema \
    -H "Authorization: Bearer $TOKEN")
echo "$RESP" | jq '.data' | grep -q "logs" \
    && echo "PASS: /admin/schema lists data types" \
    || { echo "FAIL: /admin/schema missing 'logs'"; exit 1; }

echo ""
echo "=== Step 13: Admin endpoints require auth ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/admin/stats)
[ "$STATUS" -eq 401 ] && echo "PASS: no auth → 401" \
    || { echo "FAIL: expected 401, got $STATUS"; exit 1; }

echo ""
echo "=== Step 14: Graceful shutdown ==="
kill -SIGTERM "$SERVER_PID"
wait "$SERVER_PID"
EXIT_CODE=$?
SERVER_PID=""
[ "$EXIT_CODE" -eq 0 ] && echo "PASS: clean shutdown" \
    || { echo "FAIL: exit code $EXIT_CODE"; exit 1; }

echo ""
echo "================================================"
echo "  ALL STEPS PASSED — Sprint 9 smoke test DONE  "
echo "================================================"
```

| Step | Verified | Expected |
|---|---|---|
| 1 | Build + vet | No errors |
| 2 | All tests | Pass with race detector |
| 3 | Boot | Server starts |
| 4 | `/openapi.json` public | 200 without auth |
| 5 | Spec is valid JSON | Parseable |
| 6 | `/docs` public | 200 without auth |
| 7 | Docs has Elements | `<elements-api>` present |
| 8 | Login | JWT returned |
| 9 | `/admin/stats` | Returns wal/hot/cold/runtime blocks |
| 10 | `/admin/info` | Returns version |
| 11 | `/admin/wal/rotate` | 200 |
| 12 | `/admin/schema` | Lists all data types |
| 13 | Auth check | 401 without token |
| 14 | Graceful shutdown | Exit code 0 |

---

## EXECUTION ORDER SUMMARY

```
TASK 01  →  api/openapi.json (full OpenAPI 3.0 spec for all endpoints)
TASK 02  →  internal/admin/handler.go (Stats, Info, WALStats, WALRotate, ColdStats, SchemaList, SchemaDelete)
TASK 03  →  internal/storage/wal/manager.go (add Rotate())
TASK 04  →  internal/ingestion/schema.go + internal/storage/hot/manager.go (add DeleteSchema, DeleteMeta)
            ← verify internal/admin/ compiles after this
TASK 05  →  internal/server/server.go (admin routes, docs routes, New() signature update)
            ← also update cmd/plomvix/main.go server.New() call with BuildTime + GitCommit
TASK 06  →  internal/admin/handler_test.go
TASK 07  →  openapi.json validation checks
TASK 08  →  smoke test — all 14 steps must pass
```

---

*Sprint 9 complete when TASK 08 passes with zero failures.*
*Plomvix — Built in India. Built for the world.*