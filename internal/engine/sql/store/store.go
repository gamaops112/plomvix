// Package store provides an ordered, in-memory key-value store built on
// the internal/engine/sql/key package for key ordering. It supports Put,
// Get, Delete, and half-open range Scan with copy-safety at all boundaries.
package store

import (
	"errors"
	"sort"
	"sync"

	"github.com/plomvix/plomvix/internal/engine/sql/key"
)

// Entry is a key-value pair returned by Scan.
type Entry struct {
	Key   key.Key
	Value []byte
}

type entry struct {
	key   key.Key
	value []byte
}

// Store is an ordered, in-memory key-value store safe for concurrent use.
type Store struct {
	mu      sync.RWMutex
	entries []entry
}

// Sentinel errors.
var (
	ErrNotFound = errors.New("sql/store: key not found")
	ErrNilStore = errors.New("sql/store: nil store")
)

// New returns an empty, initialized Store.
func New() *Store { return &Store{} }

// search returns the index where k is found (found=true) or should be
// inserted (found=false). Caller must hold the appropriate lock.
func (s *Store) search(k key.Key) (int, bool) {
	idx := sort.Search(len(s.entries), func(i int) bool {
		return s.entries[i].key.Compare(k) >= 0
	})
	if idx < len(s.entries) && s.entries[idx].key.Compare(k) == 0 {
		return idx, true
	}
	return idx, false
}

// Put stores value for k. Existing entries are overwritten. Value is copied.
func (s *Store) Put(k key.Key, value []byte) error {
	if s == nil {
		return ErrNilStore
	}

	stored := make([]byte, len(value))
	copy(stored, value)

	s.mu.Lock()
	defer s.mu.Unlock()

	idx, found := s.search(k)
	if found {
		s.entries[idx].value = stored
		return nil
	}
	s.entries = append(s.entries, entry{})
	copy(s.entries[idx+1:], s.entries[idx:])
	s.entries[idx] = entry{key: k, value: stored}
	return nil
}

// Get returns a copy of the value for k, or ErrNotFound if absent.
func (s *Store) Get(k key.Key) ([]byte, error) {
	if s == nil {
		return nil, ErrNilStore
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	_, found := s.search(k)
	if !found {
		return nil, ErrNotFound
	}
	// search only returns the index; we need the value.
	// Re-find the index since we can't get it from the bool-only found.
	idx := sort.Search(len(s.entries), func(i int) bool {
		return s.entries[i].key.Compare(k) >= 0
	})
	v := make([]byte, len(s.entries[idx].value))
	copy(v, s.entries[idx].value)
	return v, nil
}

// Delete removes k. Deleting a non-existent key is a no-op returning nil.
func (s *Store) Delete(k key.Key) error {
	if s == nil {
		return ErrNilStore
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	idx, found := s.search(k)
	if !found {
		return nil
	}
	s.entries = append(s.entries[:idx], s.entries[idx+1:]...)
	return nil
}

// Len returns the number of entries currently in the store.
func (s *Store) Len() int {
	if s == nil {
		return 0
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

// Scan returns all entries with keys in [start, end). If start >= end,
// an empty slice is returned. Results are copies and safe to mutate.
func (s *Store) Scan(start, end key.Key) ([]Entry, error) {
	if s == nil {
		return nil, ErrNilStore
	}
	if start.Compare(end) >= 0 {
		return []Entry{}, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	startIdx := sort.Search(len(s.entries), func(i int) bool {
		return s.entries[i].key.Compare(start) >= 0
	})

	var result []Entry
	for i := startIdx; i < len(s.entries); i++ {
		if s.entries[i].key.Compare(end) >= 0 {
			break
		}
		vc := make([]byte, len(s.entries[i].value))
		copy(vc, s.entries[i].value)
		result = append(result, Entry{Key: s.entries[i].key, Value: vc})
	}
	return result, nil
}
