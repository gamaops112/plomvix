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
	sel, ok := stmt.RawAST().(*vitess.Select)
	if !ok {
		return nil, ErrUnsupportedFeature
	}

	// Handle FROM clause: support single table and JOINs.
	op, engSchema, err := translateFrom(ctx, cat, tables, decoder, req, sel.From)
	if err != nil {
		return nil, err
	}

	// Bind WHERE clause if present.
	if sel.Where != nil {
		ps := GetPlannerSchema(op)
		pred, bindErr := BindWherePlanner(sel.Where.Expr, ps)
		if bindErr != nil {
			return nil, bindErr
		}
		if pred != nil {
			op = NewFilterNode(op, pred)
		}
	}

	// Bind projections.
	if sel.SelectExprs != nil {
		ps := GetPlannerSchema(op)
		projs, outSchema, bindErr := BindProjectionPlanner(sel.SelectExprs, ps)
		if bindErr != nil {
			return nil, bindErr
		}
		op = NewProjectNode(op, projs, outSchema.ToEngineSchema())
	} else {
		_ = engSchema
	}

	return op, nil
}

// translateFrom recursively translates a Vitess TableExpr into a Volcano operator.
func translateFrom(
	ctx context.Context,
	cat catalog.Catalog,
	tables TableRegistry,
	decoder RowDecoder,
	req *engine.Request,
	from vitess.TableExprs,
) (Operator, engine.Schema, error) {
	if len(from) == 0 {
		return nil, engine.Schema{}, ErrUnsupportedFeature
	}
	if len(from) == 1 {
		return translateTableExpr(ctx, cat, tables, decoder, req, from[0])
	}
	// Multiple tables (implicit cross join): translate leftmost, then join rest.
	left, _, err := translateTableExpr(ctx, cat, tables, decoder, req, from[0])
	if err != nil {
		return nil, engine.Schema{}, err
	}
	for _, te := range from[1:] {
		right, _, err := translateTableExpr(ctx, cat, tables, decoder, req, te)
		if err != nil {
			return nil, engine.Schema{}, err
		}
		left = NewNestedLoopJoinNode(left, right, nil)
	}
	return left, GetPlannerSchema(left).ToEngineSchema(), nil
}

// translateTableExpr translates a single TableExpr (simple table or join).
func translateTableExpr(
	ctx context.Context,
	cat catalog.Catalog,
	tables TableRegistry,
	decoder RowDecoder,
	req *engine.Request,
	te vitess.TableExpr,
) (Operator, engine.Schema, error) {
	switch expr := te.(type) {
	case *vitess.AliasedTableExpr:
		return translateAliasedTable(ctx, cat, tables, decoder, req, expr)
	case *vitess.JoinTableExpr:
		return translateJoinTableExpr(ctx, cat, tables, decoder, req, expr)
	default:
		return nil, engine.Schema{}, ErrUnsupportedFeature
	}
}

// translateAliasedTable translates a simple table reference.
func translateAliasedTable(
	ctx context.Context,
	cat catalog.Catalog,
	tables TableRegistry,
	decoder RowDecoder,
	req *engine.Request,
	ate *vitess.AliasedTableExpr,
) (Operator, engine.Schema, error) {
	tname, ok := ate.Expr.(vitess.TableName)
	if !ok {
		return nil, engine.Schema{}, ErrUnsupportedFeature
	}
	name := tname.Name.String()
	if name == "" {
		return nil, engine.Schema{}, ErrUnsupportedFeature
	}
	info, err := cat.GetTable(ctx, name)
	if err != nil {
		return nil, engine.Schema{}, err
	}
	engSchema, err := schema.Decode(info.SchemaPayload)
	if err != nil {
		return nil, engine.Schema{}, err
	}
	heap, err := tables.GetTableHeap(info.TableID)
	if err != nil {
		return nil, engine.Schema{}, ErrTableHeapNotFound
	}
	op := NewSeqScanNode(heap, decoder, engSchema, req.TxContext)
	return op, engSchema, nil
}

// translateJoinTableExpr translates a JOIN expression.
func translateJoinTableExpr(
	ctx context.Context,
	cat catalog.Catalog,
	tables TableRegistry,
	decoder RowDecoder,
	req *engine.Request,
	jte *vitess.JoinTableExpr,
) (Operator, engine.Schema, error) {
	left, _, err := translateTableExpr(ctx, cat, tables, decoder, req, jte.LeftExpr)
	if err != nil {
		return nil, engine.Schema{}, err
	}
	right, _, err := translateTableExpr(ctx, cat, tables, decoder, req, jte.RightExpr)
	if err != nil {
		return nil, engine.Schema{}, err
	}

	var cond BoundExpr
	if jte.Condition.On != nil {
		ps := GetPlannerSchema(left)
		rps := GetPlannerSchema(right)
		combined := PlannerSchema{Fields: make([]SchemaField, 0, len(ps.Fields)+len(rps.Fields))}
		combined.Fields = append(combined.Fields, ps.Fields...)
		combined.Fields = append(combined.Fields, rps.Fields...)
		cond, err = BindWherePlanner(jte.Condition.On, combined)
		if err != nil {
			return nil, engine.Schema{}, err
		}
	}

	op := NewNestedLoopJoinNode(left, right, cond)
	return op, GetPlannerSchema(op).ToEngineSchema(), nil
}
