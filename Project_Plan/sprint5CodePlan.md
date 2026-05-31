# Plomvix — Sprint 5 Task Plan
### For: DeepSeek V4 Pro Coding Agent
### Language: Go 1.22 | Module: github.com/plomvix/plomvix

> Execute tasks in exact order. Each task is atomic — one file or one concern.
> Do not skip ahead. Each task depends on the previous.
> Every task has a Verify step — do not proceed until it passes.

---

## CONTEXT

Sprints 1–4 are complete. Sprint 5 adds the **Ingestion API** and **Schema Inference Engine**.
Data can now flow into Plomvix over HTTP, be durably written to the WAL, hydrated into
RocksDB, and have its schema automatically inferred and registered.

**What Sprint 5 delivers:**
- `POST /ingest/logs` — ingest one or many log records
- `POST /ingest/metrics` — ingest one or many metric records
- `POST /ingest/json` — ingest one or many JSON documents
- `POST /ingest/kv` — ingest one or many key-value records
- Schema inference engine — auto-detects field types from JSON payloads on first write
- Schema registry — stores inferred schemas per data type in RocksDB `_meta` CF
- Batch ingestion — all endpoints accept a single record OR an array of records
- All ingestion endpoints require authentication (JWT or API key)
- Ingestion stats exposed in `GET /health`
- Full test coverage
- API documentation in `docs/api/ingest.md`

**What Sprint 5 does NOT do:**
- No query endpoints — that is Sprint 6
- No cold tier tiering — that is Sprint 7
- Schema inference is best-effort — it does not block ingestion on failure

**Write path (for all ingest endpoints):**
```
HTTP request → validate payload → WAL write → hot tier write → update schema → respond 201
```

---

## SCHEMA INFERENCE DESIGN — READ BEFORE WRITING ANY CODE

### What schema inference does

For each ingested JSON record, Plomvix automatically detects the types of each field
and stores a schema. This enables Sprint 6's query engine to know field types without
requiring the user to define a schema upfront.

### Inferred types

| JSON value | Inferred type |
|---|---|
| `true` / `false` | `bool` |
| integer number (no decimal) | `int64` |
| decimal number | `float64` |
| string | `string` |
| `null` | `null` |
| nested object | `object` |
| array | `array` |

### Schema storage

Schemas are stored in a new `_meta` column family in RocksDB.

**Key format:** `schema:{data_type}` — e.g. `schema:logs`, `schema:metrics`
**Value format:** JSON-encoded `Schema` struct

**Schema merge rule:** when a new record arrives with a field not yet in the schema,
add that field. When a field exists but the new record has a different type for it,
mark that field as `mixed`. Never remove a field once added.

### Schema struct:
```go
type FieldType string

const (
    FieldTypeBool    FieldType = "bool"
    FieldTypeInt64   FieldType = "int64"
    FieldTypeFloat64 FieldType = "float64"
    FieldTypeString  FieldType = "string"
    FieldTypeNull    FieldType = "null"
    FieldTypeObject  FieldType = "object"
    FieldTypeArray   FieldType = "array"
    FieldTypeMixed   FieldType = "mixed"  // field seen with multiple types
)

type Schema struct {
    DataType   string               `json:"data_type"`   // "logs", "metrics", "json", "kv"
    Fields     map[string]FieldType `json:"fields"`
    UpdatedAt  time.Time            `json:"updated_at"`
    RecordCount int64               `json:"record_count"` // total records ingested
}
```

---

## `_meta` COLUMN FAMILY — READ BEFORE WRITING ANY CODE

Sprint 5 adds a `_meta` column family to RocksDB for schema storage.

**Required change to `internal/storage/hot/types.go`:**

Add `CFMeta = "_meta"` constant and include it in `AllColumnFamilies()`.

```go
const (
    CFLogs    = "logs"
    CFMetrics = "metrics"
    CFJSON    = "json"
    CFKV      = "kv"
    CFMeta    = "_meta"   // ← ADD: schema registry and system metadata
)

func AllColumnFamilies() []string {
    return []string{"default", CFLogs, CFMetrics, CFJSON, CFKV, CFMeta}  // ← ADD CFMeta
}
```

**WARNING:** Existing RocksDB databases from Sprint 4 testing do NOT have the `_meta` CF.
Opening them with the updated `AllColumnFamilies()` will fail.
**Delete the `data/hot/` directory before running Sprint 5 for the first time:**
```bash
rm -rf data/hot/
```
This is expected and safe — Sprint 4 was test data only.

---

## TASK 01 — Update internal/storage/hot/types.go

**Action:** Add `CFMeta` to `internal/storage/hot/types.go`.

**Change 1 — Add constant:**
```go
const (
    CFLogs    = "logs"
    CFMetrics = "metrics"
    CFJSON    = "json"
    CFKV      = "kv"
    CFMeta    = "_meta"  // ← ADD
)
```

**Change 2 — Add to AllColumnFamilies:**
```go
func AllColumnFamilies() []string {
    return []string{"default", CFLogs, CFMetrics, CFJSON, CFKV, CFMeta}  // ← ADD CFMeta
}
```

**Verify:** `CGO_ENABLED=1 go build ./internal/storage/hot/` compiles with no errors.

---

## TASK 02 — Create internal/ingestion/ directory and types.go

**Action — Part A:** Create the ingestion package directory:
```bash
mkdir -p internal/ingestion
```

**Action — Part B:** Create `internal/ingestion/types.go`.

