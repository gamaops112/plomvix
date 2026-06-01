# Plomvix — Sprint 16 Task Plan
### For: DeepSeek V4 Pro Coding Agent
### Language: Go 1.22 | Module: github.com/plomvix/plomvix

> Execute tasks in exact order. Each task is atomic — one file or one concern.
> Do not skip ahead. Each task depends on the previous.
> Every task has a Verify step — do not proceed until it passes.

---

## CONTEXT

Sprints 1–15 are complete.

Recent UI state:
- Sprint 11 added the UI foundation.
- Sprint 12 added the Theme Engine and Developer Design Panel.
- Sprint 13 added cookie auth, login/logout UI, protected app routes, and the centralized frontend API client.
- `sprint13PatchCodePlan.md` migrated the existing Sprint 11–13 UI to Tailwind CSS + shadcn/ui.
- Sprint 14 added the Admin UI using the Tailwind/shadcn baseline.
- Sprint 15 added the Log Explorer UI using the same Tailwind/shadcn baseline.

Sprint 16 adds **Trace Storage + Trace Query API**. This is a **backend/API-only** sprint
that prepares Plomvix for OTLP trace ingestion in Sprint 17. Sprint 16 must not add
trace UI, frontend routes, Tailwind/shadcn work, or a new trace ingestion HTTP endpoint.

**What Sprint 16 delivers:**
- `internal/traces/` package with fixed Span model, validation, normalization, and helpers
- `traces` RocksDB column family
- Trace storage indexes inside the hot tier
- Atomic trace span writes using RocksDB write batches
- Hot tier methods:
  - `WriteSpan`
  - `WriteSpanPayload`
  - `GetTrace`
  - `SearchSpans`
- WAL data type `DataTypeTrace`
- WAL replay support for trace spans
- Query API:
  - `GET /query/traces/{trace_id}`
  - `GET /query/traces?service=&operation=&from=&to=&limit=&offset=`
- Trace query engine and handlers
- OpenAPI update for trace endpoints
- API docs in `docs/api/traces.md`
- Unit tests for trace validation, hot-tier trace storage, WAL replay, and query handlers
- Integration smoke tests for trace query auth and trace query behaviour

**What Sprint 16 does NOT do:**
- No OTLP receiver — Sprint 17
- No Prometheus remote write — Sprint 17
- No trace UI — deferred until after backend trace ingestion exists
- No frontend routes
- No Tailwind/shadcn tasks
- No new ingestion HTTP endpoint for traces
- No distributed tracing waterfall UI
- No trace cold-tier storage in Sprint 16
- No trace tiering policy in Sprint 16
- No schema inference changes required for traces; traces use a fixed model

---

## TRACE MODEL — READ BEFORE WRITING ANY CODE

A trace is a set of spans with the same `trace_id`. Each span is stored as one JSON
record using a fixed model. Sprint 17 OTLP ingestion will normalize incoming OTLP
spans into this model before writing to the WAL and hot tier.

**Span JSON shape:**

```json
{
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7",
  "parent_span_id": "",
  "service_name": "checkout-api",
  "operation_name": "POST /checkout",
  "start_time_unix_nano": 1700000000000000000,
  "end_time_unix_nano": 1700000000123000000,
  "duration_nano": 123000000,
  "attributes": {
    "http.method": "POST",
    "http.status_code": 200
  },
  "status": "ok"
}
```

**Validation rules:**
- `trace_id` must be exactly 32 lowercase or uppercase hex characters
- `span_id` must be exactly 16 lowercase or uppercase hex characters
- `parent_span_id` may be empty; if present, it must be exactly 16 hex characters
- `service_name` must not be empty and must not contain a NUL byte
- `operation_name` must not be empty and must not contain a NUL byte
- `start_time_unix_nano` must be greater than 0
- `end_time_unix_nano` may be 0 for in-progress spans; if non-zero, it must be greater than or equal to `start_time_unix_nano`
- `duration_nano` may be 0; if 0 and end time is present, compute it as `end - start`
- `duration_nano` must not be negative
- `status` must be one of `unset`, `ok`, or `error`; empty status normalizes to `unset`
- `attributes` may be missing or null; normalize it to an empty map

**Precision rule:** Never convert Unix nanosecond fields through `float64` or through
`map[string]interface{}` produced by JSON marshal/unmarshal. Preserve `int64` values
when converting spans to response maps.

---

## HOT TIER TRACE INDEX DESIGN — READ BEFORE WRITING ANY CODE

Sprint 16 stores trace spans in the existing hot tier RocksDB database using a new
column family:

```go
const CFTraces = "traces"
```

Each span write creates multiple records inside the `traces` column family. The span
JSON is stored once at the primary span key. Index entries store the primary span key
as their value.

**Key prefixes:**

