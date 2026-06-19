package sql

import (
	"context"
	"sync/atomic"

	"github.com/plomvix/plomvix/internal/engine"
)

// MutableTableHeap is the engine-facing contract for physical row mutation.
// Enterprise version adds CheckWriteConflict and BatchMutate.
type MutableTableHeap interface {
	CheckWriteConflict(ctx context.Context, tx engine.TxContext, rowID uint64) error
	DeleteByRowID(ctx context.Context, tx engine.TxContext, rowID uint64) error
	UpdateByRowID(ctx context.Context, tx engine.TxContext, rowID uint64, newValues []engine.Datum) error
	BatchMutate(ctx context.Context, tx engine.TxContext, mutations []RowMutation) (int, error)
}

// RowMutation describes a single row operation within a BatchMutate call.
type RowMutation struct {
	RowID     uint64
	Op        MutationOp
	NewValues []engine.Datum // nil for OpDelete
}

// MutationOp classifies a row mutation.
type MutationOp uint8

const (
	OpDelete MutationOp = iota
	OpUpdate
)

// heapMutableAdapter bridges MutableTableHeap to the internal heap.Table.
// Shares activePins and lastWriteTxID with the parent tableHeapAdapter.
type heapMutableAdapter struct {
	a              *tableHeapAdapter
	heapGeneration atomic.Uint64
}

var _ MutableTableHeap = (*heapMutableAdapter)(nil)

// AsMutable wraps a tableHeapAdapter as a MutableTableHeap for UPDATE/DELETE.
func AsMutable(a *tableHeapAdapter) MutableTableHeap {
	return &heapMutableAdapter{a: a}
}

func (m *heapMutableAdapter) checkWriteConflictLocked(_ engine.TxContext, rowID uint64) error {
	gen, _, err := engine.DecodeRowID(rowID)
	if err != nil {
		return err
	}
	if gen != m.heapGeneration.Load() {
		return ErrStaleRowID
	}
	return nil
}

func (m *heapMutableAdapter) CheckWriteConflict(ctx context.Context, tx engine.TxContext, rowID uint64) error {
	m.a.mu.Lock()
	defer m.a.mu.Unlock()
	return m.checkWriteConflictLocked(tx, rowID)
}

func (m *heapMutableAdapter) DeleteByRowID(ctx context.Context, tx engine.TxContext, rowID uint64) error {
	m.a.activePins.Add(1)
	defer m.a.activePins.Add(-1)
	m.a.mu.Lock()
	defer m.a.mu.Unlock()
	if err := m.checkWriteConflictLocked(tx, rowID); err != nil {
		return err
	}
	_, _, err := engine.DecodeRowID(rowID)
	if err != nil {
		return err
	}
	return ErrHeapMutationUnsupported // TODO: heap physical offset addressing
}

func (m *heapMutableAdapter) UpdateByRowID(ctx context.Context, tx engine.TxContext, rowID uint64, newValues []engine.Datum) error {
	m.a.activePins.Add(1)
	defer m.a.activePins.Add(-1)
	m.a.mu.Lock()
	defer m.a.mu.Unlock()
	if err := m.checkWriteConflictLocked(tx, rowID); err != nil {
		return err
	}
	_, _, err := engine.DecodeRowID(rowID)
	if err != nil {
		return err
	}
	return ErrHeapMutationUnsupported // TODO: heap physical offset addressing
}

func (m *heapMutableAdapter) BatchMutate(ctx context.Context, tx engine.TxContext, mutations []RowMutation) (int, error) {
	m.a.activePins.Add(1)
	defer m.a.activePins.Add(-1)
	m.a.mu.Lock()
	defer m.a.mu.Unlock()
	for _, mut := range mutations {
		if err := m.checkWriteConflictLocked(tx, mut.RowID); err != nil {
			return 0, err
		}
	}
	rowsAffected := 0
	for _, mut := range mutations {
		if err := m.applyLocked(ctx, tx, mut); err != nil {
			return rowsAffected, err
		}
		rowsAffected++
	}
	return rowsAffected, nil
}

func (m *heapMutableAdapter) applyLocked(_ context.Context, _ engine.TxContext, mut RowMutation) error {
	_, _, err := engine.DecodeRowID(mut.RowID)
	if err != nil {
		return err
	}
	return ErrHeapMutationUnsupported // TODO: heap physical offset addressing
}

// BumpGeneration increments the heap generation. Called after vacuum/compaction.
func (m *heapMutableAdapter) BumpGeneration() error {
	if m.a.activePins.Load() != 0 {
		return ErrVacuumBlockedByActivePins
	}
	m.heapGeneration.Add(1)
	return nil
}

func (m *heapMutableAdapter) Generation() uint64 {
	return m.heapGeneration.Load()
}
