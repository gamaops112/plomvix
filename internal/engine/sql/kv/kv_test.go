package kv

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestSentinelErrorsExist(t *testing.T) {
	errs := []error{ErrNotOpen, ErrAlreadyOpen, ErrClosed, ErrEmptyKey, ErrNilCallback}
	for i, e := range errs {
		if e == nil || e.Error() == "" {
			t.Errorf("error[%d] is nil or empty", i)
		}
	}
}

func TestOpenClose(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewBBolt("sql", dbPath)
	ctx := context.Background()
	if err := s.Open(ctx); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.Close(ctx); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestDoubleOpen(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewBBolt("sql", dbPath)
	ctx := context.Background()
	s.Open(ctx)
	if err := s.Open(ctx); err != ErrAlreadyOpen {
		t.Errorf("want ErrAlreadyOpen, got %v", err)
	}
	s.Close(ctx)
}

func TestCloseNeverOpened(t *testing.T) {
	s := NewBBolt("sql", filepath.Join(t.TempDir(), "test.db"))
	if err := s.Close(context.Background()); err != ErrNotOpen {
		t.Errorf("want ErrNotOpen, got %v", err)
	}
}

func TestClosedStoreRejectsOps(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewBBolt("sql", dbPath)
	ctx := context.Background()
	s.Open(ctx)
	s.Close(ctx)
	if err := s.Close(ctx); err != ErrClosed {
		t.Errorf("second Close: want ErrClosed, got %v", err)
	}
	if err := s.Open(ctx); err != ErrClosed {
		t.Errorf("Open after Close: want ErrClosed, got %v", err)
	}
}

func TestFileExistsAfterOpen(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewBBolt("sql", dbPath)
	s.Open(context.Background())
	defer s.Close(context.Background())
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("file missing")
	}
}

func TestOpenCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "sub", "dir", "test.db")
	s := NewBBolt("sql", dbPath)
	if err := s.Open(context.Background()); err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close(context.Background())
	if _, err := os.Stat(filepath.Dir(dbPath)); os.IsNotExist(err) {
		t.Error("dir missing")
	}
}

func TestOpenUnwritableDir(t *testing.T) {
	tmpDir := t.TempDir()
	block := filepath.Join(tmpDir, "block")
	os.WriteFile(block, []byte("x"), 0444)
	dbPath := filepath.Join(block, "sub", "test.db")
	s := NewBBolt("sql", dbPath)
	err := s.Open(context.Background())
	if err == nil {
		t.Skip("cannot trigger dir creation failure")
		defer s.Close(context.Background())
		return
	}
	if cerr := s.Close(context.Background()); cerr != ErrNotOpen {
		t.Errorf("after failed Open, store should be NeverOpened: want ErrNotOpen, got %v", cerr)
	}
}

func TestSetGet(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewBBolt("sql", dbPath)
	ctx := context.Background()
	s.Open(ctx)
	defer s.Close(ctx)

	if err := s.Set(ctx, []byte("k"), []byte("v")); err != nil {
		t.Fatal(err)
	}
	v, found, err := s.Get(ctx, []byte("k"))
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("key not found")
	}
	if string(v) != "v" {
		t.Errorf("got %q", v)
	}
}

func TestGetCopy(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewBBolt("sql", dbPath)
	ctx := context.Background()
	s.Open(ctx)
	defer s.Close(ctx)
	s.Set(ctx, []byte("k"), []byte("v"))
	v, _, _ := s.Get(ctx, []byte("k"))
	v[0] = 'x'
	v2, _, _ := s.Get(ctx, []byte("k"))
	if string(v2) != "v" {
		t.Error("mutating Get result should not affect store")
	}
}

func TestGetMissing(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewBBolt("sql", dbPath)
	ctx := context.Background()
	s.Open(ctx)
	defer s.Close(ctx)
	_, found, err := s.Get(ctx, []byte("noexist"))
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Error("should not be found")
	}
}

func TestDelete(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewBBolt("sql", dbPath)
	ctx := context.Background()
	s.Open(ctx)
	defer s.Close(ctx)
	s.Set(ctx, []byte("k"), []byte("v"))
	if err := s.Delete(ctx, []byte("k")); err != nil {
		t.Fatal(err)
	}
	_, found, _ := s.Get(ctx, []byte("k"))
	if found {
		t.Error("key should be gone after Delete")
	}
	if err := s.Delete(ctx, []byte("noexist")); err != nil {
		t.Fatal("Delete non-existent should not error")
	}
}

