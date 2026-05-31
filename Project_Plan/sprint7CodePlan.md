# Plomvix — Sprint 7 Task Plan
### For: DeepSeek V4 Pro Coding Agent
### Language: Go 1.22 | Module: github.com/plomvix/plomvix

> Execute tasks in exact order. Each task is atomic — one file or one concern.
> Do not skip ahead. Each task depends on the previous.
> Every task has a Verify step — do not proceed until it passes.

---

## CONTEXT

Sprints 1–6 are complete. Sprint 7 adds the **Cold Tier** — a Parquet-based storage
layer for data that has aged out of the hot tier, plus a **Tiering Policy** that
automatically moves old data from RocksDB to Parquet files.

**What Sprint 7 delivers:**
- Parquet writer/reader for logs, metrics, and JSON data types
- Tiering policy engine — flushes hot tier data older than `retention_days` to cold tier
- Background tiering goroutine — runs every hour
- Cold tier query integration — query endpoints now search both hot and cold tiers
- Manual tier flush endpoint: `POST /admin/tier/flush`
- Tier stats in `GET /health`
- API documentation in `docs/api/tier.md`
- Full test coverage including tiering engine and hot+cold query integration

**Explicit scope decisions:**
- **KV is excluded from tiering in Sprint 7.** KV keys have no timestamp prefix,
  so the time-based eligibility check does not apply. KV tiering requires a
  separate design (Sprint 9+).
- **Hot tier deletion IS performed after successful cold write.** Deletion failure
  is treated as a flush error — not silently ignored. However, because deletion
  happens key-by-key after the Parquet write, a failed deletion can leave partial
  hot+cold state. The flush returns an error so operators can retry/reconcile.
- **No deduplication in query merge in Sprint 7.** Normal successful flushes place
  each record in exactly one tier. If a deletion failure occurs, retrying the flush
  or running reconciliation is required before relying on duplicate-free results.
- **Reconciliation tooling is deferred to Sprint 8+.** Sprint 7 provides error
  detection but not automatic reconciliation of partial flush failures.

**Data flow:**
```
[Hot Tier - RocksDB]  →  tiering policy (logs/metrics/json only)  →  [Cold Tier - Parquet]
                                          ↓
                             Query engine reads both tiers
```

---

## PARQUET LIBRARY — READ BEFORE WRITING ANY CODE

**Use:** `github.com/parquet-go/parquet-go` — pure Go, no CGO.

```bash
go get github.com/parquet-go/parquet-go
go mod tidy
```

**Row schema:**
```go
type ParquetRow struct {
    TimestampNs int64  `parquet:"timestamp_ns"`
    Payload     string `parquet:"payload"` // raw JSON string
}
```

---

## COLD TIER DIRECTORY LAYOUT — READ BEFORE WRITING ANY CODE

```
data/cold/
├── logs/
│   └── 2024-01-15/        ← date of OLDEST record in flush batch (UTC)
│       └── part-000001.parquet
├── metrics/
│   └── 2024-01-15/
│       └── part-000001.parquet
└── json/
    └── 2024-01-15/
        └── part-000001.parquet
```

KV has no cold tier directory in Sprint 7.
Date partition is based on the **oldest record's timestamp** in each flush batch,
not the retention cutoff date.

---

## TIERING POLICY — READ BEFORE WRITING ANY CODE

**Eligible data:** logs, metrics, json only — NOT kv.
**Eligible when:** record timestamp < `time.Now() - retention_days * 24h`

**Write path:**
```
1. Scan hot tier from 0 to cutoffNs
2. Collect all eligible rows
3. If rows found:
   a. Compute refTs = time.Unix(0, oldest_row_TimestampNs).UTC()
   b. Write rows to cold Parquet using refTs as partition date
   c. Delete each key from hot tier
   d. If any deletion fails → return error immediately. This prevents silent
      duplicate creation, but does not guarantee automatic rollback of records
      already written to cold tier.
4. Update stats only after all data types finish successfully
```

Deletion failure is a flush error. The flush aborts. This prevents hot+cold duplicates.

---

## TASK 01 — Add Parquet dependency

**Action:**
```bash
go get github.com/parquet-go/parquet-go
go mod tidy
```

**Verify:** `go build ./...` — no errors, no CGO required.

---

## TASK 02 — Create internal/storage/cold/types.go

**Action — Part A:** Remove placeholder:
```bash
rm internal/storage/cold/.gitkeep
```

**Action — Part B:** Create `internal/storage/cold/types.go`.

```go
package cold

import (
    "fmt"
    "path/filepath"
    "time"
)

// ParquetRow is the universal row schema for all Parquet files in Plomvix.
type ParquetRow struct {
    TimestampNs int64  `parquet:"timestamp_ns"`
    Payload     string `parquet:"payload"`
}

// Tierable data type constants. KV is intentionally excluded from Sprint 7 tiering.
const (
    DataTypeLogs    = "logs"
    DataTypeMetrics = "metrics"
    DataTypeJSON    = "json"
)

// TierableDataTypes returns the data types eligible for cold tier in Sprint 7.
// KV is excluded — KV keys have no timestamp prefix and require separate design.
func TierableDataTypes() []string {
    return []string{DataTypeLogs, DataTypeMetrics, DataTypeJSON}
}

// PartFileName returns the filename for a part file.
// Example: PartFileName(1) → "part-000001.parquet"
func PartFileName(index int) string {
    return fmt.Sprintf("part-%06d.parquet", index)
}

// DatePartitionDir returns the date-partitioned directory path for a data type and timestamp.
// Example: DatePartitionDir("logs", t) → "logs/2024-01-15"
func DatePartitionDir(dataType string, ts time.Time) string {
    return filepath.Join(dataType, ts.UTC().Format("2006-01-02"))
}

// TierStats holds statistics about the cold tier.
type TierStats struct {
    TotalParquetFiles int
    TotalRecordsMoved int64
    LastFlushAt       time.Time
    LastFlushDuration time.Duration
}
```

**Verify:** `go build ./internal/storage/cold/` compiles with no errors.

---

## TASK 03 — Create internal/storage/cold/writer.go

**Action:** Create `internal/storage/cold/writer.go`.

```go
package cold

import (
    "fmt"
    "os"
    "path/filepath"

    parquetgo "github.com/parquet-go/parquet-go"
)

// Writer writes ParquetRows to a single Parquet part file.
type Writer struct {
    path   string
    file   *os.File
    writer *parquetgo.GenericWriter[ParquetRow]
}

// NewWriter creates a new Parquet writer at the given path.
// Creates all parent directories if they do not exist.
func NewWriter(path string) (*Writer, error) {
    if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
        return nil, fmt.Errorf("failed to create parquet directory: %w", err)
    }
    f, err := os.Create(path)
    if err != nil {
        return nil, fmt.Errorf("failed to create parquet file %s: %w", path, err)
    }
    return &Writer{
        path:   path,
        file:   f,
        writer: parquetgo.NewGenericWriter[ParquetRow](f),
    }, nil
}

// WriteRow writes a single row.
func (w *Writer) WriteRow(row ParquetRow) error {
    _, err := w.writer.Write([]ParquetRow{row})
    return err
}

// Close flushes and closes the file. Must be called after all rows are written.
func (w *Writer) Close() error {
    if err := w.writer.Close(); err != nil {
        w.file.Close()
        return fmt.Errorf("failed to close parquet writer: %w", err)
    }
    return w.file.Close()
}

// Path returns the file path.
func (w *Writer) Path() string { return w.path }
```

