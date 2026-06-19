# Joins and Multi-Table Execution Setup

| Field | Value |
| :--- | :--- |
| **Source** | `agent/joins_setup.md` |
| **Package(s)** | `internal/engine/sql/planner` |
| **Purpose** | Implement basic multi-table query support with Volcano-style Nested Loop Join execution, qualified column name resolution, and joint schema binding. |
| **Dependencies** | Planner / Volcano executor setup plans. |

## Honest Contracts & Known Trade-offs

1. **Inner Joins only on Basic Tier:** Only explicit `INNER JOIN` (or simple cross-product joins via implicit table list) is supported. `LEFT JOIN`, `RIGHT JOIN`, and sub-queries inside joins are rejected with `ErrUnsupportedFeature`.
2. **Nested Loop Join execution is O(N * M):** The basic tier uses a naive nested loop join (`NestedLoopJoinNode`). The outer table row is cached, and the inner table is re-scanned completely for every outer row. While memory usage is O(1) as it streams, this is inefficient for large tables. This is an intentional correctness-over-performance trade-off for the basic tier.
3. **Syntactic Join Ordering:** No cost-based optimization or join re-ordering is performed. Tables are joined strictly in the order they are declared in the SQL `FROM` clause.
4. **Column Ambiguity Guard:** In multi-table queries, unqualified columns (e.g. `id`) must be unique across all joined table schemas. If a column name is present in multiple tables and is unqualified in the query, the binder returns `ErrAmbiguousColumn` immediately.
5. **No Plan Caching for Joins in Basic Tier:** Plans containing joins bypass plan caching entirely to avoid complex parameterized schema mapping in the basic tier.

6. **All planner operators support Close/Open cycle:** All planner operators (e.g., `SeqScanNode`) must support `Close()` followed by `Open(ctx)` to allow correct nested-loop re-scanning.
7. **Setup tier rejects LEFT JOIN:** Setup tier rejects `LEFT JOIN`, `RIGHT JOIN`, and `FULL OUTER JOIN`. Enterprise tier updates those expectations to accept `LEFT JOIN` and only reject `RIGHT`/`FULL`.

---

## Deliverables

| File | Purpose |
| :--- | :--- |
| `internal/engine/sql/planner/join.go` | Define `NestedLoopJoinNode` operator, `PlannerSchema`, and `SchemaField` helper types. |
| `internal/engine/sql/planner/binder.go` | Refactor `BindWhere` and `BindProjection` to accept `PlannerSchema` and handle qualified column resolution. |
| `internal/engine/sql/planner/translate.go` | Refactor `Translate` to recursively parse `*vitess.JoinTableExpr` and build join operator trees. |
| `internal/engine/sql/planner/join_test.go` | Unit tests validating inner joins, qualified/unqualified name binding, ambiguous column rejection, and scan re-open logic. |

---

## Key API & Concepts

### 1. `PlannerSchema` and Column Resolution (`internal/engine/sql/planner/join.go`)

To resolve columns in multi-table queries, we define a wrapper schema tracking table qualifiers.

```go
package planner

import (
	"errors"
	"strings"

	"github.com/plomvix/plomvix/internal/engine"
)

var (
	ErrAmbiguousColumn = errors.New("planner: column reference is ambiguous")
	ErrColumnNotFound   = errors.New("planner: column not found")
)

type SchemaField struct {
	engine.Column
	TableAlias string // Table name or alias (e.g. "t1")
}

type PlannerSchema struct {
	Fields []SchemaField
}

// ResolveColumn finds the index of a column.
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

// ToEngineSchema exports fields to engine.Schema.
func (ps PlannerSchema) ToEngineSchema() engine.Schema {
	cols := make([]engine.Column, len(ps.Fields))
	for i, f := range ps.Fields {
		cols[i] = f.Column
	}
	return engine.Schema{Columns: cols}
}

// PlannerSchemaFromEngineSchema wraps engine.Schema for backward compatibility.
func PlannerSchemaFromEngineSchema(schema engine.Schema, tableAlias string) PlannerSchema {
	fields := make([]SchemaField, len(schema.Columns))
	for i, col := range schema.Columns {
		fields[i] = SchemaField{Column: col, TableAlias: tableAlias}
	}
	return PlannerSchema{Fields: fields}
}

// GetPlannerSchema retrieves the PlannerSchema mapping for any operator,
// falling back to a default un-aliased mapping if not explicitly tracked.
func GetPlannerSchema(op Operator) PlannerSchema {
	if po, ok := op.(interface{ PlannerSchema() PlannerSchema }); ok {
		return po.PlannerSchema()
	}
	return PlannerSchemaFromEngineSchema(op.Schema(), "")
}
```

