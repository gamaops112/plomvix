storage_kvstore_setup.md — On-Disk KVStore (Basic)
Scope
This plan delivers an on-disk, ordered key-value store (`kv` package) built 
on top of the hardened pager (`storage_enterprise.md`). It implements a 
page-based B+ Tree to satisfy the same `Get`/`Set`/`Delete`/`Scan` contract 
as the in-memory `store` package, using `key.Key` for ordered byte keys.

This plan does NOT deliver overflow pages for large values, buffer pool 
caching, B+ Tree node merging/rebalancing, or transactional page allocation.
It enforces strict maximum key and value sizes. 

Contract this tier honestly provides (read this before writing code)
- Ordered Key-Value Storage: Keys are stored in strict ascending order 
  using `key.Key.Compare()`. `Scan` returns results in this order.
- Strict Size Limits: Maximum key size is 64 bytes. Maximum value size 
  is 512 bytes. `Set` returns `ErrValueTooLarge` or `ErrKeyTooLarge` if 
  exceeded.
- Multi-Page Atomicity (with a known trade-off): All tree mutations 
  are wrapped in `pager.BeginTx()` and `pager.CommitTx()`. 
  *CRITICAL BASIC TIER TRADE-OFF:* Because the pager's `AllocatePage` is 
  NOT transactional, new pages required for splits are allocated *outside* 
  the KV transaction (Phase 2 of the split algorithm). If a crash occurs 
  after pages are allocated but before the KV transaction commits, those 
  pages are "leaked" (marked allocated in the pager, but unreferenced by 
  the tree). This wastes space but does not corrupt the tree.
- No Node Merging: Deleting keys removes them from the leaf only. Internal 
  separator keys are NEVER updated or removed in Basic. The tree remains 
  structurally valid for reads and scans, but space is not reclaimed.
- Concurrency: A single global `sync.RWMutex` serializes all writes. 
  Multiple concurrent scans are allowed.

Internal Node Routing Rule (Mandatory for Get/Set/Delete/Scan)
When searching for a `target` key in an internal node with keys `K[0...n-1]` 
and child pointers `C[0...n]`:
Find the first index `i` such that `target.Compare(K[i]) < 0` (strictly less). 
Follow child pointer `C[i]`.
If `target.Compare(K[i]) >= 0` for all `i`, follow the rightmost child 
pointer `C[n]`.
(This `upper_bound` rule ensures that even if a separator key is deleted 
from the leaf below it, the separator remains strictly greater than all 
keys in the left subtree, keeping routing correct without requiring 
internal node updates on delete).

Constants & Limits
package kv

import (
    "context"
    "errors"
    "github.com/plomvix/plomvix/internal/engine/sql/key"
    "github.com/plomvix/plomvix/internal/storage/pager"
)

const (
    MaxKeySize   = 64   // bytes
    MaxValSize   = 512  // bytes
)

// Derived from pager.PageSize (4096) - 12 bytes page header = 4084 bytes body.
// Node Header = 1 (Type) + 2 (NumKeys) + 8 (NextLeaf/ChildPtr0) = 11 bytes.
// Available for slots = 4084 - 11 = 4073 bytes.
// Leaf Slot = 64 (key) + 4 (val len) + 512 (val) = 580 bytes.
// MaxLeafKeys = floor(4073 / 580) = 7.
// Internal Slot = 64 (key) + 8 (child ptr) = 72 bytes.
// MaxInternalKeys = floor(4073 / 72) = 56.
const (
    MaxLeafKeys     = 7
    MaxInternalKeys = 56
    LeafSlotSize    = 580
    InternalSlotSize = 72
    
    NodeTypeLeaf     byte = 0x01
    NodeTypeInternal byte = 0x02
    NodeTypeMeta     byte = 0x03
)

On-disk Layout (FINAL, defined upfront)
KVStore Meta Page (Permanently reserved at Page 2)
Offset  Size  Field
0       1     NodeType (Must be 0x03 for Meta)
1       8     RootPageID (uint64). Sentinel 0xFFFFFFFFFFFFFFFF means empty tree.
9       ...   Reserved, zero-filled.