**Verify:** `go build ./internal/storage/cold/` compiles with no errors.

---

## TASK 04 — Create internal/storage/cold/reader.go

**Action:** Create `internal/storage/cold/reader.go`.

```go
package cold

import (
    "fmt"
    "io"
    "os"

    parquetgo "github.com/parquet-go/parquet-go"
)

// ReadFile reads all rows from a single Parquet file.
func ReadFile(path string) ([]ParquetRow, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, fmt.Errorf("failed to open parquet file %s: %w", path, err)
    }
    defer f.Close()

    fi, err := f.Stat()
    if err != nil {
        return nil, fmt.Errorf("failed to stat parquet file %s: %w", path, err)
    }

    reader := parquetgo.NewGenericReader[ParquetRow](f, fi.Size())
    defer reader.Close()

    var rows []ParquetRow
    buf := make([]ParquetRow, 1024)
    for {
        n, err := reader.Read(buf)
        if n > 0 {
            rows = append(rows, buf[:n]...)
        }
        if err == io.EOF {
            break
        }
        if err != nil {
            return nil, fmt.Errorf("failed to read parquet file %s: %w", path, err)
        }
    }
    return rows, nil
}

// ReadFileRange reads rows in [fromNs, toNs). Pass 0 for both to read all rows.
func ReadFileRange(path string, fromNs, toNs int64) ([]ParquetRow, error) {
    all, err := ReadFile(path)
    if err != nil {
        return nil, err
    }
    if fromNs == 0 && toNs == 0 {
        return all, nil
    }
    var filtered []ParquetRow
    for _, row := range all {
        if fromNs > 0 && row.TimestampNs < fromNs {
            continue
        }
        if toNs > 0 && row.TimestampNs >= toNs {
            continue
        }
        filtered = append(filtered, row)
    }
    return filtered, nil
}
```

**Verify:** `go build ./internal/storage/cold/` compiles with no errors.

---

## TASK 05 — Create internal/storage/cold/store.go

**Action:** Create `internal/storage/cold/store.go`.

```go
package cold

import (
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "sync"
    "sync/atomic"
    "time"
)

// Store manages all Parquet files in the cold tier.
// sync is used for sync.Mutex. sync/atomic is used for atomic.Int64.
type Store struct {
    rootDir      string
    mu           sync.Mutex
    recordsMoved atomic.Int64
    lastFlushAt  time.Time
    lastFlushDur time.Duration
}

// NewStore creates a cold tier Store rooted at rootDir.
// Creates subdirectories for each tierable data type.
func NewStore(rootDir string) (*Store, error) {
    for _, dt := range TierableDataTypes() {
        dir := filepath.Join(rootDir, dt)
        if err := os.MkdirAll(dir, 0755); err != nil {
            return nil, fmt.Errorf("failed to create cold dir %s: %w", dir, err)
        }
    }
    return &Store{rootDir: rootDir}, nil
}

// WriteRows writes a batch of rows for the given data type.
// refTs determines the date partition directory — use the oldest row's timestamp.
// Creates one new part file per call.
// Validates that dataType is a known tierable type.
func (s *Store) WriteRows(dataType string, rows []ParquetRow, refTs time.Time) error {
    if len(rows) == 0 {
        return nil
    }
    if !isValidDataType(dataType) {
        return fmt.Errorf("unknown cold tier data type: %q", dataType)
    }

    s.mu.Lock()
    defer s.mu.Unlock()

    partDir := filepath.Join(s.rootDir, DatePartitionDir(dataType, refTs))
    if err := os.MkdirAll(partDir, 0755); err != nil {
        return fmt.Errorf("failed to create partition dir: %w", err)
    }

    idx, err := s.nextPartIndex(partDir)
    if err != nil {
        return err
    }

    path := filepath.Join(partDir, PartFileName(idx))
    w, err := NewWriter(path)
    if err != nil {
        return err
    }

    for _, row := range rows {
        if err := w.WriteRow(row); err != nil {
            w.Close()
            // Remove partial file to avoid corrupt data
            os.Remove(path)
            return fmt.Errorf("failed to write row to %s: %w", path, err)
        }
    }
    if err := w.Close(); err != nil {
        os.Remove(path)
        return fmt.Errorf("failed to close parquet file %s: %w", path, err)
    }
    return nil
}

// ScanRows returns rows for the given data type in [fromNs, toNs), sorted by timestamp.
// Returns an error for unknown data types.
func (s *Store) ScanRows(dataType string, fromNs, toNs int64) ([]ParquetRow, error) {
    if !isValidDataType(dataType) {
        return nil, fmt.Errorf("unknown cold tier data type: %q", dataType)
    }

    dataDir := filepath.Join(s.rootDir, dataType)

    files, err := s.listParquetFiles(dataDir)
    if err != nil {
        return nil, err
    }

    var all []ParquetRow
    for _, f := range files {
        rows, err := ReadFileRange(f, fromNs, toNs)
        if err != nil {
            return nil, fmt.Errorf("failed to read %s: %w", f, err)
        }
        all = append(all, rows...)
    }

    sort.Slice(all, func(i, j int) bool {
        return all[i].TimestampNs < all[j].TimestampNs
    })
    return all, nil
}

// TotalParquetFiles returns the total Parquet file count across tierable data types.
// Errors are ignored for this best-effort health metric, so the count may be an
// undercount on filesystem problems.
func (s *Store) TotalParquetFiles() int {
    total := 0
    for _, dt := range TierableDataTypes() {
        files, err := s.listParquetFiles(filepath.Join(s.rootDir, dt))
        if err != nil {
            // Log is not available here — caller can check health separately
            continue
        }
        total += len(files)
    }
    return total
}

// AddRecordsMoved increments the moved counter.
func (s *Store) AddRecordsMoved(n int64) {
    s.recordsMoved.Add(n)
}

// SetLastFlush records time and duration of the most recent flush.
func (s *Store) SetLastFlush(at time.Time, dur time.Duration) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.lastFlushAt = at
    s.lastFlushDur = dur
}

// Stats returns a snapshot. Does not hold mu during file I/O to avoid deadlock.
func (s *Store) Stats() TierStats {
    s.mu.Lock()
    lastFlushAt := s.lastFlushAt
    lastFlushDur := s.lastFlushDur
    s.mu.Unlock()
    return TierStats{
        TotalParquetFiles: s.TotalParquetFiles(),
        TotalRecordsMoved: s.recordsMoved.Load(),
        LastFlushAt:       lastFlushAt,
        LastFlushDuration: lastFlushDur,
    }
}

// isValidDataType returns true if the data type is a known tierable type.
func isValidDataType(dt string) bool {
    for _, v := range TierableDataTypes() {
        if v == dt {
            return true
        }
    }
    return false
}

// IsTierableDataType returns true if the data type is eligible for cold tiering.
func IsTierableDataType(dt string) bool {
    return isValidDataType(dt)
}

// nextPartIndex returns the next available part file index in a directory.
func (s *Store) nextPartIndex(dir string) (int, error) {
    entries, err := os.ReadDir(dir)
    if err != nil {
        if os.IsNotExist(err) {
            return 1, nil
        }
        return 0, err
    }
    max := 0
    for _, e := range entries {
        if !strings.HasSuffix(e.Name(), ".parquet") {
            continue
        }
        var idx int
        if _, err := fmt.Sscanf(e.Name(), "part-%d.parquet", &idx); err == nil {
            if idx > max {
                max = idx
            }
        }
    }
    return max + 1, nil
}

// listParquetFiles returns all .parquet paths under dir recursively.
func (s *Store) listParquetFiles(dir string) ([]string, error) {
    var files []string
    err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        if info == nil {
            return nil
        }
        if !info.IsDir() && strings.HasSuffix(info.Name(), ".parquet") {
            files = append(files, path)
        }
        return nil
    })
    return files, err
}
```

