# Plomvix — Sprint 4 Task Plan
### For: DeepSeek V4 Pro Coding Agent
### Language: Go 1.22 | Module: github.com/plomvix/plomvix

> Execute tasks in exact order. Each task is atomic — one file or one concern.
> Do not skip ahead. Each task depends on the previous.
> Every task has a Verify step — do not proceed until it passes.

---

## CONTEXT

Sprints 1, 2, and 3 are complete. Sprint 4 adds the **Hot Tier** — a RocksDB-backed
storage engine that receives replayed WAL entries on startup and accepts new writes.

**What Sprint 4 delivers:**
- RocksDB opened with correct column families at startup
- WAL entries from Sprint 3 recovery are replayed into RocksDB
- New data can be written to RocksDB via the hot tier
- Data can be read back by key or by time range per column family
- Hot tier stats exposed in `GET /health`
- Full test coverage

**What Sprint 4 does NOT do:**
- No HTTP ingestion endpoint yet — that is Sprint 5
- No SQL query engine — that is Sprint 6
- No BoltDB migration — user auth stays on BoltDB permanently through Sprint 4.
  The original plan to migrate users to RocksDB is dropped:
  auth storage and data storage serve different purposes, BoltDB is correct for auth.

---

## ROCKSDB SETUP — READ BEFORE WRITING ANY CODE

RocksDB requires CGO and system libraries. The agent must install them before
writing any Go code or the build will fail with linker errors.

**Supported platforms:** Linux (Ubuntu/Debian) and macOS.

**Ubuntu/Debian install:**
```bash
sudo apt-get update
sudo apt-get install -y \
    librocksdb-dev \
    libsnappy-dev \
    liblz4-dev \
    libzstd-dev \
    libgflags-dev \
    build-essential
```

**macOS install:**
```bash
brew install rocksdb snappy lz4 zstd
```

**Go binding:**
```bash
go get github.com/linxGnu/grocksdb
go mod tidy
```

> `github.com/linxGnu/grocksdb` is the maintained RocksDB binding for Go.
> It wraps the C++ RocksDB library via CGO.
> CGO must be enabled: `CGO_ENABLED=1` (default when system libs are present).

**Makefile update required (TASK 01):**
All `make` commands must set `CGO_ENABLED=1` explicitly:
```makefile
CGO_ENABLED ?= 1
export CGO_ENABLED
```

**Verify RocksDB is accessible:**
```bash
CGO_ENABLED=1 go build ./...
```

---

## COLUMN FAMILY DESIGN — READ BEFORE WRITING ANY CODE

RocksDB organises data into column families (CFs). Each CF is an independent
key-value namespace with its own compaction and memory settings.

**Plomvix column families:**

| CF Name | Purpose | Key format |
|---|---|---|
| `default` | RocksDB internal (always exists) | not used by Plomvix |
| `logs` | Log entries | `{timestamp_ns_be_8bytes}{uuid_16bytes}` |
| `metrics` | Metric entries | `{timestamp_ns_be_8bytes}{metric_name_bytes}{0x00}{uuid_16bytes}` |
| `json` | JSON document entries | `{timestamp_ns_be_8bytes}{uuid_16bytes}` |
| `kv` | Key-value entries | `{user_key_bytes}` |

**Key design rationale:**
- Timestamp prefix (big-endian uint64) means keys sort chronologically within a CF
- UUID suffix prevents collisions when multiple entries share the same timestamp
- `metrics` includes metric name before UUID so range scans can filter by name
- `kv` uses the user-supplied key directly — no timestamp prefix

**Value format:** raw JSON bytes of the record payload (same as WAL payload).

---

## TASK 01 — Install RocksDB system libraries and add Go dependency

**Action — Part A: Install system libraries**

Linux:
```bash
sudo apt-get update && sudo apt-get install -y \
    librocksdb-dev libsnappy-dev liblz4-dev libzstd-dev libgflags-dev build-essential
```

macOS:
```bash
brew install rocksdb snappy lz4 zstd
```

**Action — Part B: Add Go binding**
```bash
go get github.com/linxGnu/grocksdb
go mod tidy
```

**Action — Part C: Update Makefile**

Add these two lines near the top of `Makefile`, after the existing variable definitions:
```makefile
CGO_ENABLED ?= 1
export CGO_ENABLED
```

**Verify:**
```bash
CGO_ENABLED=1 go build ./...
```
Must compile with zero errors. If linker errors appear, verify system libraries
are installed and `pkg-config --libs rocksdb` returns a result.

---

