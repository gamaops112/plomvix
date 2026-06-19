package planner

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/plomvix/plomvix/internal/engine"
	vitess "vitess.io/vitess/go/vt/sqlparser"
)

// AggOp classifies an aggregate function.
type AggOp uint8

const (
	AggCount AggOp = iota
	AggSum
	AggMin
	AggMax
)

// AggRequest specifies an aggregate to compute.
type AggRequest struct {
	Op     AggOp
	ColIdx int // -1 for COUNT(*)
}

type aggAccumulator struct {
	op     AggOp
	count  int64
	sum    engine.Datum
	min    engine.Datum
	max    engine.Datum
	hasVal bool
}

func (acc *aggAccumulator) accumulate(val engine.Datum) {
	if acc.op == AggCount {
		acc.count++
		return
	}
	if val.Value == nil {
		return
	}
	if !acc.hasVal {
		acc.sum = val.DeepCopy()
		acc.min = val.DeepCopy()
		acc.max = val.DeepCopy()
		acc.hasVal = true
		return
	}
	switch acc.op {
	case AggSum:
		acc.sum = addDatums(acc.sum, val)
	case AggMin:
		if datumLess(val, acc.min) {
			acc.min = val.DeepCopy()
		}
	case AggMax:
		if datumLess(acc.max, val) {
			acc.max = val.DeepCopy()
		}
	}
}

func (acc *aggAccumulator) result() engine.Datum {
	if acc.op == AggCount {
		return engine.Datum{Type: engine.TypeInt64, Value: acc.count}
	}
	if !acc.hasVal {
		return engine.Datum{Type: engine.TypeNull, Value: nil}
	}
	switch acc.op {
	case AggSum:
		return acc.sum
	case AggMin:
		return acc.min
	case AggMax:
		return acc.max
	}
	return engine.Datum{Type: engine.TypeNull, Value: nil}
}

// addDatums adds two numeric datums for SUM.
func addDatums(a, b engine.Datum) engine.Datum {
	switch a.Type {
	case engine.TypeInt64:
		av := a.Value.(int64)
		bv := int64(0)
		if fb, ok := b.Value.(float64); ok {
			// Promote to float.
			return engine.Datum{Type: engine.TypeFloat64, Value: float64(av) + fb}
		}
		if ib, ok := b.Value.(int64); ok {
			bv = ib
		}
		return engine.Datum{Type: engine.TypeInt64, Value: av + bv}
	case engine.TypeFloat64:
		av := a.Value.(float64)
		bv := float64(0)
		if ib, ok := b.Value.(int64); ok {
			bv = float64(ib)
		} else if fb, ok := b.Value.(float64); ok {
			bv = fb
		}
		return engine.Datum{Type: engine.TypeFloat64, Value: av + bv}
	default:
		return a
	}
}

// HashAggNode computes grouped aggregates using an in-memory hash map.
type HashAggNode struct {
	child     Operator
	groupKeys []int
	aggs      []AggRequest
	outSchema engine.Schema

	groups     []engine.Row
	aggResults []engine.Row
	outputIdx  int
	opened     bool
}

// NewHashAggNode creates a HashAggNode.
func NewHashAggNode(child Operator, groupKeys []int, aggs []AggRequest) *HashAggNode {
	childSchema := child.Schema()
	var outCols []engine.Column
	for _, gk := range groupKeys {
		outCols = append(outCols, childSchema.Columns[gk])
	}
	for _, agg := range aggs {
		name := aggName(agg)
		typ := aggResultType(agg, childSchema)
		outCols = append(outCols, engine.Column{Name: name, Type: typ})
	}
	return &HashAggNode{
		child:     child,
		groupKeys: groupKeys,
		aggs:      aggs,
		outSchema: engine.Schema{Columns: outCols},
	}
}

func aggName(a AggRequest) string {
	switch a.Op {
	case AggCount:
		return "count"
	case AggSum:
		return "sum"
	case AggMin:
		return "min"
	case AggMax:
		return "max"
	}
	return "?"
}

func aggResultType(a AggRequest, schema engine.Schema) engine.Type {
	if a.Op == AggCount {
		return engine.TypeInt64
	}
	if a.ColIdx >= 0 && a.ColIdx < len(schema.Columns) {
		return schema.Columns[a.ColIdx].Type
	}
	return engine.TypeNull
}

