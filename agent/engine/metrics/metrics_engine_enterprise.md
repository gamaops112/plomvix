# Metrics Engine Enterprise (Compression, Rollups and Tag Indexing)

| Field | Value |
| :--- | :--- |
| **Source** | `agent/engine/metrics/metrics_engine_enterprise.md` |
| **Package(s)** | `internal/engine/metrics` |
| **Purpose** | Implement Gorilla-style double-delta compression for timestamps and float values, background downsampling workers, and an inverted tag index to bypass full table scans. |
| **Dependencies** | Metrics Engine Setup plan. |

## Honest Contracts & Known Trade-offs

1. **In-Memory Tag Index Lifecycle & Memory Bounding:** The inverted tag index is kept in memory to optimize read latency. To prevent high-cardinality OOM crashes, the `TagIndex` only indexes tags within a configurable recent time window (e.g., the last 24 hours) and evicts older tag locators using an LRU policy. Its memory usage is capped via the `TagIndexMaxMemoryMB` configuration parameter.
2. **Concurrency & Ingestion Locking:** Concurrency is managed via a shared `sync.RWMutex`. Ingest/INSERT operations acquire a **Read Lock** (allowing concurrent writes), while the Rollup compaction worker acquires the **Write Lock** only when swapping the active page buffer, bounding ingestion block times to sub-milliseconds.
3. **Gorilla Bit-Packing Overhead:** Gorilla compression is bit-packed rather than byte-aligned. Reading and writing compressed points requires bitwise stream writers/readers, which introduces minor CPU parsing overhead in exchange for substantial storage savings.
4. **Basic Snapshot Consistency:** The downsampler reads only committed page offsets. Rollup outputs represent snapshots of historical data points, meaning points arriving late (out-of-order) might not be included in previously generated rollups.
5. **On-Disk Format Transition:** The Enterprise tier upgrades the ingestion path to write Gorilla-compressed records directly into active pages. Legacy uncompressed pages (from the Basic tier) are detected via a version flag in the page header and are dynamically converted to Gorilla format by a background compaction worker.

---

## Deliverables

| File | Purpose |
| :--- | :--- |
| `internal/engine/metrics/gorilla.go` | Implement Gorilla bit-stream writer and reader interfaces, performing double-delta timestamp compression and float XOR bit-packing. |
| `internal/engine/metrics/rollup.go` | Implement background downsampling workers that periodically consolidate raw metrics into 1-minute and 5-minute aggregation buckets. |
| `internal/engine/metrics/index.go` | Create a concurrent in-memory inverted tag index structure mapping tag key-value pairs to page number/offset points. |
| `internal/engine/metrics/engine.go` | Update Metrics Engine select logic to use tag indexes for filtering and route aggregation queries to rollup tables. |

---

## Key API & Concepts

### 1. Gorilla Timestamp & Float Compression (`internal/engine/metrics/gorilla.go`)

Timestamps and float values are compressed using Gorilla algorithms:
* **Timestamp Double-Delta:**
  - Given timestamps $T_1, T_2, T_3$, calculate delta $D_i = T_i - T_{i-1}$ and delta-of-deltas $D_D = D_i - D_{i-1}$.
  - If $D_D = 0$, write bit `0`.
  - If $-63 \le D_D \le 64$, write bits `10` followed by 7 bits of value.
  - If $-255 \le D_D \le 256$, write bits `110` followed by 9 bits.
  - If $-2047 \le D_D \le 2048$, write bits `1110` followed by 12 bits.
  - Otherwise, write bits `1111` followed by 32 bits of value.
* **Float XOR Compression:**
  - Calculate XOR value $X_i = V_i \oplus V_{i-1}$.
  - If $X_i = 0$, write bit `0`.
  - If $X_i \ne 0$, write bit `1`, then:
    - Control bit `0` if the sequence of leading/trailing zeros matches the previous XOR. Write the meaningful bits.
    - Control bit `1` if different. Write 5 bits of leading zero count, 6 bits of length of meaningful bits, followed by the meaningful bits.

