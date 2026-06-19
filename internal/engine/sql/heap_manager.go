// Package sql implements the SQL query execution engine for Plomvix.
// heap_manager.go bridges the physical Heap layer to the planner's abstract
// TableHeap/TableRegistry contracts and provides DDL table lifecycle.
package sql

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/plomvix/plomvix/internal/engine"
	"github.com/plomvix/plomvix/internal/engine/sql/heap"
	"github.com/plomvix/plomvix/internal/engine/sql/key"
	"github.com/plomvix/plomvix/internal/engine/sql/kv"
	"github.com/plomvix/plomvix/internal/engine/sql/planner"
)

// TableManager extends planner.TableRegistry with the ability to create
// new physical heaps at runtime (DDL).
type TableManager interface {
	planner.TableRegistry
	// CreateTableHeap initializes a physical heap for the given table ID.
	// Returns the planner.TableHeap and the deterministic file path.
	CreateTableHeap(ctx context.Context, tableID uint64, schema heap.Schema) (planner.TableHeap, string, error)
	// HeapPath returns the deterministic file path for a table ID.
	HeapPath(tableID uint64) string
}

// heapManager implements TableManager backed by an on-disk KVStore.
type heapManager struct {
	mu       sync.RWMutex
	store    kv.KVStore
	dataDir  string
	heaps    map[uint64]heap.Table
	adapters map[uint64]*tableHeapAdapter
}

// NewHeapManager creates a new TableManager.
func NewHeapManager(store kv.KVStore, dataDir string) TableManager {
	return &heapManager{
		store:    store,
		dataDir:  dataDir,
		heaps:    make(map[uint64]heap.Table),
		adapters: make(map[uint64]*tableHeapAdapter),
	}
}

// GetTableHeap returns the planner.TableHeap for a given table ID.
func (m *heapManager) GetTableHeap(tableID uint64) (planner.TableHeap, error) {
	m.mu.RLock()
	adapter, ok := m.adapters[tableID]
	m.mu.RUnlock()
	if !ok {
		return nil, planner.ErrTableHeapNotFound
	}
	return adapter, nil
}

// CreateTableHeap validates the schema and opens a new physical heap table.
// Returns the TableHeap and the deterministic file path for cleanup.
func (m *heapManager) CreateTableHeap(ctx context.Context, tableID uint64, schema heap.Schema) (planner.TableHeap, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.heaps[tableID]; exists {
		return nil, "", ErrTableExists
	}
	h := heap.New(m.store)
	schema.TableID = tableID
	t, err := h.OpenTable(schema)
	if err != nil {
		return nil, "", err
	}
	m.heaps[tableID] = t
	adapter := &tableHeapAdapter{t: t}
	m.adapters[tableID] = adapter
	path := m.heapPath(tableID)
	return adapter, path, nil
}

// HeapPath returns the deterministic file path for a table ID.
func (m *heapManager) HeapPath(tableID uint64) string {
	return m.heapPath(tableID)
}

// heapPath returns the deterministic file path for a table heap.
func (m *heapManager) heapPath(tableID uint64) string {
	return filepath.Join(m.dataDir, fmt.Sprintf("heap_%d.db", tableID))
}

// RemoveHeap physically deletes a heap file. Used for transactional cleanup.
func (m *heapManager) RemoveHeap(tableID uint64) error {
	return os.Remove(m.heapPath(tableID))
}

// tableHeapAdapter wraps a heap.Table to satisfy planner.TableHeap.
type tableHeapAdapter struct {
	t             heap.Table
	mu            sync.Mutex
	lastWriteTxID uint64
}

func (a *tableHeapAdapter) Scan(ctx context.Context, tx engine.TxContext) (planner.HeapScanIterator, error) {
	rows, err := a.t.Scan(ctx, heap.Tx{ID: tx.ReadTxID})
	if err != nil {
		return nil, err
	}
	return &rowsAdapter{rows: rows}, nil
}

// Insert satisfies InsertableTableHeap for DML execution.
func (a *tableHeapAdapter) Insert(ctx context.Context, tx engine.TxContext, row engine.Row) error {
	vals := make([]any, len(row.Datums))
	for i, d := range row.Datums {
		vals[i] = d.Value
	}
	return a.t.Insert(ctx, heap.Tx{ID: tx.WriteTxID}, vals)
}

// InsertBatch acquires the write lock once, validates WriteTxID monotonicity,
// appends all rows, and commits lastWriteTxID. Returns (rowsAffected, error).
func (a *tableHeapAdapter) InsertBatch(ctx context.Context, tx engine.TxContext, rows []engine.Row) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	// WriteTxID monotonic guard: tx.WriteTxID must be > lastWriteTxID.
	if tx.WriteTxID <= a.lastWriteTxID {
		return 0, ErrTxConflict
	}
	heapTx := heap.Tx{ID: tx.WriteTxID}
	for _, row := range rows {
		vals := make([]any, len(row.Datums))
		for i, d := range row.Datums {
			vals[i] = d.Value
		}
		if err := a.t.Insert(ctx, heapTx, vals); err != nil {
			return 0, err
		}
	}
	a.lastWriteTxID = tx.WriteTxID
	return len(rows), nil
}

