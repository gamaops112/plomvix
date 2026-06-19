# DML Execution Setup (UPDATE and DELETE)

| Field | Value |
| :--- | :--- |
| **Source** | `agent/dml_update_delete_setup.md` |
| **Package(s)** | `internal/router`, `internal/engine/sql`, `internal/sqlparser` |
| **Purpose** | Implement MVCC-correct `UPDATE` and `DELETE` execution using `RowID`-based row identity, the grounded `MutableTableHeap` interface, and the Volcano WHERE pipeline established in the DML Row Identity and Mutable Heap Contract plan. |
| **Dependencies** | DML Row Identity and Mutable Heap Contract plan, DML Execution Setup (INSERT) plan, Planner / Volcano executor plan. |

---

## Honest Contracts & Known Trade-offs

1. **Single-row WHERE only:** `UPDATE` and `DELETE` match rows through the Volcano pipeline. If more than one row matches the WHERE predicate, the engine returns `ErrMultiRowMutationUnsupported`. Multi-row mutations are deferred to the DML Enterprise plan.
2. **WHERE is required:** `UPDATE` or `DELETE` without a WHERE clause returns `ErrWhereRequired` immediately. Full-table mutations are deferred.
3. **Limited WHERE predicates:** WHERE must use operators supported by `planner.BindWhere` (`=`, `<`, `>`, `AND`, `OR` against literals). Functions or sub-selects return `ErrUnsupportedWhereExpr`. Basic tier expectation: equality on a uniquely identifying column.
4. **No PK metadata dependency:** Row targeting uses `engine.Row.RowID` (physical heap location), not logical PK columns. This bypasses the missing PK constraint entirely.
5. **Literal SET values only:** UPDATE `SET` assignments must use literals (`IntVal`, `FloatVal`, `StrVal`, `NullVal`). Expressions or function calls return `ErrUnsupportedSetValue`. Same literal switch as the DML Execution Setup (INSERT) plan.
6. **No PK column mutation:** `UpdateByRowID` in the concrete heap adapter returns `ErrPrimaryKeyUpdate` if any PK column value changes. This is propagated directly.
7. **No duplicate SET columns:** `UPDATE t SET name='a', name='b'` returns `ErrDuplicateColumn`.
8. **MVCC Append-Only:** The heap is never modified in place. DELETE appends a tombstone via `MutableTableHeap.DeleteByRowID`. UPDATE appends a new row version via `MutableTableHeap.UpdateByRowID`. Both use `req.TxContext.WriteTxID` (allocated exactly once in `SQLEngine.Execute`).
9. **Snapshot Isolation only:** Strict serializable conflict detection (detecting if another transaction modified the same row between read and write) is deferred to the DML Enterprise plan.

---

## Deliverables

| File | Purpose |
| :--- | :--- |
| `internal/sqlparser/wrapper.go` | Expose `RawUpdate() *vitess.Update` and `RawDelete() *vitess.Delete` value receiver methods. |
| `internal/router/router.go` | Extend `route()` to extract target table from UPDATE/DELETE AST, check `catalog.ActionWrite`, and dispatch. |
| `internal/engine/sql/engine.go` | Extend `Execute()` DML switch to route UPDATE/DELETE to `execUpdate` and `execDelete`. |
| `internal/engine/sql/dml_update.go` | `execUpdate()`: validate SET, bind WHERE, run Volcano pipeline, apply SET to matched row, call `MutableTableHeap.UpdateByRowID`. |
| `internal/engine/sql/dml_delete.go` | `execDelete()`: bind WHERE, run Volcano pipeline, call `MutableTableHeap.DeleteByRowID`. |
| `internal/engine/sql/errors.go` | Add `ErrMultiRowMutationUnsupported`, `ErrUnsupportedWhereExpr`, `ErrUnsupportedSetValue`, `ErrRowNotFound`, `ErrDuplicateColumn` (if not already from INSERT plan). |
| `internal/engine/sql/dml_update_delete_test.go` | Tests for RowsAffected, MVCC visibility, WHERE enforcement, SET validation, and constraint propagation. |

---

## Key API & Concepts

