// Package key implements the key-encoding layer for the Plomvix sql_engine.
// It turns logical table identifiers and primary-key column values into
// order-preserving byte keys, and decodes them back.
package key

import (
	"bytes"
	"encoding/binary"
	"errors"
)

// Keyspace tags.
const (
	TagTableData byte = 0x01
	TagMetadata  byte = 0x02
	TagIndex     byte = 0x03
)

// Kind enumerates supported key-column types.
type Kind uint8

const (
	KindNull Kind = iota
	KindBool
	KindInt64
	KindUint64
	KindString
	KindBytes
)

// Value is one primary-key column value.
type Value struct {
	kind Kind
	b    []byte
}

// Sentinel errors.
var (
	ErrEmptyKey      = errors.New("key: empty input")
	ErrBadTag        = errors.New("key: unknown keyspace tag")
	ErrBadTypeTag    = errors.New("key: unknown column type tag")
	ErrKindMismatch  = errors.New("key: decoded column kind does not match expected")
	ErrTruncated     = errors.New("key: truncated input")
	ErrBadField      = errors.New("key: malformed variable-length field")
	ErrTrailingBytes = errors.New("key: trailing bytes after decode")
	ErrNoPKColumns   = errors.New("key: at least one pk column required")
	ErrNotCanonical  = errors.New("key: non-canonical encoding")
)

func Null() Value  { return Value{kind: KindNull} }
func Bool(v bool) Value {
	b := [1]byte{0}
	if v { b[0] = 1 }
	return Value{kind: KindBool, b: b[:]}
}
func Int64(v int64) Value {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return Value{kind: KindInt64, b: b}
}
func Uint64(v uint64) Value {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	return Value{kind: KindUint64, b: b}
}
func String(v string) Value { return Value{kind: KindString, b: []byte(v)} }
func Bytes(v []byte) Value {
	b := make([]byte, len(v))
	copy(b, v)
	return Value{kind: KindBytes, b: b}
}

func (val Value) Kind() Kind { return val.kind }

func (val Value) AsBool() (bool, bool) {
	if val.kind != KindBool || len(val.b) != 1 { return false, false }
	return val.b[0] == 1, true
}
func (val Value) AsInt64() (int64, bool) {
	if val.kind != KindInt64 || len(val.b) < 8 { return 0, false }
	return int64(binary.BigEndian.Uint64(val.b)), true
}
func (val Value) AsUint64() (uint64, bool) {
	if val.kind != KindUint64 || len(val.b) < 8 { return 0, false }
	return binary.BigEndian.Uint64(val.b), true
}
func (val Value) AsString() (string, bool) {
	if val.kind != KindString { return "", false }
	return string(val.b), true
}
func (val Value) AsBytes() ([]byte, bool) {
	if val.kind != KindBytes { return nil, false }
	b := make([]byte, len(val.b))
	copy(b, val.b)
	return b, true
}

func (val Value) Equal(other Value) bool {
	if val.kind != other.kind { return false }
	if val.kind == KindNull { return true }
	return string(val.b) == string(other.b)
}

// encodeValue returns the order-preserving byte encoding of val:
// 1-byte type tag followed by the encoded payload.
func encodeValue(val Value) []byte {
	switch val.kind {
	case KindNull:
		return []byte{0x10}
	case KindBool:
		if val.b[0] == 1 {
			return []byte{0x20, 1}
		}
		return []byte{0x20, 0}
	case KindInt64:
		out := make([]byte, 9)
		out[0] = 0x30
		v := uint64(val.b[0])<<56 | uint64(val.b[1])<<48 | uint64(val.b[2])<<40 | uint64(val.b[3])<<32 |
			uint64(val.b[4])<<24 | uint64(val.b[5])<<16 | uint64(val.b[6])<<8 | uint64(val.b[7])
		binary.BigEndian.PutUint64(out[1:], v^0x8000000000000000)
		return out
	case KindUint64:
		out := make([]byte, 9)
		out[0] = 0x40
		copy(out[1:], val.b)
		return out
	case KindString, KindBytes:
		tag := byte(0x50)
		if val.kind == KindBytes { tag = 0x60 }
		var buf []byte
		buf = append(buf, tag)
		for _, c := range val.b {
			if c == 0x00 {
				buf = append(buf, 0x00, 0xFF)
			} else {
				buf = append(buf, c)
			}
		}
		buf = append(buf, 0x00, 0x01) // terminator
		return buf
	default:
		return nil
	}
}

