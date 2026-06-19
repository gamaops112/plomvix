# DML Row Identity Enterprise (Mutable Heap Hardening)

| Field | Value |
| :--- | :--- |
| **Source** | `agent/dml_row_identity_enterprise.md` |
| **Package(s)** | `internal/engine`, `internal/engine/sql`, `internal/engine/sql/planner` |
| **Purpose** | Production-harden the DML Row Identity layer with RowID generation-encoded stability guarantees, strict serializable write-write conflict detection, table-level locking for concurrent scan and mutation safety, `ErrStaleRowID` / `ErrWriteConflict` sentinels, an extended `MutableTableHeap` contract with `CheckWriteConflict`, and structured telemetry for all conflict and staleness events. |
| **Dependencies** | DML Row Identity and Mutable Heap Contract plan, Table Heap Enterprise plan, DDL Enterprise execution plan |

---

## Honest Contracts & Known Trade-offs

1. **RowID is intra-transaction valid only:** `RowID` values obtained from a `HeapScanIterator` are valid only within the same transaction scope. After the transaction ends, heap vacuum/compaction may reclaim dead row versions, shifting physical offsets. Any code that stores a `RowID` across transaction boundaries will observe `ErrStaleRowID` on the next mutation attempt. No cross-transaction RowID caching is permitted. Active DML transactions pin the current `HeapGeneration`: vacuum MUST NOT advance `HeapGeneration` for any table with an in-flight scan or mutation. The concrete adapter tracks this via an active-reader/writer count (or generation-lock). A `RowID` collected during a scan remains valid until the associated mutation commits or the transaction is rolled back.
2. **RowID encoding is generation-qualified:** The enterprise RowID encoding is `(generation << 32) | (physicalOffset + 1)`. The `generation` field is the current `HeapGeneration` counter at the time the row was scanned. Decode: `physicalOffset = (rowID & 0xFFFFFFFF) - 1`, `generation = rowID >> 32`. Zero in the low 32 bits remains the sentinel for "not from a heap scan" (`ErrMissingRowID`); this is unchanged from the setup plan.
3. **HeapGeneration bumped on compaction only:** The `HeapGeneration uint64` counter in the concrete heap is incremented only when a vacuum/compaction pass reclaims dead row space and shifts physical offsets. Routine MVCC version appends do NOT bump the generation. This minimises spurious `ErrStaleRowID` returns.
4. **Strict Serializable via write-write conflict detection:** After a Volcano scan locates a target row using `ReadTxID`, before appending the new version, the concrete adapter must check whether any version with `TxID > ReadTxID` already exists for the same physical slot. If so, `CheckWriteConflict` returns `ErrWriteConflict`. This upgrades concurrency from Snapshot Isolation to Strict Serializability for DML. Read-only `SELECT` queries are unaffected.
5. **Lock ordering is strict and non-recursive:** Table-level read lock for scans, table-level write lock for mutations. These are mutually exclusive via `sync.RWMutex`. A goroutine MUST NOT hold both simultaneously. `SeqScanNode.Open()` calls heap.Scan(ctx, tx). The concrete Scan implementation acquires the table read lock and returns an iterator that owns the lock. `SeqScanNode.Close()` calls iter.Close(), which releases the read lock. `DeleteByRowID` and `UpdateByRowID` acquire the write lock before conflict detection and release it after the KV set completes. The `CheckWriteConflict` call also runs under the write lock.
6. **CheckWriteConflict is deferred for multi-row path:** Single-row path may call CheckWriteConflict before DeleteByRowID/UpdateByRowID. Multi-row path must not call CheckWriteConflict directly; it calls BatchMutate, which performs all conflict checks internally.
7. **Concurrent scan safety is documented, not magical:** Two concurrent `HeapScanIterator` instances on the same table are safe because they both hold read locks (read-read allowed by `RWMutex`). A concurrent scan + write is serialized: the write attempt blocks until the scan releases its read lock via `iter.Close()`. Multi-row mutation batches use `BatchMutate` (see MutableTableHeap), which holds the write lock for the entire batch, preventing interleaved mutations from other writers. This means a long-running scan can delay a mutation. Callers must not hold scans open across unrelated work.
8. **`ErrStaleRowID` and `ErrWriteConflict` are terminal errors:** Both errors cause the enclosing transaction to be rolled back by the SQL Engine. Retrying with a fresh transaction and a new scan is the correct recovery path. The engine MUST NOT retry internally.
9. **Telemetry via `slog` only:** All conflict and staleness events are emitted via structured `slog` at `LevelWarn`. No separate metrics subsystem is introduced. Fields: `table_id`, `row_id`, `tx_id`, `error`. Logging is best-effort and must not be on the mutation hot path under the write lock (log after unlock).
10. **`MutableTableHeap` interface is additive:** `CheckWriteConflict` is added to `MutableTableHeap`. Existing implementations from the setup plan that do not implement the new method will fail to compile, which is intentional. There is no optional interface; enterprise code requires the full contract.

