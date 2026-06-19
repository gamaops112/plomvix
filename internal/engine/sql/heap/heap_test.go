package heap

import (
	"context"
	"os"
	"testing"

	"github.com/plomvix/plomvix/internal/engine/sql/key"
	"github.com/plomvix/plomvix/internal/engine/sql/kv"
	"github.com/plomvix/plomvix/internal/storage/pager"
)

// newTestHeap creates a Heap backed by an open KVStore for testing.
func newTestHeap(t *testing.T) (*Heap, func()) {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/heap_test.pager"
	pg := pager.New(path)
	ctx := context.Background()
	if err := pg.Open(ctx); err != nil {
		t.Fatalf("pager.Open: %v", err)
	}
	store := kv.New(pg)
	if err := store.Open(ctx); err != nil {
		t.Fatalf("kv.Open: %v", err)
	}
	cleanup := func() {
		store.Close(ctx)
		pg.Close(ctx)
	}
	return New(store), cleanup
}

// testSchema returns a simple single-column PK schema.
func testSingleColSchema() Schema {
	return Schema{
		TableID:   1,
		Columns:   []Column{{Name: "id", Kind: key.KindUint64}},
		PKIndices: []int{0},
	}
}

// testMultiColSchema returns a 3-column schema with composite PK.
func testMultiColSchema() Schema {
	return Schema{
		TableID: 2,
		Columns: []Column{
			{Name: "pk1", Kind: key.KindUint64},
			{Name: "pk2", Kind: key.KindString},
			{Name: "val", Kind: key.KindBytes},
		},
		PKIndices: []int{0, 1},
	}
}

// --- Task 1: Schema validation ---

func TestOpenTable_ValidSchema(t *testing.T) {
	h, cleanup := newTestHeap(t)
	defer cleanup()

	tbl, err := h.OpenTable(testSingleColSchema())
	if err != nil {
		t.Fatalf("OpenTable: %v", err)
	}
	if tbl == nil {
		t.Fatal("OpenTable returned nil table")
	}
}

func TestOpenTable_EmptyColumns(t *testing.T) {
	h, cleanup := newTestHeap(t)
	defer cleanup()

	_, err := h.OpenTable(Schema{TableID: 1, Columns: nil, PKIndices: []int{0}})
	if err != ErrInvalidSchema {
		t.Errorf("expected ErrInvalidSchema, got %v", err)
	}
}

func TestOpenTable_EmptyPK(t *testing.T) {
	h, cleanup := newTestHeap(t)
	defer cleanup()

	_, err := h.OpenTable(Schema{
		TableID:   1,
		Columns:   []Column{{Name: "id", Kind: key.KindUint64}},
		PKIndices: nil,
	})
	if err != ErrInvalidSchema {
		t.Errorf("expected ErrInvalidSchema, got %v", err)
	}
}

func TestOpenTable_PKOutOfBounds(t *testing.T) {
	h, cleanup := newTestHeap(t)
	defer cleanup()

	_, err := h.OpenTable(Schema{
		TableID:   1,
		Columns:   []Column{{Name: "id", Kind: key.KindUint64}},
		PKIndices: []int{1},
	})
	if err != ErrInvalidSchema {
		t.Errorf("expected ErrInvalidSchema, got %v", err)
	}
}

func TestOpenTable_DuplicateColumnNames(t *testing.T) {
	h, cleanup := newTestHeap(t)
	defer cleanup()

	_, err := h.OpenTable(Schema{
		TableID: 1,
		Columns: []Column{
			{Name: "a", Kind: key.KindUint64},
			{Name: "a", Kind: key.KindString},
		},
		PKIndices: []int{0},
	})
	if err != ErrInvalidSchema {
		t.Errorf("expected ErrInvalidSchema, got %v", err)
	}
}

// --- Task 2: Round-trip encode/decode ---

func TestEncodeDecode_RoundTrip(t *testing.T) {
	schema := testMultiColSchema()

	vals := []any{uint64(42), "hello", []byte("world")}
	data, err := encodeRowValue(schema, vals)
	if err != nil {
		t.Fatalf("encodeRowValue: %v", err)
	}

	decoded, err := decodeRowValue(schema, data)
	if err != nil {
		t.Fatalf("decodeRowValue: %v", err)
	}

	if decoded[0].(uint64) != uint64(42) {
		t.Errorf("pk1 = %v, want 42", decoded[0])
	}
	if decoded[1].(string) != "hello" {
		t.Errorf("pk2 = %v, want 'hello'", decoded[1])
	}
	if string(decoded[2].([]byte)) != "world" {
		t.Errorf("val = %v, want 'world'", decoded[2])
	}
}