| Key type | Format | Value |
|---|---|---|
| Primary span | `span:{trace_id}:{span_id}` | Span JSON |
| Trace index | `trace:{trace_id}:{start_ns_be_8bytes}:{span_id}` | Primary span key |
| Global time index | `time:{start_ns_be_8bytes}:{trace_id}:{span_id}` | Primary span key |
| Service time index | `svc:{service_name}\x00{start_ns_be_8bytes}:{trace_id}:{span_id}` | Primary span key |

**Exact prefix rules:**
- Trace lookup prefix must be exactly `trace:{trace_id}:`
- Global time search keep-prefix must be exactly `time:`
- Service search keep-prefix must be exactly `svc:{service_name}\x00`
- Service names and operation names reject NUL bytes so prefix scans cannot be escaped or made ambiguous

**Timestamp offset rules:**
- In `time:` keys, the 8-byte timestamp starts at byte offset `len("time:")`
- In `svc:` keys, the 8-byte timestamp starts at byte offset `len("svc:") + len(service_name) + 1`
- Use these offsets when extracting timestamps from index keys for tests or range bounds

**Atomicity requirement:**
A span write must use a RocksDB write batch so primary and index entries are written
together. Do not write the four keys one-by-one with separate `Put` calls.

**Range scan requirement:**
Existing hot-tier prefix scan helpers may only support exact-prefix scans. Time-range
trace search needs to seek at a lower-bound key, then continue while keys still share
a static keep-prefix (`time:` or `svc:{service}\x00`). Add a dedicated scan helper for
this instead of misusing an exact prefix like `time:{from_ns}`.

**Existing database note:**
Sprint 16 adds a new column family to an existing RocksDB database. The existing hot-tier
options must keep `CreateMissingColumnFamilies(true)`. If local development data fails
to open because an older RocksDB instance lacks the new column family, delete `data/hot/`
for local testing only. Do not add destructive automatic deletion logic.

---

## TRACE QUERY API DESIGN — READ BEFORE WRITING ANY CODE

Trace query endpoints are protected by the same auth middleware as existing query
endpoints. Browser users authenticate with Sprint 13 cookies. API clients may use JWT
bearer tokens or API keys.

| Method | Path | Auth | Purpose |
|---|---|---|---|
| `GET` | `/query/traces/{trace_id}` | JWT cookie, bearer JWT, or API key | Return all spans in one trace |
| `GET` | `/query/traces` | JWT cookie, bearer JWT, or API key | Search spans by service, operation, and time range |

**`GET /query/traces/{trace_id}` response data shape:**

```json
{
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "spans": [],
  "count": 0,
  "services": [],
  "duration_nano": 0,
  "query_ms": 1
}
```

Return HTTP 200 with an empty `spans` array if the trace does not exist. Do not use
404 for missing traces in Sprint 16.

**`GET /query/traces` query params:**

| Param | Type | Notes |
|---|---|---|
| `service` | string | Optional service name filter; NUL byte rejected |
| `operation` | string | Optional operation name filter; NUL byte rejected |
| `from` | Unix nanoseconds string | Optional; `0` means beginning of time |
| `to` | Unix nanoseconds string | Optional; defaults to now if omitted |
| `limit` | positive integer | Default 100, max 10000 |
| `offset` | non-negative integer | Number of matching spans to skip |

Search results use the existing `QueryResult` envelope with `data_type: "traces"`.
Each record is one span represented as a JSON object with nanosecond fields preserved
as integer values.

---

## TASK 01 — Confirm existing backend file names and route patterns

**Action:** Inspect the current codebase and identify the exact files and names for:
- WAL data type constants in `internal/storage/wal/types.go`
- hot tier column family constants in `internal/storage/hot/types.go`
- hot tier `Store` and `Manager` methods
- query `Engine` and `Handler`
- query route registration in `internal/server/server.go`
- OpenAPI file location `api/openapi.json`
- integration test helper location, if present

If a name differs slightly from this plan, adapt to the existing codebase while
preserving the behaviour specified here.

**Verify:** No code changes required. Write the discovered paths/names in terminal
output or a temporary note.

---

## TASK 02 — Create internal/traces directory and model.go

**Action — Part A:** Create the package directory:

```bash
mkdir -p internal/traces
```

**Action — Part B:** Create `internal/traces/model.go`.

**Requirements:**
- Package name: `traces`
- Define typed status constants:
  ```go
  type SpanStatus string

  const (
      StatusUnset SpanStatus = "unset"
      StatusOK    SpanStatus = "ok"
      StatusError SpanStatus = "error"
  )
  ```
- Define `Span` struct matching the JSON shape in `TRACE MODEL`
- Define `SearchParams` and `SearchResult` for hot-tier search
- Define:
  ```go
  const DefaultSearchLimit = 100
  const MaxSearchLimit = 10000
  ```
- Do not define the HTTP `TraceResult` response type here; that type is added in `internal/query/types.go` so query response types stay in the query package

**SearchParams fields:**
```go
type SearchParams struct {
    Service   string
    Operation string
    FromNs    int64
    ToNs      int64
    Limit     int
    Offset    int
}
```

