# Plomvix — Sprint 6 Task Plan
### For: DeepSeek V4 Pro Coding Agent
### Language: Go 1.22 | Module: github.com/plomvix/plomvix

> Execute tasks in exact order. Each task is atomic — one file or one concern.
> Do not skip ahead. Each task depends on the previous.
> Every task has a Verify step — do not proceed until it passes.

---

## CONTEXT

Sprints 1–5 are complete. Sprint 6 adds the **Query Engine** — the read path for all
data stored in the hot tier. Data ingested via Sprint 5's ingest API can now be
retrieved, filtered, and paginated over HTTP.

**What Sprint 6 delivers:**
- `GET /query/logs` — time-range scan of log records with optional field filtering
- `GET /query/metrics` — time-range scan with optional metric name filter
- `GET /query/json` — time-range scan of JSON documents with optional field filtering
- `GET /query/kv/{key}` — point lookup of a single KV record
- `GET /query/schema/{type}` — return the inferred schema for a data type
- In-memory field filter engine — `field=value`, `field>value`, `field<value`, `field!=value` combined with `AND`
- Pagination via `limit` and `offset` query parameters
- All query endpoints require authentication (JWT or API key)
- API documentation in `docs/api/query.md`
- Full test coverage

**What Sprint 6 does NOT do:**
- No full SQL parser — filter syntax is a simple subset defined below
- No cold tier queries — that is Sprint 7
- No JOINs or aggregations — deferred to a future version

**Query path:**
```
HTTP request → parse params → RocksDB scan → in-memory filter → paginate → respond 200
```

---

## FILTER SYNTAX — READ BEFORE WRITING ANY CODE

Sprint 6 supports a simple filter expression passed as a `filter` query parameter.

**Supported operators:**

| Operator | Meaning | Example |
|---|---|---|
| `=` | equals | `level=info` |
| `!=` | not equals | `level!=debug` |
| `>` | greater than (numeric) | `value>50` |
| `<` | less than (numeric) | `value<100` |
| `>=` | greater than or equal | `value>=50` |
| `<=` | less than or equal | `value<=100` |

**Combining with AND:**
Multiple conditions are separated by ` AND ` (space-AND-space):
```
level=info AND value>50
level!=debug AND value>=10 AND value<=100
```

**OR is not supported in Sprint 6.** If needed, run two separate queries.

**Filter evaluation:**
- Fields are extracted from the JSON payload of each record
- All comparisons are string-based unless the field value is numeric
- If a field does not exist in a record, the filter condition evaluates to false
  (the record is excluded)
- Filter errors (unparseable expression) return HTTP 400

---

## TASK 01 — Create internal/query/ directory and types.go

**Action — Part A:**
```bash
mkdir -p internal/query
```

**Action — Part B:** Create `internal/query/types.go`.

**Full file content:**
```go
package query

// FilterOp represents a comparison operator in a filter expression.
type FilterOp string

const (
    FilterOpEq  FilterOp = "="
    FilterOpNeq FilterOp = "!="
    FilterOpGt  FilterOp = ">"
    FilterOpLt  FilterOp = "<"
    FilterOpGte FilterOp = ">="
    FilterOpLte FilterOp = "<="
)

// FilterCondition is a single field comparison: field op value.
type FilterCondition struct {
    Field string
    Op    FilterOp
    Value string
}

// QueryParams holds the parsed parameters for a time-range query.
type QueryParams struct {
    FromNs     int64              // start timestamp, Unix nanoseconds (0 = beginning of time)
    ToNs       int64              // end timestamp, Unix nanoseconds (0 = now)
    Filters    []FilterCondition  // AND-combined filter conditions
    Limit      int                // max records to return (default 100, max 10000)
    Offset     int                // records to skip before returning
    MetricName string             // optional: metrics CF only — filter by metric name
}

// QueryResult is the response envelope for all query endpoints.
type QueryResult struct {
    Records []map[string]interface{} `json:"records"`
    Count   int                      `json:"count"`   // number of records in this page
    Total   int                      `json:"total"`   // total matching records before pagination
    Limit   int                      `json:"limit"`
    Offset  int                      `json:"offset"`
    QueryMs int64                    `json:"query_ms"` // query execution time in milliseconds
    DataType string                  `json:"data_type"`
}

// DefaultLimit is the default number of records returned per query.
const DefaultLimit = 100

// MaxLimit is the maximum number of records that can be requested in a single query.
const MaxLimit = 10000
```

**Verify:** `go build ./internal/query/` compiles with no errors.

---

## TASK 02 — Create internal/query/filter.go

**Action:** Create `internal/query/filter.go` — filter expression parser and evaluator.

**Imports required:**
```go
import (
    "fmt"
    "strconv"
    "strings"
)
```

