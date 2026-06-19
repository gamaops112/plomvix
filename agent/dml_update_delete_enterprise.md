# DML Execution Enterprise (UPDATE and DELETE)

| Field | Value |
| :--- | :--- |
| **Source** | `agent/dml_update_delete_enterprise.md` |
| **Package(s)** | `internal/engine/sql`, `internal/engine/sql/planner`, `internal/router` |
| **Purpose** | Production-harden UPDATE and DELETE execution with multi-row mutation support, all-or-nothing conflict preflight, full-table DELETE, richer WHERE predicates, write-lock discipline, and structured telemetry. |
| **Dependencies** | DML Execution Setup (UPDATE and DELETE) plan, DML Row Identity Enterprise (Mutable Heap Hardening) plan, Planner / Volcano executor plan. |

---

## Honest Contracts & Known Trade-offs

1. **Multi-row mutation is now supported:** Enterprise removes the `ErrMultiRowMutationUnsupported` guard. All rows matching the WHERE predicate are collected by the Volcano pipeline, conflict-checked with all-or-nothing preflight, and submitted to BatchMutate. Conflict failures apply zero mutations; physical append failures may be partial until WAL rollback exists. `RowsAffected` in the result reflects the actual count of mutated rows.
2. **`maxMutationRows` guards runaway batches:** `SQLEngine` gains a configurable `maxMutationRows int` field (default 1000, set via `SQLEngineConfig.MaxMutationRows`). The guard fires **during row collection** — as each row is appended to `matched`, the count is checked immediately. If it exceeds `maxMutationRows`, the scan is aborted and `ErrMutationLimitExceeded` is returned before any mutation is applied. A value of `0` in `SQLEngineConfig` defaults to 1000, and `-1` disables the guard. If the Volcano pipeline yields more matching rows than this limit, the engine returns `ErrMutationLimitExceeded` before any mutation is applied. This is not a correctness guarantee — it is a safety rail for operators.
3. **Strict serializable conflict detection — conflict-check is all-or-nothing:** Before any mutation is applied, `CheckWriteConflict` is called for every matched row inside `BatchMutate`. If any row has a write conflict, `BatchMutate` returns `ErrWriteConflict` and applies zero mutations (the write lock has not yet been used for any append at this point). **Physical append atomicity is not guaranteed**: if `BatchMutate` fails mid-append due to an I/O error on row N, rows 1..N-1 may already be written as MVCC versions. Full transactional rollback of partial append failures is deferred to the WAL/rollback plan.
4. **Full-table DELETE requires explicit opt-in:** Enterprise allows `DELETE FROM t` with no WHERE clause. However, the engine returns `ErrDeleteAllRequiresConfirmation` by default unless `engine.Request.AllowFullTableDelete` is set to `true`. This prevents accidental data loss from unqualified deletes while still supporting the operation when the caller explicitly opts in.
5. **WHERE predicate extension — IN and BETWEEN:** `planner.BindWhere` is extended to handle `*vitess.ComparisonExpr` with `vitess.InOp` (IN lists of literals) and `*vitess.RangeCond` (BETWEEN literal AND literal). Sub-selects, correlated predicates, or non-literal IN/BETWEEN operands still return `ErrUnsupportedWhereExpr`.
6. **Expression SET values remain deferred:** `SET col = col + 1` and any non-literal SET values are still rejected with `ErrUnsupportedSetValue`. A future algebraic expression evaluator plan will lift this restriction. This deferral is intentional and documented here to prevent ad-hoc workarounds.
7. **Conflict-check is all-or-nothing; append failures may be partial:** `BatchMutate` (from Row Identity Enterprise plan) first conflict-checks all rows, then appends under one write lock. Conflict failures abort with zero mutations. I/O failures mid-append may leave partial MVCC versions; the engine logs `rows_succeeded` and `total_matched` at WARN before returning the heap error. Full rollback of partial append failures requires WAL support (deferred).
8. **`BatchMutate` owns the full-batch write lock:** Enterprise multi-row mutations use `MutableTableHeap.BatchMutate(ctx, tx, mutations []RowMutation) (int, error)` (defined in Row Identity Enterprise plan). `BatchMutate` acquires the table write lock once for the entire batch, preventing interleaved mutations from concurrent writers. The engine never directly acquires table locks. For large batches, this serializes all writers to the table for the batch duration — a known correctness-over-throughput trade-off.
9. **Telemetry via structured `slog` logger:** Every successful UPDATE or DELETE emits a structured log entry via `*slog.Logger` on the `SQLEngine`. The log includes: table name, rows affected, `WriteTxID`, and whether any write conflict was detected (and resolved to `ErrWriteConflict`). All failed mutations (any error returned from BatchMutate) are logged at WARN level with the error, including when rows_succeeded is 0.
10. **Router is unchanged:** No changes to `internal/router/router.go` are required. The router already resolves the target table and dispatches to `engines[tableInfo.EngineName]` for both UPDATE and DELETE. All enterprise logic is contained within the engine layer.

