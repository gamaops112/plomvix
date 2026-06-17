# kv_store_setup.md

# Plomvix In-Memory KV Store Plan

## Purpose

Create the first in-memory key-value store for Plomvix, built on top
of the existing `internal/engine/sql/key` package.

This plan introduces `internal/engine/sql/store` — an ordered,
in-memory store that supports Put, Get, Delete, and ordered range
Scan, using `key.Key` for ordering and raw `[]byte` for values.

This is database foundation work only. It is an in-memory stepping
stone, not a persistent storage engine.

Do not add WAL.
Do not add disk persistence.
Do not add storage pages.
Do not add a buffer pool.
Do not add query execution.
Do not add transactions or multi-key atomicity.
Do not add API server.
Do not add UI.
Do not wire the store into lifecycle or runtime.

---

## Feature Name

```text
In-Memory KV Store
```

Plan file:

```text
kv_store_setup.md
```

New package:

```text
internal/engine/sql/store
```

---

## Required Starting State

This plan starts only after `sql_key_enterprise.md` is completed and
verified.

Before starting this plan, the project must already have:

```text
internal/engine/sql/key/key.go
internal/engine/sql/key/key_test.go
internal/engine/sql/key/key_internal_test.go
internal/engine/sql/key/fuzz_test.go
internal/engine/sql/key/bench_test.go
docs/sql_key.md
```

The key package must already expose:

```go
type Kind uint8
type Field struct {
    Kind   Kind
    Offset int
    Length int
}
type Key struct {
    // unexported fields only
}

func EncodeUint64(v uint64) Key
func EncodeInt64(v int64) Key
func EncodeString(s string) (Key, error)
func EncodeBytes(b []byte) Key
func EncodeSortComposite(fields ...any) (Key, error)
func EncodeStorageComposite(fields ...any) (Key, error)

func DecodeUint64(k Key) (uint64, error)
func DecodeInt64(k Key) (int64, error)
func DecodeString(k Key) (string, error)
func DecodeBytes(k Key) ([]byte, error)
func DecodeSortComposite(k Key) ([]any, error)
func DecodeStorageComposite(k Key) ([]any, error)

func ParseKey(data []byte, kinds []Kind) (Key, error)
func ParseStorageCompositeKey(data []byte, kinds []Kind) (Key, error)

func (k Key) Bytes() []byte
func (k Key) Compare(other Key) int
func (k Key) Fields() []Field
```

If this starting state is not true, stop and report that
`sql_key_enterprise.md` is incomplete.

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
```

Current stage:

```text
first storage-adjacent foundation feature
```

Current feature area:

```text
in-memory KV store
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
* Do not import internal/config, internal/logger, internal/lifecycle,
  or internal/runtime from the store package.
* The store package may import internal/engine/sql/key.
* Do not add external dependencies.
* Use only Go standard library.
* Keep tests deterministic.
* Use table-driven tests where useful.
* Do not create a root-level `tests/` directory.
* Do not add WAL, disk persistence, or storage pages in this plan.
* Do not add transactions or multi-key atomic batches in this plan.

---

## Dependency Direction Rules

Allowed imports inside `internal/engine/sql/store/store.go`:

```text
internal/engine/sql/key
sort
sync
errors
fmt
```

`bytes` is not used in `store.go` in this plan. It may be used in
`store_test.go` only, for comparing byte slice values in test
assertions. Do not import `bytes` in `store.go` unless it is
actually used there.

Forbidden imports:

```text
internal/config
internal/logger
internal/lifecycle
internal/runtime
```

Reason:

```text
The store is a database-layer component. It depends on the key
package for ordering, but must not depend on the application
composition layer. This keeps the store reusable and testable in
isolation, the same way internal/engine/sql/key has zero internal
imports beyond its own design.
```

Future packages that may import this store:

```text
internal/engine/sql/table   (not in this plan)
internal/engine/sql/index   (not in this plan)
```

---

## Design Decisions

### Underlying Data Structure

The store uses a sorted slice of entries, ordered by `key.Key.Compare()`,
with `sort.Search` for lookups and insertion position.

```go
type entry struct {
    key   key.Key
    value []byte
}

type Store struct {
    mu      sync.RWMutex
    entries []entry
}
```

Reason:

```text
A sorted slice with binary search gives correct ordering and range
scans without the complexity of a skip-list or balanced tree. This
is an in-memory stepping stone toward a future on-disk storage
engine (B-tree or LSM), not the final engine. Insertion is O(n) due
to slice shifting, which is an acceptable tradeoff at this stage.
A future storage_setup.md plan replaces this entirely once disk
persistence is needed.
```

