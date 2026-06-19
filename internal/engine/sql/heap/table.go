package heap

import (
	"context"
	"fmt"
	"sort"

	"github.com/plomvix/plomvix/internal/engine/sql/key"
)

// --- Insert ---

func (t *table) Insert(ctx context.Context, tx Tx, values []any) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(values) != len(t.schema.Columns) {
		return ErrColumnCountMismatch
	}

	pkVals := make([]any, len(t.schema.PKIndices))
	for i, idx := range t.schema.PKIndices {
		pkVals[i] = values[idx]
	}

	// ErrTxConflict takes priority over ErrDuplicateKey.
	if err := t.validateMonotonicTx(ctx, pkVals, tx.ID); err != nil {
		return err
	}

	// Check if visible row already exists.
	if _, _, err := t.findVisibleRow(ctx, pkVals, tx.ID); err == nil {
		return ErrDuplicateKey
	}

	// Encode and write.
	rowValue, err := encodeEnterpriseValue(t.schema, values, FlagNormal)
	if err != nil {
		return err
	}
	rowKey, err := encodeRowKeyFromPK(t.schema, pkVals, tx.ID)
	if err != nil {
		return err
	}
	return t.store.Set(ctx, rowKey, rowValue)
}

// --- Update ---

func (t *table) Update(ctx context.Context, tx Tx, pkValues []any, newValues []any) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(newValues) != len(t.schema.Columns) {
		return ErrColumnCountMismatch
	}

	if err := t.validateMonotonicTx(ctx, pkValues, tx.ID); err != nil {
		return err
	}

	// Check existing visible row.
	if _, _, err := t.findVisibleRow(ctx, pkValues, tx.ID); err != nil {
		return ErrKeyNotFound
	}

	// PK columns must not change.
	if !t.comparePK(pkValues, newValues) {
		return ErrPrimaryKeyUpdate
	}

	rowValue, err := encodeEnterpriseValue(t.schema, newValues, FlagNormal)
	if err != nil {
		return err
	}
	rowKey, err := encodeRowKeyFromPK(t.schema, pkValues, tx.ID)
	if err != nil {
		return err
	}
	return t.store.Set(ctx, rowKey, rowValue)
}

// --- Delete ---

func (t *table) Delete(ctx context.Context, tx Tx, pkValues []any) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.validateMonotonicTx(ctx, pkValues, tx.ID); err != nil {
		return err
	}

	// Check existing visible row.
	if _, _, err := t.findVisibleRow(ctx, pkValues, tx.ID); err != nil {
		return ErrKeyNotFound
	}

	// Write tombstone.
	tombstoneValue, err := encodeEnterpriseValue(t.schema, nil, FlagTombstone)
	if err != nil {
		return err
	}
	rowKey, err := encodeRowKeyFromPK(t.schema, pkValues, tx.ID)
	if err != nil {
		return err
	}
	return t.store.Set(ctx, rowKey, tombstoneValue)
}

// --- Get ---

func (t *table) Get(ctx context.Context, tx Tx, pkValues []any) ([]any, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	vals, _, err := t.findVisibleRow(ctx, pkValues, tx.ID)
	return vals, err
}

// --- Scan ---

func (t *table) Scan(ctx context.Context, tx Tx) (Rows, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	prefix := key.TablePrefix(t.schema.TableID)
	end := key.PrefixEnd(prefix)
	prefixKey := key.FromBytes(prefix)
	var endKey key.Key
	if end != nil {
		endKey = key.FromBytes(end)
	} else {
		endKey = key.FromBytes(nil)
	}

	entries, err := t.store.Scan(ctx, prefixKey, endKey)
	if err != nil {
		return nil, fmt.Errorf("heap: scan: %w", err)
	}

	// Debug: if raw count is wrong, the KV scan is the issue.
	_ = len(entries) // for debugging; will be removed

	// Group by PK, keep highest visible version <= tx.ID.
	pkKinds := make([]key.Kind, len(t.schema.PKIndices))
	for i, idx := range t.schema.PKIndices {
		pkKinds[i] = t.schema.Columns[idx].Kind
	}

	type entry struct {
		pkBytes []byte
		version uint64
		data    []byte
	}

	var filtered []entry
	for _, e := range entries {
		_, _, version, decErr := key.DecodeTableRowKey(e.Key.Bytes(), pkKinds)
		if decErr != nil || version > tx.ID {
			continue
		}
		// Extract PK prefix bytes for grouping.
		pkPrefix := extractPKPrefix(e.Key.Bytes())
		filtered = append(filtered, entry{pkPrefix, version, e.Value})
	}

	// Group by PK prefix and keep the latest version.
	type pkGroup struct {
		pkBytes []byte
		version uint64
		data    []byte
	}
	groups := make(map[string]pkGroup)
	for _, fe := range filtered {
		k := string(fe.pkBytes)
		if existing, ok := groups[k]; !ok || fe.version > existing.version {
			groups[k] = pkGroup{fe.pkBytes, fe.version, fe.data}
		}
	}

	// Collect visible, non-tombstone rows and decode them.
	var visRows []visibleRow
	for _, grp := range groups {
		if len(grp.data) > 0 && grp.data[0] == FlagTombstone {
			continue
		}
		vals, _, err := decodeEnterpriseValue(t.schema, grp.data)
		if err != nil {
			continue
		}
		visRows = append(visRows, visibleRow{grp.pkBytes, vals})
	}

	// Sort by PK bytes ascending.
	sort.Slice(visRows, func(i, j int) bool {
		a, b := visRows[i].pkBytes, visRows[j].pkBytes
		minLen := len(a)
		if len(b) < minLen {
			minLen = len(b)
		}
		for k := 0; k < minLen; k++ {
			if a[k] != b[k] {
				return a[k] < b[k]
			}
		}
		return len(a) < len(b)
	})

	return &enterpriseRows{
		rows:   visRows,
		schema: t.schema,
	}, nil
}