---

## Deliverables

| File | Purpose |
| :--- | :--- |
| `internal/engine/sql/errors.go` | Add `ErrMutationLimitExceeded`, `ErrDeleteAllRequiresConfirmation`; reference `ErrWriteConflict` from the Row Identity Enterprise plan. |
| `internal/engine/sql/engine.go` | Add `maxMutationRows int` to `SQLEngineConfig` (shared with INSERT Enterprise plan); `NewSQLEngine` defaults to 1000 if `MaxMutationRows == 0`; field name is `logger *slog.Logger`. |
| `internal/engine/request.go` | Add `AllowFullTableDelete bool` field to `engine.Request`. |
| `internal/engine/sql/dml_delete.go` | Replace single-row guard with multi-row batch loop; add full-table DELETE support (WHERE == nil path); conflict-check all rows before any mutation; emit telemetry. |
| `internal/engine/sql/dml_update.go` | Replace single-row guard with multi-row batch loop; conflict-check all rows before any mutation; emit telemetry. |
| `internal/engine/sql/planner/where.go` | Extend `BindWhere` to handle `*vitess.ComparisonExpr` with `vitess.InOp` and `*vitess.RangeCond` for literal operands. |
| `internal/engine/sql/dml_update_delete_enterprise_test.go` | Tests for multi-row mutations, `maxMutationRows` guard, conflict detection, full-table DELETE opt-in, IN/BETWEEN predicates, partial-failure logging, and telemetry assertions. |

---

## Key API & Concepts

### 1. New Sentinel Errors (`internal/engine/sql/errors.go`)
```go
var (
    // ErrMutationLimitExceeded is returned when the number of rows matching
    // the WHERE predicate exceeds SQLEngine.maxMutationRows. No mutations are applied.
    ErrMutationLimitExceeded = errors.New("engine: mutation row limit exceeded")

    // ErrDeleteAllRequiresConfirmation is returned when DELETE is issued without
    // a WHERE clause and engine.Request.AllowFullTableDelete is false.
    // Callers may catch this sentinel and retry with AllowFullTableDelete: true.
    ErrDeleteAllRequiresConfirmation = errors.New("engine: full-table DELETE requires AllowFullTableDelete opt-in")

    // ErrWriteConflict is defined in the DML Row Identity Enterprise plan
    // (MutableTableHeap.CheckWriteConflict). Referenced here for documentation.
    // var ErrWriteConflict = errors.New("engine: write conflict detected on row")
)
```

### 2. `engine.Request` Extension (`internal/engine/request.go`)
```go
// Request carries all inputs for a single SQL execution.
type Request struct {
    // ... existing fields (UserID, Stmt, TxContext) ...

    // AllowFullTableDelete, when true, permits DELETE FROM t with no WHERE clause.
    // If false (the default), an unqualified DELETE returns ErrDeleteAllRequiresConfirmation.
    // Callers must explicitly set this field to opt in to full-table deletion.
    AllowFullTableDelete bool
}
```

