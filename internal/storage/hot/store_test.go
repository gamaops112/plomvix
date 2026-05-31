package hot

import (
	"bytes"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/plomvix/plomvix/internal/config"
)

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

func TestStoreOpenClose(t *testing.T) {
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
