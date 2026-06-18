// Package kv provides an on-disk, ordered key-value store backed by a
// page-based B+ Tree built on the hardened pager (storage/enterprise tier).
// It implements the same Get/Set/Delete/Scan contract as the in-memory store
// but stores data durably on disk.
//
// This is the Basic tier — leaf-only deletes, no node merging, and a known
// trade-off where page allocations during splits happen outside the KV
// transaction (leaked pages on crash, but tree integrity preserved).
package kv

import (
	"context"
	"errors"

	"github.com/plomvix/plomvix/internal/engine/sql/key"
	"github.com/plomvix/plomvix/internal/storage/pager"
)

// Constants
const (
	MaxKeySize = 63
	MaxValSize = 512

	// Derived from pager.PageSize (4096) - 12 bytes page header = 4084 bytes body.
	// Node Header = 1 (Type) + 2 (NumKeys) + 8 (NextLeaf/ChildPtr0) = 11 bytes.
	// Available for slots = 4084 - 11 = 4073 bytes.
	// Leaf Slot = 1 (KeyLen) + 63 (Key) + 4 (ValLen) + 512 (Val) = 580 bytes.
	// MaxLeafKeys = floor(4073 / 580) = 7.
	// Internal Slot = 1 (KeyLen) + 63 (Key) + 8 (ChildPtr) = 72 bytes.
	// MaxInternalKeys = floor(4073 / 72) = 56.
	MaxLeafKeys      = 7
	MaxInternalKeys  = 56
	LeafSlotSize     = 580
	InternalSlotSize = 72

	NodeTypeLeaf     byte = 0x01
	NodeTypeInternal byte = 0x02
	NodeTypeMeta     byte = 0x03
)

// MetaPageID is the permanently reserved page for the KVStore meta page.
// Page 2 is the first allocatable data page (pages 0 and 1 are headers).
const MetaPageID = pager.FirstDataPageID

// rootSentinel represents "no root" (empty tree).
const rootSentinel uint64 = 0xFFFFFFFFFFFFFFFF

// Entry is a key-value pair returned by Scan.
type Entry struct {
	Key   key.Key
	Value []byte
}

// KVStore is the on-disk ordered key-value store interface.
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

// Sentinel errors.
var (
	ErrKeyNotFound   = errors.New("kv: key not found")
	ErrKeyTooLarge   = errors.New("kv: key exceeds maximum size")
	ErrValueTooLarge = errors.New("kv: value exceeds maximum size")
	ErrTreeCorrupt   = errors.New("kv: B+ tree structure is corrupt")
	ErrNotOpen       = errors.New("kv: store is not open")
	ErrClosed        = errors.New("kv: store is closed")
)

// New creates a new KVStore backed by the given pager. Does NOT call
// pager.Open or allocate pages.
func New(p pager.Pager) KVStore {
	return newBtreeStore(p)
}

// compile-time interface check
var _ KVStore = (*btreeStore)(nil)
