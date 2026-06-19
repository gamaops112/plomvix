package sql

import (
	"testing"

	"github.com/plomvix/plomvix/internal/engine"
)

func TestEncodeRowID_RoundTrip(t *testing.T) {
	tests := []struct{ gen, offset uint64 }{
		{0, 0},
		{0, 1},
		{0, 100},
		{1, 0},
		{42, 999},
		{0xFFFFFFFF, 0xFFFFFFFE},
	}
	for _, tt := range tests {
		id, err := engine.EncodeRowID(tt.gen, tt.offset)
		if err != nil {
			t.Errorf("EncodeRowID(%d, %d): %v", tt.gen, tt.offset, err)
			continue
		}
		gen, off, err := engine.DecodeRowID(id)
		if err != nil {
			t.Errorf("DecodeRowID(%d): %v", id, err)
			continue
		}
		if gen != tt.gen || off != tt.offset {
			t.Errorf("round-trip: got (%d, %d), want (%d, %d)", gen, off, tt.gen, tt.offset)
		}
	}
}

func TestEncodeRowID_OffsetOverflow(t *testing.T) {
	_, err := engine.EncodeRowID(0, 0xFFFFFFFF) // MaxUint32, too large (must be <= MaxUint32-1)
	if err != engine.ErrRowIDOffsetOverflow {
		t.Errorf("got %v, want ErrRowIDOffsetOverflow", err)
	}
}

func TestEncodeRowID_GenerationOverflow(t *testing.T) {
	_, err := engine.EncodeRowID(0x100000000, 0) // > MaxUint32
	if err != engine.ErrRowIDGenerationOverflow {
		t.Errorf("got %v, want ErrRowIDGenerationOverflow", err)
	}
}

func TestDecodeRowID_Missing(t *testing.T) {
	_, _, err := engine.DecodeRowID(0) // zero low bits = sentinel
	if err != engine.ErrMissingRowID {
		t.Errorf("got %v, want ErrMissingRowID", err)
	}
	_, _, err = engine.DecodeRowID(0x0000000100000000) // low bits zero even with generation
	if err != engine.ErrMissingRowID {
		t.Errorf("got %v, want ErrMissingRowID", err)
	}
}

func TestStaleRowID_GenMismatch(t *testing.T) {
	eng, cleanup := newTestEngineCustom(t, 1000, nil)
	defer cleanup()
	ctx := t.Context()
	createStmt := parseStmt(t, "CREATE TABLE stale_test (id bigint)")
	_, err := eng.Execute(ctx, &engine.Request{Stmt: createStmt, UserID: 1})
	if err != nil {
		t.Fatalf("CREATE: %v", err)
	}
	// Get the mutable adapter and force a generation bump.
	info, _ := eng.catalog.GetTable(ctx, "stale_test")
	th, _ := eng.tables.GetTableHeap(info.TableID)
	adapter := th.(*tableHeapAdapter)
	mh := AsMutable(adapter)
	mhImpl := mh.(*heapMutableAdapter)
	// Bump generation.
	mhImpl.BumpGeneration()
	// Now encode a RowID with the OLD generation (0) and check conflict.
	oldRowID, _ := engine.EncodeRowID(0, 1)
	err = mh.CheckWriteConflict(ctx, engine.TxContext{ReadTxID: 1}, oldRowID)
	if err != ErrStaleRowID {
		t.Errorf("got %v, want ErrStaleRowID", err)
	}
}

func TestWriteConflict_Detected(t *testing.T) {
	// Skip: requires heap-level raw version scanning which is not yet exposed.
	t.Skip("write-write conflict detection requires raw version scan from heap layer")
}

func TestConcurrentScan_SerializesWrite(t *testing.T) {
	// Skip: requires proper table-level locking in the adapter.
	t.Skip("concurrent scan/write serialization requires adapter lock model")
}

func TestVacuumBlockedByActivePins(t *testing.T) {
	eng, cleanup := newTestEngineCustom(t, 1000, nil)
	defer cleanup()
	ctx := t.Context()
	createStmt := parseStmt(t, "CREATE TABLE pin_test (id bigint)")
	_, err := eng.Execute(ctx, &engine.Request{Stmt: createStmt, UserID: 1})
	if err != nil {
		t.Fatalf("CREATE: %v", err)
	}
	info, _ := eng.catalog.GetTable(ctx, "pin_test")
	th, _ := eng.tables.GetTableHeap(info.TableID)
	adapter := th.(*tableHeapAdapter)
	mh := AsMutable(adapter)
	mhImpl := mh.(*heapMutableAdapter)
	// Pin an active scan on the shared adapter.
	adapter.activePins.Add(1)
	err = mhImpl.BumpGeneration()
	if err != ErrVacuumBlockedByActivePins {
		t.Errorf("got %v, want ErrVacuumBlockedByActivePins", err)
	}
	adapter.activePins.Add(-1)
	err = mhImpl.BumpGeneration()
	if err != nil {
		t.Errorf("BumpGeneration after pin release: %v", err)
	}
}

func TestMissingRowID_Sentinel(t *testing.T) {
	if engine.ErrMissingRowID != ErrMissingRowID {
		t.Error("ErrMissingRowID alias mismatch")
	}
}

func TestCheckWriteConflict_StaleGeneration(t *testing.T) {
	eng, cleanup := newTestEngineCustom(t, 1000, nil)
	defer cleanup()
	ctx := t.Context()
	createStmt := parseStmt(t, "CREATE TABLE cwc_test (id bigint)")
	_, err := eng.Execute(ctx, &engine.Request{Stmt: createStmt, UserID: 1})
	if err != nil {
		t.Fatalf("CREATE: %v", err)
	}
	info, _ := eng.catalog.GetTable(ctx, "cwc_test")
	th, _ := eng.tables.GetTableHeap(info.TableID)
	adapter := th.(*tableHeapAdapter)
	mh := AsMutable(adapter)
	// Create a RowID at current generation (0).
	rowID, _ := engine.EncodeRowID(0, 1)
	// Bump generation via vacuum simulation.
	mhImpl := mh.(*heapMutableAdapter)
	mhImpl.BumpGeneration()
	// Now CheckWriteConflict with old RowID → ErrStaleRowID.
	err = mh.CheckWriteConflict(ctx, engine.TxContext{ReadTxID: 1}, rowID)
	if err != ErrStaleRowID {
		t.Errorf("got %v, want ErrStaleRowID", err)
	}
}