---

## Deliverables

| File | Purpose |
| :--- | :--- |
| `internal/engine/engine.go` | Upgrade `RowID` encoding to `(generation << 32) \| (physicalOffset + 1)`. Document decode helpers in comments. |
| `internal/engine/errors.go` | New file. Declare `ErrMissingRowID`, `ErrRowIDOffsetOverflow`, and `ErrRowIDGenerationOverflow` to avoid import cycles from `sql` into `engine`. |
| `internal/engine/sql/mutable_heap.go` | Add `CheckWriteConflict(ctx context.Context, tx engine.TxContext, rowID uint64) error` to `MutableTableHeap`. |
| `internal/engine/sql/errors.go` | Add `ErrStaleRowID`, `ErrWriteConflict`. |
| `internal/engine/sql/planner/scan.go` | `SeqScanNode.Open()` calls heap.Scan(ctx, tx) (the concrete Scan implementation acquires table read lock); `Close()` calls iter.Close() to release it. `Next()` encodes generation into `RowID`. |
| `internal/engine/sql/engine.go` | Call `CheckWriteConflict` before single-row mutation dispatch. For multi-row path, call `BatchMutate` (conflict checking is deferred internally). Roll back on `ErrStaleRowID` / `ErrWriteConflict`. Emit `slog` after unlock. |
| Concrete heap adapter (within `internal/engine/sql` or adapter package) | Add `HeapGeneration uint64` field. Implement generation-qualified RowID encode/decode. Implement `CheckWriteConflict` under write lock. Bump generation on compaction. |
| `internal/engine/sql/dml_enterprise_test.go` | Concurrency tests: scan + concurrent write serialization, write-write conflict detection, stale RowID detection, generation bump on compaction. |
| `docs/dml_row_identity_enterprise.md` | Architecture documentation for enterprise Row Identity hardening. |

---

## Key API & Concepts

### 1. Generation-Qualified RowID Encoding (`internal/engine/engine.go`)

The enterprise RowID encodes both a heap generation counter and the physical offset:

```go
// RowID enterprise encoding:
//   RowID = (generation << 32) | (physicalOffset + 1)
//
// Decode:
//   physicalOffset = (rowID & 0xFFFFFFFF) - 1
//   generation     = rowID >> 32
//
// Zero in the low 32 bits is the sentinel for "not from a heap scan" (ErrMissingRowID).
// Zero is never a valid physicalOffset+1 result.
//
// generation is the HeapGeneration counter at scan time.
// If the heap generation has advanced since the RowID was issued, ErrStaleRowID is returned.

// EncodeRowID produces a generation-qualified RowID.
// physicalOffset must be <= math.MaxUint32-1; exceeding this returns ErrRowIDOffsetOverflow.
// generation must be <= math.MaxUint32; exceeding this returns ErrRowIDGenerationOverflow.
func EncodeRowID(generation uint64, physicalOffset uint64) (uint64, error) {
    if generation > math.MaxUint32 {
        return 0, ErrRowIDGenerationOverflow
    }
    if physicalOffset > math.MaxUint32-1 {
        return 0, ErrRowIDOffsetOverflow
    }
    return (generation << 32) | (physicalOffset + 1), nil
}

// DecodeRowID unpacks a generation-qualified RowID.
// Returns ErrMissingRowID if the low 32 bits are zero (sentinel).
func DecodeRowID(rowID uint64) (generation uint64, physicalOffset uint64, err error) {
    low := rowID & 0xFFFFFFFF
    if low == 0 {
        return 0, 0, ErrMissingRowID
    }
    return rowID >> 32, low - 1, nil
}
```