**Full file content:**
```go
package query

import (
    "fmt"
    "strconv"
    "strings"
)

// ParseFilter parses a filter string into a slice of FilterConditions.
// Returns an error if the expression is malformed.
// Empty string returns nil slice (no filter — all records match).
//
// Format: "field=value AND field2>value2"
// Conditions are split on " AND " (space-AND-space, case-insensitive).
func ParseFilter(expr string) ([]FilterCondition, error) {
    expr = strings.TrimSpace(expr)
    if expr == "" {
        return nil, nil
    }

    // Split on " AND " — case-insensitive
    parts := splitAND(expr)
    conditions := make([]FilterCondition, 0, len(parts))

    for _, part := range parts {
        part = strings.TrimSpace(part)
        if part == "" {
            continue
        }
        cond, err := parseCondition(part)
        if err != nil {
            return nil, fmt.Errorf("invalid filter condition %q: %w", part, err)
        }
        conditions = append(conditions, cond)
    }
    return conditions, nil
}

// splitAND splits a filter expression on " AND " (case-insensitive).
func splitAND(expr string) []string {
    upper := strings.ToUpper(expr)
    var parts []string
    for {
        idx := strings.Index(upper, " AND ")
        if idx == -1 {
            parts = append(parts, expr)
            break
        }
        parts = append(parts, expr[:idx])
        expr = expr[idx+5:]
        upper = upper[idx+5:]
    }
    return parts
}

// parseCondition parses a single "field op value" condition.
// Operators are tried longest-first to avoid matching ">" before ">=".
func parseCondition(s string) (FilterCondition, error) {
    ops := []FilterOp{FilterOpGte, FilterOpLte, FilterOpNeq, FilterOpEq, FilterOpGt, FilterOpLt}
    for _, op := range ops {
        idx := strings.Index(s, string(op))
        if idx > 0 {
            field := strings.TrimSpace(s[:idx])
            value := strings.TrimSpace(s[idx+len(op):])
            if field == "" || value == "" {
                return FilterCondition{}, fmt.Errorf("field and value must not be empty")
            }
            return FilterCondition{Field: field, Op: op, Value: value}, nil
        }
    }
    return FilterCondition{}, fmt.Errorf("no valid operator found in %q", s)
}

// ApplyFilters returns true if the record matches ALL filter conditions.
// If the record is nil or filters is nil/empty, returns true.
func ApplyFilters(record map[string]interface{}, filters []FilterCondition) bool {
    if len(filters) == 0 {
        return true
    }
    for _, f := range filters {
        if !matchCondition(record, f) {
            return false
        }
    }
    return true
}

// matchCondition checks a single filter condition against a record.
func matchCondition(record map[string]interface{}, f FilterCondition) bool {
    val, ok := record[f.Field]
    if !ok {
        return false // field not present — exclude record
    }

    recordStr := fmt.Sprintf("%v", val)

    switch f.Op {
    case FilterOpEq:
        return recordStr == f.Value
    case FilterOpNeq:
        return recordStr != f.Value
    case FilterOpGt, FilterOpLt, FilterOpGte, FilterOpLte:
        return numericCompare(val, f.Op, f.Value)
    }
    return false
}

// numericCompare performs a numeric comparison between a record value and a filter value.
// Falls back to string comparison if either value is non-numeric.
func numericCompare(recordVal interface{}, op FilterOp, filterVal string) bool {
    var recordFloat float64

    switch v := recordVal.(type) {
    case float64:
        recordFloat = v
    case int64:
        recordFloat = float64(v)
    default:
        // Try string-to-float conversion
        f, err := strconv.ParseFloat(fmt.Sprintf("%v", recordVal), 64)
        if err != nil {
            return false
        }
        recordFloat = f
    }

    filterFloat, err := strconv.ParseFloat(filterVal, 64)
    if err != nil {
        return false
    }

    switch op {
    case FilterOpGt:
        return recordFloat > filterFloat
    case FilterOpLt:
        return recordFloat < filterFloat
    case FilterOpGte:
        return recordFloat >= filterFloat
    case FilterOpLte:
        return recordFloat <= filterFloat
    }
    return false
}

```

**Verify:** `go build ./internal/query/` compiles with no errors.

---

## TASK 03 — Create internal/query/params.go

**Action:** Create `internal/query/params.go` — HTTP query parameter parsing.

**Imports required:**
```go
import (
    "fmt"
    "net/http"
    "strconv"
    "time"
)
```

**Full file content:**
```go
package query

import (
    "fmt"
    "net/http"
    "strconv"
    "time"
)

// ParseQueryParams extracts and validates query parameters from an HTTP request.
// Handles: from, to, filter, limit, offset.
//
// from / to: Unix nanoseconds as int64 strings.
//            If from is 0 or absent, defaults to 0 (beginning of time).
//            If to is 0 or absent, defaults to time.Now().UnixNano().
//
// filter: filter expression string (see filter.go).
//
// limit: max records per page. Default DefaultLimit. Max MaxLimit.
//
// offset: records to skip. Default 0.
func ParseQueryParams(r *http.Request) (*QueryParams, error) {
    q := r.URL.Query()
    params := &QueryParams{
        Limit: DefaultLimit,
    }

    // from
    if s := q.Get("from"); s != "" {
        v, err := strconv.ParseInt(s, 10, 64)
        if err != nil {
            return nil, fmt.Errorf("invalid 'from' parameter: must be Unix nanoseconds int64")
        }
        params.FromNs = v
    }

    // to
    if s := q.Get("to"); s != "" {
        v, err := strconv.ParseInt(s, 10, 64)
        if err != nil {
            return nil, fmt.Errorf("invalid 'to' parameter: must be Unix nanoseconds int64")
        }
        params.ToNs = v
    } else {
        params.ToNs = time.Now().UnixNano()
    }

    if params.FromNs > 0 && params.ToNs > 0 && params.FromNs >= params.ToNs {
        return nil, fmt.Errorf("'from' must be less than 'to'")
    }

    // filter
    if s := q.Get("filter"); s != "" {
        filters, err := ParseFilter(s)
        if err != nil {
            return nil, fmt.Errorf("invalid filter: %w", err)
        }
        params.Filters = filters
    }

    // limit
    if s := q.Get("limit"); s != "" {
        v, err := strconv.Atoi(s)
        if err != nil || v <= 0 {
            return nil, fmt.Errorf("invalid 'limit' parameter: must be a positive integer")
        }
        if v > MaxLimit {
            v = MaxLimit
        }
        params.Limit = v
    }

    // offset
    if s := q.Get("offset"); s != "" {
        v, err := strconv.Atoi(s)
        if err != nil || v < 0 {
            return nil, fmt.Errorf("invalid 'offset' parameter: must be a non-negative integer")
        }
        params.Offset = v
    }

    return params, nil
}
```

