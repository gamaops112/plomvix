# Plomvix — Sprint 3 Code Plan
### For: DeepSeek V4 Pro Coding Agent
### Language: Go 1.22 | Module: github.com/plomvix/plomvix

> Execute tasks in exact order. Each task is atomic — one file or one concern.
> Do not skip ahead. Each task depends on the previous.
> Every task has a Verify step — do not proceed until it passes.

---

## CONTEXT

You are continuing to build **Plomvix** — Sprints 1 and 2 are complete.
Sprint 3 adds the Write Ahead Log (WAL) — the durability backbone of Plomvix.
Every write is persisted to the WAL before being acknowledged to any caller.
On crash recovery, the WAL replays unprocessed entries automatically.

**Sprint 1 + 2 files already exist — do not modify except where explicitly told:**
```
cmd/plomvix/main.go
internal/config/config.go
internal/logger/logger.go
internal/server/server.go
internal/auth/          (all auth files)
pkg/utils/utils.go
pkg/utils/response.go
config.yaml
Makefile
```

**Sprint 3 produces new files in:**
```
internal/storage/wal/   ← all new WAL code
docs/api/               ← new health.md
```

**Sprint 3 modifies:**
```
cmd/plomvix/main.go       ← WAL init in boot sequence
internal/server/server.go ← WAL passed to server, health endpoint updated
```

---

## WAL BINARY FORMAT — read before writing any code

Every WAL entry on disk has this exact layout:

```
Offset  Size   Field
0       4      Magic: 0x504C4D58 ("PLMX") uint32 big-endian
4       8      SeqID: uint64 big-endian
12      8      Timestamp: int64 big-endian (Unix nanoseconds)
20      1      DataType: uint8 (1=log, 2=metric, 3=json, 4=kv)
21      4      PayloadLen: uint32 big-endian
25      4      CRC32: uint32 big-endian (IEEE checksum)
29      N      Payload: []byte of length PayloadLen
```

Total header = 29 bytes. CRC32 covers bytes 0–24 (entire header except CRC32 field) plus the payload bytes.

---

## TASK 01 — Create internal/storage/wal/ directory

**Action:**
```bash
mkdir -p internal/storage/wal
```

The `.gitkeep` placeholder from Sprint 1 already exists at
`internal/storage/wal/.gitkeep`. Remove it once the first `.go` file is created:
```bash
rm internal/storage/wal/.gitkeep
```

**Verify:** `ls internal/storage/wal/` shows the empty directory.

---

## TASK 02 — Create internal/storage/wal/types.go

**Action:** Create `internal/storage/wal/types.go`.

**Imports required:**
```go
import (
    "fmt"
    "strconv"
    "strings"
)
```

**Full file content:**
```go
package wal

import (
    "fmt"
    "strconv"
    "strings"
)

// DataType identifies the kind of data stored in a WAL entry.
type DataType uint8

const (
    DataTypeLog    DataType = 1
    DataTypeMetric DataType = 2
    DataTypeJSON   DataType = 3
    DataTypeKV     DataType = 4
)

// Magic is the 4-byte identifier at the start of every WAL entry.
// Value: "PLMX" in ASCII (0x504C4D58).
const Magic uint32 = 0x504C4D58

// HeaderSize is the fixed size in bytes of every WAL entry header.
// Layout: Magic(4) + SeqID(8) + Timestamp(8) + DataType(1) + PayloadLen(4) + CRC32(4) = 29
const HeaderSize = 29

// Entry represents a single WAL record.
type Entry struct {
    SeqID     uint64
    Timestamp int64    // Unix nanoseconds
    DataType  DataType
    Payload   []byte   // raw JSON bytes of the record
    CRC32     uint32   // checksum over header fields + payload
}

// SegmentFileName returns the filename for a WAL segment with the given index.
// Zero-pads to 6 digits for correct lexicographic sort order.
// Example: SegmentFileName(1) → "seg-000001.wal"
func SegmentFileName(index uint64) string {
    return fmt.Sprintf("seg-%06d.wal", index)
}

// ParseSegmentIndex parses the segment index from a WAL segment filename.
// Returns error if the filename does not match the pattern "seg-NNNNNN.wal".
func ParseSegmentIndex(filename string) (uint64, error) {
    if !strings.HasPrefix(filename, "seg-") || !strings.HasSuffix(filename, ".wal") {
        return 0, fmt.Errorf("not a WAL segment filename: %q", filename)
    }
    // Strip prefix "seg-" and suffix ".wal" to get the numeric part
    numeric := strings.TrimSuffix(strings.TrimPrefix(filename, "seg-"), ".wal")
    index, err := strconv.ParseUint(numeric, 10, 64)
    if err != nil {
        return 0, fmt.Errorf("invalid segment index in filename %q: %w", filename, err)
    }
    return index, nil
}
```

**Verify:** `go build ./internal/storage/wal/` compiles with no errors.

---

## TASK 03 — Create internal/storage/wal/checksum.go

**Action:** Create `internal/storage/wal/checksum.go`.

**Imports required:**
```go
import (
    "encoding/binary"
    "hash/crc32"
)
```

**Full file content:**
```go
package wal

import (
    "encoding/binary"
    "hash/crc32"
)

// ComputeCRC32 computes the CRC32 IEEE checksum for a WAL entry.
// The checksum covers (in order, big-endian):
//   Magic(4) + SeqID(8) + Timestamp(8) + DataType(1) + PayloadLen(4) + Payload(N)
// Note: the CRC32 field itself is NOT included in the checksum input.
func ComputeCRC32(e *Entry) uint32 {
    h := crc32.NewIEEE()

    var buf [4]byte

    // Magic
    binary.BigEndian.PutUint32(buf[:], Magic)
    h.Write(buf[:4])

    // SeqID
    var buf8 [8]byte
    binary.BigEndian.PutUint64(buf8[:], e.SeqID)
    h.Write(buf8[:])

    // Timestamp
    binary.BigEndian.PutUint64(buf8[:], uint64(e.Timestamp))
    h.Write(buf8[:])

    // DataType
    h.Write([]byte{byte(e.DataType)})

    // PayloadLen
    binary.BigEndian.PutUint32(buf[:], uint32(len(e.Payload)))
    h.Write(buf[:4])

    // Payload
    h.Write(e.Payload)

    return h.Sum32()
}

// VerifyCRC32 returns true if the entry's stored CRC32 matches a freshly computed checksum.
func VerifyCRC32(e *Entry) bool {
    return ComputeCRC32(e) == e.CRC32
}
```

