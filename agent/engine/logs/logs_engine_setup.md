# Logs Engine Setup (Basic Ingestion and Text Search)

| Field | Value |
| :--- | :--- |
| **Source** | `agent/engine/logs/logs_engine_setup.md` |
| **Package(s)** | `internal/engine/logs` |
| **Purpose** | Implement a pluggable, schema-less Logs Engine mapping to the `engine.Engine` interface to perform sequential log ingestion (INSERT) and substring text search scans (SELECT) with zero-DDL auto-creation. |
| **Dependencies** | Custom Pager, Catalog, Router, Parser, and Wiring configurations. |

## Honest Contracts & Known Trade-offs

1. **Uncompressed Flat Storage Layout:** Logs are written sequentially into flat 4KB pages. There is no block-level compression (ZSTD/LZ4), columnar sorting, or downsampling; these optimizations are deferred to the Enterprise tier.
2. **Synchronous Single-Record Ingestion:** Log ingestion operations parse, serialize, and write records directly to the sequential page store synchronously during execution. In-memory batch queues and background flush workers are deferred.
3. **Scan-Only Filtering:** Text searching (using `LIKE` operators) is performed by scanning all pages sequentially and checking substring equality. Sparse indices (min/max timestamps) and block-level Bloom filters are deferred.
4. **Zero-DDL Auto-Table Ingest & Routing Heuristic:** No explicit schema definitions or table creations are required. Any `INSERT` statement targeting a table not registered in the catalog automatically registers it:
   - **Disambiguation Heuristic:** If the target table name contains the substring `"log"` or `"logs"` (case-insensitive, e.g., `app_logs`, `syslog`), it is auto-created under the `"logs"` engine. All other missing tables default to auto-creation under the `"metrics"` engine.
5. **No UPDATE or DELETE:** Modifying log records is strictly forbidden. Any `UPDATE` or `DELETE` query targeting logs tables returns `ErrUnsupportedQuery`.
6. **No JOINs:** Join operations on logs tables are not supported and return `ErrUnsupportedQuery`.
7. **Write Order Crash Consistency:** The `LogsStore` writes records using a read-modify-write page cycle, ensuring that database crashes do not corrupt previously written records.
8. **JSON Parsing & Severity Mapping:**
   - **Severity Enum Values:** `"DEBUG"`/`"debug"` -> 1, `"INFO"`/`"info"` -> 2, `"WARN"`/`"warning"`/`"warn"` -> 3, `"ERROR"`/`"error"` -> 4, `"FATAL"`/`"critical"`/`"fatal"` -> 5.
   - **Ingestion Fallback:** If the log payload is not valid JSON, or if the severity key is missing or unrecognized, the engine must not fail. It treats the entire payload as raw text in `body_payload` and defaults `severity` to `INFO` (2).
   - **Attributes Extraction:** Non-standard parameters inside a JSON log are serialized into a flat JSON object inside `attributes_payload`.
9. **AST LIKE Predicate Rules:**
   - The engine evaluates the SELECT query's `WHERE` clause for `vitess.Like` or `vitess.NotLike` operators.
   - It extracts only literal string comparisons (e.g. `body LIKE '%error%'`). If the comparison references another column or a placeholder parameter (e.g. `body LIKE ?`), it must fail and return `ErrUnsupportedQuery`.

---

## Deliverables

| File | Purpose |
| :--- | :--- |
| `internal/engine/logs/engine.go` | Implement the Logs Engine executing time-series SELECT/INSERT statements and matching the `engine.Engine` interface. |
| `internal/engine/logs/store.go` | Create a flat page-based log append-only log store utilizing `storage/pager` to write sequential log records. |
| `internal/config/config.go` | Extend configuration schemas (`Config`, `StoreConfig`) to support logs database paths. |
| `internal/runtime/runtime.go` | Rewrite runtime dependency injection to initialize, register, and wire the logs engine component. |

---

## Key API & Concepts

### 1. In-Page Log Record Encoding (`internal/engine/logs/store.go`)

Log records are packed sequentially inside 4KB pages. Each page layout contains a header indicating the count of records and the offset of the next record to append, followed by variable-length log records:

```
[Page Body (4084 bytes)]
+---------------------------+----------------------------------------------+
| num_records (uint32)      | next_write_offset (uint32)                   |
+---------------------------+----------------------------------------------+
| log_records (variable length)...                                         |
+--------------------------------------------------------------------------+
```

* **Header layout:**
  - Offset 0: `num_records` (uint32) - tracks count of records in this page.
  - Offset 4: `next_write_offset` (uint32) - points to the exact byte offset where the next record starts. Initializes to `8` on a new page.
* **Log record encoding:**
  - `timestamp` (8 bytes, int64)
  - `severity` (1 byte, uint8: DEBUG=1, INFO=2, WARN=3, ERROR=4, FATAL=5)
  - `attributes_len` (2 bytes, uint16)
  - `attributes_payload` (variable bytes, flat JSON string containing key-value metadata tags)
  - `body_len` (4 bytes, uint32)
  - `body_payload` (variable bytes, raw text log or JSON body string)
