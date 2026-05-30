# Plomvix — Sprint 3

> **Plomvix** is an Indian-built, open-source, unified observability and general-purpose database
> supporting logs, metrics, telemetry, key-value, and JSON data.
> Built in Go. Production grade. Resource friendly.

---

## Architecture Layers Overview

```
Layer 1  →  Project Skeleton + Config + Logging        ← Sprint 1 ✅
Layer 2  →  Auth System (JWT + API Key)                ← Sprint 2 ✅
Layer 3  →  Write Ahead Log (WAL)                      ← Sprint 3
Layer 4  →  Hot Tier (RocksDB)
Layer 5  →  Ingestion API + Schema Inference Engine
Layer 6  →  SQL Query Engine (Hot Tier)
Layer 7  →  Cold Tier (Parquet) + Tiering Policy
Layer 8  →  Multi-Format Parsers
Layer 9  →  Admin APIs + Swagger Docs
Layer 10 →  Polish + Testing + Documentation
```

---

## Sprint 3 Goal

> By the end of Sprint 3, Plomvix has a production-grade Write Ahead Log.
> Every write is recorded to the WAL before being acknowledged to the caller.
> On crash recovery, the WAL replays all unprocessed entries automatically.
> Segment rotation, checksum validation, and configurable flush thresholds
> are all working. The WAL is fully tested and ready to be consumed by
> the RocksDB hot tier in Sprint 4.

---

## WAL Design Decisions

### Why segment-based files

A single growing WAL file becomes difficult to manage — you cannot safely delete
processed entries from the middle of a file. Segment files solve this cleanly:
each segment is an append-only binary file. Once all entries in a segment have
been flushed to the hot tier (Sprint 4), the entire segment file is deleted.

**Segment naming:** `seg-000001.wal`, `seg-000002.wal`, ...
Zero-padded to 6 digits for lexicographic sort correctness.

**Segment file location:** `{data_dir}/wal/`

### Entry format

Each WAL entry is a fixed-header binary record:

```
┌─────────────────────────────────────────────────────────────┐
│  Magic   │ Seq ID  │ Timestamp │ DataType │ PayloadLen │ CRC32 │
│  4 bytes │ 8 bytes │  8 bytes  │  1 byte  │  4 bytes   │4 bytes│
├─────────────────────────────────────────────────────────────┤
│                    Payload (variable)                         │
└─────────────────────────────────────────────────────────────┘
```

- **Magic:** `0x504C4D58` ("PLMX") — identifies Plomvix WAL entries, catches file corruption
- **Seq ID:** monotonically increasing uint64 — global across all segments
- **Timestamp:** Unix nanoseconds int64
- **DataType:** uint8 — identifies the type of data (log=1, metric=2, json=3, kv=4)
- **PayloadLen:** uint32 — length of the payload bytes that follow
- **CRC32:** IEEE checksum of all header bytes + payload bytes — detects corruption
- **Payload:** raw JSON bytes of the record

**Header size:** 4 + 8 + 8 + 1 + 4 + 4 = **29 bytes**

### Segment rotation

A new segment is created when the current segment exceeds `wal_flush_threshold`
(default 64MB from config). New entries always go to the active segment.

### Recovery behaviour

On startup, before the HTTP server starts:
1. Scan `data/wal/` for all segment files in lexicographic order
2. For each segment: read all entries, validate CRC32
3. Corrupted entries are skipped and logged at WARN level — not fatal
4. Valid entries are returned to the caller for replay into RocksDB (Sprint 4)
5. In Sprint 3, recovery reads entries but has nowhere to replay them yet —
   recovery is implemented and tested, but the replay target (RocksDB) lands in Sprint 4

### Sequence ID tracking

The WAL maintains a monotonically increasing sequence ID across all segments.
On startup, the last sequence ID is recovered from the most recent segment,
and new entries continue from `lastSeqID + 1`.

---

&nbsp;

## Feature 1 — WAL Core Types and Constants

> Define the WAL entry format, data types, and the constants
> that all other WAL code depends on.

---

### Story 1.1 — Define WAL Entry Format and Data Types

**What:**
Create `internal/storage/wal/types.go` defining all WAL types, constants,
and the entry struct.

**Imports required for types.go:**
```go
import (
    "fmt"
    "strconv"
    "strings"
)
```

`SegmentFileName` uses `fmt.Sprintf`.
`ParseSegmentIndex` uses `strings.TrimSuffix` and `strconv.ParseUint`
to strip the `.wal` extension and prefix, then parse the numeric index.