### 2. Sentinel Errors (`internal/engine/sql/errors.go`)

```go
// internal/engine/errors.go  (new file — prevents import-cycle from sql → engine)
var ErrMissingRowID = errors.New("engine: row has no physical RowID; cannot mutate")
var ErrRowIDOffsetOverflow = errors.New("engine: physicalOffset exceeds 32-bit RowID encoding limit")
var ErrRowIDGenerationOverflow = errors.New("engine: heap generation exceeds 32-bit RowID encoding limit")

// internal/engine/sql/errors.go  (enterprise additions)
var (
    // Existing (from setup plan):
    ErrWhereRequired           = errors.New("engine: UPDATE and DELETE require a WHERE clause")
    ErrHeapMutationUnsupported = errors.New("engine: target table heap does not support mutation")
    
    // Alias to preserve backward compatibility for existing code/tests pointing to sql package:
    ErrMissingRowID            = engine.ErrMissingRowID

    // Enterprise additions:
    ErrStaleRowID                = errors.New("engine: RowID is stale; heap generation has advanced (vacuum ran)")
    ErrWriteConflict             = errors.New("engine: write-write conflict detected; a concurrent transaction modified this row")
    ErrVacuumBlockedByActivePins = errors.New("engine: vacuum compaction blocked by active DML transaction generation pins")
)
```

### 3. Extended `MutableTableHeap` Interface (`internal/engine/sql/mutable_heap.go`)

```go
package sql

import (
    "context"
    "github.com/plomvix/plomvix/internal/engine"
)

// MutableTableHeap is the engine-facing contract for physical row mutation.
// Enterprise version adds CheckWriteConflict for strict serializable conflict detection.
type MutableTableHeap interface {
    // CheckWriteConflict checks whether any version with TxID > tx.ReadTxID exists
    // for the row identified by rowID. Must be called AFTER asserting MutableTableHeap
    // and BEFORE DeleteByRowID or UpdateByRowID.
    // Also validates that the RowID generation matches the current HeapGeneration.
    // Returns ErrWriteConflict, ErrStaleRowID, or nil.
    CheckWriteConflict(ctx context.Context, tx engine.TxContext, rowID uint64) error

    // DeleteByRowID appends a tombstone for the exact row version identified by rowID.
    // Returns ErrStaleRowID if the RowID generation does not match current HeapGeneration.
    DeleteByRowID(ctx context.Context, tx engine.TxContext, rowID uint64) error

    // UpdateByRowID appends a new row version replacing the exact row version identified by rowID.
    // newValues must have the same column count as the table schema.
    // Returns ErrStaleRowID if the RowID generation does not match current HeapGeneration.
    // Returns ErrPrimaryKeyUpdate if any PK column value differs from the original.
    UpdateByRowID(ctx context.Context, tx engine.TxContext, rowID uint64, newValues []engine.Datum) error

    // BatchMutate acquires the table write lock ONCE, runs CheckWriteConflict for every
    // mutation in the batch, applies all mutations sequentially, then releases the lock.
    // Returns (rowsAffected, error). On conflict or I/O failure, releases the lock and
    // returns immediately; rows mutated before the failure are NOT rolled back until
    // WAL rollback support exists (partial writes are documented in Honest Contracts).
    // Enterprise multi-row UPDATE/DELETE use this method; single-row mutations
    // may still use DeleteByRowID/UpdateByRowID directly (each acquires lock per call).
    BatchMutate(ctx context.Context, tx engine.TxContext, mutations []RowMutation) (int, error)
}

// RowMutation describes a single row operation within a BatchMutate call.
type RowMutation struct {
    RowID     uint64        // physical RowID from SeqScanNode
    Op        MutationOp    // OpDelete or OpUpdate
    NewValues []engine.Datum // nil for OpDelete
}

type MutationOp uint8
const (
    OpDelete MutationOp = iota
    OpUpdate
)
```

### 4. Concrete Heap Adapter — `HeapGeneration`, Pinning, and Conflict Check

