package system

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/plomvix/plomvix/internal/catalog"
	"github.com/plomvix/plomvix/internal/engine"
)

func TestFactory_OpenOrCreate_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	f := NewFactory(dir)

	tables, columns, users, err := f.OpenOrCreateSystemHeaps(ctx)
	if err != nil {
		t.Fatalf("OpenOrCreateSystemHeaps: %v", err)
	}
	if tables == nil || columns == nil || users == nil {
		t.Fatal("all three stores must be non-nil")
	}

	// Verify files exist.
	for _, id := range []uint64{1, 2, 3} {
		path := f.heapPath(id)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("heap file %s not created", path)
		}
	}
}

func TestFactory_OpenOrCreate_Idempotent(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	f := NewFactory(dir)

	_, _, _, err := f.OpenOrCreateSystemHeaps(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Second call should succeed (idempotent open).
	_, _, _, err = f.OpenOrCreateSystemHeaps(ctx)
	if err != nil {
		t.Fatalf("idempotent open: %v", err)
	}
}

func TestHeapPath(t *testing.T) {
	f := NewFactory("/data")
	path := f.heapPath(42)
	expected := filepath.Join("/data", "heap_42.db")
	if path != expected {
		t.Errorf("got %q, want %q", path, expected)
	}
}

func TestSystemHeapAdapter_PutGet(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	f := NewFactory(dir)
	tables, _, _, err := f.OpenOrCreateSystemHeaps(ctx)
	if err != nil {
		t.Fatal(err)
	}

	key := []byte("test_key")
	value := []byte("test_value")

	if err := tables.Put(ctx, key, value); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := tables.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != string(value) {
		t.Errorf("got %q, want %q", got, value)
	}
}

func TestSystemHeapAdapter_Delete(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	f := NewFactory(dir)
	tables, _, _, err := f.OpenOrCreateSystemHeaps(ctx)
	if err != nil {
		t.Fatal(err)
	}

	key := []byte("del_key")
	if err := tables.Put(ctx, key, []byte("val")); err != nil {
		t.Fatal(err)
	}
	if err := tables.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}

	_, err = tables.Get(ctx, key)
	if err != catalog.ErrNotFound {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestSystemHeapAdapter_Scan(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	f := NewFactory(dir)
	tables, _, _, err := f.OpenOrCreateSystemHeaps(ctx)
	if err != nil {
		t.Fatal(err)
	}

	tables.Put(ctx, []byte("a"), []byte("1"))
	tables.Put(ctx, []byte("b"), []byte("2"))
	tables.Delete(ctx, []byte("a")) // tombstone

	found := make(map[string]string)
	err = tables.Scan(ctx, func(k, v []byte) error {
		found[string(k)] = string(v)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(found) != 1 || found["b"] != "2" {
		t.Errorf("got %v, want {b=2}", found)
	}
}

func TestSystemHeapAdapter_MVCC_LastWriteWins(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	f := NewFactory(dir)
	tables, _, _, err := f.OpenOrCreateSystemHeaps(ctx)
	if err != nil {
		t.Fatal(err)
	}

	key := []byte("mvcc_key")
	tables.Put(ctx, key, []byte("v1"))
	tables.Put(ctx, key, []byte("v2"))
	tables.Put(ctx, key, []byte("v3"))

	got, err := tables.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "v3" {
		t.Errorf("got %q, want v3", got)
	}
}

func TestConcreteSystemHeap_InsertScan(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	f := NewFactory(dir)
	tables, _, _, err := f.OpenOrCreateSystemHeaps(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Insert with engine.Row via the adapter (which goes through concreteSystemHeap).
	adapter, ok := tables.(*SystemHeapAdapter)
	if !ok {
		t.Skip("adapter type assertion failed")
	}
	row := engine.Row{Datums: []engine.Datum{
		{Type: engine.TypeBytes, Value: []byte("hello")},
		{Type: engine.TypeBytes, Value: []byte("world")},
	}}
	if err := adapter.heap.Insert(ctx, engine.TxContext{WriteTxID: 1}, row); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	iter, err := adapter.heap.Scan(ctx, engine.TxContext{ReadTxID: ^uint64(0)})
	if err != nil {
		t.Fatal(err)
	}
	defer iter.Close()

	count := 0
	for {
		r, err := iter.Next(ctx)
		if err != nil {
			break
		}
		count++
		_ = r
	}
	if count == 0 {
		t.Error("no rows returned from scan")
	}
}