**Data type constants:**
```go
// DataType identifies the kind of data stored in a WAL entry.
type DataType uint8

const (
    DataTypeLog    DataType = 1
    DataTypeMetric DataType = 2
    DataTypeJSON   DataType = 3
    DataTypeKV     DataType = 4
)

// Magic is the 4-byte identifier written at the start of every WAL entry.
// Value: "PLMX" in ASCII.
const Magic uint32 = 0x504C4D58

// HeaderSize is the fixed size in bytes of every WAL entry header.
// Layout: Magic(4) + SeqID(8) + Timestamp(8) + DataType(1) + PayloadLen(4) + CRC32(4) = 29
const HeaderSize = 29
```

**Entry struct:**
```go
// Entry represents a single WAL record.
type Entry struct {
    SeqID     uint64
    Timestamp int64    // Unix nanoseconds
    DataType  DataType
    Payload   []byte   // raw JSON bytes of the record
    CRC32     uint32   // checksum — computed over header fields + payload
}
```

**Segment file naming:**
```go
// SegmentFileName returns the filename for a segment with the given index.
// Example: SegmentFileName(1) → "seg-000001.wal"
func SegmentFileName(index uint64) string {
    return fmt.Sprintf("seg-%06d.wal", index)
}

// ParseSegmentIndex parses the segment index from a filename.
// Returns error if the filename is not a valid WAL segment filename.
func ParseSegmentIndex(filename string) (uint64, error)
```

**Acceptance Criteria:**
- All constants, types, and structs compile with no errors
- `SegmentFileName(1)` returns `"seg-000001.wal"`
- `SegmentFileName(999999)` returns `"seg-999999.wal"`
- `ParseSegmentIndex("seg-000001.wal")` returns `1, nil`
- `ParseSegmentIndex("notawal.txt")` returns `0, error`
- `go build ./internal/storage/wal/` compiles with no errors

---

### Story 1.2 — CRC32 Checksum Utilities

**What:**
Create `internal/storage/wal/checksum.go` with functions for computing
and verifying entry checksums.

**Implementation:**
```go
// ComputeCRC32 computes the CRC32 IEEE checksum for a WAL entry.
// The checksum is computed over:
//   - Magic (4 bytes, big-endian)
//   - SeqID (8 bytes, big-endian)
//   - Timestamp (8 bytes, big-endian)
//   - DataType (1 byte)
//   - PayloadLen (4 bytes, big-endian)
//   - Payload bytes
func ComputeCRC32(e *Entry) uint32

// VerifyCRC32 returns true if the entry's CRC32 matches a freshly computed checksum.
func VerifyCRC32(e *Entry) bool
```

**Byte order:** All multi-byte integers use **big-endian** encoding.

**Acceptance Criteria:**
- `ComputeCRC32` produces the same value for identical entries
- `ComputeCRC32` produces different values when any field differs
- `VerifyCRC32` returns true for a freshly created entry
- `VerifyCRC32` returns false if any byte of the payload is modified after checksum computation
- `go build ./internal/storage/wal/` compiles with no errors

---

&nbsp;

## Feature 2 — WAL Segment Writer

> The writer appends entries to the active segment file.
> When the segment exceeds the flush threshold, it rotates to a new segment.

---

### Story 2.1 — Implement the Segment Writer

**What:**
Create `internal/storage/wal/writer.go` that manages writing WAL entries
to segment files on disk.

**Writer struct:**
```go
// Writer manages appending WAL entries to segment files.
type Writer struct {
    dir           string        // data/wal/ directory
    maxSegmentSize int64        // from config: wal_flush_threshold
    currentFile    *os.File     // active segment file handle
    currentIndex   uint64       // current segment index
    currentSize    int64        // bytes written to current segment
    nextSeqID      uint64       // next sequence ID to assign
    mu             sync.Mutex   // protects all fields — goroutine safe
}
```

**Public API:**
```go
// NewWriter opens or creates the WAL writer in the given directory.
// Scans existing segments to recover the latest segment index and sequence ID.
// maxSegmentSize is the byte threshold for segment rotation.
func NewWriter(dir string, maxSegmentSize int64) (*Writer, error)

// Write appends a new entry to the WAL.
// Assigns a monotonically increasing SeqID.
// Computes and stores the CRC32 checksum.
// Syncs to disk (fsync) before returning.
// Thread safe.
func (w *Writer) Write(dataType DataType, payload []byte) (*Entry, error)

// Close flushes and closes the active segment file.
// Call during graceful shutdown.
func (w *Writer) Close() error

// CurrentSegmentIndex returns the index of the active segment.
func (w *Writer) CurrentSegmentIndex() uint64

// CurrentSize returns the number of bytes written to the current segment.
func (w *Writer) CurrentSize() int64
```

