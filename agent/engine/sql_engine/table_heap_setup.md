table_heap_setup.md — Table Heap (Basic Row Storage)
Scope
This plan delivers the Table Heap (`heap` package), the first relational 
layer in the SQL Engine. It maps relational rows (schemas, columns, primary 
keys) onto the on-disk B+ Tree KVStore (`kv` package) using the wire-level 
key encoding (`feature1.md`) and storage composite encoding (`sql_key_setup.md`).

This plan does NOT deliver MVCC (row versioning), NULL value support, 
secondary indexes, or complex constraints (CHECK, FOREIGN KEY). It provides 
strict Primary Key uniqueness and full-table sequential scans.

Contract this tier honestly provides (read this before writing code)
- Row Serialization: Row values are serialized using `key.EncodeStorageComposite`. 
  This provides safe, length-prefixed framing for variable-length strings/bytes.
- Key Mapping: Rows are addressed using `key.EncodeTableRowKey`. The Basic 
  tier hardcodes the MVCC `version` parameter to `0`.
- Strict NOT NULL: The Basic tier does not support NULL values. All columns 
  are implicitly NOT NULL. Passing `nil` for any column returns `ErrNullNotSupported`.
- Primary Key Uniqueness: `Insert` performs a read-before-write to enforce 
  strict PK uniqueness, returning `ErrDuplicateKey` if the row exists.
- Buffered Iterator Facade: `Scan` returns a `Rows` iterator over a buffered 
  KV scan result. True streaming scan (O(1) memory) is deferred until KVStore 
  exposes a cursor/callback scan API.

Constants & Limits
package heap

import (
    "context"
    "errors"
    "github.com/plomvix/plomvix/internal/engine/sql/key"
    "github.com/plomvix/plomvix/internal/engine/sql/kv"
)

const (
    BasicVersion uint64 = 0 // Hardcoded MVCC version for Basic tier
)

Public API
package heap

// Column defines a single column in a table schema.
type Column struct {
    Name string
    Kind key.Kind // Must be KindUint64, KindInt64, KindString, or KindBytes
}

// Schema defines the structure of a table.
type Schema struct {
    TableID   uint64
    Columns   []Column
    PKIndices []int // Indices into Columns that form the Primary Key. Must not be empty.
}

// Heap manages table access over a KVStore.
type Heap struct { /* unexported */ }

func New(store kv.KVStore) *Heap

// Table provides relational operations for a specific schema.
type Table interface {
    // Insert adds a row. Returns ErrDuplicateKey if PK exists.
    Insert(ctx context.Context, values []any) error

    // Get retrieves a row by its Primary Key. Returns ErrKeyNotFound if missing.
    Get(ctx context.Context, pkValues []any) ([]any, error)

    // Delete removes a row by PK. No-op if missing.
    Delete(ctx context.Context, pkValues []any) error

    // Scan returns a buffered iterator over all rows in the table.
    Scan(ctx context.Context) (Rows, error)
}

// Rows is a buffered iterator facade for scan results.
type Rows interface {
    Next() bool
    Values() []any // Returns a DEEP COPY of the current row's values.
    Err() error
    Close() error
}

var (
    ErrInvalidSchema         = errors.New("heap: invalid schema definition")
    ErrColumnCountMismatch   = errors.New("heap: value count does not match schema")
    ErrTypeMismatch          = errors.New("heap: value type does not match column kind")
    ErrNullNotSupported      = errors.New("heap: NULL values are not supported in Basic tier")
    ErrDuplicateKey          = errors.New("heap: primary key violation")
    ErrKeyNotFound           = errors.New("heap: row not found")
)

// OpenTable validates the schema and returns a Table interface for operations.
func (h *Heap) OpenTable(schema Schema) (Table, error)

Tasks (do in order, one at a time)

