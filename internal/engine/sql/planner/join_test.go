package planner

import (
	"context"
	"io"
	"testing"

	"github.com/plomvix/plomvix/internal/engine"
)

func TestPlannerSchema_ResolveUnqualified(t *testing.T) {
	ps := PlannerSchema{Fields: []SchemaField{
		{Column: engine.Column{Name: "id", Type: engine.TypeInt64}, TableAlias: "t1"},
		{Column: engine.Column{Name: "name", Type: engine.TypeString}, TableAlias: "t1"},
	}}
	idx, err := ps.ResolveColumn("", "id")
	if err != nil || idx != 0 {
		t.Errorf("got (%d, %v), want (0, nil)", idx, err)
	}
	idx, err = ps.ResolveColumn("", "name")
	if err != nil || idx != 1 {
		t.Errorf("got (%d, %v), want (1, nil)", idx, err)
	}
}

func TestPlannerSchema_ResolveQualified(t *testing.T) {
	ps := PlannerSchema{Fields: []SchemaField{
		{Column: engine.Column{Name: "id", Type: engine.TypeInt64}, TableAlias: "t1"},
		{Column: engine.Column{Name: "id", Type: engine.TypeInt64}, TableAlias: "t2"},
	}}
	idx, err := ps.ResolveColumn("t1", "id")
	if err != nil || idx != 0 {
		t.Errorf("got (%d, %v), want (0, nil)", idx, err)
	}
	idx, err = ps.ResolveColumn("t2", "id")
	if err != nil || idx != 1 {
		t.Errorf("got (%d, %v), want (1, nil)", idx, err)
	}
}

func TestPlannerSchema_AmbiguousColumn(t *testing.T) {
	ps := PlannerSchema{Fields: []SchemaField{
		{Column: engine.Column{Name: "id", Type: engine.TypeInt64}, TableAlias: "t1"},
		{Column: engine.Column{Name: "id", Type: engine.TypeInt64}, TableAlias: "t2"},
	}}
	_, err := ps.ResolveColumn("", "id")
	if err != ErrAmbiguousColumn {
		t.Errorf("got %v, want ErrAmbiguousColumn", err)
	}
}

func TestPlannerSchema_ColumnNotFound(t *testing.T) {
	ps := PlannerSchema{Fields: []SchemaField{
		{Column: engine.Column{Name: "id", Type: engine.TypeInt64}, TableAlias: "t1"},
	}}
	_, err := ps.ResolveColumn("", "missing")
	if err != ErrColumnNotFound {
		t.Errorf("got %v, want ErrColumnNotFound", err)
	}
	_, err = ps.ResolveColumn("t2", "id")
	if err != ErrColumnNotFound {
		t.Errorf("got %v, want ErrColumnNotFound", err)
	}
}

func TestNestedLoopJoinNode_Schema(t *testing.T) {
	left := &fakeSchemaOp{sch: engine.Schema{Columns: []engine.Column{
		{Name: "a", Type: engine.TypeInt64},
	}}, alias: "t1"}
	right := &fakeSchemaOp{sch: engine.Schema{Columns: []engine.Column{
		{Name: "b", Type: engine.TypeString},
	}}, alias: "t2"}

	node := NewNestedLoopJoinNode(left, right, nil, false, nil)
	schema := node.Schema()
	if len(schema.Columns) != 2 {
		t.Errorf("got %d columns, want 2", len(schema.Columns))
	}
}

func TestNestedLoopJoinNode_OpenCloseCycle(t *testing.T) {
	left := &fakeRowsOp{rows: []engine.Row{
		{Datums: []engine.Datum{{Type: engine.TypeInt64, Value: int64(1)}}},
	}}
	right := &fakeRowsOp{rows: []engine.Row{
		{Datums: []engine.Datum{{Type: engine.TypeString, Value: "x"}}},
	}}

	node := NewNestedLoopJoinNode(left, right, nil, false, nil)
	ctx := context.Background()

	if err := node.Open(ctx); err != nil {
		t.Fatal(err)
	}
	row, err := node.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(row.Datums) != 2 {
		t.Fatalf("got %d datums, want 2", len(row.Datums))
	}
	if row.RowID != 0 {
		t.Error("joined rows should have RowID 0")
	}
	_ = node.Close()

	// Re-open should work.
	if err := node.Open(ctx); err != nil {
		t.Fatal(err)
	}
	row2, _ := node.Next(ctx)
	if len(row2.Datums) != 2 {
		t.Error("re-open should yield rows again")
	}
	_ = node.Close()
}

