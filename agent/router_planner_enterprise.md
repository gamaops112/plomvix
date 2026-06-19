Here is the final, fully calcified, and mathematically sound **Plan 26b**, incorporating every patch, API correction, and architectural boundary we established. It is formatted in a single code block for easy copying into your `agent/` directory.

```markdown
# Plan 26b: Router & Planner Enterprise (Hardening)

| Field | Value |
| :--- | :--- |
| **Source** | `agent/router_planner_enterprise.md` |
| **Package(s)** | `internal/engine/sql/planner`, `internal/engine/sql`, `internal/router` |
| **Purpose** | Production-harden the Plan 26a Router & Planner with a thread-safe Plan Cache, Schema Version Pinning, execution telemetry, and concurrency stress testing. |
| **Dependencies** | Plan 26a (Router & Planner Setup), Global Catalog (Plans 22-23), Structured Logger (Plans 3-4). |

## Honest Contracts & Known Trade-offs
1. **Cache Stores Templates, Not Operators:** The cache stores a `PlanTemplate` (a stateless factory), not an `Operator` tree. Operators are stateful (they hold open iterators) and cannot be shared across queries. Each cache hit calls `template.Build()` to produce a fresh Operator tree.
2. **PlanTemplate is Immutable by Contract:** `Lookup()` returns a `*PlanTemplate`. Callers MUST NOT mutate the fields of the returned template. The cache does not perform defensive deep copies on read; it relies on the immutability contract to prevent data races and corruption.
3. **FIFO Eviction, Not LRU:** The basic enterprise cache uses a simple FIFO eviction policy with a configurable max size. This prevents unbounded memory growth without the complexity of a doubly-linked-list LRU. LRU is deferred to a future optimization pass.
4. **Disabled Cache Mode:** If `NewPlanCache` is called with `maxSize <= 0`, the cache is explicitly disabled. `Lookup()` will always return nil (cache miss), and `Store()` will be a no-op. This provides a safe, zero-allocation way to disable caching for tests or specific deployments.
5. **Strict Constructor Validation:** `NewSQLEngine` returns `(*SQLEngine, error)`. It strictly validates that the `SchemaVersionProvider`, `PlanCache`, and `*slog.Logger` dependencies are non-nil, returning explicit sentinel errors if they are not.
6. **Schema Version is Additive:** This plan defines a `SchemaVersionProvider` interface. The Catalog must implement it. If the Catalog from Plans 22-23 does not yet expose `SchemaVersion() uint64`, it must be added as a trivial atomic counter that increments on every DDL mutation.
7. **No Cross-Goroutine Operator Sharing:** A `PlanTemplate` is safe to read concurrently. An `Operator` tree produced by `Build()` is NOT safe to share — each query execution gets its own tree.
8. **Telemetry via Structured Logger:** All telemetry is emitted via the standard `*slog.Logger` from Plans 3-4 using key-value fields. No separate metrics subsystem is introduced in this plan.
9. **Lock Discipline (Golden Rule #4):** The cache mutex is NEVER held while calling `planner.Plan()` (which does Catalog I/O), `template.Build()`, `op.Open()`, or `op.Next()`. The pattern is: RLock → lookup → RUnlock → [miss path: Plan() → Lock → insert → Unlock] → Build → Open → Execute.
10. **Cache Stampede Prevention Deferred:** This plan does not implement singleflight or miss-reservation. If 100 goroutines hit a cold cache simultaneously, the planner will be called up to 100 times. Stampede prevention is explicitly deferred to a future optimization pass.

---

## Deliverables

| File | Purpose |
| :--- | :--- |
| `internal/engine/sql/planner/template.go` | `PlanTemplate` struct and `Build()` factory method. |
| `internal/engine/sql/planner/cache.go` | `PlanCache` with thread-safe lookup, insertion, FIFO eviction, atomic stats, disabled mode, and schema version integration. |
| `internal/engine/sql/planner/planner.go` | Refactored `Plan()` function (replaces `Translate()`) that returns a `PlanTemplate` instead of an `Operator`. |
| `internal/engine/sql/engine.go` | Updated `SQLEngine.Execute()` with cache-first flow, telemetry, and strict constructor validation. |
| `internal/engine/sql/planner/cache_test.go` | Cache unit tests: hit, miss, eviction, disabled mode, schema version invalidation, concurrent access. |
| `internal/engine/sql/engine_enterprise_test.go` | SQL Engine concurrency stress tests: 50 goroutines planning simultaneously while a background goroutine bumps schema version. |
| `docs/router_planner_enterprise.md` | Enterprise architecture documentation. |

---

## Key API & Concepts

### 1. PlanTemplate (`internal/engine/sql/planner`)

The `PlanTemplate` is a **stateless, immutable factory** that captures the result of planning (bound expressions, schema, table ID) without holding any execution state (iterators, open files).

```go
package planner