**Verify:** `go build ./internal/storage/cold/` compiles with no errors.

---

## TASK 06 — Create internal/storage/cold/tier.go

**Action:** Create `internal/storage/cold/tier.go`.

```go
package cold

import (
    "fmt"
    "sync"
    "time"

    "go.uber.org/zap"

    "github.com/plomvix/plomvix/internal/config"
    "github.com/plomvix/plomvix/internal/logger"
    hotstore "github.com/plomvix/plomvix/internal/storage/hot"
)

// TieringEngine moves aged data from the hot tier to the cold tier.
// Only logs, metrics, and json are eligible — KV is excluded.
type TieringEngine struct {
    hot  *hotstore.Manager
    cold *Store
    cfg  *config.Config
    done chan struct{}
    once sync.Once // guards Stop() — prevents double-close panic
}

// NewTieringEngine creates a TieringEngine.
func NewTieringEngine(hot *hotstore.Manager, cold *Store, cfg *config.Config) *TieringEngine {
    return &TieringEngine{
        hot:  hot,
        cold: cold,
        cfg:  cfg,
        done: make(chan struct{}),
    }
}

// Start launches the background tiering goroutine.
// Runs Flush every hour. Call Stop() to shut it down.
func (e *TieringEngine) Start() {
    go func() {
        ticker := time.NewTicker(1 * time.Hour)
        defer ticker.Stop()
        for {
            select {
            case <-ticker.C:
                if err := e.Flush(); err != nil {
                    logger.Error("background tiering failed", zap.Error(err))
                }
            case <-e.done:
                return
            }
        }
    }()
}

// Stop signals the background goroutine to exit.
// Safe to call multiple times — uses sync.Once to prevent double-close panic.
func (e *TieringEngine) Stop() {
    e.once.Do(func() { close(e.done) })
}

// Flush moves all eligible hot tier data (logs, metrics, json) to the cold tier.
// Eligibility: timestamp < now - retention_days.
// Deletion failure is treated as a flush error to prevent hot+cold duplicates.
func (e *TieringEngine) Flush() error {
    start := time.Now()

    cutoffNs := time.Now().Add(
        -time.Duration(e.cfg.Storage.RetentionDays) * 24 * time.Hour,
    ).UnixNano()

    dataTypes := []struct {
        cf       string
        dataType string
    }{
        {hotstore.CFLogs, DataTypeLogs},
        {hotstore.CFMetrics, DataTypeMetrics},
        {hotstore.CFJSON, DataTypeJSON},
        // KV excluded: KV keys have no timestamp prefix
    }

    totalMoved := int64(0)
    for _, dt := range dataTypes {
        moved, err := e.flushDataType(dt.cf, dt.dataType, cutoffNs)
        if err != nil {
            return fmt.Errorf("flush failed for %s: %w", dt.dataType, err)
        }
        totalMoved += moved
    }

    dur := time.Since(start)
    e.cold.SetLastFlush(start, dur)
    e.cold.AddRecordsMoved(totalMoved)

    if totalMoved > 0 {
        logger.Info("tiering flush complete",
            zap.Int64("records_moved", totalMoved),
            zap.Duration("duration", dur),
        )
    } else {
        logger.Debug("tiering flush: no eligible records")
    }
    return nil
}

// flushDataType moves eligible records from one CF to cold storage.
// Computes refTs from the OLDEST row's timestamp for correct date partitioning.
// Deletion failure aborts the flush and returns an error.
func (e *TieringEngine) flushDataType(cf, dataType string, cutoffNs int64) (int64, error) {
    var rows []ParquetRow
    var keysToDelete [][]byte

    err := e.hot.ScanCFWithKeys(cf, 0, cutoffNs, func(key, payload []byte) bool {
        if len(key) < 8 {
            logger.Warn("skipping malformed hot tier key",
                zap.String("cf", cf),
                zap.Int("key_len", len(key)),
            )
            return true
        }

        rows = append(rows, ParquetRow{
            TimestampNs: int64(bigEndianUint64(key[:8])),
            Payload:     string(payload),
        })

        keyCopy := make([]byte, len(key))
        copy(keyCopy, key)
        keysToDelete = append(keysToDelete, keyCopy)
        return true
    })
    if err != nil {
        return 0, err
    }
    if len(rows) == 0 {
        return 0, nil
    }

    // Use the OLDEST row's timestamp for date partitioning — not the cutoff date.
    oldestNs := rows[0].TimestampNs
    for _, row := range rows[1:] {
        if row.TimestampNs < oldestNs {
            oldestNs = row.TimestampNs
        }
    }
    refTs := time.Unix(0, oldestNs).UTC()

    if err := e.cold.WriteRows(dataType, rows, refTs); err != nil {
        return 0, fmt.Errorf("failed to write %s to cold tier: %w", dataType, err)
    }

    // Delete from hot tier — failure is an error, not a warning.
    // This prevents the same record appearing in both tiers.
    for _, key := range keysToDelete {
        if err := e.hot.DeleteFromCF(cf, key); err != nil {
            return 0, fmt.Errorf("failed to delete tiered record from hot tier cf=%s: %w", cf, err)
        }
    }

    return int64(len(rows)), nil
}

// bigEndianUint64 decodes a big-endian uint64 from the first 8 bytes of b.
func bigEndianUint64(b []byte) uint64 {
    return uint64(b[0])<<56 | uint64(b[1])<<48 | uint64(b[2])<<40 | uint64(b[3])<<32 |
        uint64(b[4])<<24 | uint64(b[5])<<16 | uint64(b[6])<<8 | uint64(b[7])
}
```

**NOTE:** `tier.go` calls `ScanCFWithKeys` and `DeleteFromCF` on `*hotstore.Manager`.
These are added in TASK 07. Do TASK 07 before verifying TASK 06.

**Verify:** `go build ./internal/storage/cold/` compiles after TASK 07.

---

