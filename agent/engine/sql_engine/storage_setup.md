# storage_setup.md — Pager (Basic)

## Scope

This plan delivers a **page manager** (`pager` package): fixed-size page
allocation, read, write, and free, backed by a single on-disk file, with
per-page corruption detection and per-write fsync durability.

This plan does **NOT** deliver an on-disk KVStore, an index structure, or
any mapping from ordered byte keys to pages. The existing in-memory
`kv_store` (sql/key + sorted slice) is untouched by this plan and continues
to be the only KVStore implementation that exists after this plan lands.
The pager is a lower layer; a future plan builds an on-disk KVStore on top
of it. Do not add any `Get`/`Set`/`Scan`-shaped API to the pager — if a task
description starts trending that way, stop and flag it rather than
implementing it.

## Contract this tier honestly provides (read this before writing code)

- **Single-page writes are durable**: once `WritePage` returns nil error,
  the page's bytes have been fsync'd and will survive a process crash or
  power loss (assuming the underlying disk honors fsync, which is an OS/HW
  assumption, not something this code can verify).
- **Single-page writes are NOT guaranteed atomic against a torn write**: a
  crash mid-write(2) of a single page can leave that page's on-disk bytes
  in a state that is neither the old content nor the new content. This tier
  does not prevent that. It **detects** it: every page has a checksum
  written alongside it, and `ReadPage` returns `ErrPageCorrupt` (not corrupt
  bytes, not a panic) if the checksum does not match.
- **Multi-page operations are NOT crash-atomic in this tier.** If a logical
  operation spans more than one page (true of nothing in this plan, since
  the pager has no concept of "operation" above a single page write — but
  will be true of whatever is built on top of it later), a crash between
  page A's write and page B's write leaves both pages individually durable
  and individually checksum-valid, but the pair may be inconsistent with
  each other. Closing that gap requires a WAL and is explicitly out of
  scope for `storage_enterprise.md` to address, not this plan.
- **The header page (page 0) is the single point of failure for the whole
  file.** If page 0 is corrupt, the file cannot be opened. This plan does
  not implement header redundancy (e.g. a mirrored header page) — that is
  a candidate for `storage_enterprise.md`, not Basic.

## Constants

```go
package pager

const (
    PageSize       = 4096 // bytes; matches common OS page size
    HeaderPageID   = 0
    FormatVersion  = 1
    MagicNumber    = 0x506C6D76 // "Plmv" as uint32, big-endian
)
```

`PageSize` is a constant in this plan, not a configurable value. Making it
configurable means the file format must carry per-file page size and every
arithmetic site must use the carried value instead of the constant — real
complexity with no current consumer. If a future need arises, that is a
deliberate follow-up plan, not a default to add speculatively here.

## On-disk layout

The file is a flat sequence of fixed-size `PageSize`-byte pages, indexed
from 0. Page 0 is always the header page. All other pages are either
allocated (in use by something above the pager) or free (linked into the
free-list).

### Header page (page 0) binary layout

All multi-byte integers are big-endian (consistent with `sql/key`'s
existing convention, for the same reason: human-readable in a hex dump,
and avoids ever mixing two endianness conventions in the same codebase).

```
Offset  Size  Field
0       4     Magic number (must equal MagicNumber, else ErrNotAPagerFile)
4       4     Format version (must equal FormatVersion, else ErrUnsupportedVersion)
8       4     Page size this file was created with (must equal PageSize,
              else ErrPageSizeMismatch — detection only, no migration)
12      8     Page count: total number of pages in the file, including
              page 0 (uint64)
20      8     Free-list head: page ID of the first free page, or
              0xFFFFFFFFFFFFFFFF (sentinel "no free pages") (uint64)
28      4     Header checksum: CRC32 (IEEE) of bytes [0,28) of this page
              (uint32)
32      ...   Reserved, zero-filled, unused by this plan
```

The header checksum covers only bytes [0, 28) — i.e. it is computed BEFORE
being written into bytes [28, 32), and verified by recomputing over [0, 28)
and comparing against the stored value. Reserved bytes are excluded from
the checksum so future fields can be added in the reserved region without
invalidating old checksums (forward-compatibility is still bounded by
`FormatVersion`, but this avoids an unnecessary second compatibility
constraint on top of it).

### Data page binary layout