**Full file content:**
```go
package ingestion

import "time"

// FieldType represents the inferred type of a JSON field.
type FieldType string

const (
    FieldTypeBool    FieldType = "bool"
    FieldTypeInt64   FieldType = "int64"
    FieldTypeFloat64 FieldType = "float64"
    FieldTypeString  FieldType = "string"
    FieldTypeNull    FieldType = "null"
    FieldTypeObject  FieldType = "object"
    FieldTypeArray   FieldType = "array"
    FieldTypeMixed   FieldType = "mixed" // field seen with conflicting types
)

// Schema represents the inferred schema for a data type.
type Schema struct {
    DataType    string               `json:"data_type"`
    Fields      map[string]FieldType `json:"fields"`
    UpdatedAt   time.Time            `json:"updated_at"`
    RecordCount int64                `json:"record_count"`
}

// LogRecord is the expected shape of an ingested log entry.
type LogRecord struct {
    Timestamp int64             `json:"timestamp"` // Unix nanoseconds; if 0, server sets it
    Level     string            `json:"level"`
    Message   string            `json:"message"`
    Fields    map[string]interface{} `json:"fields,omitempty"`
}

// MetricRecord is the expected shape of an ingested metric.
type MetricRecord struct {
    Timestamp  int64             `json:"timestamp"` // Unix nanoseconds; if 0, server sets it
    Name       string            `json:"name"`      // metric name, required
    Value      float64           `json:"value"`
    Tags       map[string]string `json:"tags,omitempty"`
}

// JSONRecord is a free-form JSON document.
// The entire record is stored as-is; schema is inferred from top-level fields.
type JSONRecord struct {
    Timestamp int64                  `json:"timestamp"` // Unix nanoseconds; if 0, server sets it
    Data      map[string]interface{} `json:"data"`
}

// KVRecord is a key-value pair.
type KVRecord struct {
    Key   string `json:"key"`   // required, non-empty
    Value string `json:"value"` // stored as raw string
}

// IngestRequest wraps a single record or batch for all ingest endpoints.
// Either Records (batch) or a single record field is populated.
type IngestRequest[T any] struct {
    Records []T `json:"records"` // batch — 1 or more records
}

// IngestResponse is returned on successful ingestion.
type IngestResponse struct {
    Ingested  int    `json:"ingested"`   // number of records accepted
    RequestID string `json:"request_id"`
}
```

**Verify:** `go build ./internal/ingestion/` compiles with no errors.

---

## TASK 03 — Create internal/ingestion/schema.go

**Action:** Create `internal/ingestion/schema.go` — schema inference and registry.

**Imports required:**
```go
import (
    "encoding/json"
    "fmt"
    "time"

    "github.com/plomvix/plomvix/internal/storage/hot"
)
```

**Full file content:**
```go
package ingestion

import (
    "encoding/json"
    "fmt"
    "time"

    "github.com/plomvix/plomvix/internal/storage/hot"
)

// schemaKey returns the RocksDB key for a data type's schema.
func schemaKey(dataType string) []byte {
    return []byte("schema:" + dataType)
}

// InferFieldType returns the FieldType for a single JSON value.
func InferFieldType(v interface{}) FieldType {
    if v == nil {
        return FieldTypeNull
    }
    switch v.(type) {
    case bool:
        return FieldTypeBool
    case float64:
        // JSON numbers decode as float64; check if it is a whole number
        f := v.(float64)
        if f == float64(int64(f)) {
            return FieldTypeInt64
        }
        return FieldTypeFloat64
    case string:
        return FieldTypeString
    case map[string]interface{}:
        return FieldTypeObject
    case []interface{}:
        return FieldTypeArray
    default:
        return FieldTypeString
    }
}

// InferSchema inspects a flat JSON object and returns a map of field → FieldType.
// Only top-level fields are inspected.
func InferSchema(record map[string]interface{}) map[string]FieldType {
    fields := make(map[string]FieldType, len(record))
    for k, v := range record {
        fields[k] = InferFieldType(v)
    }
    return fields
}

// LoadSchema loads the current schema for a data type from RocksDB.
// Returns a new empty Schema if none exists yet.
func LoadSchema(store *hot.Manager, dataType string) (*Schema, error) {
    raw, err := store.GetMeta(schemaKey(dataType))
    if err != nil {
        return nil, fmt.Errorf("failed to load schema for %q: %w", dataType, err)
    }
    if raw == nil {
        return &Schema{
            DataType: dataType,
            Fields:   make(map[string]FieldType),
        }, nil
    }
    var s Schema
    if err := json.Unmarshal(raw, &s); err != nil {
        return nil, fmt.Errorf("failed to unmarshal schema for %q: %w", dataType, err)
    }
    return &s, nil
}

// SaveSchema persists a schema to RocksDB.
func SaveSchema(store *hot.Manager, s *Schema) error {
    raw, err := json.Marshal(s)
    if err != nil {
        return fmt.Errorf("failed to marshal schema for %q: %w", s.DataType, err)
    }
    return store.PutMeta(schemaKey(s.DataType), raw)
}

// MergeSchema merges newly inferred fields into an existing schema.
// New fields are added. Existing fields with a different type become FieldTypeMixed.
// RecordCount is incremented by delta.
func MergeSchema(s *Schema, newFields map[string]FieldType, delta int64) {
    for field, newType := range newFields {
        if currentType, ok := s.Fields[field]; ok {
            if currentType != newType {
                s.Fields[field] = FieldTypeMixed
            }
            // if same type, no change needed
        } else {
            s.Fields[field] = newType
        }
    }
    s.RecordCount += delta
    s.UpdatedAt = time.Now()
}

// UpdateSchema loads, merges, and saves the schema for a data type in one call.
// Errors are non-fatal — schema update failure does not block ingestion.
func UpdateSchema(store *hot.Manager, dataType string, records []map[string]interface{}) error {
    s, err := LoadSchema(store, dataType)
    if err != nil {
        return err
    }
    for _, record := range records {
        inferred := InferSchema(record)
        MergeSchema(s, inferred, 1)
    }
    return SaveSchema(store, s)
}
```

