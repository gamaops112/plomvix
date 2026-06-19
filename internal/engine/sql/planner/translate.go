package planner

import (
	"context"
	"fmt"
	"strings"

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

	// Handle GROUP BY and aggregates.
	if sel.GroupBy != nil && len(sel.GroupBy.Exprs) > 0 || hasAggregates(sel.SelectExprs) {
		op, err = translateGroupBy(ctx, op, sel, &engSchema)
		if err != nil {
			return nil, err
		}
	} else if sel.SelectExprs != nil {
		// Bind projections without aggregation.
		ps := GetPlannerSchema(op)
		projs, outSchema, bindErr := BindProjectionPlanner(sel.SelectExprs, ps)
		if bindErr != nil {
			return nil, bindErr
		}
		op = NewProjectNode(op, projs, outSchema.ToEngineSchema())
	}

	// Handle ORDER BY.
	if len(sel.OrderBy) > 0 {
		op, err = translateOrderBy(op, sel.OrderBy)
		if err != nil {
			return nil, err
		}
	}

	// Handle LIMIT.
	if sel.Limit != nil {
		op = translateLimit(op, sel.Limit)
	}

	return op, nil
}

// translateOrderBy parses ORDER BY and wraps the operator in a SortNode.
func translateOrderBy(op Operator, orders vitess.OrderBy) (Operator, error) {
	schema := op.Schema()
	var keys []SortKey
	for _, o := range orders {
		cn, ok := o.Expr.(*vitess.ColName)
		if !ok {
			return nil, ErrUnsupportedFeature
		}
		idx := colIndex(cn.Name.String(), schema)
		if idx < 0 {
			return nil, fmt.Errorf("planner: ORDER BY column %q not found", cn.Name.String())
		}
		keys = append(keys, SortKey{ColIdx: idx, Desc: o.Direction == vitess.DescOrder})
	}
	return NewSortNode(op, keys), nil
}

// translateGroupBy handles GROUP BY and aggregate functions.
func translateGroupBy(_ context.Context, op Operator, sel *vitess.Select, engSchema *engine.Schema) (Operator, error) {
	schema := op.Schema()
	var groupKeys []int
	if sel.GroupBy != nil {
		for _, gb := range sel.GroupBy.Exprs {
			cn, ok := gb.(*vitess.ColName)
			if !ok {
				return nil, ErrUnsupportedFeature
			}
			idx := colIndex(cn.Name.String(), schema)
			if idx < 0 {
				return nil, fmt.Errorf("planner: GROUP BY column %q not found", cn.Name.String())
			}
			groupKeys = append(groupKeys, idx)
		}
	}

	// Resolve aggregates from SelectExprs.
	var aggs []AggRequest
	for _, expr := range sel.SelectExprs.Exprs {
		ae, ok := expr.(*vitess.AliasedExpr)
		if !ok {
			return nil, ErrUnsupportedFeature
		}
		if isAggregateExpr(ae.Expr) {
			agg, err := resolveAggregate(ae, schema)
			if err != nil {
				return nil, err
			}
			aggs = append(aggs, agg)
		}
	}
	_ = engSchema
	return NewHashAggNode(op, groupKeys, aggs), nil
}

// hasAggregates checks if any select expression is an aggregate function.
func hasAggregates(exprs *vitess.SelectExprs) bool {
	if exprs == nil {
		return false
	}
	for _, expr := range exprs.Exprs {
		if ae, ok := expr.(*vitess.AliasedExpr); ok && isAggregateExpr(ae.Expr) {
			return true
		}
	}
	return false
}

func isAggregateExpr(expr vitess.Expr) bool {
	if fc, ok := expr.(*vitess.FuncExpr); ok {
		name := strings.ToUpper(fc.Name.String())
		return name == "COUNT" || name == "SUM" || name == "MIN" || name == "MAX"
	}
	return false
}

// translateLimit wraps the operator in a LimitNode.
func translateLimit(op Operator, limit *vitess.Limit) Operator {
	var rowCount, offset int64
	if limit.Rowcount != nil {
		if lit, ok := limit.Rowcount.(*vitess.Literal); ok {
			fmt.Sscanf(lit.Val, "%d", &rowCount)
		}
	}
	if limit.Offset != nil {
		if lit, ok := limit.Offset.(*vitess.Literal); ok {
			fmt.Sscanf(lit.Val, "%d", &offset)
		}
	}
	return NewLimitNode(op, rowCount, offset)
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
		left = NewNestedLoopJoinNode(left, right, nil, false, nil)
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
	// Reject RIGHT outer join.
	if jte.Join == vitess.RightJoinType {
		return nil, engine.Schema{}, ErrUnsupportedFeature
	}
	isLeftJoin := jte.Join == vitess.LeftJoinType

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

	op := BuildJoin(ctx, cat, decoder, req, left, right, cond, jte.Condition.On, isLeftJoin, nil)
	return op, GetPlannerSchema(op).ToEngineSchema(), nil
}