### 2. `NestedLoopJoinNode` Operator (`internal/engine/sql/planner/join.go`)

`NestedLoopJoinNode` couples left and right child operators under the Volcano framework.

> [!NOTE]
> Enterprise plan intentionally refactors this constructor to add `isLeftJoin` and `logger`; setup keeps the 3-arg constructor only for the basic tier.

```go
type NestedLoopJoinNode struct {
	left          Operator
	right         Operator
	cond          BoundExpr
	plannerSchema PlannerSchema
	outSchema     engine.Schema

	leftRow   engine.Row // Cached current row from the outer table
}

func NewNestedLoopJoinNode(left, right Operator, cond BoundExpr) *NestedLoopJoinNode {
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
		plannerSchema: plannerSchema,
		outSchema:     plannerSchema.ToEngineSchema(),
	}
}

func (n *NestedLoopJoinNode) PlannerSchema() PlannerSchema {
	return n.plannerSchema
}

func (n *NestedLoopJoinNode) Open(ctx context.Context) error {
	if err := n.left.Open(ctx); err != nil {
		return err
	}
	if err := n.right.Open(ctx); err != nil {
		_ = n.left.Close()
		return err
	}

	// Prime the outer loop safely
	lr, err := n.left.Next(ctx)
	if err != nil {
		if errors.Is(err, io.EOF) {
			n.leftRow.Datums = nil
			return nil
		}
		_ = n.left.Close()
		_ = n.right.Close()
		return err
	}
	n.leftRow = lr
	return nil
}

func (n *NestedLoopJoinNode) Next(ctx context.Context) (engine.Row, error) {
	for {
		if n.leftRow.Datums == nil {
			return engine.Row{}, io.EOF
		}

		rr, err := n.right.Next(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				// Re-open the inner child relation
				if err := n.right.Close(); err != nil {
					return engine.Row{}, err
				}
				if err := n.right.Open(ctx); err != nil {
					return engine.Row{}, err
				}
				
				// Advance left child row
				lr, err := n.left.Next(ctx)
				if err != nil {
					n.leftRow.Datums = nil // Ensure we return io.EOF on subsequent calls
					if errors.Is(err, io.EOF) {
						return engine.Row{}, io.EOF
					}
					return engine.Row{}, err
				}
				n.leftRow = lr
				continue
			}
			return engine.Row{}, err
		}

		// Concatenate matching tuple (joined rows have RowID == 0)
		joinedRow := engine.Row{
			Datums: make([]engine.Datum, 0, len(n.leftRow.Datums)+len(rr.Datums)),
			RowID:  0, // Derived rows must not have a valid physical RowID
		}
		joinedRow.Datums = append(joinedRow.Datums, n.leftRow.Datums...)
		joinedRow.Datums = append(joinedRow.Datums, rr.Datums...)

		if n.cond != nil {
			d, err := n.cond.Eval(joinedRow)
			if err != nil {
				return engine.Row{}, err
			}
			if b, ok := d.Value.(bool); ok && b {
				return joinedRow, nil
			}
			continue
		}

		return joinedRow, nil
	}
}

func (n *NestedLoopJoinNode) Close() error {
	n.leftRow.Datums = nil
	errL := n.left.Close()
	errR := n.right.Close()
	if errL != nil {
		return errL
	}
	return errR
}

func (n *NestedLoopJoinNode) Schema() engine.Schema {
	return n.outSchema.DeepCopy()
}
```

