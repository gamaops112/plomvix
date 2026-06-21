# Plomvix SQL Engine: Table Heap (Basic Row Storage)

The heap package provides relational table row storage on top of the on-disk
B+ Tree KVStore.

## Architecture

- Rows are stored as KVStore entries: key = encoded table row key, value = storage-composite-encoded row.
- Table row keys use the wire-level key encoding: [0x01][tableID][encoded PK columns][version].
- Row values use length-prefixed storage composite encoding for safe framing.

## Strict NOT NULL

The Basic tier does not support NULL values. All columns are implicitly NOT NULL.
Passing nil for any column returns ErrNullNotSupported.

## Primary Key Uniqueness

Insert performs a read-before-write to enforce strict PK uniqueness, returning
ErrDuplicateKey if the row already exists.

## Hardcoded MVCC Version

The Basic tier hardcodes the MVCC version to 0. Future enterprise tiers will
add row versioning for MVCC.

## Buffered Iterator

Scan returns a Rows iterator over a buffered KV scan result. The entire scan
result is loaded into memory. True streaming cursors are deferred to enterprise.

## API

type Table interface {
    Insert(ctx context.Context, values []any) error
    Get(ctx context.Context, pkValues []any) ([]any, error)
    Delete(ctx context.Context, pkValues []any) error
    Scan(ctx context.Context) (Rows, error)
}

## Enterprise Roadmap

- MVCC row versioning
- NULL bitmaps
- True streaming cursors (O(1) memory)
- Secondary indexes