### 3. `SQLEngine` MaxMutationRows (`internal/engine/sql/engine.go`)

```go
// MaxMutationRows is set via SQLEngineConfig (defined in DML Execution Enterprise (INSERT) plan).
// NewSQLEngine sets maxMutationRows = cfg.MaxMutationRows; if 0, defaults to 1000; if -1, disables the guard.
//
// SQLEngine (relevant new/changed fields only):
type SQLEngine struct {
    // ... existing fields ...
    maxMutationRows int        // default 1000 if SQLEngineConfig.MaxMutationRows == 0; -1 if disabled
    logger          *slog.Logger  // field name is logger, not log
}
```

### 4. Extended `planner.BindWhere` — IN and BETWEEN (`internal/engine/sql/planner/where.go`)

The existing `BindWhere` switch is extended with two new cases. Both cases strictly require literal operands and return `ErrUnsupportedFeature` for anything else (which `execUpdate`/`execDelete` maps to `ErrUnsupportedWhereExpr`).

```go
// IN list: *vitess.ComparisonExpr with vitess.InOp
// AST produced by: WHERE col IN (1, 2, 3)
case *vitess.ComparisonExpr:
    if expr.Operator == vitess.InOp {
        // expr.Left is the column reference (vitess.ColName).
        // expr.Right is *vitess.ValTuple containing the list of literals.
        colName := expr.Left.(*vitess.ColName).Name.String()
        tuple, ok := expr.Right.(vitess.ValTuple)
        if !ok { return nil, ErrUnsupportedFeature }
        var literals []engine.Datum
        for _, val := range tuple {
            d, err := bindLiteral(val) // shared literal binding helper
            if err != nil { return nil, ErrUnsupportedFeature }
            literals = append(literals, d)
        }
        colIdx, err := schema.ColumnIndex(colName)
        if err != nil { return nil, err }
        return &InPredicate{ColIdx: colIdx, Values: literals}, nil
    }

// BETWEEN: *vitess.RangeCond
// AST produced by: WHERE col BETWEEN 10 AND 20
case *vitess.RangeCond:
    colName := expr.Left.(*vitess.ColName).Name.String()
    lo, err := bindLiteral(expr.From)
    if err != nil { return nil, ErrUnsupportedFeature }
    hi, err := bindLiteral(expr.To)
    if err != nil { return nil, ErrUnsupportedFeature }
    colIdx, err := schema.ColumnIndex(colName)
    if err != nil { return nil, err }
    return &BetweenPredicate{ColIdx: colIdx, Lo: lo, Hi: hi}, nil
```

### 5. Multi-row Collection and Conflict-Check Pattern (Shared for `execUpdate` and `execDelete`)

