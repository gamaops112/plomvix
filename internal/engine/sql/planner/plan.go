// Package planner provides the Volcano-model query planner for the SQL engine.
// It translates parsed ASTs into physical operator trees (SeqScan, Filter,
// Project) and provides expression binding from Vitess AST nodes.
package planner

import (
	"context"
	"errors"
	"io"

	"github.com/plomvix/plomvix/internal/engine"
)

// Sentinel errors.
var (
	ErrUnsupportedFeature = errors.New("planner: unsupported feature in basic tier")
	ErrTableHeapNotFound  = errors.New("planner: table heap not found for table ID")
)

// TableRegistry abstracts physical heap lookup by TableID.
type TableRegistry interface {
	GetTableHeap(tableID uint64) (TableHeap, error)
}

// TableHeap provides a scan iterator over a table's physical rows.
type TableHeap interface {
	Scan(ctx context.Context, tx engine.TxContext) (HeapScanIterator, error)
}

// HeapScanIterator iterates over encoded tuples from a table heap.
// Returns encodedTuple, rowID (physicalOffset+1), and error. Never emits rowID == 0.
type HeapScanIterator interface {
	Next(ctx context.Context) (encodedTuple []byte, rowID uint64, err error)
	Close() error
}

// RowDecoder decodes an encoded tuple into an engine.Row.
type RowDecoder interface {
	Decode(encodedTuple []byte, schema engine.Schema) (engine.Row, error)
}

// Operator is a node in the Volcano physical plan tree.
type Operator interface {
	Open(ctx context.Context) error
	Next(ctx context.Context) (engine.Row, error)
	Close() error
	Schema() engine.Schema
}

// SeqScanNode scans all rows from a TableHeap.
type SeqScanNode struct {
	heap    TableHeap
	decoder RowDecoder
	schema  engine.Schema
	tx      engine.TxContext
	iter    HeapScanIterator
}

// NewSeqScanNode creates a sequential scan operator.
func NewSeqScanNode(heap TableHeap, decoder RowDecoder, schema engine.Schema, tx engine.TxContext) *SeqScanNode {
	return &SeqScanNode{heap: heap, decoder: decoder, schema: schema, tx: tx}
}

func (n *SeqScanNode) Open(ctx context.Context) error {
	iter, err := n.heap.Scan(ctx, n.tx)
	if err != nil {
		return err
	}
	n.iter = iter
	return nil
}

func (n *SeqScanNode) Next(ctx context.Context) (engine.Row, error) {
	if n.iter == nil {
		return engine.Row{}, io.EOF
	}
	encoded, rowID, err := n.iter.Next(ctx)
	if err != nil {
		return engine.Row{}, err
	}
	row, err := n.decoder.Decode(encoded, n.schema)
	if err != nil {
		return engine.Row{}, err
	}
	row.RowID = rowID
	return row.DeepCopy(), nil
}

func (n *SeqScanNode) Close() error {
	if n.iter != nil {
		err := n.iter.Close()
		n.iter = nil
		return err
	}
	return nil
}

func (n *SeqScanNode) Schema() engine.Schema { return n.schema.DeepCopy() }
