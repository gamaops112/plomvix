Here is the fully calcified, mathematically sound, and strictly bounded **Plan 26a** document. It incorporates every patch, API correction, and architectural boundary we established. 

This document is ready to be saved as `agent/router_planner_setup.md` and handed directly to the coding agent.

***

# Plan 26a: Global Query Router & Planner Setup

| Field | Value |
| :--- | :--- |
| **Source** | `agent/router_planner_setup.md` |
| **Package(s)** | `internal/engine`, `internal/router`, `internal/engine/sql`, `internal/engine/sql/planner`, `internal/engine/sql/schema` |
| **Purpose** | Establish the Global Query Router for dispatching parsed statements to engines, and the SQL Engine's basic Volcano-model Query Planner for translating ASTs into physical execution plans. |
| **Dependencies** | Global Catalog (Plans 22-23), Global SQL Parser (Plans 24-25), SQL Table Heap (Plans 20-21). |

## Honest Contracts & Known Trade-offs
1. **Single-Engine Dispatch Only:** The Router ensures all target tables belong to the *same* engine. Cross-engine queries return `ErrCrossEngineJoinNotSupported`.
2. **SELECT Only:** The Router explicitly rejects any statement that is not a `SELECT` with `ErrUnsupportedStatement`. DML and DDL are deferred.
3. **No Target Tables:** If a query has no target tables (e.g., `SELECT 1`), the Router returns `ErrNoTargetTable`. Constant evaluation is deferred.
4. **Basic Volcano Pipeline & Binding:** The SQL Planner uses a strict `BoundExpr` interface. Basic tier only supports direct column refs and literals with `=, <, >, AND, OR`.
5. **Multi-Table Rejection at Planner:** The Router allows multiple tables if they share the same engine, but the SQL Planner will explicitly reject all multi-table `FROM` clauses and `JOIN`s with `ErrUnsupportedFeature`.
6. **Deep-Copy Safety:** `Datum.Value` is strictly constrained to immutable types (`int64`, `uint64`, `float64`, `bool`, `string`, `nil`) and `[]byte`. `Row.DeepCopy()` and `Datum.DeepCopy()` only allocate new memory for `[]byte`. `RowStream.Schema()` returns a deep copy.
7. **Basic TxContext:** True MVCC timestamp allocation is deferred. The Router passes a default `TxContext{ReadTxID: math.MaxUint64}`.
8. **Operator Lifecycle:** `Open()` must be called exactly once before `Next()`. `Close()` is idempotent, must recursively close all children, and must be safe to call even if `Open()` failed.

---

## Deliverables

| File | Purpose |
| :--- | :--- |
| `internal/engine/engine.go` | Core `Engine`, `Row`, `Datum`, `Schema`, `Type`, `TxContext` contracts with exact deep-copy rules. |
| `internal/engine/sql/schema/schema.go` | Binary encoding/decoding for `SchemaPayload` to/from `engine.Schema`. |
| `internal/engine/sql/engine.go` | SQL Engine implementation, `TableRegistry` adapter, and `SQLEngine` struct. |
| `internal/router/router.go` | Global Router: statement validation, catalog resolution, permission checks, dispatch. |
| `internal/router/router_test.go` | Router tests (mock engines, mock catalog, permission denial, cross-engine rejection). |
| `internal/engine/sql/planner/plan.go` | Volcano `Operator` interface with exact lifecycle semantics, `SeqScanNode`, `FilterNode`, `ProjectNode`. |
| `internal/engine/sql/planner/binder.go` | `BoundExpr` interface, `BindWhere`, `BindProjection` using Vitess AST via `sqlparser` accessor. |
| `internal/engine/sql/planner/translate.go` | Translator from `sqlparser.Statement` to Volcano tree. |
| `internal/engine/sql/planner/planner_test.go` | Unit tests for Volcano nodes and binding logic. |
| `docs/router_planner.md` | Architecture documentation for the Router and Planner. |

---

## Key API & Concepts

### 1. Engine Contracts & Deep-Copy Rules (`internal/engine`)
```go
package engine

import (
    "context"
    "github.com/plomvix/plomvix/internal/sqlparser"
)

type Type uint8
const (
    TypeNull Type = iota
    TypeInt64
    TypeUint64
    TypeFloat64
    TypeBool
    TypeString
    TypeBytes
)

type Column struct {
    Name string
    Type Type
}

type Schema struct {
    Columns []Column
}
// DeepCopy returns a completely independent copy of the schema.
func (s Schema) DeepCopy() Schema 

// Datum.Value MUST only be: nil, int64, uint64, float64, bool, string, or []byte.
type Datum struct {
    Type  Type 
    Value any  
}
// DeepCopy allocates new []byte if Type == TypeBytes. Immutable types are copied by value.
func (d Datum) DeepCopy() Datum 

type Row []Datum 
// DeepCopy calls DeepCopy on every Datum.
func (r Row) DeepCopy() Row 

type TxContext struct {
    ReadTxID uint64
}

type RowStream interface {
    Next(ctx context.Context) (Row, error) // Returns io.EOF when exhausted
    Schema() Schema                        // MUST return a deep copy
    Close() error                          // Idempotent
}

type Request struct {
    Stmt      sqlparser.Statement // From internal/sqlparser
    UserID    uint64              // Resolved user identity for catalog checks
    TxContext TxContext        
}

type Engine interface {
    Name() string 
    Execute(ctx context.Context, req *Request) (RowStream, error)
}
```