**Write sequence — implement in this exact order:**
```
1. Lock mutex: w.mu.Lock()
   Use defer w.mu.Unlock() immediately — this handles all return paths
   including error returns from write or fsync, preventing deadlock.
2. Assign SeqID = nextSeqID, increment nextSeqID
3. Set Timestamp = time.Now().UnixNano()
4. Build Entry{SeqID, Timestamp, DataType, Payload}
   Call entry.CRC32 = ComputeCRC32(&entry)
   ComputeCRC32 internally uses len(entry.Payload) as PayloadLen —
   no separate encoding step needed before checksum computation.
5. Encode entry to binary using EncodeEntry (see entry encoding below)
6. Write encoded bytes to currentFile — return error if write fails
7. Fsync currentFile — return error if fsync fails
8. Add len(encodedBytes) to currentSize
9. If currentSize >= maxSegmentSize → rotate segment
10. Return &entry, nil
   (defer from step 1 fires automatically — no explicit Unlock needed)
```

**Entry binary encoding:**
```
Write in this order using binary.BigEndian:
  Magic     → uint32 (4 bytes)
  SeqID     → uint64 (8 bytes)
  Timestamp → int64  (8 bytes)
  DataType  → uint8  (1 byte)
  PayloadLen→ uint32 (4 bytes) ← length of payload
  CRC32     → uint32 (4 bytes)
  Payload   → []byte (PayloadLen bytes)
```

**Segment rotation:**
```
1. Close currentFile
2. Increment currentIndex
3. Create new file: SegmentFileName(currentIndex) in dir
4. Reset currentSize = 0
5. Set currentFile to new file handle
```

**NewWriter initialization:**
```
1. Scan dir for files matching "seg-*.wal" pattern
2. If no segments found → start with index=1, seqID=1
3. If segments found:
   a. Sort by index (lexicographic sort on filename is correct due to zero-padding)
   b. Set currentIndex to the last segment's index
   c. Open the last segment file for append
   d. Read all entries in the last segment to find the highest SeqID
   e. If the last segment has zero entries (e.g. crash immediately after rotation):
      - Scan previous segments in reverse order to find the highest SeqID
      - If no entries found anywhere → nextSeqID = 1
   f. Set nextSeqID = highestSeqID + 1
   g. Set currentSize = file size of the last segment
```

**Acceptance Criteria:**
- `Write()` is goroutine safe — use `sync.Mutex`
- Every write is fsynced before returning — no buffering
- Segment rotation creates a new file when `currentSize >= maxSegmentSize`
- After rotation, writes go to the new segment
- `NewWriter` on an existing WAL directory correctly recovers segment index and sequence ID
- `Close()` is called during graceful shutdown
- `go build ./internal/storage/wal/` compiles with no errors

---

### Story 2.2 — Entry Binary Encoding Helper

**What:**
Create `internal/storage/wal/encoding.go` with helpers for encoding and
decoding entries to/from binary format. Used by both writer and reader.

**Public API:**
```go
// EncodeEntry serializes a WAL entry to its binary wire format.
// Returns the complete byte slice ready to write to disk.
func EncodeEntry(e *Entry) ([]byte, error)

// DecodeEntry reads one WAL entry from the given reader.
// Returns ErrCorruptEntry if the magic bytes are wrong or CRC32 fails.
// Returns io.EOF if there are no more entries.
func DecodeEntry(r io.Reader) (*Entry, error)
```

**Sentinel error:**
```go
// ErrCorruptEntry is returned when a WAL entry fails CRC32 validation
// or has an invalid magic number.
var ErrCorruptEntry = errors.New("corrupt WAL entry")
```

**DecodeEntry logic:**
```
1. Read 29 bytes (HeaderSize) into header buffer
2. If read returns io.EOF and 0 bytes read → return nil, io.EOF (clean end)
3. If read returns io.EOF with partial bytes → return ErrCorruptEntry
4. Parse fields from header using binary.BigEndian:
   - Check Magic == 0x504C4D58 → if not, return ErrCorruptEntry
   - Extract SeqID, Timestamp, DataType, PayloadLen, CRC32
5. Read PayloadLen bytes into payload buffer
6. Reconstruct entry with all fields
7. Verify CRC32 → if fails, return ErrCorruptEntry
8. Return entry, nil
```

