# DML Execution Setup (INSERT)

| Field | Value |
| :--- | :--- |
| **Source** | `agent/dml_setup.md` |
| **Package(s)** | `internal/router`, `internal/engine/sql`, `internal/sqlparser` |
| **Purpose** | Unblock Router DML routing with permissions, and implement strict, single-row `INSERT` execution. `UPDATE` and `DELETE` are explicitly deferred to the subsequent UPDATE/DELETE execution plan (Plan 28b). |
| **Dependencies** | DDL Enterprise execution plan, Planner / Volcano executor plan. |

## Honest Contracts & Known Trade-offs

1. **INSERT Only for Setup:** To ensure absolute correctness with MVCC Row Identity, `UPDATE` and `DELETE` are **deferred to the subsequent UPDATE/DELETE execution plan (Plan 28b)**. This allows the current plan (Plan 28a) to focus exclusively on securing the DML Router path, the `catalog.ActionWrite` permission model, and single-row `INSERT` encoding.
2. **Strict INSERT Syntax:** 
   - Supports: `INSERT INTO t VALUES (...)` and `INSERT INTO t (a,b) VALUES (...)`.
   - Rejects: Batch inserts (`INSERT INTO t VALUES (...), (...)`) with `ErrBatchInsertUnsupported`.
   - Rejects: Subqueries (`INSERT INTO t SELECT ...`) with `ErrInsertSelectUnsupported`.
   - Rejects: Column count mismatches with `ErrColumnCountMismatch`.
3. **WriteTxID Allocation:** `WriteTxID` is allocated exactly **once** in the SQL Engine's `Execute()` wrapper. The downstream `execInsert` method uses `req.TxContext.WriteTxID` without re-allocating.
4. **Plan Cache Visibility:** DML does *not* bump the global schema version, so it does not invalidate the SELECT plan cache. However, visibility of the newly inserted rows relies entirely on the `ReadTxID` / `WriteTxID` comparisons during the Heap Scan.
5. **No Constraints / Defaults:** Explicit defaults are deferred. Unspecified columns get a `NULL` value.
6. **Literal Value Mapping Only:** `INSERT` values must be simple literals (int, string, float, null). Expressions or functions return `ErrUnsupportedInsertValue`.

---

## Deliverables

| File | Purpose |
| :--- | :--- |
| `internal/router/router.go` | Update `route()` to resolve tables, check `catalog.ActionWrite`, and dispatch. |
| `internal/engine/sql/dml_insert.go` | `execInsert()` resolving schema from payload, mapping literals via exact Vitess AST checks, and appending via `InsertableTableHeap`. |
| `internal/sqlparser/wrapper.go` | Expose value receiver method `RawInsert() *vitess.Insert`. |
| `internal/engine/sql/errors.go` | Add sentinel DML errors including `ErrUnsupportedInsertValue`, `ErrHeapInsertUnsupported`, and `ErrDuplicateColumn`. |
| `internal/engine/sql/dml_insert_test.go` | Strict `RowsAffected` checks, AST constraint testing, and literal mapping checks. |

---

## Key API & Concepts

### 1. DML Sentinel Errors (`internal/engine/sql/errors.go`)
```go
var (
    ErrUnsupportedDML                = errors.New("engine: unsupported DML statement type")
    ErrBatchInsertUnsupported        = errors.New("engine: batch insert not supported")
    ErrInsertSelectUnsupported       = errors.New("engine: insert select not supported")
    ErrColumnCountMismatch           = errors.New("engine: column count mismatch in insert")
    ErrUnknownColumn                 = errors.New("engine: unknown column")
    ErrDuplicateColumn               = errors.New("engine: duplicate column in insert list")
    ErrTypeMismatch                  = errors.New("engine: type mismatch")
    ErrUnsupportedInsertValue        = errors.New("engine: unsupported insert value expression")
    ErrHeapInsertUnsupported         = errors.New("engine: target table heap does not support insertions")
)
```

