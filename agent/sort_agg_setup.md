# Sorting and Aggregation Setup (Approved)

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
6. **Group Key Collision Guard:** When serializing group keys for the hash aggregation map, the serialization format must be collision-free (e.g., using length-prefixed formatting for variable-length types like strings and bytes, or guaranteeing that the delimiter does not collide with the serialized output) to prevent distinct grouping sets from merging.

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
			d1, d2 := r1.Datums[key.ColIdx], r2.Datums[key.ColIdx]
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
		return engine.Row{}, io.EOF
	}
	if n.rowIdx >= len(n.rows) {
		return engine.Row{}, io.EOF
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

type aggAccumulator struct {
	op     AggOp
	count  int64
	sum    engine.Datum // Accumulates SUM (float64 or int64)
	min    engine.Datum // Accumulates MIN
	max    engine.Datum // Accumulates MAX
	hasVal bool         // True if at least one non-NULL value has been accumulated
}

func (acc *aggAccumulator) accumulate(val engine.Datum) {
	if acc.op == AggCount {
		// COUNT(*) has ColIdx == -1, val.Value is nil, but we still count it
		if val.Value != nil || val.Type == 0 { // COUNT(*) or COUNT(col) where col is not NULL
			acc.count++
		}
		return
	}

	// SUM, MIN, MAX ignore NULL values
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

type HashAggNode struct {
	child      Operator
	groupKeys  []int        // Indices of GROUP BY columns
	aggs       []AggRequest // List of aggregations to compute
	outSchema  engine.Schema

	// Group-by state mapping
	groups     []engine.Row // Unique groups output list
	aggResults []engine.Row // Aggregate values matched index-for-index with groups
	outputIdx  int
	opened     bool
}

func NewHashAggNode(child Operator, groupKeys []int, aggs []AggRequest) *HashAggNode {
	// ... (Schema derivation logic: group keys followed by aggregates)
	return &HashAggNode{
		child:     child,
		groupKeys: groupKeys,
		aggs:      aggs,
	}
}

func (n *HashAggNode) Open(ctx context.Context) error {
	if err := n.child.Open(ctx); err != nil {
		return err
	}

	n.groups = nil
	n.aggResults = nil
	n.outputIdx = 0

	// Serialized group key string -> list of accumulators
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

		// Extract group key
		var keyParts []string
		var groupRow engine.Row
		groupRow.Datums = make([]engine.Datum, len(n.groupKeys))
		for idx, gKey := range n.groupKeys {
			d := row.Datums[gKey]
			groupRow.Datums[idx] = d
			keyParts = append(keyParts, serializeDatum(d))
		}
		groupKeyStr := joinKeys(keyParts)

		accums, exists := groupMap[groupKeyStr]
		if !exists {
			accums = make([]aggAccumulator, len(n.aggs))
			for i, agg := range n.aggs {
				accums[i] = aggAccumulator{op: agg.Op}
			}
			groupMap[groupKeyStr] = accums
			groupRows = append(groupRows, groupRow)
		}

		// Accumulate row values
		for i, agg := range n.aggs {
			var val engine.Datum
			if agg.ColIdx >= 0 {
				val = row.Datums[agg.ColIdx]
			}
			accums[i].accumulate(val)
		}
	}

	// Build output lists
	n.groups = groupRows
	n.aggResults = make([]engine.Row, len(groupRows))
	for i, gr := range groupRows {
		var keyParts []string
		for _, d := range gr.Datums {
			keyParts = append(keyParts, serializeDatum(d))
		}
		accums := groupMap[joinKeys(keyParts)]

		var aggRow engine.Row
		aggRow.Datums = make([]engine.Datum, len(n.aggs))
		for j, acc := range accums {
			aggRow.Datums[j] = acc.result()
		}
		n.aggResults[i] = aggRow
	}

	n.opened = true
	return nil
}

func (n *HashAggNode) Next(ctx context.Context) (engine.Row, error) {
	if !n.opened {
		return engine.Row{}, io.EOF
	}
	if n.outputIdx >= len(n.groups) {
		// Aggregate without GROUP BY on empty child returns 1 row with default aggregates
		if len(n.groupKeys) == 0 && len(n.groups) == 0 && n.outputIdx == 0 {
			n.outputIdx++
			var aggRow engine.Row
			aggRow.Datums = make([]engine.Datum, len(n.aggs))
			for i, agg := range n.aggs {
				acc := aggAccumulator{op: agg.Op}
				aggRow.Datums[i] = acc.result()
			}
			return aggRow, nil
		}
		return engine.Row{}, io.EOF
	}

	gr := n.groups[n.outputIdx]
	ar := n.aggResults[n.outputIdx]
	n.outputIdx++

	// Combine group keys and aggregations
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

func (n *HashAggNode) Schema() engine.Schema {
	return n.outSchema.DeepCopy()
}
```

### 3. SQL Semantics & Edge Cases

To ensure correct and standard SQL evaluation, the following rules apply:

1. **NULL Handling in Sorting:** 
   - `NULL` values are treated as the lowest possible value.
   - For ascending order (`ASC`), `NULL` values appear first.
   - For descending order (`DESC`), `NULL` values appear last.
   - The type comparison helper `datumLess` must order `NULL` before any non-NULL value.

2. **NULL Handling in Aggregations:**
   - `COUNT(column)` must ignore `NULL` values (i.e. do not increment counts when the column value is `NULL`).
   - `COUNT(*)` counts all rows including those with `NULL` columns.
   - `SUM`, `MIN`, and `MAX` must ignore `NULL` values. If all values in a given group are `NULL` (or if the input relation is empty), `SUM`, `MIN`, and `MAX` must return `NULL` (represented as `engine.Datum` with value `nil` and type `TypeNull` / appropriate data type), not `0`.

3. **Type Promotion / Overflow in SUM:**
   - For the basic setup tier, `SUM` on `INT64` preserves type but wraps on integer overflow. For float fields, it accumulates as `FLOAT64`.
   - In the Enterprise tier, `SUM` accumulates integers in a wider range or returns an overflow error on exceeding range limits.

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
