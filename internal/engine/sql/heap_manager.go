// Package sql implements the SQL query execution engine for Plomvix.
// heap_manager.go bridges the physical Heap layer to the planner's abstract
// TableHeap/TableRegistry contracts and provides DDL table lifecycle.
package sql

import (
	"context"
	"fmt"
	"io"
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
	CreateTableHeap(ctx context.Context, tableID uint64, schema heap.Schema) error
}

// heapManager implements TableManager backed by an on-disk KVStore.
type heapManager struct {
	mu     sync.RWMutex
	store  kv.KVStore
	heaps  map[uint64]heap.Table
}

// NewHeapManager creates a new TableManager.
func NewHeapManager(store kv.KVStore) TableManager {
	return &heapManager{
		store: store,
		heaps: make(map[uint64]heap.Table),
	}
}

// GetTableHeap returns the planner.TableHeap for a given table ID.
func (m *heapManager) GetTableHeap(tableID uint64) (planner.TableHeap, error) {
	m.mu.RLock()
	t, ok := m.heaps[tableID]
	m.mu.RUnlock()
	if !ok {
		return nil, planner.ErrTableHeapNotFound
	}
	return &tableHeapAdapter{t: t}, nil
}

// CreateTableHeap validates the schema and opens a new physical heap table.
func (m *heapManager) CreateTableHeap(ctx context.Context, tableID uint64, schema heap.Schema) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.heaps[tableID]; exists {
		return ErrTableExists
	}
	h := heap.New(m.store)
	schema.TableID = tableID
	t, err := h.OpenTable(schema)
	if err != nil {
		return err
	}
	m.heaps[tableID] = t
	return nil
}

// tableHeapAdapter wraps a heap.Table to satisfy planner.TableHeap.
type tableHeapAdapter struct {
	t heap.Table
}

func (a *tableHeapAdapter) Scan(ctx context.Context, tx engine.TxContext) (planner.HeapScanIterator, error) {
	rows, err := a.t.Scan(ctx, heap.Tx{ID: tx.ReadTxID})
	if err != nil {
		return nil, err
	}
	return &rowsAdapter{rows: rows}, nil
}

// rowsAdapter wraps heap.Rows to satisfy planner.HeapScanIterator.
type rowsAdapter struct {
	rows heap.Rows
}

func (r *rowsAdapter) Next(ctx context.Context) ([]byte, error) {
	if !r.rows.Next() {
		if err := r.rows.Err(); err != nil {
			return nil, err
		}
		return nil, io.EOF
	}
	vals := r.rows.Values()
	return encodeRowToBytes(vals)
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
		return nil, fmt.Errorf("sql: decode row: %w", err)
	}
	rawVals, err := key.DecodeStorageComposite(k)
	if err != nil {
		return nil, fmt.Errorf("sql: decode row: %w", err)
	}
	row := make(engine.Row, len(rawVals))
	for i, v := range rawVals {
		row[i] = keyValueToDatum(v, schema.Columns[i].Type)
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