```
Offset  Size  Field
0       8     Reserved for future use; always zero-filled in this plan
8       4     Page checksum: CRC32 (IEEE) of bytes [12, PageSize) of this
              page (uint32)
12      PageSize-12  Page body (opaque to the pager; owned by the caller)
```

Free pages reinterpret the page body: when a page is free, its **decoded
body** (the output of `decodeDataPage` / input to `encodeDataPage` — i.e.
post-checksum-stripping, body-relative offsets, NOT raw page-file offsets)
holds the free-list pointer at body offset [0,8): the page ID (uint64) of
the next free page in the free-list, or the sentinel
`0xFFFFFFFFFFFFFFFF` if it is the last free page.

To state this unambiguously against the raw on-disk layout below: body
offset [0,8) corresponds to raw page-file bytes [12,20), since the body
begins at raw offset 12. Raw page-file bytes [0,8) (the "reserved" field
in the data page layout table) remain reserved and zero-filled always —
they are never repurposed for the free-list pointer. The free-list pointer
is deliberately placed inside the body, not the reserved header region,
specifically so it is covered by the page checksum at raw offset [8,12)
and therefore benefits from the same corruption detection as any other
body content; a free-list pointer stored in the unchecksummed reserved
region would be undetectably corruptible.

This is the standard "free list lives inside the freed pages themselves"
technique — it requires no separate free-list storage and trivially scales
to any number of free pages.

## Public API

**Mandatory I/O discipline (applies to every method below and to every
internal helper, with no exceptions):** all page I/O against the backing
`*os.File` MUST use `file.ReadAt(buf, offset)` and `file.WriteAt(buf,
offset)` with an explicitly computed offset. Do NOT use `file.Seek`
followed by `file.Read`/`file.Write` anywhere in this package. The reason
is concurrency safety: `*os.File` holds a single shared file-position
cursor, and `Seek` mutates that shared cursor as a side effect. If two
goroutines call pager methods concurrently — `ReadPage` and `WritePage`
on different pages, for instance, both legitimate under the pager's own
`sync.RWMutex` discipline if that mutex is a `RWMutex` and the two calls
land on its read side, or even just by accident if a future task gets the
locking wrong — a `Seek` from one call can land between another call's
`Seek` and its `Read`/`Write`, causing one or both calls to operate on the
wrong page's bytes with no error returned at all. `ReadAt`/`WriteAt` take
an explicit offset per call and do not touch any shared cursor, which
makes them safe regardless of how the surrounding mutex discipline is
implemented or whether it is implemented correctly — this is a strictly
stronger guarantee than "we also hold a mutex," not a substitute for one,
and both should be true.

```go
package pager

import (
    "context"
    "errors"
)

// Pager manages fixed-size pages in a single backing file.
type Pager interface {
    // Name identifies the pager for lifecycle/logging.
    Name() string

    // Open opens (or creates, if absent) the backing file at the configured
    // path, validates or initializes the header page, and prepares the
    // pager for use. Idempotent error if already open.
    Open(ctx context.Context) error

    // Close flushes and closes the backing file. Safe to call once after
    // Open.
    Close(ctx context.Context) error

    // AllocatePage returns the page ID of a newly allocated page, taken
    // from the free-list if non-empty, otherwise extending the file by one
    // page. The returned page's body is zero-filled. The allocation is
    // durable once AllocatePage returns nil error (header free-list head
    // and/or page count have been updated and fsync'd).
    AllocatePage(ctx context.Context) (pageID uint64, err error)

    // ReadPage returns a COPY of the body of the page identified by
    // pageID. Returns ErrPageCorrupt if the page's checksum does not match
    // its body. Returns ErrInvalidPageID if pageID is 0 (the header page is
    // not readable as a data page through this method) or >= page count.
    ReadPage(ctx context.Context, pageID uint64) (body []byte, err error)

    // WritePage writes body as the new content of the page identified by
    // pageID. len(body) must equal PageSize-12; ErrBodySizeMismatch
    // otherwise. The write is durable (fsync'd) once WritePage returns nil
    // error. Returns ErrInvalidPageID under the same conditions as
    // ReadPage.
    WritePage(ctx context.Context, pageID uint64, body []byte) error

    // FreePage returns pageID to the free-list, making it available for a
    // future AllocatePage call. The page's body is zero-filled before
    // being linked into the free-list. Freeing an already-free page is
    // ErrAlreadyFree (the pager tracks this in memory while open; see
    // Task 6). Freeing the header page (0) is ErrInvalidPageID.
    FreePage(ctx context.Context, pageID uint64) error

    // PageCount returns the current total number of pages in the file,
    // including the header page. Useful for diagnostics and tests.
    PageCount(ctx context.Context) (uint64, error)
}

// Sentinel errors.
var (
    ErrNotOpen             = errors.New("pager: not open")
    ErrAlreadyOpen         = errors.New("pager: already open")
    ErrClosed              = errors.New("pager: closed")
    ErrNotAPagerFile       = errors.New("pager: file is not a pager file (bad magic)")
    ErrUnsupportedVersion  = errors.New("pager: unsupported format version")
    ErrPageSizeMismatch    = errors.New("pager: file page size does not match compiled PageSize")
    ErrHeaderCorrupt       = errors.New("pager: header checksum mismatch")
    ErrPageCorrupt         = errors.New("pager: page checksum mismatch")
    ErrInvalidPageID       = errors.New("pager: invalid page ID")
    ErrBodySizeMismatch    = errors.New("pager: body size does not match page body size")
    ErrAlreadyFree         = errors.New("pager: page is already free")
    ErrFreeListCorrupt     = errors.New("pager: free-list is corrupt (cycle, out-of-range, or invalid pointer)")
)
```

