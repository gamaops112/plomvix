package heap

import (
	"context"
	"fmt"
	"sort"

	"github.com/plomvix/plomvix/internal/engine/sql/key"
)

// anyToPrimitive validates and returns the Go primitive for a column value.
// In Enterprise, nil is allowed (returns nil, nil).
func anyToPrimitive(kind key.Kind, v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	switch kind {
	case key.KindUint64:
		val, ok := v.(uint64)
		if !ok {
			return nil, ErrTypeMismatch
		}
		return val, nil
	case key.KindInt64:
		val, ok := v.(int64)
		if !ok {
			return nil, ErrTypeMismatch
		}
		return val, nil
	case key.KindString:
		val, ok := v.(string)
		if !ok {
			return nil, ErrTypeMismatch
		}
		return val, nil
	case key.KindBytes:
		switch val := v.(type) {
		case []byte:
			cp := make([]byte, len(val))
			copy(cp, val)
			return cp, nil
		default:
			return nil, ErrTypeMismatch
		}
	default:
		return nil, ErrTypeMismatch
	}
}

// anyToKeyValue converts a Go value to a key.Value.
func anyToKeyValue(kind key.Kind, v any) (key.Value, error) {
	if v == nil {
		return key.Value{}, ErrNullNotSupported
	}
	switch kind {
	case key.KindUint64:
		val, ok := v.(uint64)
		if !ok {
			return key.Value{}, ErrTypeMismatch
		}
		return key.Uint64(val), nil
	case key.KindInt64:
		val, ok := v.(int64)
		if !ok {
			return key.Value{}, ErrTypeMismatch
		}
		return key.Int64(val), nil
	case key.KindString:
		val, ok := v.(string)
		if !ok {
			return key.Value{}, ErrTypeMismatch
		}
		return key.String(val), nil
	case key.KindBytes:
		switch val := v.(type) {
		case []byte:
			return key.Bytes(val), nil
		default:
			return key.Value{}, ErrTypeMismatch
		}
	default:
		return key.Value{}, ErrTypeMismatch
	}
}

// encodeRowKeyFromPK encodes the table row key from PK values with a version.
func encodeRowKeyFromPK(schema Schema, pkValues []any, version uint64) (key.Key, error) {
	if len(pkValues) != len(schema.PKIndices) {
		return key.Key{}, fmt.Errorf("%w: pk value count mismatch", ErrColumnCountMismatch)
	}
	kvs := make([]key.Value, len(pkValues))
	for i, v := range pkValues {
		col := schema.Columns[schema.PKIndices[i]]
		kv, err := anyToKeyValue(col.Kind, v)
		if err != nil {
			return key.Key{}, err
		}
		kvs[i] = kv
	}
	raw, err := key.EncodeTableRowKey(schema.TableID, kvs, version)
	if err != nil {
		return key.Key{}, err
	}
	return key.FromBytes(raw), nil
}

// encodeRowVersionPrefix returns scan bounds for all versions of a given PK.
func encodeRowVersionPrefix(schema Schema, pkValues []any) (key.Key, key.Key, error) {
	if len(pkValues) != len(schema.PKIndices) {
		return key.Key{}, key.Key{}, ErrColumnCountMismatch
	}
	kvs := make([]key.Value, len(pkValues))
	for i, v := range pkValues {
		col := schema.Columns[schema.PKIndices[i]]
		kv, err := anyToKeyValue(col.Kind, v)
		if err != nil {
			return key.Key{}, key.Key{}, err
		}
		kvs[i] = kv
	}
	prefix, err := key.EncodeTableRowPrefix(schema.TableID, kvs)
	if err != nil {
		return key.Key{}, key.Key{}, err
	}
	endRaw := key.PrefixEnd(prefix)
	startKey := key.FromBytes(prefix)
	var endKey key.Key
	if endRaw != nil {
		endKey = key.FromBytes(endRaw)
	} else {
		endKey = key.FromBytes(nil)
	}
	return startKey, endKey, nil
}

