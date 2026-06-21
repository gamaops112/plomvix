# Plomvix Metrics Engine: Time-Series Ingestion & Query

The metrics engine provides append-only time-series storage with range-scan
queries, tag indexing, Gorilla compression, and background downsampling rollups.

## Architecture

- **Flat Page Store**: Metric points are packed sequentially into 4KB pages
  managed by the `storage/pager`. Each page has an 8-byte header (num_points + 
  next_write_offset) followed by variable-length point records.
- **Tag Index (Enterprise)**: An in-memory inverted index maps tag key-value
  pairs to page/offset locators, avoiding full scans for filtered queries.
- **Gorilla Compression (Enterprise)**: Bit-level stream encoding using
  double-delta timestamps and XOR float packing for compact storage.
- **Rollup Downsampling (Enterprise)**: Background worker periodically groups
  raw points into configurable time buckets (1m, 5m), computing count/sum/
  min/max aggregates, writing to a separate rollup pager file.

## Storage Format

### Raw Point Page Layout

```
[Page Body (4084 bytes)]
+-------------------+----------------------------------------------+
| num_points (u32)  | next_write_offset (u32)                     |
+-------------------+----------------------------------------------+
| point records (variable length)...                                 |
+-------------------------------------------------------------------+
```

### Point Record Encoding (Basic)

| Field | Size | Description |
|-------|------|-------------|
| timestamp | 8 bytes | int64, Unix seconds |
| tags_length | 2 bytes | uint16, length of tags payload |
| tags | variable | key=value,... or JSON |
| metric_name_len | 2 bytes | uint16, length of metric name |
| metric_name | variable | string identifier |
| value | 8 bytes | float64, IEEE 754 |

### Rollup Bucket Layout (Enterprise)

Each bucket occupies 36 bytes: bucket_start (int64, 8), point_count (uint32, 4),
sum_value (float64, 8), min_value (float64, 8), max_value (float64, 8).

## API

```go
type MetricsEngine struct {
    catalog  catalog.Catalog
    store    *MetricsStore
    index    *TagIndex      // enterprise
    rollup   *RollupManager // enterprise
}

func (e *MetricsEngine) Name() string                     // returns "metrics"
func (e *MetricsEngine) ValidateSchema(schemaJSON []byte) error
func (e *MetricsEngine) Execute(ctx context.Context, req *engine.Request) (*engine.Result, error)
```

## Schema Validation

The metrics engine enforces:
- A `"time"` column of type `int64` or `uint64` MUST exist.
- At least one additional numeric column (`int64`, `uint64`, or `float64`)
  MUST exist to serve as the metric value.
- The `"time"` column does NOT count toward the numeric requirement.

## Query Support

### INSERT

`INSERT INTO <table> (time, tags, metric_name, <value_col>) VALUES (...)`

- Column order is flexible; the engine maps by column name.
- `tags` column (if present) is stored as a raw string.
- `metric_name` column (if present) overrides the table name as the series identifier.

### SELECT

`SELECT * FROM <table> WHERE time >= ? AND time <= ? [AND tag_key = 'tag_val' ...]`

- **Basic tier**: Full sequential page scan with time range and tag filtering.
- **Enterprise tier**: Tag index is consulted first; only matching page offsets
  are read. Falls back to full scan if no index or no tag constraints.

## Enterprise: Tag Index

- Concurrent `sync.RWMutex`-protected inverted map: `tag_key → tag_value → []RecordLocator`.
- `SearchAll(constraints)` returns the AND-intersection of matching locators.
- `Sweep(now)` evicts entries older than the configurable retention window.

## Enterprise: Rollup Downsampling

- Background goroutine wakes on a configurable tick interval.
- Scans raw source pages, groups points into time buckets at configured
  resolutions, and writes aggregated `RollupBucket` records to a separate
  `data/metrics_rollups.db` pager file.
- Lifecycle-compatible: implements `Name()`, `Start()`, `Stop()` for LIFO
  registration.

## Honest Contracts

1. **Append-Only**: No UPDATE or DELETE support. These return `ErrUnsupportedQuery`.
2. **No JOINs**: Cross-table joins are rejected.
3. **Simple WHERE**: Only exact tag matches and simple timestamp comparisons
   (`>=`, `<=`, `>`, `<`, `BETWEEN`) are supported.
4. **Crash Consistency**: Single-page writes are durable. Crash during
   `WritePage` may lose the last uncommitted point but never corrupts
   previously committed data or page headers.
5. **Index Memory**: The tag index is in-memory only. Eviction by retention
   window prevents unbounded growth.
