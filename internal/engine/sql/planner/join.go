package planner

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/plomvix/plomvix/internal/engine"
)

// Sentinel errors for column resolution.
var (
	ErrAmbiguousColumn = errors.New("planner: column reference is ambiguous")
	ErrColumnNotFound  = errors.New("planner: column not found")
)

// SchemaField extends engine.Column with a table qualifier for join resolution.
type SchemaField struct {
	engine.Column
	TableAlias string
}

// PlannerSchema tracks column metadata with optional table qualifiers.
type PlannerSchema struct {
	Fields []SchemaField
}

// ResolveColumn finds the index of a column by qualifier and name.
// If qualifier is empty, searches for a unique match across all fields.
func (ps PlannerSchema) ResolveColumn(qualifier, name string) (int, error) {
	nameLower := strings.ToLower(name)
	qualLower := strings.ToLower(qualifier)

	if qualLower == "" {
		matchIdx := -1
		for i, f := range ps.Fields {
			if strings.ToLower(f.Name) == nameLower {
				if matchIdx != -1 {
					return -1, ErrAmbiguousColumn
				}
				matchIdx = i
			}
		}
		if matchIdx == -1 {
			return -1, ErrColumnNotFound
		}
		return matchIdx, nil
	}

	for i, f := range ps.Fields {
		if strings.ToLower(f.TableAlias) == qualLower && strings.ToLower(f.Name) == nameLower {
			return i, nil
		}
	}
	return -1, ErrColumnNotFound
}

// ToEngineSchema exports all fields to engine.Schema.
func (ps PlannerSchema) ToEngineSchema() engine.Schema {
	cols := make([]engine.Column, len(ps.Fields))
	for i, f := range ps.Fields {
		cols[i] = f.Column
	}
	return engine.Schema{Columns: cols}
}

// PlannerSchemaFromEngineSchema wraps an engine.Schema with a single table alias.
func PlannerSchemaFromEngineSchema(schema engine.Schema, tableAlias string) PlannerSchema {
	fields := make([]SchemaField, len(schema.Columns))
	for i, col := range schema.Columns {
		fields[i] = SchemaField{Column: col, TableAlias: tableAlias}
	}
	return PlannerSchema{Fields: fields}
}

// GetPlannerSchema returns the PlannerSchema for any operator, falling back
// to an un-aliased mapping if not explicitly tracked.
func GetPlannerSchema(op Operator) PlannerSchema {
	if po, ok := op.(interface{ PlannerSchema() PlannerSchema }); ok {
		return po.PlannerSchema()
	}
	return PlannerSchemaFromEngineSchema(op.Schema(), "")
}

// NestedLoopJoinNode performs a Nested Loop Join (inner or left outer).
type NestedLoopJoinNode struct {
	left          Operator
	right         Operator
	cond          BoundExpr
	isLeftJoin    bool
	logger        *slog.Logger
	plannerSchema PlannerSchema
	outSchema     engine.Schema

	leftRow     engine.Row
	leftMatched bool

	probeRows  int
	outputRows int
	probeTime  time.Duration
}

// NewNestedLoopJoinNode creates a NestedLoopJoinNode from left and right operators,
// an optional join condition, a left-join flag, and a logger.
func NewNestedLoopJoinNode(left, right Operator, cond BoundExpr, isLeftJoin bool, logger *slog.Logger) *NestedLoopJoinNode {
	leftPS := GetPlannerSchema(left)
	rightPS := GetPlannerSchema(right)
	fields := make([]SchemaField, 0, len(leftPS.Fields)+len(rightPS.Fields))
	fields = append(fields, leftPS.Fields...)
	fields = append(fields, rightPS.Fields...)
	plannerSchema := PlannerSchema{Fields: fields}
	return &NestedLoopJoinNode{
		left:          left,
		right:         right,
		cond:          cond,
		isLeftJoin:    isLeftJoin,
		logger:        logger,
		plannerSchema: plannerSchema,
		outSchema:     plannerSchema.ToEngineSchema(),
	}
}