**Verify:** `go build ./internal/ingestion/` compiles with no errors.

---

## TASK 04 — Add GetMeta and PutMeta to internal/storage/hot/manager.go

**Action:** Add two methods to `internal/storage/hot/manager.go`.

```go
// GetMeta retrieves a value from the _meta column family by key.
// Returns nil, nil if the key does not exist.
func (m *Manager) GetMeta(key []byte) ([]byte, error) {
    return m.store.Get(CFMeta, key)
}

// PutMeta writes a key-value pair to the _meta column family.
func (m *Manager) PutMeta(key, value []byte) error {
    return m.store.Put(CFMeta, key, value)
}
```

**Verify:** `CGO_ENABLED=1 go build ./internal/storage/hot/` compiles with no errors.

---

## TASK 05 — Create internal/ingestion/handler.go

**Action:** Create `internal/ingestion/handler.go`.

**Imports required:**
```go
import (
    "encoding/json"
    "net/http"
    "time"

    "go.uber.org/zap"

    "github.com/plomvix/plomvix/internal/logger"
    "github.com/plomvix/plomvix/internal/storage/hot"
    walstore "github.com/plomvix/plomvix/internal/storage/wal"
    "github.com/plomvix/plomvix/pkg/utils"
)
```

**Handler struct:**
```go
// Handler handles all ingestion HTTP endpoints.
type Handler struct {
    hot *hot.Manager
    wal *walstore.Manager
}

// NewHandler creates a new ingestion Handler.
func NewHandler(h *hot.Manager, w *walstore.Manager) *Handler {
    return &Handler{hot: h, wal: w}
}
```

---

**`POST /ingest/logs`**

- **Auth required:** Yes — JWT or API key
- **Content-Type:** `application/json`

Request body — single record:
```json
{
  "records": [
    {
      "timestamp": 1700000000000000000,
      "level": "info",
      "message": "user logged in",
      "fields": {"user_id": "abc123", "ip": "1.2.3.4"}
    }
  ]
}
```

`timestamp` is Unix nanoseconds. If `0` or omitted, server sets `time.Now().UnixNano()`.

Success response — HTTP 201:
```json
{
  "status": "ok",
  "data": {
    "ingested": 1,
    "request_id": "uuid"
  },
  "request_id": "uuid"
}
```

Error — HTTP 400 if records array is empty or missing:
```json
{
  "status": "error",
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "records array is required and must not be empty"
  },
  "request_id": "uuid"
}
```

**Handler logic — `IngestLogs`:**
```go
// IngestLogs handles POST /ingest/logs.
//
// POST /ingest/logs
// Auth: JWT or API key
//
// Request body: {"records": [{...}]}
//
// Responses:
//   201 Created      — ingested N records
//   400 Bad Request  — VALIDATION_FAILED: missing or empty records
//   500 Internal     — INTERNAL_ERROR: WAL or hot tier write failed
func (h *Handler) IngestLogs(w http.ResponseWriter, r *http.Request) {
    var req IngestRequest[LogRecord]
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.BadRequest(w, r, utils.CodeValidationFailed, "invalid request body")
        return
    }
    if len(req.Records) == 0 {
        utils.BadRequest(w, r, utils.CodeValidationFailed,
            "records array is required and must not be empty")
        return
    }

    count := 0
    var rawRecords []map[string]interface{}

    for _, rec := range req.Records {
        if rec.Timestamp == 0 {
            rec.Timestamp = time.Now().UnixNano()
        }

        payload, err := json.Marshal(rec)
        if err != nil {
            utils.InternalError(w, r, "failed to serialize record")
            return
        }

        // Write to WAL first — durability guarantee
        if _, err := h.wal.Write(walstore.DataTypeLog, payload); err != nil {
            logger.Error("WAL write failed", zap.Error(err))
            utils.InternalError(w, r, "failed to write to WAL")
            return
        }

        // Write to hot tier
        if err := h.hot.WriteLog(rec.Timestamp, payload); err != nil {
            logger.Error("hot tier write failed", zap.Error(err))
            utils.InternalError(w, r, "failed to write to hot tier")
            return
        }

        count++

        // Collect for schema inference
        var raw map[string]interface{}
        if err := json.Unmarshal(payload, &raw); err == nil {
            rawRecords = append(rawRecords, raw)
        }
    }

    // Update schema — non-fatal if it fails
    if err := UpdateSchema(h.hot, "logs", rawRecords); err != nil {
        logger.Warn("schema update failed", zap.String("data_type", "logs"), zap.Error(err))
    }

    utils.Created(w, r, IngestResponse{
        Ingested:  count,
        RequestID: r.Header.Get("X-Request-ID"),
    })
}
```

---

**`POST /ingest/metrics`**

Same pattern as logs. Handler method: `IngestMetrics`.

**Validation rules:**
- `records` must not be empty
- Each record must have non-empty `name` field → return 400 if missing