func TestEmptyKey(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewBBolt("sql", dbPath)
	ctx := context.Background()
	s.Open(ctx)
	defer s.Close(ctx)
	if _, _, err := s.Get(ctx, nil); err != ErrEmptyKey {
		t.Errorf("Get nil key: %v", err)
	}
	if err := s.Set(ctx, nil, []byte("v")); err != ErrEmptyKey {
		t.Errorf("Set nil key: %v", err)
	}
	if err := s.Delete(ctx, nil); err != ErrEmptyKey {
		t.Errorf("Delete nil key: %v", err)
	}
	_, _, err := s.Get(ctx, []byte{})
	if err != ErrEmptyKey {
		t.Errorf("Get empty key: %v", err)
	}
}

func TestNilValue(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewBBolt("sql", dbPath)
	ctx := context.Background()
	s.Open(ctx)
	defer s.Close(ctx)
	s.Set(ctx, []byte("k"), nil)
	v, found, _ := s.Get(ctx, []byte("k"))
	if !found {
		t.Error("should be found")
	}
	if len(v) != 0 {
		t.Errorf("nil value should be zero-length, got %d", len(v))
	}
}

func TestPersistence(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s1 := NewBBolt("sql", dbPath)
	ctx := context.Background()
	s1.Open(ctx)
	s1.Set(ctx, []byte("k"), []byte("v"))
	s1.Close(ctx)

	s2 := NewBBolt("sql", dbPath)
	s2.Open(ctx)
	defer s2.Close(ctx)
	v, found, _ := s2.Get(ctx, []byte("k"))
	if !found || string(v) != "v" {
		t.Fatal("persistence failed")
	}
}

func TestConcurrentGetSet(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewBBolt("sql", dbPath)
	ctx := context.Background()
	s.Open(ctx)
	defer s.Close(ctx)

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- struct{}{} }()
			k := []byte{byte(id)}
			s.Set(ctx, k, k)
			s.Get(ctx, k)
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestScanFull(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewBBolt("sql", dbPath)
	ctx := context.Background()
	s.Open(ctx)
	defer s.Close(ctx)
	keys := [][]byte{[]byte("c"), []byte("a"), []byte("b")}
	for _, k := range keys {
		s.Set(ctx, k, k)
	}
	var got [][]byte
	s.Scan(ctx, nil, nil, func(k, v []byte) error { got = append(got, k); return nil })
	if len(got) != 3 {
		t.Fatalf("len=%d", len(got))
	}
	if string(got[0]) != "a" || string(got[1]) != "b" || string(got[2]) != "c" {
		t.Errorf("order = %s", got)
	}
}

func TestScanBounded(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewBBolt("sql", dbPath)
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
}

func TestScanEmptyRange(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewBBolt("sql", dbPath)
	ctx := context.Background()
	s.Open(ctx)
	defer s.Close(ctx)
	s.Set(ctx, []byte("x"), []byte("v"))
	count := 0
	s.Scan(ctx, []byte("x"), []byte("x"), func(k, v []byte) error { count++; return nil })
	if count != 0 {
		t.Error("empty range should visit nothing")
	}
}

func TestScanNilCallback(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewBBolt("sql", dbPath)
	ctx := context.Background()
	s.Open(ctx)
	defer s.Close(ctx)
	if err := s.Scan(ctx, nil, nil, nil); err != ErrNilCallback {
		t.Errorf("want ErrNilCallback, got %v", err)
	}
}

func TestScanFnErrorStops(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewBBolt("sql", dbPath)
	ctx := context.Background()
	s.Open(ctx)
	defer s.Close(ctx)
	s.Set(ctx, []byte("a"), nil)
	s.Set(ctx, []byte("b"), nil)
	count := 0
	errStop := fmt.Errorf("stop")
	err := s.Scan(ctx, nil, nil, func(k, v []byte) error { count++; return errStop })
	if err != errStop {
		t.Errorf("want errStop, got %v", err)
	}
	if count != 1 {
		t.Errorf("count=%d after stop", count)
	}
}

func TestScanCopies(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewBBolt("sql", dbPath)
	ctx := context.Background()
	s.Open(ctx)
	defer s.Close(ctx)
	s.Set(ctx, []byte("k"), []byte("v"))
	var savedKey, savedVal []byte
	s.Scan(ctx, nil, nil, func(k, v []byte) error { savedKey = k; savedVal = v; return nil })
	savedKey[0] = 'x'
	savedVal[0] = 'x'
	var checkKey, checkVal []byte
	s.Scan(ctx, nil, nil, func(k, v []byte) error { checkKey = k; checkVal = v; return nil })
	if string(checkKey) != "k" || string(checkVal) != "v" {
		t.Error("mutating scan copies should not affect store")
	}
}

