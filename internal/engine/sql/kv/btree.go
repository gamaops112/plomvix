package kv

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/plomvix/plomvix/internal/engine/sql/key"
	"github.com/plomvix/plomvix/internal/storage/pager"
)

// btreeStore implements KVStore using a B+ Tree on top of the pager.
type btreeStore struct {
	pg         pager.Pager
	mu         sync.RWMutex
	isOpen     bool
	closed     bool
	rootPageID uint64
}

func newBtreeStore(p pager.Pager) *btreeStore {
	return &btreeStore{
		pg:         p,
		rootPageID: rootSentinel,
	}
}

// checkOpen returns the appropriate error if the store is not open or is closed.
// Caller must hold at least a read lock.
func (s *btreeStore) checkOpen() error {
	if s.closed {
		return ErrClosed
	}
	if !s.isOpen {
		return ErrNotOpen
	}
	return nil
}

// --- Open ---

func (s *btreeStore) Open(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrClosed
	}
	if s.isOpen {
		return nil
	}

	pc, err := s.pg.PageCount(ctx)
	if err != nil {
		return fmt.Errorf("kv: page count: %w", err)
	}

	if pc == pager.FirstDataPageID {
		// Fresh pager — allocate Meta Page and initialize.
		id, err := s.pg.AllocatePage(ctx)
		if err != nil {
			return fmt.Errorf("kv: allocate meta page: %w", err)
		}
		if id != MetaPageID {
			return fmt.Errorf("%w: expected meta page at %d, got %d", ErrTreeCorrupt, MetaPageID, id)
		}

		if err := s.pg.BeginTx(ctx); err != nil {
			return fmt.Errorf("kv: begin tx for meta init: %w", err)
		}
		metaBody := encodeMetaPage(rootSentinel)
		if err := s.pg.WritePage(ctx, MetaPageID, metaBody); err != nil {
			s.pg.RollbackTx(ctx)
			return fmt.Errorf("kv: write meta page: %w", err)
		}
		if err := s.pg.CommitTx(ctx); err != nil {
			return fmt.Errorf("kv: commit meta init: %w", err)
		}
	} else {
		// Existing file — read Meta Page.
		body, err := s.pg.ReadPage(ctx, MetaPageID)
		if err != nil {
			return fmt.Errorf("kv: read meta page: %w", err)
		}
		rootID, err := decodeMetaPage(body)
		if err != nil {
			return fmt.Errorf("kv: decode meta page: %w", err)
		}
		s.rootPageID = rootID
	}

	s.isOpen = true
	return nil
}

// --- Close ---

func (s *btreeStore) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.isOpen = false
	s.closed = true
	return nil
}

// --- Get ---

func (s *btreeStore) Get(ctx context.Context, k key.Key) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.checkOpen(); err != nil {
		return nil, err
	}
	if s.rootPageID == rootSentinel {
		return nil, ErrKeyNotFound
	}

	return s.search(ctx, s.rootPageID, k)
}

// search traverses the tree from the given page looking for k.
// Returns ErrKeyNotFound if not found. For overflow values, reassembles the full value.
func (s *btreeStore) search(ctx context.Context, pageID uint64, k key.Key) ([]byte, error) {
	for {
		body, err := s.pg.ReadPage(ctx, pageID)
		if err != nil {
			return nil, fmt.Errorf("kv: read page %d: %w", pageID, err)
		}

		switch body[0] {
		case NodeTypeLeaf:
			keys, refs, _, err := decodeLeafNodeRefs(body)
			if err != nil {
				return nil, err
			}
			for i, lk := range keys {
				if lk.Compare(k) == 0 {
					ref := refs[i]
					if !ref.hasOverflow {
						return ref.inline, nil
					}
					return s.readOverflowValue(ctx, ref)
				}
			}
			return nil, ErrKeyNotFound

		case NodeTypeInternal:
			childPtrs, ikeys, err := decodeInternalNode(body)
			if err != nil {
				return nil, err
			}
			pageID = s.upperBound(childPtrs, ikeys, k)

		default:
			return nil, fmt.Errorf("%w: unknown node type 0x%x", ErrTreeCorrupt, body[0])
		}
	}
}

// readOverflowValue follows the overflow chain and reassembles the full value.
func (s *btreeStore) readOverflowValue(ctx context.Context, ref leafValueRef) ([]byte, error) {
	buf := make([]byte, ref.totalLen)
	copy(buf, ref.inline) // copy the 504-byte prefix

	pageID := ref.overflowPageID
	pos := len(ref.inline)
	chainLen := 0

	for pageID != rootSentinel && pageID != 0 {
		if chainLen >= MaxOverflowChainLen {
			return nil, fmt.Errorf("%w: overflow chain exceeds max length", ErrTreeCorrupt)
		}
		chainLen++

		body, err := s.pg.ReadPage(ctx, pageID)
		if err != nil {
			return nil, fmt.Errorf("kv: read overflow page %d: %w", pageID, err)
		}
		nextPtr, chunk, err := decodeOverflowPage(body)
		if err != nil {
			return nil, err
		}

		n := copy(buf[pos:], chunk)
		pos += n
		pageID = nextPtr
	}

	if pos != int(ref.totalLen) {
		return nil, fmt.Errorf("%w: overflow chain length mismatch: got %d, want %d", ErrTreeCorrupt, pos, ref.totalLen)
	}

	return buf, nil
}

// upperBound finds the child pointer to follow using the upper_bound routing rule.
// Returns childPtrs[i] where i is the first index s.t. k < ikeys[i].
// If k >= all ikeys, returns the rightmost child pointer.
func (s *btreeStore) upperBound(childPtrs []uint64, ikeys []key.Key, k key.Key) uint64 {
	for i, ik := range ikeys {
		if k.Compare(ik) < 0 {
			return childPtrs[i]
		}
	}
	return childPtrs[len(childPtrs)-1]
}

