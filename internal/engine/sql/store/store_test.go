package store_test

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"testing"

	"github.com/plomvix/plomvix/internal/engine/sql/key"
	"github.com/plomvix/plomvix/internal/engine/sql/store"
)

func TestSentinels(t *testing.T) {
	if store.ErrNotFound.Error() != "sql/store: key not found" {
		t.Error("ErrNotFound")
	}
	if store.ErrNilStore.Error() != "sql/store: nil store" {
		t.Error("ErrNilStore")
	}
}

func TestNew(t *testing.T) {
	s := store.New()
	if s == nil {
		t.Fatal("New() returned nil")
	}
	if s.Len() != 0 {
		t.Errorf("Len = %d", s.Len())
	}
}

func TestPutGet(t *testing.T) {
	s := store.New()
	k := key.EncodeUint64(1)
	if err := s.Put(k, []byte("a")); err != nil {
		t.Fatal(err)
	}
	v, err := s.Get(k)
	if err != nil {
		t.Fatal(err)
	}
	if string(v) != "a" {
		t.Errorf("got %q", v)
	}
}

func TestGetMissing(t *testing.T) {
	_, err := store.New().Get(key.EncodeUint64(999))
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("got %v", err)
	}
}

func TestPutOverwrite(t *testing.T) {
	s := store.New()
	k := key.EncodeUint64(1)
	s.Put(k, []byte("a"))
	s.Put(k, []byte("b"))
	v, _ := s.Get(k)
	if string(v) != "b" {
		t.Errorf("got %q", v)
	}
	if s.Len() != 1 {
		t.Errorf("Len = %d after overwrite", s.Len())
	}
}

func TestPutCopySafety(t *testing.T) {
	s := store.New()
	val := []byte("original")
	s.Put(key.EncodeUint64(1), val)
	val[0] = 'X'
	v, _ := s.Get(key.EncodeUint64(1))
	if string(v) != "original" {
		t.Errorf("got %q, put copy failed", v)
	}
}

func TestGetCopySafety(t *testing.T) {
	s := store.New()
	s.Put(key.EncodeUint64(1), []byte("original"))
	got, _ := s.Get(key.EncodeUint64(1))
	got[0] = 'X'
	got2, _ := s.Get(key.EncodeUint64(1))
	if string(got2) != "original" {
		t.Errorf("got %q, get copy failed", got2)
	}
}

func TestNilStore(t *testing.T) {
	var s *store.Store
	if _, err := s.Get(key.EncodeUint64(1)); !errors.Is(err, store.ErrNilStore) {
		t.Errorf("Get: %v", err)
	}
	if err := s.Put(key.EncodeUint64(1), []byte("x")); !errors.Is(err, store.ErrNilStore) {
		t.Errorf("Put: %v", err)
	}
	if err := s.Delete(key.EncodeUint64(1)); !errors.Is(err, store.ErrNilStore) {
		t.Errorf("Delete: %v", err)
	}
	if _, err := s.Scan(key.EncodeUint64(0), key.EncodeUint64(1)); !errors.Is(err, store.ErrNilStore) {
		t.Errorf("Scan: %v", err)
	}
	if s.Len() != 0 {
		t.Error("Len should be 0")
	}
}

func TestLen(t *testing.T) {
	s := store.New()
	if s.Len() != 0 {
		t.Error("initial")
	}
	s.Put(key.EncodeUint64(1), []byte("a"))
	s.Put(key.EncodeUint64(2), []byte("b"))
	s.Put(key.EncodeUint64(3), []byte("c"))
	if s.Len() != 3 {
		t.Error("after puts")
	}
	s.Put(key.EncodeUint64(1), []byte("x"))
	if s.Len() != 3 {
		t.Error("overwrite should not grow")
	}
}

func TestDelete(t *testing.T) {
	s := store.New()
	k := key.EncodeUint64(1)
	s.Put(k, []byte("a"))
	if err := s.Delete(k); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(k); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("got %v", err)
	}
	if s.Len() != 0 {
		t.Error("Len after delete")
	}
}

func TestDeleteMissing(t *testing.T) {
	if err := store.New().Delete(key.EncodeUint64(999)); err != nil {
		t.Errorf("delete missing should be no-op, got %v", err)
	}
}

func TestDeleteMiddle(t *testing.T) {
	s := store.New()
	s.Put(key.EncodeUint64(1), []byte("a"))
	s.Put(key.EncodeUint64(2), []byte("b"))
	s.Put(key.EncodeUint64(3), []byte("c"))
	s.Delete(key.EncodeUint64(2))
	if s.Len() != 2 {
		t.Error("Len after middle delete")
	}
	if _, err := s.Get(key.EncodeUint64(1)); err != nil {
		t.Error("key 1 missing")
	}
	if _, err := s.Get(key.EncodeUint64(3)); err != nil {
		t.Error("key 3 missing")
	}
	if _, err := s.Get(key.EncodeUint64(2)); !errors.Is(err, store.ErrNotFound) {
		t.Error("key 2 still present")
	}
}