**Verify:** `go build ./internal/query/` compiles with no errors.

---

## TASK 04 — Create internal/query/engine.go

**Action:** Create `internal/query/engine.go` — the query execution engine.

**Imports required:**
```go
import (
    "encoding/json"
    "time"

    "github.com/plomvix/plomvix/internal/ingestion"
    "github.com/plomvix/plomvix/internal/storage/hot"
)
```

**Full file content:**
```go
package query

import (
    "encoding/json"
    "time"

    "github.com/plomvix/plomvix/internal/ingestion"
    "github.com/plomvix/plomvix/internal/storage/hot"
)

// DecodePayload unmarshals a raw JSON byte slice into a map.
// Returns nil if decoding fails — callers must check for nil.
func DecodePayload(raw []byte) map[string]interface{} {
    var m map[string]interface{}
    if err := json.Unmarshal(raw, &m); err != nil {
        return nil
    }
    return m
}

// Engine executes queries against the hot tier.
type Engine struct {
    store *hot.Manager
}

// NewEngine creates a new query Engine.
func NewEngine(store *hot.Manager) *Engine {
    return &Engine{store: store}
}

// QueryLogs scans the logs column family and returns matching records.
func (e *Engine) QueryLogs(params *QueryParams) (*QueryResult, error) {
    return e.queryTimeSeries(hot.CFLogs, "logs", params)
}

// QueryJSON scans the json column family and returns matching records.
func (e *Engine) QueryJSON(params *QueryParams) (*QueryResult, error) {
    return e.queryTimeSeries(hot.CFJSON, "json", params)
}

// QueryMetrics scans the metrics column family and returns matching records.
// If params.MetricName is set (non-empty), only records with that metric name are returned.
func (e *Engine) QueryMetrics(params *QueryParams) (*QueryResult, error) {
    return e.queryTimeSeries(hot.CFMetrics, "metrics", params)
}

// QueryKV retrieves a single key-value record by key.
// Returns a QueryResult with 0 or 1 records.
func (e *Engine) QueryKV(key string) (*QueryResult, error) {
    start := time.Now()

    raw, err := e.store.GetKV(key)
    if err != nil {
        return nil, err
    }

    result := &QueryResult{
        DataType: "kv",
        Limit:    1,
        Offset:   0,
    }

    if raw == nil {
        result.Records = []map[string]interface{}{}
        result.QueryMs = time.Since(start).Milliseconds()
        return result, nil
    }

    record := DecodePayload(raw)
    if record == nil {
        result.Records = []map[string]interface{}{}
    } else {
        result.Records = []map[string]interface{}{record}
        result.Count = 1
        result.Total = 1
    }
    result.QueryMs = time.Since(start).Milliseconds()
    return result, nil
}

// QuerySchema returns the inferred schema for a data type.
// dataType must be one of: "logs", "metrics", "json", "kv".
func (e *Engine) QuerySchema(dataType string) (*ingestion.Schema, error) {
    return ingestion.LoadSchema(e.store, dataType)
}

// queryTimeSeries is the shared implementation for logs, json, and metrics queries.
func (e *Engine) queryTimeSeries(cf, dataType string, params *QueryParams) (*QueryResult, error) {
    start := time.Now()

    // Collect all matching records
    var all []map[string]interface{}

    err := e.store.ScanCF(cf, params.FromNs, params.ToNs, func(raw []byte) bool {
        record := DecodePayload(raw)
        if record == nil {
            return true // skip unparseable records, continue scanning
        }
        if ApplyFilters(record, params.Filters) {
            all = append(all, record)
        }
        return true // always continue scanning — collect all matching
    })
    if err != nil {
        return nil, err
    }

    total := len(all)

    // Apply pagination
    start2 := params.Offset
    if start2 > total {
        start2 = total
    }
    end := start2 + params.Limit
    if end > total {
        end = total
    }
    page := all[start2:end]
    if page == nil {
        page = []map[string]interface{}{}
    }

    return &QueryResult{
        Records:  page,
        Count:    len(page),
        Total:    total,
        Limit:    params.Limit,
        Offset:   params.Offset,
        QueryMs:  time.Since(start).Milliseconds(),
        DataType: dataType,
    }, nil
}
```

**IMPORTANT — `engine.go` calls `e.store.ScanCF` which is added in TASK 05.**
`go build ./internal/query/` will fail until TASK 05 is complete.
Do TASK 05 before verifying TASK 04.

---

## TASK 05 — Add ScanCF to internal/storage/hot/manager.go

**Action:** Add `ScanCF` to `internal/storage/hot/manager.go`.

`ScanCF` is a time-range scan method that the query engine uses.
It differs from the existing `ScanLogs`/`ScanJSON` methods because it exposes
the raw `[]byte` payload directly to the caller instead of returning a `[][]byte` slice.
This lets the query engine decode and filter records without loading all into memory first.