## TASK 07 — Add ScanCFWithKeys and DeleteFromCF to internal/storage/hot/manager.go

**Context:** The existing `ScanCF` method (line 145-161) passes only the payload (value) to its callback.
`ScanCFWithKeys` is a NEW method that passes both key AND payload — needed for tiering to delete keys after cold write.
These two methods serve different purposes and coexist without conflict.

**Action:** Add two methods to `internal/storage/hot/manager.go`.

```go
// ScanCFWithKeys iterates a CF in [fromNs, toNs) and calls fn with key and value.
// fromNs=0 scans from beginning. toNs=0 scans to current time.
func (m *Manager) ScanCFWithKeys(cf string, fromNs, toNs int64, fn func(key, payload []byte) bool) error {
    if toNs == 0 {
        toNs = time.Now().UnixNano()
    }
    return m.store.Scan(cf, BuildRangeScanPrefix(fromNs), func(key, value []byte) bool {
        if toNs > 0 && len(key) >= 8 {
            if int64(binary.BigEndian.Uint64(key[:8])) >= toNs {
                return false
            }
        }
        return fn(key, value)
    })
}

// DeleteFromCF deletes a record by exact key from the given column family.
func (m *Manager) DeleteFromCF(cf string, key []byte) error {
    return m.store.Delete(cf, key)
}
```

`"encoding/binary"` and `"time"` are already in manager.go from Sprint 6. No new imports needed.

**Verify:**
```bash
CGO_ENABLED=1 go build ./internal/storage/hot/
CGO_ENABLED=1 go build ./internal/storage/cold/   # now unblocked
```

---

## TASK 08 — Update query engine for cold tier

**Action:** Update `internal/query/engine.go`.

**Change 1 — Update Engine struct and NewEngine:**
```go
type Engine struct {
    store *hot.Manager
    cold  *cold.Store // nil = cold tier not available
}

// NewEngine creates a query Engine. cold may be nil for backward compatibility.
func NewEngine(store *hot.Manager, cold *cold.Store) *Engine {
    return &Engine{store: store, cold: cold}
}
```

**Change 2 — Update engine.go imports (existing imports kept, NEW are `"fmt"`, `"sort"`, `"cold"`):**
Current engine.go already imports `"encoding/json"`, `"time"`, `"ingestion"`, `"hot"`.
Add these NEW imports and the updated import block becomes:
```go
import (
    "encoding/json"
    "fmt"           // NEW - for error wrapping
    "sort"          // NEW - for sortByTimestamp
    "time"

    "github.com/plomvix/plomvix/internal/ingestion"
    "github.com/plomvix/plomvix/internal/storage/cold"  // NEW
    "github.com/plomvix/plomvix/internal/storage/hot"
)
```

**Change 3 — Replace queryTimeSeries with cold-aware version:**
```go
func (e *Engine) queryTimeSeries(cf, dataType string, params *QueryParams) (*QueryResult, error) {
    start := time.Now()
    var all []map[string]interface{}

    // Hot tier scan
    err := e.store.ScanCF(cf, params.FromNs, params.ToNs, func(raw []byte) bool {
        record := DecodePayload(raw)
        if record == nil {
            return true
        }
        if ApplyFilters(record, params.Filters) {
            all = append(all, record)
        }
        return true
    })
    if err != nil {
        return nil, fmt.Errorf("hot tier scan failed: %w", err)
    }

    // Cold tier scan — only for tierable types (not kv)
    if e.cold != nil && cold.IsTierableDataType(dataType) {
        coldRows, err := e.cold.ScanRows(dataType, params.FromNs, params.ToNs)
        if err != nil {
            return nil, fmt.Errorf("cold tier scan failed: %w", err)
        }
        for _, row := range coldRows {
            record := DecodePayload([]byte(row.Payload))
            if record == nil {
                continue
            }
            if ApplyFilters(record, params.Filters) {
                all = append(all, record)
            }
        }
    }

    // Sort by TimestampNs from the ParquetRow / ingestion Timestamp field.
    // Records are sorted by "timestamp" JSON field which all ingest handlers set.
    sortByTimestamp(all)

    total := len(all)
    start2 := params.Offset
    if start2 > total {
        start2 = total
    }
    end := start2 + params.Limit
    if end > total {
        end = total
    }
    page := all[start2:end]
    if page == nil {
        page = []map[string]interface{}{}
    }

    return &QueryResult{
        Records:  page,
        Count:    len(page),
        Total:    total,
        Limit:    params.Limit,
        Offset:   params.Offset,
        QueryMs:  time.Since(start).Milliseconds(),
        DataType: dataType,
    }, nil
}
```

**Change 4 — Add sortByTimestamp helper:**
```go
// sortByTimestamp sorts records by the "timestamp" JSON field (set by ingest handlers).
// Records without a numeric "timestamp" field sort to the end.
func sortByTimestamp(records []map[string]interface{}) {
    sort.SliceStable(records, func(i, j int) bool {
        ti, iok := records[i]["timestamp"].(float64)
        tj, jok := records[j]["timestamp"].(float64)
        if !iok {
            return false
        }
        if !jok {
            return true
        }
        return ti < tj
    })
}
```

**Note on existing engine tests:** `engine_test.go` from Sprint 6 calls `NewEngine(m)` with
one argument. Update all calls in `engine_test.go` to `NewEngine(m, nil)` — passing nil
cold store preserves existing test behaviour.

**Verify:** `CGO_ENABLED=1 go build ./internal/query/` compiles with no errors.

---

## TASK 09 — Update internal/server/server.go

**Action:** Six targeted changes to `internal/server/server.go`.

**Change 1 — Add cold fields to Server struct:**
```go
type Server struct {
    // ...existing fields...
    cold       *coldstore.Store         // ← ADD
    tierEngine *coldstore.TieringEngine // ← ADD
}
```

**Import alias to add:**
```go
coldstore "github.com/plomvix/plomvix/internal/storage/cold"
```

**Change 2 — Update New() signature:**
```go
func New(cfg *config.Config, version string, store *auth.Store,
    blacklist *auth.Blacklist, wal *walmanager.Manager,
    hotTier *hotmanager.Manager, cold *coldstore.Store,
    tierEngine *coldstore.TieringEngine) *Server {
    s := &Server{
        // ...existing assignments...
        cold:       cold,
        tierEngine: tierEngine,
    }
    // ...rest unchanged...
}
```

**Change 3 — Update setupRoutes() to pass cold to query engine:**
```go
// Replace:
queryEngine := query.NewEngine(s.hotTier)
// With:
queryEngine := query.NewEngine(s.hotTier, s.cold)
```

**Change 4 — Add tier flush route inside admin route group:**
```go
r.Post("/admin/tier/flush", s.handleTierFlush)
```