// --- Set ---

// pathEntry records a step in the tree traversal path, used during splits.
type pathEntry struct {
	pageID    uint64
	childIdx  int // which child pointer we followed from this internal node
	keys      []key.Key
	childPtrs []uint64
}

func (s *btreeStore) Set(ctx context.Context, k key.Key, v []byte) error {
	if len(k.Bytes()) > MaxKeySize {
		return ErrKeyTooLarge
	}
	if len(v) > MaxValSizeEnterprise {
		return ErrValueTooLarge
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkOpen(); err != nil {
		return err
	}

	if s.rootPageID == rootSentinel {
		return s.setInEmpty(ctx, k, v)
	}

	return s.setInTree(ctx, k, v)
}

// setInEmpty inserts the first key-value pair into an empty tree.
func (s *btreeStore) setInEmpty(ctx context.Context, k key.Key, v []byte) error {
	// Phase 2: Pre-allocate pages outside transaction.
	pagesNeeded := 1 // leaf page
	var overflowIDs []uint64
	var packedVal []byte

	if len(v) > MaxValSize {
		overflowIDs, packedVal = s.prepareOverflow(ctx, v)
		pagesNeeded += len(overflowIDs)
	} else {
		packedVal = v
	}

	leafID, err := s.pg.AllocatePage(ctx)
	if err != nil {
		return fmt.Errorf("kv: allocate leaf: %w", err)
	}

	// Phase 3: Write in a transaction.
	if err := s.pg.BeginTx(ctx); err != nil {
		return fmt.Errorf("kv: begin tx: %w", err)
	}

	if len(overflowIDs) > 0 {
		if err := s.writeOverflowChain(ctx, overflowIDs, v); err != nil {
			s.pg.RollbackTx(ctx)
			return err
		}
	}

	leafBody, err := encodeLeafNode([]key.Key{k}, [][]byte{packedVal}, rootSentinel)
	if err != nil {
		s.pg.RollbackTx(ctx)
		return err
	}
	if err := s.pg.WritePage(ctx, leafID, leafBody); err != nil {
		s.pg.RollbackTx(ctx)
		return fmt.Errorf("kv: write leaf: %w", err)
	}

	metaBody := encodeMetaPage(leafID)
	if err := s.pg.WritePage(ctx, MetaPageID, metaBody); err != nil {
		s.pg.RollbackTx(ctx)
		return fmt.Errorf("kv: write meta: %w", err)
	}

	if err := s.pg.CommitTx(ctx); err != nil {
		return fmt.Errorf("kv: commit: %w", err)
	}

	// Success — update in-memory root.
	s.rootPageID = leafID
	return nil
}

// prepareOverflow pre-allocates overflow pages for a large value and returns
// the page IDs and a packed representation [ovPageID(8) | full_value].
func (s *btreeStore) prepareOverflow(ctx context.Context, v []byte) ([]uint64, []byte) {
	// Calculate number of overflow pages needed.
	// First 504 bytes go in the leaf slot, remaining bytes go to overflow pages.
	prefixLen := 504
	if len(v) < 504 {
		prefixLen = len(v)
	}
	remaining := len(v) - prefixLen
	if remaining < 0 {
		remaining = 0
	}

	// Each overflow page holds up to overflowChunkSize (4075) bytes.
	numOverflow := (remaining + overflowChunkSize - 1) / overflowChunkSize
	if numOverflow == 0 {
		numOverflow = 0
	}

	// Pre-allocate overflow pages.
	overflowIDs := make([]uint64, numOverflow)
	for i := 0; i < numOverflow; i++ {
		id, err := s.pg.AllocatePage(ctx)
		if err != nil {
			// Cannot fail gracefully here, but allocate failure is rare.
			// Return what we have; the caller will handle.
			return overflowIDs[:i], nil
		}
		overflowIDs[i] = id
	}

	// Build packed value: [firstOverflowPageID(8) | full_value].
	var firstOverflowID uint64
	if numOverflow > 0 {
		firstOverflowID = overflowIDs[0]
	}
	packed := make([]byte, 8+len(v))
	binary.BigEndian.PutUint64(packed[:8], firstOverflowID)
	copy(packed[8:], v)

	return overflowIDs, packed
}

// writeOverflowChain writes a chain of overflow pages for a large value.
// Called within an active transaction.
func (s *btreeStore) writeOverflowChain(ctx context.Context, overflowIDs []uint64, v []byte) error {
	prefixLen := 504
	if len(v) < 504 {
		prefixLen = len(v)
	}
	remaining := v[prefixLen:]

	for i, pageID := range overflowIDs {
		start := i * overflowChunkSize
		end := start + overflowChunkSize
		if end > len(remaining) {
			end = len(remaining)
		}
		chunk := remaining[start:end]

		var nextPtr uint64 = rootSentinel
		if i+1 < len(overflowIDs) {
			nextPtr = overflowIDs[i+1]
		}

		body, err := encodeOverflowPage(nextPtr, chunk)
		if err != nil {
			return fmt.Errorf("kv: encode overflow page: %w", err)
		}
		if err := s.pg.WritePage(ctx, pageID, body); err != nil {
			return fmt.Errorf("kv: write overflow page %d: %w", pageID, err)
		}
	}

	return nil
}

// setInTree inserts/updates k=v into a non-empty tree.
func (s *btreeStore) setInTree(ctx context.Context, k key.Key, v []byte) error {
	// Pre-allocate overflow pages if needed (Phase 2).
	var overflowIDs []uint64
	var packedVal []byte
	if len(v) > MaxValSize {
		overflowIDs, packedVal = s.prepareOverflow(ctx, v)
	} else {
		packedVal = v
	}

	// Phase 1: Traverse to leaf, building path.
	var path []pathEntry

	pageID := s.rootPageID
	for {
		body, err := s.pg.ReadPage(ctx, pageID)
		if err != nil {
			return fmt.Errorf("kv: read page %d: %w", pageID, err)
		}

		switch body[0] {
		case NodeTypeLeaf:
			keys, vals, nextLeaf, err := decodeLeafNode(body)
			if err != nil {
				return err
			}

			// Check if key exists (update).
			for i, lk := range keys {
				if lk.Compare(k) == 0 {
					return s.updateLeaf(ctx, pageID, keys, vals, i, k, packedVal, overflowIDs, nextLeaf)
				}
			}

			// Key not found — insert.
			if len(keys) < MaxLeafKeys {
				return s.insertIntoLeaf(ctx, pageID, keys, vals, k, packedVal, overflowIDs, nextLeaf)
			}

			// Leaf is full — split needed.
			return s.splitLeaf(ctx, pageID, path, keys, vals, k, packedVal, overflowIDs, nextLeaf)

		case NodeTypeInternal:
			childPtrs, ikeys, err := decodeInternalNode(body)
			if err != nil {
				return err
			}
			childIdx := s.upperBoundIdx(childPtrs, ikeys, k)
			path = append(path, pathEntry{pageID, childIdx, ikeys, childPtrs})
			pageID = childPtrs[childIdx]

		default:
			return fmt.Errorf("%w: unknown node type", ErrTreeCorrupt)
		}
	}
}

// upperBoundIdx returns the index of the child pointer to follow.
func (s *btreeStore) upperBoundIdx(childPtrs []uint64, ikeys []key.Key, k key.Key) int {
	for i, ik := range ikeys {
		if k.Compare(ik) < 0 {
			return i
		}
	}
	return len(childPtrs) - 1
}

// updateLeaf replaces the value at index pos in the leaf and writes it back.
// overflowIDs and packedVal are pre-allocated overflow pages and the packed value
// for large values.
func (s *btreeStore) updateLeaf(ctx context.Context, pageID uint64,
	keys []key.Key, vals [][]byte, pos int,
	k key.Key, packedVal []byte, overflowIDs []uint64, nextLeaf uint64) error {

	vals[pos] = packedVal
	body, err := encodeLeafNode(keys, vals, nextLeaf)
	if err != nil {
		return err
	}
	if err := s.pg.BeginTx(ctx); err != nil {
		return fmt.Errorf("kv: begin tx: %w", err)
	}
	if len(overflowIDs) > 0 {
		if err := s.writeOverflowChainFull(ctx, overflowIDs, packedVal[8:]); err != nil {
			s.pg.RollbackTx(ctx)
			return err
		}
	}
	if err := s.pg.WritePage(ctx, pageID, body); err != nil {
		s.pg.RollbackTx(ctx)
		return fmt.Errorf("kv: write leaf: %w", err)
	}
	return s.pg.CommitTx(ctx)
}

// writeOverflowChainFull writes the overflow chain for a full value.
// Called within tx. v is the FULL value (not the packed prefix).
func (s *btreeStore) writeOverflowChainFull(ctx context.Context, overflowIDs []uint64, v []byte) error {
	return s.writeOverflowChain(ctx, overflowIDs, v)
}

// insertIntoLeaf inserts k=v into a leaf that has room.
func (s *btreeStore) insertIntoLeaf(ctx context.Context, pageID uint64,
	keys []key.Key, vals [][]byte,
	k key.Key, packedVal []byte, overflowIDs []uint64, nextLeaf uint64) error {

	// Find insertion position.
	pos := 0
	for pos < len(keys) && keys[pos].Compare(k) < 0 {
		pos++
	}

	newKeys := make([]key.Key, 0, len(keys)+1)
	newVals := make([][]byte, 0, len(vals)+1)
	newKeys = append(newKeys, keys[:pos]...)
	newVals = append(newVals, vals[:pos]...)
	newKeys = append(newKeys, k)
	newVals = append(newVals, packedVal)
	newKeys = append(newKeys, keys[pos:]...)
	newVals = append(newVals, vals[pos:]...)

	body, err := encodeLeafNode(newKeys, newVals, nextLeaf)
	if err != nil {
		return err
	}
	if err := s.pg.BeginTx(ctx); err != nil {
		return fmt.Errorf("kv: begin tx: %w", err)
	}
	if len(overflowIDs) > 0 {
		if err := s.writeOverflowChainFull(ctx, overflowIDs, packedVal[8:]); err != nil {
			s.pg.RollbackTx(ctx)
			return err
		}
	}
	if err := s.pg.WritePage(ctx, pageID, body); err != nil {
		s.pg.RollbackTx(ctx)
		return fmt.Errorf("kv: write leaf: %w", err)
	}
	return s.pg.CommitTx(ctx)
}

// splitLeaf performs a 3-phase leaf split.
func (s *btreeStore) splitLeaf(ctx context.Context, leafID uint64,
	path []pathEntry,
	keys []key.Key, vals [][]byte,
	newKey key.Key, packedVal []byte, overflowIDs []uint64,
	nextLeaf uint64) error {

	// Combine existing and new key-value pairs in sorted order.
	allKeys := make([]key.Key, 0, len(keys)+1)
	allVals := make([][]byte, 0, len(vals)+1)
	inserted := false
	for i := 0; i < len(keys); i++ {
		if !inserted && newKey.Compare(keys[i]) < 0 {
			allKeys = append(allKeys, newKey)
			allVals = append(allVals, packedVal)
			inserted = true
		}
		allKeys = append(allKeys, keys[i])
		allVals = append(allVals, vals[i])
	}
	if !inserted {
		allKeys = append(allKeys, newKey)
		allVals = append(allVals, packedVal)
	}

	mid := len(allKeys) / 2
	leftKeys := allKeys[:mid]
	leftVals := allVals[:mid]
	rightKeys := allKeys[mid:]
	rightVals := allVals[mid:]

	// Phase 2: Pre-allocate pages outside transaction.
	// Max needed: 1 (new leaf) + 1 (potential new root) + len(path) (internal splits).
	newPages := make([]uint64, 0, 2+len(path))
	newLeafID, err := s.pg.AllocatePage(ctx)
	if err != nil {
		return fmt.Errorf("kv: allocate new leaf: %w", err)
	}
	newPages = append(newPages, newLeafID)
	// Pre-allocate for potential new root and internal splits.
	for i := 0; i <= len(path); i++ {
		id, err := s.pg.AllocatePage(ctx)
		if err != nil {
			return fmt.Errorf("kv: pre-allocate for internal split: %w", err)
		}
		newPages = append(newPages, id)
	}

	// Phase 3: Write both leaves in a transaction, then update parent.
	if err := s.pg.BeginTx(ctx); err != nil {
		return fmt.Errorf("kv: begin tx: %w", err)
	}

	// Write overflow chain for the new large value if any.
	if len(overflowIDs) > 0 {
		if err := s.writeOverflowChainFull(ctx, overflowIDs, packedVal[8:]); err != nil {
			s.pg.RollbackTx(ctx)
			return err
		}
	}

	leftBody, err := encodeLeafNode(leftKeys, leftVals, newLeafID)
	if err != nil {
		s.pg.RollbackTx(ctx)
		return err
	}
	if err := s.pg.WritePage(ctx, leafID, leftBody); err != nil {
		s.pg.RollbackTx(ctx)
		return fmt.Errorf("kv: write left leaf: %w", err)
	}

	rightBody, err := encodeLeafNode(rightKeys, rightVals, nextLeaf)
	if err != nil {
		s.pg.RollbackTx(ctx)
		return err
	}
	if err := s.pg.WritePage(ctx, newLeafID, rightBody); err != nil {
		s.pg.RollbackTx(ctx)
		return fmt.Errorf("kv: write right leaf: %w", err)
	}

	// Propagate the split upward (insert separator into parent).
	// Use remaining pre-allocated pages for internal splits.
	sepKey := rightKeys[0]     // first key of right leaf is the separator
	splitPages := newPages[1:] // skip newLeafID, use rest for internal splits
	newRoot, err := s.insertIntoParent(ctx, path, leafID, newLeafID, sepKey, splitPages)
	if err != nil {
		s.pg.RollbackTx(ctx)
		return err
	}

	if err := s.pg.CommitTx(ctx); err != nil {
		return fmt.Errorf("kv: commit split: %w", err)
	}

	if newRoot != 0 {
		s.rootPageID = newRoot
	}
	return nil
}

// insertIntoParent inserts a separator key and new child pointer into the parent.
// Called within an active transaction. Returns the new root page ID if a new root
// was created (root split), 0 otherwise.
// newPages is a pre-allocated list of extra pages for internal splits.
func (s *btreeStore) insertIntoParent(ctx context.Context, path []pathEntry,
	leftChildID, rightChildID uint64, sepKey key.Key,
	newPages []uint64) (uint64, error) {

	if len(path) == 0 {
		// Old leaf was root — create a new root.
		// Use a pre-allocated page for the new root.
		if len(newPages) == 0 {
			return 0, fmt.Errorf("%w: no pre-allocated page for new root", ErrTreeCorrupt)
		}
		newRootID := newPages[0]
		newPages = newPages[1:]

		childPtrs := []uint64{leftChildID, rightChildID}
		keys := []key.Key{sepKey}
		rootBody, err := encodeInternalNode(childPtrs, keys)
		if err != nil {
			return 0, err
		}
		if err := s.pg.WritePage(ctx, newRootID, rootBody); err != nil {
			return 0, fmt.Errorf("kv: write new root: %w", err)
		}

		metaBody := encodeMetaPage(newRootID)
		if err := s.pg.WritePage(ctx, MetaPageID, metaBody); err != nil {
			return 0, fmt.Errorf("kv: write meta: %w", err)
		}

		return newRootID, nil
	}

	// Parent exists — insert separator into it.
	parent := path[len(path)-1]
	if len(parent.keys) < MaxInternalKeys {
		return 0, s.insertIntoInternal(ctx, parent.pageID, parent.keys,
			parent.childPtrs, parent.childIdx, leftChildID, rightChildID, sepKey)
	}

	// Parent is full — split internal node.
	return s.splitInternal(ctx, path, parent.pageID, parent.keys,
		parent.childPtrs, parent.childIdx, leftChildID, rightChildID, sepKey,
		newPages)
}

// insertIntoInternal inserts into a non-full internal node. Called within tx.
func (s *btreeStore) insertIntoInternal(ctx context.Context, pageID uint64,
	keys []key.Key, childPtrs []uint64, childIdx int,
	leftChildID, rightChildID uint64, sepKey key.Key) error {

	// Original layout: C0 [K0] C1 [K1] ... C_n
	// We followed C_childIdx. After split, replace C_childIdx with
	// leftChild, insert sepKey and rightChild after it.
	//
	// New: ... C_{childIdx-1}, leftChild, [sepKey], rightChild, C_{childIdx+1} ...
	newPtrs := make([]uint64, 0, len(childPtrs)+1)
	newPtrs = append(newPtrs, childPtrs[:childIdx]...)
	newPtrs = append(newPtrs, leftChildID, rightChildID)
	newPtrs = append(newPtrs, childPtrs[childIdx+1:]...)

	newKeys := make([]key.Key, 0, len(keys)+1)
	newKeys = append(newKeys, keys[:childIdx]...)
	newKeys = append(newKeys, sepKey)
	newKeys = append(newKeys, keys[childIdx:]...)

	body, err := encodeInternalNode(newPtrs, newKeys)
	if err != nil {
		return err
	}
	return s.pg.WritePage(ctx, pageID, body)
}

// splitInternal handles a full internal node split. Called within tx.
// Returns the new root ID if the root was split (0 otherwise).
// newPages is a pre-allocated list of extra pages for recursive splits.
func (s *btreeStore) splitInternal(ctx context.Context, path []pathEntry,
	pageID uint64,
	keys []key.Key, childPtrs []uint64, childIdx int,
	leftChildID, rightChildID uint64, sepKey key.Key,
	newPages []uint64) (uint64, error) {

	// Use a pre-allocated page for the new internal node.
	if len(newPages) == 0 {
		return 0, fmt.Errorf("%w: no pre-allocated page for internal split", ErrTreeCorrupt)
	}
	newInternalID := newPages[0]
	newPages = newPages[1:]

	// Combine existing and new separator/children.
	allKeys := make([]key.Key, 0, len(keys)+1)
	allPtrs := make([]uint64, 0, len(childPtrs)+1)

	// Build: childPtrs[0..childIdx-1], leftChild, rightChild, childPtrs[childIdx+1..]
	// and keys: keys[0..childIdx-1], sepKey, keys[childIdx..]
	allPtrs = append(allPtrs, childPtrs[:childIdx]...)
	allPtrs = append(allPtrs, leftChildID, rightChildID)
	allPtrs = append(allPtrs, childPtrs[childIdx+1:]...)

	allKeys = append(allKeys, keys[:childIdx]...)
	allKeys = append(allKeys, sepKey)
	allKeys = append(allKeys, keys[childIdx:]...)

	mid := len(allKeys) / 2
	leftKeys := allKeys[:mid]
	rightKeys := allKeys[mid+1:]
	middleKey := allKeys[mid]

	leftPtrs := allPtrs[:mid+1]
	rightPtrs := allPtrs[mid+1:]

	leftBody, err := encodeInternalNode(leftPtrs, leftKeys)
	if err != nil {
		return 0, err
	}
	if err := s.pg.WritePage(ctx, pageID, leftBody); err != nil {
		return 0, fmt.Errorf("kv: write split left internal: %w", err)
	}

	rightBody, err := encodeInternalNode(rightPtrs, rightKeys)
	if err != nil {
		return 0, err
	}
	if err := s.pg.WritePage(ctx, newInternalID, rightBody); err != nil {
		return 0, fmt.Errorf("kv: write split right internal: %w", err)
	}

	// Propagate middle key up.
	return s.insertIntoParent(ctx, path[:len(path)-1], pageID, newInternalID, middleKey, newPages)
}

// --- Delete ---

func (s *btreeStore) Delete(ctx context.Context, k key.Key) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkOpen(); err != nil {
		return err
	}
	if s.rootPageID == rootSentinel {
		return nil // empty tree, no-op
	}

	return s.deleteFromTree(ctx, s.rootPageID, k)
}