```go
// ScanCF iterates a time-series column family in the range [fromNs, toNs)
// and calls fn for each raw payload. If fn returns false, iteration stops.
// fromNs=0 scans from the beginning. toNs=0 scans to the current time.
func (m *Manager) ScanCF(cf string, fromNs, toNs int64, fn func(payload []byte) bool) error {
    if toNs == 0 {
        toNs = time.Now().UnixNano()
    }
    return m.store.Scan(cf, BuildRangeScanPrefix(fromNs), func(key, value []byte) bool {
        if toNs > 0 && len(key) >= 8 {
            keyTs := binary.BigEndian.Uint64(key[:8])
            if int64(keyTs) >= toNs {
                return false // past end of range — stop
            }
        }
        return fn(value)
    })
}
```

**Import to add to manager.go:** `"encoding/binary"` and `"time"` — both needed for `ScanCF`.

Check existing imports in manager.go. `encoding/binary` is already imported
(used by `bigEndianUint64`). Add `"time"` if not already present.

**Verify:** `CGO_ENABLED=1 go build ./internal/storage/hot/` compiles with no errors.

---

## TASK 06 — Create internal/query/handler.go

**Action:** Create `internal/query/handler.go`.

**Imports required:**
```go
import (
    "net/http"

    "github.com/go-chi/chi/v5"

    "github.com/plomvix/plomvix/pkg/utils"
)
```

**Handler struct:**
```go
// Handler handles all query HTTP endpoints.
type Handler struct {
    engine *Engine
}

// NewHandler creates a new query Handler.
func NewHandler(engine *Engine) *Handler {
    return &Handler{engine: engine}
}
```

---

**`GET /query/logs`**

- **Auth required:** Yes
- **Query params:** `from`, `to`, `filter`, `limit`, `offset`

Success response — HTTP 200:
```json
{
  "status": "ok",
  "data": {
    "records": [
      {"level": "info", "message": "hello", "timestamp": 1700000000000000000}
    ],
    "count": 1,
    "total": 1,
    "limit": 100,
    "offset": 0,
    "query_ms": 2,
    "data_type": "logs"
  },
  "request_id": "uuid"
}
```

```go
// QueryLogs handles GET /query/logs.
//
// GET /query/logs
// Auth: JWT or API key
//
// Query params:
//   from:   Unix nanoseconds start (default: 0)
//   to:     Unix nanoseconds end (default: now)
//   filter: filter expression (e.g. "level=info AND value>50")
//   limit:  max records (default 100, max 10000)
//   offset: skip N records (default 0)
//
// Responses:
//   200 OK          — query results
//   400 Bad Request — VALIDATION_FAILED: invalid params or filter
//   500 Internal    — INTERNAL_ERROR: scan failed
func (h *Handler) QueryLogs(w http.ResponseWriter, r *http.Request) {
    params, err := ParseQueryParams(r)
    if err != nil {
        utils.BadRequest(w, r, utils.CodeValidationFailed, err.Error())
        return
    }
    result, err := h.engine.QueryLogs(params)
    if err != nil {
        utils.InternalError(w, r, "query failed")
        return
    }
    utils.OK(w, r, result)
}
```

---

**`GET /query/metrics`**

Additional query param: `name` — filter by metric name (optional).

```go
// QueryMetrics handles GET /query/metrics.
//
// GET /query/metrics
// Auth: JWT or API key
//
// Query params:
//   from, to, filter, limit, offset (same as /query/logs)
//   name: optional metric name filter
//
// Responses:
//   200 OK          — query results
//   400 Bad Request — VALIDATION_FAILED
//   500 Internal    — INTERNAL_ERROR
func (h *Handler) QueryMetrics(w http.ResponseWriter, r *http.Request) {
    params, err := ParseQueryParams(r)
    if err != nil {
        utils.BadRequest(w, r, utils.CodeValidationFailed, err.Error())
        return
    }
    // Metric name filter — add as extra filter condition if provided
    if name := r.URL.Query().Get("name"); name != "" {
        params.MetricName = name
        params.Filters = append(params.Filters, FilterCondition{
            Field: "name",
            Op:    FilterOpEq,
            Value: name,
        })
    }
    result, err := h.engine.QueryMetrics(params)
    if err != nil {
        utils.InternalError(w, r, "query failed")
        return
    }
    utils.OK(w, r, result)
}
```

---

**`GET /query/json`**

Same as logs. Handler method: `QueryJSON`.

```go
// QueryJSON handles GET /query/json.
//
// GET /query/json
// Auth: JWT or API key
//
// Query params: from, to, filter, limit, offset
//
// Responses:
//   200 OK / 400 Bad Request / 500 Internal
func (h *Handler) QueryJSON(w http.ResponseWriter, r *http.Request) {
    params, err := ParseQueryParams(r)
    if err != nil {
        utils.BadRequest(w, r, utils.CodeValidationFailed, err.Error())
        return
    }
    result, err := h.engine.QueryJSON(params)
    if err != nil {
        utils.InternalError(w, r, "query failed")
        return
    }
    utils.OK(w, r, result)
}
```

---

**`GET /query/kv/{key}`**

Point lookup — no time range or filter.

```go
// QueryKV handles GET /query/kv/{key}.
//
// GET /query/kv/{key}
// Auth: JWT or API key
//
// Path param: key — the KV key to look up
//
// Responses:
//   200 OK          — record found (count=1) or not found (count=0, records=[])
//   500 Internal    — INTERNAL_ERROR
func (h *Handler) QueryKV(w http.ResponseWriter, r *http.Request) {
    key := chi.URLParam(r, "key")
    result, err := h.engine.QueryKV(key)
    if err != nil {
        utils.InternalError(w, r, "query failed")
        return
    }
    utils.OK(w, r, result)
}
```