**SearchResult fields:**
```go
type SearchResult struct {
    Spans  []*Span
    Total  int
    Limit  int
    Offset int
}
```

**Verify:** `go build ./internal/traces/` compiles with no errors.

---

## TASK 03 — Create internal/traces/validate.go

**Action:** Create `internal/traces/validate.go`.

**Imports required:**
```go
import (
    "errors"
    "fmt"
    "regexp"
    "strings"
)
```

**Sentinel errors:**
```go
var (
    ErrInvalidSpan         = errors.New("invalid trace span")
    ErrInvalidSearchParams = errors.New("invalid trace search params")
)
```

**Functions to implement:**
```go
func ValidateSpan(s *Span) error
func NormalizeSpan(s *Span) error
func IsTraceID(v string) bool
func IsSpanID(v string) bool
func ValidateSearchParams(p SearchParams) error
```

**Behaviour:**
- `NormalizeSpan` mutates the span in place:
  - lowercases `trace_id`, `span_id`, and non-empty `parent_span_id`
  - normalizes empty status to `unset`
  - initializes nil attributes to `map[string]interface{}{}`
  - computes duration when `duration_nano == 0` and end time is present
- `ValidateSpan` validates all rules from `TRACE MODEL`
- `ValidateSearchParams` rejects:
  - NUL byte in `Service` or `Operation`
  - negative `FromNs` or `ToNs`
  - `FromNs >= ToNs` when both are greater than 0
  - `Limit <= 0` or `Limit > MaxSearchLimit`
  - `Offset < 0`
- Return errors using `fmt.Errorf("%w: ...", ErrInvalidSpan)` or `fmt.Errorf("%w: ...", ErrInvalidSearchParams)` so handlers can use `errors.Is`

**Verify:** `go build ./internal/traces/` compiles with no errors.

---

## TASK 04 — Create internal/traces/map.go

**Action:** Create `internal/traces/map.go`.

**Function to implement:**
```go
func SpanToMap(s *Span) map[string]interface{}
```

**Critical precision requirement:**
Do not implement this using `json.Marshal` followed by `json.Unmarshal` into
`map[string]interface{}` because that converts large Unix nanosecond values into
`float64` and loses precision. Build the map manually and preserve all timestamp and
duration fields as `int64`.

**Verify:** `go build ./internal/traces/` compiles with no errors.

---

## TASK 05 — Create internal/traces/validate_test.go

**Action:** Create `internal/traces/validate_test.go`.

**Tests required:**
- valid span passes
- trace ID wrong length fails
- trace ID non-hex fails
- span ID wrong length fails
- parent span ID invalid fails
- empty service name fails
- service name with NUL byte fails
- empty operation name fails
- operation name with NUL byte fails
- start time `0` fails
- end time before start fails
- negative duration fails
- empty status normalizes to `unset`
- nil attributes normalize to empty map
- duration is computed from start/end when missing
- invalid search params return `ErrInvalidSearchParams`
- `SpanToMap` preserves nanosecond fields as integer values, not float values

**Verify:** `go test ./internal/traces/` passes.

---

## TASK 06 — Add traces column family to internal/storage/hot/types.go

**Action:** Update `internal/storage/hot/types.go`.

**Change 1 — Add constant:**
```go
CFTraces = "traces"
```

**Change 2 — Add `CFTraces` to `AllColumnFamilies()` after existing data CFs.**
Keep `default` first. Do not remove existing CFs such as `_meta`.

**Verify:** `CGO_ENABLED=1 go build ./internal/storage/hot/` compiles with no errors.

---

## TASK 07 — Create internal/storage/hot/trace_keys.go

**Action:** Create `internal/storage/hot/trace_keys.go`.

**Imports required:**
```go
import "encoding/binary"
```

**Functions to implement:**
```go
func BuildTracePrimaryKey(traceID, spanID string) []byte
func BuildTraceIndexPrefix(traceID string) []byte
func BuildTraceIndexKey(traceID string, startNs int64, spanID string) []byte
func BuildTimeIndexPrefix() []byte
func BuildTimeIndexSeekKey(fromNs int64) []byte
func BuildTimeIndexKey(startNs int64, traceID, spanID string) []byte
func BuildServiceIndexPrefix(service string) []byte
func BuildServiceIndexSeekKey(service string, fromNs int64) []byte
func BuildServiceIndexKey(service string, startNs int64, traceID, spanID string) []byte
func traceIndexTimestampOffset() int
func serviceIndexTimestampOffset(service string) int
```

**Key rules:**
- `BuildTraceIndexPrefix(traceID)` returns exactly `[]byte("trace:" + traceID + ":")`
- `BuildTimeIndexPrefix()` returns exactly `[]byte("time:")`
- `BuildServiceIndexPrefix(service)` returns exactly `[]byte("svc:" + service + "\x00")`
- `BuildTimeIndexSeekKey(0)` returns `BuildTimeIndexPrefix()`
- `BuildServiceIndexSeekKey(service, 0)` returns `BuildServiceIndexPrefix(service)`
- Use big-endian uint64 encoding for timestamp bytes
- Do not validate service NUL here; validation happens in `internal/traces`

