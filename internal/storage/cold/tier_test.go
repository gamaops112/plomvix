package cold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/plomvix/plomvix/internal/config"
	"github.com/plomvix/plomvix/internal/logger"
	hotstore "github.com/plomvix/plomvix/internal/storage/hot"
)

func TestMain(m *testing.M) {
	if err := logger.Init("info", "pretty"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

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
