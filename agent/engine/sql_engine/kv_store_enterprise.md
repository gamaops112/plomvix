# kv_store_enterprise.md

# Plomvix In-Memory KV Store Enterprise Hardening Plan

## Purpose

Harden the existing `internal/engine/sql/store` package into a
safer, more measurable, production-grade foundation.

This plan adds a dedicated sorted-invariant stress test under heavy
concurrent churn, and adds size-scaling benchmarks that measure how
Put/Get/Delete/Scan performance changes as the store grows. This
data is the concrete signal for when the in-memory store should be
replaced by on-disk storage.

This is still store hardening only. No new API surface, no new
data structures, no iterator-style Scan, no fuzz testing of the
search logic (not a trust boundary), no persistence.

Do not add WAL.
Do not add disk persistence.
Do not add storage pages.
Do not add transactions.
Do not add iterator-style Scan.
Do not add API server.
Do not add UI.
Do not wire the store into lifecycle or runtime.

---

## Feature Name

```text
In-Memory KV Store Enterprise Hardening
```

Plan file:

```text
kv_store_enterprise.md
```

Existing package:

```text
internal/engine/sql/store
```

---

## Required Starting State

This plan starts only after `kv_store_setup.md` is completed and
verified.

Before starting this plan, the project must already have:

```text
internal/engine/sql/store/store.go
internal/engine/sql/store/store_test.go
docs/sql_store.md
```

The store package must already expose:

```go
type Entry struct {
    Key   key.Key
    Value []byte
}

type Store struct {
    // unexported fields only
}

func New() *Store

func (s *Store) Put(k key.Key, value []byte) error
func (s *Store) Get(k key.Key) ([]byte, error)
func (s *Store) Delete(k key.Key) error
func (s *Store) Scan(start, end key.Key) ([]Entry, error)
func (s *Store) Len() int
```

Existing public errors must include:

```go
var (
    ErrNotFound = errors.New("sql/store: key not found")
    ErrNilStore = errors.New("sql/store: nil store")
)
```

Existing behavior must already include:

* sorted slice ordered by key.Key.Compare()
* single sync.RWMutex concurrency model
* Put/Get copy values at the API boundary
* Delete on missing key is a no-op returning nil
* Delete on nil store returns ErrNilStore
* Scan uses half-open [start, end) range semantics
* Scan with start >= end returns an empty slice, not an error
* nil-receiver safety on every method
* concurrency test passes under go test -race
* zero internal imports beyond internal/engine/sql/key
* `go test ./...` passes
* `go build ./...` passes
* `go test -race ./...` passes

If this starting state is not true, stop and report that
`kv_store_setup.md` is incomplete.

---

## Current Project State

Completed foundation work:

```text
config foundation:              done
enterprise config hardening:    done
basic logger setup:             done
enterprise logger hardening:    done
lifecycle foundation:           done
enterprise lifecycle hardening: done
runtime setup:                  done
enterprise runtime hardening:   done
runtime signal handling:        done
SQL key encoding:               done
SQL key encoding hardening:     done
in-memory KV store:             done
```

Current stage:

```text
in-memory KV store hardening
```

Current feature area:

```text
sql/store enterprise hardening
```

---

## Go Version Requirement

Plomvix uses:

```text
Go 1.22 or later
```

Use only Go standard library.

Do not add external dependencies.

---

## Coding Agent

Coding agent:

```text
DeepSeek V4 Pro
```

If the local environment uses a different exact DeepSeek model identifier,
use the configured DeepSeek coding model available there.

Tasks must be executed one at a time, in exact order.

Do not proceed to the next task until the current task passes verification.

---

## Graphify Rule

For every task:

1. Search Graphify before starting the task if Graphify is available.
2. Update Graphify after completing the task if Graphify is available.
3. If Graphify is unavailable, do not block the task.
4. Mention Graphify availability in the task report.

---

## Global Project Rules

Follow these rules for every task:

* Keep implementation small and focused.
* Do not add future placeholders.
* Do not add unrelated folders.
* Do not remove or rename any existing public API.
* Do not change existing Put/Get/Delete/Scan observable behavior.
* Do not add new public types or functions to the store API.
* Do not import internal/config, internal/logger, internal/lifecycle,
  or internal/runtime from the store package.
* Do not add external dependencies.
* Use only Go standard library.
* Keep tests deterministic except where stress testing concurrency
  is explicitly required.
