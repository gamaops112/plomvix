# Plomvix — Sprint 8 Task Plan
### For: DeepSeek V4 Pro Coding Agent
### Language: Go 1.22 | Module: github.com/plomvix/plomvix

> Execute tasks in exact order. Each task is atomic — one file or one concern.
> Do not skip ahead. Each task depends on the previous.
> Every task has a Verify step — do not proceed until it passes.

---

## CONTEXT

Sprints 1–7 are complete. Sprint 8 adds **Multi-Format Parsers** — the ability to
ingest log and JSON data in formats beyond JSON. The existing ingest endpoints gain
Content-Type negotiation while preserving all Sprint 5 JSON request contracts.

**What Sprint 8 delivers:**
- `Parser` interface — all parsers implement a common contract
- JSON parser — formalises the existing JSON path and preserves `{"records":[...]}` compatibility
- CSV parser — parses CSV rows into JSON records, first row is header
- Logfmt parser — parses `key=value` pairs per line
- Syslog parser — parses RFC 5424 and RFC 3164 syslog lines
- Parser registry — maps Content-Type strings to parsers
- Content-Type negotiation on `/ingest/logs` and `/ingest/json`
- JSON-only behaviour preserved for `/ingest/metrics` and `/ingest/kv`
- 400 responses for parse errors, empty input, and unsupported non-JSON formats on metrics/KV
- Tests for every parser
- No new HTTP endpoints — existing endpoints gain new Content-Type support where appropriate
- No changes to WAL, hot tier, cold tier, or query engine

**Content-Type to parser mapping:**

| Content-Type | Parser | Used with |
|---|---|---|
| `application/json` | JSON | all ingest endpoints |
| `text/csv` | CSV | `/ingest/logs`, `/ingest/json` |
| `text/x-logfmt` | Logfmt | `/ingest/logs` |
| `application/x-syslog` | Syslog | `/ingest/logs` |

**Default:** if Content-Type is absent or unrecognised, fall back to `application/json`
for backward compatibility with existing clients.

**What Sprint 8 does NOT do:**
- No OTLP/protobuf — requires protobuf dependency, deferred to Sprint 9
- No Prometheus exposition format — deferred
- No schema changes — all parsers normalise to JSON-compatible maps
- No reconciliation tooling for Sprint 7 partial hot+cold failures — deferred beyond Sprint 8
- No cold-tier, WAL, hot-tier, or query-engine refactor

**Parse output contract:**
Every parser returns `[]map[string]interface{}` — a slice of records where each
record is a flat or JSON-compatible map. The ingest handler marshals each record
to JSON and writes it to the WAL and hot tier exactly as before.

**Backward compatibility requirement:**
Sprint 5 clients send `{"records":[...]}`. Sprint 8 must continue accepting that
shape on all JSON ingest endpoints. For `/ingest/logs` and `/ingest/json`, the new
JSON parser must also accept a bare object and a bare array as convenience formats.

---

## PARSER DESIGN — READ BEFORE WRITING ANY CODE

### Parser interface

```go
// Parser converts raw bytes into a slice of records.
// Each record is a JSON-compatible map that will be JSON-marshalled before ingestion.
// Returns ErrEmptyInput if input is empty or contains no records.
// Returns ErrMalformedInput for structurally invalid input.
type Parser interface {
    Parse(data []byte) ([]map[string]interface{}, error)
    ContentType() string // canonical Content-Type this parser handles
}
```

### Sentinel errors

```go
var (
    ErrEmptyInput     = errors.New("input is empty")
    ErrMalformedInput = errors.New("malformed input")
    ErrUnsupportedContentType = errors.New("unsupported content type")
)
```

### JSON format

Accepted forms:

```json
{"records":[{"level":"info"}]}
[{"level":"info"}]
{"level":"info"}
```

Rules:
- `{"records":[...]}` is the Sprint 5 compatibility format and must remain supported.
- A bare array of objects is accepted as a batch.
- A bare object is accepted as a batch of one.
- Non-object records return `ErrMalformedInput`.
- Empty `records`, empty arrays, blank input, or `null` return `ErrEmptyInput`.

### CSV format

```
level,message,host
info,server started,web-01
warn,high memory,web-01
```

- First row is the header. Each subsequent row becomes one record.
- Fields mapped by position to header names.
- Uses Go stdlib `encoding/csv`.
- Set `FieldsPerRecord = -1` so the parser can manually skip bad row lengths.
- Empty rows are skipped.
- If a data row has a different column count than the header, skip that row without logging.
- Parser package must not depend on `internal/logger`; parser tests run before logger init.
- All values are strings in the output map. Schema inference handles later merging.