// extractPKPrefix extracts the PK prefix (everything before the version suffix) from a key.
func extractPKPrefix(fullKey []byte) []byte {
	// The key layout: [0x01][tableID][pk...][version(8)].
	// PK prefix is everything except the last 8 bytes (version).
	if len(fullKey) <= 8 {
		return fullKey
	}
	return fullKey[:len(fullKey)-8]
}

// --- Vacuum ---

func (t *table) Vacuum(ctx context.Context, olderThan uint64) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	prefix := key.TablePrefix(t.schema.TableID)
	end := key.PrefixEnd(prefix)
	prefixKey := key.FromBytes(prefix)
	var endKey key.Key
	if end != nil {
		endKey = key.FromBytes(end)
	} else {
		endKey = key.FromBytes(nil)
	}

	entries, err := t.store.Scan(ctx, prefixKey, endKey)
	if err != nil {
		return fmt.Errorf("heap: vacuum scan: %w", err)
	}

	pkKinds := make([]key.Kind, len(t.schema.PKIndices))
	for i, idx := range t.schema.PKIndices {
		pkKinds[i] = t.schema.Columns[idx].Kind
	}

	// Group by PK prefix.
	type versionEntry struct {
		version  uint64
		data     []byte
		keyBytes []byte
	}
	groupMap := make(map[string][]versionEntry)
	for _, e := range entries {
		_, _, version, decErr := key.DecodeTableRowKey(e.Key.Bytes(), pkKinds)
		if decErr != nil {
			continue
		}
		pkPrefix := extractPKPrefix(e.Key.Bytes())
		k := string(pkPrefix)
		groupMap[k] = append(groupMap[k], versionEntry{version, e.Value, e.Key.Bytes()})
	}

	for _, versions := range groupMap {
		// Sort by version descending.
		sort.Slice(versions, func(i, j int) bool {
			return versions[i].version > versions[j].version
		})

		if len(versions) == 0 {
			continue
		}

		// Find highest version <= olderThan.
		var highestOldIdx int = -1
		for i, ve := range versions {
			if ve.version <= olderThan {
				highestOldIdx = i
				break
			}
		}
		if highestOldIdx < 0 {
			// No old versions to vacuum.
			continue
		}

		highestOld := versions[highestOldIdx]
		isTomb, hasNewer := false, false
		if len(highestOld.data) > 0 && highestOld.data[0] == FlagTombstone {
			isTomb = true
		}
		// Check if there are newer versions.
		for _, ve := range versions {
			if ve.version > olderThan {
				hasNewer = true
				break
			}
		}

		// Delete appropriate versions.
		for _, ve := range versions {
			if ve.version > olderThan {
				continue // keep versions > olderThan
			}
			if isTomb && !hasNewer {
				// Delete ALL versions of this PK (all are old, all delete).
				delKey := key.FromBytes(ve.keyBytes)
				t.store.Delete(ctx, delKey)
			} else if !isTomb && ve.version < highestOld.version {
				// Delete versions strictly less than the highest visible old version.
				delKey := key.FromBytes(ve.keyBytes)
				t.store.Delete(ctx, delKey)
			}
		}
	}

	return nil
}

// --- Rows Iterator (Enterprise) ---

type visibleRow struct {
	pkBytes []byte
	values  []any
}

type enterpriseRows struct {
	rows    []visibleRow
	schema  Schema
	idx     int
	curVals []any
	lastErr error
}

func (r *enterpriseRows) Next() bool {
	r.idx++
	if r.idx >= len(r.rows) {
		return false
	}
	r.curVals = r.rows[r.idx].values
	return true
}

func (r *enterpriseRows) Values() []any {
	if r.curVals == nil {
		return nil
	}
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

func (r *enterpriseRows) Err() error   { return r.lastErr }
func (r *enterpriseRows) Close() error { r.rows = nil; return nil }
