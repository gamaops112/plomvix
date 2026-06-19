# DML Execution Enterprise (INSERT)

| Field | Value |
| :--- | :--- |
| **Source** | `agent/dml_enterprise.md` |
| **Package(s)** | `internal/engine/sql`, `internal/engine/sql/dml_insert.go`, `internal/engine/engine.go`, `internal/engine/sql/errors.go` |
| **Purpose** | Production-harden the DML Setup (INSERT) layer with Batch INSERT support, INSERT SELECT via Volcano pipeline, explicit DEFAULT value application, NOT NULL constraint enforcement, WriteTxID monotonic conflict detection, structured telemetry, strict constructor validation, and documented plan-cache bypass for DML. |
| **Dependencies** | DML Execution Setup (INSERT) plan, DDL Enterprise execution plan, Planner / Volcano executor plan. |

## Honest Contracts & Known Trade-offs

1. **Batch INSERT VALUES: validate-then-append.** All rows in `INSERT INTO t VALUES (...), (...)` are fully validated — column alignment, literal mapping, NOT NULL, DEFAULT resolution — before **any** row is appended to the heap. Validation failure returns immediately with zero heap writes. Physical append failures mid-batch (e.g., I/O error on row N) may leave partial MVCC versions; full transactional rollback of partial batch writes is deferred to the WAL/rollback plan. **INSERT SELECT is streaming and provides no batch atomicity**: if heap append fails mid-stream, previously appended rows remain as MVCC versions. The caller receives the heap error immediately.
2. **`maxBatchSize` is Engine-Level Configuration:** The maximum number of rows per batch INSERT is controlled by a `maxBatchSize int` field on `SQLEngine`, injected at construction time via `SQLEngineConfig`. If the parsed row count exceeds `maxBatchSize`, `execInsert` returns `ErrBatchTooLarge` before touching the heap. A value of `0` is invalid and is rejected by `NewSQLEngine`.
3. **INSERT SELECT Uses `planner.Translate()` for the SELECT Side:** The SELECT sub-tree is planned using the existing `planner.Translate()` (or `planner.Plan()`) call. The resulting `Operator` is opened and drained row-by-row into the heap. Type coercion is applied per-row using the target table schema, not the source schema. `INSERT SELECT` never touches the plan cache.
4. **DEFAULT Application Precedes NOT NULL Enforcement:** The column-resolution pipeline applies defaults before evaluating NOT NULL. A column that has a `DefaultValue` set and is omitted from the INSERT column list will receive the default value. NOT NULL is then evaluated on the post-default value. A column with `NotNull: true` and no default that is omitted or explicitly `NULL` returns `ErrNotNullViolation`.
5. **`DefaultValue *engine.Datum` is Optional:** `engine.Column.DefaultValue` is a pointer. `nil` means no default (behaviour identical to Setup plan: typed NULL). Non-nil means the pointed-to `Datum` is copied into the row for the column. The `Datum` stored in `DefaultValue` must have a `Type` matching `engine.Column.DataType`; a mismatch is caught during schema decode/validation, not at INSERT time.
6. **WriteTxID monotonic guard is owned by `InsertableTableHeap.InsertBatch`:** `execInsert` delegates batch write responsibility to `InsertableTableHeap.InsertBatch(ctx, tx, rows []engine.Row)`. `InsertBatch` acquires the table write lock, checks that `tx.WriteTxID` is strictly greater than the last written TxID for this table (returning `ErrTxConflict` with zero heap writes on violation), appends all rows under the lock, and on full success commits the new `lastWriteTxID`. The engine never calls `CheckAndSetWriteTxID` directly — lock, check, append, and commit are atomic within `InsertBatch`. This prevents the retry-poisoning problem: `lastWriteTxID` is only advanced after all rows are successfully appended.
7. **Telemetry is Fire-and-Forget:** A single `slog.Info` call is emitted after every successful insert (single-row, batch, and INSERT SELECT). The log record includes `table`, `rows_affected`, and `write_tx_id` fields. The logger is the same `*slog.Logger` injected into `SQLEngine` via the DDL Enterprise / Router-Planner Enterprise plan. Telemetry is never emitted on error paths.
8. **`NewSQLEngine` Validates All Dependencies:** `NewSQLEngine` is updated to validate that `catalog`, `tableRegistry`, `txMgr`, `planner`, `logger`, and `maxBatchSize` are non-nil / non-zero. It returns the sentinel errors `ErrNilCatalog`, `ErrNilTableRegistry`, `ErrNilTxManager`, `ErrNilPlanner`, `ErrNilLogger`, and `ErrInvalidBatchSize` respectively. These sentinel errors are added to `internal/engine/sql/errors.go`.
9. **Plan Cache Bypass for DML is Explicit and Documented:** DML statements (INSERT, and future UPDATE/DELETE) must never be stored in or looked up from the `PlanCache`. `SQLEngine.Execute()` enters the DML switch arm before any cache interaction. The DML code path does not call `cache.Lookup()` or `cache.Store()`. This invariant is enforced by code structure (DML branch is above the cache-first SELECT branch) and is documented in a code comment adjacent to the cache lookup call.
10. **INSERT SELECT Row Streaming, Not Buffering:** For `INSERT SELECT`, rows from the Volcano pipeline are streamed one-by-one into the heap using `BeginInsertStream` to ensure monotonic `WriteTxID` checking and lock hold for the stream duration. The entire result set is never materialised in memory. If the heap append fails mid-stream, previously appended rows for this WriteTxID will remain invisible until a future vacuum or are superseded; the caller receives the error from the first failing `stream.Append()` call. Full transactional rollback of partial batch writes is deferred to the MVCC Write-Ahead Log plan.

