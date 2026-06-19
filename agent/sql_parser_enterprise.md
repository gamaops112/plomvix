sql_parser_enterprise.md — Global SQL Parser (Enterprise Hardening)
Scope
This plan hardens the Global SQL Parser (`internal/sqlparser`) delivered 
in `sql_parser_setup.md`. It adds query normalization and fingerprinting 
(for execution plan caching), multi-error recovery (for IDEs and migration 
scripts), lightweight semantic pre-validation (to fail fast on structural 
SQL nonsense), and log-safe sanitization/comment stripping.

This plan does NOT deliver full semantic analysis (catalog resolution, 
type checking, or privilege verification). Those belong to the Query 
Planner and Catalog layers. This tier only validates structural SQL logic 
and prepares queries for downstream routing and caching.

Contract this tier honestly provides (read this before writing code)
- Unified Error Model with Byte Offsets: The Enterprise tier supersedes 
  the Basic `*SyntaxError`. All parsing methods return `*ParseError` (or 
  `ErrEmptySQL`). `ParseError` includes an `Offset` field to preserve the 
  exact byte offset from Vitess. If the offset is unknown (`-1`), Line 
  and Column default to `1`.
- Strict DDL Enforcement: `Parse()` explicitly checks if DDL statements 
  are fully parsed by testing both `vitess.DDLStatement` and `vitess.DBDDLStatement` 
  interfaces via `IsFullyParsed()`. Partial DDLs are rejected as syntax 
  errors, preserving the Basic tier's fail-fast guarantee.
- Comment-Agnostic Fingerprinting: `Fingerprint()` hashes the output of 
  `Sanitize()`. Optimizer hints and comments are intentionally ignored 
  for plan-cache grouping. Hint-aware caching is deferred.
- Guaranteed Sanitization Fallback: `Sanitize()` attempts to re-parse 
  stripped SQL. If re-parsing fails, it falls back to a strict **quote-aware 
  lexical state machine** (not regex) to redact literals, guaranteeing 
  zero PII leakage even on malformed edge cases.
- Non-Mutating Normalization: `Normalize()` uses a custom `NodeFormatter` 
  without mutating the AST.
- Race-Safe Caching: All derived string properties are cached using `sync.Once`.

Allowed Dependencies
- None. Uses only Go standard library (`crypto/sha256`, `strings`, `sync`, `fmt`, `errors`).

Public API Additions
package sqlparser

import "errors"

var (
    ErrSemanticValidation = errors.New("sqlparser: semantic validation failed")
)

// ParseError supersedes the Basic tier's SyntaxError.
type ParseError struct {
    Message string
    Line    int    // 1-based, absolute to the original script
    Column  int    // 1-based, rune count, absolute to the original script
    Offset  int    // byte offset relative to the statement/script; -1 if unknown
    Kind    string // "syntax" or "semantic"
    Cause   error  // The underlying error
}

func (e *ParseError) Error() string {
    return fmt.Sprintf("line %d, column %d (%s): %s", e.Line, e.Column, e.Kind, e.Message)
}

func (e *ParseError) Unwrap() error {
    return e.Cause
}

// Extended Statement interface
type Statement interface {
    // ... Basic methods ...
    Normalize() string
    Fingerprint() string
    Sanitize() string
    StripComments() string
}

// Extended Parser interface
type Parser interface {
    Parse(sql string) (Statement, error)
    ParseMulti(sql string) ([]Statement, error)
    ParseScript(sql string) ([]Statement, []*ParseError)
}

Tasks (do in order, one at a time)

Task 1 — Package updates, API extensions, and race-safe caching
Update `internal/sqlparser/parser.go`.
Add `ErrSemanticValidation` sentinel and `ParseError` struct (including `Offset int`).
Extend `Statement` and `Parser` interfaces.
Update `stmtWrapper` struct:
- Add `rawSQL string`.
- Add `p *vitess.Parser` to allow `Sanitize` to re-parse stripped SQL safely.
- Add `sync.Once` caching fields for `normalizedSQL`, `fingerprint`, `strippedSQL`, and `sanitizedSQL`.
Implement `isEmptySQL(sql string) bool` helper.
Tests: Verify interface compliance. Verify `ParseError` formatting. Verify `isEmptySQL`.

Task 2 — Non-Mutating Normalization & Strict DDL (`Normalize()` and `Parse()`)
Implement `Normalize()` on `stmtWrapper` using `normalizeOnce` and a custom `NodeFormatter` passed to Vitess's `TrackedBuffer`. Intercept `*vitess.Literal`, `*vitess.NullVal`, and `vitess.BoolVal` to write `?`.

Update `Parse()` on `*vitessParser`:
1. If `isEmptySQL(sql)`, return `nil, ErrEmptySQL`.
2. Call `stmt, _, err := v.p.Parse2(sql)`.
3. **Strict DDL Check:** Type-assert `stmt` to both `vitess.DDLStatement` and 
   `vitess.DBDDLStatement`. If either assertion succeeds and the interface's 
   `IsFullyParsed()` method returns `false`, return `*ParseError{Kind: "syntax", Message: "incomplete DDL", Offset: -1, Line: 1, Column: 1}`.
4. If `err` is not nil, map to `*ParseError`. Extract `PositionedErr`. If found, `Offset = pe.Pos`. Else `Offset = -1`.
5. **Offset Logic:** If `Offset >= 0`, compute 1-based `Line` and `Column` from the offset. If `Offset == -1`, set `Line=1, Column=1`.
6. If success, run `validateSemantics()`. If it returns a `*ParseError`, return it.
7. Return the wrapped statement (storing `v.p` in the wrapper).
Tests: Verify invalid SQL returns `*ParseError`. Verify partial DDL (both standard and DB-level) returns syntax error. Verify `Offset == -1` results in Line 1, Col 1.