**Acceptance Criteria:**
- `EncodeEntry` followed by `DecodeEntry` on the result returns an identical entry
- `DecodeEntry` returns `io.EOF` at the end of a valid stream
- `DecodeEntry` returns `ErrCorruptEntry` if magic bytes are wrong
- `DecodeEntry` returns `ErrCorruptEntry` if CRC32 validation fails
- `DecodeEntry` returns `ErrCorruptEntry` if the file is truncated mid-entry
- `go build ./internal/storage/wal/` compiles with no errors

---

&nbsp;

## Feature 3 — WAL Segment Reader

> The reader scans segment files and returns all valid entries.
> Used during startup recovery and by the hot tier flush process in Sprint 4.

---

### Story 3.1 — Implement the Segment Reader

**What:**
Create `internal/storage/wal/reader.go` that reads entries from WAL
segment files.

**Reader struct:**
```go
// Reader scans WAL segment files and returns entries in sequence order.
type Reader struct {
    dir      string   // data/wal/ directory
    segments []string // sorted segment filenames to read
}
```

**Public API:**
```go
// NewReader creates a reader that will scan all segment files in dir.
// Segments are read in ascending index order.
func NewReader(dir string) (*Reader, error)

// ReadAll reads all valid entries from all segments in order.
// Corrupted entries are skipped — a WARN log is emitted for each skipped entry.
// Returns the slice of all valid entries and any fatal error (e.g. cannot open file).
// A corrupted entry is NOT a fatal error — reading continues.
func (r *Reader) ReadAll() ([]*Entry, error)

// SegmentCount returns the number of segment files found.
func (r *Reader) SegmentCount() int
```

**ReadAll logic:**
```
result := []*Entry{}
for each segment file in ascending order:
    open file
    loop:
        entry, err := DecodeEntry(file)
        if err == io.EOF → break (clean end of this segment)
        if err == ErrCorruptEntry:
            log.Warn("corrupt WAL entry — stopping read of this segment",
                segment filename)
            break  // stop reading THIS segment, move to next segment
                   // cannot reliably find next header after corruption
        if err != nil (other IO error) → return nil, err
        append entry to result
    close file
return result, nil
```

**Note on corrupted entry recovery:** After a corrupt entry, the reader cannot
reliably know where the next valid header starts. The behaviour is therefore:
stop reading the current segment at the first corrupt entry and move to the next
segment. This is documented in a comment in the source code.

**Acceptance Criteria:**
- `ReadAll` returns entries sorted by `SeqID` ascending
- A file with zero entries returns an empty slice — not an error
- A corrupt entry causes a WARN log and stops reading that segment — does not crash
- `SegmentCount()` returns the correct number of segments found
- `go build ./internal/storage/wal/` compiles with no errors

---

&nbsp;

## Feature 4 — WAL Manager

> The Manager is the single public interface the rest of Plomvix uses
> to interact with the WAL. It owns the Writer, provides the recovery
> path, and exposes segment cleanup for use by Sprint 4's hot tier.

---

### Story 4.1 — Implement the WAL Manager

**What:**
Create `internal/storage/wal/manager.go` — the top-level WAL interface
used by `main.go` and eventually by the hot tier.

**Manager struct:**
```go
// Manager is the public interface for the WAL.
// All WAL operations go through the Manager.
type Manager struct {
    writer *Writer
    dir    string
    cfg    *config.Config
}
```

**Public API:**
```go
// Open initializes the WAL Manager.
// Creates the WAL directory if it does not exist.
// Opens the Writer for appending.
// Returns error if the directory cannot be created or the writer fails to open.
func Open(dir string, cfg *config.Config) (*Manager, error)

// Write appends a new entry to the WAL.
// Assigns SeqID, computes CRC32, fsyncs to disk.
// Thread safe.
func (m *Manager) Write(dataType DataType, payload []byte) (*Entry, error)

// Recover reads all valid entries from all segments.
// Called once on startup before the HTTP server starts.
// Corrupted entries are skipped and logged.
// Returns entries in ascending SeqID order.
func (m *Manager) Recover() ([]*Entry, error)

// DeleteSegment deletes a segment file by index.
// Called by Sprint 4's hot tier after all entries in the segment
// have been flushed to RocksDB.
func (m *Manager) DeleteSegment(index uint64) error

// Close flushes and closes the WAL writer.
// Call during graceful shutdown.
func (m *Manager) Close() error

// Stats returns current WAL statistics.
func (m *Manager) Stats() WALStats
```