func (n *NestedLoopJoinNode) PlannerSchema() PlannerSchema { return n.plannerSchema }

func (n *NestedLoopJoinNode) Open(ctx context.Context) error {
	if err := n.left.Open(ctx); err != nil {
		return err
	}
	if err := n.right.Open(ctx); err != nil {
		_ = n.left.Close()
		return err
	}
	n.probeRows = 0
	n.outputRows = 0
	n.probeTime = 0
	lr, err := n.left.Next(ctx)
	if err != nil {
		if errors.Is(err, io.EOF) {
			n.leftRow = engine.Row{}
			return nil
		}
		_ = n.left.Close()
		_ = n.right.Close()
		return err
	}
	n.leftRow = lr.DeepCopy()
	n.leftMatched = false
	n.probeRows++
	return nil
}

func (n *NestedLoopJoinNode) Next(ctx context.Context) (engine.Row, error) {
	start := time.Now()
	defer func() { n.probeTime += time.Since(start) }()

	for {
		if len(n.leftRow.Datums) == 0 {
			return engine.Row{}, io.EOF
		}

		rr, err := n.right.Next(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				// LEFT JOIN null-padding: emit outer row with NULL right side.
				if n.isLeftJoin && !n.leftMatched {
					padded := nullPadRight(n.leftRow, n.right.Schema())
					_ = n.right.Close()
					if errR := n.right.Open(ctx); errR != nil {
						return engine.Row{}, errR
					}
					lr, errA := n.left.Next(ctx)
					if errA != nil {
						n.leftRow = engine.Row{}
						if errors.Is(errA, io.EOF) {
							n.outputRows++
							return padded, nil
						}
						return engine.Row{}, errA
					}
					n.leftRow = lr.DeepCopy()
					n.leftMatched = false
					n.probeRows++
					n.outputRows++
					return padded, nil
				}
				// Inner join: re-open inner, advance outer.
				if err := n.right.Close(); err != nil {
					return engine.Row{}, err
				}
				if err := n.right.Open(ctx); err != nil {
					return engine.Row{}, err
				}
				lr, err := n.left.Next(ctx)
				if err != nil {
					n.leftRow = engine.Row{}
					if errors.Is(err, io.EOF) {
						return engine.Row{}, io.EOF
					}
					return engine.Row{}, err
				}
				n.leftRow = lr.DeepCopy()
				n.leftMatched = false
				n.probeRows++
				continue
			}
			return engine.Row{}, err
		}

		joinedRow := engine.Row{
			Datums: make([]engine.Datum, 0, len(n.leftRow.Datums)+len(rr.Datums)),
			RowID:  0,
		}
		joinedRow.Datums = append(joinedRow.Datums, n.leftRow.Datums...)
		joinedRow.Datums = append(joinedRow.Datums, rr.Datums...)

		if n.cond != nil {
			d, err := n.cond.Eval(joinedRow)
			if err != nil {
				return engine.Row{}, err
			}
			if b, ok := d.Value.(bool); ok && b {
				n.leftMatched = true
				n.outputRows++
				return joinedRow, nil
			}
			continue
		}
		n.leftMatched = true
		n.outputRows++
		return joinedRow, nil
	}
}

func (n *NestedLoopJoinNode) Close() error {
	if n.logger != nil {
		n.logger.Info("joins: NestedLoopJoin metrics",
			slog.String("join_algorithm", "nested_loop"),
			slog.Bool("left_outer", n.isLeftJoin),
			slog.Int("probe_rows", n.probeRows),
			slog.Int("output_rows", n.outputRows),
			slog.Duration("probe_time_ms", n.probeTime),
		)
	}
	n.leftRow = engine.Row{}
	errL := n.left.Close()
	errR := n.right.Close()
	if errL != nil {
		return errL
	}
	return errR
}

func (n *NestedLoopJoinNode) Schema() engine.Schema { return n.outSchema.DeepCopy() }