```go
// HeapGeneration is bumped only when vacuum/compaction physically reclaims dead row space
// and shifts physical offsets. Routine MVCC appends do NOT bump it.
type concreteHeapAdapter struct {
    mu             sync.RWMutex
    heapGeneration  uint64 // atomic; read with Load, bump with Add
    activePins      int64  // atomic count of in-flight scans or mutations pinning current generation
    // ... other fields
}

// Private locked helpers to prevent deadlock inside BatchMutate (no lock re-entrancy)
func (a *concreteHeapAdapter) checkWriteConflictLocked(tx engine.TxContext, rowID uint64) error {
    gen, physicalOffset, err := engine.DecodeRowID(rowID)
    if err != nil {
        return err
    }
    currentGen := atomic.LoadUint64(&a.heapGeneration)
    if gen != currentGen {
        return sql.ErrStaleRowID
    }
    if a.hasNewerVersion(physicalOffset, tx.ReadTxID) {
        return sql.ErrWriteConflict
    }
    return nil
}

func (a *concreteHeapAdapter) deleteByRowIDLocked(tx engine.TxContext, rowID uint64) error {
    // raw delete implementation
    return nil
}

func (a *concreteHeapAdapter) updateByRowIDLocked(tx engine.TxContext, rowID uint64, values []engine.Datum) error {
    // raw update implementation
    return nil
}

// CheckWriteConflict acquires the write lock and delegates to checkWriteConflictLocked.
// As it is a transient pre-flight query, it does not pin the generation outside its lock hold.
func (a *concreteHeapAdapter) CheckWriteConflict(ctx context.Context, tx engine.TxContext, rowID uint64) error {
    a.mu.Lock()
    defer a.mu.Unlock()
    return a.checkWriteConflictLocked(tx, rowID)
}

// DeleteByRowID acquires the write lock and delegates to deleteByRowIDLocked under generation pinning.
func (a *concreteHeapAdapter) DeleteByRowID(ctx context.Context, tx engine.TxContext, rowID uint64) error {
    atomic.AddInt64(&a.activePins, 1)
    defer atomic.AddInt64(&a.activePins, -1)

    a.mu.Lock()
    defer a.mu.Unlock()
    
    if err := a.checkWriteConflictLocked(tx, rowID); err != nil {
        return err
    }
    return a.deleteByRowIDLocked(tx, rowID)
}

// UpdateByRowID acquires the write lock and delegates to updateByRowIDLocked under generation pinning.
func (a *concreteHeapAdapter) UpdateByRowID(ctx context.Context, tx engine.TxContext, rowID uint64, values []engine.Datum) error {
    atomic.AddInt64(&a.activePins, 1)
    defer atomic.AddInt64(&a.activePins, -1)

    a.mu.Lock()
    defer a.mu.Unlock()

    if err := a.checkWriteConflictLocked(tx, rowID); err != nil {
        return err
    }
    return a.updateByRowIDLocked(tx, rowID, values)
}

// BatchMutate acquires the write lock once, conflict checks every mutation, and applies them.
func (a *concreteHeapAdapter) BatchMutate(ctx context.Context, tx engine.TxContext, mutations []RowMutation) (int, error) {
    // Pin generation before acquiring write lock
    atomic.AddInt64(&a.activePins, 1)
    defer atomic.AddInt64(&a.activePins, -1)

    a.mu.Lock()
    defer a.mu.Unlock()

    // 1. Conflict check all mutations first (all-or-nothing pre-flight)
    for _, m := range mutations {
        if err := a.checkWriteConflictLocked(tx, m.RowID); err != nil {
            return 0, err
        }
    }

    // 2. Apply all mutations sequentially under the write lock
    rowsAffected := 0
    for _, m := range mutations {
        var err error
        if m.Op == OpDelete {
            err = a.deleteByRowIDLocked(tx, m.RowID)
        } else {
            err = a.updateByRowIDLocked(tx, m.RowID, m.NewValues)
        }
        if err != nil {
            return rowsAffected, err // partial write is documented in contracts
        }
        rowsAffected++
    }
    return rowsAffected, nil
}

// bumpGeneration is called by the vacuum/compaction pass after reclaiming dead row space.
// Returns ErrVacuumBlockedByActivePins if active DML transactions or scans pin the current generation.
func (a *concreteHeapAdapter) bumpGeneration() error {
    if atomic.LoadInt64(&a.activePins) != 0 {
        return ErrVacuumBlockedByActivePins
    }
    atomic.AddUint64(&a.heapGeneration, 1)
    return nil
}
```

