# Plomvix Storage: Pager (Basic Tier)

The `pager` package (`internal/storage/pager`) provides a **fixed-size page
manager** backed by a single on-disk file. It is the lowest storage layer in
Plomvix and handles page allocation, read, write, and free with per-page
corruption detection and per-write fsync durability.

## Page Format

### Header Page (page 0)

| Offset | Size | Field |
|--------|------|-------|
| 0 | 4 | Magic number (`0x506C6D76` = "Plmv") |
| 4 | 4 | Format version (currently 1) |
| 8 | 4 | Page size (must equal compiled `PageSize`) |
| 12 | 8 | Page count (total pages including header) |
| 20 | 8 | Free-list head page ID (or `0xFFFFFFFFFFFFFFFF` sentinel) |
| 28 | 4 | CRC32 (IEEE) checksum of bytes [0, 28) |
| 32 | 4064 | Reserved, zero-filled |

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

## Durability Contract

### Single-page writes are durable

Once `WritePage` returns a nil error, the page's bytes have been fsync'd and
will survive a process crash or power loss (assuming the underlying disk
honours fsync).

### Single-page writes are NOT atomic against torn writes

A crash mid-write(2) of a single page can leave that page's on-disk bytes
in a state that is neither the old content nor the new content. This tier
**detects** it: every page has a CRC32 checksum, and `ReadPage` returns
`ErrPageCorrupt` if the checksum does not match.

### Multi-page operations are NOT crash-atomic

If a logical operation spans more than one page, a crash between page A's
write and page B's write leaves both pages individually durable and
checksum-valid, but the pair may be inconsistent. Closing this gap requires
a WAL (planned for the Enterprise tier).

### Header page is the single point of failure

If page 0 is corrupt, the file cannot be opened. Header redundancy (e.g. a
mirrored header page) is a candidate for the Enterprise tier.

## Free-List Design

Free pages are linked into a singly-linked list using the first 8 bytes of
each freed page's body as a "next" pointer. The free-list head is stored in
the header page. This technique requires no separate free-list storage and
trivially scales to any number of free pages.

On `Open`, the pager walks the entire free-list from the header head,
populating an in-memory set of free page IDs. The walk is defensive: it
detects cycles, out-of-range pointers, and corrupt checksums. A full
free-list walk means `Open` time is O(n) in the number of free pages, which
is an accepted Basic-tier tradeoff.

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
}
```

## Enterprise Tier Roadmap

The following are explicitly deferred to a future Enterprise hardening plan:

- **WAL** — multi-page atomicity (write-ahead logging)
- **Header redundancy** — mirrored header pages for recovery
- **On-disk free-page count** — avoid O(n) free-list walk on Open if it
  becomes a measured bottleneck