```go
// IngestMetrics handles POST /ingest/metrics.
//
// POST /ingest/metrics
// Auth: JWT or API key
//
// Request body: {"records": [{...}]}
//
// Responses:
//   201 Created     — ingested N records
//   400 Bad Request — VALIDATION_FAILED
//   500 Internal    — INTERNAL_ERROR
func (h *Handler) IngestMetrics(w http.ResponseWriter, r *http.Request) {
    var req IngestRequest[MetricRecord]
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.BadRequest(w, r, utils.CodeValidationFailed, "invalid request body")
        return
    }
    if len(req.Records) == 0 {
        utils.BadRequest(w, r, utils.CodeValidationFailed,
            "records array is required and must not be empty")
        return
    }

    count := 0
    var rawRecords []map[string]interface{}

    for _, rec := range req.Records {
        if rec.Name == "" {
            utils.BadRequest(w, r, utils.CodeValidationFailed,
                "each metric record must have a non-empty name field")
            return
        }
        if rec.Timestamp == 0 {
            rec.Timestamp = time.Now().UnixNano()
        }

        payload, err := json.Marshal(rec)
        if err != nil {
            utils.InternalError(w, r, "failed to serialize record")
            return
        }

        if _, err := h.wal.Write(walstore.DataTypeMetric, payload); err != nil {
            logger.Error("WAL write failed", zap.Error(err))
            utils.InternalError(w, r, "failed to write to WAL")
            return
        }

        if err := h.hot.WriteMetric(rec.Timestamp, rec.Name, payload); err != nil {
            logger.Error("hot tier write failed", zap.Error(err))
            utils.InternalError(w, r, "failed to write to hot tier")
            return
        }

        count++

        var raw map[string]interface{}
        if err := json.Unmarshal(payload, &raw); err == nil {
            rawRecords = append(rawRecords, raw)
        }
    }

    if err := UpdateSchema(h.hot, "metrics", rawRecords); err != nil {
        logger.Warn("schema update failed", zap.String("data_type", "metrics"), zap.Error(err))
    }

    utils.Created(w, r, IngestResponse{
        Ingested:  count,
        RequestID: r.Header.Get("X-Request-ID"),
    })
}
```

---

**`POST /ingest/json`**

Same pattern. Handler method: `IngestJSON`.

**Validation:** `records` not empty, each record must have non-nil `data` field.

```go
// IngestJSON handles POST /ingest/json.
//
// POST /ingest/json
// Auth: JWT or API key
//
// Responses:
//   201 Created     — ingested N records
//   400 Bad Request — VALIDATION_FAILED
//   500 Internal    — INTERNAL_ERROR
func (h *Handler) IngestJSON(w http.ResponseWriter, r *http.Request) {
    var req IngestRequest[JSONRecord]
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.BadRequest(w, r, utils.CodeValidationFailed, "invalid request body")
        return
    }
    if len(req.Records) == 0 {
        utils.BadRequest(w, r, utils.CodeValidationFailed,
            "records array is required and must not be empty")
        return
    }

    count := 0
    var rawRecords []map[string]interface{}

    for _, rec := range req.Records {
        if rec.Data == nil {
            utils.BadRequest(w, r, utils.CodeValidationFailed,
                "each json record must have a non-null data field")
            return
        }
        if rec.Timestamp == 0 {
            rec.Timestamp = time.Now().UnixNano()
        }

        payload, err := json.Marshal(rec)
        if err != nil {
            utils.InternalError(w, r, "failed to serialize record")
            return
        }

        if _, err := h.wal.Write(walstore.DataTypeJSON, payload); err != nil {
            logger.Error("WAL write failed", zap.Error(err))
            utils.InternalError(w, r, "failed to write to WAL")
            return
        }

        if err := h.hot.WriteJSON(rec.Timestamp, payload); err != nil {
            logger.Error("hot tier write failed", zap.Error(err))
            utils.InternalError(w, r, "failed to write to hot tier")
            return
        }

        count++
        rawRecords = append(rawRecords, rec.Data)
    }

    if err := UpdateSchema(h.hot, "json", rawRecords); err != nil {
        logger.Warn("schema update failed", zap.String("data_type", "json"), zap.Error(err))
    }

    utils.Created(w, r, IngestResponse{
        Ingested:  count,
        RequestID: r.Header.Get("X-Request-ID"),
    })
}
```

---

**`POST /ingest/kv`**

Same pattern. Handler method: `IngestKV`.

**Validation:** `records` not empty, each record must have non-empty `key` field.

```go
// IngestKV handles POST /ingest/kv.
//
// POST /ingest/kv
// Auth: JWT or API key
//
// Responses:
//   201 Created     — ingested N records
//   400 Bad Request — VALIDATION_FAILED
//   500 Internal    — INTERNAL_ERROR
func (h *Handler) IngestKV(w http.ResponseWriter, r *http.Request) {
    var req IngestRequest[KVRecord]
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.BadRequest(w, r, utils.CodeValidationFailed, "invalid request body")
        return
    }
    if len(req.Records) == 0 {
        utils.BadRequest(w, r, utils.CodeValidationFailed,
            "records array is required and must not be empty")
        return
    }

    count := 0

    for _, rec := range req.Records {
        if rec.Key == "" {
            utils.BadRequest(w, r, utils.CodeValidationFailed,
                "each kv record must have a non-empty key field")
            return
        }

        payload, err := json.Marshal(rec)
        if err != nil {
            utils.InternalError(w, r, "failed to serialize record")
            return
        }

        if _, err := h.wal.Write(walstore.DataTypeKV, payload); err != nil {
            logger.Error("WAL write failed", zap.Error(err))
            utils.InternalError(w, r, "failed to write to WAL")
            return
        }

        if err := h.hot.WriteKV(rec.Key, payload); err != nil {
            logger.Error("hot tier write failed", zap.Error(err))
            utils.InternalError(w, r, "failed to write to hot tier")
            return
        }

        count++
    }

    // KV has no schema inference — keys are user-defined strings
    utils.Created(w, r, IngestResponse{
        Ingested:  count,
        RequestID: r.Header.Get("X-Request-ID"),
    })
}
```

**Verify:** `go build ./internal/ingestion/` compiles with no errors.

---

## TASK 06 — Add write stats to internal/storage/hot/manager.go

**Action:** Add write counter tracking to `Manager` and expose via `Stats()`.