---

**`GET /query/schema/{type}`**

Returns the inferred schema for a data type.

```go
// QuerySchema handles GET /query/schema/{type}.
//
// GET /query/schema/{type}
// Auth: JWT or API key
//
// Path param: type — one of: logs, metrics, json, kv
//
// Responses:
//   200 OK          — schema for the data type
//   400 Bad Request — VALIDATION_FAILED: unknown data type
//   500 Internal    — INTERNAL_ERROR
func (h *Handler) QuerySchema(w http.ResponseWriter, r *http.Request) {
    dataType := chi.URLParam(r, "type")
    valid := map[string]bool{"logs": true, "metrics": true, "json": true, "kv": true}
    if !valid[dataType] {
        utils.BadRequest(w, r, utils.CodeValidationFailed,
            "type must be one of: logs, metrics, json, kv")
        return
    }
    schema, err := h.engine.QuerySchema(dataType)
    if err != nil {
        utils.InternalError(w, r, "failed to load schema")
        return
    }
    utils.OK(w, r, schema)
}
```

**Verify:** `go build ./internal/query/` compiles with no errors.

---

## TASK 07 — Register query routes in internal/server/server.go

**Action:** Two targeted changes to `internal/server/server.go`.

**Change 1 — Add query import:**
```go
"github.com/plomvix/plomvix/internal/query"
```

**Change 2 — Register query routes in `setupRoutes()`.**

Add after the ingestion route group:
```go
// Query — auth required
queryEngine  := query.NewEngine(s.hotTier)
queryHandler := query.NewHandler(queryEngine)
r.Group(func(r chi.Router) {
    r.Use(auth.Middleware(s.store, s.blacklist, s.cfg))
    r.Get("/query/logs",            queryHandler.QueryLogs)
    r.Get("/query/metrics",         queryHandler.QueryMetrics)
    r.Get("/query/json",            queryHandler.QueryJSON)
    r.Get("/query/kv/{key}",        queryHandler.QueryKV)
    r.Get("/query/schema/{type}",   queryHandler.QuerySchema)
})
```

**Verify:** `CGO_ENABLED=1 go build ./internal/server/` compiles with no errors.

---

## TASK 08 — Create internal/query/filter_test.go

**Action:** Create `internal/query/filter_test.go`.

**Package declaration:** `package query`

**Imports:**
```go
import "testing"
```

**Full file content:**
```go
package query

import "testing"

func TestParseFilterEmpty(t *testing.T) {
    conditions, err := ParseFilter("")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(conditions) != 0 {
        t.Errorf("expected 0 conditions, got %d", len(conditions))
    }
}

func TestParseFilterSingle(t *testing.T) {
    conditions, err := ParseFilter("level=info")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(conditions) != 1 {
        t.Fatalf("expected 1 condition, got %d", len(conditions))
    }
    if conditions[0].Field != "level" {
        t.Errorf("field = %q, want %q", conditions[0].Field, "level")
    }
    if conditions[0].Op != FilterOpEq {
        t.Errorf("op = %q, want =", conditions[0].Op)
    }
    if conditions[0].Value != "info" {
        t.Errorf("value = %q, want %q", conditions[0].Value, "info")
    }
}

func TestParseFilterMultiple(t *testing.T) {
    conditions, err := ParseFilter("level=info AND value>50")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(conditions) != 2 {
        t.Fatalf("expected 2 conditions, got %d", len(conditions))
    }
    if conditions[1].Op != FilterOpGt {
        t.Errorf("second op = %q, want >", conditions[1].Op)
    }
}

func TestParseFilterAllOps(t *testing.T) {
    tests := []struct {
        input string
        op    FilterOp
    }{
        {"x=1", FilterOpEq},
        {"x!=1", FilterOpNeq},
        {"x>1", FilterOpGt},
        {"x<1", FilterOpLt},
        {"x>=1", FilterOpGte},
        {"x<=1", FilterOpLte},
    }
    for _, tt := range tests {
        conds, err := ParseFilter(tt.input)
        if err != nil {
            t.Errorf("ParseFilter(%q) error: %v", tt.input, err)
            continue
        }
        if len(conds) != 1 || conds[0].Op != tt.op {
            t.Errorf("ParseFilter(%q) op = %q, want %q", tt.input, conds[0].Op, tt.op)
        }
    }
}

func TestParseFilterInvalid(t *testing.T) {
    _, err := ParseFilter("noop")
    if err == nil {
        t.Error("expected error for filter with no operator, got nil")
    }
}

func TestApplyFiltersEmpty(t *testing.T) {
    record := map[string]interface{}{"level": "info"}
    if !ApplyFilters(record, nil) {
        t.Error("ApplyFilters with nil conditions should return true")
    }
}

func TestApplyFiltersMatch(t *testing.T) {
    record := map[string]interface{}{"level": "info", "value": float64(75)}
    conditions := []FilterCondition{
        {Field: "level", Op: FilterOpEq, Value: "info"},
        {Field: "value", Op: FilterOpGt, Value: "50"},
    }
    if !ApplyFilters(record, conditions) {
        t.Error("expected record to match all conditions")
    }
}

func TestApplyFiltersNoMatch(t *testing.T) {
    record := map[string]interface{}{"level": "debug", "value": float64(10)}
    conditions := []FilterCondition{
        {Field: "level", Op: FilterOpEq, Value: "info"}, // does not match
    }
    if ApplyFilters(record, conditions) {
        t.Error("expected record to NOT match")
    }
}

func TestApplyFiltersMissingField(t *testing.T) {
    record := map[string]interface{}{"level": "info"}
    conditions := []FilterCondition{
        {Field: "nonexistent", Op: FilterOpEq, Value: "anything"},
    }
    if ApplyFilters(record, conditions) {
        t.Error("expected false for missing field")
    }
}

func TestApplyFiltersNeq(t *testing.T) {
    record := map[string]interface{}{"level": "warn"}
    conditions := []FilterCondition{
        {Field: "level", Op: FilterOpNeq, Value: "info"},
    }
    if !ApplyFilters(record, conditions) {
        t.Error("warn != info should be true")
    }
}

func TestNumericCompareGte(t *testing.T) {
    record := map[string]interface{}{"value": float64(50)}
    conds := []FilterCondition{{Field: "value", Op: FilterOpGte, Value: "50"}}
    if !ApplyFilters(record, conds) {
        t.Error("50 >= 50 should be true")
    }
}

func TestNumericCompareLte(t *testing.T) {
    record := map[string]interface{}{"value": float64(49)}
    conds := []FilterCondition{{Field: "value", Op: FilterOpLte, Value: "50"}}
    if !ApplyFilters(record, conds) {
        t.Error("49 <= 50 should be true")
    }
}
```