**Verify:** `go build ./internal/storage/wal/` compiles with no errors.

---

## TASK 04 — Create internal/storage/wal/encoding.go

**Action:** Create `internal/storage/wal/encoding.go`.

**Imports required:**
```go
import (
    "bytes"
    "encoding/binary"
    "errors"
    "io"
)
```

**Full file content:**
```go
package wal

import (
    "bytes"
    "encoding/binary"
    "errors"
    "io"
)

// ErrCorruptEntry is returned when a WAL entry fails CRC32 validation
// or has an invalid magic number, or the file is truncated mid-entry.
var ErrCorruptEntry = errors.New("corrupt WAL entry")

// EncodeEntry serializes a WAL entry to its binary wire format.
// The entry's CRC32 field must already be set before calling EncodeEntry.
// Returns the complete byte slice ready to write to disk.
func EncodeEntry(e *Entry) ([]byte, error) {
    buf := new(bytes.Buffer)

    // Magic
    if err := binary.Write(buf, binary.BigEndian, Magic); err != nil {
        return nil, err
    }
    // SeqID
    if err := binary.Write(buf, binary.BigEndian, e.SeqID); err != nil {
        return nil, err
    }
    // Timestamp
    if err := binary.Write(buf, binary.BigEndian, e.Timestamp); err != nil {
        return nil, err
    }
    // DataType
    if err := binary.Write(buf, binary.BigEndian, e.DataType); err != nil {
        return nil, err
    }
    // PayloadLen
    if err := binary.Write(buf, binary.BigEndian, uint32(len(e.Payload))); err != nil {
        return nil, err
    }
    // CRC32
    if err := binary.Write(buf, binary.BigEndian, e.CRC32); err != nil {
        return nil, err
    }
    // Payload
    if _, err := buf.Write(e.Payload); err != nil {
        return nil, err
    }

    return buf.Bytes(), nil
}

// DecodeEntry reads one WAL entry from the given reader.
// Returns io.EOF if there are no more entries (clean end of stream).
// Returns ErrCorruptEntry if magic is wrong, CRC32 fails, or file is truncated.
func DecodeEntry(r io.Reader) (*Entry, error) {
    // Read the fixed-size header
    header := make([]byte, HeaderSize)
    n, err := io.ReadFull(r, header)
    if err != nil {
        if err == io.EOF && n == 0 {
            return nil, io.EOF // clean end of stream
        }
        // Partial read — truncated header
        return nil, ErrCorruptEntry
    }

    // Parse header fields using big-endian
    magic := binary.BigEndian.Uint32(header[0:4])
    if magic != Magic {
        return nil, ErrCorruptEntry
    }

    seqID := binary.BigEndian.Uint64(header[4:12])
    timestamp := int64(binary.BigEndian.Uint64(header[12:20]))
    dataType := DataType(header[20])
    payloadLen := binary.BigEndian.Uint32(header[21:25])
    storedCRC := binary.BigEndian.Uint32(header[25:29])

    // Read payload
    payload := make([]byte, payloadLen)
    if _, err := io.ReadFull(r, payload); err != nil {
        // Truncated payload
        return nil, ErrCorruptEntry
    }

    entry := &Entry{
        SeqID:     seqID,
        Timestamp: timestamp,
        DataType:  dataType,
        Payload:   payload,
        CRC32:     storedCRC,
    }

    // Verify checksum
    if !VerifyCRC32(entry) {
        return nil, ErrCorruptEntry
    }

    return entry, nil
}
```

**Verify:** `go build ./internal/storage/wal/` compiles with no errors.

---

## TASK 05 — Create internal/storage/wal/writer.go

**Action:** Create `internal/storage/wal/writer.go`.

**Imports required:**
```go
import (
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "sync"
    "time"
)
```