Notes:

- `body []byte` for both `ReadPage` and `WritePage` is always exactly
  `PageSize - 12` bytes (the body region after the 12-byte page header).
  The pager does not interpret body contents at all (except for free
  pages' free-list pointer, which is the pager's own internal bookkeeping,
  not something callers ever see — `ReadPage` on a free page is itself
  invalid; track allocated-vs-free in memory per Task 6, and return
  `ErrInvalidPageID` if a caller tries to `ReadPage` or `WritePage` a page
  that is currently on the free-list).
- All page IDs are `uint64`. Page 0 is reserved for the header and is never
  returned by `AllocatePage` and never freeable.
- The three-state machine (NeverOpened / Open / Closed) from `kv_store`
  applies identically here: `ErrNotOpen` before Open or after a failed
  Open, `ErrClosed` after Close, every method must check state first.

## Tasks (do in order, one at a time)

### Task 1 — Package skeleton, constants, sentinel errors, three-state machine

Create `internal/storage/pager/pager.go` with the package declaration,
the constants block above, all sentinel errors, the `Pager` interface, and
a concrete type `filePager` holding: `path string`, `file *os.File`,
`state` (NeverOpened/Open/Closed, reuse or mirror the enum pattern from
`kv_store`), `mu sync.RWMutex`. Add a
constructor `New(path string) Pager`. Every method except `Open` must
acquire the appropriate lock and check `state`, returning `ErrNotOpen` or
`ErrClosed` as appropriate, before doing anything else — write a single
shared internal helper for this check to avoid eight copies of the same
three-line guard. No file I/O yet; `Open`/`Close` and all data methods can
return `errors.New("not implemented")` stubs for now. This task should
compile and have a trivial state-machine test (construct, call ReadPage
before Open -> ErrNotOpen, etc.) and nothing else.

### Task 2 — Header page encode/decode (pure functions, no file I/O)

