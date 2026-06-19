# Joins and Multi-Table Execution Enterprise

| Field | Value |
| :--- | :--- |
| **Source** | `agent/joins_enterprise.md` |
| **Package(s)** | `internal/engine/sql/planner` |
| **Purpose** | Production-harden joins with in-memory Hash Join execution for high-performance equijoins, LEFT OUTER JOIN support, greedy cost-based join reordering, and structured performance telemetry. |
| **Dependencies** | Joins and Multi-Table Execution Setup plan, Catalog Setup/Enterprise plans. |

## Honest Contracts & Known Trade-offs

1. **In-Memory Hash Join build relation constraint:** The Hash Join operator (`HashJoinNode`) builds an in-memory hash table of the build relation. If the build table is extremely large, this can result in high memory consumption. Disk-spill hash join or hybrid hash join is deferred to a future sharding/disk compaction plan.
2. **Equijoins only for Hash Joins:** Hash joins can only be selected if the join condition contains at least one equality predicate (e.g. `t1.id = t2.id`). The `HashJoinNode` may only be selected when the equality join key is part of an AND-only conjunctive predicate tree. Any OR in the join predicate forces `NestedLoopJoinNode` execution.
3. **Greedy Join Reordering and Logical Order Preservation:** The planner optimizes multi-way joins (e.g. `A JOIN B JOIN C`) using a greedy heuristic. However, join reordering must preserve logical output schema order matching the original SQL `FROM` clause. If physical execution orders column fields differently, the planner must automatically insert a `ProjectNode` above the join tree to re-order output columns.
4. **Outer Joins limited to LEFT:** `LEFT OUTER JOIN` is supported. `RIGHT OUTER JOIN` and `FULL OUTER JOIN` are parsed but return `ErrUnsupportedFeature`. For `LEFT OUTER JOIN`, the optimizer must NOT swap build/probe sides across the join boundary (the left/logical outer relation must remain the probe side) to preserve outer join correctness. It can, however, still build the hash table from the right child.
5. **Telemetry details logged on Close:** Joins emit execution metrics via structured `slog` logs at `INFO` level. This includes: join type, build rows, probe rows, output rows, build phase duration, probe phase duration, and estimated memory bytes. Telemetry occurs during the `Close()` call.
6. **SQL NULL join semantics:** Hash join must skip build rows with `NULL` join keys and never match probe rows with `NULL` keys. For `LEFT OUTER JOIN`, a probe row with a `NULL` key must simply emit a null-padded right side.
7. **Residual predicate evaluation:** The hash join operator supports residual predicates (non-equijoin conditions, e.g. `t1.id = t2.id AND t1.status = 'active'`). It hashes on the equality key and evaluates the residual predicate on any hash matches.
8. **Eager resource cleanup:** In `Open()`, if the build phase fails, both left and right operators are closed. On build success, the build child relation (`right`) is immediately closed to free system resources, and `Close()` is made idempotent to safely handle already-closed children.

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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/plomvix/plomvix/internal/engine"
)

var (
	ErrUnsupportedJoinKey = errors.New("planner: unsupported join key type")
)

type HashKey string

// encodeHashKey converts a datum into a string representation for map lookup.
// Skips NULL keys (returns empty string) and returns ErrUnsupportedJoinKey for unhashable types.
func encodeHashKey(d engine.Datum) (HashKey, error) {
	if d.Value == nil {
		return "", nil // NULL key
	}
	switch d.Type {
	case engine.TypeInt64:
		return HashKey(fmt.Sprintf("i:%d", d.Value.(int64))), nil
	case engine.TypeUint64:
		return HashKey(fmt.Sprintf("u:%d", d.Value.(uint64))), nil
	case engine.TypeFloat64:
		return HashKey(fmt.Sprintf("f:%f", d.Value.(float64))), nil
	case engine.TypeBool:
		return HashKey(fmt.Sprintf("b:%t", d.Value.(bool))), nil
	case engine.TypeString:
		return HashKey("s:" + d.Value.(string)), nil
	default:
		return "", ErrUnsupportedJoinKey
	}
}

type HashJoinNode struct {
	left          Operator // Probe child
	right         Operator // Build child
	leftKey       int      // Join column index for left relation
	rightKey      int      // Join column index for right relation
	isLeftJoin    bool     // True if LEFT OUTER JOIN
	residual      BoundExpr
	plannerSchema PlannerSchema
	outSchema     engine.Schema
	logger        *slog.Logger

	// Build-phase state
	hashTable   map[HashKey][]engine.Row
	buildTime   time.Duration
	rightClosed bool

	// Probe-phase state
	activeProbe   engine.Row
	activeMatches []engine.Row
	matchIdx      int
	matchedAny    bool // For LEFT outer join tracking

	// Telemetry metrics
	buildRows      int
	probeRows      int
	outputRows     int
	probeTime      time.Duration
	estimatedBytes int64
}