**Full file content:**
```go
package wal

import (
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "sync"
    "time"
)

// Writer manages appending WAL entries to segment files on disk.
// All exported methods are goroutine safe.
type Writer struct {
    dir            string
    maxSegmentSize int64
    currentFile    *os.File
    currentIndex   uint64
    currentSize    int64
    nextSeqID      uint64
    mu             sync.Mutex
}

// NewWriter opens (or creates) the WAL writer in the given directory.
// Scans existing segments to recover the current segment index and sequence ID.
// maxSegmentSize is the byte threshold that triggers segment rotation.
func NewWriter(dir string, maxSegmentSize int64) (*Writer, error) {
    if err := os.MkdirAll(dir, 0755); err != nil {
        return nil, fmt.Errorf("failed to create WAL directory: %w", err)
    }

    w := &Writer{
        dir:            dir,
        maxSegmentSize: maxSegmentSize,
    }

    if err := w.initialize(); err != nil {
        return nil, err
    }

    return w, nil
}

// initialize scans existing segments and sets currentIndex, nextSeqID, currentSize.
func (w *Writer) initialize() error {
    segments, err := listSegments(w.dir)
    if err != nil {
        return err
    }

    if len(segments) == 0 {
        // Fresh WAL — start from the beginning
        w.currentIndex = 1
        w.nextSeqID = 1
        return w.openSegment(w.currentIndex)
    }

    // Sort ascending by index
    sort.Strings(segments)

    w.currentIndex, err = ParseSegmentIndex(segments[len(segments)-1])
    if err != nil {
        return fmt.Errorf("failed to parse last segment filename: %w", err)
    }

    // Find the highest SeqID across segments, starting from the last
    highestSeqID := uint64(0)
    for i := len(segments) - 1; i >= 0; i-- {
        seqID, scanErr := highestSeqIDInSegment(filepath.Join(w.dir, segments[i]))
        if scanErr != nil {
            continue // skip unreadable segment
        }
        if seqID > 0 {
            highestSeqID = seqID
            break
        }
        // seqID == 0 means segment was empty — scan previous
    }
    w.nextSeqID = highestSeqID + 1

    // Open last segment for append and record its current size
    lastPath := filepath.Join(w.dir, SegmentFileName(w.currentIndex))
    f, err := os.OpenFile(lastPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
    if err != nil {
        return fmt.Errorf("failed to open last segment: %w", err)
    }
    fi, err := f.Stat()
    if err != nil {
        f.Close()
        return err
    }
    w.currentFile = f
    w.currentSize = fi.Size()

    return nil
}

// openSegment creates a new segment file at the given index.
func (w *Writer) openSegment(index uint64) error {
    path := filepath.Join(w.dir, SegmentFileName(index))
    f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
    if err != nil {
        return fmt.Errorf("failed to open segment %s: %w", path, err)
    }
    w.currentFile = f
    return nil
}

// rotate closes the current segment and opens a new one.
// Caller must hold the mutex.
func (w *Writer) rotate() error {
    if err := w.currentFile.Close(); err != nil {
        return fmt.Errorf("failed to close segment during rotation: %w", err)
    }
    w.currentIndex++
    w.currentSize = 0
    return w.openSegment(w.currentIndex)
}

// Write appends a new entry to the WAL.
// Assigns a monotonically increasing SeqID, computes CRC32, fsyncs to disk.
// Thread safe.
func (w *Writer) Write(dataType DataType, payload []byte) (*Entry, error) {
    w.mu.Lock()
    defer w.mu.Unlock()

    entry := Entry{
        SeqID:     w.nextSeqID,
        Timestamp: time.Now().UnixNano(),
        DataType:  dataType,
        Payload:   payload,
    }
    w.nextSeqID++
    entry.CRC32 = ComputeCRC32(&entry)

    encoded, err := EncodeEntry(&entry)
    if err != nil {
        return nil, fmt.Errorf("failed to encode WAL entry: %w", err)
    }

    if _, err := w.currentFile.Write(encoded); err != nil {
        return nil, fmt.Errorf("failed to write WAL entry: %w", err)
    }

    if err := w.currentFile.Sync(); err != nil {
        return nil, fmt.Errorf("failed to fsync WAL: %w", err)
    }

    w.currentSize += int64(len(encoded))

    if w.currentSize >= w.maxSegmentSize {
        if err := w.rotate(); err != nil {
            return nil, fmt.Errorf("failed to rotate WAL segment: %w", err)
        }
    }

    return &entry, nil
}

// Close flushes and closes the active segment file.
// Call during graceful shutdown.
func (w *Writer) Close() error {
    w.mu.Lock()
    defer w.mu.Unlock()
    if w.currentFile != nil {
        err := w.currentFile.Close()
        w.currentFile = nil // Prevent subsequent operations on closed file
        return err
    }
    return nil
}

// CurrentSegmentIndex returns the index of the currently active segment.
func (w *Writer) CurrentSegmentIndex() uint64 {
    w.mu.Lock()
    defer w.mu.Unlock()
    return w.currentIndex
}

// CurrentSize returns the number of bytes written to the current segment.
func (w *Writer) CurrentSize() int64 {
    w.mu.Lock()
    defer w.mu.Unlock()
    return w.currentSize
}

// listSegments returns all WAL segment filenames in the given directory.
func listSegments(dir string) ([]string, error) {
    entries, err := os.ReadDir(dir)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, nil
        }
        return nil, err
    }
    var segments []string
    for _, e := range entries {
        if !e.IsDir() && len(e.Name()) > 4 && e.Name()[len(e.Name())-4:] == ".wal" {
            if _, parseErr := ParseSegmentIndex(e.Name()); parseErr == nil {
                segments = append(segments, e.Name())
            }
        }
    }
    return segments, nil
}

// highestSeqIDInSegment reads a segment file and returns the highest SeqID found.
// Returns 0 if the segment is empty or unreadable.
func highestSeqIDInSegment(path string) (uint64, error) {
    f, err := os.Open(path)
    if err != nil {
        return 0, err
    }
    defer f.Close()

    var highest uint64
    for {
        entry, err := DecodeEntry(f)
        if err != nil {
            break // EOF or corrupt — stop
        }
        if entry.SeqID > highest {
            highest = entry.SeqID
        }
    }
    return highest, nil
}
```

**Verify:** `go build ./internal/storage/wal/` compiles with no errors.

---

## TASK 06 — Create internal/storage/wal/reader.go

**Action:** Create `internal/storage/wal/reader.go`.

**Imports required:**
```go
import (
    "fmt"
    "io"
    "os"
    "path/filepath"
    "sort"

    "go.uber.org/zap"

    "github.com/plomvix/plomvix/internal/logger"
)
```

**Full file content:**
```go
package wal

import (
    "fmt"
    "io"
    "os"
    "path/filepath"
    "sort"

    "go.uber.org/zap"

    "github.com/plomvix/plomvix/internal/logger"
)

// Reader scans WAL segment files and returns entries in ascending SeqID order.
type Reader struct {
    dir      string
    segments []string // sorted segment filenames (ascending by index)
}

// NewReader creates a Reader that will scan all segment files in dir.
// Returns error if the directory cannot be read.
// Returns an empty Reader (no error) if the directory is empty.
func NewReader(dir string) (*Reader, error) {
    segments, err := listSegments(dir)
    if err != nil {
        return nil, fmt.Errorf("failed to list WAL segments: %w", err)
    }
    sort.Strings(segments)
    return &Reader{dir: dir, segments: segments}, nil
}

// ReadAll reads all valid entries from all segments in ascending index order.
// On a corrupt entry: logs WARN and stops reading that segment — moves to next.
// After corruption, the reader cannot reliably locate the next valid header,
// so the remainder of the corrupt segment is abandoned.
// Returns a non-nil error only for fatal I/O failures (e.g. cannot open a file).
// A corrupt entry is NOT a fatal error.
func (r *Reader) ReadAll() ([]*Entry, error) {
    var result []*Entry

    for _, segName := range r.segments {
        path := filepath.Join(r.dir, segName)
        f, err := os.Open(path)
        if err != nil {
            return nil, fmt.Errorf("failed to open WAL segment %s: %w", segName, err)
        }

        for {
            entry, decErr := DecodeEntry(f)
            if decErr != nil {
                if decErr == io.EOF {
                    break
                }
                if decErr == ErrCorruptEntry {
                    logger.Warn("corrupt WAL entry — stopping read of this segment",
                        zap.String("segment", segName))
                    break // stop this segment, continue to next
                }
                f.Close()
                return nil, fmt.Errorf("I/O error reading segment %s: %w", segName, decErr)
            }
            result = append(result, entry)
        }

        f.Close()
    }

    return result, nil
}

// SegmentCount returns the number of segment files found during NewReader.
func (r *Reader) SegmentCount() int {
    return len(r.segments)
}
```