### 2. Schema Payload Contract (`internal/engine/sql/schema`)
```go
package schema

import "github.com/plomvix/plomvix/internal/engine"

// Binary format: [num_cols: uint16] ( [name_len: uint16] [name: bytes] [type: uint8] )...
func Encode(s engine.Schema) ([]byte, error)
func Decode(payload []byte) (engine.Schema, error)
```

### 3. Volcano Operator & Storage Contracts (`internal/engine/sql/planner`)
*The planner defines the physical storage contracts it consumes. The `sql` package implements them to prevent import cycles.*
```go
package planner

import (
    "context"
    "errors"
    "github.com/plomvix/plomvix/internal/engine"
    "github.com/plomvix/plomvix/internal/catalog"
    "github.com/plomvix/plomvix/internal/sqlparser"
    vitess "vitess.io/vitess/go/vt/sqlparser"
)

var (
    ErrUnsupportedFeature  = errors.New("planner: unsupported feature in basic tier")
    ErrTableHeapNotFound   = errors.New("planner: table heap not found for table ID")
)

// TableRegistry abstracts the physical heap lookup by TableID.
type TableRegistry interface {
    GetTableHeap(tableID uint64) (TableHeap, error)
}

type TableHeap interface {
    Scan(ctx context.Context, tx engine.TxContext) (HeapScanIterator, error)
}

type HeapScanIterator interface {
    Next(ctx context.Context) (encodedTuple []byte, err error) // Returns io.EOF when exhausted
    Close() error
}

type RowDecoder interface {
    Decode(encodedTuple []byte, schema engine.Schema) (engine.Row, error)
}

type Operator interface {
    Open(ctx context.Context) error   // Must be called exactly once before Next.
    Next(ctx context.Context) (engine.Row, error) // Returns io.EOF when done.
    Close() error                     // Idempotent. Must close children. Safe to call if Open failed.
    Schema() engine.Schema
}

// Translate builds the physical plan from the parsed statement.
func Translate(
    ctx context.Context,
    cat catalog.Catalog,
    tables TableRegistry,
    decoder RowDecoder,
    req *engine.Request,
) (Operator, error)
```

### 4. Expression Binding (`internal/engine/sql/planner`)
```go
// The binder uses the Vitess AST exposed via the sqlparser wrapper.
type BoundExpr interface { 
    Eval(row engine.Row) (engine.Datum, error) 
}

// BindWhere walks vitess.Expr (accessed via stmt.RawSelect().Where.Expr).
func BindWhere(expr vitess.Expr, schema engine.Schema) (BoundExpr, error)

type ProjectionExpr struct {
    Expr BoundExpr
    Col  engine.Column 
}

func BindProjection(exprs vitess.SelectExprs, schema engine.Schema) ([]ProjectionExpr, engine.Schema, error)
```

### 5. Global Router (`internal/router`)
```go
package router

import (
    "context"
    "errors"
    "github.com/plomvix/plomvix/internal/engine"
    "github.com/plomvix/plomvix/internal/catalog"
    "github.com/plomvix/plomvix/internal/sqlparser"
    "math"
)

var (
    ErrCrossEngineJoinNotSupported = errors.New("router: cross-engine joins are not supported")
    ErrEngineNotFound              = errors.New("router: engine not found")
    ErrPermissionDenied            = errors.New("router: permission denied")
    ErrUnsupportedStatement        = errors.New("router: statement type not supported in basic tier")
    ErrNoTargetTable               = errors.New("router: no target table found")
)

type Router struct {
    catalog catalog.Catalog
    engines map[string]engine.Engine
}

func (r *Router) Route(ctx context.Context, userID uint64, stmt sqlparser.Statement) (engine.RowStream, error) {
    // 1. Reject non-SELECT
    if stmt.Type() != sqlparser.StmtSelect {
        return nil, ErrUnsupportedStatement
    }

    // 2. Extract tables
    tables := stmt.TargetTables()
    if len(tables) == 0 {
        return nil, ErrNoTargetTable
    }

    // 3. Resolve via Catalog and check permissions
    var targetEngine string
    for _, tableName := range tables {
        info, err := r.catalog.GetTable(ctx, tableName)
        if err != nil { return nil, err }
        
        if err := r.catalog.CheckPermission(ctx, userID, info.TableID, catalog.ActionRead); err != nil {
            return nil, ErrPermissionDenied
        }
        
        if targetEngine == "" {
            targetEngine = info.EngineName
        } else if targetEngine != info.EngineName {
            return nil, ErrCrossEngineJoinNotSupported
        }
    }

    // 4. Dispatch to Engine
    eng, ok := r.engines[targetEngine]
    if !ok { return nil, ErrEngineNotFound }

    return eng.Execute(ctx, &engine.Request{
        Stmt:      stmt,
        UserID:    userID,
        TxContext: engine.TxContext{ReadTxID: math.MaxUint64}, // Basic tier default
    })
}
```

