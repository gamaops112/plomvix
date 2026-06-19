# Sorting and Aggregation Setup

| Field | Value |
| :--- | :--- |
| **Source** | `agent/sort_agg_setup.md` |
| **Package(s)** | `internal/engine/sql/planner` |
| **Purpose** | Implement basic sorting (`ORDER BY`), streaming aggregates (`COUNT`, `SUM`, `MIN`, `MAX`), grouping (`GROUP BY`), and pagination (`LIMIT`). |
| **Dependencies** | Planner / Volcano executor setup plans. |

## Honest Contracts & Known Trade-offs

1. **In-Memory Sort memory risk:** The sorting operator (`SortNode`) buffers the entire child operator's row set in memory before sorting it. This has O(N) memory complexity and risks OOM if the row count is extremely high. Disk-spill sorting is deferred to the Enterprise tier.
2. **In-Memory Hash Aggregation constraint:** `HashAggNode` buffers all unique grouping keys and aggregate states in memory using an internal map. For high-cardinality grouping keys, memory usage is O(K) where K is the number of distinct groups.
3. **No Index-Based Sort optimization in Basic Tier:** Even if the underlying table heap or keyspace is sorted, `ORDER BY` always translates to an explicit `SortNode` in the Volcano operator tree. Optimization to bypass sorting via index scans is deferred.
4. **Limited Expressions in Grouping/Ordering:** Grouping and ordering criteria are strictly limited to direct column names (e.g. `GROUP BY age`, `ORDER BY name ASC`). Ordering/grouping by expressions, aliases, or function calls (e.g. `ORDER BY age + 1`) returns `ErrUnsupportedFeature`.
5. **LIMIT and OFFSET bounds:** `LimitNode` handles pagination. If `OFFSET` is specified, the operator drains that many rows from its child before yielding results. Large offsets can consume substantial processing time, as all skipped rows must still be scanned and decoded.

---

## Deliverables

| File | Purpose |
| :--- | :--- |
| `internal/engine/sql/planner/sort.go` | Implement `SortNode` operator. |
| `internal/engine/sql/planner/agg.go` | Implement `HashAggNode` and aggregation state evaluation helpers for `COUNT`, `SUM`, `MIN`, and `MAX`. |
| `internal/engine/sql/planner/limit.go` | Implement `LimitNode` operator for `LIMIT` and `OFFSET`. |
| `internal/engine/sql/planner/translate.go` | Integrate `ORDER BY`, `GROUP BY`, and `LIMIT` parsing/translation into `Translate()`. |
| `internal/engine/sql/planner/sort_agg_test.go` | Unit tests for in-memory sorting (ASC/DESC), grouped hash aggregation, global aggregation (no group by), and limit/offset bounds. |

---

## Key API & Concepts

### 1. `SortNode` Operator (`internal/engine/sql/planner/sort.go`)

`SortNode` drains its child operator, sorts the row slice in memory using a custom sort comparator, and yields rows sequentially.

```go
package planner

import (
	"context"
	"io"
	"sort"

	"github.com/plomvix/plomvix/internal/engine"
)

type SortKey struct {
	ColIdx int
	Desc   bool // True for DESC, false for ASC
}

type SortNode struct {
	child     Operator
	keys      []SortKey
	outSchema engine.Schema

	// Buffer for in-memory sort
	rows    []engine.Row
	rowIdx  int
	opened  bool
}

func NewSortNode(child Operator, keys []SortKey) *SortNode {
	return &SortNode{
		child:     child,
		keys:      keys,
		outSchema: child.Schema(),
	}
}

func (n *SortNode) Open(ctx context.Context) error {
	if err := n.child.Open(ctx); err != nil {
		return err
	}

	// Drain all rows from the child operator
	var rows []engine.Row
	for {
		r, err := n.child.Next(ctx)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		rows = append(rows, r)
	}

	// Sort the buffered slice
	sort.SliceStable(rows, func(i, j int) bool {
		r1, r2 := rows[i], rows[j]
		for _, key := range n.keys {
			d1, d2 := r1[key.ColIdx], r2[key.ColIdx]
			if datumEqual(d1, d2) {
				continue
			}
			less := datumLess(d1, d2)
			if key.Desc {
				return !less
			}
			return less
		}
		return false
	})

	n.rows = rows
	n.rowIdx = 0
	n.opened = true
	return nil
}

func (n *SortNode) Next(ctx context.Context) (engine.Row, error) {
	if !n.opened {
		return nil, io.EOF
	}
	if n.rowIdx >= len(n.rows) {
		return nil, io.EOF
	}
	r := n.rows[n.rowIdx]
	n.rowIdx++
	return r, nil
}

func (n *SortNode) Close() error {
	n.rows = nil
	n.opened = false
	return n.child.Close()
}

func (n *SortNode) Schema() engine.Schema {
	return n.outSchema.DeepCopy()
}
```

### 2. `HashAggNode` Operator (`internal/engine/sql/planner/agg.go`)

`HashAggNode` computes group buckets. It maps group keys to lists of aggregate accumulators.

```go
type AggOp uint8
const (
	AggCount AggOp = iota
	AggSum
	AggMin
	AggMax
)

type AggRequest struct {
	Op     AggOp
	ColIdx int // Index of column to aggregate, -1 for COUNT(*)
}

type HashAggNode struct {
	child      Operator
	groupKeys  []int        // Indices of GROUP BY columns
	aggs       []AggRequest // List of aggregations to compute
	outSchema  engine.Schema

	// Group-by state mapping: serialized grouping key values -> accumulator states
	groups     []engine.Row // Unique groups output list
	aggResults []engine.Row // Aggregate values matched index-for-index with groups
	outputIdx  int
	opened     bool
}
```

---

## Tasks

1. **Implement `SortNode`:** Code `SortNode` with multi-key sorting, handling ascending and descending order, and standard type comparison wrappers.
2. **Implement `HashAggNode`:** Implement group bucket generation. Support aggregate evaluation states:
   - `COUNT`: track row increments.
   - `SUM`: sum numeric type datum values.
   - `MIN`/`MAX`: track comparisons.
   - Aggregate without GROUP BY: evaluate a single global bucket.
3. **Implement `LimitNode`:** Implement pagination filtering: skip `OFFSET` rows, stream up to `LIMIT` rows, and abort early once count is met.
4. **Refactor Planner Translation:** Extend `Translate` in `translate.go` to handle:
   - `ORDER BY` -> parse `vitess.Order` slice, translate columns to indices, and append `SortNode`.
   - `GROUP BY` / aggregates in SelectExprs -> parse grouping columns, separate aggregated and grouped projection columns, and build `HashAggNode`.
   - `LIMIT` -> parse `vitess.Limit` (Limit and Offset expressions) and append `LimitNode`.
5. **Basic Aggregation / Sort Tests:** Unit tests verifying:
   - Single and composite ordering (ASC/DESC).
   - `GROUP BY` matching count and averages.
   - Global select aggregator (e.g. `SELECT COUNT(*), SUM(a) FROM t` with zero groups returning exactly one row).
   - Offset and limit boundary checks.

---

## Completion Criteria

- [ ] `SortNode` yields rows ordered strictly by designated sort keys.
- [ ] `HashAggNode` aggregates COUNT, SUM, MIN, and MAX grouped by target columns.
- [ ] Queries without GROUP BY containing aggregate functions return exactly one row.
- [ ] `LimitNode` respects both limit boundaries and offsets.
- [ ] Vitess statements with ORDER BY, GROUP BY, and LIMIT parse and execute.