* Use table-driven tests where useful.
* Do not create a root-level `tests/` directory.
* Do not add WAL, disk persistence, transactions, or iterator-style
  Scan in this plan.
* Do not fuzz the search/insertion logic; it is not a trust boundary
  since it only ever receives keys already validated by
  internal/engine/sql/key.

---

## Dependency Direction Rules

The store package continues to import only:

```text
internal/engine/sql/key
sort
sync
errors
fmt
```

Test files may additionally import:

```text
testing
sync (for stress test WaitGroup)
math/rand
fmt (for generating benchmark data)
```

No new internal imports are introduced.

---

## Enterprise Hardening Goals

This plan adds:

* a dedicated sorted-invariant stress test under heavy concurrent
  Put/Delete churn
* size-scaling benchmarks for Put, Get, Delete, and Scan
* a documented performance baseline that signals when on-disk
  storage becomes necessary
* hardened documentation
* final scope-control review

---

## Non-Goals

Do not implement:

* WAL
* disk persistence
* storage pages
* buffer pool
* transactions
* multi-key atomic batches
* iterator-style Scan
* streaming Scan
* fuzz testing of search/insertion logic
* sharded locking
* new public API surface
* compaction
* snapshots
* config, logger, lifecycle, or runtime integration
* external dependencies

---

## Design Decisions

### Sorted-Invariant Stress Test

The existing `TestStore_ConcurrentAccess` test from
`kv_store_setup.md` proves the absence of data races. It does not
prove that the underlying slice remains correctly sorted after
heavy concurrent mutation.

This plan adds a separate, dedicated test that:

* runs many goroutines performing a mix of Put and Delete operations
  concurrently on overlapping key ranges
* after all goroutines complete, acquires no special access and
  instead uses only the public API (`Scan` over the full range) to
  verify the returned entries are in strictly ascending key order

This test exercises the actual correctness property that matters:
not "did a race detector complain," but "is the data structure's
core invariant still true."

```text
Use go test -race ./... for this stress test specifically, since
race-detector coverage and sorted-invariant verification are two
different properties and both must be checked together.
```

### Benchmark Scope: Size-Scaling

Unlike `sql/key`, where every operation is O(1) regardless of
prior state, the store's `Put` and `Delete` operations are O(n)
due to slice-shift insertion and removal. This means a single
benchmark number at one store size tells you very little about
how performance changes as the store grows.

This plan benchmarks Put, Get, Delete, and Scan at multiple fixed
pre-populated store sizes:

```text
1,000 entries
10,000 entries
100,000 entries
```

For each size, a fresh store is pre-populated with that many
entries before the timed operation begins. Setup time is excluded
from the benchmark timer using `b.ResetTimer()`.

Benchmark stability requirement:

```text
The Put benchmarks (TASK 04 and TASK 05) must keep the store at a
stable size of exactly n entries across every iteration of the
timed loop. A benchmark that inserts a new, never-before-seen key
on every iteration without removing it afterward would cause the
store to grow continuously during the run, meaning later
iterations no longer measure the intended fixed size at all. Both
Put benchmarks pair each Put with an immediate Delete of the same
key so the store size never drifts during the benchmark.
```

Reason:

```text
This produces concrete before/after numbers across store sizes.
The resulting ns/op trend for Put across 1k -> 10k -> 100k entries
is the evidence for deciding when this in-memory store needs to be
replaced by on-disk storage (storage_setup.md). Get and Scan over a
small fixed range should remain roughly O(log n) and largely flat
relative to store size, since they use binary search rather than
shifting. Put and Delete should show clear linear growth. This
divergence is expected and is itself useful information, not a bug
to fix in this plan.
```

This plan does not attempt to optimize Put/Delete performance. It
only measures and documents the existing behavior. Optimization
(if ever needed) is explicitly out of scope here, since the planned
fix for O(n) insertion is replacing this structure with on-disk
storage in a future plan, not micro-optimizing the slice shift.

---

## Task Plan

---

## TASK 01 — Add sorted-invariant stress test

### Goal

Add a dedicated test proving the store's sorted invariant survives
heavy concurrent Put/Delete churn.

### Files

Modify:

```text
internal/engine/sql/store/store_test.go
```

### Package

Keep:

```go
package store_test
```

### Requirements

Add imports as needed:

```go
math/rand
sync
```

Add test:

```go
func TestStore_SortedInvariantUnderConcurrentChurn(t *testing.T)
```

Test shape:

```go
func TestStore_SortedInvariantUnderConcurrentChurn(t *testing.T) {
    s := store.New()
    const numGoroutines = 50
    const opsPerGoroutine = 200
    const keySpace = 500

    var wg sync.WaitGroup
    for g := 0; g < numGoroutines; g++ {
        wg.Add(1)
        go func(seed int64) {
            defer wg.Done()
            r := rand.New(rand.NewSource(seed))
            for i := 0; i < opsPerGoroutine; i++ {
                n := r.Intn(keySpace)
                k := key.EncodeUint64(uint64(n))
                if r.Intn(2) == 0 {
                    _ = s.Put(k, []byte("v"))
                } else {
                    _ = s.Delete(k)
                }
            }
        }(int64(g))
    }
    wg.Wait()

    entries, err := s.Scan(key.EncodeUint64(0), key.EncodeUint64(keySpace+1))
    if err != nil {
        t.Fatalf("scan failed: %v", err)
    }

    for i := 1; i < len(entries); i++ {
        if entries[i-1].Key.Compare(entries[i].Key) >= 0 {
            t.Fatalf("sorted invariant violated at index %d: %v >= %v",
                i, entries[i-1].Key.Bytes(), entries[i].Key.Bytes())
        }
    }
}
```

Rules:

* use a per-goroutine seeded random source so the test is
  reproducible across runs (same seed sequence every time)
* do not assert a specific final length, since concurrent Put and
  Delete on overlapping keys has no single deterministic outcome
* the only invariant under test is strict ascending order after
  all operations complete
* this test must pass under `go test -race ./...`
* do not use sleeps as synchronization; rely on `sync.WaitGroup`

### Verification

Run:

```bash
go test ./...
go build ./...
go test -race ./...
```

### Completion Report

```text
TASK 01 completed.
Files changed:
- internal/engine/sql/store/store_test.go

Verification:
- go test ./...
- go build ./...
- go test -race ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 02 — Add duplicate-key concurrent stress variant

### Goal

Add a second stress test focused specifically on many goroutines
repeatedly writing to the exact same small set of keys, to exercise
the overwrite path under contention.

### Files

Modify:

```text
internal/engine/sql/store/store_test.go
```

### Requirements

Add imports as needed:

```go
fmt
sync
```

Add test:

```go
func TestStore_ConcurrentOverwriteSameKeys(t *testing.T)
```

Test shape:

```go
func TestStore_ConcurrentOverwriteSameKeys(t *testing.T) {
    s := store.New()
    const keySpace = 5
    const numGoroutines = 50
    const writesPerKey = 100

    var wg sync.WaitGroup
    for g := 0; g < numGoroutines; g++ {
        wg.Add(1)
        go func(g int) {
            defer wg.Done()
            for i := 0; i < writesPerKey; i++ {
                for k := 0; k < keySpace; k++ {
                    kk := key.EncodeUint64(uint64(k))
                    val := []byte(fmt.Sprintf("g=%d,i=%d,k=%d", g, i, k))
                    _ = s.Put(kk, val)
                }
            }
        }(g)
    }
    wg.Wait()

    if s.Len() != keySpace {
        t.Fatalf("expected Len()=%d, got %d", keySpace, s.Len())
    }

    entries, err := s.Scan(key.EncodeUint64(0), key.EncodeUint64(keySpace))
    if err != nil {
        t.Fatalf("scan failed: %v", err)
    }
    if len(entries) != keySpace {
        t.Fatalf("expected %d entries from scan, got %d", keySpace, len(entries))
    }
    for i := 1; i < len(entries); i++ {
        if entries[i-1].Key.Compare(entries[i].Key) >= 0 {
            t.Fatalf("sorted invariant violated at index %d", i)
        }
    }
}
```

Deterministic key coverage requirement:

```text
Each goroutine must write to every key in the fixed key set at
least once, using a deterministic inner loop over the full key
space (k := 0; k < keySpace; k++), not random key selection.

Random selection (for example picking a random key index per Put
call) could, with non-zero probability, result in some goroutine
run where one or more of the keySpace keys is never written to by
any goroutine at all during the test. That would make Len() < 5 a
possible outcome purely by chance, turning this into a flaky test
that fails intermittently for reasons unrelated to the property it
is meant to verify.

