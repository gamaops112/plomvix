# Metrics Engine Setup (Basic Ingestion and Query)

| Field | Value |
| :--- | :--- |
| **Source** | `agent/engine/metrics/metrics_engine_setup.md` |
| **Package(s)** | `internal/engine/metrics` |
| **Purpose** | Implement a pluggable Metrics Engine mapping to the `engine.Engine` interface to perform sequential time-series ingestion (INSERT) and simple range-scan queries (SELECT), registered with the global catalog. |
| **Dependencies** | Custom Pager, Catalog, Router, Parser, and Wiring configurations. |

## Honest Contracts & Known Trade-offs

1. **Append-Only Flat Layout:** In the Basic tier, metrics are written sequentially into flat 4KB pages. There is no columnar block partitioning, Gorilla-style XOR floating-point compression, or downsampling; these optimizations are deferred to the Enterprise tier.
2. **Synchronous Ingestion:** Ingestion operations validate and write point payloads directly to the sequential page store synchronously during execution. In-memory batch buffers and background flush workers are deferred.
3. **Scan-Only Filtering:** Time-series query processing is performed using a full sequential page scan, filtering point records matching the timestamp range and tag label constraints. Index search structures (like inverted indexes or tag lookup trees) are deferred.
4. **Simple Schema Validation:** The catalog schema for metrics tables must validate that the target table contains a `time` (int64/uint64) timestamp field and at least one numeric metric value.
5. **No UPDATE or DELETE:** Any `UPDATE` or `DELETE` statement targeting a metrics table must immediately fail and return `ErrUnsupportedQuery`. Rollbacks, tombstoning, and compactions are deferred to the Enterprise tier.
6. **No JOINs:** Queries attempting to `JOIN` a metrics table with any other table (relational or metrics) are strictly prohibited and must return `ErrUnsupportedQuery`.
7. **Simple WHERE Clauses Only:** Only exact tag matches (e.g. `host = 'server1'`) and simple numeric timestamp boundaries (`time >= X`, `time <= Y`, or `time BETWEEN X AND Y`) are supported. Any complex expressions (e.g. `time + 100 > X`), nested operators, or function evaluations in the `WHERE` clause must return `ErrUnsupportedQuery`.
8. **Write Order Crash Consistency:** The `MetricsStore` performs a read-modify-write cycle: Read page body ➔ Serialize record at `next_write_offset` ➔ Update header pointers ➔ Call `pager.WritePage()` to overwrite atomically. A crash during `WritePage` can result in losing the last uncommitted point, but it will never corrupt the page header or previously committed points.
9. **Zero-DDL Auto-Table Creation (Schema-on-Write):** To allow automated agents to store metrics without manual database/table creation:
   - When the Global Router intercepts an `INSERT` query targeting a table not registered in the catalog, it will automatically register the table as a `"metrics"` engine table with the standard metrics schema.
   - Alternatively, if writes are sent to a global `metrics` table, the tags payload must include a special `_table_name` tag to determine the target metric series.
   - Metrics are treated strictly as schema-less points (timestamp, key-value tags, series name, value) rather than rigid SQL columns.

---

## Deliverables

| File | Purpose |
| :--- | :--- |
| `internal/engine/metrics/engine.go` | Implement the Metrics Engine executing time-series SELECT/INSERT statements and matching the `engine.Engine` interface. |
| `internal/engine/metrics/store.go` | Create a flat page-based time-series append-only log store utilizing `storage/pager` to write sequential metric records. |
| `internal/config/config.go` | Extend configuration schemas (`Config`, `StoreConfig`) to support metrics database paths. |
| `internal/runtime/runtime.go` | Rewrite runtime dependency injection to initialize, register, and wire the metrics engine component. |

---

## Key API & Concepts

### 1. In-Page Sequential Storage Format (`internal/engine/metrics/store.go`)

Points are packed sequentially inside 4KB pages. Each page layout contains a header indicating the count of points and the offset of the next record to append, followed by variable-length metric records:

```
[Page Body (4084 bytes)]
+---------------------------+----------------------------------------------+
| num_points (uint32)       | next_write_offset (uint32)                   |
+---------------------------+----------------------------------------------+
| point_records (variable length)...                                       |
+--------------------------------------------------------------------------+
```

* **Header layout:**
  - Offset 0: `num_points` (uint32) - tracks count of points in this page.
  - Offset 4: `next_write_offset` (uint32) - points to the exact byte offset where the next record starts. Initializes to `8` on a new page.
* **Metric record encoding:**
  - `timestamp` (8 bytes, int64/uint64)
  - `tags_length` (2 bytes, uint16)
  - `tags_payload` (variable bytes, key=value comma-separated strings or JSON)
  - `metric_name_len` (2 bytes, uint16)
  - `metric_name` (variable bytes, string)
  - `value` (8 bytes, float64)
* **Page Full & Fragmentation Policy:**
  - If a new record's serialized length exceeds the remaining space in the current page (`4084 - next_write_offset`), the store does NOT split the record across pages.
  - Instead, the remaining bytes of the current page are left zeroed, the current page is written to disk, and a new page is allocated via `pager.AllocatePage()`.
  - The new page is initialized with `num_points = 0` and `next_write_offset = 8`, and the record is appended at offset 8.


### 2. Metrics Engine Implementation (`internal/engine/metrics/engine.go`)

The Metrics Engine validates time-series schema parameters and routes execution to the sequential page store.

