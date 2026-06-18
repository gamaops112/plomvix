storage_enterprise.md — Pager (Enterprise Hardening)
Scope
This plan hardens the `pager` package (delivered in `storage_setup.md`) for
production use. It closes the crash-consistency gaps left in Basic by
introducing a Write-Ahead Log (WAL) for multi-page atomicity, header
redundancy, an on-disk free-page count, and a file-I/O abstraction for
fault-injection testing.

This plan does NOT deliver an on-disk KVStore, an index structure, or
any mapping from ordered byte keys to pages. The pager remains a lower
layer; a future plan (`storage_kvstore_setup.md`) builds the KVStore on
top of this hardened pager.

Format version policy
This plan bumps FormatVersion from 1 (Basic) to 2 (Enterprise). The
Enterprise pager supports ONLY FormatVersion 2 files. Opening a
FormatVersion 1 file returns ErrUnsupportedVersion. In-place migration
from v1 to v2 is explicitly out of scope. Fresh Enterprise files start
with pageCount=2 (page 0 = primary header, page 1 = mirror header).
AllocatePage must never return page 0 or page 1; the first allocatable
data page is page 2.

Contract this tier provides (read this before writing code)
- Redo-only WAL: Inside a transaction, WritePage appends the new page
  body to the WAL and buffers it in memory. It does NOT write to the
  main data file until CommitTx. This makes RollbackTx trivial (discard
  buffer) and crash-before-commit safe (WAL records without an
  End-of-Transaction marker are ignored during replay).
- Multi-page atomicity: Callers can group multiple page writes into a
  single transaction (BeginTx/CommitTx). If a crash occurs before
  CommitTx completes, none of the writes are visible on the next Open.
  If a crash occurs after CommitTx returns nil, all writes are durable
  and consistent.
- Single-page writes outside a transaction: WritePage called without an
  active transaction behaves exactly as in Basic — direct write to the
  main file + fsync. No WAL involvement.
- Header redundancy: The header page (page 0) is mirrored at page 1.
  If page 0 is corrupt, the pager attempts to recover from page 1.
  Mirror writes follow a strict order: page 1 first, then page 0.
- Free-page count: The header stores an on-disk FreePageCount (uint64).
  Open reads this value immediately, though it still walks the free-list
  to validate integrity.
- Fault injection: All file I/O goes through an internal fileOps
  interface, allowing tests to simulate fsync failures, partial writes,
  and disk-full errors.
- WAL record integrity: Every WAL record carries a CRC32 checksum.
  Replay rejects corrupted complete records as fatal errors, and stops
  safely on incomplete trailing records.

Global I/O Rules (Mandatory, no exceptions)
1. ReadAt/WriteAt only: All page I/O MUST use file.ReadAt/file.WriteAt
   with explicit offsets. Seek/Read/Write is strictly forbidden.
2. Short-write check: Every call to WriteAt MUST check the returned
   byte count. If n != len(buf), the method MUST return io.ErrShortWrite
   (or a wrapped equivalent) immediately. This applies to main file
   writes, WAL appends, header mirror writes, and replay/checkpoint writes.
3. Fsync discipline: Every successful write sequence MUST be followed
   by an explicit Sync() call before returning success to the caller.

Constants & Layout Updates
package pager

const (
    // ... existing constants from storage_setup.md ...
    FormatVersion      = 2 // Bumped from 1 for Enterprise layout
    MirrorHeaderPageID = 1 // Page 1 is permanently reserved for the mirrored header
    FirstDataPageID    = 2 // First page available for AllocatePage
    
    // WAL specific constants
    walEOTPageID       = 0xFFFFFFFFFFFFFFFF // Sentinel PageID for End-of-Transaction marker
)

Header page (page 0 & page 1) binary layout — UPDATED for Enterprise
Offset  Size  Field
0       4     Magic number (must equal MagicNumber)
4       4     Format version (must equal 2)
8       4     Page size (must equal PageSize)
12      8     Page count (uint64, includes pages 0 and 1)
20      8     Free-list head (uint64, sentinel 0xFFFFFFFFFFFFFFFF = empty)
28      8     Free-page count (uint64, exact count of pages in free-list)
36      4     Header checksum: CRC32 (IEEE) of bytes [0, 36) (uint32)
40      ...   Reserved, zero-filled