import (
    "github.com/plomvix/plomvix/internal/engine"
)

// PlanTemplate is the cacheable output of the planner.
// It is safe to read concurrently from multiple goroutines.
// CONTRACT: Callers MUST NOT mutate the fields of a PlanTemplate after it is built or returned from the cache.
type PlanTemplate struct {
    TableID      uint64
    InputSchema  engine.Schema   // Schema of the source table (deep copy)
    OutputSchema engine.Schema   // Schema of the projected output (deep copy)
    WhereExpr    BoundExpr       // nil if no WHERE clause
    Projections  []ProjectionExpr // nil if SELECT *
}

// Build instantiates a fresh, stateful Operator tree from this template.
// The returned Operator has NOT been opened — the caller must call Open().
// Each call to Build produces an independent tree with no shared mutable state.
func (t *PlanTemplate) Build(heap TableHeap, decoder RowDecoder) Operator
```

### 2. Plan Function (Replaces Translate)

The existing `Translate()` function from Plan 26a is refactored into `Plan()`, which returns a `PlanTemplate` instead of an `Operator`. The actual Operator tree construction is deferred to `PlanTemplate.Build()`.

```go
package planner

// Plan resolves schemas, binds expressions, and produces a cacheable PlanTemplate.
// It does NOT create any Operators or open any iterators.
// It acquires Catalog locks only briefly for deep-copy schema resolution (Golden Rule #4).
func Plan(
    ctx context.Context,
    cat catalog.Catalog,
    req *engine.Request,
) (*PlanTemplate, error)
```

### 3. SchemaVersionProvider

```go
package planner

// SchemaVersionProvider abstracts the Catalog's DDL version counter.
// The Global Catalog must implement this interface.
type SchemaVersionProvider interface {
    SchemaVersion() uint64
}
```

### 4. PlanCache (`internal/engine/sql/planner`)

```go
package planner

import (
    "fmt"
    "sync"
    "sync/atomic"
)

// CacheKey uniquely identifies a cached plan.
type CacheKey struct {
    Fingerprint   string // AST fingerprint from sqlparser.Statement.Fingerprint()
    SchemaVersion uint64 // Catalog DDL version at time of planning
}

func (k CacheKey) String() string {
    return fmt.Sprintf("%s@v%d", k.Fingerprint, k.SchemaVersion)
}

// PlanCache is a thread-safe, bounded, FIFO-evicting cache of PlanTemplates.
type PlanCache struct {
    mu       sync.RWMutex
    maxSize  int
    disabled bool          // If true, Lookup always misses, Store is a no-op
    items    map[string]*PlanTemplate  // key: CacheKey.String()
    order    []string                  // FIFO insertion order for eviction
    
    // Atomic counters allow safe incrementing under RLock.
    hits     atomic.Uint64
    misses   atomic.Uint64
}

// NewPlanCache creates a new PlanCache. If maxSize <= 0, the cache is disabled:
// Lookup always returns nil, and Store is a no-op. This is useful for testing
// or disabling caching without changing call sites.
func NewPlanCache(maxSize int) *PlanCache {
    if maxSize <= 0 {
        return &PlanCache{disabled: true}
    }
    return &PlanCache{
        maxSize: maxSize,
        items:   make(map[string]*PlanTemplate),
    }
}

