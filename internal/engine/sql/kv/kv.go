// Package kv provides a durable, ordered []byte->[]byte key/value store
// for the Plomvix sql_engine. It is key-format-agnostic and exposes
// Get, Set, Delete, ordered Scan/ScanReverse, atomic Batch operations,
// read Snapshots, Stats, and Check diagnostics.
package kv

import (
	"context"
	"errors"
)

type KVStore interface {
	Name() string
	Open(ctx context.Context) error
	Close(ctx context.Context) error
	Get(ctx context.Context, key []byte) (value []byte, found bool, err error)
	Set(ctx context.Context, key, value []byte) error
	Delete(ctx context.Context, key []byte) error
	Scan(ctx context.Context, start, end []byte, fn func(key, value []byte) error) error
	NewBatch() Batch
	ScanReverse(ctx context.Context, start, end []byte, fn func(key, value []byte) error) error
	NewSnapshot(ctx context.Context) (Snapshot, error)
	Stats(ctx context.Context) (Stats, error)
	Check(ctx context.Context) error
}

type Batch interface {
	Set(key, value []byte)
	Delete(key []byte)
	Commit(ctx context.Context) error
	Reset()
}

type Snapshot interface {
	Get(ctx context.Context, key []byte) (value []byte, found bool, err error)
	Scan(ctx context.Context, start, end []byte, fn func(key, value []byte) error) error
	ScanReverse(ctx context.Context, start, end []byte, fn func(key, value []byte) error) error
	Close() error
}

type Stats struct {
	Backend    string
	KeyCount   int64
	SizeBytes  int64
	ReadOnly   bool
	SyncWrites bool
}

type Options struct {
	Backend      string
	Path         string
	SyncWrites   bool
	ReadOnly     bool
	CacheSizeMB  int
	MaxOpenFiles int
}

var (
	ErrNotOpen        = errors.New("kv: store not open")
	ErrAlreadyOpen    = errors.New("kv: store already open")
	ErrClosed         = errors.New("kv: store is closed")
	ErrEmptyKey       = errors.New("kv: key must not be empty")
	ErrNilCallback    = errors.New("kv: scan callback must not be nil")
	ErrReadOnly       = errors.New("kv: store is read-only")
	ErrSnapshotClosed = errors.New("kv: snapshot is closed")
	ErrSnapshotActive = errors.New("kv: snapshot is active")
	ErrUnknownBackend = errors.New("kv: unknown backend")
)

// New creates a KVStore based on opts.Backend. "bbolt" delegates to
// newBBoltWithOptions; "pebble" delegates to NewPebble.
func New(name string, opts Options) (KVStore, error) {
	switch opts.Backend {
	case "bbolt":
		return newBBoltWithOptions(name, opts.Path, opts), nil
	case "pebble":
		return NewPebble(name, opts.Path, opts)
	default:
		return nil, ErrUnknownBackend
	}
}
