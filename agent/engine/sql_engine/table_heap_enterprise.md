table_heap_enterprise.md — Table Heap (Enterprise Hardening)
Scope
This plan hardens the Table Heap (`heap` package) delivered in 
`table_heap_setup.md`. It introduces Multi-Version Concurrency Control (MVCC) 
via append-only versioning, NULL value support via a null-bitmask prefix, 
and a manual `Vacuum` API for garbage-collecting unreachable row versions.

This plan explicitly defers true O(1) memory streaming scans (which require 
a KVStore Cursor API), background garbage-collection workers, and active 
transaction tracking. It relies entirely on the Basic KVStore's slice-returning 
`Scan` and the `key` package's existing `version` slot in `EncodeTableRowKey`.

Contract this tier honestly provides (read this before writing code)
- Strictly Monotonic MVCC: For a given table row PK, each write operation 
  must use a strictly increasing `tx.ID` (> 0). If a version with `>= tx.ID` 
  already exists for that PK, the operation returns `ErrTxConflict`. This 
  enforces append-only monotonic writes per PK and prevents time-travel 
  inconsistencies or silent overwrites.
- Error Priority: `ErrTxConflict` is always evaluated before `ErrDuplicateKey`. 
  If a write attempts to use an existing or future version, it fails with 
  `ErrTxConflict` immediately.
- Append-Only MVCC: Updates and Deletes do not overwrite existing KV entries. 
  They append new entries with higher `version` numbers. Reads filter for 
  the highest version visible to the current transaction (`<= tx.ID`).
- Tombstones: Deleting a row writes a special "tombstone" marker as the 
  value for the new version, hiding the row from future reads.
- NULL Support: Row values are prefixed with a variable-length null-bitmask 
  before being passed to `key.EncodeStorageComposite`. All columns are 
  nullable by default in Enterprise.
- No PK Updates: Updating the Primary Key columns of an existing row is 
  forbidden and returns `ErrPrimaryKeyUpdate`.
- Table-Level Locking: A `sync.RWMutex` on the `table` struct serializes 
  read-before-write sequences (`Insert`, `Update`, `Delete`, `Vacuum`) to 
  prevent MVCC race conditions that the KVStore's global lock cannot cover.
- Manual Vacuum: `Vacuum(ctx, olderThan)` reclaims space by deleting old, 
  unreachable versions of rows. 
  *CRITICAL SAFETY CONTRACT:* This tier does not track active transactions. 
  `Vacuum` is unsafe if active readers exist with `Tx.ID <= olderThan`. The 
  caller MUST guarantee that `olderThan` is strictly lower than the oldest 
  active reader's `Tx.ID`. Vacuum may delete the exact version needed to 
  answer historical reads `<= olderThan`; this is safe ONLY under the 
  caller's guarantee.
- Honest Memory Footprint: `Scan` buffers all versions of a table into memory 
  before filtering MVCC visibility. True O(1) streaming is deferred.

Constants & Limits
package heap

const (
    FlagTombstone byte = 0x01 // If set in RowFlags, the row is deleted.
    FlagNormal    byte = 0x00
)

Public API Additions
package heap

// Tx represents a transaction context for MVCC visibility.
type Tx struct {
    ID uint64 // Must be > 0. ID 0 is reserved for Basic-tier non-MVCC rows.
}

// Extended Table interface
type Table interface {
    Insert(ctx context.Context, tx Tx, values []any) error
    Update(ctx context.Context, tx Tx, pkValues []any, newValues []any) error
    Get(ctx context.Context, tx Tx, pkValues []any) ([]any, error)
    Delete(ctx context.Context, tx Tx, pkValues []any) error
    Scan(ctx context.Context, tx Tx) (Rows, error)

    // Vacuum reclaims space. Caller MUST ensure olderThan < oldest active reader Tx.ID.
    Vacuum(ctx context.Context, olderThan uint64) error
}

