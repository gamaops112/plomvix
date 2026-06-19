sql_parser_setup.md — Global SQL Parser (Basic)
Scope
This plan delivers the Global SQL Parser (`sqlparser` package, located at 
`internal/sqlparser`), the first layer of the server's query processing 
pipeline. It takes raw SQL text from the wire protocol, parses it into a 
strongly-typed Abstract Syntax Tree (AST) using the industry-standard 
Vitess SQL parser, and wraps it in a Plomvix `Statement` interface.

This plan does NOT deliver the Query Router, Query Planner, or Query 
Executor. It strictly handles text-to-AST conversion, syntax error 
mapping, and base-table extraction for downstream routing.

Contract this tier honestly provides (read this before writing code)
- Production-Grade Parsing: Leverages `vitess.io/vitess/go/vt/sqlparser` 
  to support full, complex SQL (JOINs, Subqueries, CTEs, DDL). 
  *Dialect Note:* Vitess parses MySQL-compatible SQL.
- The Wrapper Pattern: The parser returns a Plomvix `Statement` interface. 
  This interface exposes engine-agnostic metadata (`Type()`, `TargetTables()`) 
  for the Global Router, while exposing the raw Vitess AST (`RawAST()`) 
  for the Engine Planners.
- Base-Table Extraction: `TargetTables()` intelligently walks CTEs and 
  derived tables (subqueries in FROM clauses), extracts the underlying 
  base tables, and explicitly excludes CTE names and table aliases from 
  the final output. This ensures the Router queries the Catalog for real 
  tables, not ephemeral aliases.
- Fail-Fast Syntax Errors (Including DDL): Parsing aborts on the first 
  syntax error. Vitess's standard `Parse` may silently return partially 
  parsed DDL; this plan explicitly enforces strict DDL parsing (via 
  `ParseStrictDDL` or `IsFullyParsed()` checks) to ensure malformed DDL 
  is rejected immediately. Vitess positioned errors are translated into 
  exact 1-based Line/Column numbers (counting runes, not bytes).
- Empty SQL Handling: Both `Parse` and `ParseMulti` explicitly reject 
  empty, whitespace-only, or semicolon-only strings with `ErrEmptySQL` 
  before invoking the Vitess parser.

Allowed Dependencies
- `vitess.io/vitess` (Explicitly allowed. Must be pinned to a specific 
  release tag in `go.mod`. `go mod tidy` must be run to document all 
  transitive dependencies).

Public API
package sqlparser

import "errors"

type StmtType int
const (
    StmtSelect StmtType = iota
    StmtInsert
    StmtUpdate
    StmtDelete
    StmtDDL
    StmtUnknown
)

// Statement is the engine-agnostic wrapper around the parsed AST.
type Statement interface {
    Type() StmtType
    TargetTables() []string // Normalized: lowercase, unqualified base tables only
    RawAST() any            // The underlying vitess.Statement
    String() string         // Reconstructed SQL string
}

// SyntaxError represents a fail-fast parsing error with location tracking.
type SyntaxError struct {
    Message string
    Line    int // 1-based
    Column  int // 1-based, rune count
}
func (e *SyntaxError) Error() string

var (
    ErrEmptySQL = errors.New("sqlparser: empty SQL statement")
)

// Parser is the global SQL parsing service.
type Parser interface {
    Parse(sql string) (Statement, error)
    ParseMulti(sql string) ([]Statement, error)
}

func New() (Parser, error)

Tasks (do in order, one at a time)

Task 1 — Package skeleton, Dependency Pinning, and Import Aliasing
Create `internal/sqlparser/parser.go`. 
*CRITICAL:* The Go package name is `sqlparser`. To avoid a naming collision 
with the Vitess import, you MUST alias the Vitess package:
`import vitess "vitess.io/vitess/go/vt/sqlparser"`
Run `go get vitess.io/vitess@<specific-release-tag>`.
Define `StmtType`, `Statement` interface, `SyntaxError` struct, `ErrEmptySQL`, and `Parser` interface.
Implement `New() (Parser, error)`:
- Call `vitess.New(vitess.Options{})` to create the underlying Vitess parser instance.
- Return a concrete `*vitessParser` struct holding `p *vitess.Parser`.
Tests: Verify `New()` initializes without error. Verify `SyntaxError` formats correctly.