### Concurrency Model

The store uses a single `sync.RWMutex` protecting the entire
`entries` slice.

```text
Get and Scan acquire a read lock (RLock).
Put and Delete acquire a write lock (Lock).
```

Reason:

```text
There is no benchmark or workload yet that demonstrates lock
contention. A single mutex is simple, provably correct, and matches
the "basic first, harden later" discipline used throughout this
project. Sharded locking is a future hardening concern, not a
day-one requirement.
```

### Value Type

Values are raw `[]byte`. The store has no knowledge of value
structure or encoding.

```go
func (s *Store) Put(k key.Key, value []byte) error
func (s *Store) Get(k key.Key) ([]byte, error)
```

Reason:

```text
This mirrors how production KV stores (LevelDB, RocksDB, Pebble,
BadgerDB) separate ordering/storage concerns from value encoding.
Callers (future table/index packages) decide what a value
represents. The store never interprets value bytes.
```

Stored values are copied on Put and copied on return from Get, so
callers cannot mutate store internals through returned slices.

### Scan Semantics

```go
func (s *Store) Scan(start, end key.Key) ([]Entry, error)
```

Scan returns all entries `e` such that:

```text
start.Compare(e.Key) <= 0 AND e.Key.Compare(end) < 0
```

This is a half-open interval `[start, end)`.

Reason:

```text
Half-open ranges compose cleanly: [a, b) followed by [b, c) covers
[a, c) with no gap and no overlap. This is the same convention used
by LevelDB, RocksDB, and Pebble. Prefix scanning can be built later
on top of this primitive by computing an appropriate end key.
```

If `start.Compare(end) >= 0`, `Scan` returns an empty result and no
error. This is not an error condition; it simply describes an empty
range.

This package intentionally has no scan-range-specific error type.
An invalid or empty scan range is not an error condition in this
API; it always produces an empty result with a nil error. Do not
add an error sentinel for scan range validation in this plan.

### Public Entry Type

Scan results are returned as a slice of a public `Entry` type so
callers do not need access to internal store structures:

```go
type Entry struct {
    Key   key.Key
    Value []byte
}
```

`Entry.Value` returned from `Scan` is a copy, not a reference into
store internals.

### Key Uniqueness

`Put` overwrites the value if the key already exists. There is no
separate "insert if not exists" operation in this plan.

`Delete` on a non-existent key is not an error; it is a no-op that
returns nil.

`Get` on a non-existent key returns `ErrNotFound`.

---

## Final Public API

Package:

```text
internal/engine/sql/store
```

Types:

```go
type Entry struct {
    Key   key.Key
    Value []byte
}

type Store struct {
    // unexported fields only
}
```

Functions and methods:

```go
func New() *Store

func (s *Store) Put(k key.Key, value []byte) error
func (s *Store) Get(k key.Key) ([]byte, error)
func (s *Store) Delete(k key.Key) error
func (s *Store) Scan(start, end key.Key) ([]Entry, error)
func (s *Store) Len() int
```

Public errors:

```go
var (
    ErrNotFound = errors.New("sql/store: key not found")
    ErrNilStore = errors.New("sql/store: nil store")
)
```

---

## Non-Goals

Do not implement:

* WAL
* disk persistence
* storage pages
* buffer pool
* transactions
* multi-key atomic batches
* compaction
* snapshots
* iterators with cursor state
* concurrent-safe lock-free structures
* sharded locking
* query execution
* indexes beyond the store's own ordering
* table heap
* schema catalog
* config, logger, lifecycle, or runtime integration
* external dependencies

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
│               └── store_test.go
├── docs/
│   └── sql_store.md
```

No other new folders are required.

---

## Task Plan

---

## TASK 01 — Create store package skeleton and types

### Goal

Create `internal/engine/sql/store` with all public types and error
sentinels, with method stubs.

### Files

Create:

```text
internal/engine/sql/store/store.go
```

### Requirements

Add package declaration:

```go
package store
```

Add imports:

```go
errors

"github.com/plomvix/plomvix/internal/engine/sql/key"
```

Add public `Entry` type:

```go
type Entry struct {
    Key   key.Key
    Value []byte
}
```

Add `entry` internal type and `Store` struct:

```go
type entry struct {
    key   key.Key
    value []byte
}