## TASK 02 — Create internal/storage/hot/ directory and types.go

**Action — Part A:** Remove the `.gitkeep` placeholder:
```bash
rm internal/storage/hot/.gitkeep
```

**Action — Part B:** Create `internal/storage/hot/types.go`.

**Full file content:**
```go
package hot

import (
    "encoding/binary"

    "github.com/google/uuid"
)

// Column family names used in RocksDB.
// These strings must never change after data is written.
const (
    CFLogs    = "logs"
    CFMetrics = "metrics"
    CFJSON    = "json"
    CFKV      = "kv"
)

// AllColumnFamilies returns all Plomvix column family names including "default".
// RocksDB requires "default" to always be opened even if unused.
func AllColumnFamilies() []string {
    return []string{"default", CFLogs, CFMetrics, CFJSON, CFKV}
}

// BuildTimeSeriesKey builds a time-series key for logs and json column families.
// Format: timestamp_ns (8 bytes big-endian) + UUID (16 bytes)
func BuildTimeSeriesKey(timestampNs int64) []byte {
    key := make([]byte, 8+16)
    binary.BigEndian.PutUint64(key[:8], uint64(timestampNs))
    id := uuid.New()
    copy(key[8:], id[:])
    return key
}

// BuildMetricKey builds a key for the metrics column family.
// Format: timestamp_ns (8 bytes big-endian) + metric_name + 0x00 + UUID (16 bytes)
func BuildMetricKey(timestampNs int64, metricName string) []byte {
    nameBytes := []byte(metricName)
    key := make([]byte, 8+len(nameBytes)+1+16)
    binary.BigEndian.PutUint64(key[:8], uint64(timestampNs))
    copy(key[8:], nameBytes)
    key[8+len(nameBytes)] = 0x00
    id := uuid.New()
    copy(key[8+len(nameBytes)+1:], id[:])
    return key
}

// BuildKVKey builds a key for the kv column family.
// Format: raw user-supplied key bytes (no timestamp prefix)
func BuildKVKey(userKey string) []byte {
    return []byte(userKey)
}

// BuildRangeScanPrefix builds the 8-byte timestamp prefix for range scans.
// Used to iterate all entries from a given start timestamp.
func BuildRangeScanPrefix(timestampNs int64) []byte {
    prefix := make([]byte, 8)
    binary.BigEndian.PutUint64(prefix, uint64(timestampNs))
    return prefix
}
```

**Verify:** `CGO_ENABLED=1 go build ./internal/storage/hot/` compiles with no errors.

---

## TASK 03 — Create internal/storage/hot/options.go

**Action:** Create `internal/storage/hot/options.go`.

This file configures RocksDB options. Good defaults matter — bad options
cause silent write amplification or excessive memory use.

**Full file content:**
```go
package hot

import (
    "github.com/linxGnu/grocksdb"
)

// newDBOptions returns RocksDB options suitable for Plomvix's hot tier.
func newDBOptions() *grocksdb.Options {
    opts := grocksdb.NewDefaultOptions()
    opts.SetCreateIfMissing(true)
    opts.SetCreateMissingColumnFamilies(true)

    // Write buffer: 64MB per column family before flush to L0
    opts.SetWriteBufferSize(64 * 1024 * 1024)

    // Allow 2 write buffers in memory before stalling writes
    opts.SetMaxWriteBufferNumber(2)

    // Parallelism: use number of CPU cores for compaction and flush
    opts.IncreaseParallelism(4)

    // Enable compression: Snappy for all levels (fast, reasonable ratio)
    opts.SetCompression(grocksdb.SnappyCompression)

    return opts
}

// newColumnFamilyOptions returns options for individual column families.
// All Plomvix CFs use the same options — can be tuned per-CF in future sprints.
func newColumnFamilyOptions() *grocksdb.Options {
    opts := grocksdb.NewDefaultOptions()
    opts.SetCompression(grocksdb.SnappyCompression)
    opts.SetWriteBufferSize(64 * 1024 * 1024)
    return opts
}

// newWriteOptions returns write options for all put operations.
// Sync is set to false — WAL provides durability, RocksDB sync adds latency.
func newWriteOptions() *grocksdb.WriteOptions {
    wo := grocksdb.NewDefaultWriteOptions()
    wo.SetSync(false)
    return wo
}

// newReadOptions returns read options for all get and iterator operations.
func newReadOptions() *grocksdb.ReadOptions {
    return grocksdb.NewDefaultReadOptions()
}
```

**Verify:** `CGO_ENABLED=1 go build ./internal/storage/hot/` compiles with no errors.

