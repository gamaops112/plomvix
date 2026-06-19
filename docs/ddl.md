# DDL Execution — Write Path Architecture

| Field | Value |
| :--- | :--- |
| **Package** | `internal/engine`, `internal/router`, `internal/engine/sql`, `internal/engine/sql/tx`, `internal/catalog`, `internal/sqlparser` |
| **Plan** | `agent/ddl_setup.md` (Plan 27a) |

## Overview

This document describes the Write Path — the execution pipeline for `CREATE TABLE`
and `DROP TABLE` statements. It bridges the SQL Parser, Global Catalog, Router,
and physical Storage layer.

## Architecture

```
SQL Text
  │
  ▼
sqlparser.Parse("CREATE TABLE t (id bigint)")
  │  Statement.Type() = StmtDDL
  ▼
Router.Route(ctx, userID, stmt)
  ├─ StmtSelect → routeSelect() → permission check → engine dispatch
  └─ StmtDDL    → routeDDL()    → default engine dispatch
                                    │
                                    ▼
                          SQLEngine.Execute()
                            │
                    stmt.Type() switch:
                    ├─ StmtSelect → executeSelect()
                    │     cache-first flow, planner.Plan(), Build(), Open()
                    │
                    └─ StmtDDL → executeDDL()
                          │
                    ddl.GetAction() switch:
                    ├─ CreateDDLAction → executeCreateTable()
                    │     1. CheckGlobalPermission(ActionDDL)
                    │     2. ddlColumnsToHeapSchema() — AST → heap.Schema
                    │     3. EncodeSchemaPayload() — heap.Schema → binary
                    │     4. catalog.AllocateTableID() — reserve ID
                    │     5. tables.CreateTableHeap() — physical init
                    │     6. catalog.RegisterTable() — metadata (only if 5 succeeds)
                    │
                    └─ DropDDLAction → executeDropTable()
                          1. CheckGlobalPermission(ActionDDL)
                          2. GetFromTables()[0] — extract table name
                          3. catalog.DropTable() — hard delete metadata
```

## API Shift

### Engine.Execute() Signature Change
- **Before:** `Execute(ctx, *Request) (RowStream, error)`
- **After:** `Execute(ctx, *Request) (*Result, error)`

The `Result` struct encapsulates stream/non-stream outcomes:
- SELECT → `Result.Stream` is non-nil, `RowsAffected` is 0
- DDL → `Result.Stream` is nil, `Message` has human-readable status
- DML (future) → `RowsAffected` is non-zero

### TxContext Extension
- `ReadTxID` — used for SELECT snapshot isolation
- `WriteTxID` — used for DDL/DML mutation timestamps

## Safe CREATE Ordering

To prevent inconsistent state (catalog pollution from failed heap init):

1. **Allocate table ID** from catalog (non-blocking, atomic bump)
2. **Initialize physical heap** on disk (I/O, outside catalog lock)
3. **If heap succeeds** → `RegisterTable()` in catalog
4. **If heap fails** → abort with zero catalog pollution

The catalog lock is held only for steps 1 and 3. Physical I/O in step 2 occurs
entirely outside the lock. This follows Golden Rule #4 (Lock Discipline).

## TxManager

`internal/engine/sql/tx` provides a basic atomic counter:
- `NextReadTx()` — allocates monotonically increasing read timestamp
- `NextWriteTx()` — allocates monotonically increasing write timestamp

Read timestamps start at 1. Write timestamps start at 1. Both are safe for
concurrent use.

## Type Mapping (Basic Tier)

| SQL Type | `key.Kind` | `engine.Type` |
| :--- | :--- | :--- |
| int, integer, bigint, smallint, tinyint, mediumint | `KindInt64` | `TypeInt64` |
| varchar, char, text, tinytext, mediumtext, longtext | `KindString` | `TypeString` |
| blob, varbinary, binary, tinyblob, mediumblob, longblob | `KindBytes` | `TypeBytes` |
| boolean, bool | `KindInt64` | `TypeInt64` |

Constraints (`NOT NULL`, `DEFAULT`, `PRIMARY KEY`) are explicitly ignored in Basic tier.

## Basic Tier Limitations
- DDL only (CREATE TABLE, DROP TABLE). INSERT/UPDATE/DELETE return `ErrUnsupportedFeature`.
- Hard delete on DROP TABLE — no physical file cleanup.
- No transactional DDL rollback — if CREATE fails halfway, partial state is cleaned up.
- First column is always the primary key.
- Zero-column tables rejected by parser or engine (`ErrEmptySchema`).
- Unsupported column types return a descriptive error.

## Lock Discipline

| Operation | Lock Held |
| :--- | :--- |
| `AllocateTableID()` | Catalog Write Lock (brief) |
| `CreateTableHeap()` | heapManager Write Lock (no catalog lock) |
| `RegisterTable()` | Catalog Write Lock (I/O inside) |
| `CheckGlobalPermission()` | Catalog Read Lock |
| `DropTable()` | Catalog Write Lock |