**NOTE:** `WriteLog`, `WriteMetric`, `WriteJSON`, `WriteKV` are called both during
WAL replay (startup) and HTTP ingestion. The counter tracks ALL writes through
these methods — including WAL replay. This is intentional: `total_data_writes`
in the health response reflects the total records in RocksDB, not just HTTP-ingested ones.

**Change 1 — Add `totalDataWrites` field to Manager:**
```go
type Manager struct {
    store          *Store
    totalDataWrites atomic.Int64  // ← ADD: all writes via Write* methods
}
```

**Change 2 — Increment in Write methods.** Add `m.totalDataWrites.Add(1)` to each of:
- `WriteLog`
- `WriteMetric`
- `WriteJSON`
- `WriteKV`

Example for `WriteLog`:
```go
func (m *Manager) WriteLog(timestampNs int64, payload []byte) error {
    key := BuildTimeSeriesKey(timestampNs)
    if err := m.store.Put(CFLogs, key, payload); err != nil {
        return err
    }
    m.totalDataWrites.Add(1)
    return nil
}
```

Apply same pattern to `WriteMetric`, `WriteJSON`, `WriteKV`.

**Change 3 — Update `HotStats`:**
```go
type HotStats struct {
    TotalWrites     int64  // RocksDB Put operations across all CFs
    TotalDataWrites int64  // ← RENAME from IngestedRecords: all Write* method calls
    DataDir         string
}
```

**Change 4 — Populate in `Stats()`:**
```go
func (m *Manager) Stats() HotStats {
    return HotStats{
        TotalWrites:     m.store.TotalWrites(),
        TotalDataWrites: m.totalDataWrites.Load(),
        DataDir:         m.store.dataDir,
    }
}
```

**Verify:** `CGO_ENABLED=1 go build ./internal/storage/hot/` compiles with no errors.

---

## TASK 07 — Register ingestion routes in internal/server/server.go

**Action:** Two targeted changes to `internal/server/server.go`.

**Change 1 — Add ingestion import:**
```go
"github.com/plomvix/plomvix/internal/ingestion"
```

**Change 2 — Register routes in `setupRoutes()`.**

Add inside the existing protected route group (after auth routes, before admin routes):
```go
// Ingestion — auth required
ingestHandler := ingestion.NewHandler(s.hotTier, s.wal)
r.Group(func(r chi.Router) {
    r.Use(auth.Middleware(s.store, s.blacklist, s.cfg))
    r.Post("/ingest/logs",    ingestHandler.IngestLogs)
    r.Post("/ingest/metrics", ingestHandler.IngestMetrics)
    r.Post("/ingest/json",    ingestHandler.IngestJSON)
    r.Post("/ingest/kv",      ingestHandler.IngestKV)
})
```

**Change 3 — Update `handleHealth` to include `total_data_writes`:**
```go
hotStats := s.hotTier.Stats()
// In the hot block of the health response:
"hot": map[string]interface{}{
    "total_writes":      hotStats.TotalWrites,
    "total_data_writes": hotStats.TotalDataWrites,  // ← RENAME from ingested_records
    "data_dir":          hotStats.DataDir,
},
```

**Verify:** `CGO_ENABLED=1 go build ./internal/server/` compiles with no errors.

---

## TASK 08 — Create internal/ingestion/schema_test.go

**Action:** Create `internal/ingestion/schema_test.go`.

**Package declaration:** `package ingestion`

**Imports:**
```go
import (
    "testing"
    "time"
)
```

**Full file content:**
```go
package ingestion

import (
    "testing"
    "time"
)

func TestInferFieldType(t *testing.T) {
    tests := []struct {
        input    interface{}
        expected FieldType
    }{
        {nil, FieldTypeNull},
        {true, FieldTypeBool},
        {false, FieldTypeBool},
        {float64(42), FieldTypeInt64},    // whole number → int64
        {float64(3.14), FieldTypeFloat64}, // decimal → float64
        {"hello", FieldTypeString},
        {map[string]interface{}{"a": 1}, FieldTypeObject},
        {[]interface{}{1, 2}, FieldTypeArray},
    }
    for _, tt := range tests {
        got := InferFieldType(tt.input)
        if got != tt.expected {
            t.Errorf("InferFieldType(%v) = %q, want %q", tt.input, got, tt.expected)
        }
    }
}

func TestInferSchema(t *testing.T) {
    record := map[string]interface{}{
        "level":   "info",
        "count":   float64(5),
        "ratio":   float64(0.5),
        "active":  true,
        "nothing": nil,
    }
    schema := InferSchema(record)

    if schema["level"] != FieldTypeString {
        t.Errorf("level: got %q, want %q", schema["level"], FieldTypeString)
    }
    if schema["count"] != FieldTypeInt64 {
        t.Errorf("count: got %q, want %q", schema["count"], FieldTypeInt64)
    }
    if schema["ratio"] != FieldTypeFloat64 {
        t.Errorf("ratio: got %q, want %q", schema["ratio"], FieldTypeFloat64)
    }
    if schema["active"] != FieldTypeBool {
        t.Errorf("active: got %q, want %q", schema["active"], FieldTypeBool)
    }
    if schema["nothing"] != FieldTypeNull {
        t.Errorf("nothing: got %q, want %q", schema["nothing"], FieldTypeNull)
    }
}

func TestMergeSchemaNewFields(t *testing.T) {
    s := &Schema{
        DataType:  "logs",
        Fields:    map[string]FieldType{},
        UpdatedAt: time.Now(),
    }
    newFields := map[string]FieldType{
        "level": FieldTypeString,
        "count": FieldTypeInt64,
    }
    MergeSchema(s, newFields, 1)

    if s.Fields["level"] != FieldTypeString {
        t.Errorf("level: got %q, want string", s.Fields["level"])
    }
    if s.Fields["count"] != FieldTypeInt64 {
        t.Errorf("count: got %q, want int64", s.Fields["count"])
    }
    if s.RecordCount != 1 {
        t.Errorf("RecordCount = %d, want 1", s.RecordCount)
    }
}

func TestMergeSchemaConflict(t *testing.T) {
    s := &Schema{
        DataType: "logs",
        Fields:   map[string]FieldType{"level": FieldTypeString},
    }
    // Same field, different type → should become mixed
    MergeSchema(s, map[string]FieldType{"level": FieldTypeInt64}, 1)
    if s.Fields["level"] != FieldTypeMixed {
        t.Errorf("level after conflict: got %q, want mixed", s.Fields["level"])
    }
}

func TestMergeSchemaSameType(t *testing.T) {
    s := &Schema{
        DataType: "logs",
        Fields:   map[string]FieldType{"level": FieldTypeString},
    }
    // Same type twice — should stay string, not become mixed
    MergeSchema(s, map[string]FieldType{"level": FieldTypeString}, 1)
    if s.Fields["level"] != FieldTypeString {
        t.Errorf("level after same type: got %q, want string", s.Fields["level"])
    }
}
```