### 2. Parser Accessors (`internal/sqlparser/wrapper.go`)
```go
import vitess "vitess.io/vitess/go/vt/sqlparser"

// Value receivers to match previous RawDDL style.
func (s Statement) RawInsert() *vitess.Insert { /* ... */ }
```

### 3. Router Target Resolution & Permission (`internal/router/router.go`)
The router resolves the target from the AST, checks permissions, and dispatches.

```go
func (r *Router) route(ctx context.Context, req *engine.Request) (*engine.Result, error) {
    if req.Stmt.Type() == sqlparser.StmtDML {
        var tableName string
        if insert := req.Stmt.RawInsert(); insert != nil {
            tableName = insert.Table.Name.String()
        } else {
            // Update/Delete are explicitly deferred.
            return nil, ErrUnsupportedStatement 
        }
        
        // Resolve TableInfo
        tableInfo, err := r.catalog.GetTable(ctx, tableName)
        if err != nil { return nil, err }
        
        // Verify Write Permissions
        if err := r.catalog.CheckPermission(ctx, req.UserID, tableInfo.TableID, catalog.ActionWrite); err != nil {
            return nil, err
        }
        
        // Dispatch to the owning engine using router-level error
        targetEngine, ok := r.engines[tableInfo.EngineName]
        if !ok { return nil, ErrEngineNotFound }
        
        return targetEngine.Execute(ctx, req)
    }
    // ...
}
```

### 4. SQL Engine Execution Switch (`internal/engine/sql/engine.go`)
```go
func (e *SQLEngine) Execute(ctx context.Context, req *engine.Request) (*engine.Result, error) {
    // WriteTxID is allocated EXACTLY ONCE per execution by the engine.
    if req.Stmt.Type() == sqlparser.StmtDML || req.Stmt.Type() == sqlparser.StmtDDL {
        req.TxContext.WriteTxID = e.txMgr.AllocateWriteTxID()
    }
    
    switch req.Stmt.Type() {
    case sqlparser.StmtDML:
        if insert := req.Stmt.RawInsert(); insert != nil {
            return e.execInsert(ctx, req, insert)
        }
        return nil, ErrUnsupportedDML // Update/Delete deferred
    }
}
```

### 5. `INSERT` Execution & Heap Contract (`internal/engine/sql/dml_insert.go`)
```go
// Define the local contract required by the engine for appending rows.
type InsertableTableHeap interface {
    Insert(ctx context.Context, tx engine.TxContext, row engine.Row) error
}

func (e *SQLEngine) execInsert(ctx context.Context, req *engine.Request, stmt *vitess.Insert) (*engine.Result, error) {
    // 1. Vitess AST Extraction & Validation
    values, ok := stmt.Rows.(vitess.Values)
    if !ok { return nil, ErrInsertSelectUnsupported }
    if len(values) != 1 { return nil, ErrBatchInsertUnsupported }
    rowExprs := values[0]
    
    // 2. Resolve Schema & Heap
    tableName := stmt.Table.Name.String()
    tableInfo, err := e.catalog.GetTable(ctx, tableName)
    if err != nil { return nil, err }
    
    engSchema, err := sqlschema.Decode(tableInfo.SchemaPayload)
    if err != nil { return nil, err }
    
    heapTarget, err := e.tableRegistry.GetTableHeap(tableInfo.TableID)
    if err != nil { return nil, err }
    
    insertHeap, ok := heapTarget.(InsertableTableHeap)
    if !ok { return nil, ErrHeapInsertUnsupported }
    
    // 3. Column Alignment & Validation
    // - Build schemaIndexByName from engSchema.Columns.
    // - Initialize mappedRow with typed NULL for every schema column.
    // - If no insert column list: map rowExprs by schema order.
    // - If column list exists: for each stmt.Columns[i], resolve schema index and place rowExprs[i] there.
    // - Reject duplicate names or unknown columns before mapping.
    
    // 4. Type Checking & Literal Mapping
    // Map Vitess literal AST nodes to engine.Datum based on target column type.
    // - NULL mappings use the target column type with Value: nil (e.g. {Type: TypeString, Value: nil})
    // Use the following strict AST handling block for literal value parsing:
    // switch v := expr.(type) {
    // case *vitess.Literal:
    //     switch v.Type {
    //     case vitess.IntVal: // maps to TypeInt64 / TypeUint64
    //     case vitess.FloatVal: // maps to TypeFloat64
    //     case vitess.StrVal: // maps to TypeString
    //     case vitess.HexNum, vitess.HexVal:
    //         return ErrUnsupportedInsertValue
    //     default:
    //         return ErrUnsupportedInsertValue
    //     }
    // case *vitess.NullVal:
    //     // typed nil using target column type
    // default:
    //     return ErrUnsupportedInsertValue
    // }
    
    // 5. Encode & Append
    // err = insertHeap.Insert(ctx, req.TxContext, mappedRow)
    
    return &engine.Result{
        Stream:       nil,
        RowsAffected: 1,
        Message:      "INSERT 0 1",
    }, nil
}
```