Task 3 — Fingerprint Generation (`Fingerprint()`)
Implement `Fingerprint()` on `stmtWrapper`:
1. Use `fingerprintOnce.Do(func() { ... })`.
2. Call `Sanitize()`.
3. Compute SHA-256 hex hash.
4. Store in `fingerprint`.
Tests: Verify identical structures with different comments yield identical fingerprints.

Task 4 — Multi-Error Recovery & Absolute Positioning (`ParseScript()`)
Implement `ParseScript(sql string) ([]Statement, []*ParseError)` on `*vitessParser`:
1. If `isEmptySQL(sql)`, return `nil, []*ParseError{{...ErrEmptySQL...}}`.
2. Call `pieces, err := v.p.SplitStatementToPieces(sql)`.
3. Track `prevEnd int`.
4. Iterate through pieces:
   - If `isEmptySQL(piece)`, skip.
   - Find `pieceStartOffset` using `strings.Index` from `prevEnd`. Handle `-1` fallback.
   - Call `stmt, parseErr := v.Parse(piece)`.
   - If success, append `stmt`.
   - If `parseErr` is not nil:
     - PRESERVE KIND.
     - If `Kind == "syntax"` and `parseErr.Offset >= 0`: `absoluteOffset = pieceStartOffset + parseErr.Offset`.
     - If `Kind == "semantic"` or `parseErr.Offset == -1`: `absoluteOffset = pieceStartOffset`.
     - Compute 1-based `Line` and `Column` from `absoluteOffset`.
     - Append updated `ParseError`.
   - Update `prevEnd`.
5. If `validStmts` is empty and `errs` is empty (all pieces were empty), return `ErrEmptySQL`.
6. Return `validStmts, errs`.
Tests: Verify absolute positioning. Verify skipping empty pieces. Verify `Offset == -1` mapping.

Task 5 — Semantic Pre-Validation
Implement `validateSemantics(stmt vitess.Statement) *ParseError` as defined previously (INSERT arity, SELECT/UPDATE duplicates). Return `*ParseError` with `Offset: 0` (mapped to piece start by `ParseScript`).

Task 6 — `ParseMulti()` Error Mapping & Empty Skipping
Implement `ParseMulti(sql string)` on `*vitessParser`:
1. If `isEmptySQL(sql)`, return `nil, ErrEmptySQL`.
2. Call `SplitStatementToPieces`.
3. Iterate pieces. If `isEmptySQL(piece)`, skip.
4. Call `v.Parse(piece)`.
5. If error, return `nil, parseErr` immediately (fail-fast).
6. If all pieces were skipped, return `nil, ErrEmptySQL`.
7. Return `[]Statement, nil`.
Tests: Verify fail-fast behavior. Verify skipping empty pieces.

Task 7 — Quote-Aware Comment Stripping & Guaranteed Sanitization
Implement `StripComments()` on `stmtWrapper` using `stripOnce` and a strictly quote-aware raw SQL lexical scanner.

Implement `Sanitize()` on `stmtWrapper`:
1. Use `sanitizeOnce.Do(func() { ... })`.
2. Call `StripComments()`.
3. Attempt to parse the stripped SQL using the stored parser `v.p`.
4. If parse succeeds, run the `NodeFormatter` normalization.
5. **LEXICAL FALLBACK:** If re-parsing fails, DO NOT use regex. Implement a strict **quote-aware lexical state machine** over the stripped SQL to redact:
   - Single-quoted strings (`'...'` -> `?`)
   - Double-quoted strings (`"..."` -> `?`)
   - Numeric literals (`123`, `1.5`, `-42`, `0xFF` -> `?`)
   This machine must respect SQL escapes, correctly distinguish between 
   numeric literals and alphanumeric identifiers (e.g., `col1`, `table2`), 
   and never expose raw literals.
6. Store in `sanitizedSQL`.

Tests: 
- `SELECT /* hidden */ 1 -- comment` -> `SELECT ?`
- `SELECT /* email: test@example.com */ 'secret' -- token` -> `SELECT ?`
- Lexical fallback numeric boundaries: `SELECT col1 FROM table2 WHERE id = 123` -> `SELECT col1 FROM table2 WHERE id = ?` (identifiers preserved).
- Lexical fallback numeric formats: `SELECT 0xFF, 1.5, -42` -> `SELECT ?, ?, ?` (hex, float, negative int redacted).
- Simulate re-parse failure -> verify lexical fallback replaces literals with `?` and leaks no PII.

Task 8 — Concurrency & Edge Cases
- `go test -race`.
- Massive script parsing.

Task 9 — Benchmarks
- Benchmark `Normalize`, `Fingerprint`, `Sanitize`, `ParseScript`.

Task 10 — docs/sql_parser.md update & Dependency Verification
Update documentation covering:
- `Offset` field and `-1` default behavior.
- Strict DDL enforcement via `IsFullyParsed()` on both DDL interfaces.
- `Sanitize()` lexical fallback (no regex, identifier-safe).
- **Note:** Optimizer hints/comments are intentionally ignored by `Fingerprint()`.
Run `go mod tidy`. Update substring-check test.

Completion criteria
All 10 tasks implemented and tested. `go test -race` passes. `ParseError` includes `Offset`. `Parse()` enforces strict DDL on both `DDLStatement` and `DBDDLStatement`. `Sanitize()` uses an identifier-safe lexical fallback. `Fingerprint()` hashes sanitized SQL. `ParseMulti` and `ParseScript` skip empty pieces. `stmtWrapper` holds the parser instance. Documentation updated.