```go
// --- Phase 1: Collect all matching rows from Volcano pipeline ---

// For DELETE: if stmt.Where == nil, proceed only if req.AllowFullTableDelete == true.
// Otherwise return ErrDeleteAllRequiresConfirmation immediately.
// For UPDATE: WHERE == nil still returns ErrWhereRequired (unchanged from setup plan).

// Resolve catalog, TableHeap, schema as in setup plan.
// Construct SeqScanNode passing TxContext to it:
scanNode := planner.NewSeqScanNode(heapTarget, engSchema, e.decoder, req.TxContext)

// Build Volcano pipeline depending on WHERE:
var source planner.Operator = scanNode
if stmt.Where != nil {
    boundWhere, err := planner.BindWhere(stmt.Where.Expr, engSchema)
    if err != nil { return nil, err }
    source = planner.NewFilterNode(scanNode, boundWhere)
}

if err := source.Open(ctx); err != nil { return nil, err }
defer source.Close()

// Collect ALL matched rows.
// Check maxMutationRows DURING collection to abort early and avoid OOM.
var matched []engine.Row
for {
    row, err := source.Next(ctx)
    if err == io.EOF { break }
    if err != nil { return nil, err }
    matched = append(matched, row.DeepCopy())
    if e.maxMutationRows > 0 && len(matched) > e.maxMutationRows {
        return nil, ErrMutationLimitExceeded // no mutations applied
    }
}

// ErrRowNotFound is still returned if no rows match.
if len(matched) == 0 { return nil, ErrRowNotFound }

// --- Phase 2: Build Mutations and dispatch to BatchMutate ---
// Zero duplicate conflict checks on the engine. Engine only verifies RowID != 0.
mutations := make([]RowMutation, 0, len(matched))
for _, row := range matched {
    if row.RowID == 0 { return nil, ErrMissingRowID } // ErrMissingRowID is engine.ErrMissingRowID alias
    
    var op MutationOp = OpDelete
    var newValues []engine.Datum = nil
    
    if isUpdate { // for UPDATE statements
        newValues = deepCopyDatums(row.Datums)
        for colName, datum := range setByName {
            newValues[schemaIndexByName[colName]] = datum
        }
        op = OpUpdate
    }
    
    mutations = append(mutations, RowMutation{
        RowID:     row.RowID,
        Op:        op,
        NewValues: newValues,
    })
}

// Dispatch to BatchMutate: acquisitions table lock, runs conflict checks for all mutations,
// and applies them.
rowsAffected, err := mutHeap.BatchMutate(ctx, req.TxContext, mutations)
if err != nil {
    // Log mutation failure including rows succeeded prior to failure.
    e.logger.WarnContext(ctx, "dml: partial mutation failure",
        "table", tableName,
        "rows_succeeded", rowsAffected,
        "total_matched", len(matched),
        "write_tx_id", req.TxContext.WriteTxID,
        "error", err.Error(),
    )
    return nil, err
}
```

### 6. `execDelete` Enterprise Contract (`internal/engine/sql/dml_delete.go`)

```go
func (e *SQLEngine) execDelete(ctx context.Context, req *engine.Request, stmt *vitess.Delete) (*engine.Result, error) {
    // Phase 0: Full-table DELETE gate.
    if stmt.Where == nil {
        if !req.AllowFullTableDelete {
            return nil, ErrDeleteAllRequiresConfirmation
        }
        // AllowFullTableDelete == true: proceed with a full heap scan (no filter node).
        // catalog.ActionWrite already checked by router.
        //
        // Full-table DELETE source: no FilterNode, scan directly.
        //   var source planner.Operator = scanNode  (skip filterNode construction)
        // Qualified DELETE source:
        //   var source planner.Operator = planner.NewFilterNode(scanNode, boundWhere)
        //
        // In both cases, collect rows from source using the Phase 1 loop.
        // AllowFullTableDelete bypasses WHERE-required only — it does NOT bypass maxMutationRows.
        // If DELETE FROM t matches all rows and maxMutationRows > 0 and count > maxMutationRows,
        // ErrMutationLimitExceeded is returned with zero mutations.
        //
        // Caller contract for AllowFullTableDelete:
        //   The HTTP/API layer or internal admin client must set engine.Request.AllowFullTableDelete = true.
        //   Router passes the Request through unchanged. No implicit opt-in exists.
    }

    // Phase 1-3: Collect, build mutations, and dispatch to BatchMutate (see section 5).

    // Phase 4: Telemetry on success.
    e.logger.InfoContext(ctx, "dml: DELETE",
        "table", tableName,
        "rows_affected", rowsAffected,
        "write_tx_id", req.TxContext.WriteTxID,
        "conflict_checked", true,
    )

    return &engine.Result{
        Stream:       nil,
        RowsAffected: uint64(rowsAffected),
        Message:      fmt.Sprintf("DELETE %d", rowsAffected),
    }, nil
}
```

### 7. `execUpdate` Enterprise Contract (`internal/engine/sql/dml_update.go`)