**WALStats struct:**
```go
type WALStats struct {
    SegmentCount    int    // computed by scanning the WAL directory on each Stats() call
    ActiveSegment   uint64
    ActiveSizeBytes int64
    TotalEntries    int64  // tracked in memory, incremented on each Write
}
```

> **SegmentCount accuracy note:** `SegmentCount` is computed by scanning the WAL
> directory on every `Stats()` call — NOT tracked as an in-memory counter.
> This ensures the value is always accurate even after `DeleteSegment` calls.
> The scan is lightweight (directory listing only, no file reads).
> `TotalEntries` is the only in-memory counter — it resets to 0 on restart,
> which is acceptable since it measures "entries written this session."

**Acceptance Criteria:**
- `Open` creates the WAL directory if it does not exist using `utils.EnsureDir`
- `Write` delegates to the underlying `Writer`
- `Recover` delegates to a `NewReader` and returns all valid entries
- `DeleteSegment` constructs the correct filename and deletes it — returns error if file does not exist
- `Close` delegates to `Writer.Close()`
- `Stats` returns accurate current values
- `go build ./internal/storage/wal/` compiles with no errors

---

&nbsp;

## Feature 5 — Main Integration

> Wire the WAL Manager into the boot sequence and graceful shutdown.
> The WAL must be open before the HTTP server starts.

---

### Story 5.1 — Integrate WAL into main.go

**What:**
Update `cmd/plomvix/main.go` to open the WAL Manager in the boot sequence
and close it on shutdown.

**New boot sequence steps — insert AFTER `defer blacklist.Stop()` and BEFORE `srv := server.New(...)`:**

The Sprint 2 boot sequence ends with:
```go
blacklist := auth.NewBlacklist()
defer blacklist.Stop()
// ← INSERT WAL CODE HERE
srv := server.New(cfg, Version, store, blacklist)
```

**Import alias to avoid collision with Go's `log` package:**
```go
import walmanager "github.com/plomvix/plomvix/internal/storage/wal"
```

**Exact code for steps 12–13 — follow the explicit close pattern from Sprint 2:**
```go
wal, err := walmanager.Open(filepath.Join(cfg.Storage.DataDir, "wal"), cfg)
if err != nil {
    logger.Error("failed to open WAL", zap.Error(err))
    os.Exit(1)
}

entries, err := wal.Recover()
if err != nil {
    wal.Close()  // explicit — os.Exit(1) bypasses defers
    logger.Error("WAL recovery failed", zap.Error(err))
    os.Exit(1)
}
defer wal.Close()  // safe to register now — no more os.Exit after this point

logger.Info("WAL recovery complete",
    zap.Int("entries_recovered", len(entries)),
    zap.Int("segments_found", wal.Stats().SegmentCount),
)
// NOTE: In Sprint 3, recovered entries are logged but not replayed.
// Sprint 4 will pass the WAL manager to the hot tier for replay.
```

**Acceptance Criteria:**
- WAL opens before HTTP server starts
- Recovery runs on every startup — logs entry count and segment count
- `wal.Close()` called during graceful shutdown
- On fresh install — recovery returns 0 entries, logs cleanly
- `go build ./cmd/plomvix/` compiles with no errors

---

### Story 5.2 — WAL Admin Stats Endpoint

**What:**
Expose WAL statistics through the existing `GET /health` endpoint.
No new endpoint needed — add WAL stats to the health response.

**Updated health response:**
```json
{
  "status": "ok",
  "data": {
    "version": "0.1.0",
    "env": "development",
    "uptime_seconds": 3600,
    "pid": 12345,
    "go_version": "go1.22.0",
    "os_arch": "linux/amd64",
    "wal": {
      "segment_count": 2,
      "active_segment": 2,
      "active_size_bytes": 1048576,
      "total_entries": 1500
    }
  },
  "request_id": "uuid"
}
```

**What to change — three specific edits to `internal/server/server.go`:**