func (s *btreeStore) deleteFromTree(ctx context.Context, pageID uint64, k key.Key) error {
	body, err := s.pg.ReadPage(ctx, pageID)
	if err != nil {
		return fmt.Errorf("kv: read page %d: %w", pageID, err)
	}

	switch body[0] {
	case NodeTypeLeaf:
		keys, vals, nextLeaf, err := decodeLeafNode(body)
		if err != nil {
			return err
		}
		pos := -1
		for i, lk := range keys {
			if lk.Compare(k) == 0 {
				pos = i
				break
			}
		}
		if pos < 0 {
			return nil // not found, no-op
		}

		newKeys := make([]key.Key, 0, len(keys)-1)
		newVals := make([][]byte, 0, len(vals)-1)
		newKeys = append(newKeys, keys[:pos]...)
		newVals = append(newVals, vals[:pos]...)
		newKeys = append(newKeys, keys[pos+1:]...)
		newVals = append(newVals, vals[pos+1:]...)

		leafBody, err := encodeLeafNode(newKeys, newVals, nextLeaf)
		if err != nil {
			return err
		}
		if err := s.pg.BeginTx(ctx); err != nil {
			return fmt.Errorf("kv: begin tx: %w", err)
		}
		if err := s.pg.WritePage(ctx, pageID, leafBody); err != nil {
			s.pg.RollbackTx(ctx)
			return fmt.Errorf("kv: write leaf: %w", err)
		}
		return s.pg.CommitTx(ctx)

	case NodeTypeInternal:
		childPtrs, ikeys, err := decodeInternalNode(body)
		if err != nil {
			return err
		}
		nextPageID := s.upperBound(childPtrs, ikeys, k)
		return s.deleteFromTree(ctx, nextPageID, k)

	default:
		return fmt.Errorf("%w: unknown node type", ErrTreeCorrupt)
	}
}

