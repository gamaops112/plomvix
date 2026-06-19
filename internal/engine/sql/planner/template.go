package planner

import (
	"context"
	"fmt"

	"github.com/plomvix/plomvix/internal/catalog"
	"github.com/plomvix/plomvix/internal/engine"
	"github.com/plomvix/plomvix/internal/engine/sql/schema"
	vitess "vitess.io/vitess/go/vt/sqlparser"
)

// SchemaVersionProvider abstracts the Catalog's DDL version counter.
type SchemaVersionProvider interface {
	SchemaVersion() uint64
}

// PlanTemplate is the cacheable, stateless output of the planner.
// It is safe to read concurrently. Callers MUST NOT mutate its fields.
type PlanTemplate struct {
	TableID      uint64
	InputSchema  engine.Schema
	OutputSchema engine.Schema
	WhereExpr    BoundExpr
	Projections  []ProjectionExpr
}

// Build instantiates a fresh Operator tree from this template.
// The returned Operator has NOT been opened. Each call produces an
// independent tree with no shared mutable state.
func (t *PlanTemplate) Build(heap TableHeap, decoder RowDecoder) Operator {
	var op Operator = NewSeqScanNode(heap, decoder, t.InputSchema)
	if t.WhereExpr != nil {
		op = NewFilterNode(op, t.WhereExpr)
	}
	if t.Projections != nil {
		op = NewProjectNode(op, t.Projections, t.OutputSchema)
	}
	return op
}

// Plan resolves schemas, binds expressions, and produces a cacheable PlanTemplate.
// It does NOT create Operators or open iterators. Catalog I/O is done here.
func Plan(
	ctx context.Context,
	cat catalog.Catalog,
	req *engine.Request,
) (*PlanTemplate, error) {
	stmt := req.Stmt
	targetTables := stmt.TargetTables()
	if len(targetTables) == 0 {
		return nil, fmt.Errorf("planner: %w", ErrUnsupportedFeature)
	}
	if len(targetTables) > 1 {
		return nil, ErrUnsupportedFeature
	}

	tableName := targetTables[0]
	info, err := cat.GetTable(ctx, tableName)
	if err != nil {
		return nil, err
	}

	engSchema, err := schema.Decode(info.SchemaPayload)
	if err != nil {
		return nil, err
	}

	tmpl := &PlanTemplate{
		TableID:     info.TableID,
		InputSchema: engSchema.DeepCopy(),
	}

	sel, ok := stmt.RawAST().(*vitess.Select)
	if ok && sel.Where != nil {
		pred, bindErr := BindWhere(sel.Where.Expr, engSchema)
		if bindErr != nil {
			return nil, bindErr
		}
		tmpl.WhereExpr = pred
	}

	if ok && sel.SelectExprs != nil {
		projs, outSchema, bindErr := BindProjection(sel.SelectExprs, engSchema)
		if bindErr != nil {
			return nil, bindErr
		}
		tmpl.Projections = projs
		tmpl.OutputSchema = outSchema.DeepCopy()
	} else {
		tmpl.OutputSchema = engSchema.DeepCopy()
	}

	return tmpl, nil
}
