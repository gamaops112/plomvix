// Package key provides the single authoritative key encoding for the Plomvix
// SQL engine. It encodes int64, uint64, string, and raw byte slice values into
// sort-safe or storage composite keys with zero internal imports.
package key

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
)

type Kind uint8

const (
	KindUint64 Kind = 1
	KindInt64  Kind = 2
	KindString Kind = 3
	KindBytes  Kind = 4
)

type Field struct {
	Kind   Kind
	Offset int
	Length int
}

type Key struct {
	data   []byte
	fields []Field
}

var (
	ErrNullByteInString    = errors.New("sql/key: string contains null byte")
	ErrUnsupportedType     = errors.New("sql/key: unsupported field type")
	ErrUnsupportedSortType = errors.New("sql/key: type not allowed in sort composite")
	ErrInvalidKey          = errors.New("sql/key: invalid key")
	ErrKindMismatch        = errors.New("sql/key: kind mismatch")
	ErrNotComposite        = errors.New("sql/key: key is not composite")
)

func (k Key) Bytes() []byte {
	if k.data == nil { return nil }
	out := make([]byte, len(k.data)); copy(out, k.data); return out
}
func (k Key) Compare(other Key) int { return bytes.Compare(k.data, other.data) }
func (k Key) Fields() []Field {
	if k.fields == nil { return nil }
	out := make([]Field, len(k.fields)); copy(out, k.fields); return out
}

func EncodeUint64(v uint64) Key {
	d := make([]byte, 8); binary.BigEndian.PutUint64(d, v)
	return Key{data: d, fields: []Field{{Kind: KindUint64, Offset: 0, Length: 8}}}
}
func EncodeInt64(v int64) Key {
	u := uint64(v) ^ (1 << 63)
	d := make([]byte, 8); binary.BigEndian.PutUint64(d, u)
	return Key{data: d, fields: []Field{{Kind: KindInt64, Offset: 0, Length: 8}}}
}
func EncodeString(s string) (Key, error) {
	if strings.ContainsRune(s, 0) { return Key{}, ErrNullByteInString }
	d := append([]byte(s), 0x00)
	return Key{data: d, fields: []Field{{Kind: KindString, Offset: 0, Length: len(d)}}}, nil
}
func EncodeBytes(b []byte) Key {
	d := make([]byte, len(b)); copy(d, b)
	return Key{data: d, fields: []Field{{Kind: KindBytes, Offset: 0, Length: len(b)}}}
}

func requireScalar(k Key) (Field, error) {
	if len(k.fields) == 0 && len(k.data) == 0 { return Field{}, ErrInvalidKey }
	if len(k.fields) != 1 { return Field{}, ErrInvalidKey }
	return k.fields[0], nil
}

func DecodeUint64(k Key) (uint64, error) {
	f, err := requireScalar(k)
	if err != nil { return 0, err }
	if f.Kind != KindUint64 { return 0, ErrKindMismatch }
	if f.Length != 8 || f.Offset+8 > len(k.data) { return 0, ErrInvalidKey }
	return binary.BigEndian.Uint64(k.data[f.Offset:]), nil
}
func DecodeInt64(k Key) (int64, error) {
	f, err := requireScalar(k)
	if err != nil { return 0, err }
	if f.Kind != KindInt64 { return 0, ErrKindMismatch }
	if f.Length != 8 || f.Offset+8 > len(k.data) { return 0, ErrInvalidKey }
	u := binary.BigEndian.Uint64(k.data[f.Offset:])
	return int64(u ^ (1 << 63)), nil
}
func DecodeString(k Key) (string, error) {
	f, err := requireScalar(k)
	if err != nil { return "", err }
	if f.Kind != KindString { return "", ErrKindMismatch }
	if f.Offset+f.Length > len(k.data) { return "", ErrInvalidKey }
	b := k.data[f.Offset : f.Offset+f.Length]
	if len(b) == 0 || b[len(b)-1] != 0x00 { return "", ErrInvalidKey }
	return string(b[:len(b)-1]), nil
}
func DecodeBytes(k Key) ([]byte, error) {
	f, err := requireScalar(k)
	if err != nil { return nil, err }
	if f.Kind != KindBytes { return nil, ErrKindMismatch }
	if f.Length > 0 && f.Offset+f.Length > len(k.data) { return nil, ErrInvalidKey }
	raw := k.data[f.Offset : f.Offset+f.Length]
	out := make([]byte, len(raw)); copy(out, raw); return out, nil
}

func ParseKey(data []byte, kinds []Kind) (Key, error) {
	if len(kinds) == 0 { return Key{}, ErrInvalidKey }
	pos := 0
	fields := make([]Field, 0, len(kinds))
	for i, kind := range kinds {
		switch kind {
		case KindUint64, KindInt64:
			if pos+8 > len(data) { return Key{}, ErrInvalidKey }
			fields = append(fields, Field{Kind: kind, Offset: pos, Length: 8})
			pos += 8
		case KindString:
			end := bytes.IndexByte(data[pos:], 0x00)
			if end < 0 { return Key{}, ErrInvalidKey }
			fields = append(fields, Field{Kind: KindString, Offset: pos, Length: end + 1})
			pos += end + 1
		case KindBytes:
			if i != len(kinds)-1 { return Key{}, ErrInvalidKey }
			fields = append(fields, Field{Kind: KindBytes, Offset: pos, Length: len(data) - pos})
			pos = len(data)
		default:
			return Key{}, ErrInvalidKey
		}
	}
	if pos != len(data) && kinds[len(kinds)-1] != KindBytes { return Key{}, ErrInvalidKey }
	d := make([]byte, len(data)); copy(d, data)
	return Key{data: d, fields: fields}, nil
}