**Change 5 — Add handleTierFlush handler with nil guard:**
```go
// handleTierFlush handles POST /admin/tier/flush.
//
// POST /admin/tier/flush
// Auth: admin only
//
// Responses:
//   200 OK       — flush complete, returns stats
//   500 Internal — INTERNAL_ERROR: flush failed
func (s *Server) handleTierFlush(w http.ResponseWriter, r *http.Request) {
    if s.tierEngine == nil || s.cold == nil {
        utils.InternalError(w, r, "tier engine not available")
        return
    }
    if err := s.tierEngine.Flush(); err != nil {
        utils.InternalError(w, r, "tier flush failed")
        return
    }
    stats := s.cold.Stats()
    utils.OK(w, r, map[string]interface{}{
        "message":        "tier flush complete",
        "records_moved":  stats.TotalRecordsMoved,
        "parquet_files":  stats.TotalParquetFiles,
        "last_flush_at":  stats.LastFlushAt,
        "flush_duration": stats.LastFlushDuration.String(),
    })
}
```

**Change 6 — Update handleHealth with nil guard and cold stats:**
```go
// Add cold stats to the health response map — with nil guard:
var coldData map[string]interface{}
if s.cold != nil {
    cs := s.cold.Stats()
    coldData = map[string]interface{}{
        "parquet_files": cs.TotalParquetFiles,
        "records_moved": cs.TotalRecordsMoved,
        "last_flush_at": cs.LastFlushAt,
    }
}
// Include in utils.OK response map:
"cold": coldData,
```

**Verify:** `CGO_ENABLED=1 go build ./internal/server/` compiles with no errors.

---

## TASK 10 — Update cmd/plomvix/main.go

**Action:** Three targeted changes to `cmd/plomvix/main.go`.

**Change 1 — Add import:**
```go
coldstore "github.com/plomvix/plomvix/internal/storage/cold"
```

**Change 2 — Insert after `defer hotTier.Close()` and before `srv := server.New(...)`:**
```go
// Open cold tier store
coldPath := filepath.Join(cfg.Storage.DataDir, "cold")
coldTier, err := coldstore.NewStore(coldPath)
if err != nil {
    hotTier.Close()
    wal.Close()
    logger.Error("failed to open cold tier", zap.Error(err))
    os.Exit(1)
}
// coldTier holds no file handles — no defer needed

// Create and start tiering engine
tierEngine := coldstore.NewTieringEngine(hotTier, coldTier, cfg)
tierEngine.Start()
defer tierEngine.Stop()
```

**Change 3 — Update server.New() call:**
```go
srv := server.New(cfg, Version, store, blacklist, wal, hotTier, coldTier, tierEngine)
```

**Verify:** `CGO_ENABLED=1 go build ./cmd/plomvix/` compiles with no errors.

---

## TASK 11 — Create internal/storage/cold/cold_test.go

**Action:** Create `internal/storage/cold/cold_test.go`.

```go
package cold

import (
    "path/filepath"
    "strings"
    "testing"
    "time"
)

func newTestStore(t *testing.T) *Store {
    t.Helper()
    store, err := NewStore(filepath.Join(t.TempDir(), "cold"))
    if err != nil {
        t.Fatalf("NewStore failed: %v", err)
    }
    return store
}

func TestWriteAndReadRows(t *testing.T) {
    s := newTestStore(t)
    now := time.Now()
    rows := []ParquetRow{
        {TimestampNs: now.UnixNano(), Payload: `{"level":"info","message":"hello"}`},
        {TimestampNs: now.Add(time.Second).UnixNano(), Payload: `{"level":"warn","message":"world"}`},
    }
    if err := s.WriteRows(DataTypeLogs, rows, now); err != nil {
        t.Fatalf("WriteRows failed: %v", err)
    }
    result, err := s.ScanRows(DataTypeLogs, 0, 0)
    if err != nil {
        t.Fatalf("ScanRows failed: %v", err)
    }
    if len(result) != 2 {
        t.Errorf("got %d rows, want 2", len(result))
    }
    // Verify payload content — not just count
    if result[0].Payload != rows[0].Payload {
        t.Errorf("payload[0] = %q, want %q", result[0].Payload, rows[0].Payload)
    }
    if result[1].Payload != rows[1].Payload {
        t.Errorf("payload[1] = %q, want %q", result[1].Payload, rows[1].Payload)
    }
    // Verify timestamps are preserved
    if result[0].TimestampNs != rows[0].TimestampNs {
        t.Errorf("TimestampNs[0] = %d, want %d", result[0].TimestampNs, rows[0].TimestampNs)
    }
}

func TestWriteEmptyRows(t *testing.T) {
    s := newTestStore(t)
    if err := s.WriteRows(DataTypeLogs, nil, time.Now()); err != nil {
        t.Errorf("WriteRows nil rows should not error: %v", err)
    }
}

func TestWriteUnknownDataType(t *testing.T) {
    s := newTestStore(t)
    err := s.WriteRows("kv", []ParquetRow{{TimestampNs: 1, Payload: "{}"}}, time.Now())
    if err == nil {
        t.Error("expected error for unknown data type 'kv', got nil")
    }
}

func TestScanUnknownDataType(t *testing.T) {
    s := newTestStore(t)
    _, err := s.ScanRows("kv", 0, 0)
    if err == nil {
        t.Error("expected error for unknown data type 'kv', got nil")
    }
}

func TestScanRowsTimeRange(t *testing.T) {
    s := newTestStore(t)
    base := time.Now()
    rows := []ParquetRow{
        {TimestampNs: base.UnixNano(), Payload: `{"i":0}`},
        {TimestampNs: base.Add(time.Second).UnixNano(), Payload: `{"i":1}`},
        {TimestampNs: base.Add(2 * time.Second).UnixNano(), Payload: `{"i":2}`},
    }
    s.WriteRows(DataTypeLogs, rows, base)

    // [base, base+2s) → rows 0 and 1 only
    result, err := s.ScanRows(DataTypeLogs, base.UnixNano(), base.Add(2*time.Second).UnixNano())
    if err != nil {
        t.Fatalf("ScanRows failed: %v", err)
    }
    if len(result) != 2 {
        t.Errorf("got %d rows, want 2", len(result))
    }
}

func TestScanRowsOrdering(t *testing.T) {
    s := newTestStore(t)
    now := time.Now()
    // Write two part files — second has earlier timestamp
    s.WriteRows(DataTypeLogs, []ParquetRow{{TimestampNs: now.Add(time.Second).UnixNano(), Payload: `{"seq":2}`}}, now)
    s.WriteRows(DataTypeLogs, []ParquetRow{{TimestampNs: now.UnixNano(), Payload: `{"seq":1}`}}, now)

    result, err := s.ScanRows(DataTypeLogs, 0, 0)
    if err != nil {
        t.Fatalf("ScanRows failed: %v", err)
    }
    if len(result) != 2 {
        t.Fatalf("got %d rows, want 2", len(result))
    }
    // Should be sorted ascending
    if result[0].TimestampNs > result[1].TimestampNs {
        t.Errorf("rows not sorted ascending: [0]=%d [1]=%d", result[0].TimestampNs, result[1].TimestampNs)
    }
}

func TestPartFileName(t *testing.T) {
    if got := PartFileName(1); got != "part-000001.parquet" {
        t.Errorf("got %q, want part-000001.parquet", got)
    }
    if got := PartFileName(42); got != "part-000042.parquet" {
        t.Errorf("got %q, want part-000042.parquet", got)
    }
}

func TestDatePartitionDir(t *testing.T) {
    ts := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
    got := DatePartitionDir(DataTypeLogs, ts)
    if got != "logs/2024-01-15" {
        t.Errorf("got %q, want logs/2024-01-15", got)
    }
}

func TestWriteRowsCreatesDatePartition(t *testing.T) {
    s := newTestStore(t)
    ts := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
    row := ParquetRow{TimestampNs: ts.UnixNano(), Payload: `{}`}
    if err := s.WriteRows(DataTypeLogs, []ParquetRow{row}, ts); err != nil {
        t.Fatalf("WriteRows failed: %v", err)
    }
    // Verify file is under logs/2024-01-15/
    files, _ := s.listParquetFiles(filepath.Join(s.rootDir, "logs"))
    if len(files) != 1 {
        t.Fatalf("expected 1 file, got %d", len(files))
    }
    if !strings.Contains(files[0], "2024-01-15") {
        t.Errorf("file not under date partition: %s", files[0])
    }
    if !strings.HasSuffix(files[0], "part-000001.parquet") {
        t.Errorf("unexpected filename: %s", files[0])
    }
}

func TestTotalParquetFiles(t *testing.T) {
    s := newTestStore(t)
    if s.TotalParquetFiles() != 0 {
        t.Error("expected 0 files on fresh store")
    }
    s.WriteRows(DataTypeLogs, []ParquetRow{{TimestampNs: 1, Payload: "{}"}}, time.Now())
    if s.TotalParquetFiles() != 1 {
        t.Errorf("expected 1 file after write, got %d", s.TotalParquetFiles())
    }
}

func TestMultiplePartFiles(t *testing.T) {
    s := newTestStore(t)
    now := time.Now()
    s.WriteRows(DataTypeLogs, []ParquetRow{{TimestampNs: 1, Payload: `{"i":1}`}}, now)
    s.WriteRows(DataTypeLogs, []ParquetRow{{TimestampNs: 2, Payload: `{"i":2}`}}, now)
    if s.TotalParquetFiles() != 2 {
        t.Errorf("expected 2 part files, got %d", s.TotalParquetFiles())
    }
    rows, err := s.ScanRows(DataTypeLogs, 0, 0)
    if err != nil {
        t.Fatalf("ScanRows failed: %v", err)
    }
    if len(rows) != 2 {
        t.Errorf("expected 2 rows, got %d", len(rows))
    }
}
```