### Logfmt format

```
level=info msg="server started" host=web-01 latency_ms=42
level=warn msg="high memory" host=web-01
```

- Each line is one record.
- Keys and values are separated by `=`.
- Values with spaces must be quoted with double quotes.
- Empty lines are skipped.
- Duplicate keys: last value wins.
- All values are strings in the output map.
- Use `github.com/kr/logfmt` through `logfmt.NewDecoder`; do not assume a handler-based API.

### Syslog format

**RFC 5424:**
```
<34>1 2024-01-15T10:30:00Z web-01 myapp 1234 ID47 [exampleSDID@32473 iut="3"] message here
```

**RFC 3164:**
```
<34>Jan 15 10:30:00 web-01 myapp[1234]: message here
```

Output record fields:
```go
map[string]interface{}{
    "priority":  34,
    "facility":  4,
    "severity":  2,
    "timestamp": "...",
    "hostname":  "web-01",
    "appname":   "myapp",
    "procid":    "1234",
    "msgid":     "ID47",
    "message":   "message here",
}
```

Use `github.com/influxdata/go-syslog/v3`. Because this library exposes parsed
messages through concrete RFC-specific structs and embedded base fields, Task 06
must use type assertions after parsing instead of assuming a single universal
method-based `SyslogMessage` interface.

---

## TASK 01 — Add logfmt and syslog dependencies

**Action:**
```bash
go get github.com/kr/logfmt
go get github.com/influxdata/go-syslog/v3
go mod tidy
```

**Verify:** `CGO_ENABLED=1 go build ./...` — zero errors.

---

## TASK 02 — Create internal/parser/ directory and interface

**Action — Part A:**
```bash
mkdir -p internal/parser
```

**Action — Part B:** Create `internal/parser/parser.go`.

**Full file content:**
```go
package parser

import "errors"

type Parser interface {
    Parse(data []byte) ([]map[string]interface{}, error)
    ContentType() string
}

var (
    ErrEmptyInput            = errors.New("input is empty")
    ErrMalformedInput        = errors.New("malformed input")
    ErrUnsupportedContentType = errors.New("unsupported content type")
)

const (
    ContentTypeJSON   = "application/json"
    ContentTypeCSV    = "text/csv"
    ContentTypeLogfmt = "text/x-logfmt"
    ContentTypeSyslog = "application/x-syslog"
)
```

**Verify:** `go build ./internal/parser/` compiles with no errors.

---

## TASK 03 — Create internal/parser/json.go

**Action:** Create `internal/parser/json.go`.

**Full file content:**
```go
package parser

import (
    "bytes"
    "encoding/json"
    "fmt"
)

type JSONParser struct{}

func (p *JSONParser) ContentType() string { return ContentTypeJSON }

func (p *JSONParser) Parse(data []byte) ([]map[string]interface{}, error) {
    data = bytes.TrimSpace(data)
    if len(data) == 0 || bytes.Equal(data, []byte("null")) {
        return nil, ErrEmptyInput
    }

    if data[0] == '[' {
        var records []map[string]interface{}
        if err := json.Unmarshal(data, &records); err != nil {
            return nil, fmt.Errorf("%w: %v", ErrMalformedInput, err)
        }
        return validateRecords(records)
    }

    if data[0] == '{' {
        var obj map[string]interface{}
        if err := json.Unmarshal(data, &obj); err != nil {
            return nil, fmt.Errorf("%w: %v", ErrMalformedInput, err)
        }

        if rawRecords, ok := obj["records"]; ok {
            marshalled, err := json.Marshal(rawRecords)
            if err != nil {
                return nil, fmt.Errorf("%w: invalid records field", ErrMalformedInput)
            }
            var records []map[string]interface{}
            if err := json.Unmarshal(marshalled, &records); err != nil {
                return nil, fmt.Errorf("%w: records must be an array of objects", ErrMalformedInput)
            }
            return validateRecords(records)
        }

        if len(obj) == 0 {
            return nil, ErrEmptyInput
        }
        return []map[string]interface{}{obj}, nil
    }

    return nil, fmt.Errorf("%w: expected JSON object, array, or records wrapper", ErrMalformedInput)
}

func validateRecords(records []map[string]interface{}) ([]map[string]interface{}, error) {
    if len(records) == 0 {
        return nil, ErrEmptyInput
    }
    for i, record := range records {
        if record == nil || len(record) == 0 {
            return nil, fmt.Errorf("%w: record %d must be a non-empty object", ErrMalformedInput, i)
        }
    }
    return records, nil
}
```