```go
package metrics

import (
	"context"
	"errors"
	"fmt"

	"github.com/plomvix/plomvix/internal/catalog"
	"github.com/plomvix/plomvix/internal/engine"
	"github.com/plomvix/plomvix/internal/sqlparser"
)

var (
	ErrMissingTimeColumn   = errors.New("metrics engine: schema must contain a 'time' column")
	ErrNoMetricValueColumn = errors.New("metrics engine: schema must contain at least one numeric value column")
	ErrUnsupportedQuery    = errors.New("metrics engine: query statement not supported in basic tier")
)

type MetricsEngine struct {
	catalog catalog.Catalog
	store   *MetricsStore
}

func NewMetricsEngine(cat catalog.Catalog, store *MetricsStore) *MetricsEngine {
	return &MetricsEngine{
		catalog: cat,
		store:   store,
	}
}

func (e *MetricsEngine) Name() string { return "metrics" }

// ValidateSchema enforces that table schema has a 'time' field and a metric value.
// Hook Integration: When executing 'CREATE TABLE ... ENGINE=metrics', the DDL planner 
// invokes ValidateSchema prior to registering table metadata in the catalog. 
// If it returns an error, the DDL transaction aborts and returns the error.
func (e *MetricsEngine) ValidateSchema(schemaJSON []byte) error {
	// Parse schema and ensure:
	// 1. Column named "time" exists and is TypeInt64/TypeUint64.
	// 2. At least one other column is a numeric metric value (TypeInt64/TypeUint64/TypeFloat64).
	return nil
}

func (e *MetricsEngine) Execute(ctx context.Context, req *engine.Request) (*engine.Result, error) {
	switch req.Stmt.Type() {
	case sqlparser.StmtInsert:
		return e.executeInsert(ctx, req)
	case sqlparser.StmtSelect:
		return e.executeSelect(ctx, req)
	default:
		return nil, ErrUnsupportedQuery
	}
}

// executeInsert processes the time-series points:
// 1. Target Name Extraction: If the query is INSERT INTO metrics ..., the engine extracts the 
//    '_table_name' tag from the tags list and treats it as the metric series name.
//    Otherwise, the SQL table name is used directly as the series/table identifier.
// 2. Point Serialization: Encodes the point as a binary record and appends it to the flat metrics store.
func (e *MetricsEngine) executeInsert(ctx context.Context, req *engine.Request) (*engine.Result, error) {
	return nil, nil
}

// executeSelect performs pushdown traversal on the SELECT WHERE clause:
// 1. AST Traversal: Walks WHERE conditions looking for 'time >= X', 'time <= Y', or 'time BETWEEN X AND Y'.
// 2. Tag Filters: Extracts exact equality tags (e.g. 'host = X' or 'region = Y').
// 3. Reject Complex: If any nested expressions, function calls, OR gates, or non-tag column predicates
//    are present, immediately aborts execution returning ErrUnsupportedQuery.
// 4. Row Mapping: Decodes point records matching criteria, returning engine.Row values matching schema:
//    Col 0 -> 'time' (int64)
//    Col 1 -> 'tags' (string)
//    Col 2 -> 'metric_name' (string)
//    Col 3 -> 'value' (float64)
// 5. Result: Returns &engine.Result{Stream: &metricsRowStream{...}} representing filtered scans.
func (e *MetricsEngine) executeSelect(ctx context.Context, req *engine.Request) (*engine.Result, error) {
	return nil, nil
}

// Router Dynamic Table Creation Integration:
// Inside internal/router/router.go, the Route method is updated as follows:
// When routing an INSERT query, if r.catalog.GetTable(ctx, tableName) returns catalog.ErrTableNotFound,
// the router automatically calls:
//    r.catalog.CreateTable(ctx, "metrics", tableName, defaultMetricsSchema)
// and then proceeds to execute the query against the newly registered metrics table.


```

### 3. Runtime Wiring Extension (`internal/runtime/runtime.go`)

We extend `runtime.New` to instantiate the metrics engine:

```go
	// 1. Construct Metrics Storage
	metricsPager := pager.New(cfg.Store.MetricsDBPath)
	metricsStore := metrics.NewStore(metricsPager)

	// 2. Initialize Metrics Engine & Register it
	metricsEngine := metrics.NewMetricsEngine(cat, metricsStore)
	if err := cat.RegisterEngine(metricsEngine); err != nil {
		return nil, fmt.Errorf("runtime: register metrics engine: %w", err)
	}
	routerService.RegisterEngine(metricsEngine)

	// 3. Register components in LIFO order
	// Add metricsPager lifecycle registration
	if err := manager.Register(&pagerComponent{p: metricsPager}); err != nil {
		return nil, err
	}
```

---

## Tasks

1. **Extend Config with Metrics Fields:** Update `internal/config/config.go` with `MetricsDBPath` in `StoreConfig`, mapping to default `data/metrics.db`.
2. **Implement Flat Storage Store:** Create `internal/engine/metrics/store.go` containing `MetricsStore` logic: opening pages, appending serialized points, and scanning pages sequentially.
3. **Implement Metrics Engine Lifecycle:** Create `internal/engine/metrics/engine.go`. Implement `ValidateSchema` checks and translate insert values to serialized tags and value records.
4. **Wire Metrics Engine to Runtime:** Integrate `metricsPager` and `metricsEngine` inside `internal/runtime/runtime.go` lifecycle registration.
5. **Add Metrics Ingestion Tests:** Write integration tests in `internal/engine/metrics/metrics_test.go` verifying that tables created under `"metrics"` engine route queries successfully, ingest values, and filter ranges during scans.

---

## Completion Criteria

- [ ] Creating a table with `engine_name = "metrics"` enforces time-series validation rules.
- [ ] Ingesting single metric rows appends serial data cleanly into 4KB data pages.
- [ ] Simple queries using timestamp filtering (`time >= ? AND time <= ?`) return visible data points.
- [ ] Metrics pager shuts down and flushes page descriptors cleanly under LIFO hooks.