---

## Deliverables

| File | Purpose |
| :--- | :--- |
| `internal/engine/engine.go` | Add `DefaultValue *Datum` and `NotNull bool` fields to `engine.Column`. |
| `internal/engine/sql/errors.go` | Add `ErrBatchTooLarge`, `ErrNotNullViolation`, `ErrTxConflict`, `ErrNilCatalog`, `ErrNilTableRegistry`, `ErrNilTxManager`, `ErrNilPlanner`, `ErrNilLogger`, `ErrInvalidBatchSize`. |
| `internal/engine/sql/dml_insert.go` | Extend `execInsert` with batch support, DEFAULT resolution, NOT NULL enforcement, WriteTxID monotonic guard, telemetry, and INSERT SELECT streaming. |
| `internal/engine/sql/engine.go` | Update `NewSQLEngine` with strict dependency validation; add `maxBatchSize` config; document DML plan-cache bypass in code comments. |
| `internal/engine/sql/dml_insert_enterprise_test.go` | Tests for batch INSERT (success, `ErrBatchTooLarge`), DEFAULT application, `ErrNotNullViolation`, `ErrTxConflict`, INSERT SELECT streaming, telemetry log emission, and constructor sentinel errors. |
| `docs/dml_enterprise.md` | Architecture documentation for enterprise INSERT hardening. |

---

## Key API & Concepts

### 1. Extended `engine.Column` (`internal/engine/engine.go`)
```go
// Column describes a single column in a table schema.
type Column struct {
    Name         string
    DataType     DataType
    NotNull      bool         // If true, NULL values are rejected at insert time.
    DefaultValue *Datum       // If non-nil, applied when the column is omitted from the INSERT list.
}
```

