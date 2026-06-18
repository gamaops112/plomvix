storage_kvstore_enterprise.md — On-Disk KVStore (Enterprise Hardening)
Scope
This plan hardens the on-disk B+ Tree KVStore (`kv` package) delivered in 
`storage_kvstore_setup.md`. It adds support for large values (up to 16MB) 
via overflow pages (TOAST), deep structural integrity verification (`Check`), 
and atomic space reclamation (`Compact`). 

This plan explicitly defers online B+ Tree node merging/rebalancing, 
buffer pool/page cache management, and pager-level free-space audits to 
future plans. It relies entirely on the pager's WAL and embraces the 
"leaked pages on crash" trade-off established in Basic.

Contract this tier honestly provides (read this before writing code)
- Large Value Support (TOAST): Values up to 16MB are supported. Values 
  > 512 bytes are stored in a linked list of overflow pages. The leaf 
  slot stores a 504-byte prefix and a pointer to the first overflow page.
- Safe Updates & Deletes: Updating or deleting a large value transactionally 
  removes the tree reference. Old overflow pages are NOT freed directly 
  during `Set` or `Delete`; they become unreachable and are reclaimed 
  later by `Compact` (if they were part of a committed old tree) or a 
  future free-space audit (if they were orphaned by a crash).
- Deep Structural Integrity: `Check(ctx)` walks the entire live B+ Tree 
  and overflow chains, verifying sorting, routing rules, pointer validity, 
  page reachability, and detecting cycles or double-references. It does 
  not fail on leaked or unreachable pages.
- Atomic Space Reclamation: `Compact(ctx)` rebuilds the B+ Tree from 
  scratch using a copy-on-write (shadow paging) approach. It atomically 
  swaps the root pointer, reclaiming all space from deleted keys, 
  underfull nodes, and pages reachable from the old tree. It does NOT 
  reclaim fully orphaned pages (e.g., allocated but never linked due to 
  a prior crash); that requires a future free-space audit feature.
- Concurrency: `Check` and `Compact` acquire the global write lock to 
  ensure a consistent snapshot.

Constants & Limits Updates
package kv

const (
    MaxValSizeEnterprise = 16 * 1024 * 1024 // 16MB hard limit for TOAST
    MaxOverflowChainLen  = 5000             // Sanity limit (16MB / 4075 ≈ 4118 pages)
    
    NodeTypeOverflow byte = 0x04           // New node type for overflow pages
)

On-disk Layout Updates
B+ Tree Leaf Node Page (Slot interpretation for Large Values)
If `ValueLength <= 512`: Behaves exactly as Basic (inline value).
If `ValueLength > 512`:
Slot Layout (580 bytes total):
  0       64    Key (zero-padded)
  64      4     ValueLength (uint32, actual total length, > 512)
  68      8     FirstOverflowPageID (uint64)
  76      504   Value Prefix (first 504 bytes of the value, zero-padded if shorter)

Overflow Page (Body is 4084 bytes)
Offset  Size  Field
0       1     NodeType (0x04)
1       8     NextOverflowPageID (uint64). Sentinel 0xFFFFFFFFFFFFFFFF if last.
9       4075  Data Chunk (raw value bytes)

Public API Additions
package kv

type KVStore interface {
    // ... all Basic methods ...

    // Check walks the live B+ Tree and verifies deep structural integrity,
    // including page reachability and cycle detection.
    Check(ctx context.Context) error

    // Compact rewrites the entire B+ Tree to new pages, reclaiming space
    // from deleted keys, underfull nodes, and pages reachable from the old tree.
    // It atomically swaps the root pointer.
    Compact(ctx context.Context) error
}

Tasks (do in order, one at a time)

Task 1 — Package updates, limits, and API extensions
Update `internal/engine/sql/kv/kv.go`. Add `MaxValSizeEnterprise`, 
`MaxOverflowChainLen`, and `NodeTypeOverflow`. Add `Check` and `Compact` 
to the `KVStore` interface. 
CRITICAL: Do NOT change the `Set` validation limit to 16MB in this task. 
Keep the Basic 512-byte limit active until Task 5 implements overflow writing.
Tests: Verify `Check` and `Compact` exist on the interface.