**Edit 1 — Add `wal` field to `Server` struct:**
```go
type Server struct {
    router     *chi.Mux
    cfg        *config.Config
    httpServer *http.Server
    startTime  time.Time
    version    string
    store      *auth.Store
    blacklist  *auth.Blacklist
    wal        *walmanager.Manager  // ← ADD
}
```

**Edit 2 — Update `New()` signature and assign the new field:**
```go
func New(cfg *config.Config, version string, store *auth.Store,
    blacklist *auth.Blacklist, wal *walmanager.Manager) *Server {
    s := &Server{
        // ... existing fields ...
        wal: wal,  // ← ADD
    }
    // ... rest unchanged ...
}
```

**Edit 3 — Update `handleHealth` to include WAL stats in response:**
```go
stats := s.wal.Stats()
utils.OK(w, r, map[string]interface{}{
    "version":        s.version,
    "env":            s.cfg.Env,
    "uptime_seconds": int64(time.Since(s.startTime).Seconds()),
    "pid":            os.Getpid(),
    "go_version":     utils.GetGoVersion(),
    "os_arch":        utils.GetOSArch(),
    "wal": map[string]interface{}{
        "segment_count":    stats.SegmentCount,
        "active_segment":   stats.ActiveSegment,
        "active_size_bytes": stats.ActiveSizeBytes,
        "total_entries":    stats.TotalEntries,
    },
})
```

**Import to add to server.go:**
```go
walmanager "github.com/plomvix/plomvix/internal/storage/wal"
```

**Acceptance Criteria:**
- `GET /health` response includes a `wal` object with current stats
- `segment_count` reflects actual number of WAL segment files on disk
- `active_size_bytes` reflects bytes written to the current segment
- Server compiles and all existing health check behaviour is unchanged

---

&nbsp;

## Feature 6 — Tests

> Every WAL component has unit and integration tests.
> Tests must be deterministic and leave no files behind.

---

### Story 6.1 — Unit Tests for Types and Checksum

**What:**
Create `internal/storage/wal/types_test.go`.

**Tests to implement:**
```
TestSegmentFileName:
  - SegmentFileName(1) == "seg-000001.wal"
  - SegmentFileName(999999) == "seg-999999.wal"
  - SegmentFileName(42) == "seg-000042.wal"

TestParseSegmentIndex:
  - ParseSegmentIndex("seg-000001.wal") returns 1, nil
  - ParseSegmentIndex("seg-000042.wal") returns 42, nil
  - ParseSegmentIndex("notawal.txt") returns 0, non-nil error
  - ParseSegmentIndex("") returns 0, non-nil error

TestCRC32:
  - ComputeCRC32 returns same value for identical entries
  - ComputeCRC32 returns different value if SeqID changes
  - ComputeCRC32 returns different value if Payload changes
  - VerifyCRC32 returns true for a freshly computed entry
  - VerifyCRC32 returns false after modifying entry.Payload
```

**Acceptance Criteria:**
- All tests pass with `go test -race ./internal/storage/wal/`
- No temp files created

---

### Story 6.2 — Unit Tests for Encoding

**What:**
Create `internal/storage/wal/encoding_test.go`.

**Test helper — define at the top of `encoding_test.go`, used by multiple tests:**
```go
func makeTestEntry() *Entry {
    e := &Entry{
        SeqID:     1,
        Timestamp: 1234567890,
        DataType:  DataTypeLog,
        Payload:   []byte(`{"msg":"test"}`),
    }
    e.CRC32 = ComputeCRC32(e)
    return e
}
```

**Tests to implement:**
```
TestEncodeDecodeRoundtrip:
  entry := &Entry{SeqID:1, Timestamp:1234, DataType:DataTypeLog, Payload:[]byte(`{"msg":"hello"}`)}
  entry.CRC32 = ComputeCRC32(entry)
  encoded, err := EncodeEntry(entry)
  - err must be nil
  - len(encoded) == HeaderSize + len(entry.Payload)
  decoded, err := DecodeEntry(bytes.NewReader(encoded))
  - err must be nil
  - decoded.SeqID == entry.SeqID
  - decoded.Timestamp == entry.Timestamp
  - decoded.DataType == entry.DataType
  - decoded.CRC32 == entry.CRC32
  - bytes.Equal(decoded.Payload, entry.Payload)

TestDecodeEOF:
  - DecodeEntry on empty reader returns nil, io.EOF

TestDecodeCorruptMagic:
  entry := makeTestEntry()
  encoded, _ := EncodeEntry(entry)
  encoded[0] = 0xFF  // corrupt magic byte
  - DecodeEntry returns ErrCorruptEntry

TestDecodeCorruptCRC:
  entry := makeTestEntry()
  encoded, _ := EncodeEntry(entry)
  encoded[len(encoded)-1] ^= 0xFF  // flip last byte of payload
  - DecodeEntry returns ErrCorruptEntry

TestDecodeTruncated:
  entry := makeTestEntry()
  encoded, _ := EncodeEntry(entry)
  truncated := encoded[:HeaderSize/2]  // truncate mid-header
  - DecodeEntry returns ErrCorruptEntry

TestDecodeMultipleEntries:
  Write 3 entries back-to-back into a bytes.Buffer
  - Read all 3 with DecodeEntry in a loop
  - All 3 entries decoded correctly
  - 4th read returns io.EOF
```