The checksum at offset 36 covers bytes [0, 36) — it does NOT include
itself. This avoids the self-referential checksum problem. The checksum
is computed over bytes [0, 36), written into bytes [36, 40), and verified
by recomputing over [0, 36) and comparing against the stored value.

Data page layout: UNCHANGED from Basic.

WAL file binary layout
The WAL is a separate file (<path>.wal). It is an append-only log of
records. Each record is framed as:

[8-byte TxnID][8-byte PageID][4-byte BodyLength][N-byte Body][4-byte CRC32]

- TxnID: Monotonically increasing uint64 per transaction.
- PageID: The page being written.
- BodyLength: Length of the body.
- Body: The page body bytes (PageSize-12 bytes for data records; 0 bytes for EOT).
- CRC32: CRC32 (IEEE) of TxnID + PageID + BodyLength + Body (i.e. all
  bytes preceding the CRC32 field in this record).

A transaction is terminated by an End-of-Transaction (EOT) marker:

[8-byte TxnID][8-byte walEOTPageID][4-byte 0][4-byte CRC32]

The EOT marker has BodyLength=0, no body bytes, and its CRC32 covers
the 20 bytes preceding it (TxnID + PageID sentinel + BodyLength).

Replay rules (CRITICAL):
- Read records sequentially from the start of the WAL file.
- If fewer bytes remain than needed for a complete record header
  (20 bytes minimum), stop replay — this is an incomplete trailing
  record from a crash mid-write. Ignore it.
- If a complete record is present but its CRC32 does not match, Open
  MUST return ErrWALCorrupt and MUST NOT truncate the WAL. Do not
  silently discard corrupted complete records or later records.
- Group valid records by TxnID. Apply ONLY transactions that have a
  valid EOT marker. Discard all others.

Public API Additions
package pager

// Options configures Enterprise pager behavior.
type Options struct {
    WALPath             string // Path to WAL file. If empty, defaults to path + ".wal"
    CheckpointThreshold int64  // Trigger checkpoint when WAL exceeds this size (bytes).
                               // 0 = checkpoint on every CommitTx.
}

// Constructors
func New(path string) Pager {
    return NewWithOptions(path, Options{})
}

func NewWithOptions(path string, opts Options) Pager

// Extended Pager interface (adds transactional methods to Basic's Pager)
type Pager interface {
    // ... all existing methods from storage_setup.md ...

    // BeginTx starts a new transaction. All subsequent WritePage calls
    // are buffered in the WAL and in-memory until CommitTx or RollbackTx.
    // Returns ErrTxAlreadyActive if a transaction is already active.
    BeginTx(ctx context.Context) error

    // CommitTx writes the EOT marker to the WAL, fsyncs the WAL, applies
    // all buffered page writes to the main file, fsyncs the main file,
    // and clears the transaction state.
    // Returns ErrNoActiveTx if no transaction is active.
    CommitTx(ctx context.Context) error

    // RollbackTx discards the in-memory transaction buffer and resets
    // transaction state. WAL records already appended for this transaction
    // remain on disk but are ignored during replay because they lack an
    // EOT marker.
    // Returns ErrNoActiveTx if no transaction is active.
    RollbackTx(ctx context.Context) error
}

// Sentinel errors (add to existing block from storage_setup.md)
var (
    // ... existing errors ...
    ErrTxAlreadyActive = errors.New("pager: transaction already active")
    ErrNoActiveTx      = errors.New("pager: no active transaction")
    ErrTxUnsupportedOp = errors.New("pager: operation not supported inside transaction")
    ErrWALCorrupt      = errors.New("pager: WAL record checksum mismatch or malformed record")
)

Transaction state rules (mandatory, no exceptions)
- Only one active transaction is allowed per Pager instance at a time.
- BeginTx while a transaction is active returns ErrTxAlreadyActive.
- CommitTx without an active transaction returns ErrNoActiveTx.
- RollbackTx without an active transaction returns ErrNoActiveTx.
- AllocatePage and FreePage are NOT transactional in this plan. If called
  while a transaction is active, they return ErrTxUnsupportedOp. This
  avoids the complexity of transactional page allocation/freeing (which
  requires transactional free-list updates) and can be added in a future
  tier if needed.
- WritePage outside a transaction: behaves exactly as Basic (direct write
  + fsync to main file, no WAL).
- WritePage inside a transaction: appends to WAL (with fsync), buffers
  the write in memory. Does NOT write to the main file.