**Verify:** `go test -race ./internal/storage/cold/` — all tests pass.

---

## TASK 12 — Create internal/storage/cold/tier_test.go

**Action:** Create `internal/storage/cold/tier_test.go`.
This tests the tiering engine integration — the core Sprint 7 feature.

```go
package cold

import (
    "path/filepath"
    "strings"
    "testing"
    "time"

    "github.com/plomvix/plomvix/internal/config"
    hotstore "github.com/plomvix/plomvix/internal/storage/hot"
)

func newTestHot(t *testing.T) *hotstore.Manager {
    t.Helper()
    dir := filepath.Join(t.TempDir(), "hot")
    cfg := &config.Config{Storage: config.StorageConfig{DataDir: dir}}
    m, err := hotstore.Open(dir, cfg)
    if err != nil {
        t.Fatalf("hot.Open failed: %v", err)
    }
    t.Cleanup(func() { m.Close() })
    return m
}

func newTestTierEngine(t *testing.T, retentionDays int) (*TieringEngine, *hotstore.Manager, *Store) {
    t.Helper()
    hot := newTestHot(t)
    cold, err := NewStore(filepath.Join(t.TempDir(), "cold"))
    if err != nil {
        t.Fatalf("NewStore failed: %v", err)
    }
    cfg := &config.Config{
        Storage: config.StorageConfig{RetentionDays: retentionDays},
    }
    engine := NewTieringEngine(hot, cold, cfg)
    return engine, hot, cold
}


func TestFlushMovesOldRecords(t *testing.T) {
    engine, hot, cold := newTestTierEngine(t, 0) // retention_days=0: everything is eligible

    // Write an old log record directly to hot tier via store
    oldTs := time.Now().Add(-1 * time.Hour)
    payload := []byte(`{"level":"info","message":"old record"}`)
    if err := hot.WriteLog(oldTs.UnixNano(), payload); err != nil {
        t.Fatalf("WriteLog failed: %v", err)
    }

    // Flush — retention_days=0 means all records are eligible
    if err := engine.Flush(); err != nil {
        t.Fatalf("Flush failed: %v", err)
    }

    // Verify cold tier has the record
    rows, err := cold.ScanRows(DataTypeLogs, 0, 0)
    if err != nil {
        t.Fatalf("cold ScanRows failed: %v", err)
    }
    if len(rows) != 1 {
        t.Errorf("cold tier has %d rows, want 1", len(rows))
    }
    if len(rows) > 0 && rows[0].Payload != string(payload) {
        t.Errorf("payload mismatch: got %q, want %q", rows[0].Payload, payload)
    }

    // Verify stats updated
    if cold.Stats().TotalRecordsMoved != 1 {
        t.Errorf("TotalRecordsMoved = %d, want 1", cold.Stats().TotalRecordsMoved)
    }
}

func TestFlushDoesNotMoveNewRecords(t *testing.T) {
    engine, hot, _ := newTestTierEngine(t, 30) // retention_days=30

    // Write a recent record — not eligible
    if err := hot.WriteLog(time.Now().UnixNano(), []byte(`{"level":"info"}`)); err != nil {
        t.Fatalf("WriteLog failed: %v", err)
    }

    if err := engine.Flush(); err != nil {
        t.Fatalf("Flush failed: %v", err)
    }

    // Record should still be in hot tier
    var found int
    hot.ScanCF(hotstore.CFLogs, 0, time.Now().Add(time.Minute).UnixNano(), func([]byte) bool {
        found++
        return true
    })
    if found != 1 {
        t.Errorf("hot tier should still have 1 record after flush, got %d", found)
    }
}

func TestFlushExcludesKV(t *testing.T) {
    engine, hot, cold := newTestTierEngine(t, 0) // retention_days=0

    // Write a KV record — should NOT be tiered
    if err := hot.WriteKV("mykey", []byte(`{"key":"mykey","value":"val"}`)); err != nil {
        t.Fatalf("WriteKV failed: %v", err)
    }

    if err := engine.Flush(); err != nil {
        t.Fatalf("Flush failed: %v", err)
    }

    // Cold tier should have zero records (KV excluded)
    if cold.Stats().TotalRecordsMoved != 0 {
        t.Errorf("KV should not be tiered, got TotalRecordsMoved=%d", cold.Stats().TotalRecordsMoved)
    }
    // KV still in hot tier
    val, err := hot.GetKV("mykey")
    if err != nil || val == nil {
        t.Errorf("KV should still be in hot tier: err=%v val=%v", err, val)
    }
}

func TestFlushPartitionsByOldestRecord(t *testing.T) {
    engine, hot, cold := newTestTierEngine(t, 0)

    // Write record with a specific old date
    specificDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
    if err := hot.WriteLog(specificDate.UnixNano(), []byte(`{"date":"2024-01-15"}`)); err != nil {
        t.Fatalf("WriteLog failed: %v", err)
    }

    if err := engine.Flush(); err != nil {
        t.Fatalf("Flush failed: %v", err)
    }

    // Partition dir should be based on 2024-01-15, not today
    files, _ := cold.listParquetFiles(filepath.Join(cold.rootDir, "logs"))
    if len(files) == 0 {
        t.Fatal("no parquet files found after flush")
    }
    for _, f := range files {
        if !strings.Contains(f, "2024-01-15") {
            t.Errorf("expected 2024-01-15 in path, got %s", f)
        }
    }
}

func TestStopIsIdempotent(t *testing.T) {
    engine, _, _ := newTestTierEngine(t, 30)
    engine.Start()
    engine.Stop()
    engine.Stop() // must not panic
}

```