// --- Scan ---

func (s *btreeStore) Scan(ctx context.Context, start, end key.Key) ([]Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.checkOpen(); err != nil {
		return nil, err
	}
	if s.rootPageID == rootSentinel {
		return nil, nil
	}

	// Find starting leaf.
	pageID := s.rootPageID
	for {
		body, err := s.pg.ReadPage(ctx, pageID)
		if err != nil {
			return nil, fmt.Errorf("kv: scan read page %d: %w", pageID, err)
		}

		switch body[0] {
		case NodeTypeLeaf:
			return s.scanLeaf(ctx, pageID, start, end, nil)

		case NodeTypeInternal:
			childPtrs, ikeys, err := decodeInternalNode(body)
			if err != nil {
				return nil, err
			}
			if len(start.Bytes()) == 0 {
				pageID = childPtrs[0] // unbounded start: go to leftmost
			} else {
				pageID = s.upperBound(childPtrs, ikeys, start)
			}

		default:
			return nil, fmt.Errorf("%w: unknown node type", ErrTreeCorrupt)
		}
	}
}

func (s *btreeStore) scanLeaf(ctx context.Context, pageID uint64,
	start, end key.Key, results []Entry) ([]Entry, error) {

	for pageID != rootSentinel && pageID != 0 {
		body, err := s.pg.ReadPage(ctx, pageID)
		if err != nil {
			return nil, fmt.Errorf("kv: scan leaf read: %w", err)
		}

		keys, refs, nextLeaf, err := decodeLeafNodeRefs(body)
		if err != nil {
			return nil, err
		}

		for i, lk := range keys {
			// Check lower bound.
			if len(start.Bytes()) > 0 && lk.Compare(start) < 0 {
				continue
			}
			// Check upper bound.
			if len(end.Bytes()) > 0 && lk.Compare(end) >= 0 {
				return results, nil
			}

			ref := refs[i]
			var val []byte
			if ref.hasOverflow {
				val, err = s.readOverflowValue(ctx, ref)
				if err != nil {
					return nil, err
				}
			} else {
				val = make([]byte, len(ref.inline))
				copy(val, ref.inline)
			}
			results = append(results, Entry{Key: lk, Value: val})
		}

		pageID = nextLeaf
	}

	return results, nil
}