**Verify:** `go build ./internal/parser/` compiles with no errors.

---

## TASK 04 — Create internal/parser/csv.go

**Action:** Create `internal/parser/csv.go`.

**Full file content:**
```go
package parser

import (
    "bytes"
    "encoding/csv"
    "fmt"
    "io"
    "strings"
)

type CSVParser struct{}

func (p *CSVParser) ContentType() string { return ContentTypeCSV }

func (p *CSVParser) Parse(data []byte) ([]map[string]interface{}, error) {
    data = bytes.TrimSpace(data)
    if len(data) == 0 {
        return nil, ErrEmptyInput
    }

    r := csv.NewReader(bytes.NewReader(data))
    r.TrimLeadingSpace = true
    r.FieldsPerRecord = -1

    headers, err := r.Read()
    if err == io.EOF {
        return nil, ErrEmptyInput
    }
    if err != nil {
        return nil, fmt.Errorf("%w: failed to read CSV header: %v", ErrMalformedInput, err)
    }
    if len(headers) == 0 {
        return nil, ErrEmptyInput
    }
    for i := range headers {
        headers[i] = strings.TrimSpace(headers[i])
        if headers[i] == "" {
            return nil, fmt.Errorf("%w: header field %d is empty", ErrMalformedInput, i)
        }
    }

    var records []map[string]interface{}
    for {
        row, err := r.Read()
        if err == io.EOF {
            break
        }
        if err != nil {
            return nil, fmt.Errorf("%w: failed to read CSV row: %v", ErrMalformedInput, err)
        }
        if isEmptyCSVRow(row) {
            continue
        }
        if len(row) != len(headers) {
            continue
        }
        record := make(map[string]interface{}, len(headers))
        for i, h := range headers {
            record[h] = row[i]
        }
        records = append(records, record)
    }

    if len(records) == 0 {
        return nil, ErrEmptyInput
    }
    return records, nil
}

func isEmptyCSVRow(row []string) bool {
    for _, cell := range row {
        if strings.TrimSpace(cell) != "" {
            return false
        }
    }
    return true
}
```

**Verify:** `go build ./internal/parser/` compiles with no errors.

---

## TASK 05 — Create internal/parser/logfmt.go

**Action:** Create `internal/parser/logfmt.go`.

**Full file content:**
```go
package parser

import (
    "bufio"
    "bytes"
    "fmt"

    "github.com/kr/logfmt"
)

type LogfmtParser struct{}

func (p *LogfmtParser) ContentType() string { return ContentTypeLogfmt }

func (p *LogfmtParser) Parse(data []byte) ([]map[string]interface{}, error) {
    data = bytes.TrimSpace(data)
    if len(data) == 0 {
        return nil, ErrEmptyInput
    }

    var records []map[string]interface{}
    scanner := bufio.NewScanner(bytes.NewReader(data))
    scanner.Buffer(make([]byte, 1024), 1024*1024)

    for scanner.Scan() {
        line := bytes.TrimSpace(scanner.Bytes())
        if len(line) == 0 {
            continue
        }

        record := make(map[string]interface{})
        dec := logfmt.NewDecoder(bytes.NewReader(line))
        for dec.ScanRecord() {
            for dec.ScanKeyval() {
                record[string(dec.Key())] = string(dec.Value())
            }
            if err := dec.Err(); err != nil {
                return nil, fmt.Errorf("%w: logfmt parse error: %v", ErrMalformedInput, err)
            }
        }
        if err := dec.Err(); err != nil {
            return nil, fmt.Errorf("%w: logfmt parse error: %v", ErrMalformedInput, err)
        }
        if len(record) > 0 {
            records = append(records, record)
        }
    }

    if err := scanner.Err(); err != nil {
        return nil, fmt.Errorf("%w: scanner error: %v", ErrMalformedInput, err)
    }
    if len(records) == 0 {
        return nil, ErrEmptyInput
    }
    return records, nil
}
```

**Verify:** `go build ./internal/parser/` compiles with no errors.

---

## TASK 06 — Create internal/parser/syslog.go

**Action:** Create `internal/parser/syslog.go`.

