// Package heap provides relational table row storage on top of the on-disk
// B+ Tree KVStore. It maps schemas, columns, and primary keys onto KV keys
// and storage-composite-encoded row values.
//
// This is the Enterprise tier — MVCC via append-only versioning, NULL support
// via null-bitmask prefix, tombstone-based deletes, and manual Vacuum for
// garbage collection.
package heap

import (
	"context"
	"errors"
	"sync"

	"github.com/plomvix/plomvix/internal/engine/sql/key"
	"github.com/plomvix/plomvix/internal/engine/sql/kv"
)

// BasicVersion is the hardcoded MVCC version for the Basic tier (kept for compat).
const BasicVersion uint64 = 0

// Enterprise row flags.
const (
	FlagNormal    byte = 0x00
	FlagTombstone byte = 0x01
)

// Column defines a single column in a table schema.
type Column struct {
	Name string
	Kind key.Kind
}

// Schema defines the structure of a table.
type Schema struct {
	TableID   uint64
	Columns   []Column
	PKIndices []int
}

// Tx represents a transaction context for MVCC visibility.
type Tx struct {
	ID uint64 // Must be > 0. ID 0 is reserved for Basic-tier non-MVCC rows.
}

// Heap manages table access over a KVStore.
type Heap struct {
	store kv.KVStore
}

// New creates a new Heap backed by the given KVStore.
func New(store kv.KVStore) *Heap {
	return &Heap{store: store}
}

// Table provides relational operations for a specific schema.
type Table interface {
	Insert(ctx context.Context, tx Tx, values []any) error
	Update(ctx context.Context, tx Tx, pkValues []any, newValues []any) error
	Get(ctx context.Context, tx Tx, pkValues []any) ([]any, error)
	Delete(ctx context.Context, tx Tx, pkValues []any) error
	Scan(ctx context.Context, tx Tx) (Rows, error)

	// Vacuum reclaims space. Caller MUST ensure olderThan < oldest active reader Tx.ID.
	Vacuum(ctx context.Context, olderThan uint64) error
}

// Rows is a buffered iterator facade for scan results.
type Rows interface {
	Next() bool
	Values() []any
	Err() error
	Close() error
}

// Sentinel errors.
var (
	ErrInvalidSchema       = errors.New("heap: invalid schema definition")
	ErrColumnCountMismatch = errors.New("heap: value count does not match schema")
	ErrTypeMismatch        = errors.New("heap: value type does not match column kind")
	ErrNullNotSupported    = errors.New("heap: NULL values are not supported in Basic tier")
	ErrDuplicateKey        = errors.New("heap: primary key violation")
	ErrKeyNotFound         = errors.New("heap: row not found")
	ErrTxConflict          = errors.New("heap: transaction version conflict (non-monotonic Tx.ID)")
	ErrInvalidTx           = errors.New("heap: invalid transaction ID (must be > 0)")
	ErrPrimaryKeyUpdate    = errors.New("heap: updating primary key columns is not supported")
)

// OpenTable validates the schema and returns a Table interface for operations.
func (h *Heap) OpenTable(schema Schema) (Table, error) {
	if len(schema.Columns) == 0 {
		return nil, ErrInvalidSchema
	}
	if len(schema.PKIndices) == 0 {
		return nil, ErrInvalidSchema
	}
	for _, idx := range schema.PKIndices {
		if idx < 0 || idx >= len(schema.Columns) {
			return nil, ErrInvalidSchema
		}
	}
	seen := make(map[string]bool)
	for _, col := range schema.Columns {
		if seen[col.Name] {
			return nil, ErrInvalidSchema
		}
		seen[col.Name] = true
	}
	for _, col := range schema.Columns {
		switch col.Kind {
		case key.KindUint64, key.KindInt64, key.KindString, key.KindBytes:
		default:
			return nil, ErrInvalidSchema
		}
	}
	return &table{
		store:  h.store,
		schema: schema,
	}, nil
}

// table is the concrete implementation of Table.
type table struct {
	mu     sync.RWMutex
	store  kv.KVStore
	schema Schema
}

var _ Table = (*table)(nil)