**Verify:** `go build ./internal/storage/wal/` compiles with no errors.

---

## TASK 07 — Create internal/storage/wal/manager.go

**Action:** Create `internal/storage/wal/manager.go`.

**Imports required:**
```go
import (
    "fmt"
    "os"
    "path/filepath"
    "sync/atomic"

    "github.com/plomvix/plomvix/internal/config"
    "github.com/plomvix/plomvix/pkg/utils"
)
```

**Full file content:**
```go
package wal

import (
    "fmt"
    "os"
    "path/filepath"
    "sync/atomic"

    "github.com/plomvix/plomvix/internal/config"
    "github.com/plomvix/plomvix/pkg/utils"
)

// WALStats holds current statistics about the WAL.
type WALStats struct {
    SegmentCount    int    // computed by scanning the directory — always accurate
    ActiveSegment   uint64
    ActiveSizeBytes int64
    TotalEntries    int64  // in-memory counter, resets on restart
}

// Manager is the public interface for all WAL operations.
// All operations are goroutine safe (delegated to Writer).
type Manager struct {
    writer       *Writer
    dir          string
    totalEntries atomic.Int64
}

// Open initializes the WAL Manager.
// Creates the WAL directory if it does not exist.
// Opens the Writer for appending.
func Open(dir string, cfg *config.Config) (*Manager, error) {
    if err := utils.EnsureDir(dir); err != nil {
        return nil, fmt.Errorf("failed to create WAL directory: %w", err)
    }

    writer, err := NewWriter(dir, cfg.Storage.WALFlushThreshold)
    if err != nil {
        return nil, fmt.Errorf("failed to open WAL writer: %w", err)
    }

    return &Manager{
        writer: writer,
        dir:    dir,
    }, nil
}

// Write appends a new entry to the WAL.
// Assigns SeqID, computes CRC32, fsyncs to disk. Thread safe.
func (m *Manager) Write(dataType DataType, payload []byte) (*Entry, error) {
    entry, err := m.writer.Write(dataType, payload)
    if err != nil {
        return nil, err
    }
    m.totalEntries.Add(1)
    return entry, nil
}

// Recover reads all valid entries from all segments in ascending SeqID order.
// Called once on startup. Corrupted entries are skipped and logged.
func (m *Manager) Recover() ([]*Entry, error) {
    r, err := NewReader(m.dir)
    if err != nil {
        return nil, err
    }
    return r.ReadAll()
}

// DeleteSegment deletes a WAL segment file by its index.
// Called by Sprint 4's hot tier after flushing a segment to RocksDB.
// Returns error if the segment file does not exist.
func (m *Manager) DeleteSegment(index uint64) error {
    path := filepath.Join(m.dir, SegmentFileName(index))
    if err := os.Remove(path); err != nil {
        return fmt.Errorf("failed to delete WAL segment %s: %w", SegmentFileName(index), err)
    }
    return nil
}

// Close flushes and closes the WAL writer. Call during graceful shutdown.
func (m *Manager) Close() error {
    return m.writer.Close()
}

// Stats returns current WAL statistics.
// SegmentCount is computed by scanning the directory on every call — always accurate.
func (m *Manager) Stats() WALStats {
    segments, _ := listSegments(m.dir)
    return WALStats{
        SegmentCount:    len(segments),
        ActiveSegment:   m.writer.CurrentSegmentIndex(),
        ActiveSizeBytes: m.writer.CurrentSize(),
        TotalEntries:    m.totalEntries.Load(),
    }
}
```

**Verify:** `go build ./internal/storage/wal/` compiles with no errors.

---

## TASK 08 — Update internal/server/server.go

**Action:** Make three targeted changes to `internal/server/server.go`.

**Change 1 — Add `wal` field to `Server` struct:**
```go
// Find the Server struct and add the wal field:
type Server struct {
    router     *chi.Mux
    cfg        *config.Config
    httpServer *http.Server
    startTime  time.Time
    version    string
    store      *auth.Store
    blacklist  *auth.Blacklist
    wal        *walmanager.Manager  // ← ADD THIS FIELD
}
```

**Change 2 — Update `New()` to accept and store the WAL manager:**
```go
// Update the signature:
func New(cfg *config.Config, version string, store *auth.Store,
    blacklist *auth.Blacklist, wal *walmanager.Manager) *Server {
    s := &Server{
        router:    chi.NewRouter(),
        cfg:       cfg,
        startTime: time.Now(),
        version:   version,
        store:     store,
        blacklist: blacklist,
        wal:       wal,       // ← ADD THIS ASSIGNMENT
    }
    // ... rest of New() unchanged (httpServer setup, setupMiddleware, setupRoutes) ...
}
```

**Change 3 — Update `handleHealth` to include WAL stats:**
```go
// In handleHealth, replace the utils.OK call with this:
stats := s.wal.Stats()
utils.OK(w, r, map[string]interface{}{
    "version":        s.version,
    "env":            s.cfg.Env,
    "uptime_seconds": int64(time.Since(s.startTime).Seconds()),
    "pid":            os.Getpid(),
    "go_version":     utils.GetGoVersion(),
    "os_arch":        utils.GetOSArch(),
    "wal": map[string]interface{}{
        "segment_count":     stats.SegmentCount,
        "active_segment":    stats.ActiveSegment,
        "active_size_bytes": stats.ActiveSizeBytes,
        "total_entries":     stats.TotalEntries,
    },
})
```

