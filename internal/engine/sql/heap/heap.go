// Package heap provides relational table row storage on top of the on-disk
// B+ Tree KVStore. It maps schemas, columns, and primary keys onto KV keys
// and storage-composite-encoded row values.
//
// This is the Basic tier — strict NOT NULL, PK uniqueness via read-before-write,
// hardcoded MVCC version 0, and buffered scan iterators.
package heap

import (
	"context"
	"errors"

	"github.com/plomvix/plomvix/internal/engine/sql/key"
	"github.com/plomvix/plomvix/internal/engine/sql/kv"
)

// BasicVersion is the hardcoded MVCC version for the Basic tier.
const BasicVersion uint64 = 0

// Column defines a single column in a table schema.
type Column struct {
	Name string
	Kind key.Kind
}

// Schema defines the structure of a table.
type Schema struct {
	TableID   uint64
	Columns   []Column
	PKIndices []int // Indices into Columns that form the Primary Key. Must not be empty.
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
	Insert(ctx context.Context, values []any) error
	Get(ctx context.Context, pkValues []any) ([]any, error)
	Delete(ctx context.Context, pkValues []any) error
	Scan(ctx context.Context) (Rows, error)
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
)

// OpenTable validates the schema and returns a Table interface for operations.
func (h *Heap) OpenTable(schema Schema) (Table, error) {
	if len(schema.Columns) == 0 {
		return nil, ErrInvalidSchema
	}
	if len(schema.PKIndices) == 0 {
		return nil, ErrInvalidSchema
	}

	// Validate PK indices.
	for _, idx := range schema.PKIndices {
		if idx < 0 || idx >= len(schema.Columns) {
			return nil, ErrInvalidSchema
		}
	}

	// Check for duplicate column names.
	seen := make(map[string]bool)
	for _, col := range schema.Columns {
		if seen[col.Name] {
			return nil, ErrInvalidSchema
		}
		seen[col.Name] = true
	}

	// Validate column kinds.
	for _, col := range schema.Columns {
		switch col.Kind {
		case key.KindUint64, key.KindInt64, key.KindString, key.KindBytes:
			// OK
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
	store  kv.KVStore
	schema Schema
}

// compile-time interface checks
var _ Table = (*table)(nil)