**Verify:** `CGO_ENABLED=1 go build ./internal/storage/hot/` compiles with no errors.

---

## TASK 08 — Create internal/storage/hot/trace_keys_test.go

**Action:** Create `internal/storage/hot/trace_keys_test.go`.

**Tests required:**
- primary key matches `span:{trace_id}:{span_id}`
- trace prefix matches `trace:{trace_id}:`
- global time index key sorts older timestamp before newer timestamp
- service index prefix contains exactly one NUL separator after service name
- service index key sorts older timestamp before newer timestamp for the same service
- timestamp offsets point to the 8 timestamp bytes in both time and service keys

**Verify:** `CGO_ENABLED=1 go test ./internal/storage/hot/ -run TraceKey` passes.

---

## TASK 09 — Add write-batch support to hot Store

**Action:** Update `internal/storage/hot/store.go`.

**Add types:**
```go
type BatchPut struct {
    CF    string
    Key   []byte
    Value []byte
}
```

**Add method:**
```go
func (s *Store) PutBatch(puts []BatchPut) error
```

**Behaviour:**
- Create a RocksDB write batch
- For each put:
  - resolve column family handle with existing `cfHandle`
  - call `batch.PutCF(handle, key, value)`
- Write the batch once using existing write options
- Increment write stats by the number of puts only after batch write succeeds
- Destroy/free the write batch after use
- Return descriptive errors with CF name when a handle is missing

**Verify:** `CGO_ENABLED=1 go build ./internal/storage/hot/` compiles with no errors.

---

## TASK 10 — Add lower-bound prefix scan support to hot Store

**Action:** Update `internal/storage/hot/store.go`.

**Add method:**
```go
func (s *Store) ScanFrom(cf string, seekKey, keepPrefix []byte, fn func(key, value []byte) bool) error
```

**Behaviour:**
- Resolve CF handle
- Create read options and iterator like existing `Scan`
- `it.Seek(seekKey)`
- Continue while iterator is valid and `bytes.HasPrefix(key, keepPrefix)`
- Copy key/value bytes before freeing iterator slices
- Stop if `fn` returns false
- Return iterator error if the RocksDB binding exposes one in the existing codebase pattern

**Why:** Trace time-range search must seek at `time:{from_ns}` or `svc:{service}\x00{from_ns}` but continue while the static prefix is still `time:` or `svc:{service}\x00`.

**Verify:** `CGO_ENABLED=1 go build ./internal/storage/hot/` compiles with no errors.

---

## TASK 11 — Expose batch and scan helpers through hot Manager

**Action:** Update `internal/storage/hot/manager.go`.

**Add unexported helper methods if needed by trace storage:**
```go
func (m *Manager) putBatch(puts []BatchPut) error
func (m *Manager) scanFromCF(cf string, seekKey, keepPrefix []byte, fn func(key, value []byte) bool) error
```

If existing `Manager` already exposes similar wrappers, use those instead.
Keep these helpers unexported unless tests require exported access.

**Verify:** `CGO_ENABLED=1 go build ./internal/storage/hot/` compiles with no errors.

---

## TASK 12 — Create internal/storage/hot/traces.go

**Action:** Create `internal/storage/hot/traces.go`.

**Imports required:**
```go
import (
    "encoding/json"
    "fmt"
    "sort"
    "time"

    "github.com/plomvix/plomvix/internal/traces"
)
```

**Public methods to implement on `Manager`:**
```go
func (m *Manager) WriteSpan(span *traces.Span) error
func (m *Manager) WriteSpanPayload(payload []byte) error
func (m *Manager) GetTrace(traceID string) ([]*traces.Span, error)
func (m *Manager) SearchSpans(params traces.SearchParams) (*traces.SearchResult, error)
```

**Private helpers to implement as needed:**
```go
func (m *Manager) getSpanByPrimaryKey(primaryKey []byte) (*traces.Span, error)
func decodeSpan(raw []byte) (*traces.Span, error)
```

**WriteSpan behaviour:**
1. Call `traces.NormalizeSpan(span)` and validate.
2. Marshal the normalized span to JSON.
3. Build primary key and three index keys.
4. Use one `PutBatch` call to write:
   - primary key → span JSON
   - trace index key → primary key
   - global time index key → primary key
   - service time index key → primary key
5. Return descriptive errors.

**WriteSpanPayload behaviour:**
- Unmarshal payload into `traces.Span`
- Call `WriteSpan(&span)`
- Used by WAL replay and future OTLP ingestion

**GetTrace behaviour:**
- Validate `traceID` with `traces.IsTraceID`; invalid IDs return an error
- Scan `CFTraces` using exact prefix `BuildTraceIndexPrefix(traceID)`
- Each index value is a primary span key; fetch the span via `getSpanByPrimaryKey`
- Sort returned spans by `StartTimeUnixNano` ascending, then `SpanID` ascending
- Return an empty slice, not nil, if no spans exist

