# Joins and Multi-Table Execution Enterprise

| Field | Value |
| :--- | :--- |
| **Source** | `agent/joins_enterprise.md` |
| **Package(s)** | `internal/engine/sql/planner` |
| **Purpose** | Production-harden joins with in-memory Hash Join execution for high-performance equijoins, LEFT OUTER JOIN support, greedy cost-based join reordering, and structured performance telemetry. |
| **Dependencies** | Joins and Multi-Table Execution Setup plan, Catalog Setup/Enterprise plans. |

## Honest Contracts & Known Trade-offs

1. **In-Memory Hash Join build relation constraint:** The Hash Join operator (`HashJoinNode`) builds an in-memory hash table of the build relation (the smaller child table). If the build table is extremely large, this can result in high memory consumption. Disk-spill hash join or hybrid hash join is deferred to a future sharding/disk compaction plan.
2. **Equijoins only for Hash Joins:** Hash joins can only be selected if the join condition is an equality predicate (e.g. `t1.id = t2.id`). Non-equality predicates (e.g. `t1.val < t2.val` or OR-joined conditions) fall back to `NestedLoopJoinNode` execution.
3. **Greedy Join Reordering:** The planner optimizes multi-way joins (e.g. `A JOIN B JOIN C`) using a greedy heuristic: it estimates table sizes from catalog metadata and schedules smaller relations to be joined first, while prioritizing joins with predicates over cross products. It does not perform full dynamic programming plan-space search.
4. **Outer Joins limited to LEFT:** `LEFT OUTER JOIN` is supported. `RIGHT OUTER JOIN` and `FULL OUTER JOIN` are parsed but return `ErrUnsupportedFeature` (with the recommendation that callers rewrite RIGHT joins as LEFT joins).
5. **Telemetry overhead is negligible:** Joins emit execution metrics via structured `slog` logs at `INFO` level. This includes join type selected, row counts, build phase latency, probe phase latency, and memory footprint metrics. Telemetry is fire-and-forget and occurs after operator close.

---

## Deliverables

| File | Purpose |
| :--- | :--- |
| `internal/engine/sql/planner/hash_join.go` | Implement `HashJoinNode` operator. |
| `internal/engine/sql/planner/join.go` | Refactor `NestedLoopJoinNode` to support `LEFT OUTER JOIN` matching states and null-padding. |
| `internal/engine/sql/planner/optimizer.go` | Implement cost-based join selector and greedy join-reordering planner. |
| `internal/engine/sql/planner/joins_enterprise_test.go` | Benchmarks comparing hash join vs loop join, unit tests for left outer join null-padding, join reordering heuristic tests, and telemetry assertion tests. |

---

## Key API & Concepts

### 1. `HashJoinNode` Operator (`internal/engine/sql/planner/hash_join.go`)

`HashJoinNode` performs in-memory hash matching. It selects the build relation, hashes join key datums, and streams matches from the probe relation.