### 1. New Sentinel Errors (`internal/engine/sql/errors.go`)
```go
var (
    ErrMultiRowMutationUnsupported = errors.New("engine: multi-row mutation not supported in basic tier")
    ErrUnsupportedWhereExpr        = errors.New("engine: unsupported WHERE expression")
    ErrUnsupportedSetValue         = errors.New("engine: unsupported SET value expression")
    ErrRowNotFound                 = errors.New("engine: no row found matching predicate")
)
// Note: ErrDuplicateColumn and ErrUnknownColumn already exist from the INSERT plan.
// Note: ErrWhereRequired and ErrHeapMutationUnsupported come from the Row Identity plan.
```

### 2. Parser Accessors (`internal/sqlparser/wrapper.go`)
```go
import vitess "vitess.io/vitess/go/vt/sqlparser"

// Value receivers matching the existing RawDDL / RawInsert style.
func (s Statement) RawUpdate() *vitess.Update { /* ... */ }
func (s Statement) RawDelete() *vitess.Delete { /* ... */ }
```

### 3. Router Target Resolution (`internal/router/router.go`)
Replace the deferred `ErrUnsupportedStatement` block in the DML case. Extract table name safely with length checks:

```go
switch {
case req.Stmt.RawInsert() != nil:
    tableName = req.Stmt.RawInsert().Table.Name.String()

case req.Stmt.RawUpdate() != nil:
    upd := req.Stmt.RawUpdate()
    if len(upd.TableExprs) != 1 { return nil, ErrUnsupportedStatement }
    aliased, ok := upd.TableExprs[0].(*vitess.AliasedTableExpr)
    if !ok { return nil, ErrUnsupportedStatement }
    tname, ok := aliased.Expr.(vitess.TableName)
    if !ok { return nil, ErrUnsupportedStatement }
    tableName = tname.Name.String()

case req.Stmt.RawDelete() != nil:
    del := req.Stmt.RawDelete()
    if len(del.TableExprs) != 1 { return nil, ErrUnsupportedStatement }
    aliased, ok := del.TableExprs[0].(*vitess.AliasedTableExpr)
    if !ok { return nil, ErrUnsupportedStatement }
    tname, ok := aliased.Expr.(vitess.TableName)
    if !ok { return nil, ErrUnsupportedStatement }
    tableName = tname.Name.String()

default:
    return nil, ErrUnsupportedStatement
}

// Resolve table metadata and enforce write permission.
tableInfo, err := r.catalog.GetTable(ctx, tableName)
if err != nil { return nil, err }
if err := r.catalog.CheckPermission(ctx, req.UserID, tableInfo.TableID, catalog.ActionWrite); err != nil {
    return nil, ErrPermissionDenied
}
targetEngine, ok := r.engines[tableInfo.EngineName]
if !ok { return nil, ErrEngineNotFound }
return targetEngine.Execute(ctx, req)
```

> **Note:** Both UPDATE and DELETE use `TableExprs` (not `.TableName`) for consistent Vitess AST handling. Always check `len == 1` before indexing to prevent panics on multi-table or malformed statements.

### 4. Engine DML Switch Extension (`internal/engine/sql/engine.go`)
The `WriteTxID` allocation is already in place from the INSERT plan. Extend:

```go
case sqlparser.StmtDML:
    if insert := req.Stmt.RawInsert(); insert != nil {
        return e.execInsert(ctx, req, insert)
    }
    if update := req.Stmt.RawUpdate(); update != nil {
        return e.execUpdate(ctx, req, update)
    }
    if del := req.Stmt.RawDelete(); del != nil {
        return e.execDelete(ctx, req, del)
    }
    return nil, ErrUnsupportedDML
```

### 5. Volcano Row Location (Shared Pattern for both execUpdate and execDelete)

