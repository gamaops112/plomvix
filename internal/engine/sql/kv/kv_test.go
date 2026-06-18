package kv

import (
	"context"
	"os"
	"testing"

	"github.com/plomvix/plomvix/internal/engine/sql/key"
	"github.com/plomvix/plomvix/internal/storage/pager"
)

// newTestKVStore creates a KVStore with an open pager for testing.
func newTestKVStore(t *testing.T) (KVStore, func()) {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/kv_test.pager"
	pg := pager.New(path)
	ctx := context.Background()
	if err := pg.Open(ctx); err != nil {
		t.Fatalf("pager.Open: %v", err)
	}
	kv := New(pg)
	cleanup := func() {
		kv.Close(ctx)
		pg.Close(ctx)
	}
	return kv, cleanup
}

func TestKVStore_Open(t *testing.T) {
	kv, cleanup := newTestKVStore(t)
	defer cleanup()

	ctx := context.Background()
	if err := kv.Open(ctx); err != nil {
		t.Fatalf("Open: %v", err)
	}
	// Open again — idempotent
	if err := kv.Open(ctx); err != nil {
		t.Fatalf("Open idempotent: %v", err)
	}
}

func TestKVStore_Close(t *testing.T) {
	kv, cleanup := newTestKVStore(t)
	defer cleanup()

	ctx := context.Background()
	kv.Open(ctx)
	if err := kv.Close(ctx); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Close again — idempotent
	if err := kv.Close(ctx); err != nil {
		t.Fatalf("Close idempotent: %v", err)
	}
	// Operations after close
	if _, err := kv.Get(ctx, makeKey(t, "a")); err != ErrClosed {
		t.Errorf("Get after Close: got %v, want ErrClosed", err)
	}
	if err := kv.Set(ctx, makeKey(t, "a"), []byte("x")); err != ErrClosed {
		t.Errorf("Set after Close: got %v, want ErrClosed", err)
	}
	if err := kv.Delete(ctx, makeKey(t, "a")); err != ErrClosed {
		t.Errorf("Delete after Close: got %v, want ErrClosed", err)
	}
	if _, err := kv.Scan(ctx, noKey(), noKey()); err != ErrClosed {
		t.Errorf("Scan after Close: got %v, want ErrClosed", err)
	}
}

func TestKVStore_GetSetBasic(t *testing.T) {
	kv, cleanup := newTestKVStore(t)
	defer cleanup()

	ctx := context.Background()
	kv.Open(ctx)

	k := makeKey(t, "hello")
	v := []byte("world")

	if err := kv.Set(ctx, k, v); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := kv.Get(ctx, k)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "world" {
		t.Errorf("Get = %q, want %q", got, "world")
	}
}

func TestKVStore_GetNotFound(t *testing.T) {
	kv, cleanup := newTestKVStore(t)
	defer cleanup()

	ctx := context.Background()
	kv.Open(ctx)

	_, err := kv.Get(ctx, makeKey(t, "noexist"))
	if err != ErrKeyNotFound {
		t.Errorf("Get missing key: got %v, want ErrKeyNotFound", err)
	}
}

func TestKVStore_SetUpdate(t *testing.T) {
	kv, cleanup := newTestKVStore(t)
	defer cleanup()

	ctx := context.Background()
	kv.Open(ctx)

	k := makeKey(t, "counter")
	kv.Set(ctx, k, []byte("1"))
	kv.Set(ctx, k, []byte("2"))

	v, _ := kv.Get(ctx, k)
	if string(v) != "2" {
		t.Errorf("update: got %q, want %q", v, "2")
	}
}

func TestKVStore_Delete(t *testing.T) {
	kv, cleanup := newTestKVStore(t)
	defer cleanup()

	ctx := context.Background()
	kv.Open(ctx)

	k := makeKey(t, "delme")
	kv.Set(ctx, k, []byte("x"))
	if err := kv.Delete(ctx, k); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := kv.Get(ctx, k)
	if err != ErrKeyNotFound {
		t.Errorf("Get after Delete: got %v, want ErrKeyNotFound", err)
	}
}

func TestKVStore_DeleteMissing(t *testing.T) {
	kv, cleanup := newTestKVStore(t)
	defer cleanup()

	ctx := context.Background()
	kv.Open(ctx)
	// Deleting non-existent key is a no-op.
	if err := kv.Delete(ctx, makeKey(t, "noexist")); err != nil {
		t.Fatalf("Delete missing: %v", err)
	}
}

func TestKVStore_ScanEmpty(t *testing.T) {
	kv, cleanup := newTestKVStore(t)
	defer cleanup()

	ctx := context.Background()
	kv.Open(ctx)

	entries, err := kv.Scan(ctx, noKey(), noKey())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("empty Scan returned %d entries", len(entries))
	}
}