**Full file content:**
```go
package parser

import (
    "bufio"
    "bytes"
    "fmt"
    "time"

    "github.com/influxdata/go-syslog/v3/rfc3164"
    "github.com/influxdata/go-syslog/v3/rfc5424"
)

type SyslogParser struct{}

func (p *SyslogParser) ContentType() string { return ContentTypeSyslog }

func (p *SyslogParser) Parse(data []byte) ([]map[string]interface{}, error) {
    data = bytes.TrimSpace(data)
    if len(data) == 0 {
        return nil, ErrEmptyInput
    }

    var records []map[string]interface{}
    scanner := bufio.NewScanner(bytes.NewReader(data))
    scanner.Buffer(make([]byte, 1024), 1024*1024)

    for scanner.Scan() {
        line := bytes.TrimSpace(scanner.Bytes())
        if len(line) == 0 {
            continue
        }
        record, err := parseSyslogLine(line)
        if err != nil {
            return nil, fmt.Errorf("%w: %v", ErrMalformedInput, err)
        }
        records = append(records, record)
    }

    if err := scanner.Err(); err != nil {
        return nil, fmt.Errorf("%w: scanner error: %v", ErrMalformedInput, err)
    }
    if len(records) == 0 {
        return nil, ErrEmptyInput
    }
    return records, nil
}

func parseSyslogLine(line []byte) (map[string]interface{}, error) {
    p5424 := rfc5424.NewParser()
    if msg, err := p5424.Parse(line); err == nil && msg != nil {
        if m, ok := msg.(*rfc5424.SyslogMessage); ok {
            return syslog5424ToRecord(m), nil
        }
    }

    p3164 := rfc3164.NewParser()
    if msg, err := p3164.Parse(line); err == nil && msg != nil {
        if m, ok := msg.(*rfc3164.SyslogMessage); ok {
            return syslog3164ToRecord(m), nil
        }
    }

    return nil, fmt.Errorf("line not valid RFC 5424 or RFC 3164 syslog: %q", string(line))
}

func syslog5424ToRecord(msg *rfc5424.SyslogMessage) map[string]interface{} {
    r := baseSyslogRecord(msg.Priority, msg.Timestamp, msg.Hostname, msg.Appname, msg.ProcID, msg.Message)
    if msg.MsgID != nil {
        r["msgid"] = *msg.MsgID
    }
    return r
}

func syslog3164ToRecord(msg *rfc3164.SyslogMessage) map[string]interface{} {
    return baseSyslogRecord(msg.Priority, msg.Timestamp, msg.Hostname, msg.Appname, msg.ProcID, msg.Message)
}

func baseSyslogRecord(priority *uint8, timestamp *time.Time, hostname, appname, procid, message *string) map[string]interface{} {
    r := map[string]interface{}{
        "priority": 0,
        "facility": 0,
        "severity": 0,
        "timestamp": "",
        "hostname": "",
        "appname": "",
        "procid": "",
        "msgid": "",
        "message": "",
    }
    if priority != nil {
        pri := int(*priority)
        r["priority"] = pri
        r["facility"] = pri / 8
        r["severity"] = pri % 8
    }
    if timestamp != nil {
        r["timestamp"] = timestamp.Format(time.RFC3339Nano)
        r["timestamp_ns"] = float64(timestamp.UnixNano())
    }
    if hostname != nil {
        r["hostname"] = *hostname
    }
    if appname != nil {
        r["appname"] = *appname
    }
    if procid != nil {
        r["procid"] = *procid
    }
    if message != nil {
        r["message"] = *message
    }
    return r
}
```

**Implementation note:** If the installed `go-syslog/v3` exposes concrete message
fields with slightly different names, inspect with:
```bash
go doc github.com/influxdata/go-syslog/v3/rfc5424.SyslogMessage
go doc github.com/influxdata/go-syslog/v3/rfc3164.SyslogMessage
```
Adjust only the field access lines. Do not change the output record contract.

**Verify:** `go build ./internal/parser/` compiles with no errors.

---

## TASK 07 — Create internal/parser/registry.go

**Action:** Create `internal/parser/registry.go`.