func TestNestedLoopJoinNode_InnerRescan(t *testing.T) {
	// Two left rows, one right row each.
	left := &fakeRowsOp{rows: []engine.Row{
		{Datums: []engine.Datum{{Type: engine.TypeInt64, Value: int64(1)}}},
		{Datums: []engine.Datum{{Type: engine.TypeInt64, Value: int64(2)}}},
	}}
	right := &fakeRowsOp{rows: []engine.Row{
		{Datums: []engine.Datum{{Type: engine.TypeString, Value: "x"}}},
	}}

	node := NewNestedLoopJoinNode(left, right, nil, false, nil)
	ctx := context.Background()
	_ = node.Open(ctx)
	defer node.Close()

	count := 0
	for {
		row, err := node.Next(ctx)
		if err != nil {
			break
		}
		count++
		_ = row
	}
	if count != 2 {
		t.Errorf("got %d rows, want 2 (1x1 per left row)", count)
	}
}

func TestNestedLoopJoinNode_WithCondition(t *testing.T) {
	left := &fakeRowsOp{rows: []engine.Row{
		{Datums: []engine.Datum{{Type: engine.TypeInt64, Value: int64(1)}}},
		{Datums: []engine.Datum{{Type: engine.TypeInt64, Value: int64(2)}}},
	}}
	right := &fakeRowsOp{rows: []engine.Row{
		{Datums: []engine.Datum{{Type: engine.TypeInt64, Value: int64(1)}}},
		{Datums: []engine.Datum{{Type: engine.TypeInt64, Value: int64(3)}}},
	}}

	// Condition: left[0] == right[0]
	cond := &eqJoinCond{leftIdx: 0, rightIdx: 1}
	node := NewNestedLoopJoinNode(left, right, cond, false, nil)
	ctx := context.Background()
	_ = node.Open(ctx)
	defer node.Close()

	count := 0
	for {
		_, err := node.Next(ctx)
		if err != nil {
			break
		}
		count++
	}
	if count != 1 {
		t.Errorf("got %d rows, want 1 (only 1=1 match)", count)
	}
}

// --- Fake operators for testing ---

type fakeSchemaOp struct {
	sch   engine.Schema
	alias string
}

func (f *fakeSchemaOp) Open(ctx context.Context) error               { return nil }
func (f *fakeSchemaOp) Next(ctx context.Context) (engine.Row, error) { return engine.Row{}, nil }
func (f *fakeSchemaOp) Close() error                                 { return nil }
func (f *fakeSchemaOp) Schema() engine.Schema                        { return f.sch }
func (f *fakeSchemaOp) PlannerSchema() PlannerSchema {
	return PlannerSchemaFromEngineSchema(f.sch, f.alias)
}

type fakeRowsOp struct {
	rows []engine.Row
	idx  int
}

func (f *fakeRowsOp) Open(ctx context.Context) error { f.idx = 0; return nil }
func (f *fakeRowsOp) Next(ctx context.Context) (engine.Row, error) {
	if f.idx >= len(f.rows) {
		return engine.Row{}, io.EOF
	}
	r := f.rows[f.idx]
	f.idx++
	return r, nil
}
func (f *fakeRowsOp) Close() error { return nil }
func (f *fakeRowsOp) Schema() engine.Schema {
	return engine.Schema{Columns: []engine.Column{{Name: "c", Type: engine.TypeInt64}}}
}

type eqJoinCond struct {
	leftIdx  int
	rightIdx int
}

func (e *eqJoinCond) Eval(row engine.Row) (engine.Datum, error) {
	if len(row.Datums) <= e.rightIdx {
		return engine.Datum{Type: engine.TypeBool, Value: false}, nil
	}
	lv := row.Datums[e.leftIdx].Value
	rv := row.Datums[e.rightIdx].Value
	return engine.Datum{Type: engine.TypeBool, Value: lv == rv}, nil
}