func EncodeSortComposite(fields ...any) (Key, error) {
	var d []byte
	fs := make([]Field, 0, len(fields))
	pos := 0
	for _, f := range fields {
		switch v := f.(type) {
		case uint64:
			b := make([]byte, 8); binary.BigEndian.PutUint64(b, v)
			d = append(d, b...)
			fs = append(fs, Field{Kind: KindUint64, Offset: pos, Length: 8})
		case int64:
			u := uint64(v) ^ (1 << 63)
			b := make([]byte, 8); binary.BigEndian.PutUint64(b, u)
			d = append(d, b...)
			fs = append(fs, Field{Kind: KindInt64, Offset: pos, Length: 8})
		case string, []byte:
			return Key{}, ErrUnsupportedSortType
		default:
			return Key{}, ErrUnsupportedType
		}
		pos += 8
	}
	return Key{data: d, fields: fs}, nil
}

func DecodeSortComposite(k Key) ([]any, error) {
	if len(k.fields) == 0 { return nil, ErrNotComposite }
	vals := make([]any, len(k.fields))
	for i, f := range k.fields {
		if f.Offset+f.Length > len(k.data) { return nil, ErrInvalidKey }
		switch f.Kind {
		case KindUint64:
			if f.Length != 8 { return nil, ErrInvalidKey }
			vals[i] = binary.BigEndian.Uint64(k.data[f.Offset:])
		case KindInt64:
			if f.Length != 8 { return nil, ErrInvalidKey }
			u := binary.BigEndian.Uint64(k.data[f.Offset:])
			vals[i] = int64(u ^ (1 << 63))
		default:
			return nil, ErrKindMismatch
		}
	}
	return vals, nil
}

// EncodeStorageComposite encodes variable-length fields using
// length-prefix framing. The resulting key is NOT sort-safe.
// Do not use storage composite keys as index keys or scan boundaries.
func EncodeStorageComposite(fields ...any) (Key, error) {
	var d []byte
	fs := make([]Field, 0, len(fields))
	for _, f := range fields {
		switch v := f.(type) {
		case uint64:
			appendLen4(&d, 8); off := len(d)
			cb := make([]byte, 8); binary.BigEndian.PutUint64(cb, v)
			d = append(d, cb...)
			fs = append(fs, Field{Kind: KindUint64, Offset: off, Length: 8})
		case int64:
			u := uint64(v) ^ (1 << 63)
			appendLen4(&d, 8); off := len(d)
			cb := make([]byte, 8); binary.BigEndian.PutUint64(cb, u)
			d = append(d, cb...)
			fs = append(fs, Field{Kind: KindInt64, Offset: off, Length: 8})
		case string:
			if strings.ContainsRune(v, 0) { return Key{}, ErrNullByteInString }
			b := []byte(v)
			appendLen4(&d, len(b)); off := len(d)
			d = append(d, b...)
			fs = append(fs, Field{Kind: KindString, Offset: off, Length: len(b)})
		case []byte:
			appendLen4(&d, len(v)); off := len(d)
			d = append(d, v...)
			fs = append(fs, Field{Kind: KindBytes, Offset: off, Length: len(v)})
		default:
			return Key{}, ErrUnsupportedType
		}
	}
	return Key{data: d, fields: fs}, nil
}

func appendLen4(d *[]byte, n int) {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(n))
	*d = append(*d, b...)
}

func DecodeStorageComposite(k Key) ([]any, error) {
	if len(k.fields) == 0 { return nil, ErrNotComposite }
	vals := make([]any, len(k.fields))
	for i, f := range k.fields {
		if f.Offset+f.Length > len(k.data) { return nil, ErrInvalidKey }
		switch f.Kind {
		case KindUint64:
			if f.Length != 8 { return nil, ErrInvalidKey }
			vals[i] = binary.BigEndian.Uint64(k.data[f.Offset:])
		case KindInt64:
			if f.Length != 8 { return nil, ErrInvalidKey }
			u := binary.BigEndian.Uint64(k.data[f.Offset:])
			vals[i] = int64(u ^ (1 << 63))
		case KindString:
			vals[i] = string(k.data[f.Offset : f.Offset+f.Length])
		case KindBytes:
			b := make([]byte, f.Length); copy(b, k.data[f.Offset:f.Offset+f.Length])
			vals[i] = b
		default:
			return nil, ErrKindMismatch
		}
	}
	return vals, nil
}

func ParseStorageCompositeKey(data []byte, kinds []Kind) (Key, error) {
	if len(kinds) == 0 { return Key{}, ErrInvalidKey }
	pos := 0
	fields := make([]Field, 0, len(kinds))
	for _, kind := range kinds {
		if pos+4 > len(data) { return Key{}, ErrInvalidKey }
		length := int(binary.BigEndian.Uint32(data[pos:]))
		pos += 4
		if pos+length > len(data) { return Key{}, ErrInvalidKey }
		fields = append(fields, Field{Kind: kind, Offset: pos, Length: length})
		pos += length
	}
	if pos != len(data) { return Key{}, ErrInvalidKey }
	d := make([]byte, len(data)); copy(d, data)
	return Key{data: d, fields: fields}, nil
}