**Acceptance Criteria:**
- All tests pass with `go test -race ./internal/storage/wal/`
- No temp files or disk I/O needed — use `bytes.Reader` and `bytes.Buffer`

---

### Story 6.3 — Integration Tests for Writer and Reader

**What:**
Create `internal/storage/wal/wal_test.go` with end-to-end tests using real files.

**Test helper:**
```go
func newTestWALDir(t *testing.T) string {
    t.Helper()
    dir := t.TempDir()
    return filepath.Join(dir, "wal")
}
```

**Tests to implement:**
```
TestWriterBasic:
  dir := newTestWALDir(t)
  w, err := NewWriter(dir, 64*1024*1024)
  - err must be nil
  e1, err := w.Write(DataTypeLog, []byte(`{"level":"info"}`))
  - err must be nil, e1.SeqID == 1
  e2, err := w.Write(DataTypeMetric, []byte(`{"metric":"cpu"}`))
  - e2.SeqID == 2
  w.Close()

TestWriterAndReaderRoundtrip:
  dir := newTestWALDir(t)
  w, _ := NewWriter(dir, 64*1024*1024)
  Write 5 entries with different DataTypes
  w.Close()
  r, _ := NewReader(dir)
  entries, err := r.ReadAll()
  - err must be nil
  - len(entries) == 5
  - entries are in ascending SeqID order (1, 2, 3, 4, 5)
  - each entry CRC32 passes VerifyCRC32

TestSegmentRotation:
  dir := newTestWALDir(t)
  // Set tiny max segment size to force rotation
  w, _ := NewWriter(dir, 100)  // 100 bytes max
  Write enough entries to force at least 2 segment rotations
  w.Close()
  r, _ := NewReader(dir)
  - r.SegmentCount() >= 2
  entries, _ := r.ReadAll()
  - All entries recovered in correct SeqID order
  - No duplicates

TestRecoveryAfterReopen:
  dir := newTestWALDir(t)
  w, _ := NewWriter(dir, 64*1024*1024)
  Write 3 entries
  w.Close()
  // Reopen — simulates crash recovery
  w2, err := NewWriter(dir, 64*1024*1024)
  - err must be nil
  // Next SeqID must continue from 4, not 1
  e, _ := w2.Write(DataTypeLog, []byte(`{}`))
  - e.SeqID == 4
  w2.Close()

TestManagerRecovery:
  dir := newTestWALDir(t)
  cfg := testWALConfig(dir)
  m, _ := Open(dir, cfg)
  m.Write(DataTypeLog, []byte(`{"a":1}`))
  m.Write(DataTypeKV, []byte(`{"b":2}`))
  m.Close()
  // Reopen and recover
  m2, _ := Open(dir, cfg)
  entries, err := m2.Recover()
  - err must be nil
  - len(entries) == 2
  - entries[0].DataType == DataTypeLog
  - entries[1].DataType == DataTypeKV
  m2.Close()

TestDeleteSegment:
  dir := newTestWALDir(t)
  cfg := testWALConfig(dir)
  // Use tiny max size to force rotation so we get at least 2 segments.
  // Then delete segment 1 (not active) to avoid deleting an open file handle.
  smallCfg := &config.Config{Storage: config.StorageConfig{
      DataDir: dir, WALFlushThreshold: 100}}
  m, _ := Open(dir, smallCfg)
  // Write enough entries to trigger segment rotation
  for i := 0; i < 5; i++ {
      m.Write(DataTypeLog, []byte(`{"fill":"padding-data-to-force-rotation"}`))
  }
  // At this point active segment is >= 2
  // Segment 1 is complete and not active — safe to delete
  m.Close()
  m2, _ := Open(dir, smallCfg)
  err := m2.DeleteSegment(1)
  - err must be nil
  - filepath.Join(dir, SegmentFileName(1)) no longer exists
  m2.Close()
```

