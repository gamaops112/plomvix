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