func TestScan(t *testing.T) {
	s := store.New()
	for i := uint64(0); i < 10; i++ {
		s.Put(key.EncodeUint64(i), []byte{byte(i)})
	}

	t.Run("half-open range", func(t *testing.T) {
		r, _ := s.Scan(key.EncodeUint64(2), key.EncodeUint64(5))
		if len(r) != 3 {
			t.Fatalf("len=%d", len(r))
		}
		for i, v := range []uint64{2, 3, 4} {
			got, _ := key.DecodeUint64(r[i].Key)
			if got != v {
				t.Errorf("r[%d] = %d", i, got)
			}
		}
	})

	t.Run("full range", func(t *testing.T) {
		r, _ := s.Scan(key.EncodeUint64(0), key.EncodeUint64(10))
		if len(r) != 10 {
			t.Errorf("len=%d", len(r))
		}
	})

	t.Run("empty start==end", func(t *testing.T) {
		r, err := s.Scan(key.EncodeUint64(3), key.EncodeUint64(3))
		if err != nil {
			t.Fatal(err)
		}
		if len(r) != 0 {
			t.Error("should be empty")
		}
	})

	t.Run("empty start>end", func(t *testing.T) {
		r, err := s.Scan(key.EncodeUint64(5), key.EncodeUint64(2))
		if err != nil {
			t.Fatal(err)
		}
		if len(r) != 0 {
			t.Error("should be empty")
		}
	})

	t.Run("no matching", func(t *testing.T) {
		r, _ := s.Scan(key.EncodeUint64(100), key.EncodeUint64(200))
		if len(r) != 0 {
			t.Error("should be empty")
		}
	})

	t.Run("ordering", func(t *testing.T) {
		r, _ := s.Scan(key.EncodeUint64(0), key.EncodeUint64(10))
		for i := 0; i < len(r)-1; i++ {
			if r[i].Key.Compare(r[i+1].Key) >= 0 {
				t.Error("not ascending")
			}
		}
	})

	t.Run("copy safety", func(t *testing.T) {
		r, _ := s.Scan(key.EncodeUint64(0), key.EncodeUint64(10))
		if len(r) > 0 {
			r[0].Value[0] = 'X'
		}
		r2, _ := s.Scan(key.EncodeUint64(0), key.EncodeUint64(10))
		if len(r2) > 0 && r2[0].Value[0] == 'X' {
			t.Error("scan copy failed")
		}
	})

	t.Run("adjacent ranges", func(t *testing.T) {
		first, _ := s.Scan(key.EncodeUint64(0), key.EncodeUint64(5))
		second, _ := s.Scan(key.EncodeUint64(5), key.EncodeUint64(10))
		if len(first)+len(second) != 10 {
			t.Error("adjacent ranges should cover all")
		}
	})
}

func TestConcurrentAccess(t *testing.T) {
	s := store.New()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			k := key.EncodeUint64(uint64(n))
			s.Put(k, []byte("value"))
			s.Get(k)
			s.Scan(key.EncodeUint64(0), key.EncodeUint64(100))
		}(i)
	}
	wg.Wait()
	if s.Len() > 50 {
		t.Errorf("Len=%d", s.Len())
	}
}

func TestDocumentation(t *testing.T) {
	data, err := os.ReadFile("../../../../docs/markdown/sql_store.md")
	if err != nil {
		t.Fatalf("docs/markdown/sql_store.md not found: %v", err)
	}
	c := string(data)
	for _, s := range []string{
		"# Plomvix SQL Store", "sql/store", "sorted slice", "sort.Search",
		"sync.RWMutex", "half-open", "[start, end)", "copy-safety",
		"in-memory", "no transactions",
		"WAL", "disk persistence", "storage pages", "buffer pool",
		"transactions", "compaction", "snapshots", "sharded locking",
		// enterprise hardening section
		"Enterprise Hardening", "sorted-invariant", "overwrite stress",
		"size-scaling", "O(n)", "flat relative to store size",
		"on-disk storage", "no API changes",
	} {
		if !contains(c, s) {
			t.Errorf("missing: %q", s)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > len(sub) && searchSub(s, sub))
}

func searchSub(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestStore_SortedInvariantUnderConcurrentChurn(t *testing.T) {
	s := store.New()
	const numGoroutines = 50
	const opsPerGoroutine = 200
	const keySpace = 500

	var wg sync.WaitGroup
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			r := rand.New(rand.NewSource(seed))
			for i := 0; i < opsPerGoroutine; i++ {
				n := r.Intn(keySpace)
				k := key.EncodeUint64(uint64(n))
				if r.Intn(2) == 0 {
					_ = s.Put(k, []byte("v"))
				} else {
					_ = s.Delete(k)
				}
			}
		}(int64(g))
	}
	wg.Wait()

	entries, err := s.Scan(key.EncodeUint64(0), key.EncodeUint64(keySpace+1))
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	for i := 1; i < len(entries); i++ {
		if entries[i-1].Key.Compare(entries[i].Key) >= 0 {
			t.Fatalf("sorted invariant violated at index %d: %v >= %v",
				i, entries[i-1].Key.Bytes(), entries[i].Key.Bytes())
		}
	}
}

func TestStore_ConcurrentOverwriteSameKeys(t *testing.T) {
	s := store.New()
	const keySpace = 5
	const numGoroutines = 50
	const writesPerKey = 100

	var wg sync.WaitGroup
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < writesPerKey; i++ {
				for k := 0; k < keySpace; k++ {
					kk := key.EncodeUint64(uint64(k))
					val := []byte(fmt.Sprintf("g=%d,i=%d,k=%d", g, i, k))
					_ = s.Put(kk, val)
				}
			}
		}(g)
	}
	wg.Wait()

	if s.Len() != keySpace {
		t.Fatalf("expected Len()=%d, got %d", keySpace, s.Len())
	}

	entries, err := s.Scan(key.EncodeUint64(0), key.EncodeUint64(keySpace))
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(entries) != keySpace {
		t.Fatalf("expected %d entries from scan, got %d", keySpace, len(entries))
	}
	for i := 1; i < len(entries); i++ {
		if entries[i-1].Key.Compare(entries[i].Key) >= 0 {
			t.Fatalf("sorted invariant violated at index %d", i)
		}
	}
}