The deterministic shape above guarantees every one of the 5 keys
is written to by every goroutine, every iteration, removing any
chance of coverage gaps regardless of how goroutines are scheduled
or interleaved by the Go runtime.
```

This specifically targets the overwrite branch of Put under
contention, which is a different code path from the insert branch
already covered by TASK 01. Both branches must be proven safe.

Use `sync.WaitGroup`. Do not use sleeps.

### Verification

Run:

```bash
go test ./...
go build ./...
go test -race ./...
```

### Completion Report

```text
TASK 02 completed.
Files changed:
- internal/engine/sql/store/store_test.go

Verification:
- go test ./...
- go build ./...
- go test -race ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 03 — Add benchmark helper for pre-populated stores

### Goal

Add a shared, unexported test helper that builds a pre-populated
store of a given size, so every size-scaling benchmark in this
plan uses identical setup logic.

### Files

Create:

```text
internal/engine/sql/store/bench_test.go
```

### Package

```go
package store_test
```

### Requirements

Add imports:

```go
fmt
testing

"github.com/plomvix/plomvix/internal/engine/sql/key"
"github.com/plomvix/plomvix/internal/engine/sql/store"
```

Add unexported helper:

```go
func newPopulatedStore(n int) *store.Store {
    s := store.New()
    for i := 0; i < n; i++ {
        k := key.EncodeUint64(uint64(i))
        v := []byte(fmt.Sprintf("value-%d", i))
        _ = s.Put(k, v)
    }
    return s
}
```

Add a second unexported helper that populates a store starting from
an arbitrary key offset, leaving keys below `start` deliberately
absent. This is required by the worst-case front-insert benchmark
in TASK 05, which needs a store where a key smaller than every
existing entry is guaranteed not to already exist:

```go
func newPopulatedStoreFrom(start, n int) *store.Store {
    s := store.New()
    for i := 0; i < n; i++ {
        k := key.EncodeUint64(uint64(start + i))
        v := []byte(fmt.Sprintf("value-%d", start+i))
        _ = s.Put(k, v)
    }
    return s
}
```

Both helpers are used by benchmarks added in subsequent tasks. They
are not themselves benchmarks and must not be registered as one.

Do not add any `BenchmarkXxx` function in this task. This task only
adds the two shared setup helpers.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 03 completed.
Files changed:
- internal/engine/sql/store/bench_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 04 — Add size-scaling Put benchmarks

### Goal

Benchmark `Put` at three fixed pre-populated store sizes to measure
how end-of-slice insertion cost scales with store size, while
keeping the store size stable across the entire benchmark run.

### Files

Modify:

```text
internal/engine/sql/store/bench_test.go
```

### Requirements

Add a parameterized benchmark using `b.Run` for sub-benchmarks:

```go
func BenchmarkPut(b *testing.B) {
    sizes := []int{1000, 10000, 100000}
    for _, n := range sizes {
        b.Run(fmt.Sprintf("size=%d", n), func(b *testing.B) {
            s := newPopulatedStore(n)
            endKey := key.EncodeUint64(uint64(n))

            b.ResetTimer()
            for i := 0; i < b.N; i++ {
                _ = s.Put(endKey, []byte("v"))
                _ = s.Delete(endKey)
            }
        })
    }
}
```

Important — why Put and Delete are paired in this benchmark:

```text
newPopulatedStore(n) creates keys 0..n-1. A single fixed endKey of
n is guaranteed to be larger than every pre-populated key, so each
Put(endKey, ...) call always measures end-of-slice append, the
best-case insertion position for a sorted slice.

If only Put(endKey, ...) were called repeatedly without the
matching Delete, the first iteration would insert endKey, and
every iteration after that would silently become an overwrite of
the now-existing endKey rather than a fresh insertion. Worse, if a
different growing key were used per iteration instead (such as
n+i), the store would grow continuously for the entire b.N loop,
meaning later iterations are no longer measuring the intended
store size at all.

Pairing Put with an immediate Delete of the same key keeps the
store at a stable size of exactly n entries for every iteration of
the benchmark, so every iteration measures the same well-defined
condition: appending one entry to a store of size n. This
benchmark measures best-case append insertion plus its matching
cleanup cost together, not insertion in isolation. Document this
combined cost explicitly wherever these benchmark results are
discussed or reported.
```

