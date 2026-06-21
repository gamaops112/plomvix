// Package logs provides a pluggable logs engine for Plomvix.
// index.go implements a concurrent in-memory inverted token index
// with LRU eviction for memory-bounded log text search.
package logs

import (
	"container/list"
	"strings"
	"sync"
)

// RecordLocator identifies a specific log record within a page.
type RecordLocator struct {
	PageID     uint64
	RecordIdx  uint32 // index within the page's record list
	Timestamp  int64
}

// TokenIndex is a memory-bounded inverted index mapping tokens to
// record locations. It enforces a memory cap via LRU key eviction.
type TokenIndex struct {
	mu          sync.RWMutex
	termLocs    map[string][]RecordLocator
	lruList     *list.List
	lruMap      map[string]*list.Element
	maxMemBytes int64
	curMemBytes int64
}

// DefaultTokenIndexConfig returns sensible defaults for the token index.
func DefaultTokenIndexConfig() TokenIndexConfig {
	return TokenIndexConfig{
		MaxMemoryMB: 64,
	}
}

// TokenIndexConfig holds configuration for the token index.
type TokenIndexConfig struct {
	MaxMemoryMB int
}

// NewTokenIndex creates a new memory-bounded inverted token index.
func NewTokenIndex(maxMemBytes int64) *TokenIndex {
	return &TokenIndex{
		termLocs:    make(map[string][]RecordLocator),
		lruList:     list.New(),
		lruMap:      make(map[string]*list.Element),
		maxMemBytes: maxMemBytes,
	}
}

// Insert adds a token → locator mapping. If the index is at capacity,
// the oldest LRU entry is evicted before insertion. If still over
// capacity after eviction, the insertion is silently dropped.
func (idx *TokenIndex) Insert(token string, loc RecordLocator) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	term := strings.ToLower(token)
	locs := idx.termLocs[term]

	// Estimate added size overhead: 16 bytes per RecordLocator entry.
	addedSize := int64(16)
	if len(locs) == 0 {
		// Track key string overhead (approx. length + map overhead).
		addedSize += int64(len(term)) + 64
	}

	// Enforce memory boundary by evicting oldest keys.
	for idx.curMemBytes+addedSize >= idx.maxMemBytes && idx.lruList.Len() > 0 {
		idx.evictOldest()
	}

	// Graceful degradation: if still over limit, drop indexing.
	if idx.curMemBytes+addedSize >= idx.maxMemBytes {
		return
	}

	idx.termLocs[term] = append(locs, loc)
	idx.curMemBytes += addedSize

	// Update LRU ordering.
	if elem, ok := idx.lruMap[term]; ok {
		idx.lruList.MoveToFront(elem)
	} else {
		elem := idx.lruList.PushFront(term)
		idx.lruMap[term] = elem
	}
}

// evictOldest removes the least-recently-used term from the index.
func (idx *TokenIndex) evictOldest() {
	elem := idx.lruList.Back()
	if elem == nil {
		return
	}
	term := elem.Value.(string)
	locs := idx.termLocs[term]

	freedSize := int64(len(locs))*16 + int64(len(term)) + 64
	delete(idx.termLocs, term)
	delete(idx.lruMap, term)
	idx.lruList.Remove(elem)

	idx.curMemBytes -= freedSize
	if idx.curMemBytes < 0 {
		idx.curMemBytes = 0
	}
}

// Search returns all record locators for the given token.
// Returns nil if the token is not indexed.
func (idx *TokenIndex) Search(token string) []RecordLocator {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	term := strings.ToLower(token)
	locs, exists := idx.termLocs[term]
	if !exists {
		return nil
	}
	copied := make([]RecordLocator, len(locs))
	copy(copied, locs)
	return copied
}

// SearchAll returns the intersection of record locators across all tokens.
// Returns nil if any token is not found (AND semantics).
func (idx *TokenIndex) SearchAll(tokens []string) []RecordLocator {
	if len(tokens) == 0 {
		return nil
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// Get locator sets for each token.
	sets := make([][]RecordLocator, len(tokens))
	for i, tok := range tokens {
		term := strings.ToLower(tok)
		locs, exists := idx.termLocs[term]
		if !exists {
			return nil
		}
		sets[i] = locs
	}

	// AND-intersection: use the smallest set as the base.
	result := intersectLocators(sets)
	copied := make([]RecordLocator, len(result))
	copy(copied, result)
	return copied
}

// intersectLocators returns the intersection of multiple locator slices.
func intersectLocators(sets [][]RecordLocator) []RecordLocator {
	if len(sets) == 0 {
		return nil
	}
	if len(sets) == 1 {
		return sets[0]
	}

	// Build a set from the first slice for quick lookup.
	first := make(map[RecordLocator]struct{}, len(sets[0]))
	for _, loc := range sets[0] {
		first[loc] = struct{}{}
	}

	var result []RecordLocator
	for _, loc := range sets[1] {
		if _, ok := first[loc]; ok {
			result = append(result, loc)
		}
	}

	if len(sets) == 2 {
		return result
	}

	// Continue intersecting with remaining sets.
	for i := 2; i < len(sets); i++ {
		cur := make(map[RecordLocator]struct{}, len(sets[i]))
		for _, loc := range sets[i] {
			cur[loc] = struct{}{}
		}
		var next []RecordLocator
		for _, loc := range result {
			if _, ok := cur[loc]; ok {
				next = append(next, loc)
			}
		}
		result = next
	}
	return result
}

// Sweep removes entries older than the given timestamp.
func (idx *TokenIndex) Sweep(retentionWindow int64) {
	// This is a lightweight sweep — the real retention is page-level.
	// We just remove locators pointing to expired timestamps.
	idx.mu.Lock()
	defer idx.mu.Unlock()

	for term, locs := range idx.termLocs {
		var kept []RecordLocator
		for _, loc := range locs {
			if loc.Timestamp >= retentionWindow {
				kept = append(kept, loc)
			}
		}
		if len(kept) == 0 {
			// Evict the term entirely.
			delete(idx.termLocs, term)
			if elem, ok := idx.lruMap[term]; ok {
				idx.lruList.Remove(elem)
				delete(idx.lruMap, term)
			}
		} else {
			idx.termLocs[term] = kept
		}
	}
}