func NewHashJoinNode(
	left, right Operator,
	leftKey, rightKey int,
	isLeftJoin bool,
	residual BoundExpr,
	logger *slog.Logger,
) *HashJoinNode {
	leftPS := GetPlannerSchema(left)
	rightPS := GetPlannerSchema(right)

	fields := make([]SchemaField, 0, len(leftPS.Fields)+len(rightPS.Fields))
	fields = append(fields, leftPS.Fields...)
	fields = append(fields, rightPS.Fields...)

	plannerSchema := PlannerSchema{Fields: fields}

	return &HashJoinNode{
		left:          left,
		right:         right,
		leftKey:       leftKey,
		rightKey:      rightKey,
		isLeftJoin:    isLeftJoin,
		residual:      residual,
		plannerSchema: plannerSchema,
		outSchema:     plannerSchema.ToEngineSchema(),
		logger:        logger,
		hashTable:     make(map[HashKey][]engine.Row),
	}
}

func (n *HashJoinNode) PlannerSchema() PlannerSchema {
	return n.plannerSchema
}

func (n *HashJoinNode) Open(ctx context.Context) error {
	if err := n.left.Open(ctx); err != nil {
		return err
	}
	n.rightClosed = false
	if err := n.right.Open(ctx); err != nil {
		_ = n.left.Close()
		return err
	}

	n.hashTable = make(map[HashKey][]engine.Row)
	n.buildRows = 0
	n.probeRows = 0
	n.outputRows = 0
	n.estimatedBytes = 0
	n.buildTime = 0
	n.probeTime = 0
	n.activeProbe = engine.Row{}
	n.activeMatches = nil
	n.matchIdx = 0
	n.matchedAny = false

	// 1. Build Phase: Consume RIGHT relation entirely and hash join key values
	start := time.Now()
	for {
		row, err := n.right.Next(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			_ = n.left.Close()
			_ = n.right.Close()
			n.rightClosed = true
			return err
		}
		
		keyVal, err := encodeHashKey(row.Datums[n.rightKey])
		if err != nil {
			_ = n.left.Close()
			_ = n.right.Close()
			n.rightClosed = true
			return err
		}
		if keyVal == "" {
			continue // SQL NULL semantics: skip build rows with NULL keys
		}

		n.hashTable[keyVal] = append(n.hashTable[keyVal], row.DeepCopy())
		n.buildRows++
		n.estimatedBytes += int64(len(row.Datums)) * 16 // crude estimation
	}
	n.buildTime = time.Since(start)

	// Build phase success: eagerly close the build child to free resources
	_ = n.right.Close()
	n.rightClosed = true

	n.matchIdx = 0
	return nil
}