**Full file content:**
```go
package parser

import "strings"

type Registry struct {
    parsers map[string]Parser
    defaultParser Parser
}

func NewRegistry() *Registry {
    jsonParser := &JSONParser{}
    return &Registry{
        parsers: map[string]Parser{
            ContentTypeJSON:   jsonParser,
            ContentTypeCSV:    &CSVParser{},
            ContentTypeLogfmt: &LogfmtParser{},
            ContentTypeSyslog: &SyslogParser{},
        },
        defaultParser: jsonParser,
    }
}

func (r *Registry) Get(contentType string) Parser {
    if idx := strings.IndexByte(contentType, ';'); idx >= 0 {
        contentType = contentType[:idx]
    }
    contentType = strings.TrimSpace(strings.ToLower(contentType))
    if p, ok := r.parsers[contentType]; ok {
        return p
    }
    return r.defaultParser
}

func NormalizeContentType(contentType string) string {
    if idx := strings.IndexByte(contentType, ';'); idx >= 0 {
        contentType = contentType[:idx]
    }
    return strings.TrimSpace(strings.ToLower(contentType))
}
```

**Verify:** `go build ./internal/parser/` compiles with no errors.

---

## TASK 08 — Update internal/ingestion/handler.go for multi-format support

**Action:** Four targeted changes to `internal/ingestion/handler.go`.

**Change 1 — Add parser registry to Handler struct:**
```go
type Handler struct {
    hot     *hot.Manager
    wal     *walstore.Manager
    parsers *parser.Registry
}

func NewHandler(h *hot.Manager, w *walstore.Manager) *Handler {
    return &Handler{hot: h, wal: w, parsers: parser.NewRegistry()}
}
```

**Imports to add:**
```go
"errors"
"fmt"
"io"
"strconv"

"github.com/plomvix/plomvix/internal/parser"
```

**Change 2 — Add `parseRequest` helper:**
```go
func (h *Handler) parseRequest(w http.ResponseWriter, r *http.Request, allowed map[string]bool) ([]map[string]interface{}, bool) {
    ct := parser.NormalizeContentType(r.Header.Get("Content-Type"))
    if ct == "" {
        ct = parser.ContentTypeJSON
    }
    if !allowed[ct] {
        utils.BadRequest(w, r, utils.CodeValidationFailed,
            fmt.Sprintf("unsupported content type for this endpoint: %s", ct))
        return nil, false
    }

    data, err := io.ReadAll(r.Body)
    if err != nil {
        utils.InternalError(w, r, "failed to read request body")
        return nil, false
    }

    records, err := h.parsers.Get(ct).Parse(data)
    if err != nil {
        if errors.Is(err, parser.ErrEmptyInput) {
            utils.BadRequest(w, r, utils.CodeValidationFailed,
                "request body is empty or contains no parseable records")
            return nil, false
        }
        utils.BadRequest(w, r, utils.CodeValidationFailed,
            fmt.Sprintf("failed to parse request body: %v", err))
        return nil, false
    }
    return records, true
}
```

**Change 3 — Add timestamp helper:**
```go
func ensureTimestampNs(record map[string]interface{}) int64 {
    if v, ok := record["timestamp_ns"]; ok {
        if ts := numericToInt64(v); ts > 0 {
            record["timestamp"] = float64(ts)
            return ts
        }
    }
    if v, ok := record["timestamp"]; ok {
        if ts := numericToInt64(v); ts > 0 {
            record["timestamp"] = float64(ts)
            return ts
        }
    }
    ts := time.Now().UnixNano()
    record["timestamp"] = float64(ts)
    return ts
}

func numericToInt64(v interface{}) int64 {
    switch t := v.(type) {
    case float64:
        return int64(t)
    case int64:
        return t
    case int:
        return int64(t)
    case json.Number:
        n, _ := t.Int64()
        return n
    case string:
        n, _ := strconv.ParseInt(t, 10, 64)
        return n
    default:
        return 0
    }
}
```

**Change 4 — Rewrite only `IngestLogs` and `IngestJSON`:**