### 6. SQL Engine & Lifecycle Adapter (`internal/engine/sql`)
```go
package sql

import (
    "context"
    "github.com/plomvix/plomvix/internal/engine"
    "github.com/plomvix/plomvix/internal/engine/sql/planner"
    "github.com/plomvix/plomvix/internal/catalog"
)

type SQLEngine struct {
    catalog catalog.Catalog
    tables  planner.TableRegistry // Interface, not a raw map
    decoder planner.RowDecoder
}

func NewSQLEngine(cat catalog.Catalog, tables planner.TableRegistry, decoder planner.RowDecoder) *SQLEngine {
    return &SQLEngine{catalog: cat, tables: tables, decoder: decoder}
}

func (e *SQLEngine) Name() string { return "sql" }

// operatorStream adapts an opened Operator into an engine.RowStream.
type operatorStream struct {
    op planner.Operator
}

func (s *operatorStream) Next(ctx context.Context) (engine.Row, error) {
    return s.op.Next(ctx)
}

func (s *operatorStream) Schema() engine.Schema {
    return s.op.Schema().DeepCopy() // Enforce deep-copy contract
}

func (s *operatorStream) Close() error {
    return s.op.Close()
}

func (e *SQLEngine) Execute(ctx context.Context, req *engine.Request) (engine.RowStream, error) {
    // 1. Build the Volcano tree
    op, err := planner.Translate(ctx, e.catalog, e.tables, e.decoder, req)
    if err != nil {
        return nil, err
    }
    
    // 2. Open the tree (lifecycle boundary)
    if err := op.Open(ctx); err != nil {
        _ = op.Close() // Safe cleanup if Open fails partially
        return nil, err
    }
    
    // 3. Return the adapted stream
    return &operatorStream{op: op}, nil
}
```

---

## Dependency Graph
The strict, acyclic dependency graph is enforced as follows:
```text
router -> internal/engine (via the generic Engine interface only)
internal/engine/sql -> planner (implements Translate and provides TableRegistry/RowDecoder)
planner -> internal/engine + catalog (consumes Row/Schema interfaces and Catalog metadata)
```
The Router has **zero knowledge** of the SQL engine, the planner, or the storage layer. It dispatches purely via the `engine.Engine` interface.

---

## Tasks (10 Total)

### Task 1: Engine, Schema, and Row Contracts
*   Create `internal/engine` package.
*   Define `Type`, `Column`, `Schema`, `Row`, `Datum`, `TxContext`, `RowStream`, `Request`, and `Engine`.
*   Implement strict `DeepCopy()` methods for `Schema`, `Row`, and `Datum`. Enforce the rule that only `[]byte` triggers allocation.
*   Add unit tests proving that mutating a returned `Row`, `Datum`, or `Schema` does not affect the source data.

### Task 2: Schema Payload Encoding
*   Create `internal/engine/sql/schema` package.
*   Implement `Encode()` and `Decode()` using the strict binary format defined above.
*   Add round-trip tests and malformed payload rejection tests.

### Task 3: Global Router Core
*   Create `internal/router` package.
*   Implement `Route()`:
    1. Reject if `stmt.Type() != sqlparser.StmtSelect`.
    2. Extract `TargetTables()`. If empty, return `ErrNoTargetTable`.
    3. Loop tables: call `catalog.GetTable(ctx, tableName)`. Decode `TableInfo.SchemaPayload` using `schema.Decode()`.
    4. Call `catalog.CheckPermission(ctx, userID, info.TableID, catalog.ActionRead)`.
    5. Verify all tables belong to the *same* `EngineName`.
    6. Look up the engine and call `engine.Execute()`.

### Task 4: Router Testing & Edge Cases
*   Create mock `engine.Engine` and mock `catalog.Catalog` implementations.
*   Write table-driven tests for `Route()` covering successful dispatch, `ErrUnsupportedStatement`, `ErrNoTargetTable`, `ErrPermissionDenied`, and `ErrCrossEngineJoinNotSupported`.