// BeginInsertStream acquires the write lock, validates WriteTxID, and returns
// a stream writer for row-by-row appending.
func (a *tableHeapAdapter) BeginInsertStream(ctx context.Context, tx engine.TxContext) (InsertStream, error) {
	a.mu.Lock()
	if tx.WriteTxID <= a.lastWriteTxID {
		a.mu.Unlock()
		return nil, ErrTxConflict
	}
	return &heapInsertStream{a: a, heapTx: heap.Tx{ID: tx.WriteTxID}, writeTxID: tx.WriteTxID}, nil
}

// heapInsertStream implements InsertStream for heap-backed tables.
type heapInsertStream struct {
	a         *tableHeapAdapter
	heapTx    heap.Tx
	writeTxID uint64
	aborted   bool
}

func (s *heapInsertStream) Append(ctx context.Context, row engine.Row) error {
	vals := make([]any, len(row.Datums))
	for i, d := range row.Datums {
		vals[i] = d.Value
	}
	return s.a.t.Insert(ctx, s.heapTx, vals)
}

func (s *heapInsertStream) Commit() error {
	s.a.lastWriteTxID = s.writeTxID
	s.a.mu.Unlock()
	return nil
}

func (s *heapInsertStream) Abort() error {
	if !s.aborted {
		s.aborted = true
		s.a.mu.Unlock()
	}
	return nil
}

var _ InsertableTableHeap = (*tableHeapAdapter)(nil)

// rowsAdapter wraps heap.Rows to satisfy planner.HeapScanIterator.
type rowsAdapter struct {
	rows    heap.Rows
	counter uint64
}

func (r *rowsAdapter) Next(ctx context.Context) ([]byte, uint64, error) {
	if !r.rows.Next() {
		if err := r.rows.Err(); err != nil {
			return nil, 0, err
		}
		return nil, 0, io.EOF
	}
	vals := r.rows.Values()
	// RowID = physicalOffset + 1. We use a synthetic counter since the heap
	// doesn't expose physical offsets yet.
	rowID := r.counter + 1
	r.counter++
	encoded, err := encodeRowToBytes(vals)
	return encoded, rowID, err
}

func (r *rowsAdapter) Close() error { return r.rows.Close() }

// encodeRowToBytes re-encodes a row of decoded values to a storage-composite
// byte slice. The planner's RowDecoder reverses this format.
func encodeRowToBytes(vals []any) ([]byte, error) {
	if len(vals) == 0 {
		return nil, nil
	}
	k, err := key.EncodeStorageComposite(vals...)
	if err != nil {
		return nil, fmt.Errorf("sql: encode row: %w", err)
	}
	return k.Bytes(), nil
}

// sqlRowDecoder implements planner.RowDecoder by decoding storage-composite
// encoded tuples into engine.Row values.
type sqlRowDecoder struct{}

// NewRowDecoder creates a new RowDecoder.
func NewRowDecoder() planner.RowDecoder { return &sqlRowDecoder{} }

func (d *sqlRowDecoder) Decode(encodedTuple []byte, schema engine.Schema) (engine.Row, error) {
	if len(encodedTuple) == 0 {
		return engine.Row{}, nil
	}
	k, err := key.ParseStorageCompositeKey(encodedTuple, schemaColumnKinds(schema))
	if err != nil {
		return engine.Row{}, fmt.Errorf("sql: decode row: %w", err)
	}
	rawVals, err := key.DecodeStorageComposite(k)
	if err != nil {
		return engine.Row{}, fmt.Errorf("sql: decode row: %w", err)
	}
	row := engine.Row{Datums: make([]engine.Datum, len(rawVals))}
	for i, v := range rawVals {
		row.Datums[i] = keyValueToDatum(v, schema.Columns[i].Type)
	}
	return row, nil
}

// schemaColumnKinds extracts key.Kind values from an engine.Schema.
func schemaColumnKinds(s engine.Schema) []key.Kind {
	kinds := make([]key.Kind, len(s.Columns))
	for i, col := range s.Columns {
		kinds[i] = engineTypeToKeyKind(col.Type)
	}
	return kinds
}

// engineTypeToKeyKind maps an engine.Type to a key.Kind.
func engineTypeToKeyKind(t engine.Type) key.Kind {
	switch t {
	case engine.TypeInt64:
		return key.KindInt64
	case engine.TypeUint64:
		return key.KindUint64
	case engine.TypeString:
		return key.KindString
	case engine.TypeBytes:
		return key.KindBytes
	default:
		return key.KindBytes
	}
}

// keyValueToDatum converts a raw decoded value to an engine.Datum.
func keyValueToDatum(v any, t engine.Type) engine.Datum {
	switch t {
	case engine.TypeInt64:
		if val, ok := v.(int64); ok {
			return engine.Datum{Type: t, Value: val}
		}
		if val, ok := v.(uint64); ok {
			return engine.Datum{Type: t, Value: int64(val)}
		}
	case engine.TypeUint64:
		if val, ok := v.(uint64); ok {
			return engine.Datum{Type: t, Value: val}
		}
	case engine.TypeString:
		if val, ok := v.(string); ok {
			return engine.Datum{Type: t, Value: val}
		}
	case engine.TypeBytes:
		if val, ok := v.([]byte); ok {
			return engine.Datum{Type: t, Value: val}
		}
	}
	return engine.Datum{Type: engine.TypeNull, Value: nil}
}

var _ planner.TableRegistry = (*heapManager)(nil)
var _ planner.RowDecoder = (*sqlRowDecoder)(nil)