**SearchSpans behaviour:**
1. If `ToNs == 0`, set it to `time.Now().UnixNano()`.
2. If `Limit == 0`, set it to `traces.DefaultSearchLimit`.
3. Validate params with `traces.ValidateSearchParams`.
4. Choose index:
   - service set → seek at `BuildServiceIndexSeekKey(service, from)` and keep prefix `BuildServiceIndexPrefix(service)`
   - service empty → seek at `BuildTimeIndexSeekKey(from)` and keep prefix `BuildTimeIndexPrefix()`
5. During scan, extract timestamp from the index key and stop when `ToNs > 0 && ts >= ToNs`.
6. Fetch primary spans from index values.
7. Apply in-memory operation filter after fetching spans.
8. Sort by `StartTimeUnixNano` ascending, then `TraceID`, then `SpanID`.
9. Compute `Total` before pagination.
10. Apply `Offset` and `Limit`.
11. Return `SearchResult` with non-nil `Spans` slice.

**Verify:** `CGO_ENABLED=1 go build ./internal/storage/hot/` compiles with no errors.

---

## TASK 13 — Create internal/storage/hot/traces_test.go

**Action:** Create `internal/storage/hot/traces_test.go`.

**Tests required:**
- `WriteSpan` then `GetTrace` returns the span
- `WriteSpan` normalizes uppercase trace/span IDs to lowercase
- `WriteSpanPayload` writes valid JSON payload
- `GetTrace` returns spans sorted by start time
- `SearchSpans` without service returns time-range matches
- `SearchSpans` with service scans only that service
- `SearchSpans` with operation filters in memory
- `SearchSpans` computes `Total` before pagination
- `SearchSpans` respects `limit` and `offset`
- invalid service or operation NUL returns `traces.ErrInvalidSearchParams`

**Verify:** `CGO_ENABLED=1 go test ./internal/storage/hot/ -run Trace` passes.

---

## TASK 14 — Add WAL DataTypeTrace

**Action:** Update `internal/storage/wal/types.go`.

**Change:** Add trace data type after existing values:
```go
DataTypeTrace DataType = 5
```

Do not renumber existing data type constants. Existing WAL files depend on current
values for logs, metrics, JSON, and KV.

**Verify:** `go build ./internal/storage/wal/` compiles with no errors.

---

## TASK 15 — Add trace WAL replay support to hot tier

**Action:** Update the existing WAL replay logic in `internal/storage/hot/manager.go`
or the file where `ReplayWAL` is implemented.

**Requirement:** Add a case for `wal.DataTypeTrace`:
```go
case wal.DataTypeTrace:
    if err := m.WriteSpanPayload(entry.Payload); err != nil {
        return ..., err
    }
```

Use the existing replay result/stats pattern. Do not add a trace HTTP ingestion endpoint.

**Verify:** `CGO_ENABLED=1 go build ./internal/storage/hot/` compiles with no errors.

---

## TASK 16 — Add WAL replay tests for traces

**Action:** Update or create hot-tier replay tests.

**Tests required:**
- A WAL entry with `DataTypeTrace` replays into trace storage
- After replay, `GetTrace(traceID)` returns the span
- Invalid trace payload during replay returns an error using the existing replay error style
- Existing replay tests for logs/metrics/json/kv still pass

**Verify:** `CGO_ENABLED=1 go test ./internal/storage/hot/ -run Replay` passes.

---

## TASK 17 — Add TraceResult to internal/query/types.go

**Action:** Update `internal/query/types.go`.

**Add response type:**
```go
type TraceResult struct {
    TraceID      string                   `json:"trace_id"`
    Spans        []map[string]interface{} `json:"spans"`
    Count        int                      `json:"count"`
    Services     []string                 `json:"services"`
    DurationNano int64                    `json:"duration_nano"`
    QueryMs      int64                    `json:"query_ms"`
}
```

**Rules:**
- Keep HTTP/query response types in the query package
- Do not duplicate this type in `internal/traces`

**Verify:** `go build ./internal/query/` compiles with no errors.

---

## TASK 18 — Create internal/query/traces.go

**Action:** Create `internal/query/traces.go`.

**Imports required:**
```go
import (
    "sort"
    "time"

    "github.com/plomvix/plomvix/internal/traces"
)
```

**Methods to implement on `Engine`:**
```go
func (e *Engine) QueryTraceByID(traceID string) (*TraceResult, error)
func (e *Engine) SearchTraces(params traces.SearchParams) (*QueryResult, error)
```

**QueryTraceByID behaviour:**
- Start timer
- Call `e.store.GetTrace(traceID)`
- Convert spans with `traces.SpanToMap`
- Compute unique service names sorted ascending
- Compute trace duration as max end/start timestamp minus min start timestamp:
  - for completed spans use `EndTimeUnixNano` when non-zero
  - for in-progress spans use `StartTimeUnixNano` as that span's endpoint
  - if no spans, duration is 0