```go
func (h *Handler) IngestLogs(w http.ResponseWriter, r *http.Request) {
    records, ok := h.parseRequest(w, r, map[string]bool{
        parser.ContentTypeJSON: true,
        parser.ContentTypeCSV: true,
        parser.ContentTypeLogfmt: true,
        parser.ContentTypeSyslog: true,
    })
    if !ok { return }

    count := 0
    var schemaRecords []map[string]interface{}
    for _, record := range records {
        tsNs := ensureTimestampNs(record)
        payload, err := json.Marshal(record)
        if err != nil { utils.InternalError(w, r, "failed to serialize record"); return }
        if _, err := h.wal.Write(walstore.DataTypeLog, payload); err != nil {
            logger.Error("WAL write failed", zap.Error(err)); utils.InternalError(w, r, "failed to write to WAL"); return
        }
        if err := h.hot.WriteLog(tsNs, payload); err != nil {
            logger.Error("hot tier write failed", zap.Error(err)); utils.InternalError(w, r, "failed to write to hot tier"); return
        }
        count++
        schemaRecords = append(schemaRecords, record)
    }
    if err := UpdateSchema(h.hot, "logs", schemaRecords); err != nil {
        logger.Warn("schema update failed", zap.String("data_type", "logs"), zap.Error(err))
    }
    utils.Created(w, r, IngestResponse{Ingested: count, RequestID: r.Header.Get("X-Request-ID")})
}

func (h *Handler) IngestJSON(w http.ResponseWriter, r *http.Request) {
    records, ok := h.parseRequest(w, r, map[string]bool{
        parser.ContentTypeJSON: true,
        parser.ContentTypeCSV: true,
    })
    if !ok { return }

    count := 0
    var schemaRecords []map[string]interface{}
    for _, record := range records {
        tsNs := ensureTimestampNs(record)
        payload, err := json.Marshal(record)
        if err != nil { utils.InternalError(w, r, "failed to serialize record"); return }
        if _, err := h.wal.Write(walstore.DataTypeJSON, payload); err != nil {
            logger.Error("WAL write failed", zap.Error(err)); utils.InternalError(w, r, "failed to write to WAL"); return
        }
        if err := h.hot.WriteJSON(tsNs, payload); err != nil {
            logger.Error("hot tier write failed", zap.Error(err)); utils.InternalError(w, r, "failed to write to hot tier"); return
        }
        count++
        schemaRecords = append(schemaRecords, record)
    }
    if err := UpdateSchema(h.hot, "json", schemaRecords); err != nil {
        logger.Warn("schema update failed", zap.String("data_type", "json"), zap.Error(err))
    }
    utils.Created(w, r, IngestResponse{Ingested: count, RequestID: r.Header.Get("X-Request-ID")})
}
```

**IngestMetrics and IngestKV:** Leave JSON decode and type-specific validation from
Sprint 5 unchanged. This preserves required `name` and `key` validation and avoids
silently accepting CSV/logfmt/syslog for endpoints that do not have a defined mapping.

**Verify:** `CGO_ENABLED=1 go build ./internal/ingestion/` compiles with no errors.

---

## TASK 09 — Create internal/parser/json_test.go

**Action:** Create `internal/parser/json_test.go`.

Tests required:
- bare array returns 2 records
- bare object returns 1 record
- Sprint 5 wrapper `{"records":[...]}` returns records
- empty body returns `ErrEmptyInput`
- empty array returns `ErrEmptyInput`
- empty wrapper `{"records":[]}` returns `ErrEmptyInput`
- malformed JSON returns `ErrMalformedInput`
- non-object JSON returns `ErrMalformedInput`
- content type is `application/json`

**Verify:** `go test -race ./internal/parser/` — all JSON tests pass.

---

## TASK 10 — Create internal/parser/csv_test.go

**Action:** Create `internal/parser/csv_test.go`.

Tests required:
- basic CSV returns 2 records
- quoted fields with embedded comma parse correctly
- blank input returns `ErrEmptyInput`
- header-only CSV returns `ErrEmptyInput`
- mismatched row is skipped and does not panic
- empty header field returns `ErrMalformedInput`
- content type is `text/csv`

**Important:** Do not initialise `internal/logger` in parser tests. The parser package
must be testable without global application boot.

**Verify:** `go test -race ./internal/parser/` — all CSV tests pass.

---

## TASK 11 — Create internal/parser/logfmt_test.go

**Action:** Create `internal/parser/logfmt_test.go`.

Tests required:
- basic logfmt parses quoted values
- multiple lines return multiple records
- blank lines are skipped
- duplicate keys use last value
- blank input returns `ErrEmptyInput`
- malformed line returns `ErrMalformedInput`
- content type is `text/x-logfmt`

**Verify:** `go test -race ./internal/parser/` — all logfmt tests pass.

---

## TASK 12 — Create internal/parser/syslog_test.go

**Action:** Create `internal/parser/syslog_test.go`.

Tests required:
- valid RFC 5424 message parses into normalized fields
- valid RFC 3164 message parses into normalized fields
- multiple lines return multiple records
- priority 34 computes facility 4 and severity 2
- blank input returns `ErrEmptyInput`
- malformed input returns `ErrMalformedInput`
- content type is `application/x-syslog`