type Store struct {
    mu      sync.RWMutex
    entries []entry
}
```

Add `sync` to imports.

Add public errors:

```go
var (
    ErrNotFound = errors.New("sql/store: key not found")
    ErrNilStore = errors.New("sql/store: nil store")
)
```

Add constructor:

```go
func New() *Store {
    return &Store{}
}
```

Add method stubs that compile and return harmless placeholder
values. These stubs do not yet implement real behavior, including
nil-receiver handling, which is added in later tasks (TASK 03 for
Put/Get, TASK 05 for Delete, TASK 07 for Scan):

```go
func (s *Store) Put(k key.Key, value []byte) error { return nil }
func (s *Store) Get(k key.Key) ([]byte, error) { return nil, ErrNotFound }
func (s *Store) Delete(k key.Key) error { return nil }
func (s *Store) Scan(start, end key.Key) ([]Entry, error) { return nil, nil }
func (s *Store) Len() int { return 0 }
```

Do not implement real behavior, including nil-store checks, in
this task. Nil-receiver handling for each method is added when
that method's real implementation is built in a later task.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 01 completed.
Files changed:
- internal/engine/sql/store/store.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 02 — Add type and error sentinel tests

### Goal

Verify error sentinel values are stable and `New()` returns a
usable store.

### Files

Create:

```text
internal/engine/sql/store/store_test.go
```

### Package

```go
package store_test
```

### Requirements

Add tests confirming error sentinel strings:

```text
ErrNotFound.Error() == "sql/store: key not found"
ErrNilStore.Error() == "sql/store: nil store"
```

Add test comment:

```go
// These values are part of the stable sql/store API.
```

Add test confirming `New()` returns a non-nil `*Store`.

Add test confirming a new store's `Len()` is 0.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 02 completed.
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

## TASK 03 — Implement Put and Get

### Goal

Implement `Put` and `Get` using a sorted slice with `sort.Search`.

### Files

Modify:

```text
internal/engine/sql/store/store.go
```

### Requirements

Add `sort` to imports.

Add an unexported helper to find the insertion/lookup index:

```go
func (s *Store) search(k key.Key) (index int, found bool)
```

Behavior:

* use `sort.Search` over `s.entries` comparing each entry's key to
  `k` via `entry.key.Compare(k)`
* `sort.Search` requires a monotonic predicate; use:

```go
idx := sort.Search(len(s.entries), func(i int) bool {
    return s.entries[i].key.Compare(k) >= 0
})
```

* this returns the first index where the stored key is >= k
* if `idx < len(s.entries)` and `s.entries[idx].key.Compare(k) == 0`,
  the key exists at `idx`; otherwise `idx` is the correct insertion
  point for a new entry

Implement `Put(k key.Key, value []byte) error`:

* if `s == nil`: return `ErrNilStore`
* acquire write lock
* find index using `search`
* copy `value` into a new slice before storing, so the caller's
  slice cannot later mutate stored data:

```go
stored := make([]byte, len(value))
copy(stored, value)
```

* if found: overwrite `s.entries[idx].value` with `stored`
* if not found: insert a new entry at `idx`, shifting subsequent
  entries

Insertion at index pattern:

```go
s.entries = append(s.entries, entry{})
copy(s.entries[idx+1:], s.entries[idx:])
s.entries[idx] = entry{key: k, value: stored}
```

* release write lock
* return nil

Implement `Get(k key.Key) ([]byte, error)`:

* if `s == nil`: return nil, `ErrNilStore`
* acquire read lock
* find index using `search`
* if not found: release read lock, return nil, `ErrNotFound`
* if found: copy the stored value into a new slice before returning,
  so the caller cannot mutate store internals through the returned
  slice
* release read lock
* return the copy, nil

Rules:

* `Put` and `Get` must always copy bytes at the store boundary,
  never share the caller's or store's underlying array directly
* `search` does not acquire any lock itself; callers of `search`
  must already hold the appropriate lock

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
- internal/engine/sql/store/store.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 04 — Add Put and Get tests

### Goal

Verify Put and Get behavior, including overwrite, copy-safety, and
not-found cases.

### Files

Modify:

```text
internal/engine/sql/store/store_test.go
```

### Requirements

Use table-driven tests.

Test basic Put/Get round-trip:

```text
Put(EncodeUint64(1), []byte("a"))
Get(EncodeUint64(1)) == []byte("a"), nil
```

Test Get on missing key:

```text
Get(EncodeUint64(999)) == nil, ErrNotFound
```

Test Put overwrites existing key:

```text
Put(EncodeUint64(1), []byte("a"))
Put(EncodeUint64(1), []byte("b"))
Get(EncodeUint64(1)) == []byte("b"), nil
Len() == 1 (not 2)
```

Test copy-safety on Put:

```go
val := []byte("original")
s.Put(key.EncodeUint64(1), val)
val[0] = 'X'
got, _ := s.Get(key.EncodeUint64(1))
// got must still be "original", not "Xriginal"
```

Test copy-safety on Get:

```go
s.Put(key.EncodeUint64(1), []byte("original"))
got, _ := s.Get(key.EncodeUint64(1))
got[0] = 'X'
got2, _ := s.Get(key.EncodeUint64(1))
// got2 must still be "original", not "Xriginal"
```

Test nil store safety:

```text
var s *store.Store
_, err := s.Get(key.EncodeUint64(1))
// err must match ErrNilStore

