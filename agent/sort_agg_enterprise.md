# Sorting and Aggregation Enterprise

| Field | Value |
| :--- | :--- |
| **Source** | `agent/sort_agg_enterprise.md` |
| **Package(s)** | `internal/engine/sql/planner` |
| **Purpose** | Production-harden sort and aggregate operations with disk-spill External Merge Sort, O(1) memory Stream Aggregation, cost-based aggregate selection, and structured telemetry logs. |
| **Dependencies** | Sorting and Aggregation Setup plan, Catalog Setup/Enterprise plans. |

## Honest Contracts & Known Trade-offs

1. **Disk-Spill External Sort I/O penalty:** If the size of the buffered row dataset exceeds the configurable memory threshold `SQLEngineConfig.MaxSortMemoryBytes` (default 64MB), the operator (`ExternalSortNode`) splits rows into chunks, sorts them, serializes them, and writes them to temporary run files in the workspace directory. Merging runs uses a Priority Queue (min-heap). While this guarantees protection against OOM, it introduces significant disk I/O latency.
2. **Temporary File Management:** Temporary run files are created in the workspace directory (e.g. `scratch/sort-runs/`). They must be eagerly cleaned up when `Close()` is called. If the query execution crashes abruptly, orphaned run files must be swept and pruned during engine boot.
3. **Stream Aggregation selection requires sorted input:** The optimizer will select the stream aggregation operator (`SortAggNode`) instead of `HashAggNode` only if the child operator guarantees sorted output matching the grouping keys (e.g. via index scan or prior sort node). `SortAggNode` aggregates grouped fields on-the-fly with O(1) auxiliary memory.
4. **No hybrid hashing for aggregates:** If aggregation grouping keys do not match child sorted order, it falls back to the in-memory `HashAggNode`. Hybrid hash aggregation (spilling aggregate buckets to disk) is deferred to a future clustering execution plan.
5. **Detailed Telemetry is emitted:** All sort and aggregate operations emit execution telemetry via slog `INFO` records at `Close()`. This includes: sorting type (In-memory vs External), count of temporary files created, spill volume in bytes, and execution duration.

---

## Deliverables

| File | Purpose |
| :--- | :--- |
| `internal/engine/sql/planner/external_sort.go` | Implement `ExternalSortNode` with priority queue merge and run file serialization. |
| `internal/engine/sql/planner/stream_agg.go` | Implement `SortAggNode` for streaming, sorted-input aggregation. |
| `internal/engine/sql/planner/optimizer.go` | Update optimizer to switch between `HashAgg` vs `SortAgg`, and `Sort` vs `ExternalSort` based on memory estimations. |
| `internal/engine/sql/planner/sort_agg_enterprise_test.go` | Benchmarks comparing sort vs external sort, tests verifying temporary file cleanup, memory foot-print metrics for stream aggregation, and telemetry assertions. |

---

## Key API & Concepts

### 1. `ExternalSortNode` Operator (`internal/engine/sql/planner/external_sort.go`)

`ExternalSortNode` manages chunked sorting and priority queue k-way merge.

```go
package planner

import (
	"container/heap"
	"context"
	"io"
	"os"
	"time"

	"github.com/plomvix/plomvix/internal/engine"
)

type ExternalSortNode struct {
	child      Operator
	keys       []SortKey
	maxMemBytes int64
	outSchema  engine.Schema

	// Run management
	tempDir   string
	runFiles  []*os.File
	readers   []*runReader
	pq        *sortPriorityQueue
	opened    bool

	// Telemetry metrics
	spillBytes  int64
	fileCount   int
	spillTime   time.Duration
}

type runReader struct {
	file   *os.File
	schema engine.Schema
}

func (r *runReader) ReadRow() (engine.Row, error) {
	// Custom binary deserializer for run files
	return nil, nil
}

// sortPriorityQueue implements container/heap.Interface for merging sorted runs
type sortPriorityQueue struct {
	items []pqItem
	keys  []SortKey
}

type pqItem struct {
	row      engine.Row
	runIndex int
}

func (pq sortPriorityQueue) Len() int           { return len(pq.items) }
func (pq sortPriorityQueue) Less(i, j int) bool {
	// Custom comparator applying keys DESC/ASC rules
	return false
}
```

### 2. `SortAggNode` Operator (`internal/engine/sql/planner/stream_agg.go`)

`SortAggNode` aggregates contiguous matching groupings, eliminating map lookups.

```go
type SortAggNode struct {
	child     Operator
	groupKeys []int
	aggs      []AggRequest
	outSchema engine.Schema

	activeGroup engine.Row
	accumulators []aggAccumulator // Trackers for COUNT, SUM, etc.
}

func (n *SortAggNode) Next(ctx context.Context) (engine.Row, error) {
	for {
		r, err := n.child.Next(ctx)
		if err != nil {
			if err == io.EOF {
				if n.activeGroup == nil {
					return nil, io.EOF
				}
				// Yield final group bucket
				res := n.buildResultRow(n.activeGroup)
				n.activeGroup = nil
				return res, nil
			}
			return nil, err
		}

		if n.activeGroup == nil {
			n.activeGroup = n.extractGroupKey(r)
			n.resetAccumulators()
			n.accumulate(r)
			continue
		}

		currentGroup := n.extractGroupKey(r)
		if n.groupEqual(n.activeGroup, currentGroup) {
			n.accumulate(r)
			continue
		}

		// Group changed. Yield active group result and start next group
		res := n.buildResultRow(n.activeGroup)
		n.activeGroup = currentGroup
		n.resetAccumulators()
		n.accumulate(r)
		return res, nil
	}
}
```

---

## Tasks

1. **Implement `ExternalSortNode`:** Code file-based chunk sorting. Serialize rows to binary run files, and implement priority queue k-way merge matching sorting keys.
2. **Implement `SortAggNode`:** Implement stream aggregation executing O(1) auxiliary memory grouping on pre-sorted child rows.
3. **Update Optimizer Decisions:** Update join/sort optimization logic:
   - Estimate input sizes using Catalog.
   - If estimated size exceeds `MaxSortMemoryBytes`, select `ExternalSortNode`.
   - If input operator guarantees grouping-key sort order, select `SortAggNode`.
4. **Cleanup Run Files:** Ensure `Close()` calls in `external_sort.go` close all files and delete temporary directories eagerly.
5. **Add Temporary File Boot Sweeper:** Update SQL Engine bootstrap lifecycle to detect and prune orphaned run files from previous aborted query executions.
6. **Enterprise Tests & Benchmarks:** Benchmarks demonstrating:
   - External sorting stability under low memory configuration limits.
   - Stream aggregation memory consumption comparison showing O(1) vs O(K) in-memory hash aggregation.
   - Assert run files are cleanly deleted post operator close.
   - Telemetry slog WARN/INFO assertions.

---

## Completion Criteria

- [ ] `ExternalSortNode` sorts datasets exceeding memory limits by spilling runs to disk.
- [ ] `SortAggNode` performs aggregation in O(1) memory on pre-sorted data.
- [ ] Run files are deleted eagerly when the sort operator closes.
- [ ] Orphaned run files are pruned on engine startup.
- [ ] Benchmark comparison confirms performance characteristics.