- ReadPage inside a transaction: reads from the main file as usual. If
  the requested page has a pending write in the current transaction's
  buffer, return the buffered version instead (read-your-own-writes
  within a transaction).

Internal file I/O abstraction (for fault injection)
type fileOps interface {
    ReadAt(p []byte, off int64) (n int, err error)
    WriteAt(p []byte, off int64) (n int, err error)
    Sync() error
    Close() error
    Stat() (os.FileInfo, error)
    Truncate(size int64) error
}

Tasks (do in order, one at a time)

Task 1 — File I/O abstraction (fileOps) & Fault Injection Helper
Refactor filePager to use an internal fileOps interface instead of
*os.File directly. Create a realFileOps struct that wraps *os.File and
implements fileOps. Add an `openErr error` field to filePager to support
deferred error reporting.

Add an internal constructor to allow dependency injection for tests:
func newPager(path string, opts Options, mainFileOps, walFileOps fileOps) (*filePager, error)
- If mainFileOps is nil, open the real main file (O_CREATE|O_RDWR, 0600).
  If this fails, return nil, err.
- If walFileOps is nil, resolve WAL path (opts.WALPath or path+".wal") and
  open the real WAL file (O_CREATE|O_RDWR, 0600). If this fails, return nil, err.
- Store the handles in p.file and p.walFile respectively.
- Return the initialized *filePager and nil error.

