// Package sql implements the SQL query execution engine for Plomvix.
// It translates parsed ASTs into Volcano-model physical plans and executes
// them against the on-disk Table Heap.
package sql

import (
	"context"

	"github.com/plomvix/plomvix/internal/catalog"
	"github.com/plomvix/plomvix/internal/engine"
	"github.com/plomvix/plomvix/internal/engine/sql/planner"
)

// SQLEngine implements engine.Engine for SQL queries.
type SQLEngine struct {
	catalog catalog.Catalog
	tables  planner.TableRegistry
	decoder planner.RowDecoder
}

// New creates a new SQL engine with the given dependencies.
func New(cat catalog.Catalog, tables planner.TableRegistry, decoder planner.RowDecoder) *SQLEngine {
	return &SQLEngine{catalog: cat, tables: tables, decoder: decoder}
}

// Name returns the engine identifier.
func (e *SQLEngine) Name() string { return "sql" }

// Execute translates the parsed statement into a physical plan and executes it.
func (e *SQLEngine) Execute(ctx context.Context, req *engine.Request) (engine.RowStream, error) {
	op, err := planner.Translate(ctx, e.catalog, e.tables, e.decoder, req)
	if err != nil {
		return nil, err
	}

	if err := op.Open(ctx); err != nil {
		_ = op.Close()
		return nil, err
	}

	return &operatorStream{op: op}, nil
}

// operatorStream adapts a planner.Operator into an engine.RowStream.
type operatorStream struct {
	op planner.Operator
}

func (s *operatorStream) Next(ctx context.Context) (engine.Row, error) {
	return s.op.Next(ctx)
}

func (s *operatorStream) Schema() engine.Schema {
	return s.op.Schema().DeepCopy()
}

func (s *operatorStream) Close() error {
	return s.op.Close()
}

// Compile-time checks.
var _ engine.Engine = (*SQLEngine)(nil)
var _ engine.RowStream = (*operatorStream)(nil)