func (n *HashJoinNode) Next(ctx context.Context) (engine.Row, error) {
	probeStart := time.Now()
	defer func() {
		n.probeTime += time.Since(probeStart)
	}()

	for {
		if n.matchIdx < len(n.activeMatches) {
			matchRow := n.activeMatches[n.matchIdx]
			n.matchIdx++
			
			// Concatenate matching tuple (joined rows have RowID == 0)
			combined := n.combine(n.activeProbe, matchRow)

			if n.residual != nil {
				d, err := n.residual.Eval(combined)
				if err != nil {
					return engine.Row{}, err
				}
				if b, ok := d.Value.(bool); ok && b {
					n.matchedAny = true
					n.outputRows++
					return combined, nil
				}
				continue
			}

			n.matchedAny = true
			n.outputRows++
			return combined, nil
		}

		// If active probe matched nothing and is LEFT join, output null-padded outer row
		if n.isLeftJoin && n.activeProbe.Datums != nil && !n.matchedAny {
			padded := n.combine(n.activeProbe, n.nullRow(n.right.Schema()))
			n.activeProbe.Datums = nil
			n.outputRows++
			return padded, nil
		}

		// Pull next probe row
		pr, err := n.left.Next(ctx)
		if err != nil {
			return engine.Row{}, err // io.EOF propagates naturally
		}

		n.probeRows++
		n.activeProbe = pr.DeepCopy()
		n.matchedAny = false

		keyVal, err := encodeHashKey(pr.Datums[n.leftKey])
		if err != nil {
			return engine.Row{}, err
		}
		if keyVal == "" {
			n.activeMatches = nil
			n.matchIdx = 0
			continue // SQL NULL semantics: NULL key in probe never matches inner keys
		}
		
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
	out := engine.Row{
		Datums: make([]engine.Datum, 0, len(left.Datums)+len(right.Datums)),
		RowID:  0,
	}
	out.Datums = append(out.Datums, left.Datums...)
	out.Datums = append(out.Datums, right.Datums...)
	return out
}

func (n *HashJoinNode) nullRow(schema engine.Schema) engine.Row {
	nulls := engine.Row{
		Datums: make([]engine.Datum, len(schema.Columns)),
		RowID:  0,
	}
	for i, col := range schema.Columns {
		nulls.Datums[i] = engine.Datum{Type: col.DataType, Value: nil}
	}
	return nulls
}

func (n *HashJoinNode) Close() error {
	// Emit telemetry metrics on Close
	if n.logger != nil {
		n.logger.Info("joins: HashJoin execution metrics",
			slog.String("join_algorithm", "hash"),
			slog.Bool("left_outer", n.isLeftJoin),
			slog.Int("build_rows", n.buildRows),
			slog.Int("probe_rows", n.probeRows),
			slog.Int("output_rows", n.outputRows),
			slog.Duration("build_time_ms", n.buildTime),
			slog.Duration("probe_time_ms", n.probeTime),
			slog.Int64("estimated_memory_bytes", n.estimatedBytes),
		)
	}

	n.hashTable = nil
	n.activeProbe.Datums = nil
	n.activeMatches = nil
	
	// Eager cleanups
	errL := n.left.Close()
	var errR error
	if !n.rightClosed {
		errR = n.right.Close()
		n.rightClosed = true
	}
	if errL != nil {
		return errL
	}
	return errR
}

func (n *HashJoinNode) Schema() engine.Schema {
	return n.outSchema.DeepCopy()
}
```

### 2. Refactored `NestedLoopJoinNode` Operator (`internal/engine/sql/planner/join.go`)

In the Enterprise tier, `NestedLoopJoinNode` is refactored to support `LEFT OUTER JOIN` null-padding, non-equality predicate evaluation, and performance telemetry tracking.

```go
type NestedLoopJoinNode struct {
	left          Operator
	right         Operator
	cond          BoundExpr
	isLeftJoin    bool
	logger        *slog.Logger
	plannerSchema PlannerSchema
	outSchema     engine.Schema

	leftRow     engine.Row // Cached current row from the outer table
	leftMatched bool       // Track if the current outer row matched any inner row (for LEFT JOIN)

	// Telemetry metrics
	probeRows  int
	outputRows int
	probeTime  time.Duration
}

func NewNestedLoopJoinNode(
	left, right Operator,
	cond BoundExpr,
	isLeftJoin bool,
	logger *slog.Logger,
) *NestedLoopJoinNode {
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

	n.probeRows = 0
	n.outputRows = 0
	n.probeTime = 0

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
	n.leftRow = lr.DeepCopy()
	n.leftMatched = false
	n.probeRows++
	return nil
}

func (n *NestedLoopJoinNode) Next(ctx context.Context) (engine.Row, error) {
	start := time.Now()
	defer func() {
		n.probeTime += time.Since(start)
	}()

	for {
		if n.leftRow.Datums == nil {
			return engine.Row{}, io.EOF
		}

		rr, err := n.right.Next(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				// If we are at the end of the inner relation, check if we need to emit a null-padded LEFT JOIN row
				if n.isLeftJoin && !n.leftMatched {
					paddedRow := engine.Row{
						Datums: make([]engine.Datum, 0, len(n.leftRow.Datums)+len(n.right.Schema().Columns)),
						RowID:  0,
					}
					paddedRow.Datums = append(paddedRow.Datums, n.leftRow.Datums...)
					for _, col := range n.right.Schema().Columns {
						paddedRow.Datums = append(paddedRow.Datums, engine.Datum{Type: col.DataType, Value: nil})
					}
					
					// Re-open right child before returning the padded row, ready for the next left row
					if err := n.right.Close(); err != nil {
						return engine.Row{}, err
					}
					if err := n.right.Open(ctx); err != nil {
						return engine.Row{}, err
					}

					// Advance outer row
					lr, errAdvance := n.left.Next(ctx)
					if errAdvance != nil {
						n.leftRow.Datums = nil
						if errors.Is(errAdvance, io.EOF) {
							// Return this padded row, next call will hit top-level io.EOF
							n.outputRows++
							return paddedRow, nil
						}
						return engine.Row{}, errAdvance
					}
					n.leftRow = lr.DeepCopy()
					n.leftMatched = false
					n.probeRows++

					n.outputRows++
					return paddedRow, nil
				}

				// Re-open right child relation
				if err := n.right.Close(); err != nil {
					return engine.Row{}, err
				}
				if err := n.right.Open(ctx); err != nil {
					return engine.Row{}, err
				}
				
				// Advance left child row
				lr, errAdvance := n.left.Next(ctx)
				if errAdvance != nil {
					n.leftRow.Datums = nil // Ensure we return io.EOF on subsequent calls
					if errors.Is(errAdvance, io.EOF) {
						return engine.Row{}, io.EOF
					}
					return engine.Row{}, errAdvance
				}
				n.leftRow = lr.DeepCopy()
				n.leftMatched = false
				n.probeRows++
				continue
			}
			return engine.Row{}, err
		}

		// Concatenate matching tuple
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
	// Emit telemetry metrics on Close
	if n.logger != nil {
		n.logger.Info("joins: NestedLoopJoin execution metrics",
			slog.String("join_algorithm", "nested_loop"),
			slog.Bool("left_outer", n.isLeftJoin),
			slog.Int("probe_rows", n.probeRows),
			slog.Int("output_rows", n.outputRows),
			slog.Duration("probe_time_ms", n.probeTime),
		)
	}

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

---

## Tasks

1. **Implement `HashJoinNode`:** Create `internal/engine/sql/planner/hash_join.go` with equijoin hashing logic.
2. **Add LEFT OUTER JOIN to `NestedLoopJoinNode`:** Update loop join in `join.go` with outer row matching state trackers and null-padding logic when the inner loop yields zero matches. Update all `NewNestedLoopJoinNode(left, right, cond)` call sites to use `NewNestedLoopJoinNode(left, right, cond, isLeftJoin, logger)`. For setup/basic inner joins, pass `isLeftJoin=false` and `logger=nil`.
3. **Write Optimizer Selector:** Implement `optimizer.go` that inspects join expressions:
   - HashJoinNode may only be selected when the equality join key is part of an AND-only conjunctive predicate tree. Any OR in the join predicate forces NestedLoopJoinNode execution.
   - If predicate is equality (e.g. `colA = colB`), select `HashJoinNode`.
   - If predicate is non-equality, default to `NestedLoopJoinNode`.
   - If multiple joins exist, query Catalog table size stats to determine join ordering (Greedy Reordering heuristic).
   - If greedy reordering changes physical child order from SQL FROM order, insert a `ProjectNode` above the join tree to restore logical output column order before projection binding and SELECT * output.
4. **Outer Join AST Parsing:** Extend `Translate` in `translate.go` to support `LEFT OUTER JOIN` syntax via `vitess.JoinTableExpr` matching `LeftJoinType`. Reject right/full joins with `ErrUnsupportedFeature`.
5. **Structured Telemetry:** Add performance logging via slog in operator `Close()` methods detailing build/probe latencies and selection type. For `HashJoinNode`, log `join_algorithm` as `"hash"`, `left_outer` (bool), along with build/probe latencies, rows, and memory bytes. For `NestedLoopJoinNode`, log `join_algorithm` as `"nested_loop"`, `left_outer` (bool), along with latencies and rows.
6. **Enterprise Join Tests & Benchmarks:** Unit tests verifying:
   - Left outer joins with missing inner matches (assert null fields).
   - Cost-based switch accuracy.
   - Benchmarks demonstrating O(N+M) hash execution over O(N*M) loop join.
   - Join ordering column stability: `SELECT * FROM large_a JOIN small_b ON ...` where physical build/probe swaps but output columns remain `large_a.*, small_b.*`.
   - LEFT JOIN + residual predicate test: `SELECT * FROM a LEFT JOIN b ON a.id = b.id AND b.status = 'active'` where a row with matched IDs but status `inactive` yields one output row with `a.*` and NULL-padded `b.*` fields.

---

## Completion Criteria

- [ ] `HashJoinNode` correctly joins equijoin keys with linear scaling.
- [ ] `LEFT OUTER JOIN` null-pads missing inner records in both loop and hash joins.
- [ ] Right and full outer joins are correctly rejected with `ErrUnsupportedFeature`.
- [ ] Optimizer correctly switches to `NestedLoopJoin` for non-equality predicates.
- [ ] Joins emit slog telemetry upon execution completion.
