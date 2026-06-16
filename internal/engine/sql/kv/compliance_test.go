package kv

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"testing"
)

// RunKVStoreComplianceTests runs the full KVStore contract against a factory
// that returns a fresh, not-yet-open store at the given path.
func RunKVStoreComplianceTests(t *testing.T, factory func(t *testing.T, path string) KVStore) {
	t.Helper()
	t.Run("OpenClose", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "test.db")
		s := factory(t, dbPath)
		ctx := context.Background()
		if err := s.Open(ctx); err != nil {
			t.Fatalf("Open: %v", err)
		}
		if err := s.Close(ctx); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})
	t.Run("DoubleOpen", func(t *testing.T) {
		s := factory(t, filepath.Join(t.TempDir(), "test.db"))
		ctx := context.Background()
		s.Open(ctx)
		defer s.Close(ctx)
		if err := s.Open(ctx); err != ErrAlreadyOpen {
			t.Errorf("want ErrAlreadyOpen, got %v", err)
		}
	})
	t.Run("CloseNeverOpened", func(t *testing.T) {
		s := factory(t, filepath.Join(t.TempDir(), "test.db"))
		if err := s.Close(context.Background()); err != ErrNotOpen {
			t.Errorf("want ErrNotOpen, got %v", err)
		}
	})
	t.Run("ClosedRejects", func(t *testing.T) {
		s := factory(t, filepath.Join(t.TempDir(), "test.db"))
		ctx := context.Background()
		s.Open(ctx)
		s.Close(ctx)
		if err := s.Close(ctx); err != ErrClosed {
			t.Errorf("second Close: %v", err)
		}
		if err := s.Open(ctx); err != ErrClosed {
			t.Errorf("Open after Close: %v", err)
		}
	})
	t.Run("GetSetDelete", func(t *testing.T) {
		s := factory(t, filepath.Join(t.TempDir(), "test.db"))
		ctx := context.Background()
		s.Open(ctx)
		defer s.Close(ctx)
		if err := s.Set(ctx, []byte("k"), []byte("v")); err != nil {
			t.Fatal(err)
		}
		v, found, _ := s.Get(ctx, []byte("k"))
		if !found || string(v) != "v" {
			t.Fatal("Get after Set failed")
		}
		if err := s.Delete(ctx, []byte("k")); err != nil {
			t.Fatal(err)
		}
		if _, found, _ := s.Get(ctx, []byte("k")); found {
			t.Error("key not deleted")
		}
	})
	t.Run("GetMissing", func(t *testing.T) {
		s := factory(t, filepath.Join(t.TempDir(), "test.db"))
		ctx := context.Background()
		s.Open(ctx)
		defer s.Close(ctx)
		_, found, err := s.Get(ctx, []byte("noexist"))
		if err != nil || found {
			t.Error("missing key should return found=false")
		}
	})
	t.Run("EmptyKey", func(t *testing.T) {
		s := factory(t, filepath.Join(t.TempDir(), "test.db"))
		ctx := context.Background()
		s.Open(ctx)
		defer s.Close(ctx)
		if _, _, err := s.Get(ctx, nil); err != ErrEmptyKey {
			t.Errorf("Get nil: %v", err)
		}
		if err := s.Set(ctx, nil, []byte("v")); err != ErrEmptyKey {
			t.Errorf("Set nil: %v", err)
		}
		if err := s.Delete(ctx, nil); err != ErrEmptyKey {
			t.Errorf("Delete nil: %v", err)
		}
	})
	t.Run("GetCopy", func(t *testing.T) {
		s := factory(t, filepath.Join(t.TempDir(), "test.db"))
		ctx := context.Background()
		s.Open(ctx)
		defer s.Close(ctx)
		s.Set(ctx, []byte("k"), []byte("val"))
		v, _, _ := s.Get(ctx, []byte("k"))
		v[0] = 'x'
		v2, _, _ := s.Get(ctx, []byte("k"))
		if string(v2) != "val" {
			t.Error("mutating Get result should not affect store")
		}
	})
	t.Run("ScanOrdered", func(t *testing.T) {
		s := factory(t, filepath.Join(t.TempDir(), "test.db"))
		ctx := context.Background()
		s.Open(ctx)
		defer s.Close(ctx)
		for _, c := range []string{"c", "a", "b"} {
			s.Set(ctx, []byte(c), []byte(c))
		}
		var got []string
		s.Scan(ctx, nil, nil, func(k, v []byte) error { got = append(got, string(k)); return nil })
		if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
			t.Errorf("order=%v", got)
		}
	})
	t.Run("ScanBounded", func(t *testing.T) {
		s := factory(t, filepath.Join(t.TempDir(), "test.db"))
		ctx := context.Background()
		s.Open(ctx)
		defer s.Close(ctx)
		for _, c := range []string{"a", "b", "c", "d", "e"} {
			s.Set(ctx, []byte(c), []byte(c))
		}
		var got []string
		s.Scan(ctx, []byte("b"), []byte("d"), func(k, v []byte) error { got = append(got, string(k)); return nil })
		if len(got) != 2 || got[0] != "b" || got[1] != "c" {
			t.Errorf("got %v", got)
		}
	})
	t.Run("ScanEmptyRange", func(t *testing.T) {
		s := factory(t, filepath.Join(t.TempDir(), "test.db"))
		ctx := context.Background()
		s.Open(ctx)
		defer s.Close(ctx)
		s.Set(ctx, []byte("x"), []byte("v"))
		count := 0
		s.Scan(ctx, []byte("x"), []byte("x"), func(k, v []byte) error { count++; return nil })
		if count != 0 {
			t.Error("empty range should visit nothing")
		}
	})
	t.Run("ScanNilCallback", func(t *testing.T) {
		s := factory(t, filepath.Join(t.TempDir(), "test.db"))
		ctx := context.Background()
		s.Open(ctx)
		defer s.Close(ctx)
		if err := s.Scan(ctx, nil, nil, nil); err != ErrNilCallback {
			t.Errorf("want ErrNilCallback, got %v", err)
		}
	})
	t.Run("ScanFnError", func(t *testing.T) {
		s := factory(t, filepath.Join(t.TempDir(), "test.db"))
		ctx := context.Background()
		s.Open(ctx)
		defer s.Close(ctx)
		s.Set(ctx, []byte("a"), nil)
		s.Set(ctx, []byte("b"), nil)
		count := 0
		stop := fmt.Errorf("stop")
		err := s.Scan(ctx, nil, nil, func(k, v []byte) error { count++; return stop })
		if err != stop {
			t.Errorf("want stop, got %v", err)
		}
		if count != 1 {
			t.Errorf("count=%d", count)
		}
	})
	t.Run("ScanCopies", func(t *testing.T) {
		s := factory(t, filepath.Join(t.TempDir(), "test.db"))
		ctx := context.Background()
		s.Open(ctx)
		defer s.Close(ctx)
		s.Set(ctx, []byte("k"), []byte("v"))
		var sk, sv []byte
		s.Scan(ctx, nil, nil, func(k, v []byte) error { sk = k; sv = v; return nil })
		sk[0] = 'x'
		sv[0] = 'x'
		s.Scan(ctx, nil, nil, func(k, v []byte) error { sk = k; sv = v; return nil })
		if string(sk) != "k" || string(sv) != "v" {
			t.Error("mutating scan copies corrupts store")
		}
	})
	t.Run("Batch", func(t *testing.T) {
		s := factory(t, filepath.Join(t.TempDir(), "test.db"))
		ctx := context.Background()
		s.Open(ctx)
		defer s.Close(ctx)
		b := s.NewBatch()
		b.Set([]byte("a"), []byte("1"))
		b.Set([]byte("b"), []byte("2"))
		if err := b.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		v, _, _ := s.Get(ctx, []byte("a"))
		if string(v) != "1" {
			t.Error("batch commit failed")
		}
	})
	t.Run("BatchUncommitted", func(t *testing.T) {
		s := factory(t, filepath.Join(t.TempDir(), "test.db"))
		ctx := context.Background()
		s.Open(ctx)
		defer s.Close(ctx)
		s.NewBatch().Set([]byte("k"), []byte("v"))
		if _, found, _ := s.Get(ctx, []byte("k")); found {
			t.Error("uncommitted batch visible")
		}
	})
	t.Run("BatchReset", func(t *testing.T) {
		s := factory(t, filepath.Join(t.TempDir(), "test.db"))
		ctx := context.Background()
		s.Open(ctx)
		defer s.Close(ctx)
		b := s.NewBatch()
		b.Set([]byte("k"), []byte("v"))
		b.Reset()
		b.Commit(ctx)
		if _, found, _ := s.Get(ctx, []byte("k")); found {
			t.Error("reset batch visible")
		}
	})
	t.Run("BatchDoubleCommit", func(t *testing.T) {
		s := factory(t, filepath.Join(t.TempDir(), "test.db"))
		ctx := context.Background()
		s.Open(ctx)
		defer s.Close(ctx)
		b := s.NewBatch()
		b.Set([]byte("k"), []byte("v"))
		b.Commit(ctx)
		if err := b.Commit(ctx); err != nil {
			t.Errorf("second commit: %v", err)
		}
	})
	t.Run("BatchEmptyKey", func(t *testing.T) {
		s := factory(t, filepath.Join(t.TempDir(), "test.db"))
		ctx := context.Background()
		s.Open(ctx)
		defer s.Close(ctx)
		b := s.NewBatch()
		b.Set([]byte("good"), []byte("v"))
		b.Set(nil, []byte("bad"))
		if err := b.Commit(ctx); err != ErrEmptyKey {
			t.Errorf("want ErrEmptyKey, got %v", err)
		}
		if _, found, _ := s.Get(ctx, []byte("good")); found {
			t.Error("batch with empty key should not apply")
		}
	})
	t.Run("BatchClosed", func(t *testing.T) {
		s := factory(t, filepath.Join(t.TempDir(), "test.db"))
		ctx := context.Background()
		s.Open(ctx)
		b := s.NewBatch()
		b.Set([]byte("k"), []byte("v"))
		s.Close(ctx)
		if err := b.Commit(ctx); err != ErrClosed {
			t.Errorf("want ErrClosed, got %v", err)
		}
	})
	t.Run("ContextCanceled", func(t *testing.T) {
		s := factory(t, filepath.Join(t.TempDir(), "test.db"))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		s.Open(context.Background())
		defer s.Close(context.Background())
		if _, _, err := s.Get(ctx, []byte("k")); !errors.Is(err, context.Canceled) {
			t.Errorf("Get: %v", err)
		}
		if err := s.Set(ctx, []byte("k"), nil); !errors.Is(err, context.Canceled) {
			t.Errorf("Set: %v", err)
		}
		if err := s.Scan(ctx, nil, nil, func(k, v []byte) error { return nil }); !errors.Is(err, context.Canceled) {
			t.Errorf("Scan: %v", err)
		}
	})
	t.Run("Persistence", func(t *testing.T) {
		dir := t.TempDir()
		// Create the directory structure if needed
		dbPath := filepath.Join(dir, "test.db")
		s1 := factory(t, dbPath)
		ctx := context.Background()
		s1.Open(ctx)
		s1.Set(ctx, []byte("k"), []byte("v"))
		s1.Close(ctx)
		s2 := factory(t, dbPath)
		s2.Open(ctx)
		defer s2.Close(ctx)
		v, found, _ := s2.Get(ctx, []byte("k"))
		if !found || string(v) != "v" {
			t.Fatal("persistence failed")
		}
	})
	t.Run("Concurrency", func(t *testing.T) {
		s := factory(t, filepath.Join(t.TempDir(), "test.db"))
		ctx := context.Background()
		s.Open(ctx)
		defer s.Close(ctx)
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				k := []byte{byte(id)}
				s.Set(ctx, k, k)
				s.Get(ctx, k)
			}(i)
		}
		wg.Wait()
	})
}

func TestBBoltCompliance(t *testing.T) {
	RunKVStoreComplianceTests(t, func(t *testing.T, path string) KVStore {
		return NewBBolt("test", path)
	})
}

func TestFeature1KeyOrderWithBolt(t *testing.T) {
	s := NewBBolt("test", filepath.Join(t.TempDir(), "test.db"))
	ctx := context.Background()
	s.Open(ctx)
	defer s.Close(ctx)

	// Insert keys in non-sorted order
	keys := [][]byte{{0, 3}, {0, 1}, {0, 5}, {0, 2}}
	for _, k := range keys {
		s.Set(ctx, k, k)
	}

	var got [][]byte
	s.Scan(ctx, nil, nil, func(k, v []byte) error { got = append(got, k); return nil })
	if !sort.SliceIsSorted(got, func(i, j int) bool { return bytes.Compare(got[i], got[j]) < 0 }) {
		t.Error("scan order is not sorted")
	}
}