**Verify:** `go test -race ./internal/query/` — all filter tests pass.

---

## TASK 09 — Create internal/query/engine_test.go

**Action:** Create `internal/query/engine_test.go`.

**Package declaration:** `package query`

**Imports:**
```go
import (
    "path/filepath"
    "testing"
    "time"

    "github.com/plomvix/plomvix/internal/config"
    "github.com/plomvix/plomvix/internal/storage/hot"
)
```

**Test helper:**
```go
func newTestEngine(t *testing.T) *Engine {
    t.Helper()
    dir := filepath.Join(t.TempDir(), "hot")
    cfg := &config.Config{
        Storage: config.StorageConfig{DataDir: dir},
    }
    m, err := hot.Open(dir, cfg)
    if err != nil {
        t.Fatalf("hot.Open failed: %v", err)
    }
    t.Cleanup(func() { m.Close() })
    return NewEngine(m)
}
```

**Tests to implement:**

```go
func TestQueryLogsEmpty(t *testing.T) {
    e := newTestEngine(t)
    params := &QueryParams{ToNs: time.Now().UnixNano(), Limit: DefaultLimit}
    result, err := e.QueryLogs(params)
    if err != nil {
        t.Fatalf("QueryLogs failed: %v", err)
    }
    if result.Total != 0 {
        t.Errorf("expected 0 results, got %d", result.Total)
    }
    if result.Records == nil {
        t.Error("Records should be non-nil empty slice, not nil")
    }
}

func TestQueryLogsWithData(t *testing.T) {
    e := newTestEngine(t)

    // Write 3 log records directly via hot tier
    base := time.Now().UnixNano()
    payloads := [][]byte{
        []byte(`{"level":"info","message":"first"}`),
        []byte(`{"level":"warn","message":"second"}`),
        []byte(`{"level":"error","message":"third"}`),
    }
    for i, p := range payloads {
        if err := e.store.WriteLog(base+int64(i)*1000, p); err != nil {
            t.Fatalf("WriteLog %d failed: %v", i, err)
        }
    }

    params := &QueryParams{
        FromNs: base - 1,
        ToNs:   base + int64(len(payloads))*1000 + 1,
        Limit:  DefaultLimit,
    }
    result, err := e.QueryLogs(params)
    if err != nil {
        t.Fatalf("QueryLogs failed: %v", err)
    }
    if result.Total != 3 {
        t.Errorf("total = %d, want 3", result.Total)
    }
}

func TestQueryLogsFilter(t *testing.T) {
    e := newTestEngine(t)

    base := time.Now().UnixNano()
    e.store.WriteLog(base+0, []byte(`{"level":"info","message":"a"}`))
    e.store.WriteLog(base+1000, []byte(`{"level":"warn","message":"b"}`))
    e.store.WriteLog(base+2000, []byte(`{"level":"info","message":"c"}`))

    conditions, _ := ParseFilter("level=info")
    params := &QueryParams{
        FromNs:  base - 1,
        ToNs:    base + 3000,
        Filters: conditions,
        Limit:   DefaultLimit,
    }
    result, err := e.QueryLogs(params)
    if err != nil {
        t.Fatalf("QueryLogs with filter failed: %v", err)
    }
    if result.Total != 2 {
        t.Errorf("total = %d, want 2 (only info records)", result.Total)
    }
}

func TestQueryLogsPagination(t *testing.T) {
    e := newTestEngine(t)

    base := time.Now().UnixNano()
    for i := 0; i < 5; i++ {
        e.store.WriteLog(base+int64(i)*1000, []byte(`{"level":"info","seq":1}`))
    }

    params := &QueryParams{
        FromNs: base - 1,
        ToNs:   base + 6000,
        Limit:  2,
        Offset: 1,
    }
    result, err := e.QueryLogs(params)
    if err != nil {
        t.Fatalf("QueryLogs pagination failed: %v", err)
    }
    if result.Total != 5 {
        t.Errorf("total = %d, want 5", result.Total)
    }
    if result.Count != 2 {
        t.Errorf("count = %d, want 2 (limit=2)", result.Count)
    }
}

func TestQueryKVFound(t *testing.T) {
    e := newTestEngine(t)
    e.store.WriteKV("mykey", []byte(`{"key":"mykey","value":"myval"}`))

    result, err := e.QueryKV("mykey")
    if err != nil {
        t.Fatalf("QueryKV failed: %v", err)
    }
    if result.Total != 1 {
        t.Errorf("total = %d, want 1", result.Total)
    }
}

func TestQueryKVNotFound(t *testing.T) {
    e := newTestEngine(t)
    result, err := e.QueryKV("doesnotexist")
    if err != nil {
        t.Fatalf("QueryKV failed: %v", err)
    }
    if result.Total != 0 {
        t.Errorf("total = %d, want 0", result.Total)
    }
    if len(result.Records) != 0 {
        t.Errorf("records len = %d, want 0", len(result.Records))
    }
}
```

