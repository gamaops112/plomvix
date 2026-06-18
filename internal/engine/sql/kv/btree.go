package kv

import (
	"context"
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
// Returns ErrKeyNotFound if not found.
func (s *btreeStore) search(ctx context.Context, pageID uint64, k key.Key) ([]byte, error) {
	for {
		body, err := s.pg.ReadPage(ctx, pageID)
		if err != nil {
			return nil, fmt.Errorf("kv: read page %d: %w", pageID, err)
		}

		switch body[0] {
		case NodeTypeLeaf:
			keys, vals, _, err := decodeLeafNode(body)
			if err != nil {
				return nil, err
			}
			for i, lk := range keys {
				if lk.Compare(k) == 0 {
					cp := make([]byte, len(vals[i]))
					copy(cp, vals[i])
					return cp, nil
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
	if len(v) > MaxValSize {
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
	// Phase 2: Allocate leaf page outside transaction.
	leafID, err := s.pg.AllocatePage(ctx)
	if err != nil {
		return fmt.Errorf("kv: allocate leaf: %w", err)
	}

	// Phase 3: Write leaf and meta in a transaction.
	if err := s.pg.BeginTx(ctx); err != nil {
		return fmt.Errorf("kv: begin tx: %w", err)
	}

	leafBody, err := encodeLeafNode([]key.Key{k}, [][]byte{v}, rootSentinel)
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

// setInTree inserts/updates k=v into a non-empty tree.
func (s *btreeStore) setInTree(ctx context.Context, k key.Key, v []byte) error {
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
					return s.updateLeaf(ctx, pageID, body, keys, vals, i, k, v, nextLeaf)
				}
			}

			// Key not found — insert.
			if len(keys) < MaxLeafKeys {
				return s.insertIntoLeaf(ctx, pageID, keys, vals, k, v, nextLeaf)
			}

			// Leaf is full — split needed.
			return s.splitLeaf(ctx, pageID, path, keys, vals, k, v, nextLeaf)

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
func (s *btreeStore) updateLeaf(ctx context.Context, pageID uint64, _ []byte, keys []key.Key, vals [][]byte, pos int, k key.Key, v []byte, nextLeaf uint64) error {
	vals[pos] = v
	body, err := encodeLeafNode(keys, vals, nextLeaf)
	if err != nil {
		return err
	}
	if err := s.pg.BeginTx(ctx); err != nil {
		return fmt.Errorf("kv: begin tx: %w", err)
	}
	if err := s.pg.WritePage(ctx, pageID, body); err != nil {
		s.pg.RollbackTx(ctx)
		return fmt.Errorf("kv: write leaf: %w", err)
	}
	return s.pg.CommitTx(ctx)
}

// insertIntoLeaf inserts k=v into a leaf that has room. Simple insert into sorted position.
func (s *btreeStore) insertIntoLeaf(ctx context.Context, pageID uint64, keys []key.Key, vals [][]byte, k key.Key, v []byte, nextLeaf uint64) error {
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
	newVals = append(newVals, v)
	newKeys = append(newKeys, keys[pos:]...)
	newVals = append(newVals, vals[pos:]...)

	body, err := encodeLeafNode(newKeys, newVals, nextLeaf)
	if err != nil {
		return err
	}
	if err := s.pg.BeginTx(ctx); err != nil {
		return fmt.Errorf("kv: begin tx: %w", err)
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
	keys []key.Key, vals [][]byte, newKey key.Key, newVal []byte,
	nextLeaf uint64) error {

	// Combine existing and new key-value pairs in sorted order.
	allKeys := make([]key.Key, 0, len(keys)+1)
	allVals := make([][]byte, 0, len(vals)+1)
	inserted := false
	for i := 0; i < len(keys); i++ {
		if !inserted && newKey.Compare(keys[i]) < 0 {
			allKeys = append(allKeys, newKey)
			allVals = append(allVals, newVal)
			inserted = true
		}
		allKeys = append(allKeys, keys[i])
		allVals = append(allVals, vals[i])
	}
	if !inserted {
		allKeys = append(allKeys, newKey)
		allVals = append(allVals, newVal)
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

		keys, vals, nextLeaf, err := decodeLeafNode(body)
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
			cpVal := make([]byte, len(vals[i]))
			copy(cpVal, vals[i])
			results = append(results, Entry{Key: lk, Value: cpVal})
		}

		pageID = nextLeaf
	}

	return results, nil
}
