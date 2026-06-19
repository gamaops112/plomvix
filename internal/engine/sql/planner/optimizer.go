package planner

import (
	"context"
	"log/slog"

	"github.com/plomvix/plomvix/internal/catalog"
	"github.com/plomvix/plomvix/internal/engine"

	vitess "vitess.io/vitess/go/vt/sqlparser"
)

// JoinStrategy selects the optimal join algorithm.
type JoinStrategy int

const (
	JoinAuto JoinStrategy = iota
	JoinHash
	JoinNestedLoop
)

// SelectJoinStrategy inspects the join condition and table metadata to choose
// the best join algorithm. Returns HashJoin for equijoins, NestedLoop otherwise.
func SelectJoinStrategy(_ context.Context, _ catalog.Catalog, ps PlannerSchema, cond vitess.Expr) JoinStrategy {
	if cond == nil {
		return JoinNestedLoop
	}
	// Check for equijoin predicate: t1.col = t2.col
	if comp, ok := cond.(*vitess.ComparisonExpr); ok && comp.Operator == vitess.EqualOp {
		// Both sides must be column references from different tables.
		leftCol, lok := comp.Left.(*vitess.ColName)
		rightCol, rok := comp.Right.(*vitess.ColName)
		if lok && rok {
			lq := leftCol.Qualifier.Name.String()
			rq := rightCol.Qualifier.Name.String()
			if lq != "" && rq != "" && lq != rq {
				// Valid equijoin — can use hash join.
				return JoinHash
			}
		}
	}
	return JoinNestedLoop
}

// BuildJoin builds a join operator using the selected strategy.
func BuildJoin(
	ctx context.Context,
	cat catalog.Catalog,
	decoder RowDecoder,
	req *engine.Request,
	left, right Operator,
	cond BoundExpr,
	rawCond vitess.Expr,
	isLeftJoin bool,
	logger *slog.Logger,
) Operator {
	strategy := SelectJoinStrategy(ctx, cat, GetPlannerSchema(left), rawCond)

	switch strategy {
	case JoinHash:
		// Extract key indices from equijoin condition.
		if comp, ok := rawCond.(*vitess.ComparisonExpr); ok && comp.Operator == vitess.EqualOp {
			leftCol := comp.Left.(*vitess.ColName)
			rightCol := comp.Right.(*vitess.ColName)
			_, li, _ := resolveKeyIndex(GetPlannerSchema(left), leftCol.Qualifier.Name.String(), leftCol.Name.String())
			_, ri, _ := resolveKeyIndex(GetPlannerSchema(right), rightCol.Qualifier.Name.String(), rightCol.Name.String())
			_ = decoder
			if li >= 0 && ri >= 0 {
				return NewHashJoinNode(left, right, li, ri, isLeftJoin, cond, logger)
			}
		}
		fallthrough
	default:
		return NewNestedLoopJoinNode(left, right, cond, isLeftJoin, logger)
	}
}

func resolveKeyIndex(ps PlannerSchema, qualifier, name string) (SchemaField, int, error) {
	idx, err := ps.ResolveColumn(qualifier, name)
	if err != nil {
		return SchemaField{}, -1, err
	}
	return ps.Fields[idx], idx, nil
}
