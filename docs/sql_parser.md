# Plomvix SQL Parser

The sqlparser package wraps the Vitess SQL parser to provide production-grade
SQL parsing with engine-agnostic AST metadata.

## Vitess Dependency

This package depends on `vitess.io/vitess/go/vt/sqlparser` (v0.24.1), which
parses MySQL-compatible SQL including JOINs, subqueries, CTEs, and DDL.

## Wrapper Pattern

The parser returns a Plomvix `Statement` interface that exposes:
- `Type()` - StmtSelect, StmtInsert, StmtUpdate, StmtDelete, or StmtDDL
- `TargetTables()` - Lowercase, unqualified base table names
- `RawAST()` - The underlying Vitess AST for engine planners
- `String()` - Reconstructed SQL

## TargetTables Extraction

Base table extraction uses Vitess's `ExtractAllTables` and then:
- Strips schema qualifiers (mydb.users -> users)
- Excludes CTE names
- Excludes the MySQL "dual" pseudo-table
- Returns lowercased names

## Error Handling

- `ErrEmptySQL` for empty/whitespace/semicolon-only input
- `SyntaxError` with message, line, and column for parse failures
- Strict fail-fast on syntax errors

## Concurrency

The Vitess Parser is safe for concurrent use across goroutines.