```go
// 1. Enforce WHERE required.
if stmt.Where == nil { return nil, ErrWhereRequired }

// 2. Extract table name from AST using the same single-table guard as the router.
//    Call a shared private helper:
//      tableName, err := extractSingleDMLTableName(stmt)
//      if err != nil { return nil, ErrUnsupportedStatement }
//    extractSingleDMLTableName accepts *vitess.Update or *vitess.Delete,
//    checks len(TableExprs) == 1, type-asserts *vitess.AliasedTableExpr and vitess.TableName,
//    and returns the table name string. Returns ErrUnsupportedStatement on any failure.

// 3. Resolve schema and heap.
tableInfo, err := e.catalog.GetTable(ctx, tableName)
if err != nil { return nil, err }
engSchema, err := sqlschema.Decode(tableInfo.SchemaPayload)
if err != nil { return nil, err }
heapTarget, err := e.tableRegistry.GetTableHeap(tableInfo.TableID)
if err != nil { return nil, err }

// (UPDATE only: step 4 — validate unknown SET columns here, before scan. See execUpdate.)

// 5. Assert heap supports mutation BEFORE any scan work.
mutHeap, ok := heapTarget.(MutableTableHeap)
if !ok { return nil, ErrHeapMutationUnsupported }

// 6. Bind WHERE. Map only ErrUnsupportedFeature; propagate other errors as-is.
boundWhere, err := planner.BindWhere(stmt.Where.Expr, engSchema)
if err != nil {
    if errors.Is(err, planner.ErrUnsupportedFeature) { return nil, ErrUnsupportedWhereExpr }
    return nil, err
}

// 7. Build Volcano pipeline.
scanNode   := planner.NewSeqScanNode(heapTarget, engSchema, e.decoder)
filterNode := planner.NewFilterNode(scanNode, boundWhere)

// 8. Collect matching rows.
if err := filterNode.Open(ctx); err != nil { return nil, err }
defer filterNode.Close()

var matched []engine.Row
for {
    row, err := filterNode.Next(ctx)
    if err == io.EOF { break }
    if err != nil { return nil, err }
    matched = append(matched, row.DeepCopy())
}

// 9. Single-row-only contract.
if len(matched) == 0 { return nil, ErrRowNotFound }
if len(matched) > 1  { return nil, ErrMultiRowMutationUnsupported }

target := matched[0]

// 10. Guard: RowID == 0 means row was not from a heap scan.
if target.RowID == 0 { return nil, ErrMissingRowID }

// mutHeap already asserted in step 5; use it directly for mutation.
```

### 6. `execDelete` Contract (`internal/engine/sql/dml_delete.go`)

```go
func (e *SQLEngine) execDelete(ctx context.Context, req *engine.Request, stmt *vitess.Delete) (*engine.Result, error) {
    // Steps 1-8: Volcano row location (see section 5). mutHeap already asserted.

    // 9. Append tombstone using RowID.
    if err := mutHeap.DeleteByRowID(ctx, req.TxContext, target.RowID); err != nil {
        return nil, err
    }

    return &engine.Result{
        Stream:       nil,
        RowsAffected: 1,
        Message:      "DELETE 1",
    }, nil
}
```

### 7. `execUpdate` Contract (`internal/engine/sql/dml_update.go`)

```go
func (e *SQLEngine) execUpdate(ctx context.Context, req *engine.Request, stmt *vitess.Update) (*engine.Result, error) {
    // 1. Validate SET assignments BEFORE any I/O.
    //    Build setByName map[string]engine.Datum.
    //    For each stmt.Exprs[i] (*vitess.UpdateExpr):
    //      - colName := stmt.Exprs[i].Name.Name.String()
    //      - Reject duplicate colName with ErrDuplicateColumn.
    //      - stmt.Exprs[i].Expr must be *vitess.Literal or *vitess.NullVal.
    //        Use same literal switch as INSERT plan. Reject else with ErrUnsupportedSetValue.

    // 2. Enforce WHERE required.
    // 3. Resolve schema and heap from catalog.
    // 4. Validate unknown SET columns using schema BEFORE scan:
    //    Build schemaIndexByName from engSchema.Columns.
    //    Reject any colName in setByName not in schemaIndexByName with ErrUnknownColumn.
    // 5. Assert MutableTableHeap (before any scan work).
    // 6. Bind WHERE via planner.BindWhere.
    // 7. Build and run Volcano pipeline (steps 7-10 of shared pattern in section 5).
    // 8. Enforce single-row-only (step 9 of shared pattern).
    // 9. Guard RowID != 0 (step 10 of shared pattern).

    // 10. Build newValues []engine.Datum from target.Datums (deep copy).
    //     Apply SET assignments using schemaIndexByName (already built in step 4).

    // 11. Append new row version.
    if err := mutHeap.UpdateByRowID(ctx, req.TxContext, target.RowID, newValues); err != nil {
        return nil, err // ErrPrimaryKeyUpdate propagated here if PK column changed.
    }

    return &engine.Result{
        Stream:       nil,
        RowsAffected: 1,
        Message:      "UPDATE 1",
    }, nil
}
```