**Verify:** `go test -race ./internal/ingestion/` — all schema tests pass.

---

## TASK 09 — Create internal/ingestion/handler_test.go

**Action:** Create `internal/ingestion/handler_test.go`.

**Package declaration:** `package ingestion`

**Imports:**
```go
import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "path/filepath"
    "testing"

    "github.com/plomvix/plomvix/internal/config"
    hotstore "github.com/plomvix/plomvix/internal/storage/hot"
    walstore "github.com/plomvix/plomvix/internal/storage/wal"
)
```

**Test helpers:**
```go
func newTestHot(t *testing.T) *hotstore.Manager {
    t.Helper()
    dir := filepath.Join(t.TempDir(), "hot")
    cfg := &config.Config{
        Storage: config.StorageConfig{DataDir: dir},
    }
    m, err := hotstore.Open(dir, cfg)
    if err != nil {
        t.Fatalf("hot.Open failed: %v", err)
    }
    t.Cleanup(func() { m.Close() })
    return m
}

func newTestWAL(t *testing.T) *walstore.Manager {
    t.Helper()
    dir := filepath.Join(t.TempDir(), "wal")
    cfg := &config.Config{
        Storage: config.StorageConfig{
            DataDir:           dir,
            WALFlushThreshold: 64 * 1024 * 1024,
        },
    }
    m, err := walstore.Open(dir, cfg)
    if err != nil {
        t.Fatalf("wal.Open failed: %v", err)
    }
    t.Cleanup(func() { _ = m.Close() })
    return m
}

func newTestHandler(t *testing.T) *Handler {
    t.Helper()
    return NewHandler(newTestHot(t), newTestWAL(t))
}

func postJSON(t *testing.T, handler http.HandlerFunc, body interface{}) *httptest.ResponseRecorder {
    t.Helper()
    b, err := json.Marshal(body)
    if err != nil {
        t.Fatalf("failed to marshal body: %v", err)
    }
    req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    handler(w, req)
    return w
}
```

**Tests to implement:**

```go
func TestIngestLogsSuccess(t *testing.T) {
    h := newTestHandler(t)
    body := map[string]interface{}{
        "records": []map[string]interface{}{
            {"level": "info", "message": "hello", "timestamp": 0},
        },
    }
    w := postJSON(t, h.IngestLogs, body)
    if w.Code != http.StatusCreated {
        t.Errorf("status = %d, want 201", w.Code)
    }
    var resp map[string]interface{}
    json.NewDecoder(w.Body).Decode(&resp)
    data := resp["data"].(map[string]interface{})
    if data["ingested"].(float64) != 1 {
        t.Errorf("ingested = %v, want 1", data["ingested"])
    }
}

func TestIngestLogsEmptyRecords(t *testing.T) {
    h := newTestHandler(t)
    body := map[string]interface{}{"records": []interface{}{}}
    w := postJSON(t, h.IngestLogs, body)
    if w.Code != http.StatusBadRequest {
        t.Errorf("status = %d, want 400", w.Code)
    }
}

func TestIngestMetricsMissingName(t *testing.T) {
    h := newTestHandler(t)
    body := map[string]interface{}{
        "records": []map[string]interface{}{
            {"value": 42.0},  // name is missing
        },
    }
    w := postJSON(t, h.IngestMetrics, body)
    if w.Code != http.StatusBadRequest {
        t.Errorf("status = %d, want 400", w.Code)
    }
}

func TestIngestMetricsSuccess(t *testing.T) {
    h := newTestHandler(t)
    body := map[string]interface{}{
        "records": []map[string]interface{}{
            {"name": "cpu.usage", "value": 87.5},
        },
    }
    w := postJSON(t, h.IngestMetrics, body)
    if w.Code != http.StatusCreated {
        t.Errorf("status = %d, want 201", w.Code)
    }
}

func TestIngestJSONMissingData(t *testing.T) {
    h := newTestHandler(t)
    body := map[string]interface{}{
        "records": []map[string]interface{}{
            {"timestamp": 0},  // data field is nil/missing
        },
    }
    w := postJSON(t, h.IngestJSON, body)
    if w.Code != http.StatusBadRequest {
        t.Errorf("status = %d, want 400", w.Code)
    }
}

func TestIngestJSONSuccess(t *testing.T) {
    h := newTestHandler(t)
    body := map[string]interface{}{
        "records": []map[string]interface{}{
            {"data": map[string]interface{}{"event": "order_placed", "amount": 99.99}},
        },
    }
    w := postJSON(t, h.IngestJSON, body)
    if w.Code != http.StatusCreated {
        t.Errorf("status = %d, want 201", w.Code)
    }
}

func TestIngestKVMissingKey(t *testing.T) {
    h := newTestHandler(t)
    body := map[string]interface{}{
        "records": []map[string]interface{}{
            {"value": "somevalue"},  // key is missing
        },
    }
    w := postJSON(t, h.IngestKV, body)
    if w.Code != http.StatusBadRequest {
        t.Errorf("status = %d, want 400", w.Code)
    }
}

func TestIngestKVSuccess(t *testing.T) {
    h := newTestHandler(t)
    body := map[string]interface{}{
        "records": []map[string]interface{}{
            {"key": "user:alice", "value": "active"},
        },
    }
    w := postJSON(t, h.IngestKV, body)
    if w.Code != http.StatusCreated {
        t.Errorf("status = %d, want 201", w.Code)
    }
}

func TestIngestBatchMultipleRecords(t *testing.T) {
    h := newTestHandler(t)
    body := map[string]interface{}{
        "records": []map[string]interface{}{
            {"level": "info", "message": "first"},
            {"level": "warn", "message": "second"},
            {"level": "error", "message": "third"},
        },
    }
    w := postJSON(t, h.IngestLogs, body)
    if w.Code != http.StatusCreated {
        t.Errorf("status = %d, want 201", w.Code)
    }
    var resp map[string]interface{}
    json.NewDecoder(w.Body).Decode(&resp)
    data := resp["data"].(map[string]interface{})
    if data["ingested"].(float64) != 3 {
        t.Errorf("ingested = %v, want 3", data["ingested"])
    }
}
```