```go
package planner

import (
	"context"
	"io"
	"time"

	"github.com/plomvix/plomvix/internal/engine"
)

type HashJoinNode struct {
	left      Operator // Probe child
	right     Operator // Build child
	leftKey   int      // Join column index for left relation
	rightKey  int      // Join column index for right relation
	isLeftJoin bool     // True if LEFT OUTER JOIN
	outSchema engine.Schema

	// Build-phase state
	hashTable   map[any][]engine.Row
	buildTime   time.Duration

	// Probe-phase state
	probeIter   Operator
	activeProbe engine.Row
	activeMatches []engine.Row
	matchIdx    int
	matchedAny  bool // For LEFT outer join tracking
}

func NewHashJoinNode(left, right Operator, leftKey, rightKey int, isLeftJoin bool) *HashJoinNode {
	leftSchema := left.Schema()
	rightSchema := right.Schema()
	cols := append(leftSchema.Columns, rightSchema.Columns...)

	return &HashJoinNode{
		left:       left,
		right:      right,
		leftKey:    leftKey,
		rightKey:   rightKey,
		isLeftJoin: isLeftJoin,
		outSchema:  engine.Schema{Columns: cols},
		hashTable:  make(map[any][]engine.Row),
	}
}

func (n *HashJoinNode) Open(ctx context.Context) error {
	if err := n.left.Open(ctx); err != nil {
		return err
	}
	if err := n.right.Open(ctx); err != nil {
		_ = n.left.Close()
		return err
	}

	// 1. Build Phase: Consume RIGHT relation entirely and hash join key values
	start := time.Now()
	for {
		row, err := n.right.Next(ctx)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		keyVal := row[n.rightKey].Value
		n.hashTable[keyVal] = append(n.hashTable[keyVal], row)
	}
	n.buildTime = time.Since(start)

	n.matchIdx = 0
	return nil
}

func (n *HashJoinNode) Next(ctx context.Context) (engine.Row, error) {
	for {
		if n.matchIdx < len(n.activeMatches) {
			matchRow := n.activeMatches[n.matchIdx]
			n.matchIdx++
			n.matchedAny = true
			return n.combine(n.activeProbe, matchRow), nil
		}

		// If active probe matched nothing and is LEFT join, output null-padded outer row
		if n.isLeftJoin && n.activeProbe != nil && !n.matchedAny {
			padded := n.combine(n.activeProbe, n.nullRow(n.right.Schema()))
			n.activeProbe = nil
			return padded, nil
		}

		// Pull next probe row
		pr, err := n.left.Next(ctx)
		if err != nil {
			return nil, err // io.EOF propagates naturally
		}

		n.activeProbe = pr
		n.matchedAny = false
		keyVal := pr[n.leftKey].Value
		
		if matches, ok := n.hashTable[keyVal]; ok {
			n.activeMatches = matches
			n.matchIdx = 0
		} else {
			n.activeMatches = nil
			n.matchIdx = 0
			// Loop immediately to process next probe row or output LEFT outer nulls
		}
	}
}

func (n *HashJoinNode) combine(left, right engine.Row) engine.Row {
	out := make(engine.Row, len(left)+len(right))
	copy(out, left)
	copy(out[len(left):], right)
	return out
}

func (n *HashJoinNode) nullRow(schema engine.Schema) engine.Row {
	nulls := make(engine.Row, len(schema.Columns))
	for i, col := range schema.Columns {
		nulls[i] = engine.Datum{Type: col.Type, Value: nil}
	}
	return nulls
}

func (n *HashJoinNode) Close() error {
	n.hashTable = nil
	n.activeProbe = nil
	n.activeMatches = nil
	errL := n.left.Close()
	errR := n.right.Close()
	if errL != nil {
		return errL
	}
	return errR
}

func (n *HashJoinNode) Schema() engine.Schema {
	return n.outSchema.DeepCopy()
}
```

---

## Tasks

1. **Implement `HashJoinNode`:** Create `internal/engine/sql/planner/hash_join.go` with equijoin hashing logic.
2. **Add LEFT OUTER JOIN to `NestedLoopJoinNode`:** Update loop join in `join.go` with outer row matching state trackers and null-padding logic when the inner loop yields zero matches.
3. **Write Optimizer Selector:** Implement `optimizer.go` that inspects join expressions:
   - If predicate is equality (e.g. `colA = colB`), select `HashJoinNode`.
   - If predicate is non-equality, default to `NestedLoopJoinNode`.
   - If multiple joins exist, query Catalog table size stats to determine join ordering (Greedy Reordering heuristic).
4. **Outer Join AST Parsing:** Extend `Translate` in `translate.go` to support `LEFT OUTER JOIN` syntax via `vitess.JoinTableExpr` matching `LeftJoinType`. Reject right/full joins with `ErrUnsupportedFeature`.
5. **Structured Telemetry:** Add performance logging via slog in operator `Close()` methods detailing build/probe latencies and selection type.
6. **Enterprise Join Tests & Benchmarks:** Unit tests verifying left outer joins with missing inner matches (assert null fields), cost-based switch accuracy, and benchmarks demonstrating O(N+M) hash execution over O(N*M) loop join.

---

## Completion Criteria

- [ ] `HashJoinNode` correctly joins equijoin keys with linear scaling.
- [ ] `LEFT OUTER JOIN` null-pads missing inner records in both loop and hash joins.
- [ ] Right and full outer joins are correctly rejected with `ErrUnsupportedFeature`.
- [ ] Optimizer correctly switches to `NestedLoopJoin` for non-equality predicates.
- [ ] Joins emit slog telemetry upon execution completion.