// Lookup returns the cached template if present. Returns nil on miss.
// Acquires RLock only. Does NOT call any external I/O.
// Increments atomic hit/miss counters.
func (c *PlanCache) Lookup(key CacheKey) *PlanTemplate

// Store inserts a template into the cache. If the cache is full,
// the oldest entry (FIFO) is evicted.
// Acquires Lock. Does NOT call any external I/O.
func (c *PlanCache) Store(key CacheKey, template *PlanTemplate)

// Stats returns current cache statistics.
func (c *PlanCache) Stats() (hits, misses uint64, size int)
```

### 5. Updated SQLEngine Execute Flow (`internal/engine/sql`)

```go
package sql

import (
    "context"
    "errors"
    "fmt"
    "log/slog"
    "time"
    
    "github.com/plomvix/plomvix/internal/engine"
    "github.com/plomvix/plomvix/internal/engine/sql/planner"
    "github.com/plomvix/plomvix/internal/catalog"
)

// Sentinel errors for SQLEngine construction.
var (
    ErrNilSchemaVersionProvider = errors.New("sql engine: nil schema version provider")
    ErrNilPlanCache             = errors.New("sql engine: nil plan cache")
    ErrNilLogger                = errors.New("sql engine: nil logger")
)

type SQLEngine struct {
    catalog  catalog.Catalog
    versions planner.SchemaVersionProvider 
    tables   planner.TableRegistry
    decoder  planner.RowDecoder
    cache    *planner.PlanCache
    log      *slog.Logger                  
}

// NewSQLEngine constructs a new SQL Engine. It returns an error if any of the
// critical dependencies (versions, cache, log) are nil.
func NewSQLEngine(
    cat catalog.Catalog,
    versions planner.SchemaVersionProvider,
    tables planner.TableRegistry,
    decoder planner.RowDecoder,
    cache *planner.PlanCache,
    log *slog.Logger,
) (*SQLEngine, error) {
    if versions == nil {
        return nil, ErrNilSchemaVersionProvider
    }
    if cache == nil {
        return nil, ErrNilPlanCache
    }
    if log == nil {
        return nil, ErrNilLogger
    }
    return &SQLEngine{
        catalog:  cat,
        versions: versions,
        tables:   tables,
        decoder:  decoder,
        cache:    cache,
        log:      log,
    }, nil
}

func (e *SQLEngine) Name() string { return "sql" }

