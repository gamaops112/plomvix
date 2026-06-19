# DML Row Identity and Mutable Heap Contract

| Field | Value |
| :--- | :--- |
| **Source** | `agent/dml_row_identity_setup.md` |
| **Package(s)** | `internal/engine`, `internal/engine/sql/planner`, `internal/engine/sql` |
| **Purpose** | Establish the missing contracts that make UPDATE and DELETE possible: a hidden `RowID` embedded in every scanned row, a `MutableTableHeap` interface using `engine.TxContext` (not `heap.Tx`), and exact grounded signatures for the Volcano WHERE predicate binder. |
| **Dependencies** | Planner / Volcano executor plan, Table Heap Enterprise plan, DML Execution Setup (INSERT) plan. |

---

## Honest Contracts & Known Trade-offs

1. **RowID is internal and opaque:** `RowID` is a hidden uint64 encoding the physical location (page + slot offset) of the row version in the heap. Never exposed to SQL callers. Not a user-visible primary key.
2. **RowID zero is reserved:** Raw physical offset is stored as `physicalOffset + 1`. Zero is the sentinel meaning "not from a heap scan." Mutations on `RowID == 0` return `ErrMissingRowID`. This guarantees no real row can collide with the sentinel even if page 0 / slot 0 exists.
3. **No PK constraint required:** Row targeting uses physical `RowID`, not logical PK columns. PK constraint enforcement deferred.
4. **`MutableTableHeap` is engine-facing:** It uses `engine.TxContext` exclusively. The internal `heap.Tx` type (from the Table Heap Enterprise plan) is constructed only inside the concrete heap adapter, never exposed to `internal/engine/sql`.
5. **`BindWhere` for DML:** Reuses the existing `planner.BindWhere` signature exactly. The SQL Engine calls it with the WHERE expression and schema. `ErrUnsupportedFeature` from the binder is mapped by the engine to a DML-specific sentinel.
6. **`SeqScanNode` and `FilterNode` constructors are grounded:** This plan locks down their exact signatures so the UPDATE/DELETE plan can reference them without ambiguity.
7. **WHERE is required for mutation:** `UPDATE` or `DELETE` without a WHERE clause is rejected by the SQL Engine with `ErrWhereRequired` before the Volcano pipeline is built.

---

## Deliverables

| File | Purpose |
| :--- | :--- |
| `internal/engine/engine.go` | Add `RowID uint64` field to `engine.Row`. Add `WriteTxID uint64` to `engine.TxContext` (confirm it was added in DDL plan). |
| `internal/engine/sql/planner/plan.go` | Lock down `NewSeqScanNode` and `NewFilterNode` constructor signatures. |
| `internal/engine/sql/mutable_heap.go` | Define `MutableTableHeap` local interface using `engine.TxContext`. |
| `internal/engine/sql/errors.go` | Add `ErrWhereRequired`, `ErrHeapMutationUnsupported`, `ErrMissingRowID`. |
| `internal/engine/sql/planner/scan.go` | Update `SeqScanNode.Next()` to embed `RowID` in the returned `engine.Row`. |

---

## Key API & Concepts

### 1. `RowID` in `engine.Row` (`internal/engine/engine.go`)

`engine.Row` is extended with a hidden identity field so the SQL Engine can target the exact physical row version after Volcano locates it:

```go
// Row is a slice of Datum values representing a single tuple.
// RowID is set by SeqScanNode during a scan and is opaque to callers.
// Encoding: RowID = physicalOffset + 1. Zero means "not from a heap scan" (ErrMissingRowID).
type Row struct {
    Datums []Datum
    RowID  uint64 // physicalOffset+1; 0 = sentinel (not from heap scan)
}
```

> **Note:** All existing code using `Row` as `[]Datum` must update to `row.Datums`. `DeepCopy()` copies `RowID` unchanged (uint64, not pointer).

### 2. `engine.TxContext` (`internal/engine/engine.go`)

Confirm `WriteTxID` was added in the DDL Execution plan. The full struct is:

```go
type TxContext struct {
    ReadTxID  uint64 // Snapshot for reads; math.MaxUint64 in basic tier.
    WriteTxID uint64 // Allocated once by SQLEngine.Execute for DDL/DML; 0 for SELECT.
}
```

### 3. `MutableTableHeap` Interface (`internal/engine/sql/mutable_heap.go`)

This is the engine-facing write contract. It uses `engine.TxContext` only. The bridge to the internal `heap.Tx` type lives in the concrete adapter, not here:

```go
package sql

import (
    "context"
    "github.com/plomvix/plomvix/internal/engine"
)

// MutableTableHeap is the engine-facing contract for physical row mutation.
// Implementations bridge to the internal heap.Tx type using req.TxContext.WriteTxID.
type MutableTableHeap interface {
    // DeleteByRowID appends a tombstone for the exact row version identified by rowID.
    DeleteByRowID(ctx context.Context, tx engine.TxContext, rowID uint64) error

    // UpdateByRowID appends a new row version replacing the exact row version identified by rowID.
    // newValues must have the same column count as the table schema.
    // The concrete adapter is responsible for loading the original row by rowID and comparing
    // PK column values using its internal schema + table context. Returns ErrPrimaryKeyUpdate
    // if any PK column value differs from the original.
    UpdateByRowID(ctx context.Context, tx engine.TxContext, rowID uint64, newValues []engine.Datum) error
}
```

> **Note:** `DeleteByRowID` and `UpdateByRowID` use `RowID` to target mutations. No PK metadata needed at the engine layer. The concrete adapter resolves original row + PK metadata internally from its table context and the provided `rowID`.

### 4. New Sentinel Errors (`internal/engine/sql/errors.go`)

