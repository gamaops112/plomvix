// Package system provides the SystemHeapFactory that creates and opens
// the physical heap files for Plomvix system tables. It defines a local
// SystemHeap interface to avoid importing concrete heap implementations,
// and a SystemHeapAdapter that maps catalog.SystemTable KV operations
// onto MVCC heap row operations.
package system

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"path/filepath"
	"sync/atomic"

	"github.com/plomvix/plomvix/internal/catalog"
	"github.com/plomvix/plomvix/internal/engine"
	"github.com/plomvix/plomvix/internal/engine/sql/key"
	"github.com/plomvix/plomvix/internal/engine/sql/kv"
	"github.com/plomvix/plomvix/internal/storage/pager"
	"github.com/plomvix/plomvix/internal/systemids"
)

// SystemHeap abstracts MVCC row-oriented storage for system metadata.
// Defined locally to sever dependency on concrete heap implementations.
type SystemHeap interface {
	Insert(ctx context.Context, tx engine.TxContext, row engine.Row) error
	Scan(ctx context.Context, tx engine.TxContext) (SystemHeapIterator, error)
}

// SystemHeapIterator iterates over system heap rows.
type SystemHeapIterator interface {
	Next(ctx context.Context) (engine.Row, error) // io.EOF when exhausted
	Close() error
}

// Factory creates and opens the physical storage for system tables.
type Factory struct {
	dataDir string
}

// NewFactory creates a SystemHeapFactory for the given data directory.
func NewFactory(dataDir string) *Factory {
	return &Factory{dataDir: dataDir}
}

// OpenOrCreateSystemHeaps creates/opens the physical heap files for the
// three reserved system tables and returns them as catalog.SystemTable
// implementations, wrapped in SystemHeapAdapter.
func (f *Factory) OpenOrCreateSystemHeaps(ctx context.Context) (tables, columns, users catalog.SystemTable, err error) {
	makeStore := func(id uint64) (*concreteSystemHeap, error) {
		path := f.heapPath(id)
		pg := pager.New(path)
		if err := pg.Open(ctx); err != nil {
			return nil, fmt.Errorf("system: open pager %d: %w", id, err)
		}
		store := kv.New(pg)
		if err := store.Open(ctx); err != nil {
			pg.Close(ctx)
			return nil, fmt.Errorf("system: open kv %d: %w", id, err)
		}
		return &concreteSystemHeap{store: store, pg: pg}, nil
	}

	tablesHeap, err := makeStore(systemids.SystemTableTables)
	if err != nil {
		return nil, nil, nil, err
	}
	columnsHeap, err := makeStore(systemids.SystemTableColumns)
	if err != nil {
		return nil, nil, nil, err
	}
	usersHeap, err := makeStore(systemids.SystemTableUsers)
	if err != nil {
		return nil, nil, nil, err
	}
	return NewSystemHeapAdapter(tablesHeap), NewSystemHeapAdapter(columnsHeap), NewSystemHeapAdapter(usersHeap), nil
}

// heapPath returns the deterministic file path for a system heap.
func (f *Factory) heapPath(tableID uint64) string {
	return filepath.Join(f.dataDir, fmt.Sprintf("heap_%d.db", tableID))
}

// concreteSystemHeap implements SystemHeap using direct KV-store operations.
// Key encoding: the adapter's key is the KV key. The KV value stores the
// adapter's value prefixed by an 8-byte big-endian version.
type concreteSystemHeap struct {
	store kv.KVStore
	pg    pager.Pager
}

func (c *concreteSystemHeap) Insert(ctx context.Context, tx engine.TxContext, row engine.Row) error {
	if len(row) < 2 {
		return fmt.Errorf("system: insert requires at least 2 columns (key, value)")
	}
	k, err := rowToKey(row[0])
	if err != nil {
		return err
	}
	v := encodeSystemValue(tx.WriteTxID, row)
	return c.store.Set(ctx, k, v)
}

func (c *concreteSystemHeap) Scan(ctx context.Context, tx engine.TxContext) (SystemHeapIterator, error) {
	entries, err := c.store.Scan(ctx, key.Key{}, key.Key{})
	if err != nil {
		return nil, err
	}
	return &directIterator{entries: entries, readTxID: tx.ReadTxID}, nil
}

// rowToKey converts a datum to a KV key.
func rowToKey(d engine.Datum) (key.Key, error) {
	switch v := d.Value.(type) {
	case []byte:
		return key.EncodeBytes(v), nil
	case string:
		return key.EncodeString(v)
	default:
		return key.Key{}, fmt.Errorf("system: unsupported key type %T", d.Value)
	}
}

// encodeSystemValue encodes [version(8)][value_bytes].
func encodeSystemValue(version uint64, row engine.Row) []byte {
	if len(row) < 2 {
		return nil
	}
	var valBytes []byte
	switch v := row[1].Value.(type) {
	case []byte:
		valBytes = v
	case string:
		valBytes = []byte(v)
	default:
		return nil
	}
	// Prefix with 8-byte version.
	buf := make([]byte, 8+len(valBytes))
	binary.BigEndian.PutUint64(buf[:8], version)
	copy(buf[8:], valBytes)
	return buf
}

