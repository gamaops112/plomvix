package planner

import (
	"context"

	"github.com/plomvix/plomvix/internal/catalog"
	"github.com/plomvix/plomvix/internal/engine"
	"github.com/plomvix/plomvix/internal/engine/sql/schema"
	vitess "vitess.io/vitess/go/vt/sqlparser"
)

// --- FilterNode ---

// FilterNode passes through rows matching a predicate.
type FilterNode struct {
	child Operator
	pred  BoundExpr
}

// NewFilterNode creates a filter operator.
func NewFilterNode(child Operator, pred BoundExpr) *FilterNode {
	return &FilterNode{child: child, pred: pred}
}

func (n *FilterNode) Open(ctx context.Context) error { return n.child.Open(ctx) }

func (n *FilterNode) Next(ctx context.Context) (engine.Row, error) {
	for {
		row, err := n.child.Next(ctx)
		if err != nil {
			return engine.Row{}, err
		}
		d, err := n.pred.Eval(row)
		if err != nil {
			return engine.Row{}, err
		}
		if b, ok := d.Value.(bool); ok && b {
			return row, nil
		}
	}
}

func (n *FilterNode) Close() error          { return n.child.Close() }
func (n *FilterNode) Schema() engine.Schema { return n.child.Schema() }

// --- ProjectNode ---

// ProjectNode applies a projection to each input row.
type ProjectNode struct {
	child     Operator
	exprs     []ProjectionExpr
	outSchema engine.Schema
}

// NewProjectNode creates a projection operator.
func NewProjectNode(child Operator, exprs []ProjectionExpr, outSchema engine.Schema) *ProjectNode {
	return &ProjectNode{child: child, exprs: exprs, outSchema: outSchema}
}

func (n *ProjectNode) Open(ctx context.Context) error { return n.child.Open(ctx) }

func (n *ProjectNode) Next(ctx context.Context) (engine.Row, error) {
	row, err := n.child.Next(ctx)
	if err != nil {
		return engine.Row{}, err
	}
	out := engine.Row{Datums: make([]engine.Datum, len(n.exprs))}
	for i, pe := range n.exprs {
		d, err := pe.Expr.Eval(row)
		if err != nil {
			return engine.Row{}, err
		}
		out.Datums[i] = d
	}
	return out, nil
}

func (n *ProjectNode) Close() error          { return n.child.Close() }
func (n *ProjectNode) Schema() engine.Schema { return n.outSchema.DeepCopy() }

// --- Translate ---

// Translate builds a Volcano physical plan from a parsed statement.
func Translate(
	ctx context.Context,
	cat catalog.Catalog,
	tables TableRegistry,
	decoder RowDecoder,
	req *engine.Request,
) (Operator, error) {
	stmt := req.Stmt
	targetTables := stmt.TargetTables()
	if len(targetTables) > 1 {
		return nil, ErrUnsupportedFeature
	}

	// Get the single table.
	tableName := targetTables[0]
	info, err := cat.GetTable(ctx, tableName)
	if err != nil {
		return nil, err
	}

	// Decode the schema.
	engSchema, err := schema.Decode(info.SchemaPayload)
	if err != nil {
		return nil, err
	}

	// Get the table heap.
	heap, err := tables.GetTableHeap(info.TableID)
	if err != nil {
		return nil, ErrTableHeapNotFound
	}

	// Build the operator tree.
	var op Operator = NewSeqScanNode(heap, decoder, engSchema, req.TxContext)

	// Bind WHERE clause if present.
	sel, ok := stmt.RawAST().(*vitess.Select)
	if ok && sel.Where != nil {
		pred, bindErr := BindWhere(sel.Where.Expr, engSchema)
		if bindErr != nil {
			return nil, bindErr
		}
		if pred != nil {
			op = NewFilterNode(op, pred)
		}
	}

	// Bind projections.
	if ok && sel.SelectExprs != nil {
		projs, outSchema, bindErr := BindProjection(sel.SelectExprs, engSchema)
		if bindErr != nil {
			return nil, bindErr
		}
		op = NewProjectNode(op, projs, outSchema)
	}

	return op, nil
}
