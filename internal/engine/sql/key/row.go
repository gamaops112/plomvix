package key

import (
	"encoding/binary"
)

// TableRowTag is the keyspace tag for table row keys.
const tableRowTag byte = 0x01

// EncodeTableRowKey builds [0x01][tableID][encoded pk][version].
// Requires len(pk) >= 1. Returns raw bytes (not a Key).
func EncodeTableRowKey(tableID uint64, pk []Value, version uint64) ([]byte, error) {
	if len(pk) == 0 {
		return nil, ErrNoPKColumns
	}

	// Estimate size: 1 + 8 + (pk contrib) + 8.
	size := 1 + 8 + 8
	for _, v := range pk {
		switch v.kind {
		case KindUint64, KindInt64:
			size += 8
		case KindString:
			size += len(v.s)
		case KindBytes:
			size += len(v.b)
		}
	}

	buf := make([]byte, 0, size)
	buf = append(buf, tableRowTag)

	// tableID: 8 bytes big-endian.
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, tableID)
	buf = append(buf, b...)

	// pk columns: encoded using their natural representation.
	for _, v := range pk {
		switch v.kind {
		case KindUint64:
			cb := make([]byte, 8)
			binary.BigEndian.PutUint64(cb, v.u)
			buf = append(buf, cb...)
		case KindInt64:
			u := uint64(int64(v.u)) ^ (1 << 63)
			cb := make([]byte, 8)
			binary.BigEndian.PutUint64(cb, u)
			buf = append(buf, cb...)
		case KindString:
			buf = append(buf, []byte(v.s)...)
			buf = append(buf, 0x00)
		case KindBytes:
			buf = append(buf, v.b...)
		}
	}

	// version: 8 bytes big-endian.
	vb := make([]byte, 8)
	binary.BigEndian.PutUint64(vb, version)
	buf = append(buf, vb...)

	return buf, nil
}

// TablePrefix returns [0x01][tableID]; bounds all rows of a table.
func TablePrefix(tableID uint64) []byte {
	buf := make([]byte, 9)
	buf[0] = tableRowTag
	binary.BigEndian.PutUint64(buf[1:9], tableID)
	return buf
}

// PrefixEnd returns the smallest byte slice strictly greater than every
// key starting with p. Returns nil if p is all-0xFF (unbounded).
func PrefixEnd(p []byte) []byte {
	if len(p) == 0 {
		return nil
	}
	// Check if already all-0xFF.
	allFF := true
	for _, b := range p {
		if b != 0xFF {
			allFF = false
			break
		}
	}
	if allFF {
		return nil
	}
	end := make([]byte, len(p))
	copy(end, p)
	// Increment the last byte.
	for i := len(end) - 1; i >= 0; i-- {
		end[i]++
		if end[i] != 0 {
			break
		}
	}
	return end
}