**Test config helper:**
```go
func testWALConfig(dataDir string) *config.Config {
    return &config.Config{
        Storage: config.StorageConfig{
            DataDir:           dataDir,
            WALFlushThreshold: 64 * 1024 * 1024,
        },
    }
}
```

**Acceptance Criteria:**
- All tests pass with `go test -race ./internal/storage/wal/`
- `t.TempDir()` used for all directories — auto-cleaned
- No test shares state with another test
- Tests run in under 5 seconds total

---

&nbsp;

## Feature 7 — API Documentation

> WAL exposes stats through the health endpoint.
> Document the updated health response for developers.

---

### Story 7.1 — Update docs/api/health.md

**What:**
Create `docs/api/health.md` documenting the health endpoint
with the new WAL stats field added in Sprint 3.

**File:** `docs/api/health.md`

**Sections:**
```
# Plomvix Health API

## GET /health

Auth: none (public endpoint)

### Response — 200 OK (all checks pass)
[Show full JSON example with wal block]

### Response — 503 Service Unavailable (checks failing)
[Show full JSON example with details array]

### WAL Stats Fields
| Field | Type | Description |
|---|---|---|
| segment_count | int | Number of WAL segment files on disk |
| active_segment | uint64 | Index of the currently active segment |
| active_size_bytes | int64 | Bytes written to the active segment |
| total_entries | int64 | Total entries written since server start |

### Example curl
curl http://localhost:8080/health
```

**Acceptance Criteria:**
- `docs/api/health.md` exists and renders on GitHub
- WAL stats fields are documented with types and descriptions
- Curl example is accurate

---

&nbsp;

## Sprint 3 — Definition of Done

Sprint 3 is complete when **all of the following are true:**

- [ ] `go mod tidy` runs with zero errors
- [ ] `go build ./...` compiles with zero errors and zero warnings
- [ ] `make test` passes with zero failures and race detector enabled
- [ ] `make vet` passes with zero issues
- [ ] WAL directory created automatically at `{data_dir}/wal/` on first boot
- [ ] `SegmentFileName` produces zero-padded filenames correctly
- [ ] WAL entries written with correct binary format (magic, seqID, timestamp, dataType, payloadLen, CRC32, payload)
- [ ] Every write is fsynced to disk before returning
- [ ] Segment rotation creates new file when active segment exceeds `wal_flush_threshold`
- [ ] New writer correctly recovers sequence ID from existing segments on reopen
- [ ] `ReadAll` returns entries in ascending SeqID order
- [ ] Corrupted entries are skipped with WARN log — not fatal
- [ ] Recovery on fresh install returns 0 entries and logs cleanly
- [ ] Recovery on existing WAL returns all valid entries
- [ ] `DeleteSegment` correctly removes segment file by index
- [ ] WAL stats visible in `GET /health` response under `wal` key
- [ ] `wal.Close()` called during graceful shutdown
- [ ] All unit tests for types, checksum, encoding pass
- [ ] All integration tests for writer, reader, manager pass
- [ ] `docs/api/health.md` created and documents WAL stats fields
- [ ] No temp files left behind by any test

---

&nbsp;

## Sprint 3 — Story Summary

| Feature | Story | Description |
|---|---|---|
| 1 — Core Types | 1.1 | WAL entry format, data types, constants |
| 1 — Core Types | 1.2 | CRC32 checksum utilities |
| 2 — Writer | 2.1 | Segment writer implementation |
| 2 — Writer | 2.2 | Entry binary encoding/decoding |
| 3 — Reader | 3.1 | Segment reader implementation |
| 4 — Manager | 4.1 | WAL Manager — top-level interface |
| 5 — Integration | 5.1 | Wire WAL into main.go boot sequence |
| 5 — Integration | 5.2 | WAL stats in health endpoint |
| 6 — Tests | 6.1 | Unit tests: types and checksum |
| 6 — Tests | 6.2 | Unit tests: encoding |
| 6 — Tests | 6.3 | Integration tests: writer, reader, manager |
| 7 — Docs | 7.1 | Update docs/api/health.md |
| **Total** | **12 stories** | |

---

&nbsp;

*Plomvix — Built in India. Built for the world.*