// encodeEnterpriseValue encodes row values with null-bitmask prefix.
// Layout: [1-byte RowFlags][N-byte NullBitmask][EncodeStorageComposite(non-null primitives)].
func encodeEnterpriseValue(schema Schema, values []any, flags byte) ([]byte, error) {
	// For tombstones, values may be nil (just store flags + zero bitmask).
	if flags == FlagTombstone && values == nil {
		values = make([]any, len(schema.Columns))
	}
	if len(values) != len(schema.Columns) {
		return nil, ErrColumnCountMismatch
	}

	// Build null bitmask.
	numCols := len(schema.Columns)
	bitmaskLen := (numCols + 7) / 8
	bitmask := make([]byte, bitmaskLen)

	var nonNullPrimitives []any
	for i, v := range values {
		if v == nil {
			// Set null bit.
			bitmask[i/8] |= 1 << (i % 8)
		} else {
			p, err := anyToPrimitive(schema.Columns[i].Kind, v)
			if err != nil {
				return nil, err
			}
			nonNullPrimitives = append(nonNullPrimitives, p)
		}
	}

	// Encode non-null primitives via storage composite.
	var compositeKey key.Key
	if len(nonNullPrimitives) > 0 {
		var err error
		compositeKey, err = key.EncodeStorageComposite(nonNullPrimitives...)
		if err != nil {
			return nil, err
		}
	}

	// Build final value.
	result := make([]byte, 0, 1+bitmaskLen+len(compositeKey.Bytes()))
	result = append(result, flags)
	result = append(result, bitmask...)
	result = append(result, compositeKey.Bytes()...)
	return result, nil
}

// decodeEnterpriseValue decodes the enterprise row value.
// Returns values, isTombstone, and error.
func decodeEnterpriseValue(schema Schema, data []byte) (values []any, isTombstone bool, err error) {
	if len(data) < 1 {
		return nil, false, fmt.Errorf("heap: value too short")
	}

	flags := data[0]
	switch flags {
	case FlagNormal:
		isTombstone = false
	case FlagTombstone:
		isTombstone = true
	default:
		return nil, false, fmt.Errorf("heap: unknown row flags 0x%x", flags)
	}

	numCols := len(schema.Columns)
	bitmaskLen := (numCols + 7) / 8
	if len(data) < 1+bitmaskLen {
		return nil, false, fmt.Errorf("heap: value truncated at bitmask")
	}
	bitmask := data[1 : 1+bitmaskLen]

	if isTombstone {
		return nil, true, nil
	}

	// Decode composite payload.
	payload := data[1+bitmaskLen:]

	// Count non-null columns to determine kinds for ParseStorageCompositeKey.
	var nonNullKinds []key.Kind
	for i := 0; i < numCols; i++ {
		if bitmask[i/8]&(1<<(i%8)) == 0 {
			nonNullKinds = append(nonNullKinds, schema.Columns[i].Kind)
		}
	}

	var nonNullVals []any
	if len(payload) > 0 && len(nonNullKinds) > 0 {
		k, parseErr := key.ParseStorageCompositeKey(payload, nonNullKinds)
		if parseErr != nil {
			return nil, false, fmt.Errorf("heap: %w", parseErr)
		}
		var decErr error
		nonNullVals, decErr = key.DecodeStorageComposite(k)
		if decErr != nil {
			return nil, false, fmt.Errorf("heap: %w", decErr)
		}
	}

	// Reconstruct full values (inserting nil for null columns).
	values = make([]any, numCols)
	nnIdx := 0
	for i := 0; i < numCols; i++ {
		if bitmask[i/8]&(1<<(i%8)) != 0 {
			values[i] = nil
		} else {
			if nnIdx >= len(nonNullVals) {
				return nil, false, fmt.Errorf("heap: value decode mismatch")
			}
			values[i] = nonNullVals[nnIdx]
			nnIdx++
		}
	}

	return values, false, nil
}

// validateMonotonicTx checks that no version >= txID exists for this PK.
func (t *table) validateMonotonicTx(ctx context.Context, pkValues []any, txID uint64) error {
	if txID == 0 {
		return ErrInvalidTx
	}

	start, end, err := encodeRowVersionPrefix(t.schema, pkValues)
	if err != nil {
		return err
	}

	entries, err := t.store.Scan(ctx, start, end)
	if err != nil {
		return fmt.Errorf("heap: tx scan: %w", err)
	}

	// We need to decode versions from keys.
	pkKinds := make([]key.Kind, len(t.schema.PKIndices))
	for i, idx := range t.schema.PKIndices {
		pkKinds[i] = t.schema.Columns[idx].Kind
	}

	for _, e := range entries {
		_, _, version, decErr := key.DecodeTableRowKey(e.Key.Bytes(), pkKinds)
		if decErr != nil {
			continue
		}
		if version >= txID {
			return ErrTxConflict
		}
	}

	return nil
}