### 2. In-Memory Inverted Tag Index (`internal/engine/metrics/index.go`)

To avoid scanning all flat pages during tag queries, an inverted index maps tags to record offsets:

```go
package metrics

import (
	"sync"
)

type RecordLocator struct {
	PageID uint64
	Offset uint32
}

type TagIndex struct {
	mu    sync.RWMutex
	index map[string]map[string][]RecordLocator // TagKey -> TagValue -> Locations
}

func NewTagIndex() *TagIndex {
	return &TagIndex{
		index: make(map[string]map[string][]RecordLocator),
	}
}

func (idx *TagIndex) Insert(key, val string, loc RecordLocator) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	vals, exists := idx.index[key]
	if !exists {
		vals = make(map[string][]RecordLocator)
		idx.index[key] = vals
	}
	vals[val] = append(vals[val], loc)
}

func (idx *TagIndex) Search(key, val string) []RecordLocator {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	vals, exists := idx.index[key]
	if !exists {
		return nil
	}
	locs := vals[val]
	if len(locs) == 0 {
		return nil
	}
	// Return a deep copy to prevent mutation
	copied := make([]RecordLocator, len(locs))
	copy(copied, locs)
	return copied
}
```

### 3. Query Execution Optimization & Rollup Schema

When executing a select query containing time bucket aggregations (e.g., `SELECT time_bucket(time, '5m'), AVG(value) ...`), the Metrics Engine intercepts the AST:

1. **Rollup Matching:** Checks if the bucket resolution (e.g. `5m`) matches an existing rollup table.
2. **Path Redirection:** If matched, the engine routes the scan to the pre-consolidated 5-minute rollup storage file instead of the raw point database file, scaling read speeds by magnitudes.
3. **Rollup Record Schema:**
   Each downsampled point record in the rollup storage uses the following schema:
   * `bucket_start_time` (8 bytes, int64) - timestamp representing start of bucket.
   * `point_count` (4 bytes, uint32) - number of raw points grouped in this bucket.
   * `sum_value` (8 bytes, float64) - sum of all point values.
   * `min_value` (8 bytes, float64) - minimum value.
   * `max_value` (8 bytes, float64) - maximum value.
4. **Storage Isolation:**
   Rollup records are saved in a **separate dedicated pager file** (e.g., `data/metrics_rollups.db`) and are not mixed with raw point pages to prevent storage fragmentation and read pollution.

---

## Tasks

1. **Implement Gorilla Compression:** Create `internal/engine/metrics/gorilla.go` containing bit-packing stream reader/writer utilities implementing the double-delta and float XOR compaction rules.
2. **Build Tag Indexing:** Create `internal/engine/metrics/index.go`. Update the append-loop in `MetricsStore` to index tags into the in-memory inverted directory.
3. **Build Downsampling Rollup Task:** Create `internal/engine/metrics/rollup.go`. Implement a background daemon worker that wakes up periodically (or on command), scans raw pages, groups points into 1-minute and 5-minute buckets, aggregates statistics, and writes them to rollup storage.
4. **Optimize Select Path:** Update `internal/engine/metrics/engine.go` to parse index matches for exact tag filters and use the rollup storage files for bucket queries.
5. **Verify Index and Compression Tests:** Add tests in `internal/engine/metrics/metrics_enterprise_test.go` verifying that compressed pages occupy less space than raw logs, tag search returns exact rows without full scans, and rollups produce accurate downsampled counts.

---

## Completion Criteria

- [ ] Bit-packing tests prove Gorilla compression delta-of-delta and float XOR logic decodes error-free.
- [ ] Queries filtering by tags execute using the inverted index, reading only matching page segments.
- [ ] Background workers generate downsampled 1-minute and 5-minute rollup files on schedule.
- [ ] Rollup query redirection reads fewer pages than scanning raw log metrics files.