B+ Tree Leaf Node Page (Body is 4084 bytes)
Offset  Size  Field
0       1     NodeType (0x01)
1       2     NumKeys (uint16)
3       8     NextLeafPtr (uint64). Sentinel 0xFFFFFFFFFFFFFFFF if last leaf.
11      ...   Keys and Values stored as fixed-size slots (580 bytes each).
Slot Layout (repeated NumKeys times):
  0       64    Key (zero-padded if shorter)
  64      4     ValueLength (uint32, actual length of value, <= 512)
  68      512   Value (zero-padded if shorter)

B+ Tree Internal Node Page (Body is 4084 bytes)
Offset  Size  Field
0       1     NodeType (0x02)
1       2     NumKeys (uint16)
3       8     ChildPtr0 (uint64) - Pointer to the leftmost child.
11      ...   Keys and Child Pointers stored as fixed-size slots (72 bytes each).
Slot Layout (repeated NumKeys times):
  0       64    Key (zero-padded) - The separator key.
  64      8     ChildPtr (uint64) - Pointer to the right child for this key.

Public API
package kv

type Entry struct {
    Key   key.Key
    Value []byte
}

type KVStore interface {
    // Open initializes the KVStore, reading or creating the Meta Page.
    // Idempotent: if already open, returns nil. Returns ErrClosed if Close was called.
    Open(ctx context.Context) error

    // Get returns the value for the given key. Returns ErrKeyNotFound if missing.
    Get(ctx context.Context, k key.Key) ([]byte, error)

    // Set inserts or updates the key-value pair. Returns ErrKeyTooLarge or 
    // ErrValueTooLarge if limits are exceeded.
    Set(ctx context.Context, k key.Key, v []byte) error

    // Delete removes the key from the leaf. No-op if missing.
    Delete(ctx context.Context, k key.Key) error

    // Scan returns all entries where start <= key < end. 
    // An empty key.Key (len(k.Bytes()) == 0) represents unbounded start/end.
    Scan(ctx context.Context, start, end key.Key) ([]Entry, error)

    // Close releases KVStore resources. Does NOT close the underlying pager.
    // Idempotent. Subsequent operations return ErrClosed.
    Close(ctx context.Context) error
}

var (
    ErrKeyNotFound   = errors.New("kv: key not found")
    ErrKeyTooLarge   = errors.New("kv: key exceeds maximum size")
    ErrValueTooLarge = errors.New("kv: value exceeds maximum size")
    ErrTreeCorrupt   = errors.New("kv: B+ tree structure is corrupt")
    ErrNotOpen       = errors.New("kv: store is not open")
    ErrClosed        = errors.New("kv: store is closed")
)

// Constructor. Does NOT call pager.Open or allocate pages.
func New(p pager.Pager) KVStore

Tasks (do in order, one at a time)

Task 1 — Package skeleton, interface, constants, and Close
Create `internal/engine/sql/kv/kv.go`. Define the `KVStore` interface, 
`Entry` struct, sentinel errors, and all constants.
Implement the `New(p pager.Pager) KVStore` constructor. It returns a 
concrete `*btreeStore` struct holding the `pager.Pager`, a `sync.RWMutex`, 
`isOpen bool`, `closed bool`, and `rootPageID uint64` (initialized to sentinel).

Implement `Close`:
- Acquire `Lock`.
- Idempotent: if `p.closed` is true, return nil.
- Set `p.isOpen = false` and `p.closed = true`.
- Do NOT call `pager.Close()`.
- Return nil.
Tests: Verify constructor initializes fields. Verify interface compliance.
Verify Close is idempotent. Verify Open/Get/Set/Delete/Scan return ErrClosed 
after Close is called.

Task 2 — Node encode/decode (pure functions)
Create `internal/engine/sql/kv/node.go`. Implement pure functions for 
encoding and decoding Leaf, Internal, and Meta nodes based on the FINAL 
layouts defined above. No file I/O here.

func encodeLeafNode(keys []key.Key, values [][]byte, nextLeaf uint64) ([]byte, error)
  - Validates: len(keys) == len(values), len(keys) <= MaxLeafKeys.
  - Validates: all keys <= MaxKeySize, all values <= MaxValSize.
  - Validates: keys are in strictly ascending order.
  - Returns ErrTreeCorrupt (or specific validation error) on failure.

func decodeLeafNode(data []byte) (keys []key.Key, values [][]byte, nextLeaf uint64, err error)
  - Validates `len(data) == pager.PageSize - 12`.
  - Reads NodeType. If != NodeTypeLeaf, return ErrTreeCorrupt.
  - Reads NumKeys, NextLeafPtr.
  - Iterates NumKeys times, reading 580-byte slots from offset 11.
  - Trims zero-padding from keys and values based on ValueLength.