func TestEncodeRowValue_Nil(t *testing.T) {
	schema := testSingleColSchema()
	_, err := encodeRowValue(schema, []any{nil})
	if err != ErrNullNotSupported {
		t.Errorf("expected ErrNullNotSupported, got %v", err)
	}
}

func TestEncodeRowValue_TypeMismatch(t *testing.T) {
	schema := testSingleColSchema()
	_, err := encodeRowValue(schema, []any{"not_uint64"})
	if err != ErrTypeMismatch {
		t.Errorf("expected ErrTypeMismatch, got %v", err)
	}
}

func TestEncodeRowValue_ColumnCountMismatch(t *testing.T) {
	schema := testSingleColSchema()
	_, err := encodeRowValue(schema, []any{uint64(1), uint64(2)})
	if err != ErrColumnCountMismatch {
		t.Errorf("expected ErrColumnCountMismatch, got %v", err)
	}
}

func TestEncodeRowKeyFromPK(t *testing.T) {
	schema := testMultiColSchema()
	k, err := encodeRowKeyFromPK(schema, []any{uint64(42), "hello"})
	if err != nil {
		t.Fatalf("encodeRowKeyFromPK: %v", err)
	}
	if len(k.Bytes()) == 0 {
		t.Error("empty key")
	}
}

// --- Task 3: Insert ---

func TestInsert_ValidRow(t *testing.T) {
	h, cleanup := newTestHeap(t)
	defer cleanup()
	ctx := context.Background()

	tbl, _ := h.OpenTable(testSingleColSchema())
	if err := tbl.Insert(ctx, []any{uint64(1)}); err != nil {
		t.Fatalf("Insert: %v", err)
	}
}

func TestInsert_DuplicatePK(t *testing.T) {
	h, cleanup := newTestHeap(t)
	defer cleanup()
	ctx := context.Background()

	tbl, _ := h.OpenTable(testSingleColSchema())
	tbl.Insert(ctx, []any{uint64(1)})
	err := tbl.Insert(ctx, []any{uint64(1)})
	if err != ErrDuplicateKey {
		t.Errorf("expected ErrDuplicateKey, got %v", err)
	}
}

func TestInsert_WrongColumnCount(t *testing.T) {
	h, cleanup := newTestHeap(t)
	defer cleanup()
	ctx := context.Background()

	tbl, _ := h.OpenTable(testSingleColSchema())
	err := tbl.Insert(ctx, []any{uint64(1), uint64(2)})
	if err != ErrColumnCountMismatch {
		t.Errorf("expected ErrColumnCountMismatch, got %v", err)
	}
}

func TestInsert_WrongType(t *testing.T) {
	h, cleanup := newTestHeap(t)
	defer cleanup()
	ctx := context.Background()

	tbl, _ := h.OpenTable(testSingleColSchema())
	err := tbl.Insert(ctx, []any{"not_a_number"})
	if err != ErrTypeMismatch {
		t.Errorf("expected ErrTypeMismatch, got %v", err)
	}
}

func TestInsert_NilValue(t *testing.T) {
	h, cleanup := newTestHeap(t)
	defer cleanup()
	ctx := context.Background()

	tbl, _ := h.OpenTable(testSingleColSchema())
	err := tbl.Insert(ctx, []any{nil})
	if err != ErrNullNotSupported {
		t.Errorf("expected ErrNullNotSupported, got %v", err)
	}
}

// --- Task 4: Get and Delete ---

func TestGet_Existing(t *testing.T) {
	h, cleanup := newTestHeap(t)
	defer cleanup()
	ctx := context.Background()

	tbl, _ := h.OpenTable(testSingleColSchema())
	tbl.Insert(ctx, []any{uint64(7)})

	vals, err := tbl.Get(ctx, []any{uint64(7)})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if vals[0].(uint64) != uint64(7) {
		t.Errorf("got %v, want 7", vals[0])
	}
}