Add `header.go` with a `pagerHeader` struct mirroring the header layout
table above, plus:
```go
func encodeHeader(h pagerHeader) []byte   // returns exactly PageSize bytes
func decodeHeader(data []byte) (pagerHeader, error)
```
`encodeHeader` writes magic, version, page size, page count, free-list
head, computes the CRC32 over bytes [0,28), writes it at offset 28, and
zero-fills the rest of the page (bytes [32, PageSize)). `decodeHeader`
must reject input where `len(data) != PageSize` with `ErrHeaderCorrupt`
(do not introduce a separate `ErrHeaderSizeMismatch` — wrong-length input
is just another way the header is unusable, and a caller handling header
problems only needs to branch on one error, not two; `ErrBodySizeMismatch`
is the wrong error here regardless, since this isn't a body), then
validate magic -> `ErrNotAPagerFile`, then validate
version -> `ErrUnsupportedVersion`, then validate page size ->
`ErrPageSizeMismatch`, then recompute and compare the checksum over [0,28)
-> `ErrHeaderCorrupt` on mismatch. Validation order matters and must be
exactly this order (magic before version before page-size before
checksum), because a file that isn't a pager file at all should fail with
`ErrNotAPagerFile`, not a confusing checksum error. Add unit tests: valid
round-trip, each rejection case individually (wrong magic, wrong version,
wrong page size, corrupted checksum byte), and a hand-written golden byte
vector for one valid header (write the expected bytes by hand from the
spec table above, not by calling encodeHeader — this is the same
methodology used for Feature 1's golden vectors, and for the same reason:
a computed expected value only compares the code to itself).

### Task 3 — Data page encode/decode (pure functions, no file I/O)

Add `page.go` with:
```go
func encodeDataPage(body []byte) ([]byte, error)  // body must be PageSize-12 bytes; returns PageSize bytes
func decodeDataPage(data []byte) (body []byte, err error) // data must be PageSize bytes
```
`encodeDataPage` rejects `len(body) != PageSize-12` with
`ErrBodySizeMismatch`, writes 8 zero bytes (reserved) at offset 0, computes
CRC32 over the body (bytes that will land at [12, PageSize)), writes the
checksum at offset 8, then the body at offset 12. `decodeDataPage` rejects
wrong-length input, recomputes the checksum over [12, PageSize) and
compares against the stored value at offset 8, returning `ErrPageCorrupt`
on mismatch, otherwise returns a COPY of bytes [12, PageSize) (caller
must never get a slice aliasing the pager's internal buffer). Unit tests:
round-trip, wrong body length, corrupted single byte in body -> detected,
corrupted checksum byte itself -> detected, returned body slice is
independent of input (mutate input after decode, assert output unaffected).

### Task 4 — Free-list pointer encode/decode within a page body

Add to `page.go`:
```go
func encodeFreeListPointer(nextPageID uint64) []byte // returns PageSize-12 zero-filled bytes with nextPageID at [0,8)
func decodeFreeListPointer(body []byte) (nextPageID uint64, err error) // body is PageSize-12 bytes
const freeListSentinel uint64 = 0xFFFFFFFFFFFFFFFF
```
These operate on the *body* (post `decodeDataPage`/pre `encodeDataPage`),
not on raw page bytes — keep the free-list pointer concept layered
strictly on top of Task 3's body-level encode/decode, never duplicating
checksum logic. Unit tests: round-trip a normal page ID, round-trip the
sentinel, reject body of wrong length.

### Task 5 — Open: create-if-absent, validate-if-present

Implement `Open` on `filePager`. If the file at `path` does not exist,
create it (`os.OpenFile` with `O_CREATE|O_RDWR`, permissions `0600`),
write an initial header page (page count = 1, free-list head = sentinel,
magic/version/page size from the constants), fsync, and transition to
Open. If the file exists, open it `O_RDWR`, read page 0, decode it via
`decodeHeader` (Task 2), and on any decode error, leave the pager in
NeverOpened state and return that error wrapped with context (do not
transition to Open on a failed validation). On success, cache the decoded
header's page count and free-list head in memory (under the pager's
mutex; these are read on every `AllocatePage`/`PageCount` call and must
never require re-reading page 0 from disk on the hot path) and transition
to Open. `Open` on an already-Open pager returns `ErrAlreadyOpen` without
touching the file. Tests: fresh path creates a valid file (reopen it
separately and decode the header to confirm), reopening an existing valid
file succeeds and reports the correct page count, reopening a file with
deliberately corrupted header bytes (each rejection case from Task 2)
correctly fails Open and leaves state NeverOpened (verify via
`PageCount` -> `ErrNotOpen`), calling Open twice returns `ErrAlreadyOpen`
on the second call without re-validating.

### Task 6 — In-memory free/allocated page tracking, ReadPage, WritePage

Add an in-memory `freeSet map[uint64]struct{}` to `filePager`, populated
during `Open` by walking the free-list starting from the header's
free-list head. The walk must be defensive, because it runs against
on-disk bytes that may be corrupt or have been hand-edited (e.g. by a
future bug elsewhere, or by the deliberately-corrupted test fixtures in
Task 10) — an unvalidated walk over a corrupt or cyclic free-list can loop
forever and hang `Open`, which is unacceptable. At each step of the walk:

- If the current page ID equals the sentinel (`0xFFFFFFFFFFFFFFFF`), stop;
  the walk is complete.
- If the current page ID is `0`, or `>= pageCount`, fail `Open` with
  `ErrFreeListCorrupt` (new sentinel error, add it to the errors block) —
  the free-list must never point at the header page or out of range.
- If the current page ID has already been seen earlier in this same walk
  (track seen IDs in a local set as you go, separate from `freeSet` until
  the walk completes successfully), fail `Open` with `ErrFreeListCorrupt`
  — this is cycle detection; a free-list that loops back on itself would
  otherwise hang the walk indefinitely.
- Otherwise, read the page's decoded body directly via a private internal
  helper `readBodyUnchecked(pageID uint64) ([]byte, error)` that computes
  the page's file offset (`int64(pageID) * PageSize`) and reads exactly
  `PageSize` raw bytes from disk via `file.ReadAt(buf, offset)` (NOT
  `Seek` followed by `Read` — see the I/O discipline rule in the Public
  API section; this matters here specifically because the free-list walk
  runs inside `Open`, and all page I/O throughout this package must follow
  the same `ReadAt`/`WriteAt` discipline uniformly, with no exceptions
  carved out for any one call site),
  decodes them via `decodeDataPage` (propagating `ErrPageCorrupt` if the
  checksum fails), and returns the resulting `PageSize-12`-byte body —
  the same shape `ReadPage` returns, but **without** the `freeSet`
  membership check that public `ReadPage` performs (unlike `ReadPage`,
  which rejects free pages — the free-list walk's entire job is to read
  free pages, so it must use a path that doesn't reject them). Introduce
  `readBodyUnchecked` and its write-side counterpart
  `writeBodyUnchecked(pageID uint64, body []byte) error` (which takes a
  `PageSize-12`-byte body, encodes it via `encodeDataPage`, writes the
  resulting `PageSize` raw bytes to disk, and fsyncs — again, the same
  shape `WritePage` performs, minus the `freeSet` check) here in Task 6,
  since this is the first task that needs them; Task 7
  (`AllocatePage`/`FreePage`) reuses both rather than redefining them.

  Naming and contract, stated explicitly because getting this wrong is a
  real correctness risk: despite "Unchecked" referring to the bypassed
  `freeSet` check, **both helpers operate on decoded body bytes
  (`PageSize-12` length), never on raw `PageSize`-length page bytes.**
  Passing a `PageSize`-length buffer to `writeBodyUnchecked`, or expecting
  a `PageSize`-length buffer back from `readBodyUnchecked`, is a bug — the
  raw-vs-body distinction matters exactly as much here as it does for the
  public `ReadPage`/`WritePage` methods, and these helpers exist to let
  free-list management call into the same encode/decode machinery without
  re-deriving it, not to provide an unchecksummed raw-byte escape hatch.

  Using `readBodyUnchecked`, if the returned error is `ErrPageCorrupt`,
  propagate `ErrPageCorrupt`
  from `Open` directly (do not mask it as `ErrFreeListCorrupt` — a
  checksum failure is a distinct, more specific diagnosis than "the
  free-list structure itself is malformed", and callers may want to
  distinguish "my disk has bitrot" from "my free-list pointers are
  nonsensical"). Otherwise decode the free-list pointer from the returned
  body via Task 4's `decodeFreeListPointer`, add the current page ID to both
  the walk's local seen-set and to `freeSet`, and continue the walk from
  the decoded next pointer.

On any failure during this walk, `Open` must fail (returning the
propagated error, wrapped with context) and the pager must remain in
NeverOpened state — exactly as Task 5 already requires for header
decode failures; the free-list walk is part of `Open`'s overall
validation, not a step that happens after `Open` has already "succeeded."

Document as the reason large numbers of free pages slow down Open that
this means a full free-list walk at startup; this is an accepted
Basic-tier tradeoff (an on-disk free-page count alone, without walking,
would be cheaper but is a candidate for `storage_enterprise.md`, not this
plan).

Implement `ReadPage`: validate state, validate `pageID != 0` and
`pageID < pageCount` -> `ErrInvalidPageID`, validate `pageID` is not in
`freeSet` -> `ErrInvalidPageID`, compute the page's file offset
(`int64(pageID) * PageSize`) and read exactly `PageSize` bytes via
`file.ReadAt(buf, offset)`, decode via
`decodeDataPage` (Task 3), return the body or propagate `ErrPageCorrupt`.

Implement `WritePage`: validate state, validate `pageID` the same way as
ReadPage (including the freeSet check — writing to a free page through
this method is also `ErrInvalidPageID`; allocation status changes only
through `AllocatePage`/`FreePage`), validate `len(body) ==
PageSize-12` -> `ErrBodySizeMismatch`, encode via `encodeDataPage`, compute
the page's offset
(`int64(pageID) * PageSize`) and write exactly `PageSize` bytes via
`file.WriteAt(buf, offset)`, fsync the file, return
nil. The fsync is unconditional on every `WritePage` call — no batching,
no delayed fsync, no write-combining in this tier; that is the entire
durability story for Basic and must not be optimized away even though
it is slow, because removing it without replacing it with something
equally durable (e.g. a WAL) would silently break the contract documented
at the top of this plan. Tests: read-after-write round trip, read of an
out-of-range page ID, read of page 0 via ReadPage rejected, read/write of
a page currently in `freeSet` rejected, write with wrong body length
rejected, write-then-close-then-reopen-then-read survives (this is the
actual crash-consistency-relevant test: it proves data really hit disk,
not just an in-process buffer).

In addition to the `ReadPage`/`WritePage` tests above, this task is also
where the free-list walk's defensive validation (described earlier in
this task) must be locked down with its own explicit tests — this logic
is the most dangerous new surface in this task (a bug here can hang
`Open` forever), so "implemented" is not sufficient without these tests
specifically:

- A file whose header free-list head is the sentinel opens cleanly with
  an empty `freeSet` (the trivial/no-free-pages case, as a baseline).
- A file deliberately constructed (via direct file manipulation, not
  through the public API) so the free-list head points at page `0` ->
  `Open` returns `ErrFreeListCorrupt`, and the pager remains in
  NeverOpened state afterward (verify via a subsequent call, e.g.
  `PageCount` -> `ErrNotOpen`).
- A file constructed so the free-list head points at a page ID
  `>= pageCount` -> `Open` returns `ErrFreeListCorrupt`, state remains
  NeverOpened.
- A file constructed with a free-list cycle (e.g. two free pages whose
  pointers point at each other, or a free page whose pointer points back
  at itself) -> `Open` returns `ErrFreeListCorrupt`, state remains
  NeverOpened, and critically the test must assert this completes
  promptly (use `t.Run` with a reasonable timeout, e.g. via a goroutine
  and `select` with a time.After, or simply rely on the test framework's
  overall timeout) rather than merely asserting the error — a regression
  that removes cycle detection should fail this test by hanging, not by
  returning the wrong error, and the test should make that failure mode
  visible rather than just timing out the whole test binary unhelpfully.
- A file constructed so a page reachable via the free-list has a
  corrupted checksum -> `Open` returns `ErrPageCorrupt` specifically (not
  `ErrFreeListCorrupt` — confirm the two are distinguishable via
  `errors.Is`), state remains NeverOpened.
- For every one of the four failure cases above, confirm the pager
  remains usable for a *fresh* `Open` attempt against a *different*,
  valid file (i.e. confirm the failure is properly scoped to that one
  `Pager` instance/file and hasn't left any process-global state
  corrupted) — this can be a single shared test helper invoked from each
  failure case rather than a fully separate test per case.

### Task 7 — AllocatePage, FreePage, PageCount

Implement `AllocatePage`: validate state. If the in-memory free-list head
is **not** the sentinel (i.e. there is at least one free page), allocation
must take **exactly that page ID** — the current free-list head — and no
other. `freeSet` is a membership-test structure only (used by
`ReadPage`/`WritePage` to reject access to free pages, and by `FreePage`
to reject double-frees); it must never be the source of which page ID
gets allocated, since map iteration order is unspecified and "pick any
key from freeSet" would make allocation non-deterministic and would
desync from the persisted free-list head stored in the header.

**Mutation ordering rule (applies to both branches below): compute
everything needed first, perform both required disk writes, and only
after both have durably succeeded, commit the in-memory changes
(`freeSet`, free-list head, page count) in a single uninterrupted step
right before returning.** Do not mutate `freeSet` or the in-memory
free-list head/page count at any point before the header write/fsync has
returned nil. If a disk write or fsync fails partway through, return that
error immediately and leave every in-memory field exactly as it was
before the call — the pager's in-memory state must always describe
reality as of the last successful durable write, never reality as of a
write that was merely attempted. This plan does not attempt to roll back
or repair a partial on-disk write (e.g. a data page that was written but
whose header update then failed) — that scenario is already covered by
the top-level contract's "multi-page operations are NOT crash-atomic"
statement, and is consistent with this plan deliberately keeping mutation
ordering simple rather than building partial-failure recovery into Basic.

Concretely, free-list-reuse branch: read the free-list head page's body
using `readBodyUnchecked` (the
internal helper introduced in Task 6 — reuse it here rather than
redefining it; the head
page is currently in `freeSet` and public `ReadPage` would reject it, so
this internal path is required), decode its free-list
pointer (Task 4) to determine what the new free-list head *would* become
(hold this in a local variable; do not write it to the in-memory field
yet), zero-fill the allocated page's body (a `PageSize-12`-byte
zero buffer), write it via
`writeBodyUnchecked` (also reused from Task 6) — if this write fails,
return the error now, before touching the header or any in-memory state.
Update and fsync the header page with the new free-list head value — if
this fails, return the error now; the data page write above already
succeeded and is durable, but since no in-memory state has been mutated
yet, the pager's in-memory view is still consistent with the *previous*
free-list head, which remains a true (if stale) description of what the
header itself will say on next read, so no inconsistency is introduced by
returning here. Only once the header write/fsync has returned nil:
remove the allocated page ID from `freeSet`, set the in-memory free-list
head to the previously-held local variable, and return the allocated page
ID (the one that was the head before this call) with a nil error — this
final block of in-memory updates plus the return should be the last thing
the method does, with no disk I/O after it.

Free-list-empty branch (in-memory free-list head **is** the sentinel):
determine the new page ID (current in-memory page count) and hold it in a
local variable, zero-fill its body, write it via `writeBodyUnchecked` —
return immediately on failure, before touching the header or any
in-memory state. Update and fsync the header page with the new page count
— return immediately on failure, for the same reason given above: no
in-memory state has changed yet, so returning here leaves the pager
consistent with what the header still durably says. Only once the header
write/fsync has returned nil: set the in-memory page count to (previous
count + 1), and return the new page ID with a nil error.

Do not require a deterministic injected header write/fsync failure test
in Basic: `filePager` owns a concrete `*os.File`, and this plan does not
introduce a test-only file interface or fault-injection layer, so such a
test cannot be implemented without changing the production type shape.
Instead, verifying this ordering rule is a code-review requirement (see
Completion criteria) rather than an automated test in this plan.
Fault injection for partial write/fsync failures belongs in
`storage_enterprise.md`, where a small file I/O abstraction can be
introduced deliberately for that purpose.

Implement `FreePage`: validate state, validate pageID is not 0 and not
already in `freeSet` -> `ErrAlreadyFree` if already free, `ErrInvalidPageID`
if it's 0 or out of range. Apply the same mutation ordering rule as
`AllocatePage` above: encode a free-list pointer (Task 4) with
`nextPageID` = current in-memory free-list head (held in a local
variable; do not touch the in-memory field yet), write it as the page's
body via `writeBodyUnchecked` — return immediately on failure, before
touching the header or any in-memory state. Update and fsync the header
with the new free-list head (pageID) — return immediately on failure, for
the same reason as `AllocatePage`: no in-memory state has changed, so the
pager's view stays consistent with what the header durably says. Only
once the header write/fsync has returned nil: add pageID to `freeSet` and
update the in-memory free-list head to pageID, as the last step before
returning nil.

Implement `PageCount`: validate state, return the in-memory page count
(no disk read).

Tests: allocate N pages, confirm distinct IDs and correct PageCount;
allocate, free, allocate again -> confirm the freed ID is reused (free-list
behaves as a stack, LIFO, which is fine -- this plan does not need FIFO);
free a page then call ReadPage/WritePage on it -> ErrInvalidPageID; free an
already-free page -> ErrAlreadyFree; free page 0 -> ErrInvalidPageID;
close and reopen after several allocate/free cycles, confirm PageCount and
free-list state survive (reopen, then AllocatePage, confirm it correctly
reuses or extends based on the persisted free-list).

### Task 8 — Close

Implement `Close`: validate state is Open (NeverOpened or already-Closed
-> respectively `ErrNotOpen`/`ErrClosed`, matching the contract note on
the interface that Close is safe to call once after Open, not from any
other state), fsync the file (defensive; every mutating call above already
fsyncs after itself, so this is a no-op in practice today, but document
why it's still here: it is cheap insurance against a future mutating
method that forgets to fsync internally, and it gives Close a single
well-defined place to extend if that ever happens), close the underlying
`*os.File`, transition to Closed. ctx is accepted for interface
consistency; Close may check `ctx.Err()` before starting but must not
skip closing once close begins (same rule already established for
`kv_store`'s Close). Tests: Close after Open succeeds and transitions
state; calling any data method after Close returns ErrClosed; calling
Close twice returns ErrClosed on the second call.

### Task 9 — Lifecycle Component wiring

Confirm `filePager` already satisfies the `Component` shape used
elsewhere (`Name()/Open(ctx)/Close(ctx)`) — it does, by construction of
Tasks 1/5/8. Add a small example test in the `pager` package that
constructs a `Pager` via `New` with a `t.TempDir()` path, opens it,
allocates a handful of pages, writes and reads them back, frees one,
closes, and then **constructs a second, separate `Pager` instance via a
fresh call to `New` with the same path** and calls `Open` on that new
instance to confirm persistence. Do NOT call any method on the original,
now-closed `Pager` instance after `Close` — a closed `Pager` is terminal
and any further method call on it must return `ErrClosed`, never
`ErrNotOpen` and never silently succeed; "reopening" in this codebase
always means a new `Pager` value pointed at the same on-disk path, never
resurrecting a closed instance. Do NOT modify
`cmd/plomvix/main.go` or runtime wiring in this plan; engine registration
is deferred to whatever plan assembles the on-disk KVStore on top of this
pager, the same boundary `feature2.md` drew around `kv_store`'s own
runtime wiring.

### Task 10 — Crash-consistency test: simulated torn write detection

Add a dedicated test that deliberately corrupts a single byte in a data
page's checksum region directly on disk (close the pager first, open the
raw file with `os.OpenFile`, seek to a known page's checksum byte, flip a
bit, close the raw file) then reopens the pager and calls `ReadPage` on
that page, asserting `ErrPageCorrupt` is returned — not a panic, not
silently-wrong bytes. Add a second variant doing the same to the header
page's checksum region, asserting `Open` fails with `ErrHeaderCorrupt` and
the pager remains in NeverOpened state afterward. These two tests are the
concrete proof of the "detection, not prevention" contract stated at the
top of this plan, and should be commented as such so a future reader
understands why they exist.

### Task 11 — Benchmarks

Add benchmarks for `AllocatePage`, `WritePage`, `ReadPage` at a fixed small
page count (e.g. 1000 pre-allocated pages) to establish a baseline. Do NOT
add benchmarks with growing/scaling page counts in this plan — size-scaling
benchmark design (1k/10k/100k) was the right call for `kv_store` because
the in-memory sorted-slice/Scan performance characteristics actually change
shape with size; the pager's per-page operations are O(1) regardless of
total page count (file seek + fixed-size read/write + fsync), so a
size-scaling suite here would not reveal anything a fixed-size benchmark
doesn't already show, and would only add maintenance cost. If a future
free-list walk during Open (Task 6) ever becomes a measured concern at
scale, that is the specific benchmark to add then, not now, and not here.

### Task 12 — docs/storage.md

Write `docs/storage.md` covering: the page format (header + data page
layout tables, reproduced from this plan), the durability contract
(single-page durable + checksummed, multi-page NOT crash-atomic, header
is single point of failure with no redundancy in Basic), the free-list
design, and an explicit "what storage_enterprise.md is expected to add"
section listing: WAL for multi-page atomicity, header redundancy, and
(if it becomes a real bottleneck per Task 11's note) an on-disk free-page
count to avoid the full free-list walk on Open. Add a substring-check test
asserting the doc file contains the key contract phrases verbatim (same
pattern used for `docs/runtime.md` in `runtime_signals.md`'s tasks 11–12),
so the doc can't silently drift from the code without a test failing.

## Completion criteria

All 12 tasks implemented and tested. `go test ./internal/storage/pager/...`
passes including the two crash-consistency detection tests (Task 10). No
`Get`/`Set`/`Scan`-shaped API exists anywhere in the `pager` package — it
allocates, reads, writes, and frees pages by ID only. `kv_store` package is
untouched. `docs/storage.md` exists and its substring-check test passes.
Confirm by code review that `AllocatePage` and `FreePage` do not mutate
in-memory metadata (`freeSet`, free-list head, page count) before the
required data-page write and header write/fsync have both succeeded.