- Return HTTP response data shape from `TRACE QUERY API DESIGN`
- Missing trace returns empty spans and 200, not error

**SearchTraces behaviour:**
- Start timer
- Call `e.store.SearchSpans(params)`
- Convert each span with `traces.SpanToMap`
- Return `QueryResult` with:
  - `DataType: "traces"`
  - `Records`: converted spans
  - `Count`: page count
  - `Total`: result total before pagination
  - `Limit`, `Offset`: from search result
  - `QueryMs`: elapsed milliseconds

**Verify:** `go build ./internal/query/` compiles with no errors.

---

## TASK 19 — Create internal/query/traces_params.go

**Action:** Create `internal/query/traces_params.go`.

**Imports required:**
```go
import (
    "fmt"
    "net/http"
    "strconv"
    "strings"
    "time"

    "github.com/plomvix/plomvix/internal/traces"
)
```

**Function to implement:**
```go
func ParseTraceSearchParams(r *http.Request) (traces.SearchParams, error)
```

**Behaviour:**
- Parse `service`, `operation`, `from`, `to`, `limit`, and `offset`
- Trim spaces from service and operation
- Default `ToNs` to `time.Now().UnixNano()` when `to` is absent or empty
- Default `Limit` to `traces.DefaultSearchLimit`
- Clamp nothing silently except no clamp; if `limit > traces.MaxSearchLimit`, return validation error
- Return `traces.ErrInvalidSearchParams` wrapped error for invalid values so handlers can map it to 400
- Do not parse timestamps as floats

**Verify:** `go build ./internal/query/` compiles with no errors.

---

## TASK 20 — Add trace query handlers

**Action:** Update the existing query handler file, or create `internal/query/trace_handler.go`
if handlers are split by file.

**Handlers to implement:**
```go
func (h *Handler) GetTrace(w http.ResponseWriter, r *http.Request)
func (h *Handler) SearchTraces(w http.ResponseWriter, r *http.Request)
```

**GetTrace behaviour:**
- Read `trace_id` from URL path using existing router pattern
- Validate with `traces.IsTraceID`
- Invalid trace ID returns HTTP 400 with `utils.CodeValidationFailed`
- Call `engine.QueryTraceByID`
- Return `utils.OK`

**SearchTraces behaviour:**
- Parse params with `ParseTraceSearchParams`
- If `errors.Is(err, traces.ErrInvalidSearchParams)`, return HTTP 400 with `utils.CodeValidationFailed`
- Other unexpected errors return HTTP 500
- Call `engine.SearchTraces`
- Return `utils.OK`

**Imports:** include `errors` only if needed by `SearchTraces`.

**Verify:** `go build ./internal/query/` compiles with no errors.

---

## TASK 21 — Register trace query routes in internal/server/server.go

**Action:** Update the existing authenticated query route group in `internal/server/server.go`.

**Routes to add:**
```go
r.Get("/query/traces", queryHandler.SearchTraces)
r.Get("/query/traces/{trace_id}", queryHandler.GetTrace)
```

**Rules:**
- Both routes must be inside the existing query/auth middleware group
- They must accept Sprint 13 cookie auth, bearer JWT, and API key through the existing middleware
- Do not register these routes as public
- Do not add `/app/traces` or any frontend route
- Register the exact `/query/traces` route before the parameterized route for readability

**Verify:** `CGO_ENABLED=1 go build ./internal/server/` compiles with no errors.

---

## TASK 22 — Create internal/query/traces_test.go

**Action:** Create `internal/query/traces_test.go`.

**Tests required:**
- parse params defaults `to` and `limit`
- parse params rejects float timestamps
- parse params rejects negative `from`, `to`, and offset
- parse params rejects `from >= to`
- parse params rejects limit above max
- parse params rejects NUL service and operation
- `QueryTraceByID` returns empty result for missing trace
- `QueryTraceByID` computes services sorted ascending
- `QueryTraceByID` computes duration using min start and max end
- `SearchTraces` returns `QueryResult` with `data_type: "traces"`
- handlers return 400 for invalid trace ID/search params

**Verify:** `go test ./internal/query/` passes.

---

## TASK 23 — Update api/openapi.json for trace query endpoints

**Action:** Update `api/openapi.json`.

**Add schemas:**
- `TraceSpan`
- `TraceResult`

**TraceSpan schema fields:**
- `trace_id`: string
- `span_id`: string
- `parent_span_id`: string
- `service_name`: string
- `operation_name`: string
- `start_time_unix_nano`: integer, format `int64`
- `end_time_unix_nano`: integer, format `int64`
- `duration_nano`: integer, format `int64`
- `attributes`: object
- `status`: string enum `["unset", "ok", "error"]`

**Add paths:**
- `GET /query/traces/{trace_id}`
- `GET /query/traces`

**Security:** both routes use `[{"BearerAuth":[]},{"APIKeyAuth":[]}]`.