Task 1 — Package skeleton, Schema validation, and API
Create `internal/engine/sql/heap/heap.go`. Define all types, interfaces, and errors.
Implement `New(store kv.KVStore) *Heap`.
Implement `OpenTable(schema Schema) (Table, error)`:
- Validate `len(schema.Columns) > 0`.
- Validate `len(schema.PKIndices) > 0`.
- Validate all `PKIndices` are within bounds of `Columns`.
- Validate no duplicate column names.
- Validate all `Column.Kind` values are supported (`KindUint64`, `KindInt64`, `KindString`, `KindBytes`). Reject unsupported kinds with `ErrInvalidSchema`.
- Return a concrete `*table` struct holding the `kv.KVStore` and a validated `Schema`.
Tests: Valid schema opens. Invalid schemas (empty columns, empty PK, out-of-bounds PK index, duplicate names, unsupported kinds) return `ErrInvalidSchema`.

Task 2 — Row Encoding/Decoding Helpers & Key Wrapping
Create `internal/engine/sql/heap/encode.go`.

Implement `anyToPrimitive(kind key.Kind, v any) (any, error)`:
- If `v == nil`, return `ErrNullNotSupported`.
- Switch on `kind` and `v`'s underlying type.
- If `[]byte`, return a COPY of the slice to prevent caller mutation.
- Return the primitive Go type (`uint64`, `int64`, `string`, `[]byte`).
- Return `ErrTypeMismatch` if types don't align.

Implement `anyToKeyValue(kind key.Kind, v any) (key.Value, error)`:
- If `v == nil`, return `ErrNullNotSupported`.
- KindUint64 accepts `uint64` and returns `key.Uint64(v)`.
- KindInt64 accepts `int64` and returns `key.Int64(v)`.
- KindString accepts `string` and returns `key.String(v)`.
- KindBytes accepts `[]byte`, copies it, and returns `key.Bytes(copy)`.
- Otherwise return `ErrTypeMismatch`.

Implement `encodeRowKeyFromRow(schema Schema, values []any) (key.Key, error)`:
- Extract PK values using `schema.PKIndices`.
- Call `encodeRowKeyFromPK`.

Implement `encodeRowKeyFromPK(schema Schema, pkValues []any) (key.Key, error)`:
- Validate `len(pkValues) == len(schema.PKIndices)`.
- Convert to `[]key.Value` using `anyToKeyValue`.
- Call `key.EncodeTableRowKey(schema.TableID, kvs, BasicVersion)`.
- Wrap the resulting `[]byte` into a `key.Key`. (Note: If the `key` package does not expose a raw-bytes-to-Key wrapper like `key.FromBytes`, add one to the `key` package as a prerequisite, since `KVStore` requires `key.Key` but wire-level encoding produces `[]byte`).

Implement `encodeRowValue(schema Schema, values []any) ([]byte, error)`:
- Validate `len(values) == len(schema.Columns)`.
- Extract primitives using `anyToPrimitive`.
- Call `key.EncodeStorageComposite(primitives...)`. (Note: `EncodeStorageComposite` expects `...any`, so passing the extracted primitive Go types directly is correct).
- Return `Key.Bytes()`.

Implement `decodeRowValue(schema Schema, data []byte) ([]any, error)`:
- Extract `[]key.Kind` from `schema.Columns`.
- Call `key.ParseStorageCompositeKey(data, kinds)`.
- Extract the underlying Go types from the returned `Key.Fields()` back into `[]any`.
- CRITICAL: Return fresh COPIES of any `[]byte` values.
Tests: Round-trip encode/decode for all supported types. Verify `nil` returns `ErrNullNotSupported`. Verify type mismatches return `ErrTypeMismatch`. Verify `[]byte` inputs are copied.

Task 3 — `Insert` Implementation
Implement `Insert` on `*table`.
1. Validate `len(values) == len(schema.Columns)`.
2. Encode row key (`encodeRowKeyFromRow`) and row value (`encodeRowValue`).
3. To enforce strict PK uniqueness: Call `kv.Get(ctx, rowKey)`.
4. Error Mapping: If `kv.Get` returns `kv.ErrKeyNotFound`, proceed with insert. If it returns any other error, propagate it. If it returns a value (found == true), return `ErrDuplicateKey`.
5. Call `kv.Set(ctx, rowKey, rowValue)`.
Tests: Insert valid row. Insert duplicate PK (expect `ErrDuplicateKey`). Insert with wrong column count. Insert with wrong types. Insert with `nil`.