---

## Tasks

1. **Sentinel Errors:** Add `ErrMultiRowMutationUnsupported`, `ErrUnsupportedWhereExpr`, `ErrUnsupportedSetValue`, and `ErrRowNotFound` to `internal/engine/sql/errors.go`.
2. **Parser Accessors:** Implement `RawUpdate() *vitess.Update` and `RawDelete() *vitess.Delete` (value receivers) in `internal/sqlparser/wrapper.go`.
3. **Update Router:** Replace the deferred `ErrUnsupportedStatement` block with the safe table-name extraction for UPDATE and DELETE (with `len(TableExprs) == 1` guard), routing both through `engines[tableInfo.EngineName]`.
4. **Engine DML Switch:** Extend the `StmtDML` case in `SQLEngine.Execute` to route to `execUpdate` and `execDelete`.
5. **Implement `execDelete`:** Build `internal/engine/sql/dml_delete.go` per the contract in section 6.
6. **Implement `execUpdate`:** Build `internal/engine/sql/dml_update.go` per the contract in section 7. Validate SET literals first (before any I/O), build `setByName`, then run Volcano, then apply SET, then call `UpdateByRowID`.
7. **RowsAffected Tests:** Assert `RowsAffected == 1` and `Stream == nil` on success.
8. **Constraint Tests:** Test: `ErrWhereRequired` (no WHERE), `ErrRowNotFound` (no match), `ErrMultiRowMutationUnsupported` (multiple matches), `ErrPrimaryKeyUpdate` (PK in SET), `ErrDuplicateColumn` (repeated SET col), `ErrUnsupportedSetValue` (expression in SET), `ErrUnsupportedWhereExpr` (function in WHERE), `ErrUnknownColumn` (unknown SET col), `ErrHeapMutationUnsupported` (non-mutable heap), `ErrMissingRowID` (zero RowID on target row), multi-table UPDATE/DELETE rejected by router, router enforces `catalog.ActionWrite`.
9. **Parser Tests:** Verify `RawUpdate()` and `RawDelete()` return correct AST nodes; nil for wrong statement types.
10. **MVCC Visibility Tests:** Provide test helper constructing `engine.Request` with explicit `TxContext.ReadTxID`. Verify:
   - After `DELETE FROM t WHERE ...` with `WriteTxID=10`: `SELECT` with `ReadTxID=11` returns no row; `SELECT` with `ReadTxID=9` still sees original.
   - After `UPDATE t SET ...` with `WriteTxID=10`: `SELECT` with `ReadTxID=11` returns updated values; `ReadTxID=9` returns original.

---

## Completion Criteria
- [ ] Router resolves target table with `len(TableExprs) == 1` guard, checks `catalog.ActionWrite`, and dispatches via `engines[tableInfo.EngineName]`.
- [ ] `RawUpdate()` and `RawDelete()` are value receivers in `internal/sqlparser/wrapper.go`.
- [ ] `MutableTableHeap` asserted before Volcano pipeline is built.
- [ ] `execDelete` enforces `ErrWhereRequired`, guards `RowID != 0`, and calls `DeleteByRowID`.
- [ ] `execUpdate` validates SET literals + duplicates before any I/O, validates unknown columns after schema load, enforces `ErrWhereRequired`, guards `RowID != 0`, and calls `UpdateByRowID`.
- [ ] `BindWhere` error: only `ErrUnsupportedFeature` maps to `ErrUnsupportedWhereExpr`; others propagated.
- [ ] Duplicate SET columns return `ErrDuplicateColumn`.
- [ ] Multi-row matches return `ErrMultiRowMutationUnsupported`.
- [ ] Missing row returns `ErrRowNotFound`.
- [ ] PK column mutation returns `ErrPrimaryKeyUpdate` from heap.
- [ ] Zero RowID returns `ErrMissingRowID`.
- [ ] Successful UPDATE and DELETE return `RowsAffected: 1` and `Stream: nil`.
- [ ] MVCC visibility tests pass with explicit `ReadTxID` control.
- [ ] All tests pass via the following commands:
```bash
go test ./internal/sqlparser/... ./internal/router/... ./internal/engine/sql/...
go test -race ./...
```