Task 2 — Single Statement Parsing, Empty Checks, Strict DDL & Exact Error Mapping
Implement `Parse(sql string)` on `*vitessParser`:
1. EMPTY CHECK: If `strings.TrimSpace(sql)` is empty, or contains only 
   semicolons and whitespace, return `nil, ErrEmptySQL` immediately.
2. Call `stmt, err := v.p.Parse(sql)`.
3. If error, attempt to extract position:
   - Use `errors.As(err, &positionedErr)` (where `positionedErr` is the 
     Vitess specific positioned error type, e.g., `vitess.PositionedErr`).
   - If position is available, map the byte offset to 1-based `Line` and 
     `Column`. *CRITICAL:* Column must count **runes**, not bytes, to 
     handle multi-byte UTF-8 characters safely.
   - If no position is available, default to `Line=1, Column=1`.
   - Return `&SyntaxError{Message: err.Error(), Line: line, Column: col}`.
4. *CRITICAL DDL STRICTNESS:* Vitess's standard `Parse` may silently return 
   a partially parsed DDL statement without an error. To enforce the fail-fast 
   contract, check if `stmt` is a DDL (e.g., type assertion to `vitess.DDLStatement` 
   or `vitess.DBDDLStatement`). If it is, verify it is fully parsed (e.g., 
   `!stmt.IsFullyParsed()`). Alternatively, pass the SQL to `v.p.ParseStrictDDL(sql)` 
   as a secondary validation step. If the DDL is partial or invalid, return 
   a `SyntaxError` indicating incomplete or malformed DDL.
5. If success and fully valid, wrap the resulting `vitess.Statement` in our internal `stmtWrapper`.
Tests: Parse valid SELECT. Parse invalid SQL -> verify SyntaxError contains correct 1-based Line/Column. Parse SQL with multi-byte UTF-8 characters before the error -> verify Column counts runes correctly. Parse("") -> ErrEmptySQL. Parse("   ") -> ErrEmptySQL. Parse(";") -> ErrEmptySQL. Parse malformed DDL (e.g., "CREATE TABLE foo (id int") -> verify it returns a SyntaxError and NOT a partial AST.

Task 3 — Statement Type Resolution & String Reconstruction
Implement `Type()` on `stmtWrapper`:
- Use a type-switch on the underlying Vitess AST node.
- Map `*vitess.Select` -> `StmtSelect`
- Map `*vitess.Insert` -> `StmtInsert`
- Map `*vitess.Update` -> `StmtUpdate`
- Map `*vitess.Delete` -> `StmtDelete`
- Map `*vitess.CreateTable`, `DropTable`, etc. -> `StmtDDL`
- Default -> `StmtUnknown`

Implement `String()` on `stmtWrapper`:
- Explicitly use `vitess.String(v.ast)` to reconstruct the SQL string.
Tests: Verify all major SQL commands map to the correct StmtType. Verify String() reconstructs valid SQL.

Task 4 — Base-Table Extraction, CTEs, and Aliases (The Visitor)
Implement `TargetTables()` on `stmtWrapper`. This is the most critical task for the Router.
Use `vitess.Walk` or a recursive AST traversal to extract base table names.
CRITICAL ALIAS/CTE RULES:
1. Maintain a set of `knownAliases` (e.g., `map[string]struct{}`).
2. When encountering a `WITH` clause (CTE), add the CTE name to `knownAliases`, 
   but CONTINUE walking the CTE's inner `SELECT` body to find real base tables.