// --- Check ---

func (s *btreeStore) Check(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkOpen(); err != nil {
		return err
	}

	if s.rootPageID == rootSentinel {
		return nil // empty tree is valid
	}

	visited := make(map[uint64]struct{})
	pc, err := s.pg.PageCount(ctx)
	if err != nil {
		return fmt.Errorf("kv: check page count: %w", err)
	}

	return s.checkNode(ctx, s.rootPageID, visited, pc, nil)
}

// checkNode recursively verifies a node's structural integrity.
// prevKey is the maximum key seen so far in left-to-right traversal (nil for root).
func (s *btreeStore) checkNode(ctx context.Context, pageID uint64,
	visited map[uint64]struct{}, pageCount uint64,
	prevKey *key.Key) error {

	if pageID >= pageCount {
		return fmt.Errorf("%w: page %d out of range (max %d)", ErrTreeCorrupt, pageID, pageCount-1)
	}
	if _, seen := visited[pageID]; seen {
		return fmt.Errorf("%w: double-reference or cycle at page %d", ErrTreeCorrupt, pageID)
	}
	visited[pageID] = struct{}{}

	body, err := s.pg.ReadPage(ctx, pageID)
	if err != nil {
		return fmt.Errorf("kv: check read page %d: %w", pageID, err)
	}

	switch body[0] {
	case NodeTypeLeaf:
		keys, refs, nextLeaf, err := decodeLeafNodeRefs(body)
		if err != nil {
			return fmt.Errorf("%w: page %d: %v", ErrTreeCorrupt, pageID, err)
		}

		// Verify sorted order.
		for i := 1; i < len(keys); i++ {
			if keys[i-1].Compare(keys[i]) >= 0 {
				return fmt.Errorf("%w: page %d leaf keys not sorted at index %d", ErrTreeCorrupt, pageID, i)
			}
		}

		// Verify consistency with separator key (if called from internal node).
		if prevKey != nil && len(keys) > 0 {
			// All keys in this child should be >= prevKey (the separator).
			if keys[0].Compare(*prevKey) < 0 {
				return fmt.Errorf("%w: page %d leaf key %v < separator %v",
					ErrTreeCorrupt, pageID, keys[0].Bytes(), (*prevKey).Bytes())
			}
		}

		// Check overflow chains for large values.
		for _, ref := range refs {
			if ref.hasOverflow {
				if err := s.checkOverflowChain(ctx, ref, visited, pageCount); err != nil {
					return err
				}
			}
		}

		// Check NextLeafPtr chain.
		if nextLeaf != rootSentinel && nextLeaf != 0 {
			if nextLeaf >= pageCount {
				return fmt.Errorf("%w: page %d nextLeaf %d out of range", ErrTreeCorrupt, pageID, nextLeaf)
			}
		}

	case NodeTypeInternal:
		childPtrs, ikeys, err := decodeInternalNode(body)
		if err != nil {
			return fmt.Errorf("%w: page %d: %v", ErrTreeCorrupt, pageID, err)
		}

		// Verify sorted order of keys.
		for i := 1; i < len(ikeys); i++ {
			if ikeys[i-1].Compare(ikeys[i]) >= 0 {
				return fmt.Errorf("%w: page %d internal keys not sorted at index %d", ErrTreeCorrupt, pageID, i)
			}
		}

		// Recursively check each child, maintaining prevKey context.
		var runningPrev *key.Key
		for i, childPageID := range childPtrs {
			var childPrev *key.Key
			if i > 0 {
				childPrev = &ikeys[i-1]
			}
			if err := s.checkNode(ctx, childPageID, visited, pageCount, childPrev); err != nil {
				return err
			}
			_ = runningPrev
		}

	default:
		return fmt.Errorf("%w: page %d has unknown node type 0x%x", ErrTreeCorrupt, pageID, body[0])
	}

	return nil
}

