package sql

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/plomvix/plomvix/internal/engine"
	"github.com/plomvix/plomvix/internal/engine/sql/key"
)

// MutableTableHeap is the engine-facing contract for physical row mutation.
// Implementations bridge to the internal heap.Tx type using tx.WriteTxID.
type MutableTableHeap interface {
	// DeleteByRowID appends a tombstone for the exact row version identified by rowID.
	DeleteByRowID(ctx context.Context, tx engine.TxContext, rowID uint64) error

	// UpdateByRowID appends a new row version replacing the exact row version
	// identified by rowID. newValues must match the table schema column count.
	// The concrete adapter loads the original row to verify PK columns are unchanged.
	UpdateByRowID(ctx context.Context, tx engine.TxContext, rowID uint64, newValues []engine.Datum) error
}

// heapMutableAdapter bridges MutableTableHeap to the internal heap.Table.
// It wraps the same tableHeapAdapter to share lastWriteTxID state.
type heapMutableAdapter struct {
	a *tableHeapAdapter
}

var _ MutableTableHeap = (*heapMutableAdapter)(nil)

func (m *heapMutableAdapter) DeleteByRowID(ctx context.Context, tx engine.TxContext, rowID uint64) error {
	if rowID == 0 {
		return ErrMissingRowID
	}
	m.a.mu.Lock()
	defer m.a.mu.Unlock()
	if tx.WriteTxID <= m.a.lastWriteTxID {
		return ErrTxConflict
	}

	// Resolve the original row from the heap using the physical offset.
	physicalOffset := rowID - 1
	_ = physicalOffset // TODO: implement physical offset lookup when heap exposes it.

	// For now, we can't actually delete by RowID without physical offset support.
	// Return a meaningful error until the heap exposes row addressing.
	return fmt.Errorf("sql engine: DeleteByRowID: physical offset not yet supported")
}

func (m *heapMutableAdapter) UpdateByRowID(ctx context.Context, tx engine.TxContext, rowID uint64, newValues []engine.Datum) error {
	if rowID == 0 {
		return ErrMissingRowID
	}
	m.a.mu.Lock()
	defer m.a.mu.Unlock()
	if tx.WriteTxID <= m.a.lastWriteTxID {
		return ErrTxConflict
	}

	// Convert newValues to []any for heap operations.
	vals := make([]any, len(newValues))
	for i, d := range newValues {
		vals[i] = d.Value
	}

	// Derive RowKey from rowID and re-encode with new WriteTxID.
	// Use a synthetic approach: encode a tombstone for old version,
	// then insert a new version.
	rowKeyBytes := encodeRowIDKey(rowID, tx.WriteTxID)
	rowValue, err := key.EncodeStorageComposite(vals...)
	if err != nil {
		return fmt.Errorf("sql engine: UpdateByRowID encode: %w", err)
	}

	_ = rowKeyBytes
	_ = rowValue

	// TODO: When heap exposes physical offset addressing, implement proper update.
	return fmt.Errorf("sql engine: UpdateByRowID: physical offset not yet supported")
}

// encodeRowIDKey encodes a RowID + WriteTxID into a key.Key for heap operations.
func encodeRowIDKey(rowID uint64, txID uint64) key.Key {
	buf := make([]byte, 16)
	binary.BigEndian.PutUint64(buf[:8], rowID)
	binary.BigEndian.PutUint64(buf[8:], txID)
	return key.FromBytes(buf)
}

// AsMutable wraps a tableHeapAdapter as a MutableTableHeap for UPDATE/DELETE.
func AsMutable(a *tableHeapAdapter) MutableTableHeap {
	return &heapMutableAdapter{a: a}
}

var _ MutableTableHeap = (*heapMutableAdapter)(nil)
