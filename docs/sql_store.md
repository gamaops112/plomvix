# Plomvix SQL Store

The `sql/store` package provides an ordered, **in-memory** key-value store
built on `internal/engine/sql/key` for key ordering.

## Design

- **sorted slice** — entries are stored in a single sorted slice, ordered by
  `key.Key.Compare()`
- **sort.Search** — binary search is used for lookup and insertion position
- **sync.RWMutex** — a single read/write mutex protects all access
- **half-open** range scan with **`[start, end)`** semantics
- **copy-safety** — values are copied on Put, Get, and Scan; callers cannot
  mutate store internals

This is an **in-memory** stepping stone, not a persistent storage engine.

## API

```go
func New() *Store
func (s *Store) Put(k key.Key, value []byte) error
func (s *Store) Get(k key.Key) ([]byte, error)
func (s *Store) Delete(k key.Key) error
func (s *Store) Scan(start, end key.Key) ([]Entry, error)
func (s *Store) Len() int
```

## Errors

- `ErrNotFound` — key not present in the store
- `ErrNilStore` — operation attempted on a nil `*Store`

## Enterprise Hardening

The in-memory store includes enterprise-grade hardening for production workloads:

### Stress Tests

Two dedicated stress tests validate correctness under concurrent churn:

| Test | Purpose |
|------|---------|
| `TestStore_SortedInvariantUnderConcurrentChurn` | 50 goroutines × 200 ops each across a keyspace of 500. Verifies the **sorted-invariant** is never violated after concurrent Put/Delete churn. |
| `TestStore_ConcurrentOverwriteSameKeys` | 50 goroutines × 100 writes per key across 5 keys. Verifies Len and Scan remain correct under extreme **overwrite stress** across a tight keyspace. |

### Benchmarks

| Benchmark | Pattern |
|-----------|---------|
| `BenchmarkPut` | Appends at end of slice (best case); paired with Delete to keep store size stable. |
| `BenchmarkPut_WorstCaseFrontInsert` | Inserts at position 0 (worst-case **O(n)**). |
| `BenchmarkGet` | Reads from the middle key; roughly **O(log n)**. |
| `BenchmarkDelete` | Removes the middle key; cost dominated by slice shift **O(n)**. |
| `BenchmarkScan` | Fixed 100-entry window; cost is **flat relative to store size** (just slice traversal). |

All benchmarks include **size-scaling** sub-benchmarks at 1 k, 10 k, and 100 k entries.

### Performance Profile

Operation costs for the in-memory, sorted-slice design:

| Operation | Complexity | Notes |
|-----------|------------|-------|
| Get | O(log n) | sort.Search + value copy |
| Put | O(n) | sort.Search + slice insertion |
| Delete | O(n) | sort.Search + slice deletion |
| Scan | O(log n + m) | sort.Search + copy of m entries |

### Future Direction

The sorted-slice design is a stepping stone. A persistent B-Tree or LSM-based
engine will eventually replace it, providing **on-disk storage** and
transactional semantics. All existing tests and benchmarks validate that
hardening will survive such a transition with **no API changes** to the
`Store` type.

## Non-Goals

The in-memory store intentionally does not implement:

- WAL
- disk persistence
- storage pages
- buffer pool
- **no transactions**
- compaction
- snapshots
- sharded locking