Update public constructors to use deferred errors (preserving Basic's `New(path) Pager` signature):
func NewWithOptions(path string, opts Options) Pager {
    p, err := newPager(path, opts, nil, nil)
    if err != nil {
        return &filePager{openErr: err}
    }
    return p
}
func New(path string) Pager { return NewWithOptions(path, Options{}) }

In the test file (e.g., pager_test.go or fault_test.go), create a
faultInjectingFileOps struct that wraps realFileOps. It must accept
configuration to:
- Fail on the Nth call to Sync() or WriteAt() with a specific error.
- Return a short write (n < len(buf), err == nil) on the Nth call to WriteAt().
Tests: Confirm all existing Basic tests still pass unchanged. Confirm
faultInjectingFileOps correctly delegates until the configured failure
threshold is reached.

Task 2 — Header layout update, checksum fix, and mirror
Update encodeHeader/decodeHeader to match the new Enterprise layout:
FreePageCount at offset 28 (8 bytes), checksum at offset 36 covering
bytes [0, 36). Bump FormatVersion to 2.

Implement header mirroring: whenever the header is updated, write using
this strict order:
  1. Write the header bytes to page 1 (MirrorHeaderPageID) via WriteAt.
     Check n == len(buf); return io.ErrShortWrite if false.
  2. Fsync the main file.
  3. Write the same header bytes to page 0 (HeaderPageID) via WriteAt.
     Check n == len(buf); return io.ErrShortWrite if false.
  4. Fsync the main file.
Return nil only after both writes and both fsyncs succeed. If either
write or fsync fails, return the error immediately and do NOT update
in-memory metadata (page count, free-list head, free-page count).

On Open: decode page 0 first. If page 0 fails validation (bad magic,
bad version, bad checksum), attempt page 1. If page 1 is valid, copy
its bytes to page 0 (following the write order above), then proceed.
If both fail, return ErrHeaderCorrupt.

Note: Page 1 is permanently reserved. AllocatePage must never return
page 0 or page 1. The first allocatable page is page 2 (FirstDataPageID).
When creating a fresh file, initialize with pageCount=2 and write the
header to both page 0 and page 1.

Tests:
- Corrupt page 0 checksum, confirm recovery from page 1.
- Corrupt both pages, confirm ErrHeaderCorrupt.
- Confirm AllocatePage never returns 0 or 1.
- Fresh file has pageCount=2 and both headers are identical.
- Simulate page 0 write failure (using faultInjectingFileOps from Task 1):
  confirm page 1 was written but in-memory state is unchanged.

Task 3 — On-disk free-page count
Update AllocatePage and FreePage to maintain the in-memory freeCount
field and include it in every header write. Update Open to read this
value from the decoded header. The free-list walk (from Basic Task 6)
MUST still run to validate integrity (cycle detection, etc.). If the
walk finds a different count than the header's FreePageCount, return
ErrFreeListCorrupt.

Tests:
- Allocate/free pages, close, reopen, confirm FreePageCount matches
  actual free-list length.
- Corrupt the on-disk FreePageCount to mismatch the actual free-list
  length, confirm Open returns ErrFreeListCorrupt.

Task 4 — WAL record encode/decode with CRC32
Create wal.go. Implement:

const walEOTPageID uint64 = 0xFFFFFFFFFFFFFFFF

func encodeWALRecord(txnID, pageID uint64, body []byte) []byte
  - Returns the full framed record including trailing CRC32.
  - CRC32 covers TxnID + PageID + BodyLength + Body.

func decodeNextWALRecord(data []byte) (
    txnID uint64,
    pageID uint64,
    body []byte,
    consumed int,
    err error,
)
  - If len(data) < 20, return 0, 0, nil, 0, io.EOF (trailing incomplete).
  - Read BodyLength first (at offset 16).
  - Required full length = 8 + 8 + 4 + BodyLength + 4.
  - If len(data) < required length, return 0, 0, nil, 0, io.EOF (trailing incomplete).
  - Validates the record shape strictly:
    If pageID == walEOTPageID: BodyLength MUST be 0.
    Else (Normal data record): BodyLength MUST equal PageSize-12.
    Any other BodyLength/PageID combination is invalid and MUST return
    ErrWALCorrupt.
  - Recompute CRC32 over all bytes except the trailing 4, compares.
  - Returns ErrWALCorrupt on mismatch.
  - Return consumed = required length.

func encodeEOTMarker(txnID uint64) []byte
  - Returns the EOT marker with CRC32.

func isEOTMarker(pageID uint64, bodyLength uint32) bool
  - Returns true if pageID == walEOTPageID and bodyLength == 0.

Tests:
- Round-trip encode/decode for normal records.
- Round-trip encode/decode for EOT markers.
- Corrupted CRC32 -> ErrWALCorrupt.
- Truncated record (fewer bytes than expected) -> io.EOF.
- Wrong BodyLength for a normal record -> ErrWALCorrupt.
- Normal PageID with BodyLength=0 -> ErrWALCorrupt.

Task 5 — Transactional API with redo-only WAL
Add to filePager:
  inTxn      bool
  currentTxn uint64
  txnBuffer  map[uint64][]byte  // pageID -> body (PageSize-12 bytes)
  walSize    int64              // current WAL file size in bytes

Implement BeginTx:
  - If inTxn is true, return ErrTxAlreadyActive.
  - Increment a monotonic txnID counter.
  - Set inTxn=true, currentTxn=txnID, txnBuffer=make(map[uint64][]byte).
  - Return nil.

Implement WritePage modification:
  - If !inTxn: behave EXACTLY like Basic (encode data page, write to
    main file via fileOps.WriteAt, check n==len(buf), fsync main file
    via fileOps.Sync).
  - If inTxn:
    1. Validate pageID and body length as Basic does.
    2. Encode a WAL record via encodeWALRecord(currentTxn, pageID, body).
    3. Append the record to the WAL file via walFile.WriteAt(record, walSize).
       Check n == len(record); return io.ErrShortWrite if false.
    4. Fsync the WAL file via walFile.Sync().
    5. If either write or fsync fails, return the error (the WAL may have
       a partial record, but replay handles this via CRC validation).
    6. Store body (a COPY) in txnBuffer[pageID].
    7. Update walSize += len(record).
    8. Return nil. Do NOT write to the main file.

Implement ReadPage modification:
  - If inTxn and pageID exists in txnBuffer, return a COPY of
    txnBuffer[pageID] (read-your-own-writes).
  - Otherwise, proceed with Basic's ReadPage logic (read from main file).

Implement CommitTx:
  - If !inTxn, return ErrNoActiveTx.
  - Step 1: Encode EOT marker via encodeEOTMarker(currentTxn).
  - Step 2: Append EOT to WAL file via walFile.WriteAt. Check n==len(eot).
            Fsync WAL file.
  - CRITICAL ERROR-STATE & TIMING RULE:
    If Step 1 or Step 2 fails, keep inTxn=true and keep txnBuffer.
    The transaction is not durably committed; the caller may retry
    CommitTx or call RollbackTx.
    If Step 2 succeeds, the transaction is durably committed on disk.
    From this point onward, CommitTx MUST clear inTxn, txnBuffer, and
    currentTxn before returning, even if subsequent steps fail.
  - Step 3 (State Clear & Buffer Handoff):
    - Update walSize += len(eotRecord).
    - Copy txnBuffer to a local variable applyBuffer.
    - Immediately clear inTxn=false, txnBuffer=nil, currentTxn=0.
  - Step 4 (Apply to Main File):
    - For each (pageID, body) in applyBuffer:
      a. Encode data page via encodeDataPage(body).
      b. Write to main file at offset int64(pageID)*PageSize via WriteAt.
         Check n == len(buf); return io.ErrShortWrite if false.
  - Step 5: Fsync main file ONCE (not per-page).
  - Step 6: Checkpoint trigger:
      if opts.CheckpointThreshold == 0 || walSize >= opts.CheckpointThreshold {
          call checkpoint() (Task 7)
      }
  - Return nil on success, or the error from Step 4/5/6.

Implement RollbackTx:
  - If !inTxn, return ErrNoActiveTx.
  - Set inTxn=false, clear txnBuffer, reset currentTxn.
  - Do NOT truncate the WAL (the uncommitted records are harmless;
    replay ignores transactions without EOT markers). Truncation
    happens during the next checkpoint or Open.
  - Return nil.

Implement AllocatePage/FreePage guard:
  - At the top of both methods, after the state check, add:
    if p.inTxn { return ErrTxUnsupportedOp }

Tests:
- WritePage outside BeginTx -> data visible after reopen (Basic behavior).
- BeginTx, WritePage, RollbackTx, reopen -> changes NOT visible.
- BeginTx, WritePage, CommitTx, reopen -> changes visible.
- BeginTx, WritePage to page X, ReadPage page X -> returns buffered
  version (read-your-own-writes).
- BeginTx while already in tx -> ErrTxAlreadyActive.
- CommitTx without BeginTx -> ErrNoActiveTx.
- RollbackTx without BeginTx -> ErrNoActiveTx.
- AllocatePage during tx -> ErrTxUnsupportedOp.
- FreePage during tx -> ErrTxUnsupportedOp.

Task 6 — WAL Replay on Open
Update the Open() method: At the very beginning of Open(), before doing
any file I/O, check if p.openErr != nil. If so, return p.openErr immediately.

After validating the header (and recovering from mirror if needed):

  1. File handles are already initialized by newPager. Use the existing
     p.file and p.walFile.
  2. Initialize p.walSize from p.walFile.Stat().Size().
  3. If walSize > 0, perform WAL replay:
     a. Read the entire WAL file contents into a byte slice.
     b. Parse records sequentially using decodeNextWALRecord.
        - Advance offset += consumed after each record.
        - If decodeNextWALRecord returns io.EOF (trailing incomplete), stop parsing.
        - If a complete record fails CRC validation or shape validation,
          return ErrWALCorrupt immediately. Do NOT truncate the WAL.
     c. Group parsed records by TxnID. CRITICAL: Preserve the order in which
        TxnIDs first appeared in the WAL. Do not use a standard Go map for
        the final application order, as map iteration is randomized. Use an
        ordered structure (e.g., a slice of TxnIDs in parse order, or sort
        the collected TxnIDs ascending, which matches WAL append order).
     d. For each TxnID group in WAL order:
        - For each data record in the group, if the pageID was already
          written by an earlier record in the *same* transaction, the later
          record overwrites it (last write wins within a transaction).
        - Encode the final body for each page via encodeDataPage and write
          to the main file at the correct offset via WriteAt.
          Check n == len(buf); return io.ErrShortWrite if false.
        - After all pages for this transaction are written, fsync the
          main file ONCE.
        - CRITICAL FAILURE RULE: If any main-file write or fsync fails
          during replay, return the error immediately and DO NOT truncate
          the WAL. The WAL remains the source of truth for recovery.
     e. After ALL completed transactions are successfully applied and
        fsynced, truncate the WAL file to 0 bytes (via walFile.Truncate(0)).
     f. Reset walSize to 0.

Tests:
- Write a transaction (BeginTx, WritePage), simulate crash by closing
  without CommitTx (manually verify WAL has records but no EOT),
  reopen -> transaction NOT applied, WAL truncated.
- Write and Commit, simulate crash after WAL fsync but before main
  file write (use faultInjectingFileOps to fail the main file write
  in CommitTx step 4), reopen -> WAL replay recovers the data, WAL
  truncated afterward.
- Incomplete trailing WAL record -> replay ignores the trailing partial
  record, applies prior completed transactions, and truncates WAL.
- Corrupt a complete WAL record CRC -> Open returns ErrWALCorrupt and
  WAL is NOT truncated.
- Two committed transactions write the same page; after replay, the
  second transaction's value (later in WAL order) is visible.
- One transaction writes the same page twice; after replay, the last
  write in that transaction is visible.
- Simulate main-file write failure during replay (via fault injection) ->
  Open returns error, WAL is NOT truncated, data is recoverable on next Open.

Task 7 — Checkpointing
Implement checkpoint():
  1. Read all records from WAL (same parsing logic as Task 6 replay).
  2. Apply all completed transactions to the main file (same WriteAt +
     short-write check logic as Task 6).
  3. Fsync main file.
  4. CRITICAL FAILURE RULE: If any main-file write or fsync fails during
     checkpoint, return the error immediately and DO NOT truncate the WAL.
  5. Truncate WAL to 0 bytes.
  6. Reset walSize to 0.

Call checkpoint() in:
  - Close(), before closing the file handles.
  - CommitTx, if opts.CheckpointThreshold == 0 || walSize >= opts.CheckpointThreshold.

Tests:
- Fill WAL beyond threshold via multiple CommitTx calls, confirm WAL
  is truncated after the triggering CommitTx.
- Set CheckpointThreshold = 0, confirm WAL is truncated after every CommitTx.
- Call Close with non-empty WAL, confirm WAL is truncated.
- Call Close with empty WAL, confirm no error.

Task 8 — Fault injection tests
Using the faultInjectingFileOps created in Task 1, write tests that
simulate:
- WAL fsync fails during CommitTx (step 2) -> CommitTx returns error,
  inTxn remains true, main file unchanged, reopen shows no changes.
- Main file write fails during CommitTx (step 4) after WAL EOT was
  fsynced -> CommitTx returns error, inTxn is cleared, WAL has the
  committed data. Reopen -> WAL replay recovers the data.
- Header page 0 write fails during mirror update -> page 1 was written,
  in-memory state unchanged, reopen recovers from page 1.
- Header page 1 write fails during mirror update -> neither page
  updated, in-memory state unchanged, original headers intact.
- Short-write injection: Configure faultInjectingFileOps to return
  n < len(buf) on a WriteAt call during a normal WritePage or CommitTx.
  Confirm the pager method immediately returns io.ErrShortWrite and
  leaves the file/in-memory state consistent.

Task 9 — Benchmarks
Add benchmarks for:
- WritePage outside transaction (Basic baseline).
- CommitTx with 1, 10, 100 pages per transaction.
Compare the two to quantify WAL overhead. Use a fixed page count
(same reasoning as Basic Task 11: per-page operations are O(1),
size-scaling adds no insight).

Task 10 — docs/storage.md update
Update docs/storage.md to reflect Enterprise features:
- WAL format (record layout with CRC32, EOT marker).
- Transactional API (BeginTx/CommitTx/RollbackTx) with state rules.
- Header redundancy (page 0 + page 1 mirror, write order).
- Free-page count.
- Format version policy (v2 only, no v1 migration).
- "Enterprise vs Basic" section clearly delineating guarantees.

Add/Update a substring-check test in `docs/storage_test.go` (or equivalent)
asserting the doc file contains the following key contract phrases verbatim:
- "Multi-page atomicity"
- "WAL format"
- "Header redundancy"
- "Format version 2"
This ensures the doc cannot silently drift from the implemented Enterprise
contract without a test failing.

Completion criteria
All 10 tasks implemented and tested. go test ./internal/storage/pager/...
passes including fault injection and crash recovery tests. The Pager
interface supports BeginTx/CommitTx/RollbackTx with correct state machine
behavior. Header redundancy works (page 1 mirror with strict write order).
Free-page count is accurate. WAL replay recovers committed transactions
after simulated crashes and correctly discards uncommitted ones. WAL
records carry CRC32 checksums and replay stops safely on incomplete
trailing records, but returns ErrWALCorrupt on complete corrupted records
without truncating the WAL. FormatVersion 1 files are rejected with
ErrUnsupportedVersion. AllocatePage and FreePage return ErrTxUnsupportedOp
during transactions. Code review confirms: CommitTx writes EOT + fsyncs
WAL BEFORE writing any pages to the main file, and clears in-memory
transaction state immediately after WAL fsync succeeds, regardless of
main-file apply success. All WriteAt calls explicitly check for short writes.