### 3. Binder Compatibility Interface (`internal/engine/sql/planner/binder.go`)

To prevent breaking existing DML plans (UPDATE, DELETE) and single-table SELECT logic, we preserve the original `BindWhere` and `BindProjection` signatures. They internally delegate to the new qualified-column-aware implementations:

```go
func BindWhere(expr vitess.Expr, schema engine.Schema) (BoundExpr, error) {
	return BindWherePlanner(expr, PlannerSchemaFromEngineSchema(schema, ""))
}

func BindWherePlanner(expr vitess.Expr, ps PlannerSchema) (BoundExpr, error) {
	// New qualified-column implementation using ps.ResolveColumn
}

func BindProjection(exprs *vitess.SelectExprs, schema engine.Schema) ([]ProjectionExpr, engine.Schema, error) {
	proj, ps, err := BindProjectionPlanner(exprs, PlannerSchemaFromEngineSchema(schema, ""))
	if err != nil {
		return nil, engine.Schema{}, err
	}
	return proj, ps.ToEngineSchema(), nil
}

func BindProjectionPlanner(exprs *vitess.SelectExprs, ps PlannerSchema) ([]ProjectionExpr, PlannerSchema, error) {
	// New qualified-column projection implementation returning new PlannerSchema
}
```

---

## Tasks

1. **Create `PlannerSchema` Primitives:** Implement `PlannerSchema`, `SchemaField`, `ResolveColumn`, and validation helpers in `internal/engine/sql/planner/join.go`.
2. **Implement `NestedLoopJoinNode`:** Code the Volcano operator in `internal/engine/sql/planner/join.go`, ensuring correct re-opening on inner relation EOF.
3. **Refactor Binder for Qualified Columns:** Refactor `binder.go` to support qualified column mapping. Preserve the original `BindWhere(expr vitess.Expr, schema engine.Schema) (BoundExpr, error)` and `BindProjection(exprs *vitess.SelectExprs, schema engine.Schema) ([]ProjectionExpr, engine.Schema, error)` signatures (which internally call `PlannerSchemaFromEngineSchema` to avoid breaking single-table code and existing DML plans). Implement overloaded functions `BindWherePlanner(expr vitess.Expr, ps PlannerSchema) (BoundExpr, error)` and `BindProjectionPlanner(exprs *vitess.SelectExprs, ps PlannerSchema) ([]ProjectionExpr, PlannerSchema, error)` to handle `*vitess.ColName` table qualifiers via `ps.ResolveColumn(qualifier, name)`.
4. **Extend Translation to Join Expressions:** Update `Translate` in `internal/engine/sql/planner/translate.go` to parse `*vitess.JoinTableExpr` (INNER JOIN ON). Recursively translate left and right tables into `SeqScanNode`s, bind the join predicate ON expression against the combined `PlannerSchema`, and construct the `NestedLoopJoinNode`.
5. **Add Ambiguity Guards:** Assert that any unqualified select expression or WHERE reference matches exactly one field across the schemas; return `ErrAmbiguousColumn` otherwise.
6. **Inner Join Setup Tests:** Write tests verifying:
   - Join result schema correctness.
   - Successful inner join match output.
   - Column ambiguity guard (returns `ErrAmbiguousColumn` for overlapping column names).
   - Qualified table references override name collisions.

---

## Completion Criteria

- [ ] `PlannerSchema` correctly resolves qualified and unqualified columns.
- [ ] `NestedLoopJoinNode` yields matches matching join criteria.
- [ ] Inner relation iterator re-opens successfully on reaching outer loops.
- [ ] Refactored `Translate` successfully processes `SELECT * FROM t1 JOIN t2 ON t1.id = t2.id`.
- [ ] Plan binder rejects ambiguous column names.