### 5. `SeqScanNode` Lock Acquisition (`internal/engine/sql/planner/scan.go`)

```go
// SeqScanNode stores tx at construction time — Open() signature stays Open(ctx context.Context)
// to preserve the existing Operator interface contract.
//
// Lock model: Scan() implementation acquires the table read lock before returning the iterator.
// The iterator holds the read lock for its entire lifetime. iter.Close() releases it.
// SeqScanNode never directly accesses heap.mu — lock is fully encapsulated in the iterator.

// NewSeqScanNode now accepts tx so it can be passed to Scan() at Open() time.
func NewSeqScanNode(heap TableHeap, schema engine.Schema, decoder RowDecoder, tx engine.TxContext) *SeqScanNode

// Open calls heap.Scan(ctx, n.tx) which acquires read lock and returns a locked iterator.
// Operator interface signature is preserved: Open(ctx context.Context) error.
func (n *SeqScanNode) Open(ctx context.Context) error {
    iter, err := n.heap.Scan(ctx, n.tx) // Scan() acquires read lock internally
    if err != nil { return err }
    n.iter = iter
    return nil
}

// Close delegates to iter.Close() which releases the read lock.
func (n *SeqScanNode) Close() error {
    return n.iter.Close() // iter.Close() releases read lock
}

// Next reads the generation-qualified RowID from the iterator.
func (n *SeqScanNode) Next(ctx context.Context) (engine.Row, error) {
    encoded, rawRowID, err := n.iter.Next(ctx)
    if err != nil { return engine.Row{}, err }
    row, err := n.decoder.Decode(encoded, n.schema)
    if err != nil { return engine.Row{}, err }
    row.RowID = rawRowID // already (generation << 32) | (physicalOffset + 1)
    return row, nil
}
```

### 6. Engine Dispatch with `CheckWriteConflict` and Telemetry (`internal/engine/sql/engine.go`)

```go
// execMutation is the pre-flight check only used by single-row direct mutation paths
// (e.g. basic tier compatibility or single-row fast paths).
// Multi-row enterprise paths bypass this completely and delegate all checks to BatchMutate.
func (e *SQLEngine) execMutation(
    ctx context.Context,
    tx engine.TxContext,
    mh sql.MutableTableHeap,
    tableID uint64,
    rowID uint64,
) error {
    if err := mh.CheckWriteConflict(ctx, tx, rowID); err != nil {
        // Log after returning from CheckWriteConflict (lock is already released inside adapter).
        e.logger.Warn("dml: mutation pre-flight failed",
            slog.Uint64("table_id", tableID),
            slog.Uint64("row_id", rowID),
            slog.Uint64("tx_id", tx.WriteTxID),
            slog.String("error", err.Error()),
        )
        return err // ErrStaleRowID or ErrWriteConflict; caller must roll back transaction
    }
    return nil
}
```

### 7. Lock Ordering Contract (Documented)

```
Lock ordering rules (MUST NOT be violated):
  1. Scan path:    heap.Scan(ctx, tx) acquires RLock → iterator.Next() runs under RLock → iterator.Close() releases RLock
                   SeqScanNode never touches heap.mu directly.
  2. Mutation path (single-row): DeleteByRowID/UpdateByRowID each acquire Lock() → check conflict → KV set → Unlock()
  3. Mutation path (multi-row):  BatchMutate acquires Lock() once → CheckWriteConflict all rows → apply all → Unlock()
  4. A goroutine MUST NOT hold read lock and write lock simultaneously on the same table.
  5. Two concurrent scans: both hold RLock → safe (RWMutex allows concurrent readers).
  6. Concurrent scan + BatchMutate: BatchMutate blocks until all scan iterators Close().
  7. slog emission: ALWAYS after mutex release; never under the lock.
  8. Engine-layer CheckWriteConflict call: only for single-row mutations before DeleteByRowID/UpdateByRowID; multi-row calls BatchMutate directly without engine-layer pre-flight loops.
```