func TestKVStore_ScanBasic(t *testing.T) {
	kv, cleanup := newTestKVStore(t)
	defer cleanup()

	ctx := context.Background()
	kv.Open(ctx)

	// Insert in non-sorted order.
	pairs := []struct {
		k string
		v string
	}{
		{"c", "three"},
		{"a", "one"},
		{"b", "two"},
	}
	for _, p := range pairs {
		kv.Set(ctx, makeKey(t, p.k), []byte(p.v))
	}

	entries, err := kv.Scan(ctx, noKey(), noKey())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("Scan count = %d, want 3", len(entries))
	}
	// Should be in sorted order.
	expected := []string{"one", "two", "three"}
	for i, e := range entries {
		if string(e.Value) != expected[i] {
			t.Errorf("Scan[%d] = %q, want %q", i, e.Value, expected[i])
		}
	}
}

func TestKVStore_ScanRange(t *testing.T) {
	kv, cleanup := newTestKVStore(t)
	defer cleanup()

	ctx := context.Background()
	kv.Open(ctx)

	for _, c := range []string{"a", "b", "c", "d", "e"} {
		kv.Set(ctx, makeKey(t, c), []byte(c))
	}

	// Scan [b, d)
	entries, err := kv.Scan(ctx, makeKey(t, "b"), makeKey(t, "d"))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("Scan count = %d, want 2", len(entries))
	}
	if string(entries[0].Value) != "b" {
		t.Errorf("Scan[0] = %q, want 'b'", entries[0].Value)
	}
	if string(entries[1].Value) != "c" {
		t.Errorf("Scan[1] = %q, want 'c'", entries[1].Value)
	}
}

func TestKVStore_SizeLimits(t *testing.T) {
	kv, cleanup := newTestKVStore(t)
	defer cleanup()

	ctx := context.Background()
	kv.Open(ctx)

	// Key too large (use raw bytes, not string which rejects null bytes).
	bigKey := key.EncodeBytes(make([]byte, MaxKeySize+1))
	if err := kv.Set(ctx, bigKey, []byte("x")); err != ErrKeyTooLarge {
		t.Errorf("oversized key: got %v, want ErrKeyTooLarge", err)
	}

	// Value too large.
	if err := kv.Set(ctx, makeKey(t, "ok"), make([]byte, MaxValSize+1)); err != ErrValueTooLarge {
		t.Errorf("oversized value: got %v, want ErrValueTooLarge", err)
	}
}

func TestKVStore_ManyKeys(t *testing.T) {
	kv, cleanup := newTestKVStore(t)
	defer cleanup()

	ctx := context.Background()
	kv.Open(ctx)

	n := 100
	for i := 0; i < n; i++ {
		k := makeKeyGen(t, i)
		v := []byte{byte(i)}
		if err := kv.Set(ctx, k, v); err != nil {
			t.Fatalf("Set %d: %v", i, err)
		}
	}

	// Verify all can be retrieved.
	for i := 0; i < n; i++ {
		k := makeKeyGen(t, i)
		v, err := kv.Get(ctx, k)
		if err != nil {
			t.Fatalf("Get %d: %v", i, err)
		}
		if len(v) != 1 || v[0] != byte(i) {
			t.Errorf("Get %d wrong value", i)
		}
	}

	// Scan all.
	entries, err := kv.Scan(ctx, noKey(), noKey())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(entries) != n {
		t.Errorf("Scan count = %d, want %d", len(entries), n)
	}
}

// makeKey is a helper that creates a key from a string.
func makeKey(t *testing.T, s string) key.Key {
	t.Helper()
	k, err := key.EncodeString(s)
	if err != nil {
		t.Fatalf("EncodeString(%q): %v", s, err)
	}
	return k
}

// makeKeyGen creates a key for a numeric index with zero-padded ordering.
func makeKeyGen(t *testing.T, i int) key.Key {
	t.Helper()
	return key.EncodeUint64(uint64(i))
}

// noKey returns an empty key (used for unbounded scan).
func noKey() key.Key {
	// key.EncodeBytes with empty slice creates a key with empty data.
	return key.EncodeBytes([]byte{})
}

func TestNew_SetsUpCorrectly(t *testing.T) {
	dir := t.TempDir()
	pg := pager.New(dir + "/test.pager")
	kv := New(pg)
	if kv == nil {
		t.Fatal("New returned nil")
	}
	// Verify interface compliance at compile time and runtime.
	if _, ok := kv.(KVStore); !ok {
		t.Fatal("kv does not implement KVStore")
	}
}

func TestKVStore_DocsFileExists(t *testing.T) {
	data, err := os.ReadFile("../../../../docs/sql_engine_kv.md")
	if err != nil {
		t.Fatalf("docs/sql_engine_kv.md not found: %v", err)
	}
	doc := string(data)
	required := []string{
		"# Plomvix SQL Engine: On-Disk KVStore",
		"kv",
		"B+ Tree",
		"Multi-page atomicity",
		"Leaked Pages on Crash",
		"3-Phase Split Algorithm",
		"Upper Bound Routing Rule",
		"No Internal Separator Updates on Delete",
		"Format version 2",
	}
	for _, s := range required {
		if !containsStr(doc, s) {
			t.Errorf("missing required phrase in docs/sql_engine_kv.md: %q", s)
		}
	}
}

// containsStr reports whether sub is a substring of s.
func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