Task 4 — `Get` and `Delete` Implementation
Implement `Get`:
1. Encode row key using `encodeRowKeyFromPK`.
2. Call `kv.Get(ctx, rowKey)`.
3. Error Mapping: If `kv.Get` returns `kv.ErrKeyNotFound`, return `heap.ErrKeyNotFound`. Propagate other errors.
4. Decode row value using `decodeRowValue`. Return `[]any`.

Implement `Delete`:
1. Encode row key using `encodeRowKeyFromPK`.
2. Call `kv.Delete(ctx, rowKey)`. (No-op if missing is standard KV behavior).
Tests: Get existing. Get missing (expect `heap.ErrKeyNotFound`). Delete existing. Delete missing.

Task 5 — `Scan` and `Rows` Buffered Iterator
Implement `Scan`:
1. Generate the table prefix using `key.TablePrefix(schema.TableID)`.
2. Calculate the prefix end bound using `key.PrefixEnd(prefix)`.
3. Wrap both `[]byte` bounds into `key.Key` (using the same wrapping mechanism as Task 2).
4. Call `entries, err := t.store.Scan(ctx, prefixKey, prefixEndKey)`.
5. Return a concrete `*rows` struct that holds the `[]kv.Entry` slice and an internal index.

Implement `Rows` interface (`Next`, `Values`, `Err`, `Close`):
- `Next()` advances the internal index. Returns false when exhausted.
- `Values()` decodes the current KV value using `decodeRowValue` and returns a DEEP COPY of the resulting `[]any`.
- `Close()` releases the underlying slice/memory.
Tests: Scan empty table. Scan populated table. Verify iterator order matches PK sort order. Verify `Values()` returns independent deep copies.

Task 6 — Concurrency & Race Testing
Add `go test -race ./internal/engine/sql/heap/...`.
Write test: 50 goroutines inserting unique rows concurrently.
Write test: Concurrent `Scan` and `Insert`. 
Expectation: Concurrent operations should not race or corrupt data. Do NOT assert snapshot isolation or non-blocking behavior, as the buffered `Scan` holds the KV read lock only until the slice is returned.

Task 7 — Edge Cases & Compliance
Table-driven suite:
- Max size strings/bytes (verify TOAST in KVStore handles it transparently).
- Empty string / empty byte slice handling.
- Schema with composite Primary Key (multiple PKIndices).
- Verify `Scan` correctly isolates tables (insert into Table A, insert into Table B, Scan Table A only returns Table A's rows).

Task 8 — Benchmarks
Pre-populate 10,000 rows.
Benchmark `Insert` (measures KV Set + Read-before-write overhead).
Benchmark `Get` (measures KV Get + decode overhead).
Benchmark full `Scan` (measures KV Scan + decode overhead).

Task 9 — docs/sql_engine_heap.md
Write documentation covering:
- How relational rows map to KV keys and values.
- The strict NOT NULL and PK uniqueness guarantees.
- The hardcoded MVCC version (Basic tier limitation).
- Honest "Buffered Iterator" contract for `Scan`.
- Explicit "What Enterprise will add" section: MVCC, NULL bitmaps, true streaming cursors, secondary indexes.
Add substring-check test in `internal/engine/sql/heap/docs_test.go`.

Completion criteria
All 9 tasks implemented and tested. `go test -race` passes. The `Table` interface correctly maps relational operations to the KVStore using `key.Key`. `Scan` returns a buffered `Rows` iterator. Strict PK uniqueness and NOT NULL constraints are enforced. KV errors are correctly mapped to Heap errors. Documentation exists and passes substring checks.