### 8. RowID Generation Bump on Compaction

```go
// vacuumTable is called by the background vacuum worker after compaction.
// It reclaims dead row versions, potentially shifting physical offsets.
// It MUST bump HeapGeneration so all pre-compaction RowIDs become stale.
// If any active reader/writer holds a pin on the current generation, vacuum skips compaction.
func (w *vacuumWorker) vacuumTable(ctx context.Context, tableID uint64) error {
    adapter := w.getAdapter(tableID)
    if atomic.LoadInt64(&adapter.activePins) != 0 {
        // Skip vacuum and compaction since active DML transactions pin the current generation.
        return ErrVacuumBlockedByActivePins
    }
    // ... perform compaction ...
    if err := adapter.bumpGeneration(); err != nil {
        return err
    }
    w.logger.Info("vacuum: heap generation bumped",
        slog.Uint64("table_id", tableID),
        slog.Uint64("new_generation", atomic.LoadUint64(&adapter.heapGeneration)),
    )
    return nil
}
```

---

## Tasks

1. **Upgrade RowID encoding:** Add `EncodeRowID(generation, physicalOffset uint64) (uint64, error)` validating both physical offset overflow (`ErrRowIDOffsetOverflow`) and generation overflow (`ErrRowIDGenerationOverflow`) and `DecodeRowID(rowID uint64) (generation, physicalOffset uint64, err error)` to `internal/engine/engine.go`. Document the encoding contract in package-level comments.
2. **Add enterprise sentinel errors:** Add `ErrStaleRowID` and `ErrWriteConflict` to `internal/engine/sql/errors.go`, and define `ErrRowIDGenerationOverflow` in `internal/engine/errors.go`. Add `ErrMissingRowID = engine.ErrMissingRowID` alias to `internal/engine/sql/errors.go`.
3. **Add `HeapGeneration` and Generation Pinning to concrete adapter:** Add `heapGeneration uint64` and `activePins int64` fields (accessed via `sync/atomic`). Implement generation pinning: increment `activePins` when `Scan()` returns iterator, decrement in `iterator.Close()`; increment `activePins` at start of mutations/`BatchMutate`, decrement on completion. Compaction must verify `activePins == 0` before shifting offsets or calling `bumpGeneration()`.
4. **Update `HeapScanIterator`:** Iterator must emit generation-qualified RowIDs: `(currentGeneration << 32) | (physicalOffset + 1)`. Never emits zero in the low 32 bits.
5. **Extend `MutableTableHeap`:** Add `CheckWriteConflict(ctx context.Context, tx engine.TxContext, rowID uint64) error` and `BatchMutate(ctx context.Context, tx engine.TxContext, mutations []RowMutation) (int, error)` to the interface in `internal/engine/sql/mutable_heap.go`.
6. **Implement conflict check and locked helpers in concrete adapter:** Implement locked helper methods `checkWriteConflictLocked`, `deleteByRowIDLocked`, and `updateByRowIDLocked` to prevent deadlock when `BatchMutate` calls them under the same write lock. Under write lock: check write conflict locked compares generation (returns `ErrStaleRowID` on mismatch) and scans versions (returns `ErrWriteConflict` on conflict).
7. **Update `DeleteByRowID` and `UpdateByRowID`:** Both must acquire the table write lock before performing conflict detection + KV set (delegating to locked helpers). Release after KV set. Validate generation as a secondary guard even after `CheckWriteConflict` (lock-based safety net).
8. **Update `SeqScanNode`:** `SeqScanNode` is updated with `tx engine.TxContext` constructor param; `Open()` calls `heap.Scan()` which manages locks and increments pins; `SeqScanNode` never accesses `heap.mu` directly. Also note `Scan()` concrete impl must acquire read lock before returning iterator; `iter.Close()` releases lock and decrements pins. `Next()` passes generation-qualified RowID through directly from iterator.
9. **Update engine dispatch:** In single-row mutations, engine may call `CheckWriteConflict` as a pre-flight. For multi-row mutations, the engine builds `[]RowMutation` and calls `BatchMutate` (CheckWriteConflict is deferred to BatchMutate internally, which does the conflict check on all rows under the write lock).
10. **Add structured telemetry:** In `execMutation` (for single-row pre-flight) or on `BatchMutate` failure, emit `slog.Warn` with `table_id`, `row_id`, `tx_id`, `error`. Logging must occur after the mutex has been released by the adapter.
11. **Tests:** Write `internal/engine/sql/dml_enterprise_test.go` covering: (a) generation-qualified RowID encode/decode round-trip; (b) `ErrStaleRowID` on generation mismatch; (c) `ErrWriteConflict` on concurrent write detected; (d) scan + concurrent write is serialized (read lock blocks write lock); (e) two concurrent scans succeed; (f) `bumpGeneration` invalidates pre-compaction RowIDs; (g) test for overflow guards (offset and generation) in `EncodeRowID` and vacuum pin rule.

