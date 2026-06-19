package planner

import (
	"fmt"

	"github.com/plomvix/plomvix/internal/engine"
	vitess "vitess.io/vitess/go/vt/sqlparser"
)

// BoundExpr evaluates an expression against a row.
type BoundExpr interface {
	Eval(row engine.Row) (engine.Datum, error)
}

type colRef struct {
	idx int
	typ engine.Type
}

func (c *colRef) Eval(row engine.Row) (engine.Datum, error) {
	if c.idx >= len(row.Datums) {
		return engine.Datum{}, fmt.Errorf("planner: column index %d out of range", c.idx)
	}
	return row.Datums[c.idx], nil
}

type literalExpr struct {
	d engine.Datum
}

func (l *literalExpr) Eval(_ engine.Row) (engine.Datum, error) {
	return l.d, nil
}

type cmpExpr struct {
	left, right BoundExpr
	op          string
}

func (c *cmpExpr) Eval(row engine.Row) (engine.Datum, error) {
	l, err := c.left.Eval(row)
	if err != nil {
		return engine.Datum{}, err
	}
	r, err := c.right.Eval(row)
	if err != nil {
		return engine.Datum{}, err
	}
	eq := datumEqual(l, r)
	switch c.op {
	case "=":
		return engine.Datum{Type: engine.TypeBool, Value: eq}, nil
	case "<":
		return engine.Datum{Type: engine.TypeBool, Value: datumLess(l, r)}, nil
	case ">":
		return engine.Datum{Type: engine.TypeBool, Value: datumLess(r, l)}, nil
	default:
		return engine.Datum{}, fmt.Errorf("planner: unsupported op %q", c.op)
	}
}

type boolExpr struct {
	left, right BoundExpr
	op          string
}

func (b *boolExpr) Eval(row engine.Row) (engine.Datum, error) {
	l, err := b.left.Eval(row)
	if err != nil {
		return engine.Datum{}, err
	}
	lb, ok := l.Value.(bool)
	if !ok {
		return engine.Datum{}, fmt.Errorf("planner: AND/OR requires bool")
	}
	if b.op == "and" && !lb {
		return engine.Datum{Type: engine.TypeBool, Value: false}, nil
	}
	if b.op == "or" && lb {
		return engine.Datum{Type: engine.TypeBool, Value: true}, nil
	}
	r, err := b.right.Eval(row)
	if err != nil {
		return engine.Datum{}, err
	}
	rb, ok := r.Value.(bool)
	if !ok {
		return engine.Datum{}, fmt.Errorf("planner: AND/OR requires bool")
	}
	if b.op == "and" {
		return engine.Datum{Type: engine.TypeBool, Value: lb && rb}, nil
	}
	return engine.Datum{Type: engine.TypeBool, Value: lb || rb}, nil
}

// ProjectionExpr pairs a bound expression with its output column.
type ProjectionExpr struct {
	Expr BoundExpr
	Col  engine.Column
}

// BindWhere walks a Vitess expression and returns a BoundExpr.
func BindWhere(expr vitess.Expr, schema engine.Schema) (BoundExpr, error) {
	if expr == nil {
		return nil, nil
	}
	switch e := expr.(type) {
	case *vitess.ComparisonExpr:
		// IN list: WHERE col IN (1, 2, 3)
		if e.Operator == vitess.InOp {
			return bindInPredicate(e, schema)
		}
		left, err := BindWhere(e.Left, schema)
		if err != nil {
			return nil, err
		}
		right, err := BindWhere(e.Right, schema)
		if err != nil {
			return nil, err
		}
		op := comparisonOpString(e.Operator)
		return &cmpExpr{left: left, right: right, op: op}, nil
	case *vitess.BetweenExpr:
		// BETWEEN: WHERE col BETWEEN lo AND hi
		return bindBetweenPredicate(e, schema)
	case *vitess.AndExpr:
		left, err := BindWhere(e.Left, schema)
		if err != nil {
			return nil, err
		}
		right, err := BindWhere(e.Right, schema)
		if err != nil {
			return nil, err
		}
		return &boolExpr{left: left, right: right, op: "and"}, nil
	case *vitess.OrExpr:
		left, err := BindWhere(e.Left, schema)
		if err != nil {
			return nil, err
		}
		right, err := BindWhere(e.Right, schema)
		if err != nil {
			return nil, err
		}
		return &boolExpr{left: left, right: right, op: "or"}, nil
	case *vitess.ColName:
		idx := colIndex(e.Name.String(), schema)
		if idx < 0 {
			return nil, fmt.Errorf("planner: column %q not found", e.Name.String())
		}
		return &colRef{idx: idx, typ: schema.Columns[idx].Type}, nil
	case *vitess.Literal:
		return bindLiteral(e)
	default:
		return nil, fmt.Errorf("%w: expression type %T", ErrUnsupportedFeature, expr)
	}
}