**Import alias to add to server.go imports:**
```go
walmanager "github.com/plomvix/plomvix/internal/storage/wal"
```

**Verify:** `go build ./internal/server/` compiles with no errors.

---

## TASK 09 — Update cmd/plomvix/main.go

**Action:** Make three targeted changes to `cmd/plomvix/main.go`.

**Change 1 — Add import alias:**
```go
// Add to imports block:
walmanager "github.com/plomvix/plomvix/internal/storage/wal"
```

**Change 2 — Insert WAL init after `defer blacklist.Stop()` and before `srv := server.New(...)`:**

Find this exact code in main.go:
```go
blacklist := auth.NewBlacklist()
defer blacklist.Stop()

// ← INSERT HERE

srv := server.New(cfg, Version, store, blacklist)
```

Insert the following:
```go
// Open WAL
wal, err := walmanager.Open(
    filepath.Join(cfg.Storage.DataDir, "wal"), cfg)
if err != nil {
    logger.Error("failed to open WAL", zap.Error(err))
    os.Exit(1)
}

// Run recovery — logs entries found, does not replay in Sprint 3
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
// In Sprint 3, recovered entries are logged but not replayed.
// Sprint 4 will consume them when the hot tier is implemented.
_ = entries // suppress unused variable warning
```

**Change 3 — Pass `wal` to `server.New()`:**
```go
// Change this line:
srv := server.New(cfg, Version, store, blacklist)

// To this:
srv := server.New(cfg, Version, store, blacklist, wal)
```

**Verify:** `go build ./cmd/plomvix/` compiles with no errors.

---

## TASK 10 — Create internal/storage/wal/types_test.go

**Action:** Create `internal/storage/wal/types_test.go`.

**Package declaration:** `package wal`

**Imports required:**
```go
import "testing"
```

**Full file content:**
```go
package wal

import "testing"

func TestSegmentFileName(t *testing.T) {
    tests := []struct {
        index    uint64
        expected string
    }{
        {1, "seg-000001.wal"},
        {42, "seg-000042.wal"},
        {999999, "seg-999999.wal"},
    }
    for _, tt := range tests {
        got := SegmentFileName(tt.index)
        if got != tt.expected {
            t.Errorf("SegmentFileName(%d) = %q, want %q", tt.index, got, tt.expected)
        }
    }
}

func TestParseSegmentIndex(t *testing.T) {
    valid := []struct {
        filename string
        expected uint64
    }{
        {"seg-000001.wal", 1},
        {"seg-000042.wal", 42},
        {"seg-999999.wal", 999999},
    }
    for _, tt := range valid {
        got, err := ParseSegmentIndex(tt.filename)
        if err != nil {
            t.Errorf("ParseSegmentIndex(%q) unexpected error: %v", tt.filename, err)
        }
        if got != tt.expected {
            t.Errorf("ParseSegmentIndex(%q) = %d, want %d", tt.filename, got, tt.expected)
        }
    }

    invalid := []string{"notawal.txt", "", "seg-.wal", "seg-abc.wal"}
    for _, name := range invalid {
        _, err := ParseSegmentIndex(name)
        if err == nil {
            t.Errorf("ParseSegmentIndex(%q) expected error, got nil", name)
        }
    }
}

func TestComputeCRC32Deterministic(t *testing.T) {
    e := &Entry{SeqID: 1, Timestamp: 12345, DataType: DataTypeLog, Payload: []byte(`{"a":1}`)}
    c1 := ComputeCRC32(e)
    c2 := ComputeCRC32(e)
    if c1 != c2 {
        t.Error("ComputeCRC32 not deterministic for identical entry")
    }
}

func TestComputeCRC32ChangesWithFields(t *testing.T) {
    base := &Entry{SeqID: 1, Timestamp: 12345, DataType: DataTypeLog, Payload: []byte(`{"a":1}`)}
    baseHash := ComputeCRC32(base)

    changed := &Entry{SeqID: 2, Timestamp: 12345, DataType: DataTypeLog, Payload: []byte(`{"a":1}`)}
    if ComputeCRC32(changed) == baseHash {
        t.Error("CRC32 did not change when SeqID changed")
    }

    changedPayload := &Entry{SeqID: 1, Timestamp: 12345, DataType: DataTypeLog, Payload: []byte(`{"a":2}`)}
    if ComputeCRC32(changedPayload) == baseHash {
        t.Error("CRC32 did not change when Payload changed")
    }
}

func TestVerifyCRC32(t *testing.T) {
    e := &Entry{SeqID: 1, Timestamp: 12345, DataType: DataTypeLog, Payload: []byte(`{"a":1}`)}
    e.CRC32 = ComputeCRC32(e)

    if !VerifyCRC32(e) {
        t.Error("VerifyCRC32 returned false for valid entry")
    }

    // Tamper with payload
    e.Payload = []byte(`{"a":9}`)
    if VerifyCRC32(e) {
        t.Error("VerifyCRC32 returned true after payload was modified")
    }
}
```

**Verify:** `go test -race ./internal/storage/wal/` — TestSegmentFileName, TestParseSegmentIndex, TestComputeCRC32*, TestVerifyCRC32 all pass.

---

## TASK 11 — Create internal/storage/wal/encoding_test.go

**Action:** Create `internal/storage/wal/encoding_test.go`.

**Package declaration:** `package wal`

**Imports required:**
```go
import (
    "bytes"
    "io"
    "testing"
)
```

