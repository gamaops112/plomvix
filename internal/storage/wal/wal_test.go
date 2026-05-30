package wal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/plomvix/plomvix/internal/config"
)

func newTestWALDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "wal")
}

func testWALConfig(dataDir string) *config.Config {
	return &config.Config{
		Storage: config.StorageConfig{
			DataDir:           dataDir,
			WALFlushThreshold: 64 * 1024 * 1024,
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
	w, err := NewWriter(dir, 100)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

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
	for i, e := range entries {
		if e.SeqID != uint64(i+1) {
			t.Errorf("entries[%d].SeqID = %d, want %d", i, e.SeqID, i+1)
		}
	}
}

func TestRecoveryAfterReopen(t *testing.T) {
	dir := newTestWALDir(t)

	w, _ := NewWriter(dir, 64*1024*1024)
	for i := 0; i < 3; i++ {
		w.Write(DataTypeLog, []byte(`{}`))
	}
	w.Close()

	w2, err := NewWriter(dir, 64*1024*1024)
	if err != nil {
		t.Fatalf("NewWriter reopen failed: %v", err)
	}
	defer w2.Close()

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

	m, err := Open(dir, cfg)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	m.Write(DataTypeLog, []byte(`{"a":1}`))
	m.Write(DataTypeKV, []byte(`{"b":2}`))
	m.Close()

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
	smallCfg := &config.Config{Storage: config.StorageConfig{
		DataDir:           dir,
		WALFlushThreshold: 100,
	}}

	m, err := Open(dir, smallCfg)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	for i := 0; i < 5; i++ {
		m.Write(DataTypeLog, []byte(`{"fill":"padding-data-to-force-rotation"}`))
	}
	m.Close()

	m2, err := Open(dir, smallCfg)
	if err != nil {
		t.Fatalf("Open (reopen) failed: %v", err)
	}
	defer m2.Close()

	if err := m2.DeleteSegment(1); err != nil {
		t.Fatalf("DeleteSegment(1) failed: %v", err)
	}

	seg1Path := filepath.Join(dir, SegmentFileName(1))
	if _, err := os.Stat(seg1Path); !os.IsNotExist(err) {
		t.Errorf("segment 1 still exists after DeleteSegment")
	}
}
