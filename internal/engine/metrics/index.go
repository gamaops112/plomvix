// Package metrics provides a time-series metrics engine for Plomvix.
// index.go implements a concurrent in-memory inverted tag index that
// maps tag key-value pairs to page/offset record locators, avoiding
// full table scans for tag-filtered queries.
package metrics

import (
	"sync"
	"time"
)

// RecordLocator points to a specific metric record on disk.
type RecordLocator struct {
	PageID    uint64
	Offset    uint32
	Timestamp int64 // for LRU eviction by age
}

// TagIndex is a concurrent in-memory inverted index mapping
// tag key → tag value → list of record locators.
//
// Concurrency: Insert acquires write lock; Search acquires read lock.
// Memory bounding: records older than retentionWindow are evicted
// on Insert (best-effort; a background sweep runs periodically).
type TagIndex struct {
	mu              sync.RWMutex
	index           map[string]map[string][]RecordLocator // key → value → locators
	retentionWindow time.Duration
	lastSweep       time.Time
}

// TagIndexConfig configures the tag index.
type TagIndexConfig struct {
	RetentionWindow time.Duration // how long to retain entries
}

// DefaultTagIndexConfig returns sensible defaults.
func DefaultTagIndexConfig() TagIndexConfig {
	return TagIndexConfig{
		RetentionWindow: 24 * time.Hour,
	}
}

// NewTagIndex creates a new inverted tag index.
func NewTagIndex(cfg TagIndexConfig) *TagIndex {
	return &TagIndex{
		index:           make(map[string]map[string][]RecordLocator),
		retentionWindow: cfg.RetentionWindow,
		lastSweep:       time.Now(),
	}
}

// Insert adds a record locator for each tag key-value pair found in tagsStr.
func (idx *TagIndex) Insert(tagsStr string, loc RecordLocator) {
	pairs := splitTagPairs(tagsStr)
	if len(pairs) == 0 {
		return
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	for k, v := range pairs {
		vals, exists := idx.index[k]
		if !exists {
			vals = make(map[string][]RecordLocator)
			idx.index[k] = vals
		}
		vals[v] = append(vals[v], loc)
	}
}

// Search returns record locators matching a single tag key-value pair.
// Returns nil if not found. The returned slice is a deep copy.
func (idx *TagIndex) Search(key, val string) []RecordLocator {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	vals, exists := idx.index[key]
	if !exists {
		return nil
	}
	locs := vals[val]
	if len(locs) == 0 {
		return nil
	}
	copied := make([]RecordLocator, len(locs))
	copy(copied, locs)
	return copied
}

// SearchAll returns the intersection of locator sets for all given tag
// constraints. For each key=value pair, the matching locators are
// intersected (AND logic). If constraints is empty, returns nil.
func (idx *TagIndex) SearchAll(constraints map[string]string) []RecordLocator {
	if len(constraints) == 0 {
		return nil
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var result []RecordLocator
	first := true
	for k, v := range constraints {
		vals, exists := idx.index[k]
		if !exists {
			return nil
		}
		locs := vals[v]
		if len(locs) == 0 {
			return nil
		}
		if first {
			result = make([]RecordLocator, len(locs))
			copy(result, locs)
			first = false
		} else {
			result = intersectLocs(result, locs)
			if len(result) == 0 {
				return nil
			}
		}
	}
	return result
}

// Sweep removes entries older than the retention window. Called
// periodically by the rollup manager or on-demand.
func (idx *TagIndex) Sweep(now time.Time) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	cutoff := now.Add(-idx.retentionWindow).Unix()
	for key, vals := range idx.index {
		for val, locs := range vals {
			kept := locs[:0]
			for _, loc := range locs {
				if loc.Timestamp >= cutoff {
					kept = append(kept, loc)
				}
			}
			if len(kept) == 0 {
				delete(vals, val)
			} else {
				vals[val] = kept
			}
		}
		if len(vals) == 0 {
			delete(idx.index, key)
		}
	}
	idx.lastSweep = now
}

// Stats returns approximate index statistics.
func (idx *TagIndex) Stats() (numKeys, numEntries int) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	numKeys = len(idx.index)
	for _, vals := range idx.index {
		for _, locs := range vals {
			numEntries += len(locs)
		}
	}
	return
}

// intersectLocs returns the intersection of two locator slices.
// Both slices are assumed sorted by (PageID, Offset).
func intersectLocs(a, b []RecordLocator) []RecordLocator {
	var result []RecordLocator
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if a[i].PageID < b[j].PageID || (a[i].PageID == b[j].PageID && a[i].Offset < b[j].Offset) {
			i++
		} else if a[i].PageID > b[j].PageID || (a[i].PageID == b[j].PageID && a[i].Offset > b[j].Offset) {
			j++
		} else {
			result = append(result, a[i])
			i++
			j++
		}
	}
	return result
}
