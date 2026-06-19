# DDL Enterprise Hardening

| Field | Value |
| :--- | :--- |
| **Package** | `internal/systemids`, `internal/catalog`, `internal/engine/sql/system`, `internal/engine/sql`, `internal/engine/sql/vacuum` |
| **Plan** | `agent/ddl_enterprise.md` (Plan 27b) |

## Overview

Production-hardens the Plan 27a DDL layer with:
- Neutral system table ID constants (`systemids`)
- Decoupled storage abstraction (`SystemTable` interface)
- Standalone physical heap factory (`SystemHeapFactory`)
- SystemHeap adapter mapping KV to MVCC heap rows
- Thread-safe metadata cache with lock/recheck flow and deep-copy safety
- Vacuum Manager for background physical file deletion
- Transactional DDL cleanup

## Initialization Order

```
1. vacuum.NewManager(workers, queueSize) → vac.Start(ctx)
2. system.NewFactory(dataDir)
3. factory.OpenOrCreateSystemHeaps(ctx) → tables, columns, users
4. catalog.NewWithStores(tables, columns, users)
5. cat.Start(ctx) → bootstrap + cache load
6. sql.NewSQLEngine(cat, versions, tables, decoder, cache, txm, vacuum, log)
```

This order ensures no import cycles: `catalog` has zero imports of `engine/sql`.
The factory is the sole bridge between physical storage and catalog abstraction.

## Neutral System IDs (`internal/systemids`)

```go
SystemTableMinID = 1
SystemTableMaxID = 999
SystemTableTables  = 1  // _plomvix_tables
SystemTableColumns = 2  // _plomvix_columns
SystemTableUsers   = 3  // _plomvix_users
```

Both `catalog` and `vacuum` import this package. The Vacuum Manager strictly
rejects deletion of any table ID in `[1, 999]` with `ErrSystemTableDeletionForbidden`.

## SystemHeap Interface & Adapter

The `system` package defines a local `SystemHeap` interface to avoid depending
on concrete heap implementations:

```go
type SystemHeap interface {
    Insert(ctx, tx, row) error
    Scan(ctx, tx) (SystemHeapIterator, error)
}
```

`SystemHeapAdapter` wraps `SystemHeap` to implement `catalog.SystemTable`:
- **Put/Get/Delete**: maps KV operations to MVCC row inserts with an internal
  `atomic.Uint64` counter for monotonically increasing TxIDs.
- **Scan**: deduplicates by key (latest version wins), filters tombstones.
- **Delete**: appends a tombstone row (key + nil value).

The `SystemHeapFactory.OpenOrCreateSystemHeaps(ctx)` method:
1. Computes deterministic paths: `{dataDir}/heap_1.db`, `heap_2.db`, `heap_3.db`
2. Initializes Pager → KVStore → Heap for each
3. Wraps in `concreteSystemHeap` → `SystemHeapAdapter` → `catalog.SystemTable`

## Metadata Cache (Lock/Recheck Flow)

The Catalog cache provides thread-safe, deep-copy-safe metadata:

```
RLock (check SchemaVersion + map)
  → RUnlock
  → Disk I/O (if cache miss)
  → Lock (re-check SchemaVersion, clear map if bumped)
  → Insert
  → Unlock
  → Return Deep Copy
```

- `GetTable()` returns a deep copy with independent `[]byte` allocations.
- `DropTable()` invalidates the cache entry.
- `SchemaVersion` bumps on DDL mutations, triggering cache re-check.

## Vacuum Manager

Implements `lifecycle.Component` with a strict state machine:

| State | Meaning |
| :--- | :--- |
| `StateNew` | Constructed, not started |
| `StateStarted` | Workers running, accepting jobs |
| `StateStopped` | Workers drained, rejecting jobs |

Key properties:
- **Constructor validation**: `workers >= 1`, `queueSize >= 1`
- **Non-blocking enqueue**: `ScheduleDeletion()` does a select with `default`,
  returning `ErrVacuumQueueFull` if the channel is full.
- **System table protection**: rejects deletion of `TableID` in `[SystemTableMinID, SystemTableMaxID]`.
- **State enforcement**: `ErrVacuumNotStarted`, `ErrVacuumAlreadyStarted`, `ErrVacuumStopped`.
- **Drain()**: test-only polling loop (10ms intervals) for deterministic assertions.
- **Stop()**: closes `stopCh`, worker drains remaining items, waits on `workersWg`.

## Transactional DDL Cleanup

In `executeCreateTable`:
1. `AllocateTableID()` — reserve ID
2. `CreateTableHeap()` — returns `(TableHeap, heapPath, error)`
3. `RegisterTable()` — if this fails, `os.Remove(heapPath)` cleans up orphan
4. Return Result or error

In `executeDropTable`:
1. `GetTable()` — get TableID before dropping
2. `DropTable()` — remove from catalog
3. `vacuum.ScheduleDeletion(tableID, heapPath)` — non-blocking file deletion

## Deferred Features

- **ALTER TABLE**: explicitly deferred to Plan 29.
- **Singleflight cache stampede prevention**: cache guarantees eventual consistency
  and deep-copy safety, not read-amortization.
- **Physical file cleanup for system tables**: system table files are never deleted.