---

## TASK 04 — Create internal/storage/hot/store.go

**Action:** Create `internal/storage/hot/store.go`.

This is the core RocksDB wrapper. All reads and writes go through this file.

**Imports required:**
```go
import (
    "fmt"
    "sync/atomic"

    "github.com/linxGnu/grocksdb"

    "github.com/plomvix/plomvix/internal/config"
    "github.com/plomvix/plomvix/pkg/utils"
)
```

**Full file content:**
```go
package hot

import (
    "fmt"
    "sync/atomic"

    "github.com/linxGnu/grocksdb"

    "github.com/plomvix/plomvix/internal/config"
    "github.com/plomvix/plomvix/pkg/utils"
)

// Store wraps a RocksDB instance with Plomvix-specific column family handles.
type Store struct {
    db      *grocksdb.DB
    cfs     map[string]*grocksdb.ColumnFamilyHandle
    wo      *grocksdb.WriteOptions
    ro      *grocksdb.ReadOptions
    writes  atomic.Int64 // total writes since open
    dataDir string
}

// openRocksDB opens (or creates) the RocksDB hot tier at the given path.
// Unexported — callers use hot.Open() in manager.go.
func openRocksDB(dataDir string, cfg *config.Config) (*Store, error) {
    dbPath := dataDir

    if err := utils.EnsureDir(dbPath); err != nil {
        return nil, fmt.Errorf("failed to create hot tier directory: %w", err)
    }

    dbOpts := newDBOptions()
    cfNames := AllColumnFamilies()
    cfOpts := make([]*grocksdb.Options, len(cfNames))
    for i := range cfNames {
        cfOpts[i] = newColumnFamilyOptions()
    }

    db, cfHandles, err := grocksdb.OpenDbColumnFamilies(dbOpts, dbPath, cfNames, cfOpts)
    if err != nil {
        return nil, fmt.Errorf("failed to open RocksDB: %w", err)
    }

    // Map CF names to handles
    cfs := make(map[string]*grocksdb.ColumnFamilyHandle, len(cfNames))
    for i, name := range cfNames {
        cfs[name] = cfHandles[i]
    }

    return &Store{
        db:      db,
        cfs:     cfs,
        wo:      newWriteOptions(),
        ro:      newReadOptions(),
        dataDir: dbPath,
    }, nil
}

// Put writes a key-value pair to the given column family.
func (s *Store) Put(cf string, key, value []byte) error {
    handle, err := s.cfHandle(cf)
    if err != nil {
        return err
    }
    if err := s.db.PutCF(s.wo, handle, key, value); err != nil {
        return fmt.Errorf("RocksDB put failed in CF %q: %w", cf, err)
    }
    s.writes.Add(1)
    return nil
}

// Get retrieves a value from the given column family by key.
// Returns nil, nil if the key does not exist.
func (s *Store) Get(cf string, key []byte) ([]byte, error) {
    handle, err := s.cfHandle(cf)
    if err != nil {
        return nil, err
    }
    slice, err := s.db.GetCF(s.ro, handle, key)
    if err != nil {
        return nil, fmt.Errorf("RocksDB get failed in CF %q: %w", cf, err)
    }
    defer slice.Free()
    if !slice.Exists() {
        return nil, nil
    }
    // Copy data out before Free() is called
    data := make([]byte, len(slice.Data()))
    copy(data, slice.Data())
    return data, nil
}

// Delete removes a key from the given column family.
func (s *Store) Delete(cf string, key []byte) error {
    handle, err := s.cfHandle(cf)
    if err != nil {
        return err
    }
    return s.db.DeleteCF(s.wo, handle, key)
}

// Scan iterates all keys in the given column family with the given prefix,
// calling fn for each key-value pair. Stops if fn returns false.
func (s *Store) Scan(cf string, prefix []byte, fn func(key, value []byte) bool) error {
    handle, err := s.cfHandle(cf)
    if err != nil {
        return err
    }

    ro := newReadOptions()
    defer ro.Destroy()

    it := s.db.NewIteratorCF(ro, handle)
    defer it.Close()

    if len(prefix) > 0 {
        it.Seek(prefix)
    } else {
        it.SeekToFirst()
    }

    for ; it.Valid(); it.Next() {
        k := it.Key()
        v := it.Value()

        // Copy data before releasing iterator resources
        keyCopy := make([]byte, len(k.Data()))
        copy(keyCopy, k.Data())
        valCopy := make([]byte, len(v.Data()))
        copy(valCopy, v.Data())

        k.Free()
        v.Free()

        if !fn(keyCopy, valCopy) {
            break
        }
    }

    if err := it.Err(); err != nil {
        return fmt.Errorf("RocksDB scan error in CF %q: %w", cf, err)
    }
    return nil
}

// Close closes the RocksDB database and all column family handles.
// Call during graceful shutdown.
func (s *Store) Close() {
    s.wo.Destroy()
    s.ro.Destroy()
    for _, handle := range s.cfs {
        handle.Destroy()
    }
    s.db.Close()
}

// TotalWrites returns the number of Put operations since Open.
func (s *Store) TotalWrites() int64 {
    return s.writes.Load()
}

// cfHandle returns the column family handle for the given name.
func (s *Store) cfHandle(cf string) (*grocksdb.ColumnFamilyHandle, error) {
    handle, ok := s.cfs[cf]
    if !ok {
        return nil, fmt.Errorf("unknown column family: %q", cf)
    }
    return handle, nil
}
```