func TestBatchCommit(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewBBolt("sql", dbPath)
	ctx := context.Background()
	s.Open(ctx)
	defer s.Close(ctx)
	b := s.NewBatch()
	b.Set([]byte("a"), []byte("1"))
	b.Set([]byte("b"), []byte("2"))
	b.Delete([]byte("c")) // non-existent, OK
	if err := b.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	v, _, _ := s.Get(ctx, []byte("a"))
	if string(v) != "1" {
		t.Error("a not set")
	}
	v, _, _ = s.Get(ctx, []byte("b"))
	if string(v) != "2" {
		t.Error("b not set")
	}
}

func TestBatchNoCommitNoChange(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewBBolt("sql", dbPath)
	ctx := context.Background()
	s.Open(ctx)
	defer s.Close(ctx)
	s.NewBatch().Set([]byte("k"), []byte("v"))
	_, found, _ := s.Get(ctx, []byte("k"))
	if found {
		t.Error("uncommitted batch should not write")
	}
}

func TestBatchReset(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewBBolt("sql", dbPath)
	ctx := context.Background()
	s.Open(ctx)
	defer s.Close(ctx)
	b := s.NewBatch()
	b.Set([]byte("k"), []byte("v"))
	b.Reset()
	b.Commit(ctx)
	_, found, _ := s.Get(ctx, []byte("k"))
	if found {
		t.Error("reset batch should not write")
	}
}

func TestBatchDoubleCommit(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewBBolt("sql", dbPath)
	ctx := context.Background()
	s.Open(ctx)
	defer s.Close(ctx)
	b := s.NewBatch()
	b.Set([]byte("k"), []byte("v"))
	b.Commit(ctx)
	// Second Commit is no-op
	if err := b.Commit(ctx); err != nil {
		t.Errorf("second Commit should be nil, got %v", err)
	}
}

func TestBatchEmptyKey(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewBBolt("sql", dbPath)
	ctx := context.Background()
	s.Open(ctx)
	defer s.Close(ctx)
	b := s.NewBatch()
	b.Set([]byte("good"), []byte("v"))
	b.Set(nil, []byte("bad"))
	if err := b.Commit(ctx); err != ErrEmptyKey {
		t.Errorf("want ErrEmptyKey, got %v", err)
	}
	_, found, _ := s.Get(ctx, []byte("good"))
	if found {
		t.Error("good key should not be applied when batch has empty key")
	}
}

func TestBatchClosedStore(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewBBolt("sql", dbPath)
	ctx := context.Background()
	s.Open(ctx)
	b := s.NewBatch()
	b.Set([]byte("k"), []byte("v"))
	s.Close(ctx)
	if err := b.Commit(ctx); err != ErrClosed {
		t.Errorf("want ErrClosed, got %v", err)
	}
}

func TestBatchNeverOpenedStore(t *testing.T) {
	s := NewBBolt("sql", filepath.Join(t.TempDir(), "test.db"))
	b := s.NewBatch()
	b.Set([]byte("k"), []byte("v"))
	if err := b.Commit(context.Background()); err != ErrNotOpen {
		t.Errorf("want ErrNotOpen, got %v", err)
	}
}

func TestBatchMutationAfterSet(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := NewBBolt("sql", dbPath)
	ctx := context.Background()
	s.Open(ctx)
	defer s.Close(ctx)
	k := []byte("key")
	orig := []byte("val")
	mut := make([]byte, len(orig))
	copy(mut, orig)
	b := s.NewBatch()
	b.Set(k, mut)
	mut[0] = 'x'
	b.Commit(ctx)
	v, _, _ := s.Get(ctx, k)
	if string(v) != "val" {
		t.Errorf("mutating after batch.Set should not change stored value: got %q", v)
	}
}

// TestEndToEnd exercises the full KVStore lifecycle the way runtime wiring will:
// construct via NewBBolt with a path derived from [sql_engine] data_dir,
// Open (creates missing dir), Set/Get/Scan, Close.
// Runtime/engine wiring (a LATER feature) is responsible for reading
// cfg.SQL.DataDir and calling NewBBolt("sql", filepath.Join(cfg.SQL.DataDir, "sql.db")).
func TestEndToEnd(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sql", "sql.db") // dir does not exist yet
	s := NewBBolt("sql", dbPath)
	ctx := context.Background()

	if err := s.Open(ctx); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.Set(ctx, []byte("k1"), []byte("v1")); err != nil {
		t.Fatal(err)
	}
	v, found, err := s.Get(ctx, []byte("k1"))
	if err != nil || !found || string(v) != "v1" {
		t.Fatal("Get after Set failed")
	}

	// Scan
	s.Set(ctx, []byte("a"), []byte("1"))
	s.Set(ctx, []byte("b"), []byte("2"))
	var keys []string
	s.Scan(ctx, nil, nil, func(k, v []byte) error { keys = append(keys, string(k)); return nil })
	if len(keys) != 3 {
		t.Errorf("scan got %d keys", len(keys))
	}

	if err := s.Close(ctx); err != nil {
		t.Fatal(err)
	}
}