---

## Completion Criteria

- [ ] `EncodeRowID` and `DecodeRowID` exist in `internal/engine/engine.go`; encoding is `(generation << 32) | (physicalOffset + 1)`; decode returns `ErrMissingRowID` for zero low bits.
- [ ] `ErrStaleRowID` and `ErrWriteConflict` exist in `internal/engine/sql/errors.go`.
- [ ] `HeapGeneration uint64` field exists in the concrete heap adapter; `bumpGeneration()` is called only by the vacuum/compaction path.
- [ ] `HeapScanIterator.Next()` emits generation-qualified RowIDs; never emits zero in the low 32 bits.
- [ ] `MutableTableHeap` interface includes `CheckWriteConflict(ctx, tx engine.TxContext, rowID uint64) error`.
- [ ] `CheckWriteConflict` in concrete adapter: validates generation (→ `ErrStaleRowID`) and checks for newer TxID versions (→ `ErrWriteConflict`) under the write lock.
- [ ] `DeleteByRowID` and `UpdateByRowID` acquire table write lock before conflict detection + KV set; release after KV set.
- [ ] `SeqScanNode.Open()` calls heap.Scan(ctx, tx) which acquires the table read lock; `SeqScanNode.Close()` calls iter.Close() which releases it.
- [ ] Single-row direct mutation path: engine calls CheckWriteConflict before DeleteByRowID/UpdateByRowID; multi-row path: engine calls BatchMutate which performs conflict checks internally.
- [ ] Engine rolls back on `ErrStaleRowID` or `ErrWriteConflict`; does not retry internally.
- [ ] `slog.Warn` emitted with `table_id`, `row_id`, `tx_id`, `error` on both `ErrStaleRowID` and `ErrWriteConflict`; logged after mutex release.
- [ ] Lock ordering contract documented: read lock for scans, write lock for mutations, never held simultaneously, `slog` always outside the lock.
- [ ] `ErrMissingRowID`, `ErrRowIDOffsetOverflow`, and `ErrRowIDGenerationOverflow` declared in `internal/engine/errors.go`; alias `ErrMissingRowID` in `internal/engine/sql/errors.go` exists.
- [ ] `EncodeRowID` returns `(uint64, error)` and returns `ErrRowIDOffsetOverflow` when `physicalOffset > math.MaxUint32-1`, and `ErrRowIDGenerationOverflow` when `generation > math.MaxUint32`.
- [ ] `Scan()` concrete implementation acquires table read lock before returning iterator; `iter.Close()` releases it. `SeqScanNode` never accesses `heap.mu` directly.
- [ ] `MutableTableHeap` includes `BatchMutate(ctx, tx, mutations []RowMutation) (int, error)`.
- [ ] Enterprise multi-row UPDATE/DELETE use `BatchMutate`; single-row use `DeleteByRowID`/`UpdateByRowID`.
- [ ] Vacuum does not advance `HeapGeneration` for tables with active scans or mutations (activePins pinning mechanism).
- [ ] Adapter implements locked helpers (`checkWriteConflictLocked`, `deleteByRowIDLocked`, `updateByRowIDLocked`) to prevent recursive mutex deadlock.
- [ ] All tests pass via the following commands:
```bash
go test ./internal/engine/... ./internal/engine/sql/... ./internal/engine/sql/planner/...
go test -race ./...
```