**Verify:** `CGO_ENABLED=1 go build ./internal/storage/hot/` compiles with no errors.

---

## TASK 05 — Create internal/storage/hot/manager.go

**Action:** Create `internal/storage/hot/manager.go`.

The Manager is the public interface the rest of Plomvix uses for the hot tier.
It wraps the Store and provides WAL-aware write methods.

**Full file content:**
```go
package hot

import (
    "encoding/binary"
    "fmt"

    "github.com/plomvix/plomvix/internal/config"
    walstore "github.com/plomvix/plomvix/internal/storage/wal"
)

// HotStats holds current hot tier statistics.
type HotStats struct {
    TotalWrites int64
    DataDir     string
}

// Manager is the public interface for hot tier operations.
type Manager struct {
    store *Store
}

// Open opens the hot tier at the given directory path.
// Calls openRocksDB (defined in store.go) — not the exported Open in this file.
func Open(dataDir string, cfg *config.Config) (*Manager, error) {
    store, err := openRocksDB(dataDir, cfg)
    if err != nil {
        return nil, err
    }
    return &Manager{store: store}, nil
}

// WriteLog writes a log entry to the logs column family.
func (m *Manager) WriteLog(timestampNs int64, payload []byte) error {
    key := BuildTimeSeriesKey(timestampNs)
    return m.store.Put(CFLogs, key, payload)
}

// WriteMetric writes a metric entry to the metrics column family.
func (m *Manager) WriteMetric(timestampNs int64, metricName string, payload []byte) error {
    key := BuildMetricKey(timestampNs, metricName)
    return m.store.Put(CFMetrics, key, payload)
}

// WriteJSON writes a JSON document to the json column family.
func (m *Manager) WriteJSON(timestampNs int64, payload []byte) error {
    key := BuildTimeSeriesKey(timestampNs)
    return m.store.Put(CFJSON, key, payload)
}

// WriteKV writes a key-value entry to the kv column family.
func (m *Manager) WriteKV(userKey string, payload []byte) error {
    key := BuildKVKey(userKey)
    return m.store.Put(CFKV, key, payload)
}

// ReplayWALEntry replays a single WAL entry into the correct column family.
// Called during startup to hydrate RocksDB from the WAL.
func (m *Manager) ReplayWALEntry(entry *walstore.Entry) error {
    ts := entry.Timestamp
    payload := entry.Payload

    switch entry.DataType {
    case walstore.DataTypeLog:
        return m.WriteLog(ts, payload)
    case walstore.DataTypeMetric:
        // For WAL replay, metric name is not available separately —
        // use a placeholder key. Sprint 5 ingestion will supply the real name.
        return m.WriteMetric(ts, "unknown", payload)
    case walstore.DataTypeJSON:
        return m.WriteJSON(ts, payload)
    case walstore.DataTypeKV:
        // For WAL replay, KV key is embedded in payload — use timestamp as key.
        // Sprint 5 ingestion will supply the real key.
        return m.WriteKV(fmt.Sprintf("wal_replay_%d", ts), payload)
    default:
        return fmt.Errorf("unknown WAL data type: %d", entry.DataType)
    }
}

// ReplayWAL replays all recovered WAL entries into RocksDB.
// Returns the count of successfully replayed entries.
func (m *Manager) ReplayWAL(entries []*walstore.Entry) (int, error) {
    count := 0
    for _, entry := range entries {
        if err := m.ReplayWALEntry(entry); err != nil {
            return count, fmt.Errorf("WAL replay failed at SeqID %d: %w", entry.SeqID, err)
        }
        count++
    }
    return count, nil
}

// ScanLogs returns all log entries in the given time range [fromNs, toNs).
// fromNs and toNs are Unix nanoseconds. Pass 0 for fromNs to start from beginning.
func (m *Manager) ScanLogs(fromNs, toNs int64) ([][]byte, error) {
    return m.scanTimeRange(CFLogs, fromNs, toNs)
}

// ScanJSON returns all JSON entries in the given time range [fromNs, toNs).
func (m *Manager) ScanJSON(fromNs, toNs int64) ([][]byte, error) {
    return m.scanTimeRange(CFJSON, fromNs, toNs)
}

// GetKV retrieves a value by key from the kv column family.
// Returns nil, nil if the key does not exist.
func (m *Manager) GetKV(userKey string) ([]byte, error) {
    return m.store.Get(CFKV, BuildKVKey(userKey))
}

// scanTimeRange scans a time-series column family for entries in [fromNs, toNs).
func (m *Manager) scanTimeRange(cf string, fromNs, toNs int64) ([][]byte, error) {
    var results [][]byte
    prefix := BuildRangeScanPrefix(fromNs)

    err := m.store.Scan(cf, prefix, func(key, value []byte) bool {
        if toNs > 0 && len(key) >= 8 {
            keyTs := int64(bigEndianUint64(key[:8]))
            if keyTs >= toNs {
                return false
            }
        }
        results = append(results, value)
        return true
    })
    return results, err
}

// Stats returns current hot tier statistics.
func (m *Manager) Stats() HotStats {
    return HotStats{
        TotalWrites: m.store.TotalWrites(),
        DataDir:     m.store.dataDir,
    }
}

// Close closes the underlying RocksDB store.
func (m *Manager) Close() {
    m.store.Close()
}

// bigEndianUint64 decodes a big-endian uint64 from b.
func bigEndianUint64(b []byte) uint64 {
    return binary.BigEndian.Uint64(b)
}
```