**NOTE:** `e.store` is a private field. Since the test is `package query` (not `package query_test`),
it can access `e.store` directly. The test compiles correctly. ✅

**Verify:** `CGO_ENABLED=1 go test -race ./internal/query/` — all engine tests pass.

---

## TASK 10 — Create docs/api/query.md

**Action:** Create `docs/api/query.md` with full actual content.

```markdown
# Plomvix Query API Reference

All query endpoints require authentication.
Use `Authorization: Bearer <token>` or `X-API-Key: <key>`.

---

## GET /query/logs

Query log records by time range with optional filtering.

**Auth:** JWT or API key

**Query parameters:**

| Param | Type | Default | Description |
|---|---|---|---|
| `from` | int64 | 0 | Start timestamp in Unix nanoseconds |
| `to` | int64 | now | End timestamp in Unix nanoseconds |
| `filter` | string | none | Filter expression (see Filter Syntax below) |
| `limit` | int | 100 | Max records to return (max 10000) |
| `offset` | int | 0 | Records to skip (for pagination) |

**Response 200:**
{
  "status": "ok",
  "data": {
    "records": [{"level":"info","message":"hello","timestamp":1700000000000000000}],
    "count": 1,
    "total": 1,
    "limit": 100,
    "offset": 0,
    "query_ms": 2,
    "data_type": "logs"
  }
}

**curl example:**
curl "http://localhost:8080/query/logs?from=1700000000000000000&filter=level%3Dinfo&limit=50" \
  -H "Authorization: Bearer <token>"

---

## GET /query/metrics

Query metric records by time range.

**Auth:** JWT or API key

**Additional query parameter:**

| Param | Type | Default | Description |
|---|---|---|---|
| `name` | string | none | Filter by metric name (e.g. `cpu.usage`) |

All other params same as /query/logs.

**curl example:**
curl "http://localhost:8080/query/metrics?name=cpu.usage&from=1700000000000000000" \
  -H "Authorization: Bearer <token>"

---

## GET /query/json

Query JSON document records by time range.

Same parameters as /query/logs.

**curl example:**
curl "http://localhost:8080/query/json?filter=event%3Dorder_placed" \
  -H "Authorization: Bearer <token>"

---

## GET /query/kv/{key}

Retrieve a single key-value record by key.

**Auth:** JWT or API key

**Path parameter:** `key` — the KV key to look up

**Response 200 (found):**
{
  "status": "ok",
  "data": {
    "records": [{"key":"mykey","value":"myval"}],
    "count": 1, "total": 1, "limit": 1, "offset": 0, "query_ms": 1, "data_type": "kv"
  }
}

**Response 200 (not found):** count=0, records=[]

**curl example:**
curl "http://localhost:8080/query/kv/user:alice:session" \
  -H "Authorization: Bearer <token>"

---

## GET /query/schema/{type}

Returns the inferred schema for a data type.

**Auth:** JWT or API key

**Path parameter:** `type` — one of: `logs`, `metrics`, `json`, `kv`

**Response 200:**
{
  "status": "ok",
  "data": {
    "data_type": "logs",
    "fields": {"level":"string","message":"string","timestamp":"int64"},
    "updated_at": "2024-01-15T10:30:00Z",
    "record_count": 150
  }
}

**Response 400:** invalid type value

**curl example:**
curl "http://localhost:8080/query/schema/logs" \
  -H "Authorization: Bearer <token>"

---

## Filter Syntax

The `filter` query parameter accepts a simple expression:

**Single condition:**
  filter=level=info
  filter=value>50
  filter=name!=debug

**Multiple conditions (AND only):**
  filter=level=info AND value>50
  filter=level!=debug AND value>=10 AND value<=100

**Supported operators:**

| Operator | Meaning |
|---|---|
| `=` | equals |
| `!=` | not equals |
| `>` | greater than (numeric) |
| `<` | less than (numeric) |
| `>=` | greater than or equal |
| `<=` | less than or equal |

URL-encode the filter value when using curl:
  filter=level=info → ?filter=level%3Dinfo
  filter=level=info AND value>50 → ?filter=level%3Dinfo%20AND%20value%3E50

Numeric comparisons require the field value to be a number in the record.
If a field is absent in a record, the record is excluded.
OR is not supported — run two separate queries.

---

## Pagination

All time-range endpoints support pagination:

curl "http://localhost:8080/query/logs?limit=20&offset=40" \
  -H "Authorization: Bearer <token>"

Response includes `total` (all matching records) and `count` (records in this page).
Default limit: 100. Maximum limit: 10000.
```