func encodeInternalNode(childPtrs []uint64, keys []key.Key) ([]byte, error)
  - Validates: len(childPtrs) == len(keys) + 1, len(keys) <= MaxInternalKeys.

func decodeInternalNode(data []byte) (childPtrs []uint64, keys []key.Key, err error)
  - Validates NodeType == NodeTypeInternal.
  - Reads NumKeys, ChildPtr0.
  - Iterates NumKeys times, reading 72-byte slots from offset 11.

func encodeMetaPage(rootPageID uint64) []byte
func decodeMetaPage(data []byte) (rootPageID uint64, err error)

Tests: Round-trip encode/decode for empty, full, and partially full nodes. 
Verify zero-padding trimming. Verify malformed data/wrong NodeType returns ErrTreeCorrupt.
Verify encode validation rejects unsorted keys, mismatched lengths, and oversized keys/values.

Task 3 — Open, Meta Page Init, and B+ Tree Search (Get)
Implement `Open` on `btreeStore`.
1. Acquire `Lock`.
2. If `p.closed` is true, return `ErrClosed`.
3. If `p.isOpen` is true, return nil (idempotent).
4. Check `pager.PageCount()`.
5. If `PageCount == 2` (fresh pager, only header and mirror exist):
   - `id, err := pager.AllocatePage()`. If `id != 2`, return `ErrTreeCorrupt`.
   - `pager.BeginTx()`
   - Encode an empty Meta Page (rootPageID = sentinel), write to Page 2 via `pager.WritePage`.
   - `pager.CommitTx()`. If commit fails, return the error.
6. If `PageCount > 2`:
   - Read Page 2 via `pager.ReadPage`.
   - If it is not a valid Meta Page (NodeType != 0x03), return `ErrTreeCorrupt`.
   - Decode `rootPageID` and store it in `p.rootPageID`.
7. Set `p.isOpen = true`.

Implement `Get`:
1. Acquire `RLock`. Defer `RUnlock`. Check `isOpen`/`closed`.
2. If `rootPageID` is sentinel, return `ErrKeyNotFound`.
3. Traverse the tree using the `upper_bound` routing rule:
   - Read current node page via `pager.ReadPage`. Decode it.
   - If Internal: Find child pointer using `upper_bound` on keys. Follow pointer.
   - If Leaf: Binary search for exact key. If found, return COPY of value. 
     If not, return `ErrKeyNotFound`.
Tests: Open on fresh pager. Open on existing tree. Open idempotency. Open fails if 
first allocated page is not Page 2. Get on empty tree. Get on multi-level tree. Get missing key.

Task 4 — B+ Tree Insert (Set) & 3-Phase Leaf Splitting
Implement `Set` on `btreeStore`.
1. Acquire `Lock`. Defer `Unlock`. Check `isOpen`/`closed`. Validate sizes.
2. If `rootPageID` is sentinel:
   - Allocate leaf page (OUTSIDE transaction).
   - `pager.BeginTx()`
   - Encode leaf, write via `pager.WritePage`.
   - Update Meta Page with new rootPageID, write via `pager.WritePage`.
   - `pager.CommitTx()`. 
   - CRITICAL STATE RULE: Do not update the in-memory `p.rootPageID` until 
     `pager.CommitTx()` returns `nil`. If commit fails, return the error 
     and leave in-memory state unchanged.
   - Return nil.
3. Phase 1 (Traversal & Planning):
   - Traverse to target leaf using `upper_bound`, building `pathStack`.
   - If leaf has space, no split needed.
   - If leaf is full, calculate split depth (e.g., leaf split = 1 new page. 
     If parent full, +1 new page, etc., up to root).
4. Phase 2 (Pre-allocation):
   - Allocate ALL required new pages via `pager.AllocatePage()` OUTSIDE 
     any transaction. Store them in a local slice `newPages`.
   - If any allocation fails, return the error immediately.