**Verify:** `CGO_ENABLED=1 go build ./internal/storage/hot/` compiles with no errors.

---

## TASK 06 — Update cmd/plomvix/main.go — Hot Tier init

**Action:** Make three targeted changes to `cmd/plomvix/main.go`.

**Change 1 — Add import:**
```go
hot "github.com/plomvix/plomvix/internal/storage/hot"
```

**Change 2 — Insert hot tier init after `defer wal.Close()` and before `srv := server.New(...)`:**

Find this exact code:
```go
defer wal.Close()
// ...WAL recovery log...
_ = entries

// ← INSERT HERE

srv := server.New(cfg, Version, store, blacklist, wal)
```

Insert:
```go
// Open hot tier
hotPath := filepath.Join(cfg.Storage.DataDir, "hot")
hotTier, err := hot.Open(hotPath, cfg)
if err != nil {
    wal.Close()  // explicit — os.Exit bypasses defers
    logger.Error("failed to open hot tier", zap.Error(err))
    os.Exit(1)
}
defer hotTier.Close()

// Replay WAL entries into hot tier
replayCount, err := hotTier.ReplayWAL(entries)
if err != nil {
    hotTier.Close()
    wal.Close()
    logger.Error("WAL replay into hot tier failed", zap.Error(err))
    os.Exit(1)
}
logger.Info("hot tier ready",
    zap.Int("wal_entries_replayed", replayCount),
    zap.String("path", hotPath),
)
```

**Change 3 — Pass `hotTier` to `server.New()`:**
```go
// Change:
srv := server.New(cfg, Version, store, blacklist, wal)

// To:
srv := server.New(cfg, Version, store, blacklist, wal, hotTier)
```

**Verify:** `CGO_ENABLED=1 go build ./cmd/plomvix/` compiles with no errors.

---

## TASK 07 — Update internal/server/server.go — Hot Tier integration

**Action:** Three targeted changes to `internal/server/server.go`.

**Change 1 — Add `hotTier` field to `Server` struct:**
```go
type Server struct {
    router     *chi.Mux
    cfg        *config.Config
    httpServer *http.Server
    startTime  time.Time
    version    string
    store      *auth.Store
    blacklist  *auth.Blacklist
    wal        *walmanager.Manager
    hotTier    *hotmanager.Manager  // ← ADD
}
```

**Change 2 — Update `New()` signature:**
```go
func New(cfg *config.Config, version string, store *auth.Store,
    blacklist *auth.Blacklist, wal *walmanager.Manager,
    hotTier *hotmanager.Manager) *Server {   // ← ADD hotTier parameter
    s := &Server{
        // ...existing fields...
        hotTier: hotTier,   // ← ADD assignment
    }
    // ...rest unchanged...
}
```