**Verify:** `cat docs/api/query.md` shows full content.

---

## TASK 11 — Full build and smoke test

**Action:** Run the following verification sequence:

```bash
#!/bin/bash
set -euo pipefail

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
./plomvix > /tmp/plomvix_s6.log 2>&1 &
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
echo "=== Step 5: Ingest test data ==="
curl -sf -X POST http://localhost:8080/ingest/logs \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"records":[{"level":"info","message":"hello"},{"level":"warn","message":"world"}]}' \
    > /dev/null
curl -sf -X POST http://localhost:8080/ingest/kv \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"records":[{"key":"smoke:key","value":"smoke_value"}]}' \
    > /dev/null
echo "Test data ingested"

echo ""
echo "=== Step 6: Query logs — expect 2 records ==="
RESP=$(curl -sf "http://localhost:8080/query/logs" \
    -H "Authorization: Bearer $TOKEN")
TOTAL=$(echo "$RESP" | jq '.data.total')
[ "$TOTAL" -ge 2 ] \
    && echo "PASS: total=$TOTAL" \
    || { echo "FAIL: total=$TOTAL, want >= 2"; exit 1; }

echo ""
echo "=== Step 7: Query logs with filter ==="
RESP=$(curl -sf "http://localhost:8080/query/logs?filter=level%3Dinfo" \
    -H "Authorization: Bearer $TOKEN")
TOTAL=$(echo "$RESP" | jq '.data.total')
[ "$TOTAL" -ge 1 ] \
    && echo "PASS: filter returned $TOTAL records" \
    || { echo "FAIL: filter returned 0"; exit 1; }

echo ""
echo "=== Step 8: Query KV — found ==="
RESP=$(curl -sf "http://localhost:8080/query/kv/smoke:key" \
    -H "Authorization: Bearer $TOKEN")
COUNT=$(echo "$RESP" | jq '.data.count')
[ "$COUNT" -eq 1 ] \
    && echo "PASS: KV lookup found 1 record" \
    || { echo "FAIL: count=$COUNT, want 1"; exit 1; }

echo ""
echo "=== Step 9: Query KV — not found ==="
RESP=$(curl -sf "http://localhost:8080/query/kv/doesnotexist" \
    -H "Authorization: Bearer $TOKEN")
COUNT=$(echo "$RESP" | jq '.data.count')
[ "$COUNT" -eq 0 ] \
    && echo "PASS: KV not found returns count=0" \
    || { echo "FAIL: count=$COUNT, want 0"; exit 1; }

echo ""
echo "=== Step 10: Query schema ==="
RESP=$(curl -sf "http://localhost:8080/query/schema/logs" \
    -H "Authorization: Bearer $TOKEN")
echo "$RESP" | jq '.data.fields' | grep -q "level" \
    && echo "PASS: schema has 'level' field" \
    || { echo "FAIL: schema missing 'level' field"; exit 1; }

echo ""
echo "=== Step 11: Query without auth returns 401 ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    "http://localhost:8080/query/logs")
[ "$STATUS" -eq 401 ] \
    && echo "PASS: no auth → 401" \
    || { echo "FAIL: expected 401, got $STATUS"; exit 1; }

echo ""
echo "=== Step 12: Invalid filter returns 400 ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    "http://localhost:8080/query/logs?filter=noop" \
    -H "Authorization: Bearer $TOKEN")
[ "$STATUS" -eq 400 ] \
    && echo "PASS: invalid filter → 400" \
    || { echo "FAIL: expected 400, got $STATUS"; exit 1; }

echo ""
echo "=== Step 13: Graceful shutdown ==="
kill -SIGTERM "$SERVER_PID"
wait "$SERVER_PID"
EXIT_CODE=$?
SERVER_PID=""
[ "$EXIT_CODE" -eq 0 ] \
    && echo "PASS: clean shutdown" \
    || { echo "FAIL: exit code $EXIT_CODE"; exit 1; }

echo ""
echo "================================================"
echo "  ALL STEPS PASSED — Sprint 6 smoke test DONE  "
echo "================================================"
```

**Expected results:**

| Step | What is verified | Expected |
|---|---|---|
| 1 | Build + vet with CGO | Binary produced, no errors |
| 2 | All tests including query tests | All pass with race detector |
| 3 | Boot | Server starts cleanly |
| 4 | Login | Returns valid JWT |
| 5 | Ingest test data | Logs and KV written |
| 6 | Query logs | total >= 2 |
| 7 | Query logs with filter | level=info filter works |
| 8 | KV found | count=1 |
| 9 | KV not found | count=0, no error |
| 10 | Schema query | 'level' field present in schema |
| 11 | No auth | 401 |
| 12 | Invalid filter | 400 |
| 13 | Graceful shutdown | Exit code 0 |

---

## EXECUTION ORDER SUMMARY

```
TASK 01  →  internal/query/types.go
TASK 02  →  internal/query/filter.go
TASK 03  →  internal/query/params.go
TASK 05  →  internal/storage/hot/manager.go (add ScanCF) ← must come before TASK 04
TASK 04  →  internal/query/engine.go
TASK 06  →  internal/query/handler.go
TASK 07  →  internal/server/server.go (register routes)
TASK 08  →  internal/query/filter_test.go
TASK 09  →  internal/query/engine_test.go
TASK 10  →  docs/api/query.md
TASK 11  →  smoke test — all 13 steps must pass
```

---

*Sprint 6 complete when TASK 11 passes with zero failures.*
*Plomvix — Built in India. Built for the world.*