```go
func (e *SQLEngine) execUpdate(ctx context.Context, req *engine.Request, stmt *vitess.Update) (*engine.Result, error) {
    // Step 1: Validate SET assignments (literals only, no duplicates) — unchanged from setup plan.
    //         Build setByName map[string]engine.Datum.
    // Step 2: WHERE == nil returns ErrWhereRequired (unchanged).
    // Step 3: Resolve schema and heap from catalog.
    // Step 4: Validate unknown SET columns against schema — unchanged from setup plan.
    // Step 5: Assert MutableTableHeap — unchanged from setup plan.
    // Step 6: Bind WHERE via extended planner.BindWhere (now supports IN and BETWEEN).

    // Phase 1-3: Multi-row collect, build mutations, and dispatch to BatchMutate (see section 5).
    // ErrPrimaryKeyUpdate propagated directly from heap if PK column appears in SET.

    // Phase 4: Telemetry on success.
    e.logger.InfoContext(ctx, "dml: UPDATE",
        "table", tableName,
        "rows_affected", rowsAffected,
        "write_tx_id", req.TxContext.WriteTxID,
        "conflict_checked", true,
    )

    return &engine.Result{
        Stream:       nil,
        RowsAffected: uint64(rowsAffected),
        Message:      fmt.Sprintf("UPDATE %d", rowsAffected),
    }, nil
}
```

---

## Tasks

1. **New Sentinel Errors:** Add `ErrMutationLimitExceeded` and `ErrDeleteAllRequiresConfirmation` to `internal/engine/sql/errors.go`. Add a comment referencing `ErrWriteConflict` from the Row Identity Enterprise plan (do not re-declare it).
2. **`engine.Request` Extension:** Add `AllowFullTableDelete bool` field to `engine.Request` in `internal/engine/request.go`.
3. **`SQLEngine` MaxMutationRows:** `maxMutationRows int` is a field in `SQLEngineConfig` (alongside `MaxBatchSize`, defined in DML Execution Enterprise (INSERT) plan). `NewSQLEngine` defaults to 1000 when `cfg.MaxMutationRows == 0`. A value of -1 disables the guard. No functional option (`WithMaxMutationRows`) is added; all engine config goes through `SQLEngineConfig`.
4. **Extend `planner.BindWhere`:** Add `*vitess.ComparisonExpr` (InOp) and `*vitess.RangeCond` cases to `BindWhere` in `internal/engine/sql/planner/where.go`. Add `InPredicate` and `BetweenPredicate` bound expression types and their `Evaluate` implementations. Require all operands to be literals; return `ErrUnsupportedFeature` otherwise.
5. **Refactor `execDelete` — Multi-row + Full-table:** Replace the single-row-only guard with the multi-row collection loop (section 5). Add the full-table DELETE gate at the top (WHERE == nil path). Build `[]RowMutation` with `OpDelete` and dispatch to `BatchMutate`. Log partial failures on `BatchMutate` error. Emit success telemetry.
6. **Refactor `execUpdate` — Multi-row:** Replace the single-row-only guard with the multi-row collection loop (section 5). Build `[]RowMutation` with `OpUpdate` and target values from SET map, and dispatch to `BatchMutate`. Log partial failures on `BatchMutate` error. Emit success telemetry.
7. **Multi-row UPDATE Tests:** Test UPDATE matching 3 rows: assert `RowsAffected == 3`, all rows updated in MVCC, no rows skipped. Test `maxMutationRows = 2` with 3 matching rows: assert `ErrMutationLimitExceeded`, zero mutations applied.
8. **Multi-row DELETE Tests:** Test DELETE matching 3 rows: assert `RowsAffected == 3`, all rows tombstoned. Test full-table `DELETE FROM t` (no WHERE) with `AllowFullTableDelete: false`: assert `ErrDeleteAllRequiresConfirmation`. Retry with `AllowFullTableDelete: true`: assert all rows deleted.
9. **Write Conflict Tests:** Inject a mock `MutableTableHeap` whose `BatchMutate` returns `ErrWriteConflict`. Assert: `BatchMutate` is called once, no direct `DeleteByRowID` or `UpdateByRowID` is called, and the returned error is `ErrWriteConflict`.
10. **IN and BETWEEN Predicate Tests:** Test `DELETE FROM t WHERE id IN (1, 2, 3)` on a table with rows id=1,2,3,4: assert `RowsAffected == 3`, id=4 untouched. Test `UPDATE t SET name='x' WHERE age BETWEEN 20 AND 30` on rows age=15,25,35: assert only age=25 row updated.
11. **Telemetry Tests:** Use a `*slog.Logger` backed by a `slog.NewJSONHandler` writing to a `bytes.Buffer`. After a successful UPDATE and DELETE, assert that the log output contains `"table"`, `"rows_affected"`, `"write_tx_id"`, and `"conflict_checked"` keys (snake_case).
12. **Partial Failure Logging Test:** Inject a mock heap whose `BatchMutate` returns `rowsAffected=1` and `io.ErrUnexpectedEOF`. Assert: error is returned, log contains `"rows_succeeded": 1` and `"total_matched": 3`.
13. **Regression Tests:** Re-run all existing setup plan tests to ensure no regressions: `ErrWhereRequired` (UPDATE only), `ErrRowNotFound`, `ErrPrimaryKeyUpdate`, `ErrDuplicateColumn`, `ErrUnsupportedSetValue`, `ErrUnknownColumn`, `ErrHeapMutationUnsupported`, `ErrMissingRowID`, `ErrUnsupportedWhereExpr` (non-literal function in WHERE).

