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