**Full file content:**
```go
package wal

import (
    "bytes"
    "io"
    "testing"
)

// makeTestEntry creates a valid Entry with CRC32 computed.
// Used by multiple encoding tests.
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

func TestEncodeDecodeRoundtrip(t *testing.T) {
    e := &Entry{
        SeqID:     42,
        Timestamp: 9876543210,
        DataType:  DataTypeMetric,
        Payload:   []byte(`{"metric":"cpu","value":87.5}`),
    }
    e.CRC32 = ComputeCRC32(e)

    encoded, err := EncodeEntry(e)
    if err != nil {
        t.Fatalf("EncodeEntry failed: %v", err)
    }

    expectedLen := HeaderSize + len(e.Payload)
    if len(encoded) != expectedLen {
        t.Errorf("encoded length = %d, want %d", len(encoded), expectedLen)
    }

    decoded, err := DecodeEntry(bytes.NewReader(encoded))
    if err != nil {
        t.Fatalf("DecodeEntry failed: %v", err)
    }

    if decoded.SeqID != e.SeqID {
        t.Errorf("SeqID = %d, want %d", decoded.SeqID, e.SeqID)
    }
    if decoded.Timestamp != e.Timestamp {
        t.Errorf("Timestamp = %d, want %d", decoded.Timestamp, e.Timestamp)
    }
    if decoded.DataType != e.DataType {
        t.Errorf("DataType = %d, want %d", decoded.DataType, e.DataType)
    }
    if decoded.CRC32 != e.CRC32 {
        t.Errorf("CRC32 = %d, want %d", decoded.CRC32, e.CRC32)
    }
    if !bytes.Equal(decoded.Payload, e.Payload) {
        t.Errorf("Payload = %q, want %q", decoded.Payload, e.Payload)
    }
}

func TestDecodeEOF(t *testing.T) {
    _, err := DecodeEntry(bytes.NewReader([]byte{}))
    if err != io.EOF {
        t.Errorf("expected io.EOF on empty reader, got %v", err)
    }
}

func TestDecodeCorruptMagic(t *testing.T) {
    e := makeTestEntry()
    encoded, _ := EncodeEntry(e)
    encoded[0] = 0xFF // corrupt first byte of magic
    _, err := DecodeEntry(bytes.NewReader(encoded))
    if err != ErrCorruptEntry {
        t.Errorf("expected ErrCorruptEntry for corrupt magic, got %v", err)
    }
}

func TestDecodeCorruptCRC(t *testing.T) {
    e := makeTestEntry()
    encoded, _ := EncodeEntry(e)
    encoded[len(encoded)-1] ^= 0xFF // flip last byte of payload
    _, err := DecodeEntry(bytes.NewReader(encoded))
    if err != ErrCorruptEntry {
        t.Errorf("expected ErrCorruptEntry for corrupt payload, got %v", err)
    }
}

func TestDecodeTruncated(t *testing.T) {
    e := makeTestEntry()
    encoded, _ := EncodeEntry(e)
    truncated := encoded[:HeaderSize/2] // truncate mid-header
    _, err := DecodeEntry(bytes.NewReader(truncated))
    if err != ErrCorruptEntry {
        t.Errorf("expected ErrCorruptEntry for truncated entry, got %v", err)
    }
}

func TestDecodeMultipleEntries(t *testing.T) {
    buf := new(bytes.Buffer)

    // Write 3 entries back-to-back
    for i := uint64(1); i <= 3; i++ {
        e := &Entry{
            SeqID:     i,
            Timestamp: int64(i * 1000),
            DataType:  DataTypeLog,
            Payload:   []byte(`{"seq":"test"}`),
        }
        e.CRC32 = ComputeCRC32(e)
        encoded, err := EncodeEntry(e)
        if err != nil {
            t.Fatalf("EncodeEntry failed: %v", err)
        }
        buf.Write(encoded)
    }

    r := bytes.NewReader(buf.Bytes())
    for i := uint64(1); i <= 3; i++ {
        entry, err := DecodeEntry(r)
        if err != nil {
            t.Fatalf("DecodeEntry[%d] failed: %v", i, err)
        }
        if entry.SeqID != i {
            t.Errorf("entry %d SeqID = %d, want %d", i, entry.SeqID, i)
        }
    }

    // 4th read must return io.EOF
    _, err := DecodeEntry(r)
    if err != io.EOF {
        t.Errorf("expected io.EOF after 3 entries, got %v", err)
    }
}
```

**Verify:** `go test -race ./internal/storage/wal/` — all encoding tests pass with no disk I/O.

---

## TASK 12 — Create internal/storage/wal/wal_test.go

**Action:** Create `internal/storage/wal/wal_test.go`.

**Package declaration:** `package wal`

**Imports required:**
```go
import (
    "path/filepath"
    "testing"

    "github.com/plomvix/plomvix/internal/config"
)
```