var (
    // ... Basic errors ...
    ErrTxConflict      = errors.New("heap: transaction version conflict (non-monotonic Tx.ID)")
    ErrInvalidTx       = errors.New("heap: invalid transaction ID (must be > 0)")
    ErrPrimaryKeyUpdate = errors.New("heap: updating primary key columns is not supported")
)

Tasks (do in order, one at a time)

Task 1 — API updates, Tx definition, Table Locking, and Value Layout
Update `internal/engine/sql/heap/heap.go`. Define `Tx`, `FlagTombstone`, 
`ErrTxConflict`, `ErrInvalidTx`, `ErrPrimaryKeyUpdate`, and the extended `Table` interface. 
Add a `mu sync.RWMutex` to the concrete `*table` struct.
Define the uniform Enterprise Row Value Layout:
  [1-byte RowFlags][N-byte NullBitmask][key.EncodeStorageComposite payload]
- RowFlags: 0x00 = normal, 0x01 = tombstone.
- NullBitmask: 1 bit per column. Length is ceil(len(Columns) / 8) bytes.
- Tombstone Layout: [0x01][zero-filled NullBitmask][empty payload]. 
  (Uniform length simplifies decoder validation).
Tests: Verify interface compliance.

Task 2 — Null-Bitmask & Tombstone Encoding/Decoding
Update `internal/engine/sql/heap/encode.go`.
Implement `encodeEnterpriseValue(schema Schema, values []any) ([]byte, error)`:
- Calculate null bitmask. For each column: if value is nil, set null bit 
  and DO NOT call `anyToPrimitive`. Otherwise, call `anyToPrimitive`.
- Encode composite payload using only non-null primitives.
- Prepend RowFlags (0x00) and NullBitmask.
Implement `decodeEnterpriseValue(schema Schema, data []byte) (values []any, isTombstone bool, err error)`:
- Read RowFlags. If flag is not `0x00` or `0x01`, return `ErrTreeCorrupt` (or equivalent corruption error).
- Read NullBitmask (always present, length derived from schema).
- If tombstone (RowFlags == 0x01), return nil values and true.
- Parse composite payload.
- Reconstruct `[]any`, inserting `nil` for NULL bits.
Tests: Round-trip rows with various NULL patterns. Verify uniform tombstone encoding. Verify unknown flags are rejected.

Task 3 — PK Prefix Helpers & Monotonic Tx Validation
*Prerequisite:* Ensure the `key` package provides a helper to encode a table row prefix without the version slot (e.g., `key.EncodeTableRowPrefix(tableID, pkValues)`). If it does not, add it to the `key` package first.

Add to `encode.go`:
Implement `encodeRowVersionPrefix(schema Schema, pkValues []any) (start, end key.Key, error)`:
- Uses the prerequisite `key.EncodeTableRowPrefix` to encode TableID + PK columns.
- Returns `start` (the exact prefix) and `end` (using `key.PrefixEnd`).

Implement `validateMonotonicTx(ctx context.Context, store kv.KVStore, schema Schema, pkValues []any, txID uint64) error`:
- If `txID == 0`, return `ErrInvalidTx`.
- Scans the KVStore using `encodeRowVersionPrefix`.
- Decodes the version from each returned key.
- If ANY existing version `>= txID` is found, return `ErrTxConflict`.
Tests: Insert at tx=1, Insert at tx=1 -> ErrTxConflict. Insert at tx=10, Insert at tx=5 -> ErrTxConflict. Write with tx=0 -> ErrInvalidTx.

Task 4 — MVCC `Insert` and `Update`
Implement `Insert` and `Update` on `*table`.
Both methods MUST acquire `t.mu.Lock()` (write lock) at the start and defer unlock.
1. Call `validateMonotonicTx`. (Note: This guarantees `ErrTxConflict` takes priority over `ErrDuplicateKey` for same-version inserts).
2. `Insert`: Scan for visible version `< tx.ID`. If found and not tombstone, return `ErrDuplicateKey`.
3. `Update`: 
   - Scan for highest visible version `< tx.ID`. If none or tombstone, return `ErrKeyNotFound`.
   - Extract PK columns from `newValues` using `schema.PKIndices`. Compare them to `pkValues`. If they differ, return `ErrPrimaryKeyUpdate`.