Task 2 — Overflow page encode/decode (pure functions)
Add to `internal/engine/sql/kv/node.go`:
func encodeOverflowPage(nextPtr uint64, chunk []byte) ([]byte, error)
  - Validates len(chunk) <= 4075.
func decodeOverflowPage(data []byte) (nextPtr uint64, chunk []byte, err error)
  - Validates NodeType == NodeTypeOverflow.
Tests: Round-trip encode/decode. Verify chunk size limits.

Task 3 — Leaf node large-value handling & `leafValueRef`
Introduce a decoded slot struct to replace the ambiguous `values [][]byte` 
return type from Basic:
type leafValueRef struct {
    totalLen       uint32
    inline         []byte   // up to 512 bytes (if !hasOverflow) or 504 bytes (if hasOverflow)
    overflowPageID uint64   // valid only if hasOverflow
    hasOverflow    bool
}

Update `encodeLeafNode` and `decodeLeafNode` in `node.go` to use `[]leafValueRef`.
If `totalLen > 512`:
- `encode` expects the 512-byte inline space to contain the 8-byte 
  `FirstOverflowPageID` followed by the 504-byte prefix.
- `decode` reads `FirstOverflowPageID` and the prefix, setting `hasOverflow = true`.
Tests: Round-trip a leaf with mixed inline and overflow values.

Task 4 — `Get` with overflow traversal
Update `Get` in `kv.go`.
Use the `leafValueRef` returned by `decodeLeafNode`.
If `ref.hasOverflow` is true:
1. Allocate a buffer of size `ref.totalLen`.
2. Copy `ref.inline` (the 504-byte prefix) into the buffer.
3. Follow `ref.overflowPageID`. Read page, decode as overflow page.
4. Copy chunk into buffer. Follow `NextOverflowPageID`.
5. Stop when `NextOverflowPageID` is sentinel or chain exceeds `MaxOverflowChainLen`.
6. Return the full buffer.
Tests: Get a 10KB value. Get a 1MB value. Get a value with corrupt overflow chain (expect ErrTreeCorrupt).

Task 5 — `Set` with overflow allocation (3-phase) & 16MB Limit
Update `Set` validation in `kv.go` to use `MaxValSizeEnterprise` (16MB).
If `len(v) > 512`:
1. Phase 1 (Planning): Calculate N = ceil((len(v) - 504) / 4075).
2. Phase 2 (Pre-allocation): Allocate N overflow pages + any B+ Tree split pages OUTSIDE transaction.
3. Phase 3 (Transaction):
   - `pager.BeginTx()`
   - Write all N overflow pages, linking them together.
   - Encode leaf slot with `FirstOverflowPageID` and 504-byte prefix.
   - Write leaf page.
   - `pager.CommitTx()`.

Handling Updates: If the key already exists and its old `leafValueRef.hasOverflow` 
was true, the old overflow pages are NOT freed during this transaction. They 
become unreachable and are reclaimed later by `Compact`.
Tests: 
- Insert 10KB value, verify retrieval. 
- Set large -> Set small -> Get returns small -> Check passes. 
- Set large -> Set large -> Get returns new large. 
- Crash simulation between Phase 2 and 3 (verify leaked pages, tree intact). 
- Set value exactly MaxValSizeEnterprise succeeds. 
- Set value MaxValSizeEnterprise + 1 fails with ErrValueTooLarge.

Task 6 — `Delete` (Leaf reference removal only)
Update `Delete` in `kv.go`.
1. Traverse to leaf. 
2. `pager.BeginTx()`, remove slot, write leaf, `pager.CommitTx()`.
CRITICAL BASIC-COMPATIBLE RULE: Delete NEVER frees overflow pages directly. 
It only removes the leaf reference transactionally. Unreachable overflow 
pages from committed deletes/updates are reclaimed exclusively by `Compact`. 
Fully orphaned pages from crashed transactions require a future audit feature.
Tests: Delete large value. Verify `Check` passes (tree is valid, just has unreachable pages). Verify `Compact` reclaims the space.