**Full file content:**
```go
package wal

import (
    "path/filepath"
    "testing"

    "github.com/plomvix/plomvix/internal/config"
)

// newTestWALDir returns a temp WAL directory path (not yet created).
// Using t.TempDir() ensures cleanup after each test.
func newTestWALDir(t *testing.T) string {
    t.Helper()
    return filepath.Join(t.TempDir(), "wal")
}

// testWALConfig returns a minimal config for WAL tests.
func testWALConfig(dataDir string) *config.Config {
    return &config.Config{
        Storage: config.StorageConfig{
            DataDir:           dataDir,
            WALFlushThreshold: 64 * 1024 * 1024, // 64MB
        },
    }
}

func TestWriterBasic(t *testing.T) {
    dir := newTestWALDir(t)
    w, err := NewWriter(dir, 64*1024*1024)
    if err != nil {
        t.Fatalf("NewWriter failed: %v", err)
    }
    defer w.Close()

    e1, err := w.Write(DataTypeLog, []byte(`{"level":"info"}`))
    if err != nil {
        t.Fatalf("Write 1 failed: %v", err)
    }
    if e1.SeqID != 1 {
        t.Errorf("e1.SeqID = %d, want 1", e1.SeqID)
    }

    e2, err := w.Write(DataTypeMetric, []byte(`{"metric":"cpu"}`))
    if err != nil {
        t.Fatalf("Write 2 failed: %v", err)
    }
    if e2.SeqID != 2 {
        t.Errorf("e2.SeqID = %d, want 2", e2.SeqID)
    }
}

func TestWriterAndReaderRoundtrip(t *testing.T) {
    dir := newTestWALDir(t)

    // Write 5 entries
    w, _ := NewWriter(dir, 64*1024*1024)
    payloads := [][]byte{
        []byte(`{"type":"log"}`),
        []byte(`{"type":"metric"}`),
        []byte(`{"type":"json"}`),
        []byte(`{"type":"kv"}`),
        []byte(`{"type":"log2"}`),
    }
    types := []DataType{DataTypeLog, DataTypeMetric, DataTypeJSON, DataTypeKV, DataTypeLog}
    for i, p := range payloads {
        if _, err := w.Write(types[i], p); err != nil {
            t.Fatalf("Write %d failed: %v", i+1, err)
        }
    }
    w.Close()

    // Read all back
    r, err := NewReader(dir)
    if err != nil {
        t.Fatalf("NewReader failed: %v", err)
    }
    entries, err := r.ReadAll()
    if err != nil {
        t.Fatalf("ReadAll failed: %v", err)
    }
    if len(entries) != 5 {
        t.Errorf("len(entries) = %d, want 5", len(entries))
    }
    for i, e := range entries {
        if e.SeqID != uint64(i+1) {
            t.Errorf("entries[%d].SeqID = %d, want %d", i, e.SeqID, i+1)
        }
        if !VerifyCRC32(e) {
            t.Errorf("entries[%d] CRC32 verification failed", i)
        }
    }
}

func TestSegmentRotation(t *testing.T) {
    dir := newTestWALDir(t)
    // 100 bytes max — each entry is ~70 bytes so 1 entry per segment
    w, err := NewWriter(dir, 100)
    if err != nil {
        t.Fatalf("NewWriter failed: %v", err)
    }

    // Write 4 entries — should produce at least 3 segments
    for i := 0; i < 4; i++ {
        if _, err := w.Write(DataTypeLog, []byte(`{"fill":"rotation-test-data"}`)); err != nil {
            t.Fatalf("Write %d failed: %v", i+1, err)
        }
    }
    w.Close()

    r, err := NewReader(dir)
    if err != nil {
        t.Fatalf("NewReader failed: %v", err)
    }
    if r.SegmentCount() < 2 {
        t.Errorf("SegmentCount = %d, want >= 2 (rotation not triggered)", r.SegmentCount())
    }

    entries, err := r.ReadAll()
    if err != nil {
        t.Fatalf("ReadAll failed: %v", err)
    }
    if len(entries) != 4 {
        t.Errorf("len(entries) = %d, want 4", len(entries))
    }
    // Verify ascending SeqID order with no duplicates
    for i, e := range entries {
        if e.SeqID != uint64(i+1) {
            t.Errorf("entries[%d].SeqID = %d, want %d", i, e.SeqID, i+1)
        }
    }
}

func TestRecoveryAfterReopen(t *testing.T) {
    dir := newTestWALDir(t)

    // Write 3 entries and close
    w, _ := NewWriter(dir, 64*1024*1024)
    for i := 0; i < 3; i++ {
        w.Write(DataTypeLog, []byte(`{}`))
    }
    w.Close()

    // Reopen — simulates crash recovery
    w2, err := NewWriter(dir, 64*1024*1024)
    if err != nil {
        t.Fatalf("NewWriter reopen failed: %v", err)
    }
    defer w2.Close()

    // Next SeqID must continue from 4, not restart at 1
    e, err := w2.Write(DataTypeLog, []byte(`{}`))
    if err != nil {
        t.Fatalf("Write after reopen failed: %v", err)
    }
    if e.SeqID != 4 {
        t.Errorf("SeqID after reopen = %d, want 4", e.SeqID)
    }
}

func TestManagerRecovery(t *testing.T) {
    dir := newTestWALDir(t)
    cfg := testWALConfig(dir)

    // Write 2 entries via Manager and close
    m, err := Open(dir, cfg)
    if err != nil {
        t.Fatalf("Open failed: %v", err)
    }
    m.Write(DataTypeLog, []byte(`{"a":1}`))
    m.Write(DataTypeKV, []byte(`{"b":2}`))
    m.Close()

    // Reopen and recover
    m2, err := Open(dir, cfg)
    if err != nil {
        t.Fatalf("Open (reopen) failed: %v", err)
    }
    defer m2.Close()

    entries, err := m2.Recover()
    if err != nil {
        t.Fatalf("Recover failed: %v", err)
    }
    if len(entries) != 2 {
        t.Fatalf("len(entries) = %d, want 2", len(entries))
    }
    if entries[0].DataType != DataTypeLog {
        t.Errorf("entries[0].DataType = %d, want DataTypeLog", entries[0].DataType)
    }
    if entries[1].DataType != DataTypeKV {
        t.Errorf("entries[1].DataType = %d, want DataTypeKV", entries[1].DataType)
    }
}

func TestDeleteSegment(t *testing.T) {
    dir := newTestWALDir(t)
    // Use tiny max size to force rotation and get at least 2 segments
    smallCfg := &config.Config{Storage: config.StorageConfig{
        DataDir:           dir,
        WALFlushThreshold: 100,
    }}

    m, err := Open(dir, smallCfg)
    if err != nil {
        t.Fatalf("Open failed: %v", err)
    }
    // Write 5 entries — forces multiple segment rotations
    for i := 0; i < 5; i++ {
        m.Write(DataTypeLog, []byte(`{"fill":"padding-data-to-force-rotation"}`))
    }
    m.Close()

    // Reopen — segment 1 is complete and not the active segment
    m2, err := Open(dir, smallCfg)
    if err != nil {
        t.Fatalf("Open (reopen) failed: %v", err)
    }
    defer m2.Close()

    // Delete segment 1 (closed, not active)
    if err := m2.DeleteSegment(1); err != nil {
        t.Fatalf("DeleteSegment(1) failed: %v", err)
    }

    // Verify segment 1 file no longer exists
    seg1Path := filepath.Join(dir, SegmentFileName(1))
    if _, err := os.Stat(seg1Path); !os.IsNotExist(err) {
        t.Errorf("segment 1 still exists after DeleteSegment")
    }
}
```

**NOTE:** `os` is used in `TestDeleteSegment` — add it to imports:
```go
import (
    "os"
    "path/filepath"
    "testing"

    "github.com/plomvix/plomvix/internal/config"
)
```

**Verify:** `go test -race ./internal/storage/wal/` — all wal_test.go tests pass.

