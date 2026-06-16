package kv

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/cockroachdb/pebble"
)

type pebbleStore struct {
	mu         sync.RWMutex
	name       string
	path       string
	state      storeState
	db         *pebble.DB
	readOnly   bool
	syncWrites bool
	snapCount  int64
}

// NewPebble creates a KVStore backed by Pebble.
func NewPebble(name, path string, opts Options) (KVStore, error) {
	return &pebbleStore{
		name: name, path: path, state: stateNeverOpened,
		readOnly: opts.ReadOnly, syncWrites: opts.SyncWrites,
	}, nil
}

func (s *pebbleStore) Name() string { return s.name }

func (s *pebbleStore) Open(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	switch s.state {
	case stateOpen:
		return ErrAlreadyOpen
	case stateClosed:
		return ErrClosed
	}

	if s.readOnly {
		db, err := pebble.Open(s.path, &pebble.Options{ReadOnly: true})
		if err != nil {
			return fmt.Errorf("kv: open pebble read-only: %w", err)
		}
		s.db = db
		s.state = stateOpen
		return nil
	}

	if err := os.MkdirAll(s.path, 0700); err != nil {
		return fmt.Errorf("kv: create directory %s: %w", s.path, err)
	}
	db, err := pebble.Open(s.path, &pebble.Options{})
	if err != nil {
		return fmt.Errorf("kv: open pebble: %w", err)
	}
	s.db = db
	s.state = stateOpen
	return nil
}

func (s *pebbleStore) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch s.state {
	case stateNeverOpened:
		return ErrNotOpen
	case stateClosed:
		return ErrClosed
	}
	if atomic.LoadInt64(&s.snapCount) > 0 {
		return ErrSnapshotActive
	}
	if s.db != nil {
		err := s.db.Close()
		s.db = nil
		s.state = stateClosed
		return err
	}
	s.state = stateClosed
	return nil
}