// decodeValue reads one value from the front of b, expecting the given Kind.
// Returns the Value, number of bytes consumed, and any error.
func decodeValue(b []byte, expected Kind) (Value, int, error) {
	if len(b) == 0 { return Value{}, 0, ErrTruncated }

	tag := b[0]
	// Validate tag matches expected kind
	switch tag {
	case 0x10:
		if expected != KindNull { return Value{}, 0, ErrKindMismatch }
		return Null(), 1, nil
	case 0x20:
		if expected != KindBool { return Value{}, 0, ErrKindMismatch }
		if len(b) < 2 { return Value{}, 0, ErrTruncated }
		return Bool(b[1] == 1), 2, nil
	case 0x30:
		if expected != KindInt64 { return Value{}, 0, ErrKindMismatch }
		if len(b) < 9 { return Value{}, 0, ErrTruncated }
		v := binary.BigEndian.Uint64(b[1:9]) ^ 0x8000000000000000
		return Int64(int64(v)), 9, nil
	case 0x40:
		if expected != KindUint64 { return Value{}, 0, ErrKindMismatch }
		if len(b) < 9 { return Value{}, 0, ErrTruncated }
		return Uint64(binary.BigEndian.Uint64(b[1:9])), 9, nil
	case 0x50, 0x60:
		expKind := KindString
		if tag == 0x60 { expKind = KindBytes }
		if expected != expKind { return Value{}, 0, ErrKindMismatch }
		return decodeVarLen(b[1:], expKind)
	default:
		return Value{}, 0, ErrBadTypeTag
	}
}

// decodeVarLen decodes an escape-terminated string or bytes from b.
func decodeVarLen(b []byte, kind Kind) (Value, int, error) {
	var raw []byte
	i := 0
	for i < len(b) {
		if b[i] == 0x00 {
			if i+1 >= len(b) { return Value{}, 0, ErrTruncated }
			switch b[i+1] {
			case 0xFF:
				raw = append(raw, 0x00)
				i += 2
			case 0x01:
				// terminator found
				i += 2
				if kind == KindString {
					return String(string(raw)), i + 1, nil // +1 for original tag
				}
				return Bytes(raw), i + 1, nil
			default:
				return Value{}, 0, ErrBadField
			}
		} else {
			raw = append(raw, b[i])
			i++
		}
	}
	return Value{}, 0, ErrTruncated
}

// EncodeTableRowKey builds [0x01][tableID][encoded pk][version].
func EncodeTableRowKey(tableID uint64, pk []Value, version uint64) ([]byte, error) {
	if len(pk) == 0 { return nil, ErrNoPKColumns }
	var buf []byte
	buf = append(buf, TagTableData)
	buf = append(buf, 0, 0, 0, 0, 0, 0, 0, 0)
	binary.BigEndian.PutUint64(buf[1:9], tableID)
	for _, col := range pk {
		buf = append(buf, encodeValue(col)...)
	}
	var vb [8]byte
	binary.BigEndian.PutUint64(vb[:], ^version)
	buf = append(buf, vb[:]...)
	return buf, nil
}

// DecodeTableRowKey parses a row key. expectedKinds supplies the PK column
// types from the table schema.
func DecodeTableRowKey(b []byte, expectedKinds []Kind) (tableID uint64, pk []Value, version uint64, err error) {
	if len(b) == 0 { return 0, nil, 0, ErrEmptyKey }
	if len(expectedKinds) == 0 { return 0, nil, 0, ErrNoPKColumns }
	if b[0] != TagTableData { return 0, nil, 0, ErrBadTag }
	if len(b) < 9 { return 0, nil, 0, ErrTruncated }
	tableID = binary.BigEndian.Uint64(b[1:9])
	pos := 9
	pk = make([]Value, 0, len(expectedKinds))
	for _, kind := range expectedKinds {
		if pos >= len(b) { return 0, nil, 0, ErrTruncated }
		val, consumed, decErr := decodeValue(b[pos:], kind)
		if decErr != nil { return 0, nil, 0, decErr }
		pk = append(pk, val)
		pos += consumed
	}
	if len(b)-pos < 8 { return 0, nil, 0, ErrTruncated }
	version = ^binary.BigEndian.Uint64(b[pos:])
	pos += 8
	if pos != len(b) { return 0, nil, 0, ErrTrailingBytes }
	return tableID, pk, version, nil
}

// TablePrefix returns [0x01][tableID]; bounds all rows of a table.
func TablePrefix(tableID uint64) []byte {
	buf := make([]byte, 9)
	buf[0] = TagTableData
	binary.BigEndian.PutUint64(buf[1:], tableID)
	return buf
}

// IsCanonical reports whether b is in canonical form: it decodes cleanly with
// expectedKinds AND re-encoding the decoded result produces byte-identical output.
func IsCanonical(b []byte, expectedKinds []Kind) (bool, error) {
	id, pk, ver, err := DecodeTableRowKey(b, expectedKinds)
	if err != nil {
		return false, err
	}
	reenc, err := EncodeTableRowKey(id, pk, ver)
	if err != nil {
		return false, err
	}
	if !bytes.Equal(b, reenc) {
		return false, ErrNotCanonical
	}
	return true, nil
}

// PrefixEnd returns the smallest byte slice strictly greater than every key
// with prefix p. Returns nil if p is empty or all 0xFF (unbounded).
func PrefixEnd(p []byte) []byte {
	if len(p) == 0 {
		return nil
	}
	end := make([]byte, len(p))
	copy(end, p)
	for i := len(end) - 1; i >= 0; i-- {
		if end[i] < 0xFF {
			end[i]++
			return end[:i+1]
		}
	}
	return nil // all 0xFF, unbounded
}

// TableRange returns [start, end) bounding all keys for tableID.
func TableRange(tableID uint64) (start, end []byte) {
	start = TablePrefix(tableID)
	end = PrefixEnd(start)
	return
}