---

## Tasks

1. **Sentinel Errors:** Add the required DML error variables to `internal/engine/sql/errors.go`, including `ErrUnsupportedInsertValue`, `ErrHeapInsertUnsupported`, and `ErrDuplicateColumn`.
2. **Parser Accessors:** Implement `RawInsert() *vitess.Insert` (value receiver) in `internal/sqlparser/wrapper.go`. Do NOT implement update/delete.
3. **Update Router:** Implement table extraction via `RawInsert`, permission checks, and dynamic engine dispatch returning `ErrEngineNotFound` in `internal/router/router.go`.
4. **Engine DML Switch & TxID:** Update `SQLEngine.Execute` to allocate `WriteTxID` exactly once and route to `execInsert(ctx, req, insert)`.
5. **Implement `execInsert`:** Build `internal/engine/sql/dml_insert.go`. Decode schema via `sqlschema.Decode(tableInfo.SchemaPayload)`, fetch physical target from `TableRegistry` (asserting `InsertableTableHeap`), enforce exact Vitess AST shape (`stmt.Rows.(vitess.Values)`), map literal values, and invoke `insertHeap.Insert`. Enforce the deterministic column alignment mapping rule (building schemaIndexByName, initializing mappedRow with typed NULLs, mapping rowExprs to their resolved index based on presence of insert column list, and rejecting duplicate or unknown columns before mapping).
6. **RowsAffected Tests:** Write tests guaranteeing `RowsAffected == 1` and `Stream == nil` on success.
7. **Constraint Tests:** Write tests validating rejected AST forms, literal mapping failures, duplicate columns, and type mismatches.
8. **End-to-End Visibility Test:** Write an integration test executing `CREATE TABLE t (...)`, `INSERT INTO t VALUES (...)`, and `SELECT * FROM t`, asserting the inserted row is correctly returned and visible in the result stream.

## Completion Criteria
- [ ] Router strictly checks `catalog.ActionWrite` before routing DML.
- [ ] Router dynamically dispatches DML to `engines[engineName]` instead of a hardcoded field.
- [ ] `SQLEngine.Execute` assigns `WriteTxID` exactly once.
- [ ] `execInsert` correctly fetches the physical target from `TableRegistry` and requires `InsertableTableHeap`.
- [ ] Batch inserts, `INSERT SELECT`, and column count/duplicate mismatches return the correct sentinel errors.
- [ ] Omitted or explicitly `NULL` values map to the exact target column type with a nil `Value`.
- [ ] Successful insert returns `RowsAffected: 1` and `Stream: nil`.
- [ ] UPDATE and DELETE are rejected safely before execution (e.g., in router).
- [ ] An integration test verifies end-to-end visibility: a row inserted after table creation is successfully queried and verified via `SELECT * FROM t`.
- [ ] All tests pass via the following commands:
```bash
go test ./internal/sqlparser/... ./internal/router/... ./internal/engine/sql/...
go test -race ./...
```
