package kv

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	bolt "go.etcd.io/bbolt"
)

const bucketName = "plomvix_sql"

type storeState int

const (
	stateNeverOpened storeState = iota
	stateOpen
	stateClosed
)

type bboltStore struct {
	mu         sync.RWMutex
	name       string
	path       string
	state      storeState
	db         *bolt.DB
	readOnly   bool
	syncWrites bool
	snapCount  int64
	snapMu     sync.Mutex
}

// NewBBolt creates a KVStore backed by a bbolt database at path.
func NewBBolt(name, path string) KVStore {
	return newBBoltWithOptions(name, path, Options{
		Backend: "bbolt", Path: path, SyncWrites: true, ReadOnly: false,
	})
}

func newBBoltWithOptions(name, path string, opts Options) KVStore {
	return &bboltStore{
		name: name, path: path, state: stateNeverOpened,
		readOnly: opts.ReadOnly, syncWrites: opts.SyncWrites,
	}
}

func (s *bboltStore) Name() string { return s.name }

func (s *bboltStore) Open(ctx context.Context) error {
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
		db, err := bolt.Open(s.path, 0600, &bolt.Options{ReadOnly: true})
		if err != nil {
			return fmt.Errorf("kv: open bbolt read-only: %w", err)
		}
		s.db = db
		s.state = stateOpen
		return nil
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("kv: create directory %s: %w", dir, err)
	}

	boltOpts := &bolt.Options{}
	if !s.syncWrites {
		boltOpts.NoSync = true
	}
	db, err := bolt.Open(s.path, 0600, boltOpts)
	if err != nil {
		return fmt.Errorf("kv: open bbolt: %w", err)
	}

	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		return err
	}); err != nil {
		db.Close()
		return fmt.Errorf("kv: create bucket: %w", err)
	}
	s.db = db
	s.state = stateOpen
	return nil
}

func (s *bboltStore) Close(ctx context.Context) error {
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

func (s *bboltStore) Get(ctx context.Context, key []byte) ([]byte, bool, error) {
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

	var val []byte
	var found bool
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		v := b.Get(key)
		found = v != nil
		if found {
			val = make([]byte, len(v))
			copy(val, v)
		}
		return nil
	})
	return val, found, err
}

func (s *bboltStore) Set(ctx context.Context, key, value []byte) error {
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
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		v := make([]byte, len(value))
		copy(v, value)
		return b.Put(key, v)
	})
}

func (s *bboltStore) Delete(ctx context.Context, key []byte) error {
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
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		return b.Delete(key)
	})
}

func (s *bboltStore) stateError() error {
	switch s.state {
	case stateNeverOpened:
		return ErrNotOpen
	case stateClosed:
		return ErrClosed
	default:
		return nil
	}
}

func (s *bboltStore) Scan(ctx context.Context, start, end []byte, fn func(key, value []byte) error) error {
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

	return s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		c := b.Cursor()
		var k, v []byte
		if start == nil {
			k, v = c.First()
		} else {
			k, v = c.Seek(start)
		}
		for k != nil && (end == nil || bytes.Compare(k, end) < 0) {
			if err := ctx.Err(); err != nil {
				return err
			}
			kc := make([]byte, len(k))
			copy(kc, k)
			vc := make([]byte, len(v))
			copy(vc, v)
			if err := fn(kc, vc); err != nil {
				return err
			}
			k, v = c.Next()
		}
		return nil
	})
}

func (s *bboltStore) NewBatch() Batch {
	return &bboltBatch{store: s}
}

type bboltBatch struct {
	store *bboltStore
	ops   []batchOp
}

type batchOp struct {
	kind  uint8 // 0=set, 1=delete
	key   []byte
	value []byte
}

func (b *bboltBatch) Set(key, value []byte) {
	kc := make([]byte, len(key))
	copy(kc, key)
	vc := make([]byte, len(value))
	copy(vc, value)
	b.ops = append(b.ops, batchOp{kind: 0, key: kc, value: vc})
}

func (b *bboltBatch) Delete(key []byte) {
	kc := make([]byte, len(key))
	copy(kc, key)
	b.ops = append(b.ops, batchOp{kind: 1, key: kc})
}

func (b *bboltBatch) Commit(ctx context.Context) error {
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
			b.ops = nil // clear batch, apply nothing
			return ErrEmptyKey
		}
	}

	if len(b.ops) == 0 {
		return nil
	}

	err := b.store.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketName))
		for _, op := range b.ops {
			if op.kind == 0 {
				if err := bucket.Put(op.key, op.value); err != nil {
					return err
				}
			} else {
				if err := bucket.Delete(op.key); err != nil {
					return err
				}
			}
		}
		return nil
	})
	b.ops = nil
	return err
}

func (b *bboltBatch) Reset() {
	b.ops = nil
}