**Verify:** `CGO_ENABLED=1 go test -race ./internal/ingestion/` — all handler tests pass.

---

## TASK 10 — Create docs/api/ingest.md

**Action:** Create `docs/api/ingest.md`.

Write the following **actual content** — not placeholders:

```markdown
# Plomvix Ingest API Reference

All ingest endpoints require authentication.
Use `Authorization: Bearer <token>` or `X-API-Key: <key>`.

All endpoints accept a batch of records via the `records` array.
Minimum 1 record per request.

---

## POST /ingest/logs

Ingest one or more log records.

**Auth:** JWT or API key

**Request body:**
{
  "records": [
    {
      "timestamp": 1700000000000000000,
      "level": "info",
      "message": "user logged in",
      "fields": {"user_id": "abc123"}
    }
  ]
}

Field notes:
- timestamp: Unix nanoseconds. Omit or set 0 — server uses current time.
- level: any string (info, warn, error, debug, etc.)
- message: required, non-empty recommended
- fields: optional arbitrary key-value pairs

**Response 201:**
{ "status": "ok", "data": { "ingested": 1, "request_id": "uuid" } }

**Response 400:** records array empty or body malformed
**Response 401:** missing or invalid credentials
**Response 500:** WAL or hot tier write failure

**curl example:**
curl -X POST http://localhost:8080/ingest/logs \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"records":[{"level":"info","message":"hello world"}]}'

---

## POST /ingest/metrics

Ingest one or more metric records.

**Auth:** JWT or API key

**Request body:**
{
  "records": [
    {
      "timestamp": 1700000000000000000,
      "name": "cpu.usage",
      "value": 87.5,
      "tags": {"host": "server-01", "region": "in-south"}
    }
  ]
}

Field notes:
- name: required, non-empty
- value: float64
- timestamp: optional, defaults to server time
- tags: optional string key-value pairs

**Response 201:** { "status": "ok", "data": { "ingested": 1 } }
**Response 400:** name missing, records empty, or body malformed

**curl example:**
curl -X POST http://localhost:8080/ingest/metrics \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"records":[{"name":"cpu.usage","value":42.5}]}'

---

## POST /ingest/json

Ingest one or more arbitrary JSON documents.

**Auth:** JWT or API key

**Request body:**
{
  "records": [
    {
      "timestamp": 1700000000000000000,
      "data": {
        "event": "order_placed",
        "amount": 299.99,
        "currency": "INR"
      }
    }
  ]
}

Field notes:
- data: required, must be a JSON object (not null, not array)
- timestamp: optional, defaults to server time
- Schema is inferred from top-level fields of data

**Response 201:** { "status": "ok", "data": { "ingested": 1 } }
**Response 400:** data null/missing, records empty, or body malformed

**curl example:**
curl -X POST http://localhost:8080/ingest/json \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"records":[{"data":{"event":"signup","plan":"pro"}}]}'

---

## POST /ingest/kv

Ingest one or more key-value pairs.

**Auth:** JWT or API key

**Request body:**
{
  "records": [
    {
      "key": "user:alice:session",
      "value": "token_abc123"
    }
  ]
}

Field notes:
- key: required, non-empty string
- value: string, stored as-is
- No schema inference for kv — keys are user-defined

**Response 201:** { "status": "ok", "data": { "ingested": 1 } }
**Response 400:** key empty, records empty, or body malformed

**curl example:**
curl -X POST http://localhost:8080/ingest/kv \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"records":[{"key":"config:max_retries","value":"5"}]}'

---

## Schema Inference

Plomvix automatically infers the schema of ingested JSON records.
Schemas are stored in RocksDB and updated on every ingest call.

**Inferred types:**
| JSON value | Type |
|---|---|
| true/false | bool |
| whole number | int64 |
| decimal number | float64 |
| string | string |
| null | null |
| object | object |
| array | array |

If the same field is seen with different types across records, its type becomes `mixed`.
Schema is available via the query API (Sprint 6).

---

## Batch Ingestion

All endpoints support batching. Send multiple records in one request:

curl -X POST http://localhost:8080/ingest/logs \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "records": [
      {"level":"info","message":"first"},
      {"level":"warn","message":"second"},
      {"level":"error","message":"third"}
    ]
  }'

Response: { "status": "ok", "data": { "ingested": 3 } }
```