* call `b.ResetTimer()` after the store is pre-populated, so setup
  cost is excluded from the measured time
* use `b.Run` sub-benchmarks named by size so results are easy to
  compare in `go test -bench` output
* do not use a different key per iteration; the fixed `endKey`
  combined with the Put/Delete pairing is what keeps store size
  stable, and is required for this benchmark to be meaningful

### Verification

Run:

```bash
go test ./...
go build ./...
go test -bench=BenchmarkPut -run=^$ ./internal/engine/sql/store/
```

Expected:

```text
go test ./... passes.
Three sub-benchmark results are reported, one per size.
Store size remains stable at n entries throughout each sub-benchmark.
```

### Completion Report

```text
TASK 04 completed.
Files changed:
- internal/engine/sql/store/bench_test.go

Verification:
- go test ./...
- go build ./...
- go test -bench=BenchmarkPut -run=^$ ./internal/engine/sql/store/

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 05 — Add worst-case Put benchmark

### Goal

Benchmark `Put` specifically for worst-case insertion position
(beginning of the slice) at the same three sizes, since
end-of-slice insertion (TASK 04) is the best case and does not by
itself reveal the true cost of the O(n) shift.

### Files

Modify:

```text
internal/engine/sql/store/bench_test.go
```

### Requirements

Add:

```go
func BenchmarkPut_WorstCaseFrontInsert(b *testing.B) {
    sizes := []int{1000, 10000, 100000}
    for _, n := range sizes {
        b.Run(fmt.Sprintf("size=%d", n), func(b *testing.B) {
            s := newPopulatedStoreFrom(1, n)
            frontKey := key.EncodeUint64(0)

            b.ResetTimer()
            for i := 0; i < b.N; i++ {
                _ = s.Put(frontKey, []byte("v"))
                _ = s.Delete(frontKey)
            }
        })
    }
}
```

Important clarification on benchmark design:

```text
newPopulatedStore(n) populates keys 0..n-1, which means key 0
already exists in that store. Using newPopulatedStore here would
make the first Put a genuine front-insert but every Put on every
later iteration an overwrite of an already-present key, since
frontKey would never actually be absent after iteration 0. That
silently changes what the benchmark measures partway through the
run and corrupts its stated purpose.

newPopulatedStoreFrom(1, n) instead populates keys 1..n, leaving
key 0 deliberately and permanently absent from the populated
store. frontKey (key 0) is therefore guaranteed to be smaller than
every existing key and guaranteed to be missing before every Put
call, since the matching Delete in the same iteration removes it
again immediately after insertion. This keeps every iteration a
genuine front-of-slice insertion into a store that remains at a
stable size of exactly n entries throughout the benchmark.

As with TASK 04, this benchmark measures the combined cost of
front-insertion plus its matching cleanup delete, not insertion in
complete isolation. Document this combined cost explicitly
wherever these benchmark results are discussed or reported.
```

* call `b.ResetTimer()` after setup
* use `b.Run` sub-benchmarks named by size
* do not use `newPopulatedStore` for this benchmark; it must use
  `newPopulatedStoreFrom(1, n)` so key 0 is genuinely and
  permanently absent from the pre-populated data

### Verification

Run:

```bash
go test ./...
go build ./...
go test -bench=BenchmarkPut_WorstCaseFrontInsert -run=^$ ./internal/engine/sql/store/
```

Expected:

```text
go test ./... passes.
Every iteration performs a genuine front-of-slice insertion,
since key 0 is never part of the pre-populated data and is removed
again at the end of each iteration.
```

### Completion Report

```text
TASK 05 completed.
Files changed:
- internal/engine/sql/store/bench_test.go

Verification:
- go test ./...
- go build ./...
- go test -bench=BenchmarkPut_WorstCaseFrontInsert -run=^$ ./internal/engine/sql/store/

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 06 — Add size-scaling Get benchmarks

### Goal

Benchmark `Get` at the same three fixed store sizes to confirm
lookup remains roughly O(log n) regardless of store size.

### Files

Modify:

```text
internal/engine/sql/store/bench_test.go
```

### Requirements

Add:

```go
func BenchmarkGet(b *testing.B) {
    sizes := []int{1000, 10000, 100000}
    for _, n := range sizes {
        b.Run(fmt.Sprintf("size=%d", n), func(b *testing.B) {
            s := newPopulatedStore(n)
            lookupKey := key.EncodeUint64(uint64(n / 2))
            b.ResetTimer()
            for i := 0; i < b.N; i++ {
                _, _ = s.Get(lookupKey)
            }
        })
    }
}
```