// ---- new method stubs (to be replaced in later tasks) ----

func (s *bboltStore) ScanReverse(ctx context.Context, start, end []byte, fn func(key, value []byte) error) error {
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

	return s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		c := b.Cursor()
		var k, v []byte
		if end == nil {
			k, v = c.Last()
		} else {
			k, v = c.Seek(end)
			if k == nil || bytes.Compare(k, end) >= 0 {
				k, v = c.Last()
			} else {
				k, v = c.Prev()
			}
		}
		for k != nil && (start == nil || bytes.Compare(k, start) >= 0) {
			if err := ctx.Err(); err != nil {
				return err
			}
			kc := make([]byte, len(k))
			copy(kc, k)
			vc := make([]byte, len(v))
			copy(vc, v)
			if err := fn(kc, vc); err != nil {
				return err
			}
			k, v = c.Prev()
		}
		return nil
	})
}

func (s *bboltStore) NewSnapshot(ctx context.Context) (Snapshot, error) {
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

	tx, err := s.db.Begin(false)
	if err != nil {
		return nil, fmt.Errorf("kv: snapshot: %w", err)
	}

	atomic.AddInt64(&s.snapCount, 1)
	return &bboltSnapshot{store: s, tx: tx, closed: false}, nil
}

func (s *bboltStore) Stats(ctx context.Context) (Stats, error) {
	if err := ctx.Err(); err != nil {
		return Stats{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.state != stateOpen {
		return Stats{}, s.stateError()
	}
	var count int64
	s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		c := b.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			count++
		}
		return nil
	})
	return Stats{Backend: "bbolt", KeyCount: count, ReadOnly: s.readOnly, SyncWrites: s.syncWrites}, nil
}

func (s *bboltStore) Check(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.state != stateOpen {
		return s.stateError()
	}
	return s.db.View(func(tx *bolt.Tx) error {
		if tx.Bucket([]byte(bucketName)) == nil {
			return fmt.Errorf("kv: bucket %s missing", bucketName)
		}
		c := tx.Bucket([]byte(bucketName)).Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			_ = v
		}
		return nil
	})
}

type bboltSnapshot struct {
	store  *bboltStore
	tx     *bolt.Tx
	mu     sync.Mutex
	closed bool
}

func (ss *bboltSnapshot) Get(ctx context.Context, key []byte) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	ss.mu.Lock()
	if ss.closed {
		ss.mu.Unlock()
		return nil, false, ErrSnapshotClosed
	}
	ss.mu.Unlock()

	b := ss.tx.Bucket([]byte(bucketName))
	v := b.Get(key)
	if v == nil {
		return nil, false, nil
	}
	val := make([]byte, len(v))
	copy(val, v)
	return val, true, nil
}

func (ss *bboltSnapshot) Scan(ctx context.Context, start, end []byte, fn func(key, value []byte) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return ss.scanDir(ctx, start, end, fn, false)
}

func (ss *bboltSnapshot) ScanReverse(ctx context.Context, start, end []byte, fn func(key, value []byte) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return ss.scanDir(ctx, start, end, fn, true)
}

func (ss *bboltSnapshot) scanDir(ctx context.Context, start, end []byte, fn func(key, value []byte) error, reverse bool) error {
	if fn == nil {
		return ErrNilCallback
	}
	ss.mu.Lock()
	if ss.closed {
		ss.mu.Unlock()
		return ErrSnapshotClosed
	}
	ss.mu.Unlock()

	b := ss.tx.Bucket([]byte(bucketName))
	c := b.Cursor()
	var k, v []byte
	if reverse {
		if end == nil {
			k, v = c.Last()
		} else {
			k, v = c.Seek(end)
			if k == nil || bytes.Compare(k, end) >= 0 {
				k, v = c.Last()
			} else {
				k, v = c.Prev()
			}
		}
	} else {
		if start == nil {
			k, v = c.First()
		} else {
			k, v = c.Seek(start)
		}
	}
	for k != nil {
		if reverse {
			if start != nil && bytes.Compare(k, start) < 0 {
				break
			}
		} else {
			if end != nil && bytes.Compare(k, end) >= 0 {
				break
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		kc := make([]byte, len(k))
		copy(kc, k)
		vc := make([]byte, len(v))
		copy(vc, v)
		if err := fn(kc, vc); err != nil {
			return err
		}
		if reverse {
			k, v = c.Prev()
		} else {
			k, v = c.Next()
		}
	}
	return nil
}

func (ss *bboltSnapshot) Close() error {
	ss.mu.Lock()
	if ss.closed {
		ss.mu.Unlock()
		return ErrSnapshotClosed
	}
	ss.closed = true
	ss.mu.Unlock()
	ss.tx.Rollback()
	atomic.AddInt64(&ss.store.snapCount, -1)
	return nil
}