func TestGet_Missing(t *testing.T) {
	h, cleanup := newTestHeap(t)
	defer cleanup()
	ctx := context.Background()

	tbl, _ := h.OpenTable(testSingleColSchema())
	_, err := tbl.Get(ctx, []any{uint64(999)})
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestDelete_Existing(t *testing.T) {
	h, cleanup := newTestHeap(t)
	defer cleanup()
	ctx := context.Background()

	tbl, _ := h.OpenTable(testSingleColSchema())
	tbl.Insert(ctx, []any{uint64(1)})
	if err := tbl.Delete(ctx, []any{uint64(1)}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := tbl.Get(ctx, []any{uint64(1)})
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound after Delete, got %v", err)
	}
}

func TestDelete_Missing(t *testing.T) {
	h, cleanup := newTestHeap(t)
	defer cleanup()
	ctx := context.Background()

	tbl, _ := h.OpenTable(testSingleColSchema())
	// Delete non-existent key should be no-op.
	if err := tbl.Delete(ctx, []any{uint64(999)}); err != nil {
		t.Fatalf("Delete missing: %v", err)
	}
}

// --- Task 5: Scan ---

func TestScan_Empty(t *testing.T) {
	h, cleanup := newTestHeap(t)
	defer cleanup()
	ctx := context.Background()

	tbl, _ := h.OpenTable(testSingleColSchema())
	rows, err := tbl.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}
	if count != 0 {
		t.Errorf("empty scan returned %d rows", count)
	}
}

func TestScan_Populated(t *testing.T) {
	h, cleanup := newTestHeap(t)
	defer cleanup()
	ctx := context.Background()

	tbl, _ := h.OpenTable(testSingleColSchema())
	for i := uint64(1); i <= 5; i++ {
		tbl.Insert(ctx, []any{i})
	}

	rows, err := tbl.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	defer rows.Close()

	var ids []uint64
	for rows.Next() {
		ids = append(ids, rows.Values()[0].(uint64))
	}
	if len(ids) != 5 {
		t.Fatalf("got %d rows, want 5", len(ids))
	}
	for i, id := range ids {
		if id != uint64(i+1) {
			t.Errorf("row %d: got %d, want %d", i, id, i+1)
		}
	}
}

func TestScan_TableIsolation(t *testing.T) {
	h, cleanup := newTestHeap(t)
	defer cleanup()
	ctx := context.Background()

	schemaA := Schema{TableID: 10, Columns: []Column{{Name: "id", Kind: key.KindUint64}}, PKIndices: []int{0}}
	schemaB := Schema{TableID: 20, Columns: []Column{{Name: "id", Kind: key.KindUint64}}, PKIndices: []int{0}}

	tblA, _ := h.OpenTable(schemaA)
	tblB, _ := h.OpenTable(schemaB)

	tblA.Insert(ctx, []any{uint64(1)})
	tblA.Insert(ctx, []any{uint64(2)})
	tblB.Insert(ctx, []any{uint64(10)})

	// Scan A should only return A's rows.
	rows, _ := tblA.Scan(ctx)
	defer rows.Close()
	count := 0
	for rows.Next() {
		count++
	}
	if count != 2 {
		t.Fatalf("table isolation: got %d rows from A, want 2", count)
	}
}

// --- Task 6: Concurrency ---

func TestConcurrent_Inserts(t *testing.T) {
	h, cleanup := newTestHeap(t)
	defer cleanup()
	ctx := context.Background()

	schema := Schema{
		TableID: 100,
		Columns: []Column{{Name: "id", Kind: key.KindUint64}},
		PKIndices: []int{0},
	}
	tbl, _ := h.OpenTable(schema)

	const goroutines = 50
	done := make(chan error, goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id uint64) {
			done <- tbl.Insert(ctx, []any{id})
		}(uint64(i))
	}

	for i := 0; i < goroutines; i++ {
		if err := <-done; err != nil {
			t.Errorf("concurrent insert %d: %v", i, err)
		}
	}

	rows, _ := tbl.Scan(ctx)
	defer rows.Close()
	count := 0
	for rows.Next() {
		count++
	}
	if count != goroutines {
		t.Errorf("concurrent insert count: got %d, want %d", count, goroutines)
	}
}