err2 := s.Put(key.EncodeUint64(1), []byte("x"))
// err2 must match ErrNilStore
```

Test multiple Put calls maintain correct Len:

```text
Put three distinct keys, Len() == 3
Put a fourth key equal to one already present, Len() remains 3
```

Use `errors.Is` for all error checks.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 04 completed.
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

## TASK 05 — Implement Delete and Len

### Goal

Implement `Delete` and `Len`.

### Files

Modify:

```text
internal/engine/sql/store/store.go
```

### Requirements

Implement `Delete(k key.Key) error`:

* if `s == nil`: return `ErrNilStore`
* acquire write lock
* find index using `search`
* if not found: release write lock, return nil (no-op, not an error)
* if found: remove the entry at that index, shifting subsequent
  entries down

Removal pattern:

```go
s.entries = append(s.entries[:idx], s.entries[idx+1:]...)
```

* release write lock
* return nil

Implement `Len() int`:

* if `s == nil`: return 0
* acquire read lock
* return `len(s.entries)`
* release read lock

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 05 completed.
Files changed:
- internal/engine/sql/store/store.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 06 — Add Delete and Len tests

### Goal

Verify Delete and Len behavior, including no-op delete and nil
store safety.

### Files

Modify:

```text
internal/engine/sql/store/store_test.go
```

### Requirements

Test basic Delete:

```text
Put(EncodeUint64(1), []byte("a"))
Delete(EncodeUint64(1))
Get(EncodeUint64(1)) == nil, ErrNotFound
Len() == 0
```

Test Delete on missing key is a no-op, not an error:

```text
Delete(EncodeUint64(999)) == nil
Len() unchanged
```

Test Delete removes only the matching key:

```text
Put three distinct keys
Delete the middle one
Len() == 2
Get on the two remaining keys still succeeds
Get on the deleted key returns ErrNotFound
```

Test nil store safety:

```text
var s *store.Store
err := s.Delete(key.EncodeUint64(1))
// err must match ErrNilStore, not nil
// Delete on a nil store is a nil-receiver safety case, distinct
// from Delete on a non-nil store with a missing key, which is a
// no-op returning nil

s.Len() == 0
```

Important distinction:

```text
Delete on a nil *Store returns ErrNilStore.
Delete on a non-nil *Store with a key that does not exist returns
nil (no-op). These are two different cases and must not be
confused: nil-receiver safety versus missing-key no-op.
```

Use `errors.Is` for all error checks.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 06 completed.
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

## TASK 07 — Implement Scan

### Goal

Implement `Scan` with half-open range semantics `[start, end)`.

### Files

Modify:

```text
internal/engine/sql/store/store.go
```

### Requirements

Implement `Scan(start, end key.Key) ([]Entry, error)`:

* if `s == nil`: return nil, `ErrNilStore`
* if `start.Compare(end) >= 0`: return an empty non-nil `[]Entry{}`
  and nil error (this is an empty range, not an error)
* acquire read lock
* find the start index using `sort.Search` for the first entry with
  key >= start
* iterate from that index while the entry's key is < end:

```go
startIdx := sort.Search(len(s.entries), func(i int) bool {
    return s.entries[i].key.Compare(start) >= 0
})