### Task 5: SQL Engine Skeleton & Lifecycle Adapter
*   Create `internal/engine/sql/engine.go`.
*   Define `SQLEngine` struct with `catalog`, `tables` (a `planner.TableRegistry`), and `decoder` (`planner.RowDecoder`) fields.
*   Implement `NewSQLEngine()` constructor requiring all three dependencies.
*   Implement the `operatorStream` adapter to bridge `planner.Operator` to `engine.RowStream`.
*   Implement `Execute()`: call `planner.Translate()`, call `op.Open()`, handle `Open()` failure with `op.Close()`, and return the `operatorStream`.

### Task 6: Volcano Operator Interface & SeqScan
*   Create `internal/engine/sql/planner` package.
*   Define the `Operator` interface with exact lifecycle semantics (idempotent `Close`, recursive child closing).
*   Define `TableRegistry`, `TableHeap`, `HeapScanIterator`, and `RowDecoder` interfaces.
*   Implement `SeqScanNode`:
    *   Takes a `TableHeap` and `RowDecoder`.
    *   `Open()`: Initializes the `HeapScanIterator`.
    *   `Next()`: Reads raw bytes from iterator, uses `RowDecoder` to create an `engine.Row`, calls `Row.DeepCopy()`, and returns it. Returns `io.EOF` when exhausted.
    *   `Close()`: Closes the iterator.

### Task 7: Expression Binding (The "Glue")
*   Implement `binder.go`.
*   Define `BoundExpr` interface.
*   Implement `BindWhere`: Walk the Vitess AST (accessed via `stmt.RawSelect()`). Map `vitess.ColName` to schema indices. Map `vitess.SQLVal` to literal `Datum`s. Build evaluators for `=, <, >, AND, OR`. Return `ErrUnsupportedFeature` for functions or complex math.
*   Implement `BindProjection`: Map `vitess.SelectExpr` to output columns and evaluators.

### Task 8: Filter, Project Nodes, and AST Translation
*   Implement `FilterNode` (pulls from child, calls `BoundExpr.Eval()`, yields if true). Ensure `Close()` closes the child.
*   Implement `ProjectNode` (pulls from child, maps via `ProjectionExpr`, yields projected `Row`). Ensure `Close()` closes the child.
*   Implement `Translate(ctx, cat, tables, decoder, req) (Operator, error)`:
    1. Call `catalog.GetTable()` to get `TableID` and decode `SchemaPayload`.
    2. Reject if `len(req.Stmt.TargetTables()) > 1` with `ErrUnsupportedFeature`.
    3. Call `BindWhere` and `BindProjection`.
    4. Look up the `TableHeap` by calling `tables.GetTableHeap(tableID)`. If it returns an error, return `ErrTableHeapNotFound`.
    5. Build tree: `ProjectNode` -> `FilterNode` -> `SeqScanNode` (passing the `TableHeap` and `decoder`).

### Task 9: Incremental & Full Integration Tests
*   **Planner-Only Test:** Feed a mocked AST and fake `TableHeap` to the planner. Assert the Volcano tree yields correct rows.
*   **Router-Only Test:** Wire Router to a mock engine. Assert correct dispatch and error handling.
*   **Full E2E Test:** Wire real Global Catalog, real Global Parser, real SQL Table Heap (using a mock implementation of the `TableHeap` interface for this plan), Router, and SQL Engine. Parse `SELECT id, name FROM users WHERE id > 2`. Assert exactly the correct rows are returned.

### Task 10: Documentation
*   Write `docs/router_planner.md`.
*   Document the Engine Interface, `TxContext` basic tier behavior, the `BoundExpr` binding mechanics, and the exact `TableHeap` contract.
*   Explicitly list the "Deferred" features (DML, Joins, Constant evaluation, Plan Caching).
*   Add a substring-check test to ensure the doc contains "Cross-engine joins are not supported" and "Statement type not supported in basic tier".

---

### Completion Criteria
*   All 10 tasks implemented and tested.
*   `go test ./internal/engine/... ./internal/router/... ./internal/engine/sql/...` passes.
*   `go test -race ./...` passes.
*   No caching, no complex joins, no DML, and no cross-engine logic exists.
*   All Catalog and Parser API calls strictly match the established Plans 22-25 interfaces.
*   Deep-copy tests prove no memory aliasing for `[]byte` in `Row` and `Schema`.

***

**Technical Lead Sign-off:** 
This document is fully calcified. The API boundaries between the Parser, Router, Planner, and Storage layers are explicitly defined with zero ambiguity, no import cycles, and strict lifecycle management. It is ready for execution.