### 2. New Sentinel Errors (`internal/engine/sql/errors.go`)
```go
var (
    // Batch INSERT errors
    ErrBatchTooLarge         = errors.New("engine: batch insert exceeds maxBatchSize")

    // Constraint errors
    ErrNotNullViolation      = errors.New("engine: NOT NULL constraint violation")
    ErrTxConflict            = errors.New("engine: WriteTxID monotonic conflict")

    // Constructor validation errors
    ErrNilCatalog            = errors.New("engine: catalog dependency is nil")
    ErrNilTableRegistry      = errors.New("engine: tableRegistry dependency is nil")
    ErrNilTxManager          = errors.New("engine: txManager dependency is nil")
    ErrNilPlanner            = errors.New("engine: planner dependency is nil")
    ErrNilLogger             = errors.New("engine: logger dependency is nil")
    ErrInvalidBatchSize      = errors.New("engine: maxBatchSize must be > 0")
)
```

### 3. `SQLEngineConfig` & Strict `NewSQLEngine` (`internal/engine/sql/engine.go`)
```go
// SQLEngineConfig holds all injectable dependencies for the SQL engine.
// MaxBatchSize must be >= 1 (ErrInvalidBatchSize returned if zero).
// MaxMutationRows uses 0 for default 1000 and -1 to disable the guard.
type SQLEngineConfig struct {
    Catalog         catalog.Catalog
    TableRegistry   TableRegistry
    TxManager       TxManager
    Planner         Planner
    Logger          *slog.Logger
    MaxBatchSize    int // Must be >= 1
    MaxMutationRows int // 0 = default 1000, -1 = disabled
}

// NewSQLEngine constructs a SQLEngine, validating all dependencies.
// Returns a sentinel error if any required field is nil or invalid.
func NewSQLEngine(cfg SQLEngineConfig) (*SQLEngine, error) {
    if cfg.Catalog == nil       { return nil, ErrNilCatalog }
    if cfg.TableRegistry == nil { return nil, ErrNilTableRegistry }
    if cfg.TxManager == nil     { return nil, ErrNilTxManager }
    if cfg.Planner == nil       { return nil, ErrNilPlanner }
    if cfg.Logger == nil        { return nil, ErrNilLogger }
    if cfg.MaxBatchSize < 1     { return nil, ErrInvalidBatchSize }
    
    maxMutationRows := cfg.MaxMutationRows
    if maxMutationRows == 0 {
        maxMutationRows = 1000
    }
    
    return &SQLEngine{
        catalog:         cfg.Catalog,
        tableRegistry:   cfg.TableRegistry,
        txMgr:           cfg.TxManager,
        planner:         cfg.Planner,
        logger:          cfg.Logger,
        maxBatchSize:    cfg.MaxBatchSize,
        maxMutationRows: maxMutationRows,
    }, nil
}
```

### 4. DML Plan Cache Bypass (`internal/engine/sql/engine.go`)
```go
func (e *SQLEngine) Execute(ctx context.Context, req *engine.Request) (*engine.Result, error) {
    // WriteTxID is allocated EXACTLY ONCE per DML/DDL execution.
    if req.Stmt.Type() == sqlparser.StmtDML || req.Stmt.Type() == sqlparser.StmtDDL {
        req.TxContext.WriteTxID = e.txMgr.AllocateWriteTxID()
    }

    // DML MUST be handled before the plan cache lookup below.
    // DML statements NEVER interact with the plan cache — no Lookup(), no Store().
    // This is enforced by code structure: the DML switch arm is above the cache-first SELECT path.
    switch req.Stmt.Type() {
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
    }

    // --- SELECT path: cache-first flow (DML never reaches here) ---
    // cache.Lookup() → miss → planner.Plan() → cache.Store() → Build() → Execute
    // ...
}
```