// decodeSystemValue decodes a stored value. Returns nil if the version is higher
// than readTxID or the value is empty (= tombstone).
func decodeSystemValue(b []byte, readTxID uint64) ([]byte, bool) {
	if len(b) < 8 {
		return nil, false
	}
	version := binary.BigEndian.Uint64(b[:8])
	if version > readTxID {
		return nil, false
	}
	val := b[8:]
	if len(val) == 0 {
		return nil, false // tombstone
	}
	cp := make([]byte, len(val))
	copy(cp, val)
	return cp, true
}

// directIterator iterates raw KV entries and filters by readTxID.
type directIterator struct {
	entries  []kv.Entry
	idx      int
	readTxID uint64
}

func (it *directIterator) Next(ctx context.Context) (engine.Row, error) {
	for it.idx < len(it.entries) {
		e := it.entries[it.idx]
		it.idx++
		val, ok := decodeSystemValue(e.Value, it.readTxID)
		if !ok {
			continue
		}
		keyBytes := keyDecodeToBytes(e.Key)
		row := engine.Row{
			{Type: engine.TypeBytes, Value: keyBytes},
			{Type: engine.TypeBytes, Value: val},
		}
		return row, nil
	}
	return nil, io.EOF
}

func (it *directIterator) Close() error { return nil }

func keyDecodeToBytes(k key.Key) []byte {
	b := k.Bytes()
	cp := make([]byte, len(b))
	copy(cp, b)
	return cp
}

// SystemHeapAdapter implements catalog.SystemTable using a SystemHeap.
// It maps KV (Put/Get/Delete/Scan) to MVCC row operations with an
// internal monotonically-increasing TxID counter.
type SystemHeapAdapter struct {
	heap     SystemHeap
	nextTxID atomic.Uint64
}

// NewSystemHeapAdapter wraps a SystemHeap as a catalog.SystemTable.
func NewSystemHeapAdapter(h SystemHeap) catalog.SystemTable {
	return &SystemHeapAdapter{heap: h}
}

// Put stores a new MVCC version of the key-value pair.
func (a *SystemHeapAdapter) Put(ctx context.Context, key, value []byte) error {
	txID := a.nextTxID.Add(1)
	row := engine.Row{
		{Type: engine.TypeBytes, Value: key},
		{Type: engine.TypeBytes, Value: value},
	}
	return a.heap.Insert(ctx, engine.TxContext{WriteTxID: txID}, row)
}

// Get scans for the latest visible value of key, filtering tombstones.
func (a *SystemHeapAdapter) Get(ctx context.Context, key []byte) ([]byte, error) {
	iter, err := a.heap.Scan(ctx, engine.TxContext{ReadTxID: ^uint64(0)})
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	var latestValue []byte
	found := false
	for {
		row, err := iter.Next(ctx)
		if err != nil {
			break
		}
		if len(row) < 2 {
			continue
		}
		rowKey, ok := row[0].Value.([]byte)
		if !ok || !bytes.Equal(rowKey, key) {
			continue
		}
		found = true
		if row[1].Value == nil {
			latestValue = nil // tombstone (nil)
		} else if b, ok := row[1].Value.([]byte); ok {
			if len(b) == 0 {
				latestValue = nil // tombstone (empty slice)
			} else {
				latestValue = b
			}
		}
	}
	if !found || latestValue == nil {
		return nil, catalog.ErrNotFound
	}
	return latestValue, nil
}

// Delete tombstones a key by inserting an empty value (nil not supported by heap).
func (a *SystemHeapAdapter) Delete(ctx context.Context, key []byte) error {
	txID := a.nextTxID.Add(1)
	row := engine.Row{
		{Type: engine.TypeBytes, Value: key},
		{Type: engine.TypeBytes, Value: []byte{}}, // empty = tombstone
	}
	return a.heap.Insert(ctx, engine.TxContext{WriteTxID: txID}, row)
}

// Scan iterates all live key-value pairs, deduplicating by key (latest wins).
func (a *SystemHeapAdapter) Scan(ctx context.Context, fn func(k, v []byte) error) error {
	iter, err := a.heap.Scan(ctx, engine.TxContext{ReadTxID: ^uint64(0)})
	if err != nil {
		return err
	}
	defer iter.Close()

	latest := make(map[string][]byte) // key -> latest non-tombstone value
	for {
		row, err := iter.Next(ctx)
		if err != nil {
			break
		}
		if len(row) < 2 {
			continue
		}
		k, ok := row[0].Value.([]byte)
		if !ok {
			continue
		}
		ks := string(k)
		if row[1].Value == nil {
			latest[ks] = nil // tombstone
		} else if v, ok := row[1].Value.([]byte); ok {
			if len(v) == 0 {
				latest[ks] = nil // tombstone (empty slice)
			} else {
				latest[ks] = v
			}
		}
	}
	for k, v := range latest {
		if v != nil {
			if err := fn([]byte(k), v); err != nil {
				return err
			}
		}
	}
	return nil
}

var _ catalog.SystemTable = (*SystemHeapAdapter)(nil)