**Final imports for `tier_test.go`:**
```go
import (
    "path/filepath"
    "strings"
    "testing"
    "time"

    "github.com/plomvix/plomvix/internal/config"
    hotstore "github.com/plomvix/plomvix/internal/storage/hot"
)
```

Do not include `bytes`, `encoding/binary`, `makeTimestampKey`, or `containsString`.
Use `strings.Contains` directly in `TestFlushPartitionsByOldestRecord`.

**Verify:** `CGO_ENABLED=1 go test -race ./internal/storage/cold/` — all tests pass.

---

## TASK 13 — Add hot+cold query integration test to internal/query/engine_test.go

**Action:** Add these tests to the existing `internal/query/engine_test.go`.

Also update existing `newTestEngine` call to pass nil cold:
```go
// In newTestEngine helper — update NewEngine call:
return NewEngine(m)
// To:
return NewEngine(m, nil)
```

**New test helper:**
```go
func newTestEngineWithCold(t *testing.T) (*Engine, *hotstore.Manager) {
    t.Helper()
    dir := t.TempDir()
    cfg := &config.Config{Storage: config.StorageConfig{DataDir: dir}}

    hotDir := filepath.Join(dir, "hot")
    m, err := hot.Open(hotDir, cfg)
    if err != nil {
        t.Fatalf("hot.Open failed: %v", err)
    }
    t.Cleanup(func() { m.Close() })

    coldDir := filepath.Join(dir, "cold")
    cs, err := cold.NewStore(coldDir)
    if err != nil {
        t.Fatalf("cold.NewStore failed: %v", err)
    }

    return NewEngine(m, cs), m
}
```

**New tests to add:**
```go
func TestQueryLogsHotAndCold(t *testing.T) {
    e, hotMgr := newTestEngineWithCold(t)
    base := time.Now().UnixNano()

    // Write 1 record to hot tier directly
    hotMgr.WriteLog(base, []byte(`{"level":"info","message":"hot record","timestamp":1}`))

    // Write 1 record to cold tier directly via engine's cold store
    e.cold.WriteRows(cold.DataTypeLogs, []cold.ParquetRow{
        {TimestampNs: base - 1000, Payload: `{"level":"warn","message":"cold record","timestamp":0}`},
    }, time.Now())

    params := &QueryParams{
        FromNs: base - 2000,
        ToNs:   base + 1000,
        Limit:  DefaultLimit,
    }
    result, err := e.QueryLogs(params)
    if err != nil {
        t.Fatalf("QueryLogs failed: %v", err)
    }
    if result.Total != 2 {
        t.Errorf("total = %d, want 2 (1 hot + 1 cold)", result.Total)
    }
}

func TestQueryLogsColdOnly(t *testing.T) {
    e, _ := newTestEngineWithCold(t)
    base := time.Now().UnixNano()

    // Only cold tier has data
    e.cold.WriteRows(cold.DataTypeLogs, []cold.ParquetRow{
        {TimestampNs: base, Payload: `{"level":"info","message":"cold only","timestamp":1}`},
    }, time.Now())

    params := &QueryParams{FromNs: base - 1, ToNs: base + 1, Limit: DefaultLimit}
    result, err := e.QueryLogs(params)
    if err != nil {
        t.Fatalf("QueryLogs failed: %v", err)
    }
    if result.Total != 1 {
        t.Errorf("total = %d, want 1", result.Total)
    }
}

func TestQueryLogsNilCold(t *testing.T) {
    // Engine with nil cold store should not panic and return only hot results
    e := newTestEngine(t) // uses nil cold
    params := &QueryParams{ToNs: time.Now().UnixNano(), Limit: DefaultLimit}
    result, err := e.QueryLogs(params)
    if err != nil {
        t.Fatalf("QueryLogs with nil cold failed: %v", err)
    }
    if result == nil {
        t.Error("result should not be nil")
    }
}
```

**Ensure `engine_test.go` imports include these packages if not already present:**
```go
"path/filepath"
"testing"
"time"

"github.com/plomvix/plomvix/internal/config"
"github.com/plomvix/plomvix/internal/storage/cold"
hotstore "github.com/plomvix/plomvix/internal/storage/hot"
```

**Verify:** `CGO_ENABLED=1 go test -race ./internal/query/` — all tests pass.

---

## TASK 14 — Create docs/api/tier.md

**Action:** Create `docs/api/tier.md`.

```markdown
# Plomvix Tier API Reference

The tiering system automatically moves aged data from the hot tier (RocksDB)
to the cold tier (Parquet files) based on the `retention_days` config value.

**Tierable data types:** logs, metrics, json.
**KV is not tiered in Sprint 7** — KV keys have no timestamp prefix.
KV data always stays in the hot tier.

---

## POST /admin/tier/flush

Triggers an immediate cold tier flush outside the hourly background schedule.
Moves all eligible hot tier data (older than `retention_days`) to Parquet files.

**Auth:** Admin only

**Request body:** none

**Response 200:**
```json
{
  "status": "ok",
  "data": {
    "message": "tier flush complete",
    "records_moved": 1500,
    "parquet_files": 3,
    "last_flush_at": "2024-01-15T10:30:00Z",
    "flush_duration": "1.23s"
  },
  "request_id": "uuid"
}
```

**Response 500:** flush failed — check server logs for details.

**curl example:**
```bash
curl -X POST http://localhost:8080/admin/tier/flush \
  -H "Authorization: Bearer <token>"
```

---

## Health Endpoint — Cold Tier Stats

```json
{
  "status": "ok",
  "data": {
    "cold": {
      "parquet_files": 3,
      "records_moved": 1500,
      "last_flush_at": "2024-01-15T10:30:00Z"
    }
  }
}
```

| Field | Type | Description |
|---|---|---|
| `cold.parquet_files` | int | Total Parquet files currently on disk |
| `cold.records_moved` | int64 | Records moved to cold tier this process lifetime (resets on restart) |
| `cold.last_flush_at` | time | Timestamp of most recent flush |

---

## Configuration

```yaml
storage:
  retention_days: 30   # data older than this moves to cold tier