---

## Completion Criteria

- [ ] `ErrMutationLimitExceeded` and `ErrDeleteAllRequiresConfirmation` declared in `internal/engine/sql/errors.go`.
- [ ] `engine.Request.AllowFullTableDelete bool` field added.
- [ ] `SQLEngine.maxMutationRows` defaults to 1000 when `SQLEngineConfig.MaxMutationRows == 0`; `-1` disables the guard; `logger *slog.Logger` field used throughout.
- [ ] `planner.BindWhere` handles `vitess.InOp` (`*vitess.ComparisonExpr`) and `*vitess.RangeCond` with literal operands; non-literals return `ErrUnsupportedFeature`.
- [ ] `InPredicate` and `BetweenPredicate` implement the `BoundExpr` interface with correct `Evaluate` semantics.
- [ ] `execDelete`: WHERE == nil + `AllowFullTableDelete: false` returns `ErrDeleteAllRequiresConfirmation`; WHERE == nil + `AllowFullTableDelete: true` performs full heap scan and tombstones all rows.
- [ ] `execDelete` and `execUpdate`: `maxMutationRows` guard fires before any mutation; `ErrMutationLimitExceeded` returned with zero mutations applied.
- [ ] Conflict checking is deferred to `BatchMutate`; engine does not perform manual `CheckWriteConflict` loops.
- [ ] `execDelete` and `execUpdate` construct `[]RowMutation` and call `BatchMutate` once for multi-row operations.
- [ ] Partial mid-batch heap failure logs `"rows_succeeded"` and `"total_matched"` at WARN level before returning the error.
- [ ] Successful UPDATE and DELETE emit `slog` INFO log with `"table"`, `"rows_affected"`, `"write_tx_id"`, `"conflict_checked"` keys (snake_case, matching INSERT enterprise telemetry).
- [ ] `RowsAffected` in `engine.Result` equals the actual count of rows mutated.
- [ ] `Message` field is `"UPDATE N"` / `"DELETE N"` where N == `RowsAffected`.
- [ ] All existing setup plan completion criteria still pass (no regressions).
- [ ] All tests pass via the following commands:
```bash
go test ./internal/sqlparser/... ./internal/router/... ./internal/engine/sql/... ./internal/engine/sql/planner/...
go test -race ./...
```