**Responses:** include 200, 400, 401, and 500 responses using existing response schemas.

**Important:** Do not document a trace ingestion endpoint in Sprint 16.

**Verify:**
```bash
cat api/openapi.json | python3 -m json.tool > /dev/null
! grep -R "\.\.\.\|TODO\|PLACEHOLDER" api/openapi.json
grep -n '"/query/traces"' api/openapi.json
grep -n '"/query/traces/{trace_id}"' api/openapi.json
```

---

## TASK 24 — Create docs/api/traces.md

**Action:** Create `docs/api/traces.md`.

**Document:**
- Sprint 16 trace scope
- Span JSON model
- Validation rules
- Hot tier trace indexes at a high level
- `GET /query/traces/{trace_id}`
- `GET /query/traces`
- Auth requirements
- Example requests and responses
- Note that traces are written internally in Sprint 16 tests/WAL replay only; OTLP ingestion arrives in Sprint 17
- Note that there is no trace UI in Sprint 16

**Verify:**
```bash
grep -n "GET /query/traces" docs/api/traces.md
grep -n "trace_id" docs/api/traces.md
grep -n "Sprint 17" docs/api/traces.md
```

---

## TASK 25 — Update README.md with trace query note

**Action:** Update `README.md`.

**Add:**
- Brief trace storage/query overview
- Mention `GET /query/traces/{trace_id}` and `GET /query/traces`
- Link/reference to `docs/api/traces.md`
- Mention OTLP receiver is Sprint 17, not Sprint 16

**Verify:**
```bash
grep -n "Trace" README.md
grep -n "query/traces" README.md
grep -n "OTLP" README.md
```

---

## TASK 26 — Add trace integration helper without breaking existing tests

**Action:** Update integration test helpers only if Sprint 10 integration scaffolding exists.

**Requirement:** Preserve the existing `testServer(t)` signature so old integration tests do not break.

**Add a new helper instead of changing the old one:**
```go
func testServerWithHot(t *testing.T) (baseURL string, hot *hotstore.Manager, cleanup func())
```

**Behaviour:**
- Internally share setup code with `testServer(t)` if possible
- Return the hot manager so trace tests can seed spans directly with `hot.WriteSpan(...)`
- Keep cleanup responsible for shutting down server, stores, blacklist, logger, and temp data

If integration test scaffolding does not exist, skip this task and document integration tests as deferred in `docs/api/traces.md`.

**Verify:** `CGO_ENABLED=1 go test -race ./tests/integration/...` passes if integration tests exist.

---

## TASK 27 — Add trace integration tests

**Action:** Create `tests/integration/traces_test.go` if integration scaffolding exists.

**Tests required:**
- unauthenticated `GET /query/traces/{trace_id}` returns 401
- unauthenticated `GET /query/traces` returns 401
- seed two spans through returned hot manager, then query by trace ID with admin token
- query by service returns only matching service
- query by operation returns only matching operation
- missing trace ID returns 200 with empty spans
- invalid trace ID returns 400

**Important:** Since Sprint 16 has no trace ingestion HTTP endpoint, tests must seed data through hot-tier methods, not through a fake HTTP ingest route.

**Verify:** `CGO_ENABLED=1 go test -race ./tests/integration/...` passes.

---

## TASK 28 — Run Go formatting and package tests

**Action:**
```bash
find . -name '*.go' -not -path './vendor/*' -exec gofmt -w {} +
CGO_ENABLED=1 go test ./internal/traces/...
CGO_ENABLED=1 go test ./internal/storage/wal/...
CGO_ENABLED=1 go test ./internal/storage/hot/...
CGO_ENABLED=1 go test ./internal/query/...
CGO_ENABLED=1 go test ./internal/server/...
```

**Verify:** All commands exit with code 0.

---

## TASK 29 — Run full backend tests

**Action:**
```bash
CGO_ENABLED=1 go test ./...
```

**Verify:** Full test suite exits with code 0.

---

## TASK 30 — Run OpenAPI and docs checks

**Action:**
```bash
cat api/openapi.json | python3 -m json.tool > /dev/null
! grep -R "\.\.\.\|TODO\|PLACEHOLDER" api/openapi.json docs/api/traces.md README.md
grep -n '"/query/traces"' api/openapi.json
grep -n '"/query/traces/{trace_id}"' api/openapi.json
grep -n "GET /query/traces" docs/api/traces.md
```

**Verify:** All commands exit with code 0.

---

## TASK 31 — Run UI build sanity check

**Action:** Sprint 16 does not modify the frontend, but the repository must still build after backend changes:

```bash
make ui-build
```

If the project uses direct npm commands instead of `make ui-build`, run:

```bash
cd ui
npm run typecheck
npm run build
```

**Verify:** UI typecheck/build exits with code 0.

---

## TASK 32 — Full build and smoke test

**Action:** Run the following verification sequence from project root:

```bash
#!/bin/bash
set -euo pipefail

TMP_DIR=$(mktemp -d)
SERVER_PID=""
cleanup() {
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        kill -SIGTERM "$SERVER_PID" 2>/dev/null || true
        wait "$SERVER_PID" 2>/dev/null || true
    fi
    rm -rf "$TMP_DIR"
}
trap cleanup EXIT

export PLOMVIX_ENV="development"
export PLOMVIX_STORAGE_DATA_DIR="$TMP_DIR/data"
export PLOMVIX_THEME_PATH="$TMP_DIR/theme.json"
export PLOMVIX_UI_ENABLED="true"
export PLOMVIX_UI_DEV_MODE="false"

echo "=== Step 1: Backend + UI build ==="
CGO_ENABLED=1 make build

echo "=== Step 2: Run tests ==="
CGO_ENABLED=1 make test

echo "=== Step 3: Boot server ==="
./plomvix > "$TMP_DIR/plomvix_s16.log" 2>&1 &
SERVER_PID=$!
sleep 3

echo "=== Step 4: Public health still works ==="
curl -sf http://localhost:8080/health >/dev/null \
    && echo "PASS: health endpoint works" \
    || { echo "FAIL: health endpoint failed"; exit 1; }

echo "=== Step 5: Trace query requires auth ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    http://localhost:8080/query/traces/4bf92f3577b34da6a3ce929d0e0e4736)
[ "$STATUS" -eq 401 ] && echo "PASS: trace by ID requires auth" \
    || { echo "FAIL: expected 401 for trace by ID without auth, got $STATUS"; exit 1; }

STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    'http://localhost:8080/query/traces?service=checkout-api')
[ "$STATUS" -eq 401 ] && echo "PASS: trace search requires auth" \
    || { echo "FAIL: expected 401 for trace search without auth, got $STATUS"; exit 1; }

echo "=== Step 6: UI still loads ==="
curl -sf http://localhost:8080/app/ | grep -qi "plomvix" \
    && echo "PASS: UI app loads" \
    || { echo "FAIL: UI app did not load"; exit 1; }

echo "Sprint 16 smoke test PASSED"
```

**Verify:** Script completes with `Sprint 16 smoke test PASSED`.

---

## TASK 33 — Final lint and repository check

**Action:**
```bash
CGO_ENABLED=1 make lint
CGO_ENABLED=1 make build
git status --short
```

**Verify:**
- `make lint` exits with code 0
- `make build` exits with code 0
- `git status --short` shows only intentional Sprint 16 files changed

---

## TASK 34 — Final grep guard against out-of-scope trace UI or ingest endpoints

**Action:** Run:
```bash
! grep -R '"/app/traces"\|/app/traces\|Trace UI\|trace waterfall' ui internal docs README.md api || true
! grep -R 'POST /ingest/traces\|/ingest/traces\|IngestTraces' internal docs README.md api || true
```

**IMPORTANT:** If these commands print matches that are not in this Sprint 16 plan's
negative-scope notes, remove the out-of-scope code or docs. Sprint 16 must not add
trace UI or trace ingestion endpoints.

**Verify:** No out-of-scope trace UI or trace ingest implementation remains.

---

## TASK 35 — Final full verification

**Action:** Run:
```bash
find . -name '*.go' -not -path './vendor/*' -exec gofmt -w {} +
CGO_ENABLED=1 go test ./...
make ui-build
CGO_ENABLED=1 make lint
CGO_ENABLED=1 make build
```

**Verify:** Every command exits with code 0.

---

## FINAL SPRINT 16 ACCEPTANCE CHECKLIST

- `internal/traces/` exists with model, validation, normalization, and tests
- `DataTypeTrace` exists in WAL without renumbering existing data types
- `CFTraces` exists in hot tier without removing existing column families
- Trace span writes are atomic via RocksDB write batch
- Trace index keys follow the exact prefix and timestamp offset rules
- Time-range trace search uses lower-bound seek plus static keep-prefix scan
- `WriteSpan`, `WriteSpanPayload`, `GetTrace`, and `SearchSpans` exist
- WAL replay handles `DataTypeTrace`
- `TraceResult` lives in `internal/query`, not `internal/traces`
- `GET /query/traces/{trace_id}` requires auth and returns trace spans
- `GET /query/traces` requires auth and searches spans by service/operation/time
- Invalid trace IDs and invalid search params return HTTP 400
- Missing trace returns HTTP 200 with empty spans
- Nanosecond fields are preserved as integers, not float values
- OpenAPI includes trace query endpoints and int64 timestamp fields
- `docs/api/traces.md` exists
- README mentions trace query support and Sprint 17 OTLP scope
- No trace UI route exists
- No trace ingest HTTP endpoint exists
- Existing Tailwind/shadcn UI still builds, but Sprint 16 does not modify frontend UI
- `CGO_ENABLED=1 go test ./...` passes
- `make ui-build` passes
- `CGO_ENABLED=1 make lint` passes
- `CGO_ENABLED=1 make build` passes