```

---

## Cold Tier Directory Structure

```
data/cold/
├── logs/YYYY-MM-DD/part-000001.parquet
├── metrics/YYYY-MM-DD/part-000001.parquet
└── json/YYYY-MM-DD/part-000001.parquet
```

Date partition is based on the **oldest record's timestamp** in each flush batch (UTC).
KV has no cold tier directory.

---

## Query Behaviour

GET /query/logs, GET /query/metrics, GET /query/json automatically search
both hot and cold tiers. Results are merged and sorted by timestamp ascending
before pagination.

GET /query/kv/{key} searches hot tier only — KV is not tiered in Sprint 7.

---

## Tiering Behaviour

- Background flush runs every hour automatically.
- Each flush moves all records older than `retention_days` from RocksDB to Parquet.
- Records are deleted from RocksDB after successful cold write.
- If any deletion fails, the flush returns an error immediately. Since deletion
  happens after the Parquet write and key-by-key, partial hot+cold state is possible
  on failure. Operators should retry the flush or run reconciliation before assuming
  duplicate-free query results.
- `records_moved` in health reflects process lifetime only — resets on restart.
```

**Verify:** `cat docs/api/tier.md` shows full content.

---

## TASK 15 — Full build and smoke test

**Action:**

**Environment override verified:** Config uses Viper with `SetEnvPrefix("PLOMVIX")` and
`SetEnvKeyReplacer(strings.NewReplacer(".", "_"))`, so `PLOMVIX_STORAGE_RETENTION_DAYS=0`
is supported. No fallback config file needed.

```bash
#!/bin/bash
set -euo pipefail

echo "=== Clearing stale data ==="
rm -rf data/hot/ data/wal/ data/cold/

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
echo "=== Step 3: Boot server with retention_days=0 to enable flush in smoke test ==="
# Override retention_days so fresh records are immediately eligible for tiering
PLOMVIX_STORAGE_RETENTION_DAYS=0 ./plomvix > /tmp/plomvix_s7.log 2>&1 &
SERVER_PID=$!
sleep 3

echo ""
echo "=== Step 4: Login ==="
TOKEN=$(curl -sf -X POST http://localhost:8080/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"changeme"}' | jq -r '.data.token')
echo "Token acquired"

echo ""
echo "=== Step 5: Ingest test data ==="
curl -sf -X POST http://localhost:8080/ingest/logs \
    -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
    -d '{"records":[{"level":"info","message":"tier smoke test"}]}' > /dev/null
echo "Data ingested"

echo ""
echo "=== Step 6: Health includes cold stats ==="
HEALTH=$(curl -sf http://localhost:8080/health)
echo "$HEALTH" | jq '.data.cold' | grep -q "parquet_files" \
    && echo "PASS: cold block in health" \
    || { echo "FAIL: cold block missing"; exit 1; }

echo ""
echo "=== Step 7: Manual flush with retention_days=0 actually moves records ==="
RESP=$(curl -sf -X POST http://localhost:8080/admin/tier/flush \
    -H "Authorization: Bearer $TOKEN")
echo "$RESP" | jq .
MOVED=$(echo "$RESP" | jq '.data.records_moved')
[ "$MOVED" -ge 1 ] \
    && echo "PASS: records_moved=$MOVED >= 1" \
    || { echo "FAIL: records_moved=$MOVED, want >= 1"; exit 1; }

echo ""
echo "=== Step 8: Cold parquet file created ==="
FILES=$(echo "$RESP" | jq '.data.parquet_files')
[ "$FILES" -ge 1 ] \
    && echo "PASS: parquet_files=$FILES" \
    || { echo "FAIL: parquet_files=$FILES, want >= 1"; exit 1; }

echo ""
echo "=== Step 9: Query logs still returns data after flush (from cold tier) ==="
RESP=$(curl -sf "http://localhost:8080/query/logs" \
    -H "Authorization: Bearer $TOKEN")
TOTAL=$(echo "$RESP" | jq '.data.total')
[ "$TOTAL" -ge 1 ] \
    && echo "PASS: query after flush returns $TOTAL records" \
    || { echo "FAIL: query returned 0 after flush"; exit 1; }

echo ""
echo "=== Step 10: Tier flush requires auth ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/admin/tier/flush)
[ "$STATUS" -eq 401 ] && echo "PASS: no auth → 401" \
    || { echo "FAIL: expected 401, got $STATUS"; exit 1; }

echo ""
echo "=== Step 11: Graceful shutdown ==="
kill -SIGTERM "$SERVER_PID"
wait "$SERVER_PID"
EXIT_CODE=$?
SERVER_PID=""
[ "$EXIT_CODE" -eq 0 ] && echo "PASS: clean shutdown" \
    || { echo "FAIL: exit code $EXIT_CODE"; exit 1; }

echo ""
echo "================================================"
echo "  ALL STEPS PASSED — Sprint 7 smoke test DONE  "
echo "================================================"
```

| Step | Verified | Expected |
|---|---|---|
| 1 | Build + vet | No errors |
| 2 | All tests | Pass with race detector |
| 3 | Boot with retention_days=0 | Server starts |
| 4 | Login | JWT returned |
| 5 | Ingest | Log record written |
| 6 | Health | cold block present |
| 7 | Flush | records_moved >= 1 (not 0) |
| 8 | Parquet file | parquet_files >= 1 |
| 9 | Query after flush | Returns cold tier data |
| 10 | Auth check | 401 without token |
| 11 | Shutdown | Exit code 0 |

---

## EXECUTION ORDER SUMMARY

```
TASK 01  →  go get parquet-go
TASK 02  →  internal/storage/cold/types.go
TASK 03  →  internal/storage/cold/writer.go
TASK 04  →  internal/storage/cold/reader.go
TASK 05  →  internal/storage/cold/store.go
TASK 06  →  internal/storage/cold/tier.go
TASK 07  →  internal/storage/hot/manager.go (ScanCFWithKeys, DeleteFromCF)
            ← verify TASK 06 after this
TASK 08  →  internal/query/engine.go (cold search + sortByTimestamp)
            ← also update existing NewEngine(m) calls to NewEngine(m, nil) in engine_test.go
TASK 09  →  internal/server/server.go (struct, New(), routes, health, nil guards)
TASK 10  →  cmd/plomvix/main.go (cold init, tiering engine)
TASK 11  →  internal/storage/cold/cold_test.go
TASK 12  →  internal/storage/cold/tier_test.go
TASK 13  →  internal/query/engine_test.go (add hot+cold integration tests)
TASK 14  →  docs/api/tier.md
TASK 15  →  smoke test — all 11 steps must pass
```

---

## FUTURE WORK (Sprint 8+)

- **Reconciliation tooling:** Detect and fix partial flush failures automatically
- **Parquet compaction:** Merge multiple small part files in same partition
- **Partition pruning:** Skip scanning partitions outside query time range
- **Cold tier metrics:** Per-partition size, record counts, oldest/newest timestamps
- **KV tiering:** Design timestamp-independent tiering policy for KV data

---

*Sprint 7 complete when TASK 15 passes with zero failures.*
*Plomvix — Built in India. Built for the world.*