### 5. Batch INSERT — Validate-then-Append (`internal/engine/sql/dml_insert.go`)
```go
func (e *SQLEngine) execInsert(ctx context.Context, req *engine.Request, stmt *vitess.Insert) (*engine.Result, error) {
    // 1. Detect INSERT SELECT vs. VALUES
    values, ok := stmt.Rows.(vitess.Values)
    if !ok {
        return e.execInsertSelect(ctx, req, stmt) // See Section 6
    }

    // 2. Enforce maxBatchSize
    if len(values) > e.maxBatchSize {
        return nil, ErrBatchTooLarge
    }

    // 3. Resolve schema & heap (unchanged from Setup plan)
    tableName := stmt.Table.Name.String()
    tableInfo, err := e.catalog.GetTable(ctx, tableName)
    if err != nil { return nil, err }
    engSchema, err := sqlschema.Decode(tableInfo.SchemaPayload)
    if err != nil { return nil, err }
    heapTarget, err := e.tableRegistry.GetTableHeap(tableInfo.TableID)
    if err != nil { return nil, err }
    insertHeap, ok := heapTarget.(InsertableTableHeap)
    if !ok { return nil, ErrHeapInsertUnsupported }

    // 4. Validate ALL rows first — zero heap writes on any error
    mappedRows := make([]engine.Row, 0, len(values))
    for _, rowExprs := range values {
        row, err := e.buildRow(engSchema, stmt.Columns, rowExprs)
        if err != nil { return nil, err } // ErrNotNullViolation, ErrUnknownColumn, etc.
        mappedRows = append(mappedRows, row)
    }

    // 5. Delegate to InsertBatch: atomically lock → check WriteTxID monotonic guard →
    //    append all rows → commit lastWriteTxID → unlock.
    //    ErrTxConflict returned with zero heap writes if WriteTxID is non-monotonic.
    rowsAffected, err := insertHeap.InsertBatch(ctx, req.TxContext, mappedRows)
    if err != nil {
        return nil, err
    }

    e.logger.Info("insert",
        slog.String("table", tableName),
        slog.Int64("rows_affected", int64(rowsAffected)),
        slog.Uint64("write_tx_id", req.TxContext.WriteTxID),
    )
    return &engine.Result{RowsAffected: uint64(rowsAffected), Message: fmt.Sprintf("INSERT 0 %d", rowsAffected)}, nil
}
```

### 6. `buildRow` — DEFAULT & NOT NULL Resolution (`internal/engine/sql/dml_insert.go`)
```go
// buildRow maps a single Vitess Values row to an engine.Row, applying DEFAULTs and
// enforcing NOT NULL. Called for every row before any heap write.
func (e *SQLEngine) buildRow(
    schema *sqlschema.Schema,
    insertCols vitess.Columns,
    rowExprs vitess.ValTuple,
) (engine.Row, error) {
    // 1. Build schemaIndexByName; reject duplicates in insertCols.
    // 2. Initialise mappedRow: for each schema column apply DefaultValue if non-nil, else typed NULL.
    // 3. Map rowExprs to schema positions via insertCols (or positional if no column list).
    //    Use the exact Vitess AST literal switch from the Setup plan.
    // 4. After mapping, iterate schema columns:
    //    if col.NotNull && mappedRow[i].Value == nil { return nil, ErrNotNullViolation }
    return mappedRow, nil
}
```

