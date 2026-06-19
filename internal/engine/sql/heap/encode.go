package heap

import (
	"fmt"

	"github.com/plomvix/plomvix/internal/engine/sql/key"
)

// anyToPrimitive validates and returns the Go primitive for a column value.
// Returns ErrNullNotSupported for nil, ErrTypeMismatch for wrong types.
func anyToPrimitive(kind key.Kind, v any) (any, error) {
	if v == nil {
		return nil, ErrNullNotSupported
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

// encodeRowKeyFromRow encodes the table row key from full row values.
func encodeRowKeyFromRow(schema Schema, values []any) (key.Key, error) {
	pkVals := make([]any, len(schema.PKIndices))
	for i, idx := range schema.PKIndices {
		pkVals[i] = values[idx]
	}
	return encodeRowKeyFromPK(schema, pkVals)
}

// encodeRowKeyFromPK encodes the table row key from PK values only.
func encodeRowKeyFromPK(schema Schema, pkValues []any) (key.Key, error) {
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

	raw, err := key.EncodeTableRowKey(schema.TableID, kvs, BasicVersion)
	if err != nil {
		return key.Key{}, err
	}

	return key.FromBytes(raw), nil
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