**Change 3 — Update `handleHealth` to include hot tier stats:**
```go
hotStats := s.hotTier.Stats()
// In the utils.OK call, add:
"hot": map[string]interface{}{
    "total_writes": hotStats.TotalWrites,
    "data_dir":     hotStats.DataDir,
},
```

**Import alias to add:**
```go
hotmanager "github.com/plomvix/plomvix/internal/storage/hot"
```

**Full updated health response shape:**
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
    "wal": { "segment_count": 1, "active_segment": 1, "active_size_bytes": 0, "total_entries": 0 },
    "hot": { "total_writes": 0, "data_dir": "./data/hot" }
  },
  "request_id": "uuid"
}
```

**Verify:** `CGO_ENABLED=1 go build ./internal/server/` compiles with no errors.

---

## TASK 08 — Create internal/storage/hot/store_test.go

**Action:** Create `internal/storage/hot/store_test.go`.

**Package declaration:** `package hot`

**Imports:**
```go
import (
    "bytes"
    "path/filepath"
    "testing"

    "github.com/plomvix/plomvix/internal/config"
)
```

**Test helper:**
```go
func newTestStore(t *testing.T) *Store {
    t.Helper()
    dir := filepath.Join(t.TempDir(), "hot")
    cfg := &config.Config{
        Storage: config.StorageConfig{DataDir: dir},
    }
    store, err := openRocksDB(dir, cfg)
    if err != nil {
        t.Fatalf("openRocksDB failed: %v", err)
    }
    t.Cleanup(func() { store.Close() })
    return store
}
```

**Tests to implement:**

```go
func TestStoreOpenClose(t *testing.T) {
    // newTestStore opens and cleanup closes — if no panic, test passes
    _ = newTestStore(t)
}

func TestStorePutAndGet(t *testing.T) {
    s := newTestStore(t)
    key := []byte("testkey")
    val := []byte(`{"msg":"hello"}`)

    if err := s.Put(CFLogs, key, val); err != nil {
        t.Fatalf("Put failed: %v", err)
    }

    got, err := s.Get(CFLogs, key)
    if err != nil {
        t.Fatalf("Get failed: %v", err)
    }
    if !bytes.Equal(got, val) {
        t.Errorf("Get returned %q, want %q", got, val)
    }
}

func TestStoreGetMissing(t *testing.T) {
    s := newTestStore(t)
    got, err := s.Get(CFLogs, []byte("nonexistent"))
    if err != nil {
        t.Fatalf("Get on missing key returned error: %v", err)
    }
    if got != nil {
        t.Errorf("Get on missing key returned %q, want nil", got)
    }
}

func TestStoreDelete(t *testing.T) {
    s := newTestStore(t)
    key := []byte("delkey")
    s.Put(CFLogs, key, []byte(`{"x":1}`))
    if err := s.Delete(CFLogs, key); err != nil {
        t.Fatalf("Delete failed: %v", err)
    }
    got, _ := s.Get(CFLogs, key)
    if got != nil {
        t.Errorf("key still present after Delete")
    }
}

func TestStoreScan(t *testing.T) {
    s := newTestStore(t)
    // Write 3 entries to kv CF
    for i := 0; i < 3; i++ {
        key := []byte(fmt.Sprintf("key%d", i))
        val := []byte(fmt.Sprintf(`{"i":%d}`, i))
        s.Put(CFKV, key, val)
    }

    var collected [][]byte
    err := s.Scan(CFKV, nil, func(k, v []byte) bool {
        collected = append(collected, v)
        return true
    })
    if err != nil {
        t.Fatalf("Scan failed: %v", err)
    }
    if len(collected) != 3 {
        t.Errorf("Scan returned %d entries, want 3", len(collected))
    }
}