**Verify:** `go test -race ./internal/parser/` — all syslog tests pass.

---

## TASK 13 — Create internal/parser/registry_test.go

**Action:** Create `internal/parser/registry_test.go`.

Tests required:
- exact JSON, CSV, logfmt, and syslog content types resolve correctly
- unknown type defaults to JSON
- empty type defaults to JSON
- parameters are stripped, e.g. `text/csv; charset=utf-8`
- lookup is case-insensitive

**Verify:** `go test -race ./internal/parser/` — all registry tests pass.

---

## TASK 14 — Create docs/api/formats.md

**Action:** Create `docs/api/formats.md`.

```markdown
# Plomvix Supported Input Formats

In Sprint 8, Plomvix supports multiple input formats through the `Content-Type` header.

## Supported Formats

| Content-Type | Format | Endpoints |
|---|---|---|
| `application/json` | JSON | all ingest endpoints |
| `text/csv` | CSV with header row | `/ingest/logs`, `/ingest/json` |
| `text/x-logfmt` | Logfmt key=value pairs | `/ingest/logs` |
| `application/x-syslog` | Syslog RFC 5424 or RFC 3164 | `/ingest/logs` |

If `Content-Type` is absent or unrecognised, `application/json` is assumed.

## JSON

Accepted on all ingest endpoints. Existing Sprint 5 wrapper format is still supported:

```bash
curl -X POST http://localhost:8080/ingest/logs \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"records":[{"level":"info","message":"hello world"}]}'
```

`/ingest/logs` and `/ingest/json` also accept bare objects and arrays:

```bash
curl -X POST http://localhost:8080/ingest/logs \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '[{"level":"info","message":"first"},{"level":"warn","message":"second"}]'
```

## CSV

First row is the header. Each subsequent row becomes one record. CSV is supported
on `/ingest/logs` and `/ingest/json`.

```bash
curl -X POST http://localhost:8080/ingest/logs \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: text/csv" \
  --data-binary $'level,message,host\ninfo,server started,web-01\nwarn,high memory,web-02'
```

## Logfmt

One record per non-blank line. Logfmt is supported on `/ingest/logs` only.

```bash
curl -X POST http://localhost:8080/ingest/logs \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: text/x-logfmt" \
  --data-binary $'level=info msg="server started" host=web-01\nlevel=warn msg="high load" host=web-01'
```

## Syslog

Syslog supports RFC 5424 and RFC 3164. Syslog is supported on `/ingest/logs` only.

```bash
curl -X POST http://localhost:8080/ingest/logs \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/x-syslog" \
  --data-binary '<34>1 2024-01-15T10:30:00Z web-01 myapp 1234 ID47 - message here'
```

## Error Responses

| Condition | HTTP Status | Code |
|---|---|---|
| Empty or blank body | 400 | `VALIDATION_FAILED` |
| Malformed input | 400 | `VALIDATION_FAILED` |
| Unsupported format for endpoint | 400 | `VALIDATION_FAILED` |
| Missing required fields for metrics/KV JSON | 400 | `VALIDATION_FAILED` |
```

**Verify:** `cat docs/api/formats.md` shows full content.

---

## TASK 15 — Full build and smoke test

**Action:**

