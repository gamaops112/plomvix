# Plan 27a: DDL Execution Setup (`CREATE TABLE` / `DROP TABLE`)

| Field | Value |
| :--- | :--- |
| **Source** | `agent/ddl_setup.md` |
| **Package(s)** | `internal/engine`, `internal/router`, `internal/engine/sql`, `internal/engine/sql/tx`, `internal/catalog`, `internal/sqlparser` |
| **Purpose** | Unlock the Write Path by allowing the database to create and drop tables. This bridges the SQL Parser, the Global Catalog, and the physical Storage layer. |
| **Dependencies** | Plan 26a/26b (Router/Planner), Plan 22-23 (Global Catalog), Plan 20-21 (Table Heap). |

## Honest Contracts & Known Trade-offs
1. **The API Shift (Breaking Change):** The `Engine.Execute()` and `Router.Route()` signatures change from returning `engine.RowStream` to returning `*engine.Result`. This is strictly necessary to support non-streaming DDL/DML responses.
2. **DDL Only (No DML):** This plan strictly unblocks `CREATE TABLE` and `DROP TABLE`. `INSERT`, `UPDATE`, and `DELETE` remain blocked with `ErrUnsupportedStatement` and are explicitly deferred to Plan 28.
3. **Safe CREATE Ordering:** To prevent inconsistent state, `CREATE TABLE` allocates the TableID, initializes the physical Heap on disk, and **only if the Heap succeeds**, it registers the metadata in the Catalog. If Heap initialization fails, the operation aborts with zero catalog pollution.
4. **Hard Delete on DROP:** Basic tier `DROP TABLE` performs a hard delete of the metadata from the Catalog system tables. Recreating a dropped table with the same name immediately succeeds. Physical file cleanup (orphaned heaps) is explicitly deferred to Plan 27b (Enterprise).
5. **Basic TxManager:** We introduce a basic `TxManager` to allocate monotonically increasing `uint64` timestamps. The SQL Engine uses this to assign `ReadTxID` for SELECTs and `WriteTxID` for DDL/DML.
6. **No Transactional DDL Rollback:** If a `CREATE TABLE` fails halfway through (e.g., disk full during Heap initialization), the system aborts. We do not implement complex rollback logic for partial physical file creation in Basic.
7. **Constraints Ignored in Basic:** The Basic DDL parser extracts column names and base types. It **explicitly ignores** constraints like `NOT NULL`, `DEFAULT`, and `PRIMARY KEY`. They are parsed from the AST but not enforced by the Heap or stored in the `SchemaPayload` yet.
8. **Empty Schema Rejected:** `CREATE TABLE` with zero columns (e.g., `CREATE TABLE empty ()`) is explicitly rejected with `ErrEmptySchema`.
9. **Lock Discipline (Golden Rule #4):** The Catalog mutex is held only briefly to insert the new table metadata and bump the SchemaVersion. Physical Heap initialization (Disk I/O) happens entirely outside the Catalog lock.

---

## Deliverables

| File | Purpose |
| :--- | :--- |
| `internal/engine/engine.go` | **API Shift:** Define `Result` struct. Update `Engine.Execute()` signature. Add `WriteTxID` to `TxContext`. |
| `internal/router/router.go` | **API Shift:** Update `Router.Route()` signature. Implement split DDL routing (CREATE vs DROP). |
| `internal/sqlparser/statement.go` | Add `RawDDL() *vitess.DDL` accessor to the Statement wrapper. |
| `internal/engine/sql/tx/tx.go` | Basic `TxManager` implementation (atomic uint64 counter). |
| `internal/catalog/catalog.go` | Add DDL methods: `AllocateTableID()`, `RegisterTable()`, `DropTable()`, `CheckGlobalPermission()`. |
| `internal/engine/sql/engine.go` | Implement DDL execution logic, AST type mapping (via `querypb`), wire `TxManager`, update constructor for strict validation. |
| `internal/engine/sql/heap_manager.go` | Extend `TableRegistry` with `CreateTableHeap()` to initialize physical storage. |
| `internal/engine/sql/ddl_test.go` | End-to-end tests for CREATE and DROP table execution, plus doc substring checks. |
| `docs/ddl.md` | Architecture documentation for the Write Path and DDL execution. |

---

## Key API & Concepts

### 1. The API Shift & TxContext (`internal/engine`)
```go
package engine

import "context"

// TxContext extended to support mutations.
type TxContext struct {
    ReadTxID  uint64 // Used for SELECT snapshot isolation
    WriteTxID uint64 // Used for DDL/DML mutation timestamps
}

// Result encapsulates the outcome of an engine execution.
type Result struct {
    Stream       RowStream // Non-nil for SELECT. nil for DDL/DML.
    RowsAffected int64     // Non-zero for DML. 0 for DDL/SELECT.
    Message      string    // Human-readable status for DDL.
}

type Engine interface {
    Name() string 
    Execute(ctx context.Context, req *Request) (*Result, error)
}