3. When encountering a derived table (a subquery in a `FROM` or `JOIN` clause 
   with an alias, e.g., `FROM (SELECT...) AS u`), add the alias `u` to `knownAliases`, 
   and CONTINUE walking the subquery.
4. When encountering a standard `TableName` (e.g., `*vitess.TableName`):
   - Extract the table name string.
   - Check if it exists in `knownAliases`. If YES, ignore it (it's a CTE or derived table reference).
   - If NO, it is a real base table. Apply normalization:
     a. Strip any database/schema qualifiers (e.g., `mydb.users` -> `users`).
     b. Convert to lowercase.
     c. Add to the final result set.
5. Return a deduplicated slice of the final base tables.
6. If the statement has no base tables (e.g., `SELECT 1`), return an empty slice `[]string{}`.

Tests: 
- Simple SELECT -> 1 lowercase table.
- `SELECT * FROM MyDB.Users` -> `["users"]`.
- Complex JOIN -> 2+ tables.
- `WITH recent AS (SELECT * FROM orders) SELECT * FROM recent` -> `["orders"]` (recent is excluded).
- `SELECT * FROM (SELECT * FROM users) AS u` -> `["users"]` (u is excluded).
- `SELECT 1` -> empty slice.

Task 5 — Multi-Statement Parsing
Implement `ParseMulti(sql string)` on `*vitessParser`:
1. EMPTY CHECK: If `strings.TrimSpace(sql)` is empty or contains only semicolons/whitespace, return `nil, ErrEmptySQL`.
2. Call `pieces, err := v.p.SplitStatementToPieces(sql)`.
3. If error, map to `SyntaxError` using the same logic as Task 2.
4. Iterate through the pieces. If a piece is empty/whitespace after splitting, skip it.
5. Call `v.Parse()` on each valid piece.
6. If any piece fails, return the `SyntaxError` immediately (fail-fast).
7. Return `[]Statement`.
Tests: Parse "SELECT 1; SELECT 2;". Parse "SELECT 1; INVALID; SELECT 2;" -> fails on INVALID. Parse "" -> ErrEmptySQL. Parse ";" -> ErrEmptySQL.

Task 6 — Concurrency & Edge Cases
- `go test -race`: Parse concurrently from 50 goroutines. Verify that the 
  held `*vitess.Parser` instance is thread-safe (Vitess uses `sync.Pool` 
  internally for the yacc parser state, so the `*Parser` struct itself 
  is safe to share across goroutines).
- Massively long query -> verify no OOM or stack overflow.

Task 7 — Benchmarks
- Benchmark `Parse` on a simple SELECT.
- Benchmark `Parse` on a complex 10-table JOIN with subqueries and CTEs.
- Benchmark `TargetTables` extraction on complex CTE queries.

Task 8 — docs/sql_parser.md & Dependency Verification
Write documentation covering:
- The Vitess dependency and **MySQL-dialect** baseline.
- The Wrapper Pattern (why we don't translate the AST).
- Base Table extraction rules (CTE and derived-table alias exclusion).
- Syntax error Line/Column mapping (rune counting) and Strict DDL enforcement.
Run `go mod tidy` and explicitly document all transitive dependencies pulled in by Vitess in the docs.
Add substring-check test in `internal/sqlparser/docs_test.go`.

Completion criteria
All 8 tasks implemented and tested. `go test -race` passes. The Parser 
successfully wraps the Vitess AST into the Plomvix `Statement` interface. 
`TargetTables()` correctly extracts base tables, excludes CTEs/aliases, 
strips qualifiers, and lowercases names. Syntax errors return exact 
1-based Line/Column numbers (counting runes). Malformed DDL is strictly 
rejected via `ParseStrictDDL` / `IsFullyParsed()` checks. `Parse` and 
`ParseMulti` correctly handle empty/semicolon-only strings with `ErrEmptySQL`. 
`go mod tidy` confirms the pinned Vitess version and documents transitive deps. 
Documentation exists.