func (s *pebbleStore) Get(ctx context.Context, key []byte) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if len(key) == 0 {
		return nil, false, ErrEmptyKey
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.state != stateOpen {
		return nil, false, s.stateError()
	}
	v, closer, err := s.db.Get(key)
	if err == pebble.ErrNotFound {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	val := make([]byte, len(v))
	copy(val, v)
	closer.Close()
	return val, true, nil
}

func (s *pebbleStore) Set(ctx context.Context, key, value []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(key) == 0 {
		return ErrEmptyKey
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.state != stateOpen {
		return s.stateError()
	}
	if s.readOnly {
		return ErrReadOnly
	}
	opts := pebble.NoSync
	if s.syncWrites {
		opts = pebble.Sync
	}
	return s.db.Set(key, value, opts)
}

func (s *pebbleStore) Delete(ctx context.Context, key []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(key) == 0 {
		return ErrEmptyKey
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.state != stateOpen {
		return s.stateError()
	}
	if s.readOnly {
		return ErrReadOnly
	}
	opts := pebble.NoSync
	if s.syncWrites {
		opts = pebble.Sync
	}
	return s.db.Delete(key, opts)
}

func (s *pebbleStore) Scan(ctx context.Context, start, end []byte, fn func(key, value []byte) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if fn == nil {
		return ErrNilCallback
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.state != stateOpen {
		return s.stateError()
	}

	iter, _ := s.db.NewIter(nil)
	defer iter.Close()
	if start == nil {
		iter.First()
	} else {
		iter.SeekGE(start)
	}
	for iter.Valid() && (end == nil || bytes.Compare(iter.Key(), end) < 0) {
		if err := ctx.Err(); err != nil {
			return err
		}
		kc := make([]byte, len(iter.Key()))
		copy(kc, iter.Key())
		vc := make([]byte, len(iter.Value()))
		copy(vc, iter.Value())
		if err := fn(kc, vc); err != nil {
			return err
		}
		iter.Next()
	}
	return iter.Error()
}

func (s *pebbleStore) NewBatch() Batch { return &pebbleBatch{store: s} }

type pebbleBatch struct {
	store *pebbleStore
	ops   []batchOp
}

func (b *pebbleBatch) Set(key, value []byte) {
	kc := make([]byte, len(key))
	copy(kc, key)
	vc := make([]byte, len(value))
	copy(vc, value)
	b.ops = append(b.ops, batchOp{kind: 0, key: kc, value: vc})
}
func (b *pebbleBatch) Delete(key []byte) {
	kc := make([]byte, len(key))
	copy(kc, key)
	b.ops = append(b.ops, batchOp{kind: 1, key: kc})
}
func (b *pebbleBatch) Commit(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.store.mu.RLock()
	defer b.store.mu.RUnlock()
	if b.store.state != stateOpen {
		return b.store.stateError()
	}
	if b.store.readOnly && len(b.ops) > 0 {
		b.ops = nil
		return ErrReadOnly
	}
	for _, op := range b.ops {
		if len(op.key) == 0 {
			b.ops = nil
			return ErrEmptyKey
		}
	}
	if len(b.ops) == 0 {
		return nil
	}
	pb := b.store.db.NewBatch()
	for _, op := range b.ops {
		if op.kind == 0 {
			pb.Set(op.key, op.value, nil)
		} else {
			pb.Delete(op.key, nil)
		}
	}
	opts := pebble.NoSync
	if b.store.syncWrites {
		opts = pebble.Sync
	}
	if err := pb.Commit(opts); err != nil {
		b.ops = nil
		return err
	}
	b.ops = nil
	return nil
}
func (b *pebbleBatch) Reset() { b.ops = nil }

// ---- stubs (to be filled by later tasks) ----
func (s *pebbleStore) ScanReverse(ctx context.Context, start, end []byte, fn func(key, value []byte) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if fn == nil {
		return ErrNilCallback
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.state != stateOpen {
		return s.stateError()
	}

	iter, _ := s.db.NewIter(nil)
	defer iter.Close()
	if end == nil {
		iter.Last()
	} else {
		iter.SeekLT(end)
	}
	for iter.Valid() && (start == nil || bytes.Compare(iter.Key(), start) >= 0) {
		if err := ctx.Err(); err != nil {
			return err
		}
		kc := make([]byte, len(iter.Key()))
		copy(kc, iter.Key())
		vc := make([]byte, len(iter.Value()))
		copy(vc, iter.Value())
		if err := fn(kc, vc); err != nil {
			return err
		}
		iter.Prev()
	}
	return iter.Error()
}
func (s *pebbleStore) NewSnapshot(ctx context.Context) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	state := s.state
	s.mu.RUnlock()
	switch state {
	case stateNeverOpened:
		return nil, ErrNotOpen
	case stateClosed:
		return nil, ErrClosed
	}

	snap := s.db.NewSnapshot()
	atomic.AddInt64(&s.snapCount, 1)
	return &pebbleSnapshot{store: s, snap: snap, closed: false}, nil
}
func (s *pebbleStore) Stats(ctx context.Context) (Stats, error) {
	if err := ctx.Err(); err != nil {
		return Stats{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.state != stateOpen {
		return Stats{}, s.stateError()
	}
	var count int64
	iter, _ := s.db.NewIter(nil)
	defer iter.Close()
	for iter.First(); iter.Valid(); iter.Next() {
		count++
	}
	return Stats{Backend: "pebble", KeyCount: count, ReadOnly: s.readOnly, SyncWrites: s.syncWrites}, nil
}
func (s *pebbleStore) Check(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.state != stateOpen {
		return s.stateError()
	}
	iter, _ := s.db.NewIter(nil)
	defer iter.Close()
	for iter.First(); iter.Valid(); iter.Next() {
		_ = iter.Value()
	}
	return iter.Error()
}
func (s *pebbleStore) stateError() error {
	switch s.state {
	case stateNeverOpened:
		return ErrNotOpen
	case stateClosed:
		return ErrClosed
	}
	return nil
}

// ---- pebbleSnapshot ----

type pebbleSnapshot struct {
	store  *pebbleStore
	snap   *pebble.Snapshot
	mu     sync.Mutex
	closed bool
}

func (ps *pebbleSnapshot) Get(ctx context.Context, key []byte) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	ps.mu.Lock()
	if ps.closed {
		ps.mu.Unlock()
		return nil, false, ErrSnapshotClosed
	}
	ps.mu.Unlock()
	v, closer, err := ps.snap.Get(key)
	if err == pebble.ErrNotFound {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	val := make([]byte, len(v))
	copy(val, v)
	closer.Close()
	return val, true, nil
}

func (ps *pebbleSnapshot) Scan(ctx context.Context, start, end []byte, fn func(key, value []byte) error) error {
	return ps.scanDir(ctx, start, end, fn, false)
}

func (ps *pebbleSnapshot) ScanReverse(ctx context.Context, start, end []byte, fn func(key, value []byte) error) error {
	return ps.scanDir(ctx, start, end, fn, true)
}

func (ps *pebbleSnapshot) scanDir(ctx context.Context, start, end []byte, fn func(key, value []byte) error, reverse bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if fn == nil {
		return ErrNilCallback
	}
	ps.mu.Lock()
	if ps.closed {
		ps.mu.Unlock()
		return ErrSnapshotClosed
	}
	ps.mu.Unlock()

	iter, _ := ps.snap.NewIter(nil)
	defer iter.Close()
	if reverse {
		if end == nil {
			iter.Last()
		} else {
			iter.SeekLT(end)
		}
	} else {
		if start == nil {
			iter.First()
		} else {
			iter.SeekGE(start)
		}
	}
	for iter.Valid() {
		if reverse {
			if start != nil && bytes.Compare(iter.Key(), start) < 0 {
				break
			}
		} else {
			if end != nil && bytes.Compare(iter.Key(), end) >= 0 {
				break
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		kc := make([]byte, len(iter.Key()))
		copy(kc, iter.Key())
		vc := make([]byte, len(iter.Value()))
		copy(vc, iter.Value())
		if err := fn(kc, vc); err != nil {
			return err
		}
		if reverse {
			iter.Prev()
		} else {
			iter.Next()
		}
	}
	return iter.Error()
}

func (ps *pebbleSnapshot) Close() error {
	ps.mu.Lock()
	if ps.closed {
		ps.mu.Unlock()
		return ErrSnapshotClosed
	}
	ps.closed = true
	ps.mu.Unlock()
	ps.snap.Close()
	atomic.AddInt64(&ps.store.snapCount, -1)
	return nil
}