func TestConcurrent_ScanAndInsert(t *testing.T) {
	h, cleanup := newTestHeap(t)
	defer cleanup()
	ctx := context.Background()

	schema := Schema{
		TableID: 200,
		Columns: []Column{{Name: "id", Kind: key.KindUint64}},
		PKIndices: []int{0},
	}
	tbl, _ := h.OpenTable(schema)

	// Pre-populate.
	for i := uint64(0); i < 20; i++ {
		tbl.Insert(ctx, []any{i})
	}

	done := make(chan struct{}, 2)
	go func() {
		for i := uint64(20); i < 30; i++ {
			tbl.Insert(ctx, []any{i})
		}
		done <- struct{}{}
	}()
	go func() {
		rows, _ := tbl.Scan(ctx)
		rows.Close()
		done <- struct{}{}
	}()

	<-done
	<-done
}

// --- Task 7: Edge cases ---

func TestCompositePK(t *testing.T) {
	h, cleanup := newTestHeap(t)
	defer cleanup()
	ctx := context.Background()

	tbl, _ := h.OpenTable(testMultiColSchema())

	tbl.Insert(ctx, []any{uint64(1), "a", []byte("first")})
	tbl.Insert(ctx, []any{uint64(1), "b", []byte("second")})
	tbl.Insert(ctx, []any{uint64(2), "a", []byte("third")})

	// Get by composite PK.
	vals, err := tbl.Get(ctx, []any{uint64(1), "b"})
	if err != nil {
		t.Fatalf("Get composite: %v", err)
	}
	if string(vals[2].([]byte)) != "second" {
		t.Errorf("wrong value: %q", vals[2])
	}

	// Scan should return all rows in PK order.
	rows, _ := tbl.Scan(ctx)
	defer rows.Close()
	count := 0
	for rows.Next() {
		count++
	}
	if count != 3 {
		t.Errorf("composite scan: got %d, want 3", count)
	}
}

func TestEmptyBytesValue(t *testing.T) {
	h, cleanup := newTestHeap(t)
	defer cleanup()
	ctx := context.Background()

	schema := Schema{
		TableID: 5,
		Columns: []Column{
			{Name: "id", Kind: key.KindUint64},
			{Name: "data", Kind: key.KindBytes},
		},
		PKIndices: []int{0},
	}
	tbl, _ := h.OpenTable(schema)

	tbl.Insert(ctx, []any{uint64(1), []byte{}})

	vals, err := tbl.Get(ctx, []any{uint64(1)})
	if err != nil {
		t.Fatalf("Get empty bytes: %v", err)
	}
	if b, ok := vals[1].([]byte); !ok || len(b) != 0 {
		t.Errorf("expected empty bytes, got %v", vals[1])
	}
}

func TestMaxSizeString(t *testing.T) {
	h, cleanup := newTestHeap(t)
	defer cleanup()
	ctx := context.Background()

	schema := Schema{
		TableID: 6,
		Columns: []Column{
			{Name: "id", Kind: key.KindUint64},
			{Name: "data", Kind: key.KindString},
		},
		PKIndices: []int{0},
	}
	tbl, _ := h.OpenTable(schema)

	longStr := ""
	for i := 0; i < 500; i++ {
		longStr += "x"
	}

	tbl.Insert(ctx, []any{uint64(1), longStr})
	vals, _ := tbl.Get(ctx, []any{uint64(1)})
	if s, ok := vals[1].(string); !ok || s != longStr {
		t.Errorf("max string round-trip failed")
	}
}

// --- Docs test ---

func TestHeap_DocsFileExists(t *testing.T) {
	data, err := os.ReadFile("../../../../docs/sql_engine_heap.md")
	if err != nil {
		t.Fatalf("docs/sql_engine_heap.md not found: %v", err)
	}
	doc := string(data)
	required := []string{
		"# Plomvix SQL Engine: Table Heap",
		"heap",
		"Strict NOT NULL",
		"Primary Key Uniqueness",
		"Hardcoded MVCC Version",
		"Buffered Iterator",
		"Enterprise Roadmap",
	}
	for _, s := range required {
		if !containsStr(doc, s) {
			t.Errorf("missing required phrase in docs/sql_engine_heap.md: %q", s)
		}
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
