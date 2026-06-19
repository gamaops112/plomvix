package heap

import (
	"context"
	"fmt"

	"github.com/plomvix/plomvix/internal/engine/sql/key"
	"github.com/plomvix/plomvix/internal/engine/sql/kv"
)

// --- Insert ---

func (t *table) Insert(ctx context.Context, values []any) error {
	if len(values) != len(t.schema.Columns) {
		return ErrColumnCountMismatch
	}

	rowKey, err := encodeRowKeyFromRow(t.schema, values)
	if err != nil {
		return err
	}

	// Read-before-write for PK uniqueness.
	_, getErr := t.store.Get(ctx, rowKey)
	if getErr == nil {
		return ErrDuplicateKey
	}
	if getErr != kv.ErrKeyNotFound {
		return fmt.Errorf("heap: insert get: %w", getErr)
	}

	rowValue, err := encodeRowValue(t.schema, values)
	if err != nil {
		return err
	}

	return t.store.Set(ctx, rowKey, rowValue)
}

// --- Get ---

func (t *table) Get(ctx context.Context, pkValues []any) ([]any, error) {
	rowKey, err := encodeRowKeyFromPK(t.schema, pkValues)
	if err != nil {
		return nil, err
	}

	data, err := t.store.Get(ctx, rowKey)
	if err == kv.ErrKeyNotFound {
		return nil, ErrKeyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("heap: get: %w", err)
	}

	return decodeRowValue(t.schema, data)
}

// --- Delete ---

func (t *table) Delete(ctx context.Context, pkValues []any) error {
	rowKey, err := encodeRowKeyFromPK(t.schema, pkValues)
	if err != nil {
		return err
	}
	return t.store.Delete(ctx, rowKey)
}

// --- Scan ---

func (t *table) Scan(ctx context.Context) (Rows, error) {
	prefix := key.TablePrefix(t.schema.TableID)
	end := key.PrefixEnd(prefix)

	prefixKey := key.FromBytes(prefix)
	var endKey key.Key
	if end != nil {
		endKey = key.FromBytes(end)
	} else {
		endKey = key.FromBytes(nil) // unbounded
	}

	entries, err := t.store.Scan(ctx, prefixKey, endKey)
	if err != nil {
		return nil, fmt.Errorf("heap: scan: %w", err)
	}

	return &rows{
		entries: entries,
		schema:  t.schema,
		idx:     -1,
	}, nil
}

// --- Rows Iterator ---

type rows struct {
	entries []kv.Entry
	schema  Schema
	idx     int
	curVals []any
	lastErr error
}

func (r *rows) Next() bool {
	r.idx++
	if r.idx >= len(r.entries) {
		return false
	}

	vals, err := decodeRowValue(r.schema, r.entries[r.idx].Value)
	if err != nil {
		r.lastErr = err
		return false
	}
	r.curVals = vals
	return true
}

func (r *rows) Values() []any {
	if r.curVals == nil {
		return nil
	}
	// Deep copy.
	out := make([]any, len(r.curVals))
	for i, v := range r.curVals {
		switch val := v.(type) {
		case []byte:
			cp := make([]byte, len(val))
			copy(cp, val)
			out[i] = cp
		default:
			out[i] = val
		}
	}
	return out
}

func (r *rows) Err() error {
	return r.lastErr
}

func (r *rows) Close() error {
	r.entries = nil
	r.curVals = nil
	return nil
}