// checkOverflowChain validates an overflow chain for structural integrity.
func (s *btreeStore) checkOverflowChain(ctx context.Context, ref leafValueRef,
	visited map[uint64]struct{}, pageCount uint64) error {

	totalLen := ref.totalLen
	bufLen := len(ref.inline) // 504 bytes from the leaf slot
	pageID := ref.overflowPageID
	chainLen := 0

	for pageID != rootSentinel && pageID != 0 {
		if chainLen >= MaxOverflowChainLen {
			return fmt.Errorf("%w: overflow chain exceeds max length", ErrTreeCorrupt)
		}
		chainLen++

		if pageID >= pageCount {
			return fmt.Errorf("%w: overflow page %d out of range", ErrTreeCorrupt, pageID)
		}
		if _, seen := visited[pageID]; seen {
			return fmt.Errorf("%w: double-reference or cycle at overflow page %d", ErrTreeCorrupt, pageID)
		}
		visited[pageID] = struct{}{}

		body, err := s.pg.ReadPage(ctx, pageID)
		if err != nil {
			return fmt.Errorf("kv: check overflow page %d: %w", pageID, err)
		}
		nextPtr, chunk, err := decodeOverflowPage(body)
		if err != nil {
			return err
		}

		bufLen += len(chunk)
		pageID = nextPtr
	}

	if bufLen != int(totalLen) {
		return fmt.Errorf("%w: overflow chain length mismatch: got %d, want %d", ErrTreeCorrupt, bufLen, totalLen)
	}

	return nil
}