// findVisibleRow scans all versions of a PK and returns the highest version <= txID.
// Returns (values, version, nil) if found. Returns (nil, 0, ErrKeyNotFound) if none.
func (t *table) findVisibleRow(ctx context.Context, pkValues []any, txID uint64) ([]any, uint64, error) {
	start, end, err := encodeRowVersionPrefix(t.schema, pkValues)
	if err != nil {
		return nil, 0, err
	}

	entries, err := t.store.Scan(ctx, start, end)
	if err != nil {
		return nil, 0, fmt.Errorf("heap: visibility scan: %w", err)
	}

	pkKinds := make([]key.Kind, len(t.schema.PKIndices))
	for i, idx := range t.schema.PKIndices {
		pkKinds[i] = t.schema.Columns[idx].Kind
	}

	// Sort by version descending (entries are ordered by key ascending, key includes version).
	// Reverse iterate.
	type rowVersion struct {
		version uint64
		data    []byte
	}
	var rowVersions []rowVersion
	for _, e := range entries {
		_, _, version, decErr := key.DecodeTableRowKey(e.Key.Bytes(), pkKinds)
		if decErr != nil {
			continue
		}
		if version <= txID {
			rowVersions = append(rowVersions, rowVersion{version, e.Value})
		}
	}

	if len(rowVersions) == 0 {
		return nil, 0, ErrKeyNotFound
	}

	// Find highest version.
	sort.Slice(rowVersions, func(i, j int) bool {
		return rowVersions[i].version > rowVersions[j].version
	})

	latest := rowVersions[0]
	vals, isTombstone, err := decodeEnterpriseValue(t.schema, latest.data)
	if err != nil {
		return nil, 0, err
	}
	if isTombstone {
		return nil, 0, ErrKeyNotFound
	}

	return vals, latest.version, nil
}

// comparePK checks if the PK columns in newValues match pkValues.
func (t *table) comparePK(pkValues []any, newValues []any) bool {
	for i, idx := range t.schema.PKIndices {
		if idx >= len(newValues) {
			return false
		}
		if !valueEqual(pkValues[i], newValues[idx]) {
			return false
		}
	}
	return true
}

func valueEqual(a, b any) bool {
	switch av := a.(type) {
	case uint64:
		bv, ok := b.(uint64)
		return ok && av == bv
	case int64:
		bv, ok := b.(int64)
		return ok && av == bv
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case []byte:
		bv, ok := b.([]byte)
		if !ok {
			return false
		}
		if len(av) != len(bv) {
			return false
		}
		for i := range av {
			if av[i] != bv[i] {
				return false
			}
		}
		return true
	}
	return false
}

// encodeRowValue encodes a full row into storage-composite format.
func encodeRowValue(schema Schema, values []any) ([]byte, error) {
	if len(values) != len(schema.Columns) {
		return nil, ErrColumnCountMismatch
	}

	primitives := make([]any, len(values))
	for i, v := range values {
		p, err := anyToPrimitive(schema.Columns[i].Kind, v)
		if err != nil {
			return nil, err
		}
		primitives[i] = p
	}

	storageKey, err := key.EncodeStorageComposite(primitives...)
	if err != nil {
		return nil, err
	}

	return storageKey.Bytes(), nil
}

// decodeRowValue decodes a storage-composite-encoded row back into []any.
func decodeRowValue(schema Schema, data []byte) ([]any, error) {
	kinds := make([]key.Kind, len(schema.Columns))
	for i, col := range schema.Columns {
		kinds[i] = col.Kind
	}

	k, err := key.ParseStorageCompositeKey(data, kinds)
	if err != nil {
		return nil, fmt.Errorf("heap: %w", key.ErrInvalidKey)
	}

	return key.DecodeStorageComposite(k)
}