* lookup a key in the middle of the populated range so the
  benchmark reflects a typical binary search cost, not a
  best-case/worst-case edge
* call `b.ResetTimer()` after setup

### Verification

Run:

```bash
go test ./...
go build ./...
go test -bench=BenchmarkGet -run=^$ ./internal/engine/sql/store/
```

### Completion Report

```text
TASK 06 completed.
Files changed:
- internal/engine/sql/store/bench_test.go

Verification:
- go test ./...
- go build ./...
- go test -bench=BenchmarkGet -run=^$ ./internal/engine/sql/store/

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 07 — Add size-scaling Delete benchmarks

### Goal

Benchmark `Delete` at the same three fixed store sizes, mirroring
the Put benchmark's shift cost on removal, without rebuilding the
entire store on every iteration.

### Files

Modify:

```text
internal/engine/sql/store/bench_test.go
```

### Requirements

Add:

```go
func BenchmarkDelete(b *testing.B) {
    sizes := []int{1000, 10000, 100000}
    for _, n := range sizes {
        b.Run(fmt.Sprintf("size=%d", n), func(b *testing.B) {
            s := newPopulatedStore(n)
            deleteKey := key.EncodeUint64(uint64(n / 2))

            b.ResetTimer()
            for i := 0; i < b.N; i++ {
                _ = s.Delete(deleteKey)

                b.StopTimer()
                _ = s.Put(deleteKey, []byte("v"))
                b.StartTimer()
            }
        })
    }
}
```

Important:

```text
A naive Delete benchmark that rebuilds a fresh store of size n on
every iteration is impractical for n=100000, since Go's benchmark
runner may execute many thousands of iterations to get a stable
measurement, meaning a 100,000-entry store would be constructed
from scratch that many times. Even with setup excluded from the
measured time via b.StopTimer()/b.StartTimer(), the real wall-clock
cost of rebuilding the store repeatedly makes the benchmark run far
slower than necessary, and does not scale to larger sizes at all.

Instead, build the populated store once outside the loop. Each
iteration deletes the same fixed deleteKey, then immediately
restores it with Put before the next iteration, with the restore
step excluded from the timed measurement via
b.StopTimer()/b.StartTimer(). This keeps the store at a stable
size of exactly n entries for the entire benchmark run, measures
only the cost of the Delete call itself, and avoids the cost of
ever rebuilding the store more than once per size.

This mirrors the same Put/Delete (or Delete/Put) pairing pattern
already used in TASK 04 and TASK 05 to keep store size stable
without full rebuilds.
```

* delete a key from the middle of the populated range for a
  representative shift cost
* build the populated store exactly once per size, outside the
  `b.N` loop
* use `b.ResetTimer()` after the one-time setup, and
  `b.StopTimer()`/`b.StartTimer()` only around the restore Put
  inside the loop
* do not rebuild the store inside the `b.N` loop

### Verification

Run:

```bash
go test ./...
go build ./...
go test -bench=BenchmarkDelete -run=^$ ./internal/engine/sql/store/
```

Expected:

```text
go test ./... passes.
The benchmark completes in a practical amount of time at all three
sizes, including 100,000, since the store is built only once per
size rather than once per iteration.
```

### Completion Report

```text
TASK 07 completed.
Files changed:
- internal/engine/sql/store/bench_test.go

Verification:
- go test ./...
- go build ./...
- go test -bench=BenchmarkDelete -run=^$ ./internal/engine/sql/store/

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 08 — Add size-scaling Scan benchmarks

### Goal

Benchmark `Scan` at the same three fixed store sizes for a small,
fixed-size range, to confirm scan cost over a constant-size range
does not significantly grow with total store size.

### Files

Modify:

```text
internal/engine/sql/store/bench_test.go
```

### Requirements

Add:

```go
func BenchmarkScan(b *testing.B) {
    sizes := []int{1000, 10000, 100000}
    const scanWidth = 100

    for _, n := range sizes {
        b.Run(fmt.Sprintf("size=%d", n), func(b *testing.B) {
            s := newPopulatedStore(n)
            mid := n / 2
            start := key.EncodeUint64(uint64(mid))
            end := key.EncodeUint64(uint64(mid + scanWidth))
            b.ResetTimer()
            for i := 0; i < b.N; i++ {
                _, _ = s.Scan(start, end)
            }
        })
    }
}
```