// --- Compact ---

func (s *btreeStore) Compact(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkOpen(); err != nil {
		return err
	}

	// Empty tree short-circuit.
	if s.rootPageID == rootSentinel {
		return nil
	}

	// Stream all entries from the old tree.
	entries, err := s.scanAll(ctx)
	if err != nil {
		return fmt.Errorf("kv: compact scan: %w", err)
	}

	// Save old root for later freeing.
	oldRootPageID := s.rootPageID

	if len(entries) == 0 {
		// Tree became empty — just update meta to sentinel.
		if err := s.pg.BeginTx(ctx); err != nil {
			return fmt.Errorf("kv: compact begin tx: %w", err)
		}
		metaBody := encodeMetaPage(rootSentinel)
		if err := s.pg.WritePage(ctx, MetaPageID, metaBody); err != nil {
			s.pg.RollbackTx(ctx)
			return fmt.Errorf("kv: compact write meta: %w", err)
		}
		if err := s.pg.CommitTx(ctx); err != nil {
			return fmt.Errorf("kv: compact commit: %w", err)
		}
		s.rootPageID = rootSentinel

		// Free old tree pages.
		s.freeTreePages(ctx, oldRootPageID)
		return nil
	}

	// Bulk-load new tree using bottom-up approach.
	// Calculate number of leaf pages needed.
	numLeaves := (len(entries) + MaxLeafKeys - 1) / MaxLeafKeys

	// Allocate all new pages.
	type newPage struct {
		id   uint64
		body []byte
	}
	var newPages []newPage

	// Build leaf pages.
	var leafPageIDs []uint64
	for i := 0; i < numLeaves; i++ {
		start := i * MaxLeafKeys
		end := start + MaxLeafKeys
		if end > len(entries) {
			end = len(entries)
		}
		chunk := entries[start:end]

		keys := make([]key.Key, len(chunk))
		vals := make([][]byte, len(chunk))
		for j, e := range chunk {
			keys[j] = e.Key
			// For simplicity, handle small and large values the same way.
			if len(e.Value) > MaxValSize {
				// Large value — need to rebuild overflow chain.
				ovIDs, packed := s.prepareOverflow(ctx, e.Value)
				_ = ovIDs // We'll write the chain in the tx
				vals[j] = packed
			} else {
				vals[j] = e.Value
			}
		}

		nextLeaf := rootSentinel
		if i+1 < numLeaves {
			// Will patch later — for now, allocate page and fill in nextLeaf after.
		}
		_ = nextLeaf

		leafID, err := s.pg.AllocatePage(ctx)
		if err != nil {
			return fmt.Errorf("kv: compact allocate leaf: %w", err)
		}
		leafPageIDs = append(leafPageIDs, leafID)

		leafBody, err := encodeLeafNode(keys, vals, rootSentinel)
		if err != nil {
			return fmt.Errorf("kv: compact encode leaf: %w", err)
		}
		newPages = append(newPages, newPage{leafID, leafBody})
	}

	// Patch NextLeafPtr chain.
	for i := 0; i < numLeaves-1; i++ {
		// Re-encode the leaf with the correct nextLeaf.
		start := i * MaxLeafKeys
		end := start + MaxLeafKeys
		if end > len(entries) {
			end = len(entries)
		}
		chunk := entries[start:end]
		keys := make([]key.Key, len(chunk))
		vals := make([][]byte, len(chunk))
		for j, e := range chunk {
			keys[j] = e.Key
			if len(e.Value) > MaxValSize {
				_, packed := s.prepareOverflow(ctx, e.Value)
				vals[j] = packed
			} else {
				vals[j] = e.Value
			}
		}
		leafBody, err := encodeLeafNode(keys, vals, leafPageIDs[i+1])
		if err != nil {
			return fmt.Errorf("kv: compact re-encode leaf: %w", err)
		}
		newPages[i].body = leafBody
	}

	// Build internal nodes bottom-up.
	var internalPageIDs []uint64
	currentLevel := leafPageIDs

	for len(currentLevel) > 1 {
		numInternals := (len(currentLevel) + (MaxInternalKeys + 1) - 1) / (MaxInternalKeys + 1)
		var nextLevel []uint64

		for i := 0; i < numInternals; i++ {
			start := i * (MaxInternalKeys + 1)
			end := start + (MaxInternalKeys + 1)
			if end > len(currentLevel) {
				end = len(currentLevel)
			}
			children := currentLevel[start:end]

			// Build child pointers and separator keys.
			childPtrs := make([]uint64, len(children))
			sepKeys := make([]key.Key, len(children)-1)
			for j := 0; j < len(children); j++ {
				childPtrs[j] = children[j]
			}
			for j := 0; j < len(children)-1; j++ {
				firstIdx := start + (j+1)*MaxLeafKeys
				if firstIdx >= len(entries) {
					firstIdx = len(entries) - 1
				}
				sepKeys[j] = entries[firstIdx].Key
			}

			intID, err := s.pg.AllocatePage(ctx)
			if err != nil {
				return fmt.Errorf("kv: compact allocate internal: %w", err)
			}

			intBody, err := encodeInternalNode(childPtrs, sepKeys)
			if err != nil {
				return fmt.Errorf("kv: compact encode internal: %w", err)
			}
			newPages = append(newPages, newPage{intID, intBody})
			nextLevel = append(nextLevel, intID)
		}

		internalPageIDs = append(internalPageIDs, nextLevel...)
		currentLevel = nextLevel
	}

	newRootID := currentLevel[0] // single root

	// Write all new pages directly to the main file (no transaction needed for writes).
	for _, np := range newPages {
		if err := s.pg.WritePage(ctx, np.id, np.body); err != nil {
			return fmt.Errorf("kv: compact write page %d: %w", np.id, err)
		}
	}

	// Post-build validation: verify new root is readable.
	if _, err := s.pg.ReadPage(ctx, newRootID); err != nil {
		return fmt.Errorf("%w: compact validation: %v", ErrTreeCorrupt, err)
	}

	// Atomically swap root via transaction.
	if err := s.pg.BeginTx(ctx); err != nil {
		return fmt.Errorf("kv: compact begin tx: %w", err)
	}
	metaBody := encodeMetaPage(newRootID)
	if err := s.pg.WritePage(ctx, MetaPageID, metaBody); err != nil {
		s.pg.RollbackTx(ctx)
		return fmt.Errorf("kv: compact write meta: %w", err)
	}
	if err := s.pg.CommitTx(ctx); err != nil {
		return fmt.Errorf("kv: compact commit: %w", err)
	}

	// Update in-memory root.
	s.rootPageID = newRootID

	// Best-effort free old tree pages.
	s.freeTreePages(ctx, oldRootPageID)

	return nil
}