var result []Entry
for i := startIdx; i < len(s.entries); i++ {
    if s.entries[i].key.Compare(end) >= 0 {
        break
    }
    valueCopy := make([]byte, len(s.entries[i].value))
    copy(valueCopy, s.entries[i].value)
    result = append(result, Entry{
        Key:   s.entries[i].key,
        Value: valueCopy,
    })
}
```

* release read lock
* return `result, nil`

Rules:

* `Scan` must copy each returned value, never share the store's
  underlying byte slices with the caller
* `key.Key` itself is already safe to share by value since its own
  `Bytes()`/`Fields()` methods return copies; the `Key` struct
  fields are unexported and the struct is small, so returning it by
  value in `Entry.Key` does not expose internal mutability
* an empty range or a range with no matching entries returns a
  non-nil empty slice, not nil, so callers can safely call `len()`
  on the result without a nil check

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 07 completed.
Files changed:
- internal/engine/sql/store/store.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 08 — Add Scan tests

### Goal

Verify Scan range semantics, ordering, copy-safety, and edge cases.

### Files

Modify:

```text
internal/engine/sql/store/store_test.go
```

### Requirements

Use table-driven tests.

Setup for scan tests: Put keys for uint64 values 0 through 9.

Test basic half-open range:

```text
Scan(EncodeUint64(2), EncodeUint64(5))
returns entries for keys 2, 3, 4 — NOT 5
```

Test range covering everything:

```text
Scan(EncodeUint64(0), EncodeUint64(10))
returns all 10 entries in ascending order
```

Test empty range when start == end:

```text
Scan(EncodeUint64(3), EncodeUint64(3))
returns empty slice, nil error
```

Test empty range when start > end:

```text
Scan(EncodeUint64(5), EncodeUint64(2))
returns empty slice, nil error
```

Test range with no matching entries:

```text
Scan(EncodeUint64(100), EncodeUint64(200))
returns empty slice, nil error
```

Test result ordering is ascending:

```text
verify Scan results are returned in ascending key order
```

Test copy-safety of Scan results:

```go
results, _ := s.Scan(key.EncodeUint64(0), key.EncodeUint64(10))
results[0].Value[0] = 'X'
results2, _ := s.Scan(key.EncodeUint64(0), key.EncodeUint64(10))
// results2[0].Value must be unchanged
```

Test nil store safety:

```text
var s *store.Store
_, err := s.Scan(key.EncodeUint64(0), key.EncodeUint64(10))
// err must match ErrNilStore
```

Test adjacent half-open ranges cover without overlap:

```go
first, _ := s.Scan(key.EncodeUint64(0), key.EncodeUint64(5))
second, _ := s.Scan(key.EncodeUint64(5), key.EncodeUint64(10))
// len(first) + len(second) must equal total entries in [0,10)
// no key should appear in both first and second
```

Use `errors.Is` for all error checks.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 08 completed.
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

## TASK 09 — Add concurrency safety tests

### Goal

Verify the store is safe for concurrent use under `go test -race`.

### Files

Modify:

```text
internal/engine/sql/store/store_test.go
```

### Requirements

Add a test using multiple goroutines performing concurrent
operations on the same store:

```go
func TestStore_ConcurrentAccess(t *testing.T) {
    s := store.New()
    var wg sync.WaitGroup

    for i := 0; i < 50; i++ {
        wg.Add(1)
        go func(n int) {
            defer wg.Done()
            k := key.EncodeUint64(uint64(n))
            _ = s.Put(k, []byte("value"))
            _, _ = s.Get(k)
            _, _ = s.Scan(key.EncodeUint64(0), key.EncodeUint64(100))
        }(i)
    }

    wg.Wait()

    if s.Len() > 50 {
        t.Fatalf("unexpected length: %d", s.Len())
    }
}
```

Add `sync` to test imports.

Rules:

* this test must pass under `go test -race ./...`
* do not use sleeps as synchronization
* use `sync.WaitGroup` to wait for all goroutines to finish
* the test does not need to assert exact ordering of concurrent
  writes, only that no race is detected and the store remains in a
  consistent, queryable state afterward

### Verification

Run:

```bash
go test ./...
go build ./...
go test -race ./...
```

### Completion Report

```text
TASK 09 completed.
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

## TASK 10 — Add store documentation

### Goal

Document the in-memory KV store package.

### Files

Create:

```text
docs/sql_store.md
```

### Requirements

Create documentation with heading:

```text
# Plomvix SQL Store
```

Document:

* purpose: in-memory ordered KV store built on internal/engine/sql/key
* sorted slice with sort.Search as the underlying structure
* single sync.RWMutex concurrency model
* raw []byte values, store is opaque to value structure
* Put, Get, Delete, Scan, Len API
* half-open Scan range semantics [start, end)
* copy-safety guarantees at all API boundaries
* this is an in-memory stepping stone, not a persistent engine
* no transactions, no WAL, no disk persistence in this plan
* non-goals