```bash
#!/bin/bash
set -euo pipefail

echo "=== Clearing stale data ==="
rm -rf data/hot/ data/wal/ data/cold/

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
./plomvix > /tmp/plomvix_s8.log 2>&1 &
SERVER_PID=$!
sleep 3

echo ""
echo "=== Step 4: Login ==="
TOKEN=$(curl -sf -X POST http://localhost:8080/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"changeme"}' | jq -r '.data.token')
echo "Token acquired"

echo ""
echo "=== Step 5: JSON ingest wrapper still works ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/logs \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"records":[{"level":"info","message":"json wrapper test"}]}')
[ "$STATUS" -eq 201 ] && echo "PASS: JSON wrapper ingest 201" \
    || { echo "FAIL: JSON wrapper ingest got $STATUS"; exit 1; }

echo ""
echo "=== Step 6: JSON bare array works ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/logs \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '[{"level":"info","message":"json array test"}]')
[ "$STATUS" -eq 201 ] && echo "PASS: JSON array ingest 201" \
    || { echo "FAIL: JSON array ingest got $STATUS"; exit 1; }

echo ""
echo "=== Step 7: CSV ingest ==="
RESP=$(curl -sf -X POST http://localhost:8080/ingest/logs \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: text/csv" \
    --data-binary $'level,message\ninfo,a\nwarn,b')
INGESTED=$(echo "$RESP" | jq -r '.data.ingested')
[ "$INGESTED" -eq 2 ] && echo "PASS: CSV ingested=$INGESTED" \
    || { echo "FAIL: CSV ingested=$INGESTED, want 2"; exit 1; }

echo ""
echo "=== Step 8: Logfmt ingest ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/logs \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: text/x-logfmt" \
    --data-binary $'level=info msg="logfmt test" host=web-01')
[ "$STATUS" -eq 201 ] && echo "PASS: logfmt ingest 201" \
    || { echo "FAIL: logfmt ingest got $STATUS"; exit 1; }

echo ""
echo "=== Step 9: Syslog ingest ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/logs \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/x-syslog" \
    --data-binary '<34>1 2024-01-15T10:30:00Z web-01 testapp 1 - - syslog test message')
[ "$STATUS" -eq 201 ] && echo "PASS: syslog ingest 201" \
    || { echo "FAIL: syslog ingest got $STATUS"; exit 1; }

echo ""
echo "=== Step 10: Empty body returns 400 ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/logs \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '')
[ "$STATUS" -eq 400 ] && echo "PASS: empty body → 400" \
    || { echo "FAIL: expected 400, got $STATUS"; exit 1; }

echo ""
echo "=== Step 11: Header-only CSV returns 400 ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/logs \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: text/csv" \
    --data-binary $'level,message\n')
[ "$STATUS" -eq 400 ] && echo "PASS: header-only CSV → 400" \
    || { echo "FAIL: expected 400, got $STATUS"; exit 1; }

echo ""
echo "=== Step 12: Unsupported format on metrics returns 400 ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/ingest/metrics \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: text/csv" \
    --data-binary $'name,value\ncpu,1')
[ "$STATUS" -eq 400 ] && echo "PASS: metrics CSV → 400" \
    || { echo "FAIL: expected 400, got $STATUS"; exit 1; }

echo ""
echo "=== Step 13: Query logs includes multi-format records ==="
RESP=$(curl -sf "http://localhost:8080/query/logs" \
    -H "Authorization: Bearer $TOKEN")
TOTAL=$(echo "$RESP" | jq '.data.total')
[ "$TOTAL" -ge 5 ] && echo "PASS: total=$TOTAL" \
    || { echo "FAIL: total=$TOTAL, want >= 5"; exit 1; }

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
echo "  ALL STEPS PASSED — Sprint 8 smoke test DONE  "
echo "================================================"
```

| Step | Verified | Expected |
|---|---|---|
| 1 | Build + vet | No errors |
| 2 | All tests | Pass with race detector |
| 3 | Boot | Server starts |
| 4 | Login | JWT returned |
| 5 | JSON wrapper ingest | 201 backward compatibility |
| 6 | JSON bare array ingest | 201 convenience format |
| 7 | CSV ingest | ingested=2 |
| 8 | Logfmt ingest | 201 |
| 9 | Syslog ingest | 201 |
| 10 | Empty body | 400 |
| 11 | Header-only CSV | 400 |
| 12 | CSV on metrics | 400 unsupported format |
| 13 | Query after multi-format ingest | total >= 5 |
| 14 | Graceful shutdown | Exit code 0 |

---

## EXECUTION ORDER SUMMARY

```
TASK 01  →  go get logfmt + syslog dependencies
TASK 02  →  internal/parser/parser.go (interface + constants)
TASK 03  →  internal/parser/json.go
TASK 04  →  internal/parser/csv.go
TASK 05  →  internal/parser/logfmt.go
TASK 06  →  internal/parser/syslog.go
TASK 07  →  internal/parser/registry.go
TASK 08  →  internal/ingestion/handler.go (parser registry, parseRequest, rewrite IngestLogs + IngestJSON only)
TASK 09  →  internal/parser/json_test.go
TASK 10  →  internal/parser/csv_test.go
TASK 11  →  internal/parser/logfmt_test.go
TASK 12  →  internal/parser/syslog_test.go
TASK 13  →  internal/parser/registry_test.go
TASK 14  →  docs/api/formats.md
TASK 15  →  smoke test — all 14 steps must pass
```

---

*Sprint 8 complete when TASK 15 passes with zero failures.*
*Plomvix — Built in India. Built for the world.*