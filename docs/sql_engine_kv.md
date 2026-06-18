# Plomvix SQL Engine: On-Disk KVStore (B+ Tree)

The `kv` package (`internal/engine/sql/kv`) provides an on-disk, ordered
key-value store built on the hardened pager (`internal/storage/pager`). It
implements a page-based B+ Tree with multi-page atomicity via Write-Ahead
Log (WAL) transactions.

## Architecture

- **Page 2** (MetaPageID): Permanently reserved meta page storing the root
  page ID. Sentinel `0xFFFFFFFFFFFFFFFF` represents an empty tree.
- **Leaf pages**: Store key-value pairs with NextLeafPtr for ordered scans.
- **Internal pages**: Store separator keys and child page pointers.

## Strict Limits

- **Maximum key size**: 63 bytes.
- **Maximum value size**: 512 bytes.
- **Max leaf keys per page**: 7.
- **Max internal keys per page**: 56.

## Multi-page atomicity

All tree mutations are wrapped in `pager.BeginTx()`/`CommitTx()`.

### Known Trade-Off: Leaked Pages on Crash

Page allocation happens OUTSIDE the KV transaction. If a crash occurs after
allocation but before commit, pages are leaked (wasted space, no corruption).

## 3-Phase Split Algorithm

1. Phase 1: Traverse to leaf.
2. Phase 2: Pre-allocate pages OUTSIDE transaction.
3. Phase 3: Write node pages inside transaction, commit.

## Upper Bound Routing Rule

Find first index i where target < K[i]; follow C[i]. Else follow rightmost child.

## No Internal Separator Updates on Delete

Only leaf keys are removed. Internal separators are never updated.
The upper_bound rule guarantees correct routing.

## Concurrency

`sync.RWMutex`: writes serialized, scans concurrent.

## API

```go
type KVStore interface {
    Open(ctx context.Context) error
    Get(ctx context.Context, k key.Key) ([]byte, error)
    Set(ctx context.Context, k key.Key, v []byte) error
    Delete(ctx context.Context, k key.Key) error
    Scan(ctx context.Context, start, end key.Key) ([]Entry, error)
    Close(ctx context.Context) error
}
```

## Format version 2

Uses Enterprise pager files (FormatVersion 2).
