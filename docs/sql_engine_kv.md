# Plomvix SQL Engine: On-Disk KVStore (B+ Tree)

The kv package provides an on-disk, ordered key-value store built on the
hardened pager. It implements a page-based B+ Tree with multi-page atomicity
via Write-Ahead Log (WAL) transactions.

## Architecture

- Page 2 (MetaPageID): Permanently reserved meta page.
- Leaf pages: Store key-value pairs with NextLeafPtr for ordered scans.
- Internal pages: Store separator keys and child page pointers.

## Strict Limits

- Maximum key size: 63 bytes.
- Maximum value size: 16MB (Enterprise TOAST), 512 bytes inline.
- Max leaf keys per page: 7.
- Max internal keys per page: 56.

## Multi-page atomicity

All tree mutations are wrapped in pager.BeginTx/CommitTx.

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

## Enterprise Tier: TOAST, Check, Compact

### TOAST — Large Values

Values up to 16MB are supported via TOAST overflow pages. Values above 512 bytes
are stored as a 504-byte inline prefix plus a linked list of overflow pages.

### Check — Structural Integrity

Check walks the live B+ Tree and overflow chains, verifying sort order,
routing rules, pointer validity, and detecting cycles or double-references.
Leaked pages do not cause Check to fail.

### Compact — Shadow Paging Rebuild

Compact atomically rebuilds the entire B+ Tree via shadow paging. It streams
all entries, builds a new tree on fresh pages, atomically swaps the root,
and best-effort frees old pages. This reclaims space from deleted keys and
old overflow chains.

## Concurrency

sync.RWMutex: writes serialized, scans concurrent.

## API

type KVStore interface {
    Open(ctx context.Context) error
    Get(ctx context.Context, k key.Key) ([]byte, error)
    Set(ctx context.Context, k key.Key, v []byte) error
    Delete(ctx context.Context, k key.Key) error
    Scan(ctx context.Context, start, end key.Key) ([]Entry, error)
    Close(ctx context.Context) error
    Check(ctx context.Context) error
    Compact(ctx context.Context) error
}

## Format version 2

Uses Enterprise pager files (FormatVersion 2).