---

## TASK 13 — Create docs/api/health.md

**Action:**
```bash
mkdir -p docs/api  # already exists from Sprint 2
```

Create `docs/api/health.md` with the following **actual content** — not placeholders:

```markdown
# Plomvix Health API

## GET /health

Returns the current health status of the Plomvix server.

**Auth:** None — public endpoint, no authentication required.

---

### Response — 200 OK (all checks pass)

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
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

### Response — 503 Service Unavailable (checks failing)

```json
{
  "status": "error",
  "error": {
    "code": "HEALTH_CHECK_FAILED",
    "message": "One or more health checks failed",
    "details": [
      "data directory not writable: ./data/wal",
      "data directory not writable: ./data/hot"
    ]
  },
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

---

### Data Fields

| Field | Type | Description |
|---|---|---|
| `version` | string | Binary version (injected at build time) |
| `env` | string | Environment mode: `development` or `production` |
| `uptime_seconds` | int64 | Seconds since server started |
| `pid` | int | Operating system process ID |
| `go_version` | string | Go runtime version, e.g. `go1.22.0` |
| `os_arch` | string | OS and architecture, e.g. `linux/amd64` |

### WAL Stats Fields (added in Sprint 3)

| Field | Type | Description |
|---|---|---|
| `wal.segment_count` | int | Number of WAL segment files currently on disk |
| `wal.active_segment` | uint64 | Index of the currently active (writable) segment |
| `wal.active_size_bytes` | int64 | Bytes written to the active segment so far |
| `wal.total_entries` | int64 | Total WAL entries written since server start (resets on restart) |

---

### Example

```bash
curl http://localhost:8080/health
```

### Checks Performed

The health handler verifies that each data subdirectory is writable by
creating and immediately deleting a temporary file. Directories checked:

- `{data_dir}/wal`
- `{data_dir}/hot`
- `{data_dir}/cold/logs`
- `{data_dir}/cold/metrics`
- `{data_dir}/cold/json`
- `{data_dir}/cold/kv`

If any directory is not writable, the response is 503 with a `details` array
listing each failing directory.
```

**Verify:** `cat docs/api/health.md` shows full content. Renders correctly on GitHub.

---

## TASK 14 — Full build and smoke test

**Action:** Run the following verification sequence:

```bash
#!/bin/bash
set -euo pipefail

SERVER_PID=""
cleanup() {
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        kill -SIGTERM "$SERVER_PID" 2>/dev/null || true
        wait "$SERVER_PID" 2>/dev/null || true
    fi
}
trap cleanup EXIT

echo "=== Step 1: Build ==="
make vet
make build

echo ""
echo "=== Step 2: Run all tests ==="
make test

echo ""
echo "=== Step 3: Boot server ==="
./plomvix > /tmp/plomvix_s3.log 2>&1 &
SERVER_PID=$!
sleep 2

echo ""
echo "=== Step 4: Health check includes WAL stats ==="
HEALTH=$(curl -sf http://localhost:8080/health)
echo "$HEALTH" | jq .
# Verify wal block is present
echo "$HEALTH" | jq '.data.wal' | grep -q "segment_count" \
    && echo "PASS: WAL stats in health response" \
    || { echo "FAIL: wal block missing from health response"; exit 1; }

echo ""
echo "=== Step 5: WAL recovery log on fresh boot ==="
grep -i "WAL recovery complete" /tmp/plomvix_s3.log \
    && echo "PASS: WAL recovery logged" \
    || { echo "FAIL: WAL recovery log not found"; exit 1; }

echo ""
echo "=== Step 6: Auth still works after WAL integration ==="
TOKEN=$(curl -sf -X POST http://localhost:8080/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"changeme"}' \
    | jq -r '.data.token')
curl -sf http://localhost:8080/admin/users \
    -H "Authorization: Bearer $TOKEN" | jq . > /dev/null \
    && echo "PASS: Auth still works" \
    || { echo "FAIL: Auth broken after WAL integration"; exit 1; }

echo ""
echo "=== Step 7: Graceful shutdown ==="
kill -SIGTERM "$SERVER_PID"
wait "$SERVER_PID"
EXIT_CODE=$?
SERVER_PID=""
[ "$EXIT_CODE" -eq 0 ] \
    && echo "PASS: clean shutdown (exit code 0)" \
    || { echo "FAIL: exit code $EXIT_CODE"; exit 1; }

echo ""
echo "================================================"
echo "  ALL STEPS PASSED — Sprint 3 smoke test DONE  "
echo "================================================"
```

**Expected results:**

| Step | What is verified | Expected |
|---|---|---|
| 1 | Build + vet | Binary produced, no errors |
| 2 | All tests including WAL tests | All pass with race detector |
| 3 | Boot | Server starts, WAL opens, recovery runs |
| 4 | Health endpoint | `wal` block present with `segment_count`, `active_segment`, etc. |
| 5 | WAL recovery log | "WAL recovery complete" appears in startup logs |
| 6 | Auth regression | Login and protected routes still work after WAL integration |
| 7 | Graceful shutdown | Exit code 0, WAL closed cleanly |

---

## EXECUTION ORDER SUMMARY

```
TASK 01  →  Create internal/storage/wal/ directory
TASK 02  →  internal/storage/wal/types.go
TASK 03  →  internal/storage/wal/checksum.go
TASK 04  →  internal/storage/wal/encoding.go
TASK 05  →  internal/storage/wal/writer.go
TASK 06  →  internal/storage/wal/reader.go
TASK 07  →  internal/storage/wal/manager.go
TASK 08  →  internal/server/server.go (3 targeted changes)
TASK 09  →  cmd/plomvix/main.go (3 targeted changes)
TASK 10  →  internal/storage/wal/types_test.go
TASK 11  →  internal/storage/wal/encoding_test.go
TASK 12  →  internal/storage/wal/wal_test.go
TASK 13  →  docs/api/health.md
TASK 14  →  smoke test — all 7 steps must pass
```

---

*Sprint 3 complete when TASK 14 passes with zero failures.*
*Plomvix — Built in India. Built for the world.*