The documentation must include these exact strings because TASK 11
checks them:

```text
# Plomvix SQL Store
sql/store
sorted slice
sort.Search
sync.RWMutex
half-open
[start, end)
copy-safety
in-memory
no transactions
```

Non-goals section must include:

```text
WAL
disk persistence
storage pages
buffer pool
transactions
compaction
snapshots
sharded locking
```

Do not document future behavior as already implemented.

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
- docs/sql_store.md

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 11 — Add store documentation tests

### Goal

Verify documentation exists and covers required content.

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

Add a documentation test that reads:

```go
os.ReadFile("../../../../docs/sql_store.md")
```

Path note:

```text
This path assumes the test runs from internal/engine/sql/store/,
which is the default behavior of go test ./...
```

Test that the document contains these exact strings:

```text
# Plomvix SQL Store
sql/store
sorted slice
sort.Search
sync.RWMutex
half-open
[start, end)
copy-safety
in-memory
no transactions
WAL
disk persistence
storage pages
buffer pool
transactions
compaction
snapshots
sharded locking
```

Use stable substring checks.

Do not make fragile checks for full paragraphs.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 11 completed.
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

## TASK 12 — Final review

### Goal

Review the store package for correctness, completeness, scope
control, and project cleanliness.

### Files

Review only unless fixes are required:

```text
internal/engine/sql/store/store.go
internal/engine/sql/store/store_test.go
docs/sql_store.md
go.mod
go.sum
```

### Requirements

Confirm:

* package is `internal/engine/sql/store`
* store imports only `internal/engine/sql/key` plus standard library
* no config/logger/lifecycle/runtime imports exist
* `Store` struct fields are unexported
* `New()` returns a non-nil usable store
* `Put` copies the value before storing
* `Get` copies the value before returning
* `Put` overwrites existing keys without growing Len
* `Delete` on a missing key is a no-op, not an error
* `Delete` correctly removes only the matching entry
* `Scan` uses half-open `[start, end)` semantics
* `Scan` with `start >= end` returns empty slice, not an error
* `Scan` results are copy-safe
* `Scan` results are returned in ascending order
* nil store receiver is handled safely on every method
  (Get/Put/Delete/Scan return ErrNilStore, Len returns 0)
* concurrency test passes under `go test -race ./...`
* `search` helper does not acquire its own lock
* error sentinels are stable and tested
* only ErrNotFound and ErrNilStore exist; no unused scan-range
  error sentinel was added
* documentation exists and is tested
* no transactions were added
* no WAL was added
* no disk persistence was added
* no external dependencies were added
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
go mod tidy
go test ./...
```

### Completion Report

```text
TASK 12 completed.
Files reviewed:
- internal/engine/sql/store/store.go
- internal/engine/sql/store/store_test.go
- docs/sql_store.md
- go.mod
- go.sum

Final verification:
- go test ./...
- go build ./...
- go test -race ./...
- go mod tidy
- go test ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable

Final status:
- in-memory KV store complete
- Put/Get/Delete/Scan implemented and tested
- copy-safety verified at all API boundaries
- concurrency-safe under single RWMutex
- no non-goal systems introduced
```

---

## Completion Criteria

This plan is complete only when:

* `internal/engine/sql/store/store.go` exists
* `internal/engine/sql/store/store_test.go` exists
* `docs/sql_store.md` exists
* Put/Get/Delete/Scan/Len are all implemented and tested
* copy-safety is verified for Put, Get, and Scan
* Delete is idempotent and safe on missing keys
* Scan uses half-open range semantics and is tested for adjacency,
  emptiness, and ordering
* nil store receiver is safe on every method
* concurrency test passes under `go test -race ./...`
* documentation exists and is tested
* store imports only sql/key plus standard library
* no external dependencies added
* `go test ./...` passes
* `go build ./...` passes
* `go test -race ./...` passes
* `go mod tidy` produces no unwanted changes
* final `go test ./...` passes
* no non-goal systems introduced

---

## Recommended Next Step After Completion

After `kv_store_setup.md` is completed and verified, the in-memory
KV store foundation is in place.

Do not automatically move to the next feature. Continue with the
selected database foundation roadmap as confirmed by the project
owner.

Possible next directions:

```text
kv_store_enterprise.md   — hardening, fuzzing, benchmarks for the store
storage_setup.md         — page-based file storage and persistence
```