4. Encode new value and `kv.Set` with `tx.ID` as version.
Tests: Standard Insert/Update. Verify ErrTxConflict on out-of-order Tx IDs. Verify ErrPrimaryKeyUpdate when attempting to change a PK column.

Task 5 — MVCC `Delete` (Tombstones)
Implement `Delete` on `*table`.
Acquire `t.mu.Lock()`.
1. `validateMonotonicTx`.
2. Scan for visible version `< tx.ID`. If none or tombstone, return `ErrKeyNotFound`.
3. Encode uniform tombstone value.
4. `kv.Set` with `tx.ID` as version.
Tests: Delete visible. Delete missing. Delete already-deleted. Verify ErrTxConflict.

Task 6 — MVCC `Get` and `Scan` (Visibility Filtering)
Implement `Get` and `Scan` on `*table`.
Both methods MUST acquire `t.mu.RLock()` (read lock).
`Get`:
1. Scan KVStore for PK prefix.
2. Iterate backwards (highest version first).
3. Return first version where `version <= tx.ID` and not tombstone.

`Scan`:
1. Call `kv.Scan` for the whole table prefix.
2. Group entries by PK (using `key.DecodeTableRowKey` to extract PK and ignore version).
3. For each PK group, find highest version `<= tx.ID`.
4. If tombstone, discard. Otherwise decode and buffer.
5. Sort buffered visible rows by PK ascending.
6. Return `Rows` iterator.
Tests: Read your own writes. Isolation. Scan filters tombstones. Scan output is strictly PK-sorted.

Task 7 — `Vacuum` Implementation
Implement `Vacuum(ctx, olderThan uint64)`.
Acquire `t.mu.Lock()`.
1. Scan entire table prefix. Group by PK.
2. For each PK group:
   - Find highest version `<= olderThan`.
   - If it is a tombstone:
     - Check if ANY version `> olderThan` exists. 
     - If yes, delete all versions `<= olderThan` (preserve newer ones).
     - If no, delete ALL versions of this PK.
   - If it is a normal row: delete all versions strictly less than this highest visible version.
   
   *SAFETY WARNING:* This logic intentionally deletes the version needed to 
   answer historical reads `<= olderThan`. This is safe ONLY under the 
   strict caller guarantee that no active reader exists with `Tx.ID <= olderThan`.
Tests: 
- v1 normal, v3 normal, Vacuum(2) -> keeps v1 and v3.
- v1 normal, v2 normal, Vacuum(2) -> deletes v1, keeps v2.
- v1 normal, v2 tombstone, v3 normal, Vacuum(2) -> deletes v1 and v2, keeps v3.

Task 8 — Concurrency & Edge Cases
- Concurrent Inserts of same PK with different Tx IDs (Table mutex serializes them, preventing KV-level races).
- Vacuum on empty table.
- Scan with massive version history (document OOM risk).

Task 9 — Benchmarks
- `Insert` / `Update` overhead (measures table lock + read-before-write + append).
- `Get` overhead.
- `Vacuum` speed.

Task 10 — docs/sql_engine_heap.md update
Update documentation to cover:
- Strictly monotonic Tx IDs, `ErrTxConflict` priority, and `Tx.ID > 0` rule.
- Table-level locking model.
- Uniform tombstone layout and NULL bitmask.
- Prevention of Primary Key updates.
- *CRITICAL:* The `Vacuum` safety contract regarding active readers and historical state deletion.
Update substring-check test.

Completion criteria
All 10 tasks implemented and tested. MVCC visibility and monotonic Tx rules strictly enforced. Table-level locking prevents read-before-write races. Tombstones and NULLs correctly encoded. `Vacuum` safely reclaims space without violating the active-reader contract. Documentation updated.