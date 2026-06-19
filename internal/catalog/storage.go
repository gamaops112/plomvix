// Package catalog provides the Global System Catalog.
// storage.go defines the storage abstraction the catalog uses to persist
// metadata. It decouples the catalog from concrete heap implementations.
package catalog

import (
	"context"
	"errors"
)

// SystemTable abstracts KV-like storage for catalog metadata.
// Implementations map this contract onto the underlying storage engine
// (e.g., MVCC Heap via SystemHeapAdapter).
type SystemTable interface {
	// Get retrieves the latest visible value for key. Returns (nil, ErrNotFound)
	// if the key does not exist or has been tombstoned.
	Get(ctx context.Context, key []byte) ([]byte, error)
	// Put stores a new value for key, creating a new MVCC version.
	Put(ctx context.Context, key, value []byte) error
	// Delete tombstones the key so future Get calls return ErrNotFound.
	Delete(ctx context.Context, key []byte) error
	// Scan iterates all live (non-tombstoned) key-value pairs, calling fn
	// for each. fn must not retain references to k or v after returning.
	Scan(ctx context.Context, fn func(k, v []byte) error) error
}

// Sentinel errors for SystemTable operations.
var (
	ErrNotFound = errors.New("catalog: key not found")
)