func (e *SQLEngine) Execute(ctx context.Context, req *engine.Request) (engine.RowStream, error) {
    start := time.Now()
    
    // 1. Build cache key
    fingerprint := req.Stmt.Fingerprint()
    schemaVersion := e.versions.SchemaVersion()
    key := planner.CacheKey{
        Fingerprint:   fingerprint,
        SchemaVersion: schemaVersion,
    }
    
    // 2. Cache lookup (RLock only, no I/O)
    tmpl := e.cache.Lookup(key)
    
    if tmpl == nil {
        // 3. Cache miss — plan from scratch (Catalog I/O happens here, outside cache lock)
        e.log.Debug("planner", "event", "cache_miss", "fingerprint", fingerprint)
        
        planStart := time.Now()
        var err error
        tmpl, err = planner.Plan(ctx, e.catalog, req)
        if err != nil {
            return nil, err
        }
        
        e.log.Debug("planner",
            "event", "plan_generated",
            "fingerprint", fingerprint,
            "latency_ns", time.Since(planStart).Nanoseconds(),
        )
        
        // 4. Store in cache (Lock, no I/O)
        e.cache.Store(key, tmpl)
    } else {
        e.log.Debug("planner", "event", "cache_hit", "fingerprint", fingerprint)
    }
    
    // 5. Resolve the physical heap (outside all locks)
    heap, err := e.tables.GetTableHeap(tmpl.TableID)
    if err != nil {
        // Wrap error with TableID for easier debugging
        return nil, fmt.Errorf("sql engine: table heap %d: %w", tmpl.TableID, planner.ErrTableHeapNotFound)
    }
    
    // 6. Build fresh Operator tree from template (no locks held)
    op := tmpl.Build(heap, e.decoder)
    
    // 7. Open the tree (lifecycle boundary)
    if err := op.Open(ctx); err != nil {
        _ = op.Close()
        return nil, err
    }
    
    e.log.Debug("planner",
        "event", "plan_opened",
        "fingerprint", fingerprint,
        "total_latency_ns", time.Since(start).Nanoseconds(),
    )
    
    // 8. Return the adapted stream
    return &operatorStream{op: op}, nil
}
```

---

## Dependency Graph (Unchanged, Extended)

```text
router -> internal/engine (via the generic Engine interface only)
internal/engine/sql -> planner (uses PlanCache, PlanTemplate, Plan())
planner -> internal/engine + catalog (consumes Row/Schema interfaces and Catalog metadata)
```

The cache lives entirely within the `planner` package. The SQL Engine owns the cache instance and orchestrates the lookup/miss/build flow. No new dependency edges are introduced.

---

## Tasks (8 Total)

### Task 1: PlanTemplate & Build Factory
*   Create `internal/engine/sql/planner/template.go`.
*   Define `PlanTemplate` struct with `TableID`, `InputSchema`, `OutputSchema`, `WhereExpr`, and `Projections`.
*   Implement `Build(heap TableHeap, decoder RowDecoder) Operator`:
    1. Create `SeqScanNode` with the heap and decoder.
    2. If `WhereExpr != nil`, wrap in `FilterNode`.
    3. If `Projections != nil`, wrap in `ProjectNode`.
    4. Return the root Operator (NOT opened).
*   Add unit test: create a PlanTemplate manually, call Build(), open the tree, pull rows, verify correctness.
*   Add unit test: call Build() twice on the same template, verify the two Operator trees are completely independent (closing one does not affect the other).

### Task 2: Refactor Translate → Plan
*   Refactor the existing `Translate()` function from Plan 26a into `Plan()`.
*   `Plan()` does everything `Translate()` did (schema resolution, binding, multi-table rejection) but returns a `*PlanTemplate` instead of an `Operator`.
*   The Operator tree construction logic moves to `PlanTemplate.Build()` (Task 1).
*   Update all existing Plan 26a tests to use the new `Plan()` + `Build()` flow.
*   Ensure `go test ./internal/engine/sql/planner/...` still passes.

### Task 3: PlanCache Implementation
*   Create `internal/engine/sql/planner/cache.go`.
*   Define `CacheKey` struct with `Fingerprint` and `SchemaVersion`.
*   Implement `PlanCache` with `sync.RWMutex`, bounded map, FIFO eviction via insertion-order slice, `atomic.Uint64` for hits/misses, and a `disabled bool` flag.
*   Implement `NewPlanCache(maxSize int)`: if `maxSize <= 0`, set `disabled = true`.
*   Implement `Lookup()`: if `disabled`, increment miss and return nil. Otherwise, RLock → map lookup → RUnlock. Increment atomic hit/miss counter.
*   Implement `Store()`: if `disabled`, return immediately. Otherwise, Lock → if at capacity, evict oldest key → insert new entry → Unlock.
*   Implement `Stats()` returning hits, misses, and current size (0 if disabled).

### Task 4: PlanCache Unit Tests
*   Create `internal/engine/sql/planner/cache_test.go`.
*   Write table-driven tests:
    *   **Miss on empty cache:** Lookup returns nil.
    *   **Hit after Store:** Store a template, Lookup returns the same template.
    *   **FIFO eviction:** Set maxSize=3, store 4 entries, verify the first entry is evicted.
    *   **Disabled mode:** Create cache with maxSize=0. Store a template, verify Lookup returns nil. Verify Stats() shows 0 size.
    *   **Schema version invalidation:** Store with version=1, Lookup with version=2 returns nil (different key).
    *   **Concurrent access:** 50 goroutines doing Lookup and Store simultaneously. `go test -race` must pass.

### Task 5: SchemaVersionProvider & Catalog Integration
*   Define `SchemaVersionProvider` interface in the `planner` package.
*   Document that the Global Catalog must implement `SchemaVersion() uint64`.
*   If the Catalog does not yet have this method, add a trivial implementation: an `atomic.Uint64` field that increments on every DDL mutation (CREATE TABLE, DROP TABLE, ALTER TABLE).
*   Add a unit test proving that SchemaVersion increments after a mock DDL operation.

### Task 6: Updated SQLEngine with Cache-First Flow & Validation
*   Update `SQLEngine` struct to include `versions planner.SchemaVersionProvider`, `cache *planner.PlanCache`, and `log *slog.Logger`.
*   Define sentinel errors: `ErrNilSchemaVersionProvider`, `ErrNilPlanCache`, `ErrNilLogger`.
*   Update `NewSQLEngine()` to return `(*SQLEngine, error)`. Validate that `versions`, `cache`, and `log` are non-nil, returning the corresponding sentinel error if they are nil.
*   Rewrite `Execute()` to follow the exact cache-first flow defined in the Key API section, including the `fmt.Errorf` wrapping for `GetTableHeap` failures.
*   Update all existing SQL Engine tests to handle the new error return from `NewSQLEngine()` and pass the new dependencies.

### Task 7: Concurrency Stress Tests
*   Create `internal/engine/sql/engine_enterprise_test.go` (NOT in the planner package, to avoid import cycles).
*   Implement a stress test:
    *   Create a mock Catalog, mock SchemaVersionProvider, mock TableHeap, mock RowDecoder, and a real PlanCache.
    *   Launch 50 goroutines, each calling `SQLEngine.Execute()` with the same query (same fingerprint).
    *   Launch 1 background goroutine that periodically bumps the Catalog's SchemaVersion.
    *   Assert: no panics, no races (`go test -race`), all goroutines receive valid RowStreams, cache stats are consistent (hits + misses == total queries).
*   Implement a cache stampede test:
    *   100 goroutines all call Execute() simultaneously on a cold cache.
    *   Assert: no panics, no races, all goroutines eventually get valid results.
    *   *Honest Contract:* Do not assert a limit on how many times the planner is called. Stampede prevention (singleflight) is explicitly deferred.

### Task 8: Enterprise Documentation
*   Write `docs/router_planner_enterprise.md`.
*   Document:
    *   The PlanTemplate factory pattern and why Operators cannot be cached directly.
    *   The immutability contract for PlanTemplate (callers must not mutate).
    *   The cache key structure (Fingerprint + SchemaVersion).
    *   The FIFO eviction policy and its trade-offs vs LRU.
    *   The disabled cache mode (`maxSize <= 0`).
    *   The lock discipline: cache mutex is NEVER held during I/O or Operator execution.
    *   The telemetry fields emitted by the SQL Engine.
    *   Explicit deferral of cache stampede prevention (singleflight).
*   Add substring-check tests ensuring the doc contains:
    *   "PlanTemplate is safe to read concurrently"
    *   "Operator tree is NOT safe to share"
    *   "cache mutex is never held during I/O"
    *   "stampede prevention is explicitly deferred"

---

## Completion Criteria
*   All 8 tasks implemented and tested.
*   `go test ./internal/engine/... ./internal/router/... ./internal/engine/sql/...` passes.
*   `go test -race ./...` passes with zero race conditions.
*   The PlanCache correctly evicts entries when full (FIFO) and correctly no-ops when disabled.
*   `NewSQLEngine` correctly rejects nil dependencies with explicit sentinel errors.
*   Schema version bumps cause immediate cache misses for affected plans.
*   Telemetry logs are emitted for cache_hit, cache_miss, plan_generated, and plan_opened events.
*   Concurrency stress tests pass with 50+ goroutines and concurrent schema version bumps.
*   `docs/router_planner_enterprise.md` exists and passes all substring checks.
*   No new third-party dependencies introduced.
```