# Plomvix Storage: Pager (Enterprise Tier)

The `pager` package (`internal/storage/pager`) provides a **fixed-size page
manager** backed by a single on-disk file and a Write-Ahead Log (WAL). It is
the lowest storage layer in Plomvix and handles page allocation, read, write,
and free with per-page corruption detection, per-write fsync durability,
multi-page atomicity via WAL, and header redundancy.

## Format version

Format version 2 is the current Enterprise layout. Format version 1 files are
rejected with `ErrUnsupportedVersion`. In-place migration from v1 to v2 is
explicitly out of scope.

## Page Format

### Header Page (page 0 & page 1 — mirror)

Page 0 (primary) and page 1 (mirror) share the same layout. Writes follow a
strict order: page 1 first, fsync, page 0 second, fsync. If page 0 is corrupt,
the pager recovers from page 1 automatically on Open.

| Offset | Size | Field |
|--------|------|-------|
| 0 | 4 | Magic number (`0x506C6D76` = "Plmv") |
| 4 | 4 | Format version (must equal 2) |
| 8 | 4 | Page size (must equal compiled `PageSize`) |
| 12 | 8 | Page count (total pages including header pages 0 and 1) |
| 20 | 8 | Free-list head page ID (or `0xFFFFFFFFFFFFFFFF` sentinel) |
| 28 | 8 | Free-page count (uint64, exact count of pages in free-list) |
| 36 | 4 | CRC32 (IEEE) checksum of bytes [0, 36) |
| 40 | ... | Reserved, zero-filled |

The checksum at offset 36 covers bytes [0, 36) — it does NOT include itself.

All multi-byte integers are big-endian.

### Data Page

| Offset | Size | Field |
|--------|------|-------|
| 0 | 8 | Reserved, zero-filled |
| 8 | 4 | CRC32 (IEEE) checksum of bytes [12, PageSize) |
| 12 | PageSize-12 | Page body (opaque to the pager) |

### Free Page

Same layout as a data page, but the **body** (post-checksum-stripping) holds
the free-list pointer at body offset [0, 8): the next free page ID (uint64),
or `0xFFFFFFFFFFFFFFFF` sentinel if this is the last free page.

## WAL format

The WAL is a separate file (`<path>.wal`) — an append-only log of records.
Each record is framed as:

```
[8-byte TxnID][8-byte PageID][4-byte BodyLength][N-byte Body][4-byte CRC32]
```

- **TxnID**: Monotonically increasing uint64 per transaction.
- **PageID**: The page being written.
- **BodyLength**: Length of the body (0 for EOT markers).
- **Body**: The page body bytes (PageSize-12 bytes for data records; 0 bytes for EOT).
- **CRC32**: CRC32 (IEEE) of TxnID + PageID + BodyLength + Body.

### End-of-Transaction (EOT) marker

A transaction is terminated by:

```
[8-byte TxnID][8-byte 0xFFFFFFFFFFFFFFFF][4-byte 0][4-byte CRC32]
```

The EOT marker has BodyLength=0, no body bytes. Its CRC32 covers the 20 bytes
preceding it.

### WAL replay

On `Open`, if the WAL file contains data, the pager replays all completed
transactions (those with an EOT marker) and truncates the WAL. Incomplete
trailing records (from a crash mid-write) are silently ignored. Corrupted
complete records (CRC mismatch or invalid shape) cause `Open` to return
`ErrWALCorrupt` without truncating the WAL.

## Durability Contract

### Multi-page atomicity

Callers can group multiple page writes into a single transaction via
`BeginTx`/`CommitTx`. If a crash occurs before `CommitTx` completes, none of
the writes are visible on the next `Open`. If a crash occurs after `CommitTx`
returns nil, all writes are durable and consistent.

### Single-page writes are durable

Once `WritePage` returns a nil error (outside a transaction), the page's
bytes have been fsync'd and will survive a process crash or power loss.

### Single-page writes are NOT atomic against torn writes

A crash mid-write(2) of a single page can leave that page's on-disk bytes
in a state that is neither the old content nor the new content. This tier
**detects** it: every page has a CRC32 checksum, and `ReadPage` returns
`ErrPageCorrupt` if the checksum does not match.

### Header redundancy

The header page (page 0) is mirrored at page 1. If page 0 is corrupt, the
pager attempts to recover from page 1. Mirror writes follow a strict order:
page 1 first, then page 0.

### Header page is the single point of failure

If both page 0 and page 1 are corrupt, the file cannot be opened. This should
be extremely rare given the mirror and CRC32 protections.

## Free-List Design

Free pages are linked into a singly-linked list using the first 8 bytes of
each freed page's body as a "next" pointer. The free-list head and free-page
count are stored in the header page. On `Open`, the pager walks the entire
free-list to validate integrity (cycle detection, range checks, checksum
validation) and verifies the on-disk `FreePageCount` matches the walk result.
A mismatch returns `ErrFreeListCorrupt`.

## API

```go
type Pager interface {
    Name() string
    Open(ctx context.Context) error
    Close(ctx context.Context) error
    AllocatePage(ctx context.Context) (pageID uint64, err error)
    ReadPage(ctx context.Context, pageID uint64) (body []byte, err error)
    WritePage(ctx context.Context, pageID uint64, body []byte) error
    FreePage(ctx context.Context, pageID uint64) error
    PageCount(ctx context.Context) (uint64, error)
    BeginTx(ctx context.Context) error
    CommitTx(ctx context.Context) error
    RollbackTx(ctx context.Context) error
}
```

## Enterprise vs Basic

| Feature | Basic | Enterprise |
|---------|-------|------------|
| Format version | 1 | 2 |
| Single-page durability | Yes | Yes |
| CRC32 checksums | Yes | Yes |
| Multi-page atomicity | No | Yes (WAL) |
| Header redundancy | No | Yes (page 1 mirror) |
| On-disk free-page count | No | Yes |
| Fault injection testing | No | Yes |
| Transactional API | No | Yes |