5. Phase 3 (Transaction & Write):
   - `pager.BeginTx()`
   - If no split: insert key/value, write leaf.
   - If split:
     - Pop a new page ID from `newPages` for the new leaf.
     - Distribute keys, update NextLeafPtr, write old and new leaf.
     - Call `insertIntoParent` using pre-allocated pages from `newPages` 
       for any internal splits.
     - If root splits, pop a new page ID for the new root, write it, 
       update Meta Page.
   - `pager.CommitTx()`.
   - CRITICAL STATE RULE: If a root split occurred, do not update the 
     in-memory `p.rootPageID` until `pager.CommitTx()` returns `nil`.
   - Rollback Rule: If `pager.CommitTx()` fails, return the error. Do not 
     attempt manual rollback; the pager's WAL guarantees atomicity and will 
     recover or rollback on the next `Open()`.
   
   `insertIntoParent` logic:
   - Pop parent from pathStack. If stack empty (old leaf was root):
     - Pop new internal node ID from `newPages`.
     - Encode internal node, write it. Update Meta Page rootPageID.
   - Else (parent exists):
     - Read parent. If space, insert sep/newPtr, write parent.
     - If full, pop new internal ID from `newPages`, distribute keys, 
       push middle key up recursively.
Tests: Insert until leaf splits. Insert until root splits. Insert duplicate (update). 
Verify tree structure. Verify leaked pages on simulated crash between Phase 2 and Phase 3.
Verify in-memory rootPageID is NOT updated if CommitTx fails.

Task 5 — B+ Tree Delete (Basic)
Implement `Delete` on `btreeStore`.
1. Acquire `Lock`. Defer `Unlock`. Check `isOpen`/`closed`.
2. Traverse to leaf using `upper_bound`. If key not found, return nil.
3. `pager.BeginTx()`
4. Remove key/value from leaf. Shift remaining slots. Write leaf.
5. `pager.CommitTx()`.
6. CRITICAL BASIC TIER CONSTRAINT: Internal separator keys are NEVER 
   updated or removed. The `upper_bound` routing rule guarantees that 
   stale separators remain valid routing keys. Document this explicitly.
Tests: Delete existing. Delete missing. Delete separator key (verify tree 
still routes correctly to the leaf).

Task 6 — Scan Implementation
Implement `Scan` on `btreeStore`.
1. Acquire `RLock`. Defer `RUnlock`. Check `isOpen`/`closed`.
2. If `rootPageID` is sentinel, return empty slice.
3. Find starting leaf: Traverse using `upper_bound` with `start` key (or 
   follow ChildPtr0 to leftmost leaf if `start` is empty).
4. Iterate leaf keys. If `key >= start` (or start empty), add to results.
5. If leaf exhausted, follow `NextLeafPtr`. Stop if sentinel.
6. Stop scanning when `key >= end` (if end is not empty).
7. Return collected `[]Entry` (all copies).
Tests: Full range. Specific bounds. Empty bounds. Scan across multiple leaves.

Task 7 — Concurrency & Race Testing
Add `go test -race ./internal/engine/sql/kv/...`.
Write test: 50 goroutines doing concurrent `Set` and `Get`.
Write test: Concurrent `Scan` and `Set`. Verify multiple scans run 
concurrently; writes are serialized.

Task 8 — Compliance & Edge Case Tests
Table-driven suite:
- Empty key / Empty value.
- Max size key / Max size value.
- Oversized key / value (expect errors).
- Insert 1000 keys, verify retrieval.
- Delete all keys, verify `ErrKeyNotFound`.
- Unbounded scans.

Task 9 — Benchmarks
Pre-populate 10,000 keys.
Benchmark random `Get`, sequential `Set`, full `Scan`.

Task 10 — docs/sql_engine_kv.md
Write documentation covering architecture, strict limits, concurrency, 
and atomicity guarantees. 
Explicitly document the "leaked pages on crash" trade-off, the 3-phase 
split algorithm, and the "no internal separator updates on delete" behavior.
Add substring-check test in `internal/engine/sql/kv/docs_test.go` asserting 
key contract phrases.

Completion criteria
All 10 tasks implemented and tested. `go test -race` passes. B+ Tree 
correctly handles splits via the 3-phase pre-allocation algorithm and 
basic leaf-only deletes. `Scan` uses `NextLeafPtr`. All multi-page 
mutations use `pager.BeginTx()`/`CommitTx()`. Page allocation happens 
strictly OUTSIDE transactions. Size limits enforced. `upper_bound` routing 
is used consistently. In-memory `rootPageID` is only updated after successful 
commit. Meta page initialization is fully transactional. Documentation 
exists and passes substring checks.