func TestStoreAllColumnFamilies(t *testing.T) {
    s := newTestStore(t)
    // Write and read from each CF to confirm all are accessible
    for _, cf := range []string{CFLogs, CFMetrics, CFJSON, CFKV} {
        key := []byte("probe")
        val := []byte(`{"cf":"` + cf + `"}`)
        if err := s.Put(cf, key, val); err != nil {
            t.Errorf("Put to CF %q failed: %v", cf, err)
        }
        got, err := s.Get(cf, key)
        if err != nil {
            t.Errorf("Get from CF %q failed: %v", cf, err)
        }
        if !bytes.Equal(got, val) {
            t.Errorf("CF %q: Get returned %q, want %q", cf, got, val)
        }
    }
}
```

Add `"fmt"` to imports — used in `TestStoreScan`.

**Verify:** `CGO_ENABLED=1 go test -race ./internal/storage/hot/` — all store tests pass.

---

## TASK 09 — Create internal/storage/hot/manager_test.go

**Action:** Create `internal/storage/hot/manager_test.go`.

**Package declaration:** `package hot`

**Imports:**
```go
import (
    "path/filepath"
    "testing"
    "time"

    "github.com/plomvix/plomvix/internal/config"
    walstore "github.com/plomvix/plomvix/internal/storage/wal"
)
```

**Test helper:**
```go
func newTestManager(t *testing.T) *Manager {
    t.Helper()
    dir := filepath.Join(t.TempDir(), "hot")
    cfg := &config.Config{
        Storage: config.StorageConfig{DataDir: dir},
    }
    m, err := Open(dir, cfg)
    if err != nil {
        t.Fatalf("hot.Open failed: %v", err)
    }
    t.Cleanup(func() { m.Close() })
    return m
}
```

**Tests to implement:**

```go
func TestManagerWriteLog(t *testing.T) {
    m := newTestManager(t)
    ts := time.Now().UnixNano()
    err := m.WriteLog(ts, []byte(`{"level":"info","msg":"hello"}`))
    if err != nil {
        t.Fatalf("WriteLog failed: %v", err)
    }
    if m.Stats().TotalWrites != 1 {
        t.Errorf("TotalWrites = %d, want 1", m.Stats().TotalWrites)
    }
}

func TestManagerWriteMetric(t *testing.T) {
    m := newTestManager(t)
    ts := time.Now().UnixNano()
    err := m.WriteMetric(ts, "cpu.usage", []byte(`{"value":87.5}`))
    if err != nil {
        t.Fatalf("WriteMetric failed: %v", err)
    }
}

func TestManagerWriteJSON(t *testing.T) {
    m := newTestManager(t)
    ts := time.Now().UnixNano()
    err := m.WriteJSON(ts, []byte(`{"event":"order_placed"}`))
    if err != nil {
        t.Fatalf("WriteJSON failed: %v", err)
    }
}

func TestManagerWriteAndGetKV(t *testing.T) {
    m := newTestManager(t)
    err := m.WriteKV("user:123", []byte(`{"name":"alice"}`))
    if err != nil {
        t.Fatalf("WriteKV failed: %v", err)
    }
    val, err := m.GetKV("user:123")
    if err != nil {
        t.Fatalf("GetKV failed: %v", err)
    }
    if string(val) != `{"name":"alice"}` {
        t.Errorf("GetKV returned %q, want %q", val, `{"name":"alice"}`)
    }
}

func TestManagerScanLogs(t *testing.T) {
    m := newTestManager(t)
    base := time.Now().UnixNano()

    // Write 3 log entries at distinct timestamps
    for i := 0; i < 3; i++ {
        m.WriteLog(base+int64(i)*1000, []byte(`{"seq":"log"}`))
    }

    // Scan all 3
    results, err := m.ScanLogs(base, base+int64(3)*1000)
    if err != nil {
        t.Fatalf("ScanLogs failed: %v", err)
    }
    if len(results) != 3 {
        t.Errorf("ScanLogs returned %d entries, want 3", len(results))
    }
}

func TestManagerReplayWAL(t *testing.T) {
    m := newTestManager(t)

    // Build fake WAL entries
    entries := []*walstore.Entry{
        {SeqID: 1, Timestamp: time.Now().UnixNano(), DataType: walstore.DataTypeLog,
            Payload: []byte(`{"msg":"replayed_log"}`)},
        {SeqID: 2, Timestamp: time.Now().UnixNano(), DataType: walstore.DataTypeJSON,
            Payload: []byte(`{"event":"replayed_json"}`)},
        {SeqID: 3, Timestamp: time.Now().UnixNano(), DataType: walstore.DataTypeKV,
            Payload: []byte(`{"kv":"replayed"}`)},
    }

    count, err := m.ReplayWAL(entries)
    if err != nil {
        t.Fatalf("ReplayWAL failed: %v", err)
    }
    if count != 3 {
        t.Errorf("ReplayWAL count = %d, want 3", count)
    }
    if m.Stats().TotalWrites != 3 {
        t.Errorf("TotalWrites = %d, want 3", m.Stats().TotalWrites)
    }
}
```

**Verify:** `CGO_ENABLED=1 go test -race ./internal/storage/hot/` — all manager tests pass.

---

## TASK 10 — Update docs/api/health.md

**Action:** Update `docs/api/health.md` to add the `hot` block to the 200 response example
and add a Hot Tier Stats table. The file already exists from Sprint 3.

**Add to the 200 OK response JSON example** (after the `wal` block):
```json
"hot": {
  "total_writes": 0,
  "data_dir": "./data/hot"
}
```

**Add a new section after WAL Stats Fields:**
```markdown
### Hot Tier Stats Fields (added in Sprint 4)