* scan a fixed-width range of 100 entries from the middle of the
  populated range, regardless of total store size
* call `b.ResetTimer()` after setup

### Verification

Run:

```bash
go test ./...
go build ./...
go test -bench=BenchmarkScan -run=^$ ./internal/engine/sql/store/
```

### Completion Report

```text
TASK 08 completed.
Files changed:
- internal/engine/sql/store/bench_test.go

Verification:
- go test ./...
- go build ./...
- go test -bench=BenchmarkScan -run=^$ ./internal/engine/sql/store/

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 09 — Harden documentation

### Goal

Document the enterprise hardening additions in the existing store
documentation.

### Files

Modify:

```text
docs/sql_store.md
```

### Requirements

Add a new section documenting:

* the sorted-invariant stress test exists and what it proves
* the duplicate-key overwrite stress test exists and what it proves
* size-scaling benchmarks exist for Put, Get, Delete, and Scan
* Put and Delete are expected to show roughly linear (O(n)) growth
  with store size due to slice-shift insertion/removal
* Get and Scan over a fixed-width range are expected to remain
  roughly flat relative to store size
* this benchmark data is the intended signal for when on-disk
  storage (a future plan) becomes necessary, not a reason to
  optimize this in-memory store further
* this hardening did not change any existing public API or
  observable behavior

The documentation must include these exact strings because TASK 10
checks them:

```text
enterprise hardening
sorted-invariant
overwrite stress
size-scaling
O(n)
flat relative to store size
on-disk storage
no API changes
```

Do not document future behavior as already implemented.

Do not remove any existing required strings from the original
`kv_store_setup.md` documentation task. This is an addition, not a
replacement.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 09 completed.
Files changed:
- docs/sql_store.md

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 10 — Add documentation tests for hardening section

### Goal

Verify the new hardening documentation section exists and contains
required content.

### Files

Modify:

```text
internal/engine/sql/store/store_test.go
```

### Package

Keep:

```go
package store_test
```

### Requirements

Extend the existing documentation test (added in `kv_store_setup.md`
TASK 11) to also check for these additional strings:

```text
enterprise hardening
sorted-invariant
overwrite stress
size-scaling
O(n)
flat relative to store size
on-disk storage
no API changes
```

Do not remove any of the existing required string checks from the
original documentation test. Add to the same check, do not create
a second documentation test function.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 10 completed.
Files changed:
- internal/engine/sql/store/store_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 11 — Final hardening review

### Goal

Review the hardening work for correctness, completeness, scope
control, and project cleanliness.

### Files

Review only unless fixes are required:

```text
internal/engine/sql/store/store.go
internal/engine/sql/store/store_test.go
internal/engine/sql/store/bench_test.go
docs/sql_store.md
go.mod
go.sum
```

### Requirements

Confirm:

* `store.go` is unchanged; this plan only added tests and benchmarks
* no public API signature changed
* no existing observable behavior changed
* all tests from `kv_store_setup.md` still pass unmodified
* `TestStore_SortedInvariantUnderConcurrentChurn` exists and passes
  under `go test -race ./...`
* `TestStore_ConcurrentOverwriteSameKeys` exists and passes under
  `go test -race ./...`
* `TestStore_ConcurrentOverwriteSameKeys` uses a deterministic inner
  loop over the full key space in every goroutine, not random key
  selection, so all keys are guaranteed to be written and the test
  cannot fail intermittently due to coverage gaps
* `newPopulatedStore` helper exists in `bench_test.go` and is reused
  by all benchmarks, not duplicated
* `newPopulatedStoreFrom(start, n)` helper exists in `bench_test.go`
  alongside `newPopulatedStore(n)`
* `BenchmarkPut` exists with size=1000/10000/100000 sub-benchmarks
* `BenchmarkPut` uses a single fixed `endKey` paired with an
  immediate `Delete` each iteration, so store size never grows
  during the benchmark run
* `BenchmarkPut_WorstCaseFrontInsert` exists with the same sizes
* `BenchmarkPut_WorstCaseFrontInsert` uses `newPopulatedStoreFrom(1, n)`,
  not `newPopulatedStore(n)`, so key 0 is genuinely and permanently
  absent from the pre-populated data, making every iteration a true
  front-of-slice insertion rather than an overwrite after the first
  iteration
* `BenchmarkGet` exists with the same sizes
* `BenchmarkDelete` exists with the same sizes
* `BenchmarkDelete` builds the populated store exactly once per
  size outside the `b.N` loop, not on every iteration
* `BenchmarkDelete` restores the deleted key via `Put` inside the
  loop with the restore excluded from the timed measurement via
  `b.StopTimer()`/`b.StartTimer()`, keeping store size stable at
  exactly n without ever rebuilding the whole store
* `BenchmarkScan` exists with the same sizes and a fixed scan width
* all benchmarks compile and run under
  `go test -bench=. -run=^$ ./internal/engine/sql/store/`
* documentation includes the new hardening section
* documentation test covers the new hardening strings
* original documentation strings from `kv_store_setup.md` are still
  present and tested
* no fuzz testing was added for search/insertion logic, consistent
  with the design decision that it is not a trust boundary
* no iterator-style Scan was added
* no new public API surface was added
* no external dependencies were added
* zero internal imports beyond internal/engine/sql/key confirmed
* `go.mod` unchanged
* `go.sum` unchanged

If issues are found:

1. Fix them.
2. Run final verification again.
3. Report what was fixed.

### Final Verification

Run:

```bash
go test ./...
go build ./...
go test -race ./...
go test -bench=. -run=^$ ./internal/engine/sql/store/
go mod tidy
go test ./...
```

### Completion Report

```text
TASK 11 completed.
Files reviewed:
- internal/engine/sql/store/store.go
- internal/engine/sql/store/store_test.go
- internal/engine/sql/store/bench_test.go
- docs/sql_store.md
- go.mod
- go.sum