```go
var (
    ErrWhereRequired           = errors.New("engine: UPDATE and DELETE require a WHERE clause")
    ErrHeapMutationUnsupported = errors.New("engine: target table heap does not support mutation")
    ErrMissingRowID            = errors.New("engine: row has no physical RowID; cannot mutate")
)
```

### 5. Grounded `TableHeap`, `HeapScanIterator`, and Constructors

**`TableHeap` scan contract** — the exact method `SeqScanNode` depends on:

```go
// TableHeap is the planner-facing read contract for the physical heap.
type TableHeap interface {
    // Scan returns an iterator over all row versions visible to tx.
    Scan(ctx context.Context, tx engine.TxContext) (HeapScanIterator, error)
}

// HeapScanIterator yields encoded tuples with their physical RowID.
// RowID = physicalOffset + 1. Iterator never emits RowID == 0.
type HeapScanIterator interface {
    Next(ctx context.Context) (encodedTuple []byte, rowID uint64, err error) // io.EOF when done
    Close() error
}
```

**Constructors** — exact signatures for UPDATE/DELETE:

```go
// NewSeqScanNode constructs a full-table scan. Decoder produces engine.Row with RowID set.
func NewSeqScanNode(heap TableHeap, schema engine.Schema, decoder RowDecoder) *SeqScanNode

// NewFilterNode wraps child; yields only rows where predicate evals true.
func NewFilterNode(child Operator, predicate BoundExpr) *FilterNode
```

> **RowID population rule:** `SeqScanNode.Next()` sets `row.RowID = rawRowID` from iterator. Raw value is already `physicalOffset + 1`; no further adjustment needed.

### 6. Grounded `BindWhere` Signature (`internal/engine/sql/planner/binder.go`)

Already established in the Planner / Volcano executor plan. Confirmed exact signature:

```go
// BindWhere walks the Vitess WHERE expression and builds a BoundExpr evaluator.
// Returns ErrUnsupportedFeature for sub-selects, functions, or unsupported operators.
func BindWhere(expr vitess.Expr, schema engine.Schema) (BoundExpr, error)
```

The SQL Engine calls this with the WHERE expression from `stmt.RawUpdate().Where.Expr` or `stmt.RawDelete().Where.Expr`. It maps `ErrUnsupportedFeature` → `ErrUnsupportedWhereExpr`.

### 7. WHERE-Required Enforcement

Before building the Volcano pipeline in `execUpdate` or `execDelete`, the engine must check:

```go
// For UPDATE:
if stmt.Where == nil { return nil, ErrWhereRequired }

// For DELETE:
if stmt.Where == nil { return nil, ErrWhereRequired }
```

---

## Tasks

1. **Extend `engine.Row`:** Change `Row` from `[]Datum` to struct with `Datums []Datum` and `RowID uint64`. Update all usages (`SeqScanNode`, `FilterNode`, `ProjectNode`, `operatorStream`, tests).
2. **Confirm `engine.TxContext.WriteTxID`:** Verify from DDL plan. Add if missing.
3. **Update `HeapScanIterator`:** Change `Next()` to return `(encodedTuple []byte, rowID uint64, err error)`. Concrete adapter emits `physicalOffset + 1` as `rowID`; never emits 0.
4. **Update `SeqScanNode.Next()`:** Set `row.RowID = rowID` from iterator directly (already `physicalOffset + 1`).
5. **Define `MutableTableHeap`:** Create `internal/engine/sql/mutable_heap.go` with `DeleteByRowID` and `UpdateByRowID` using `engine.TxContext`.
6. **Implement concrete adapter:** Bridge `engine.TxContext.WriteTxID` → `heap.Tx{ID: tx.WriteTxID}` and `rowID` → physical row key. When mutating, the concrete adapter must decode RowID back to the physical heap key by subtracting 1 before addressing the underlying heap (`physicalOffset := rowID - 1`). Adapter loads original row by `physicalOffset` internally for PK comparison in `UpdateByRowID`.
7. **Lock down constructors:** Add `NewSeqScanNode` and `NewFilterNode` with exact signatures to `internal/engine/sql/planner/plan.go`.
8. **Sentinel Errors:** Add `ErrWhereRequired`, `ErrHeapMutationUnsupported`, `ErrMissingRowID` to `internal/engine/sql/errors.go`.
9. **Tests:** Verify `SeqScanNode` emits non-zero `RowID`. Verify iterator never emits `rowID == 0`. Verify `DeleteByRowID`/`UpdateByRowID` bridge correctly. Verify `BindWhere` returns `ErrUnsupportedFeature` for unsupported expressions.

---

## Completion Criteria
- [ ] `engine.Row` is a struct with `Datums []Datum` and `RowID uint64`.
- [ ] `engine.TxContext` has both `ReadTxID` and `WriteTxID`.
- [ ] `HeapScanIterator.Next()` returns `(encodedTuple []byte, rowID uint64, err error)`; iterator never emits `rowID == 0`.
- [ ] `RowID` encoding is `physicalOffset + 1`; zero sentinel documented and enforced.
- [ ] `SeqScanNode.Next()` sets `row.RowID` from iterator.
- [ ] `MutableTableHeap` defined with `DeleteByRowID` / `UpdateByRowID` using `engine.TxContext`.
- [ ] Concrete adapter bridges `WriteTxID` → `heap.Tx` and loads original row for PK comparison internally.
- [ ] `NewSeqScanNode` and `NewFilterNode` constructors locked down.
- [ ] `ErrWhereRequired`, `ErrHeapMutationUnsupported`, `ErrMissingRowID` exist in `internal/engine/sql/errors.go`.
- [ ] All tests pass via the following commands:
```bash
go test ./internal/engine/... ./internal/engine/sql/planner/... ./internal/engine/sql/...
go test -race ./...
```