func (n *HashAggNode) Open(ctx context.Context) error {
	if err := n.child.Open(ctx); err != nil {
		return err
	}
	n.groups = nil
	n.aggResults = nil
	n.outputIdx = 0

	groupMap := make(map[string][]aggAccumulator)
	var groupRows []engine.Row

	for {
		row, err := n.child.Next(ctx)
		if err != nil {
			if err == io.EOF {
				break
			}
			_ = n.child.Close()
			return err
		}
		// Serialize group key.
		var keyParts []string
		var groupRowDatums []engine.Datum
		for _, gk := range n.groupKeys {
			d := row.Datums[gk]
			groupRowDatums = append(groupRowDatums, d)
			keyParts = append(keyParts, serializeDatum(d))
		}
		keyStr := strings.Join(keyParts, "\x00")

		accums, exists := groupMap[keyStr]
		if !exists {
			accums = make([]aggAccumulator, len(n.aggs))
			for i, agg := range n.aggs {
				accums[i] = aggAccumulator{op: agg.Op}
			}
			groupMap[keyStr] = accums
			groupRows = append(groupRows, engine.Row{Datums: groupRowDatums})
		}
		for i, agg := range n.aggs {
			var val engine.Datum
			if agg.ColIdx >= 0 {
				val = row.Datums[agg.ColIdx]
			}
			accums[i].accumulate(val)
		}
	}

	// Build ordered output.
	for _, gr := range groupRows {
		var kp []string
		for _, d := range gr.Datums {
			kp = append(kp, serializeDatum(d))
		}
		accums := groupMap[strings.Join(kp, "\x00")]
		var aggRowDatums []engine.Datum
		for _, acc := range accums {
			aggRowDatums = append(aggRowDatums, acc.result())
		}
		n.aggResults = append(n.aggResults, engine.Row{Datums: aggRowDatums})
	}
	n.groups = groupRows
	n.opened = true
	return nil
}

func (n *HashAggNode) Next(ctx context.Context) (engine.Row, error) {
	if !n.opened {
		return engine.Row{}, io.EOF
	}
	// Global aggregate (no GROUP BY) on empty input → emit 1 row.
	if len(n.groupKeys) == 0 && len(n.groups) == 0 {
		n.outputIdx = 1
		var rowDatums []engine.Datum
		for _, agg := range n.aggs {
			acc := aggAccumulator{op: agg.Op}
			rowDatums = append(rowDatums, acc.result())
		}
		return engine.Row{Datums: rowDatums, RowID: 0}, nil
	}
	if n.outputIdx >= len(n.groups) {
		return engine.Row{}, io.EOF
	}
	gr := n.groups[n.outputIdx]
	ar := n.aggResults[n.outputIdx]
	n.outputIdx++
	out := engine.Row{
		Datums: make([]engine.Datum, 0, len(gr.Datums)+len(ar.Datums)),
		RowID:  0,
	}
	out.Datums = append(out.Datums, gr.Datums...)
	out.Datums = append(out.Datums, ar.Datums...)
	return out, nil
}

func (n *HashAggNode) Close() error {
	n.groups = nil
	n.aggResults = nil
	n.opened = false
	return n.child.Close()
}

func (n *HashAggNode) Schema() engine.Schema { return n.outSchema.DeepCopy() }

// serializeDatum converts a datum to a collision-free string for group key hashing.
func serializeDatum(d engine.Datum) string {
	if d.Value == nil {
		return "N"
	}
	switch d.Type {
	case engine.TypeInt64:
		return fmt.Sprintf("I%d", d.Value.(int64))
	case engine.TypeUint64:
		return fmt.Sprintf("U%d", d.Value.(uint64))
	case engine.TypeFloat64:
		return fmt.Sprintf("F%f", d.Value.(float64))
	case engine.TypeBool:
		if d.Value.(bool) {
			return "BT"
		}
		return "BF"
	case engine.TypeString:
		return "S" + d.Value.(string)
	case engine.TypeBytes:
		b := d.Value.([]byte)
		return fmt.Sprintf("B%d:%s", len(b), string(b))
	}
	return "?"
}

// resolveAggregate parses a Vitess expression into an AggRequest.
func resolveAggregate(ae *vitess.AliasedExpr, schema engine.Schema) (AggRequest, error) {
	// COUNT(*) is represented as *vitess.CountStar in Vitess v0.24+.
	if _, isCS := ae.Expr.(*vitess.CountStar); isCS {
		return AggRequest{Op: AggCount, ColIdx: -1}, nil
	}
	fc, ok := ae.Expr.(*vitess.FuncExpr)
	if !ok {
		return AggRequest{}, fmt.Errorf("planner: %w: expected aggregate function", ErrUnsupportedFeature)
	}
	name := strings.ToUpper(fc.Name.String())
	switch name {
	case "COUNT":
		if len(fc.Exprs) == 0 {
			return AggRequest{Op: AggCount, ColIdx: -1}, nil
		}
		return aggColRefExpr(fc.Exprs[0], schema, AggCount)
	case "SUM":
		return aggColRefExpr(fc.Exprs[0], schema, AggSum)
	case "MIN":
		return aggColRefExpr(fc.Exprs[0], schema, AggMin)
	case "MAX":
		return aggColRefExpr(fc.Exprs[0], schema, AggMax)
	}
	return AggRequest{}, fmt.Errorf("planner: %w: unsupported aggregate %q", ErrUnsupportedFeature, name)
}

func aggColRefExpr(expr vitess.Expr, schema engine.Schema, op AggOp) (AggRequest, error) {
	cn, ok := expr.(*vitess.ColName)
	if !ok {
		return AggRequest{}, fmt.Errorf("planner: %w: aggregate requires column reference", ErrUnsupportedFeature)
	}
	idx := colIndex(cn.Name.String(), schema)
	if idx < 0 {
		return AggRequest{}, fmt.Errorf("planner: column %q not found", cn.Name.String())
	}
	return AggRequest{Op: op, ColIdx: idx}, nil
}
