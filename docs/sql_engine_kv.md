# Plomvix SQL Engine KVStore

The KVStore is a durable, ordered `[]byte→[]byte` key/value store for the
Plomvix sql_engine. It is **key-format-agnostic** — it stores raw bytes and
does not interpret tableIDs, versions, or any Feature 1 structure.

## Interface

```go
type KVStore interface {
    Name() string
    Open(ctx context.Context) error
    Close(ctx context.Context) error
    Get(ctx context.Context, key []byte) (value []byte, found bool, err error)
    Set(ctx context.Context, key, value []byte) error
    Delete(ctx context.Context, key []byte) error
    Scan(ctx context.Context, start, end []byte, fn func(key, value []byte) error) error
    NewBatch() Batch
}

type Batch interface {
    Set(key, value []byte)
    Delete(key []byte)
    Commit(ctx context.Context) error
    Reset()
}
```

## Semantics

- **Get/Scan return copies** — callers may mutate returned slices freely.
- **Scan is ordered, half-open `[start, end)`** — `start==nil` means from the
  first key; `end==nil` means to the last key.
- **Empty key** is invalid and returns `ErrEmptyKey`.
- **Batch** accumulates Set/Delete ops in memory and applies them atomically
  via `Commit`. Validation (including empty-key rejection) happens at Commit.
  Double commit is a no-op. `Reset` discards uncommitted ops.
- **Store state machine**: NeverOpened → Open → Closed. Never-opened ops
  return `ErrNotOpen`; closed ops return `ErrClosed`; double open returns
  `ErrAlreadyOpen`; `Open` after `Close` returns `ErrClosed`.
- **Close transitions to Closed unconditionally** — even if `db.Close()`
  returns an error, the store is marked closed.
- **Context**: get/set/delete/scan/commit check `ctx.Err()` before starting a
  bbolt transaction. Scan also checks between rows.

## Basic Backend: bbolt

The Basic tier uses [bbolt](https://github.com/etcd-io/bbolt) (v1.4.3).
Keys are stored in a single fixed bucket `plomvix_sql`.

### Configuration

```toml
[sql_engine]
data_dir = "data/sql"
backend  = "bbolt"
```

- `data_dir` — directory for the bbolt database file (`sql.db`).
- `backend` — only `"bbolt"` is accepted in the Basic tier.

### Construction

```go
s := kv.NewBBolt("sql", filepath.Join(cfg.SQL.DataDir, "sql.db"))
```

## Composition with Feature 1

Feature 1 produces **ordered byte keys**. Feature 2 stores and **range-scans**
them in that exact order. Together they enable:
```go
EncodeTableRowKey(...)  // Feature 1 → ordered bytes
kv.Set(key, row)        // Feature 2 → durable storage
kv.Scan(TablePrefix(7), TablePrefix(8), fn) // ordered scan of one table
```

## Non-Goals

- No transaction policy (Begin/Commit/Rollback) — this is Feature 6
- No key parsing or structural awareness
- No MVCC version management
- No Pebble backend (enterprise tier)
- No reverse iteration (enterprise tier)