func comparisonOpString(op vitess.ComparisonExprOperator) string {
	switch op {
	case vitess.EqualOp:
		return "="
	case vitess.LessThanOp:
		return "<"
	case vitess.GreaterThanOp:
		return ">"
	default:
		return "?"
	}
}

// BindProjection maps select expressions to output columns.
func BindProjection(exprs *vitess.SelectExprs, schema engine.Schema) ([]ProjectionExpr, engine.Schema, error) {
	var projs []ProjectionExpr
	var outCols []engine.Column

	for _, expr := range exprs.Exprs {
		ae, ok := expr.(*vitess.AliasedExpr)
		if !ok {
			return nil, engine.Schema{}, fmt.Errorf("%w: non-aliased select expr", ErrUnsupportedFeature)
		}
		bound, err := BindWhere(ae.Expr, schema)
		if err != nil {
			return nil, engine.Schema{}, err
		}
		colName := ae.As.String()
		if colName == "" {
			if cn, ok := ae.Expr.(*vitess.ColName); ok {
				colName = cn.Name.String()
			} else {
				colName = "?"
			}
		}
		colType := engine.TypeString
		if cr, ok := bound.(*colRef); ok {
			colType = cr.typ
		}
		col := engine.Column{Name: colName, Type: colType}
		outCols = append(outCols, col)
		projs = append(projs, ProjectionExpr{Expr: bound, Col: col})
	}
	return projs, engine.Schema{Columns: outCols}, nil
}

func bindLiteral(lit *vitess.Literal) (BoundExpr, error) {
	switch lit.Type {
	case vitess.IntVal, vitess.StrVal, vitess.FloatVal:
		return &literalExpr{d: literalToDatum(lit)}, nil
	default:
		return nil, fmt.Errorf("%w: unsupported literal type", ErrUnsupportedFeature)
	}
}

func literalToDatum(lit *vitess.Literal) engine.Datum {
	switch lit.Type {
	case vitess.IntVal:
		return engine.Datum{Type: engine.TypeInt64, Value: parseIntOrZero(lit.Val)}
	case vitess.FloatVal:
		return engine.Datum{Type: engine.TypeFloat64, Value: parseFloatOrZero(lit.Val)}
	case vitess.StrVal:
		return engine.Datum{Type: engine.TypeString, Value: lit.Val}
	default:
		return engine.Datum{}
	}
}

// InPredicate evaluates col IN (v1, v2, ...).
type InPredicate struct {
	ColIdx int
	Values []engine.Datum
}

func (p *InPredicate) Eval(row engine.Row) (engine.Datum, error) {
	if p.ColIdx >= len(row.Datums) {
		return engine.Datum{}, fmt.Errorf("planner: IN column index out of range")
	}
	val := row.Datums[p.ColIdx]
	for _, v := range p.Values {
		if datumEqual(val, v) {
			return engine.Datum{Type: engine.TypeBool, Value: true}, nil
		}
	}
	return engine.Datum{Type: engine.TypeBool, Value: false}, nil
}

// BetweenPredicate evaluates col BETWEEN lo AND hi.
type BetweenPredicate struct {
	ColIdx int
	Lo, Hi engine.Datum
}

