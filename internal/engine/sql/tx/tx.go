// Package tx provides a basic transaction manager for the SQL engine.
// It allocates monotonically increasing uint64 timestamps used as MVCC
// transaction IDs for SELECT (read) and DDL/DML (write) operations.
package tx

import "sync/atomic"

// Manager allocates monotonically increasing transaction IDs.
// It is safe for concurrent use.
type Manager struct {
	nextRead  atomic.Uint64
	nextWrite atomic.Uint64
}

// NewManager creates a new TxManager.
func NewManager(readBase, writeBase uint64) *Manager {
	m := &Manager{}
	m.nextRead.Store(readBase)
	m.nextWrite.Store(writeBase)
	return m
}

// NextReadTx allocates a new read transaction ID.
func (m *Manager) NextReadTx() uint64 {
	return m.nextRead.Add(1)
}

// NextWriteTx allocates a new write transaction ID.
func (m *Manager) NextWriteTx() uint64 {
	return m.nextWrite.Add(1)
}

// CurrentReadTx returns the current read transaction ID without incrementing.
func (m *Manager) CurrentReadTx() uint64 {
	return m.nextRead.Load()
}

// CurrentWriteTx returns the current write transaction ID without incrementing.
func (m *Manager) CurrentWriteTx() uint64 {
	return m.nextWrite.Load()
}
