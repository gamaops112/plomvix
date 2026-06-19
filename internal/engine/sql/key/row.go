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

// EncodeTableRowPrefix builds [0x01][tableID][encoded pk] without the version
// suffix. Used to scan all versions of a given PK.
func EncodeTableRowPrefix(tableID uint64, pk []Value) ([]byte, error) {
	if len(pk) == 0 {
		return nil, ErrNoPKColumns
	}
	// Build the same prefix as EncodeTableRowKey but omit the version.
	size := 1 + 8
	for _, v := range pk {
		switch v.kind {
		case KindUint64, KindInt64:
			size += 8
		case KindString:
			size += len(v.s) + 1 // +1 for null terminator
		case KindBytes:
			size += len(v.b)
		}
	}
	buf := make([]byte, 0, size)
	buf = append(buf, tableRowTag)

	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, tableID)
	buf = append(buf, b...)

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
	return buf, nil
}

// DecodeTableRowKey parses a table row key. The caller must supply expectedKinds
// (from the table schema): the exact ordered Kinds of the PK columns.
// Returns tableID, pk values, version, and error.
func DecodeTableRowKey(b []byte, expectedKinds []Kind) (tableID uint64, pk []Value, version uint64, err error) {
	if len(b) < 1+8 { // tag + tableID minimum
		return 0, nil, 0, ErrTruncated
	}
	if b[0] != tableRowTag {
		return 0, nil, 0, ErrBadTag
	}
	pos := 1
	tableID = binary.BigEndian.Uint64(b[pos:])
	pos += 8

	pk = make([]Value, len(expectedKinds))
	for i, k := range expectedKinds {
		if pos >= len(b) {
			return 0, nil, 0, ErrTruncated
		}
		switch k {
		case KindUint64:
			if pos+8 > len(b) {
				return 0, nil, 0, ErrTruncated
			}
			v := binary.BigEndian.Uint64(b[pos:])
			pk[i] = Uint64(v)
			pos += 8
		case KindInt64:
			if pos+8 > len(b) {
				return 0, nil, 0, ErrTruncated
			}
			u := binary.BigEndian.Uint64(b[pos:])
			pk[i] = Int64(int64(u ^ (1 << 63)))
			pos += 8
		case KindString:
			// Find null terminator.
			end := -1
			for j := pos; j < len(b); j++ {
				if b[j] == 0x00 {
					end = j
					break
				}
			}
			if end < 0 {
				return 0, nil, 0, ErrTruncated
			}
			pk[i] = String(string(b[pos:end]))
			pos = end + 1
		case KindBytes:
			// Bytes is the last kind; consume rest minus version.
			if i != len(expectedKinds)-1 {
				return 0, nil, 0, ErrInvalidKey
			}
			// Remaining bytes minus 8-byte version.
			if pos+8 > len(b) {
				return 0, nil, 0, ErrTruncated
			}
			dataLen := len(b) - pos - 8
			pk[i] = Bytes(b[pos : pos+dataLen])
			pos += dataLen
		default:
			return 0, nil, 0, ErrInvalidKey
		}
	}

	if pos+8 != len(b) {
		return 0, nil, 0, ErrTruncated
	}
	version = binary.BigEndian.Uint64(b[pos:])
	return tableID, pk, version, nil
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