Task 7 — `Check` implementation (Deep Reachability)
Implement `Check(ctx)`.
1. Acquire `Lock`.
2. Maintain a `visitedPages map[uint64]struct{}`.
3. Walk tree from root. Verify internal node routing rules (`upper_bound`).
4. Verify leaf sorted order and `NextLeafPtr` chain (detect cycles).
5. For every page read, verify `pageID < pager.PageCount()` and `NodeType` matches expectation.
6. For every child pointer and overflow page, check if it is already in `visitedPages`. If so, return `ErrTreeCorrupt` (detects double-references and cycles).
7. For large values, walk the overflow chain. Verify total length matches `ValueLength`.

SCOPE RULE: `Check` validates ONLY the live tree reachable from the current 
meta root and its referenced overflow chains. It does NOT scan the pager 
free-list or all allocated pages. Leaked or unreachable pages do not cause 
`Check` to fail.
Tests: Manually corrupt an overflow pointer to create a cycle, verify `Check` detects it. Manually create a double-reference, verify `Check` detects it.

Task 8 — `Compact` implementation (Shadow Paging Rebuild)
Implement `Compact(ctx)`.
1. Acquire `Lock`.
2. EMPTY TREE SHORT-CIRCUIT: If the tree is empty (rootPageID is sentinel), 
   return nil immediately without allocating or writing any pages.
3. Stream all KVs from the old tree (traverse leaves via `NextLeafPtr`).
4. Bulk-load a NEW B+ Tree:
   - Calculate total pages needed.
   - Allocate ALL new leaf and internal pages OUTSIDE transaction.
   - Write all new pages directly to the main file via `pager.WritePage`.
   - CRITICAL LEAK RULE: A crash during this rebuild phase may leak the 
     newly allocated shadow pages, but the old tree remains intact and valid.
5. Post-Build Validation: Traverse the new shadow tree from the new root 
   and verify all copied values are readable. If validation fails, abort 
   and return `ErrTreeCorrupt` (leaking the shadow pages).
6. `pager.BeginTx()`
7. Write new Meta Page pointing to the new root.
8. `pager.CommitTx()`
9. Best-effort Freeing: Traverse the OLD tree (reachable from the old meta 
   root) and call `pager.FreePage` on all old pages (including old overflow 
   pages) outside the transaction. If freeing fails, return the error, but 
   the new tree remains valid and durable.
   
   CRITICAL RECLAMATION SCOPE: This step reclaims pages reachable from the 
   old tree. It does NOT reclaim fully orphaned pages that were leaked by 
   previous crashes (e.g., allocated but never linked into any tree) and 
   are unreachable from the old root. Those require a future pager-level 
   free-space audit feature.
   
   CRITICAL RETRY SAFETY RULE: After the meta root swap commits, old-tree 
   pages are no longer part of the live tree. Best-effort freeing failures 
   must not affect `Check(ctx)`, because `Check` only validates pages 
   reachable from the current meta root. A later `Compact` may ignore 
   already-unreachable pages and does not need to free the same old tree 
   again unless a future free-space audit feature is added.

Tests: 
- Insert 1000 keys, delete 500, verify `Compact` increases reusable free pages (future allocations reuse freed pages). Do NOT require `PageCount` to decrease. 
- Crash during step 4 (verify old tree intact). 
- Crash during step 9 (verify new tree intact, old pages leaked).
- Compact on empty tree returns nil and performs no I/O.

Task 9 — Edge cases & Concurrency
- Concurrent `Set` (large) and `Compact`.
- `Check` on empty tree.

Task 10 — Benchmarks
- `Set` and `Get` for 10KB, 1MB, 10MB values.
- `Compact` speed on a 100MB database with 50% fragmentation.

Task 11 — docs/sql_engine_kv.md update
Update documentation to cover TOAST layout, `Check` reachability rules, and `Compact` shadow-paging semantics. 
Explicitly document that `Compact` reclaims pages from the old tree, but 
fully orphaned leaked pages (from crashed allocations) require a future 
pager-level audit feature.
Update substring-check test.

Completion criteria
All 11 tasks implemented and tested. `Check` correctly identifies structural corruption, cycles, and double-references within the live tree. `Compact` successfully rebuilds the tree via shadow paging and reclaims space from the old tree. Large values (up to 16MB) are correctly stored, retrieved, and deleted without unsafe non-transactional freeing.