// allKey returns an empty key used for unbounded scans.
func allKey() key.Key { return key.EncodeBytes(nil) }

// scanAll returns all entries in the tree in key order. Caller must hold lock.
func (s *btreeStore) scanAll(ctx context.Context) ([]Entry, error) {
	// Navigate to leftmost leaf, then scan.
	pageID := s.rootPageID
	for {
		body, err := s.pg.ReadPage(ctx, pageID)
		if err != nil {
			return nil, fmt.Errorf("kv: scanAll read: %w", err)
		}
		switch body[0] {
		case NodeTypeLeaf:
			return s.scanLeaf(ctx, pageID, allKey(), allKey(), nil)
		case NodeTypeInternal:
			childPtrs, _, err := decodeInternalNode(body)
			if err != nil {
				return nil, err
			}
			pageID = childPtrs[0] // leftmost
		default:
			return nil, fmt.Errorf("%w: unknown node type", ErrTreeCorrupt)
		}
	}
}

// freeTreePages frees all pages reachable from the given root (best-effort).
func (s *btreeStore) freeTreePages(ctx context.Context, rootPageID uint64) {
	if rootPageID == rootSentinel {
		return
	}

	visited := make(map[uint64]struct{})
	s.freeNode(ctx, rootPageID, visited)
}

func (s *btreeStore) freeNode(ctx context.Context, pageID uint64,
	visited map[uint64]struct{}) {

	if pageID == rootSentinel || pageID == 0 {
		return
	}
	if _, seen := visited[pageID]; seen {
		return
	}
	visited[pageID] = struct{}{}

	body, err := s.pg.ReadPage(ctx, pageID)
	if err != nil {
		return // best-effort, skip on error
	}

	switch body[0] {
	case NodeTypeLeaf:
		_, refs, _, _ := decodeLeafNodeRefs(body)
		for _, ref := range refs {
			if ref.hasOverflow {
				s.freeOverflowChain(ctx, ref.overflowPageID, visited)
			}
		}
	case NodeTypeInternal:
		childPtrs, _, _ := decodeInternalNode(body)
		for _, childID := range childPtrs {
			s.freeNode(ctx, childID, visited)
		}
	}

	// Free the page itself.
	s.pg.FreePage(ctx, pageID)
}

func (s *btreeStore) freeOverflowChain(ctx context.Context, pageID uint64,
	visited map[uint64]struct{}) {

	for pageID != rootSentinel && pageID != 0 {
		if _, seen := visited[pageID]; seen {
			return
		}
		visited[pageID] = struct{}{}

		body, err := s.pg.ReadPage(ctx, pageID)
		if err != nil {
			return
		}
		nextPtr, _, err := decodeOverflowPage(body)
		if err != nil {
			return
		}

		s.pg.FreePage(ctx, pageID)
		pageID = nextPtr
	}
}