### 7. INSERT SELECT — Volcano Streaming (`internal/engine/sql/dml_insert.go`)
```go
// execInsertSelect handles INSERT INTO t SELECT ... using the existing Volcano pipeline.
func (e *SQLEngine) execInsertSelect(
    ctx context.Context,
    req *engine.Request,
    stmt *vitess.Insert,
) (*engine.Result, error) {
    // 1. Extract the SELECT sub-statement from stmt.Rows (type *vitess.Select or *vitess.Union).
    // 2. Build a synthetic engine.Request for the SELECT side with req.TxContext.
    // 3. Call planner.Translate(ctx, e.catalog, selectReq) to obtain an Operator tree.
    //    NOTE: This call does NOT interact with the plan cache (DML bypass).
    // 4. Resolve target schema & heap as in execInsert.
    // 5. Apply WriteTxID monotonic guard.
    // 6. op.Open(ctx) → stream rows via op.Next(ctx) until io.EOF.
    //    For each row: coerce to target schema column types, enforce NOT NULL, stream.Append().
    // 7. op.Close(); emit telemetry; return Result.

    tableName := stmt.Table.Name.String()
    // ... (resolve tableInfo, schema, heap, WriteTxID guard as above)

    op, err := e.planner.Translate(ctx, e.catalog, selectReq)
    if err != nil { return nil, err }
    defer op.Close()

    if err := op.Open(ctx); err != nil { return nil, err }

    stream, err := insertHeap.BeginInsertStream(ctx, req.TxContext)
    if err != nil { return nil, err }
    
    committed := false
    defer func() {
        if !committed {
            _ = stream.Abort()
        }
    }()

    var rowsAffected uint64
    for {
        srcRow, err := op.Next(ctx)
        if errors.Is(err, io.EOF) { break }
        if err != nil { return nil, err }

        targetRow, err := coerceRow(engSchema, srcRow)
        if err != nil { return nil, err } // type mismatch or NOT NULL violation

        if err := stream.Append(ctx, targetRow); err != nil {
            return nil, err
        }
        rowsAffected++
    }

    if err := stream.Commit(); err != nil {
        return nil, err
    }
    committed = true

    e.logger.Info("insert",
        slog.String("table", tableName),
        slog.Uint64("rows_affected", rowsAffected),
        slog.Uint64("write_tx_id", req.TxContext.WriteTxID),
    )
    return &engine.Result{RowsAffected: rowsAffected, Message: fmt.Sprintf("INSERT 0 %d", rowsAffected)}, nil
}
```

### 8. WriteTxID Monotonic Guard (`internal/engine/sql/dml_insert.go` via `TableRegistry`)
```go
type TableRegistry interface {
    GetTableHeap(tableID uint64) (TableHeap, error)
    // No CheckAndSetWriteTxID here — WriteTxID check is owned by InsertableTableHeap.InsertBatch.
}

// InsertableTableHeap enterprise extension — replaces single-row Insert for batch paths.
type InsertableTableHeap interface {
    TableHeap // embeds read/scan contract

    // InsertBatch acquires the table write lock once, validates that tx.WriteTxID is
    // strictly greater than the last committed WriteTxID for this table (ErrTxConflict
    // on violation, zero heap writes), appends all rows, commits lastWriteTxID, releases lock.
    // Returns (rowsAffected, error).
    InsertBatch(ctx context.Context, tx engine.TxContext, rows []engine.Row) (int, error)

    // BeginInsertStream acquires the table write lock, validates tx.WriteTxID is monotonic
    // (ErrTxConflict on violation, releasing lock), and returns a stream writer.
    BeginInsertStream(ctx context.Context, tx engine.TxContext) (InsertStream, error)
}

type InsertStream interface {
    // Append appends a row under the held write lock.
    Append(ctx context.Context, row engine.Row) error
    // Commit commits lastWriteTxID and releases the table write lock.
    Commit() error
    // Abort releases the table write lock without committing lastWriteTxID.
    Abort() error
}
```

---

## Tasks