func (p *BetweenPredicate) Eval(row engine.Row) (engine.Datum, error) {
	if p.ColIdx >= len(row.Datums) {
		return engine.Datum{}, fmt.Errorf("planner: BETWEEN column index out of range")
	}
	val := row.Datums[p.ColIdx]
	geLo := datumEqual(val, p.Lo) || datumLess(p.Lo, val)
	leHi := datumEqual(val, p.Hi) || datumLess(val, p.Hi)
	return engine.Datum{Type: engine.TypeBool, Value: geLo && leHi}, nil
}

func bindInPredicate(e *vitess.ComparisonExpr, schema engine.Schema) (BoundExpr, error) {
	colName, ok := e.Left.(*vitess.ColName)
	if !ok {
		return nil, fmt.Errorf("%w: IN requires column reference", ErrUnsupportedFeature)
	}
	idx := colIndex(colName.Name.String(), schema)
	if idx < 0 {
		return nil, fmt.Errorf("planner: IN column %q not found", colName.Name.String())
	}
	tuple, ok := e.Right.(vitess.ValTuple)
	if !ok {
		return nil, fmt.Errorf("%w: IN requires literal tuple", ErrUnsupportedFeature)
	}
	var values []engine.Datum
	for _, expr := range tuple {
		lit, ok := expr.(*vitess.Literal)
		if !ok {
			return nil, fmt.Errorf("%w: IN values must be literals", ErrUnsupportedFeature)
		}
		values = append(values, literalToDatum(lit))
	}
	return &InPredicate{ColIdx: idx, Values: values}, nil
}

func bindBetweenPredicate(e *vitess.BetweenExpr, schema engine.Schema) (BoundExpr, error) {
	colName, ok := e.Left.(*vitess.ColName)
	if !ok {
		return nil, fmt.Errorf("%w: BETWEEN requires column reference", ErrUnsupportedFeature)
	}
	idx := colIndex(colName.Name.String(), schema)
	if idx < 0 {
		return nil, fmt.Errorf("planner: BETWEEN column %q not found", colName.Name.String())
	}
	loLit, ok := e.From.(*vitess.Literal)
	if !ok {
		return nil, fmt.Errorf("%w: BETWEEN requires literal bounds", ErrUnsupportedFeature)
	}
	hiLit, ok := e.To.(*vitess.Literal)
	if !ok {
		return nil, fmt.Errorf("%w: BETWEEN requires literal bounds", ErrUnsupportedFeature)
	}
	return &BetweenPredicate{ColIdx: idx, Lo: literalToDatum(loLit), Hi: literalToDatum(hiLit)}, nil
}

func parseIntOrZero(s string) int64 {
	var i int64
	fmt.Sscanf(s, "%d", &i)
	return i
}

func parseFloatOrZero(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

func colIndex(name string, schema engine.Schema) int {
	for i, col := range schema.Columns {
		if col.Name == name {
			return i
		}
	}
	return -1
}

func datumEqual(a, b engine.Datum) bool {
	if a.Type != b.Type {
		return false
	}
	switch a.Type {
	case engine.TypeInt64:
		return a.Value.(int64) == b.Value.(int64)
	case engine.TypeUint64:
		return a.Value.(uint64) == b.Value.(uint64)
	case engine.TypeFloat64:
		return a.Value.(float64) == b.Value.(float64)
	case engine.TypeBool:
		return a.Value.(bool) == b.Value.(bool)
	case engine.TypeString:
		return a.Value.(string) == b.Value.(string)
	case engine.TypeBytes:
		ab, bb := a.Value.([]byte), b.Value.([]byte)
		if len(ab) != len(bb) {
			return false
		}
		for i := range ab {
			if ab[i] != bb[i] {
				return false
			}
		}
		return true
	}
	return false
}

func datumLess(a, b engine.Datum) bool {
	switch a.Type {
	case engine.TypeInt64:
		return a.Value.(int64) < b.Value.(int64)
	case engine.TypeUint64:
		return a.Value.(uint64) < b.Value.(uint64)
	case engine.TypeFloat64:
		return a.Value.(float64) < b.Value.(float64)
	case engine.TypeString:
		return a.Value.(string) < b.Value.(string)
	}
	return false
}
