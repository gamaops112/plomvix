package hot

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/plomvix/plomvix/internal/config"
	walstore "github.com/plomvix/plomvix/internal/storage/wal"
)

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

	for i := 0; i < 3; i++ {
		m.WriteLog(base+int64(i)*1000, []byte(`{"seq":"log"}`))
	}

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