**Verify:** `cat docs/api/ingest.md` shows full content. Renders on GitHub.

---

## TASK 11 — Full build and smoke test

**Action:** Run the following verification sequence:

```bash
#!/bin/bash
set -euo pipefail

# Delete stale RocksDB data — _meta CF added in Sprint 5
echo "=== Clearing stale RocksDB data ==="
rm -rf data/hot/
rm -rf data/wal/

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
./plomvix > /tmp/plomvix_s5.log 2>&1 &
SERVER_PID=$!
sleep 3

echo ""
echo "=== Step 4: Login ==="
TOKEN=$(curl -sf -X POST http://localhost:8080/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"changeme"}' \
    | jq -r '.data.token')
echo "Token acquired"

echo ""
echo "=== Step 5: Ingest a log ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/logs \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"records":[{"level":"info","message":"smoke test log"}]}')
[ "$STATUS" -eq 201 ] \
    && echo "PASS: log ingested (201)" \
    || { echo "FAIL: expected 201, got $STATUS"; exit 1; }

echo ""
echo "=== Step 6: Ingest a metric ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/metrics \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"records":[{"name":"cpu.usage","value":55.5}]}')
[ "$STATUS" -eq 201 ] \
    && echo "PASS: metric ingested (201)" \
    || { echo "FAIL: expected 201, got $STATUS"; exit 1; }

echo ""
echo "=== Step 7: Ingest JSON ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/json \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"records":[{"data":{"event":"smoke_test","ok":true}}]}')
[ "$STATUS" -eq 201 ] \
    && echo "PASS: json ingested (201)" \
    || { echo "FAIL: expected 201, got $STATUS"; exit 1; }

echo ""
echo "=== Step 8: Ingest KV ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/kv \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"records":[{"key":"test:key","value":"test_value"}]}')
[ "$STATUS" -eq 201 ] \
    && echo "PASS: kv ingested (201)" \
    || { echo "FAIL: expected 201, got $STATUS"; exit 1; }

echo ""
echo "=== Step 9: Ingest without auth returns 401 ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/logs \
    -H "Content-Type: application/json" \
    -d '{"records":[{"level":"info","message":"unauthorized"}]}')
[ "$STATUS" -eq 401 ] \
    && echo "PASS: no auth → 401" \
    || { echo "FAIL: expected 401, got $STATUS"; exit 1; }

echo ""
echo "=== Step 10: Batch ingest 3 logs ==="
RESP=$(curl -sf -X POST http://localhost:8080/ingest/logs \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"records":[{"level":"info","message":"a"},{"level":"warn","message":"b"},{"level":"error","message":"c"}]}')
INGESTED=$(echo "$RESP" | jq -r '.data.ingested')
[ "$INGESTED" -eq 3 ] \
    && echo "PASS: batch ingested 3 records" \
    || { echo "FAIL: ingested=$INGESTED, want 3"; exit 1; }

echo ""
echo "=== Step 11: Health shows total_data_writes > 0 ==="
HEALTH=$(curl -sf http://localhost:8080/health)
TOTAL_DATA_WRITES=$(echo "$HEALTH" | jq '.data.hot.total_data_writes')
[ "$TOTAL_DATA_WRITES" -gt 0 ] \
    && echo "PASS: total_data_writes=$TOTAL_DATA_WRITES" \
    || { echo "FAIL: total_data_writes=$TOTAL_DATA_WRITES, want > 0"; exit 1; }

echo ""
echo "=== Step 12: Graceful shutdown ==="
kill -SIGTERM "$SERVER_PID"
wait "$SERVER_PID"
EXIT_CODE=$?
SERVER_PID=""
[ "$EXIT_CODE" -eq 0 ] \
    && echo "PASS: clean shutdown" \
    || { echo "FAIL: exit code $EXIT_CODE"; exit 1; }

echo ""
echo "================================================"
echo "  ALL STEPS PASSED — Sprint 5 smoke test DONE  "
echo "================================================"
```

**Expected results:**

| Step | What is verified | Expected |
|---|---|---|
| 1 | Build + vet with CGO | Binary produced, no errors |
| 2 | All tests including schema + handler tests | All pass with race detector |
| 3 | Boot | Server starts with _meta CF present |
| 4 | Login | Returns valid JWT |
| 5 | Ingest log | 201 |
| 6 | Ingest metric | 201 |
| 7 | Ingest JSON | 201 |
| 8 | Ingest KV | 201 |
| 9 | No auth | 401 |
| 10 | Batch ingest | 3 records accepted, ingested=3 |
| 11 | Health stats | total_data_writes > 0 |
| 12 | Graceful shutdown | Exit code 0 |

---

## EXECUTION ORDER SUMMARY

```
TASK 01  →  internal/storage/hot/types.go (add CFMeta)
TASK 02  →  internal/ingestion/types.go
TASK 03  →  internal/ingestion/schema.go
TASK 04  →  internal/storage/hot/manager.go (add GetMeta, PutMeta)
TASK 05  →  internal/ingestion/handler.go
TASK 06  →  internal/storage/hot/manager.go (add ingestion stats)
TASK 07  →  internal/server/server.go (register routes, update health)
TASK 08  →  internal/ingestion/schema_test.go
TASK 09  →  internal/ingestion/handler_test.go
TASK 10  →  docs/api/ingest.md
TASK 11  →  smoke test — all 12 steps must pass
```

---

*Sprint 5 complete when TASK 11 passes with zero failures.*
*Plomvix — Built in India. Built for the world.*