| Field | Type | Description |
|---|---|---|
| `hot.total_writes` | int64 | Total Put operations to RocksDB since server start (resets on restart) |
| `hot.data_dir` | string | Filesystem path of the RocksDB data directory |
```

**Verify:** `cat docs/api/health.md` shows both wal and hot sections.

---

## TASK 11 — Full build and smoke test

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
CGO_ENABLED=1 make vet
CGO_ENABLED=1 make build

echo ""
echo "=== Step 2: Run all tests ==="
CGO_ENABLED=1 make test

echo ""
echo "=== Step 3: Boot server ==="
./plomvix > /tmp/plomvix_s4.log 2>&1 &
SERVER_PID=$!
sleep 3  # RocksDB open takes slightly longer than BoltDB

echo ""
echo "=== Step 4: Health includes hot tier stats ==="
HEALTH=$(curl -sf http://localhost:8080/health)
echo "$HEALTH" | jq .
echo "$HEALTH" | jq '.data.hot' | grep -q "total_writes" \
    && echo "PASS: hot tier stats in health response" \
    || { echo "FAIL: hot block missing from health response"; exit 1; }

echo ""
echo "=== Step 5: WAL replay logged on boot ==="
grep -i "hot tier ready" /tmp/plomvix_s4.log \
    && echo "PASS: hot tier ready logged" \
    || { echo "FAIL: hot tier ready log not found"; exit 1; }

echo ""
echo "=== Step 6: WAL + auth regression check ==="
grep -i "WAL recovery complete" /tmp/plomvix_s4.log \
    && echo "PASS: WAL recovery logged" \
    || { echo "FAIL: WAL recovery log missing"; exit 1; }

TOKEN=$(curl -sf -X POST http://localhost:8080/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"changeme"}' \
    | jq -r '.data.token')
curl -sf http://localhost:8080/admin/users \
    -H "Authorization: Bearer $TOKEN" | jq . > /dev/null \
    && echo "PASS: Auth still works" \
    || { echo "FAIL: Auth broken"; exit 1; }

echo ""
echo "=== Step 7: Graceful shutdown ==="
kill -SIGTERM "$SERVER_PID"
wait "$SERVER_PID"
EXIT_CODE=$?
SERVER_PID=""
[ "$EXIT_CODE" -eq 0 ] \
    && echo "PASS: clean shutdown" \
    || { echo "FAIL: exit code $EXIT_CODE"; exit 1; }

echo ""
echo "================================================"
echo "  ALL STEPS PASSED — Sprint 4 smoke test DONE  "
echo "================================================"
```

**Expected results:**

| Step | What is verified | Expected |
|---|---|---|
| 1 | Build + vet with CGO | Binary produced, no errors |
| 2 | All tests including hot tier | All pass with race detector |
| 3 | Boot | Server starts, RocksDB opens, WAL replays |
| 4 | Health endpoint | `hot` block present with `total_writes` and `data_dir` |
| 5 | Hot tier ready log | "hot tier ready" with replay count in startup logs |
| 6 | WAL + auth regression | Both still work after hot tier integration |
| 7 | Graceful shutdown | Exit code 0, RocksDB closed cleanly |

---

## EXECUTION ORDER SUMMARY

```
TASK 01  →  Install RocksDB libs + go get grocksdb + update Makefile
TASK 02  →  internal/storage/hot/types.go
TASK 03  →  internal/storage/hot/options.go
TASK 04  →  internal/storage/hot/store.go
TASK 05  →  internal/storage/hot/manager.go
TASK 06  →  cmd/plomvix/main.go (3 targeted changes)
TASK 07  →  internal/server/server.go (3 targeted changes)
TASK 08  →  internal/storage/hot/store_test.go
TASK 09  →  internal/storage/hot/manager_test.go
TASK 10  →  docs/api/health.md (add hot block)
TASK 11  →  smoke test — all 7 steps must pass
```

---

*Sprint 4 complete when TASK 11 passes with zero failures.*
*Plomvix — Built in India. Built for the world.*