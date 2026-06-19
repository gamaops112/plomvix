// Package engine defines the core contracts for pluggable query execution
// engines in Plomvix. It provides types for schemas, rows, datum values,
// transaction context, and the Engine interface that the Router dispatches to.
package engine

import (
	"context"

	"github.com/plomvix/plomvix/internal/sqlparser"
)

// Type classifies the runtime type of a column or datum.
type Type uint8

const (
	TypeNull Type = iota
	TypeInt64
	TypeUint64
	TypeFloat64
	TypeBool
	TypeString
	TypeBytes
)

// Column describes a single column in a schema.
type Column struct {
	Name string
	Type Type
}

// Schema describes the column layout of a row set.
type Schema struct {
	Columns []Column
}

// DeepCopy returns a completely independent copy of the schema.
func (s Schema) DeepCopy() Schema {
	cp := Schema{Columns: make([]Column, len(s.Columns))}
	copy(cp.Columns, s.Columns)
	return cp
}

// Datum is a single typed value in a row. Value MUST only be one of:
// nil, int64, uint64, float64, bool, string, or []byte.
type Datum struct {
	Type  Type
	Value any
}

// DeepCopy allocates new []byte if Type == TypeBytes. Immutable types are copied by value.
func (d Datum) DeepCopy() Datum {
	if d.Type == TypeBytes {
		if b, ok := d.Value.([]byte); ok {
			cp := make([]byte, len(b))
			copy(cp, b)
			return Datum{Type: d.Type, Value: cp}
		}
	}
	return Datum{Type: d.Type, Value: d.Value}
}

// Row is a slice of Datum values representing one result row.
type Row []Datum

// DeepCopy calls DeepCopy on every Datum.
func (r Row) DeepCopy() Row {
	cp := make(Row, len(r))
	for i, d := range r {
		cp[i] = d.DeepCopy()
	}
	return cp
}

// TxContext holds transaction-scoped metadata for query execution.
type TxContext struct {
	ReadTxID  uint64
	WriteTxID uint64 // Used for DDL/DML mutation timestamps. 0 = read-only.
}

// RowStream is an iterator over query result rows.
type RowStream interface {
	Next(ctx context.Context) (Row, error) // Returns io.EOF when exhausted
	Schema() Schema                        // MUST return a deep copy
	Close() error                          // Idempotent
}

// Result encapsulates the outcome of an engine execution.
// For SELECT: Stream is non-nil, RowsAffected is 0.
// For DDL: Stream is nil, RowsAffected is 0, Message has status.
// For DML: Stream may be nil, RowsAffected is non-zero.
type Result struct {
	Stream       RowStream // Non-nil for SELECT. nil for DDL/DML.
	RowsAffected int64     // Non-zero for DML. 0 for DDL/SELECT.
	Message      string    // Human-readable status for DDL.
}

// Request encapsulates a parsed statement with execution context.
type Request struct {
	Stmt      sqlparser.Statement
	UserID    uint64
	TxContext TxContext
}

// Engine is a pluggable query execution backend.
type Engine interface {
	Name() string
	Execute(ctx context.Context, req *Request) (*Result, error)
}