1. **Extend `engine.Column`:** Add `NotNull bool` and `DefaultValue *engine.Datum` fields to `engine.Column` in `internal/engine/engine.go`. Update `sqlschema.Decode` to populate these fields from the schema payload.
2. **Add Sentinel Errors:** Add `ErrBatchTooLarge`, `ErrNotNullViolation`, `ErrTxConflict`, `ErrNilCatalog`, `ErrNilTableRegistry`, `ErrNilTxManager`, `ErrNilPlanner`, `ErrNilLogger`, and `ErrInvalidBatchSize` to `internal/engine/sql/errors.go`.
3. **`SQLEngineConfig` & Strict Constructor:** Introduce `SQLEngineConfig` and update `NewSQLEngine` to accept it, validate all fields as non-nil/non-zero, and return the appropriate sentinel errors.
4. **Document DML Cache Bypass:** Add an explicit code comment in `SQLEngine.Execute()` before the cache-first SELECT block, documenting that DML never interacts with the plan cache and that the DML arm must remain above the cache lookup.
5. **`InsertBatch` on `InsertableTableHeap`:** Add `InsertBatch(ctx context.Context, tx engine.TxContext, rows []engine.Row) (int, error)` to `InsertableTableHeap` interface. Implement: acquire write lock → validate `tx.WriteTxID > lastWriteTxID` (else `ErrTxConflict`, no writes) → append all rows → on success commit `lastWriteTxID = tx.WriteTxID` → release lock.
6. **Extend `execInsert` for Batch:** Update `execInsert` to accept multi-row `vitess.Values`, enforce `maxBatchSize`, iterate `buildRow` over all rows to validate first, then call `insertHeap.InsertBatch(ctx, req.TxContext, mappedRows)` and use the returned `(rowsAffected, error)`. Emit telemetry on success.
7. **Implement `buildRow`:** Extract the single-row column alignment, DEFAULT application, literal mapping (exact Vitess AST switch from Setup plan), and NOT NULL enforcement into a standalone `buildRow` helper function. The Setup plan's single-row path becomes a `buildRow` call with `len(values) == 1`.
8. **Implement `execInsertSelect`:** Implement INSERT SELECT streaming using `BeginInsertStream` session API. Call `planner.Translate()` on the SELECT sub-tree, open the Operator, drain rows one-by-one, applying type coercion, and append to the stream. Call Commit() to persist writes or Abort() on error to release the table lock. Emit telemetry on success.
9. **Enterprise Tests:** Write `internal/engine/sql/dml_insert_enterprise_test.go` covering:
   - Batch INSERT success (multiple rows, all visible post-insert).
   - `ErrBatchTooLarge` when row count exceeds `maxBatchSize`.
   - DEFAULT value applied for omitted column.
   - `ErrNotNullViolation` when required column is omitted with no default.
   - `ErrTxConflict` on non-monotonic `WriteTxID`.
   - INSERT SELECT end-to-end: CREATE TABLE src, INSERT rows, INSERT INTO dst SELECT * FROM src, SELECT * FROM dst.
   - `slog` telemetry emission captured via a test logger handler.
   - `NewSQLEngine` sentinel errors for each nil/invalid dependency.

---

## Completion Criteria

- [ ] `engine.Column` has `NotNull bool` and `DefaultValue *engine.Datum` fields; `sqlschema.Decode` populates them.
- [ ] `ErrBatchTooLarge`, `ErrNotNullViolation`, `ErrTxConflict`, and all constructor sentinel errors are defined in `internal/engine/sql/errors.go`.
- [ ] `NewSQLEngine` rejects nil/zero dependencies with the correct sentinel errors.
- [ ] A code comment in `SQLEngine.Execute()` explicitly documents that DML bypasses the plan cache, and the DML arm appears before the cache-first SELECT block.
- [ ] Batch INSERT validates all rows before writing any; `ErrBatchTooLarge` is returned when `len(values) > maxBatchSize`.
- [ ] `buildRow` applies `DefaultValue` before evaluating `NotNull`; `ErrNotNullViolation` is returned for NULL values in NOT NULL columns.
- [ ] `InsertableTableHeap.InsertBatch` atomically lock → check WriteTxID → append all rows → commit lastWriteTxID → unlock; `ErrTxConflict` returned with zero heap writes on non-monotonic WriteTxID.
- [ ] `InsertableTableHeap` includes `BeginInsertStream` returning `InsertStream`; `InsertStream` has `Append`, `Commit`, and `Abort` methods.
- [ ] INSERT SELECT streams rows one-by-one from the Volcano pipeline; the result set is never fully materialised in memory.
- [ ] Telemetry is emitted via `slog.Info` with `table`, `rows_affected`, and `write_tx_id` fields on every successful insert.
- [ ] Single-row INSERT (from Setup plan) continues to pass all existing tests unchanged.
- [ ] All tests pass via the following commands:
```bash
go test ./internal/engine/... ./internal/engine/sql/...
go test -race ./...
```