Final verification:
- go test ./...
- go build ./...
- go test -race ./...
- go test -bench=. -run=^$
- go mod tidy
- go test ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable

Final status:
- sql/store enterprise hardening complete
- sorted invariant proven under concurrent churn
- overwrite path proven safe under contention
- size-scaling benchmarks established for all four operations
- benchmark data available to inform future storage_setup.md timing
- no public API or behavior changes
- no non-goal systems introduced
```

---

## Final Expected Structure

After this plan:

```text
plomvix/
├── internal/
│   └── engine/
│       └── sql/
│           ├── key/
│           │   └── (existing, unchanged)
│           └── store/
│               ├── store.go
│               ├── store_test.go
│               └── bench_test.go
├── docs/
│   └── sql_store.md
```

No new folders are required.

---

## Completion Criteria

This plan is complete only when:

* `store.go` is unchanged from `kv_store_setup.md`
* `TestStore_SortedInvariantUnderConcurrentChurn` exists and passes
* `TestStore_ConcurrentOverwriteSameKeys` exists and passes, using
  deterministic full-key-space coverage in every goroutine rather
  than random key selection
* both stress tests pass under `go test -race ./...`
* `bench_test.go` exists with `newPopulatedStore` and
  `newPopulatedStoreFrom` shared helpers
* size-scaling benchmarks exist for Put (best and worst case), Get,
  Delete, and Scan at sizes 1000/10000/100000
* Put benchmarks keep store size stable across all iterations via
  Put/Delete pairing on a single fixed key, never an
  ever-growing or ever-shrinking key sequence
* worst-case front-insert benchmark uses `newPopulatedStoreFrom(1, n)`
  so the front key is genuinely absent on every iteration, not an
  overwrite of an already-populated key
* Delete benchmark builds its populated store exactly once per size,
  never inside the `b.N` loop, and restores the deleted key via a
  timer-excluded Put so the benchmark remains practical to run even
  at size=100000
* all benchmarks compile and run successfully
* documentation hardening section exists and is tested
* `go test ./...` passes
* `go build ./...` passes
* `go test -race ./...` passes
* `go mod tidy` produces no unwanted changes
* final `go test ./...` passes
* no non-goal systems introduced
* no public API or wire/behavior changes from `kv_store_setup.md`

---

## Recommended Next Step After Completion

After `kv_store_enterprise.md` is completed and verified, the
in-memory KV store foundation is complete and measured.

Do not automatically move to the next feature. Continue with the
selected database foundation roadmap as confirmed by the project
owner.

Possible next directions:

```text
storage_setup.md   — page-based file storage and persistence
                      (informed by the benchmark data from this plan)
```