* **Page Full & Fragmentation Policy:**
  - If a new record's serialized length exceeds the remaining space in the current page (`4084 - next_write_offset`), the store does NOT split the record across pages.
  - Instead, the remaining bytes of the current page are left zeroed, the current page is written to disk, and a new page is allocated via `pager.AllocatePage()`.
  - The new page is initialized with `num_records = 0` and `next_write_offset = 8`, and the record is appended at offset 8.

### 2. Logs Engine Implementation (`internal/engine/logs/engine.go`)

The Logs Engine validates standard logs schema parameters, handles dynamic ingestion parsing, and performs query scanning.

```go
package logs

import (
	"context"
	"errors"
	"fmt"

	"github.com/plomvix/plomvix/internal/catalog"
	"github.com/plomvix/plomvix/internal/engine"
	"github.com/plomvix/plomvix/internal/sqlparser"
)

var (
	ErrUnsupportedQuery = errors.New("logs engine: query statement not supported in basic tier")
)

type LogsEngine struct {
	catalog catalog.Catalog
	store   *LogsStore
}

func NewLogsEngine(cat catalog.Catalog, store *LogsStore) *LogsEngine {
	return &LogsEngine{
		catalog: cat,
		store:   store,
	}
}

func (e *LogsEngine) Name() string { return "logs" }

// ValidateSchema enforces that table schema contains standard log fields
func (e *LogsEngine) ValidateSchema(schemaJSON []byte) error {
	// Ensures table contains standard structure: time (int64), severity (string/int), attributes (string), body (string)
	return nil
}

func (e *LogsEngine) Execute(ctx context.Context, req *engine.Request) (*engine.Result, error) {
	switch req.Stmt.Type() {
	case sqlparser.StmtInsert:
		return e.executeInsert(ctx, req)
	case sqlparser.StmtSelect:
		return e.executeSelect(ctx, req)
	default:
		return nil, ErrUnsupportedQuery
	}
}

// executeInsert parses logs dynamically:
// 1. JSON parsing: If insertion message is a JSON string, the engine extracts
//    keys like 'level' or 'status' to standard uint8 severity enum, moving
//    other fields to attributes_payload, and placing the core message in body_payload.
// 2. Raw text: Places the entire string in body_payload, defaulting level to INFO.
func (e *LogsEngine) executeInsert(ctx context.Context, req *engine.Request) (*engine.Result, error) {
	return nil, nil
}

// executeSelect performs full sequential page scans:
// 1. AST Filtering: Walks WHERE clause for time bounds, exact tag attribute matches,
//    and substring content filters (e.g. 'body LIKE %error%').
// 2. Reject Complex: Aborts and returns ErrUnsupportedQuery on nested expressions or functions.
// 3. Row Mapping: Converts binary records to standard engine.Row columns:
//    Col 0 -> time (int64)
//    Col 1 -> severity (string)
//    Col 2 -> attributes (string JSON)
//    Col 3 -> body (string)
func (e *LogsEngine) executeSelect(ctx context.Context, req *engine.Request) (*engine.Result, error) {
	return nil, nil
}
```

### 3. Router Dynamic Table Creation Integration

Inside `internal/router/router.go`, when routing an `INSERT` query:
* If `r.catalog.GetTable(ctx, tableName)` returns `catalog.ErrTableNotFound`:
  - **Routing Collision Resolving:** The router checks the target table name. If `strings.Contains(strings.ToLower(tableName), "log")` (case-insensitive check for substrings like "log" or "logs"), it automatically registers it as a Logs Engine table:
    `r.catalog.CreateTable(ctx, "logs", tableName, defaultLogsSchema)`
  - Otherwise, it defaults to the Metrics Engine and registers it using:
    `r.catalog.CreateTable(ctx, "metrics", tableName, defaultMetricsSchema)`
  - Proceed with executing the insertion on the newly registered engine table.


---

## Tasks

1. **Extend Config with Logs Path:** Update `internal/config/config.go` with `LogsDBPath` in `StoreConfig`, mapping to default `data/logs.db`.
2. **Implement Flat Storage Store:** Create `internal/engine/logs/store.go` containing `LogsStore` logic: opening pages, appending serialized log records, and scanning pages sequentially.
3. **Implement Logs Engine Lifecycle:** Create `internal/engine/logs/engine.go` implementing `ValidateSchema` checks, dynamic JSON/raw text insertion parsing, and pushdown substring scans.
4. **Wire Logs Engine to Runtime:** Integrate `logsPager` and `logsEngine` inside `internal/runtime/runtime.go` lifecycle registration.
5. **Add Logs Ingestion & Search Tests:** Write integration tests in `internal/engine/logs/logs_test.go` verifying that logs tables automatically register, ingest JSON/raw lines, and search keywords using `LIKE` operators.

---

## Completion Criteria

- [ ] Inserting to a non-existent log table automatically registers it as a logs table.
- [ ] Opaque log lines and JSON fields serialize cleanly into sequential log pages.
- [ ] Substring filter searches (`LIKE '%keyword%'`) correctly scan and return matching log rows.
- [ ] Logs storage pager shuts down and flushes page buffers cleanly under LIFO hooks.
