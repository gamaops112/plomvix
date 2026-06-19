// Package schema provides binary encoding/decoding for engine.Schema payloads
// stored in the catalog. The format is a simple length-prefixed binary layout:
//
//	[num_cols: uint16]
//	( [name_len: uint16] [name: bytes] [type: uint8] [flags: uint8]
//	  [default_type: uint8]? [default_val: bytes]? )...
//
// Flags byte: bit 0 = NotNull, bit 1 = has DefaultValue.
// If has DefaultValue bit is set, default_type (uint8) and default_val follow.
// default_val encoding: TypeInt64/TypeUint64 = 8 bytes big-endian,
// TypeFloat64 = 8 bytes IEEE 754, TypeBool = 1 byte (0/1),
// TypeString = [len: uint16] [bytes], TypeBytes = [len: uint16] [bytes],
// TypeNull = no value (zero-length).
package schema

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/plomvix/plomvix/internal/engine"
)

const (
	flagNotNull    byte = 0x01
	flagHasDefault byte = 0x02
)

// Encode serializes an engine.Schema into a binary payload.
func Encode(s engine.Schema) ([]byte, error) {
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, uint16(len(s.Columns)))
	for _, col := range s.Columns {
		nameBytes := []byte(col.Name)
		nameLen := make([]byte, 2)
		binary.BigEndian.PutUint16(nameLen, uint16(len(nameBytes)))
		buf = append(buf, nameLen...)
		buf = append(buf, nameBytes...)
		buf = append(buf, byte(col.Type))
		// Flags byte.
		var flags byte
		if col.NotNull {
			flags |= flagNotNull
		}
		if col.DefaultValue != nil {
			flags |= flagHasDefault
		}
		buf = append(buf, flags)
		// Default value if present.
		if col.DefaultValue != nil {
			if err := encodeDatum(&buf, *col.DefaultValue); err != nil {
				return nil, err
			}
		}
	}
	return buf, nil
}

func encodeDatum(buf *[]byte, d engine.Datum) error {
	*buf = append(*buf, byte(d.Type))
	switch d.Type {
	case engine.TypeInt64:
		v, _ := d.Value.(int64)
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, uint64(v))
		*buf = append(*buf, b...)
	case engine.TypeUint64:
		v, _ := d.Value.(uint64)
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, v)
		*buf = append(*buf, b...)
	case engine.TypeFloat64:
		v, _ := d.Value.(float64)
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, math.Float64bits(v))
		*buf = append(*buf, b...)
	case engine.TypeBool:
		v, _ := d.Value.(bool)
		if v {
			*buf = append(*buf, 1)
		} else {
			*buf = append(*buf, 0)
		}
	case engine.TypeString:
		v, _ := d.Value.(string)
		l := make([]byte, 2)
		binary.BigEndian.PutUint16(l, uint16(len(v)))
		*buf = append(*buf, l...)
		*buf = append(*buf, []byte(v)...)
	case engine.TypeBytes:
		v, _ := d.Value.([]byte)
		l := make([]byte, 2)
		binary.BigEndian.PutUint16(l, uint16(len(v)))
		*buf = append(*buf, l...)
		*buf = append(*buf, v...)
	case engine.TypeNull:
		// No additional bytes.
	default:
		return fmt.Errorf("schema: unsupported default value type %d", d.Type)
	}
	return nil
}

// Decode parses a binary payload into an engine.Schema.
func Decode(payload []byte) (engine.Schema, error) {
	if len(payload) < 2 {
		return engine.Schema{}, fmt.Errorf("schema: payload too short for column count")
	}
	numCols := int(binary.BigEndian.Uint16(payload[:2]))
	pos := 2
	cols := make([]engine.Column, 0, numCols)
	for i := 0; i < numCols; i++ {
		if pos+2 > len(payload) {
			return engine.Schema{}, fmt.Errorf("schema: truncated at name length for column %d", i)
		}
		nameLen := int(binary.BigEndian.Uint16(payload[pos:]))
		pos += 2
		if pos+nameLen > len(payload) {
			return engine.Schema{}, fmt.Errorf("schema: truncated at name for column %d", i)
		}
		name := string(payload[pos : pos+nameLen])
		pos += nameLen
		if pos+1 >= len(payload) {
			// Old format: just type, no flags. Tolerate.
			if pos >= len(payload) {
				return engine.Schema{}, fmt.Errorf("schema: truncated at type for column %d", i)
			}
			colType := engine.Type(payload[pos])
			pos++
			cols = append(cols, engine.Column{Name: name, Type: colType})
			continue
		}
		colType := engine.Type(payload[pos])
		pos++
		// Check if we have a flags byte (new format) or we're at end (old format).
		if pos >= len(payload) {
			cols = append(cols, engine.Column{Name: name, Type: colType})
			continue
		}
		flags := payload[pos]
		pos++
		col := engine.Column{Name: name, Type: colType, NotNull: flags&flagNotNull != 0}
		if flags&flagHasDefault != 0 {
			dv, n, err := decodeDatum(payload[pos:])
			if err != nil {
				return engine.Schema{}, fmt.Errorf("schema: default value for column %q: %w", name, err)
			}
			col.DefaultValue = &dv
			pos += n
		}
		cols = append(cols, col)
	}
	return engine.Schema{Columns: cols}, nil
}

func decodeDatum(b []byte) (engine.Datum, int, error) {
	if len(b) < 1 {
		return engine.Datum{}, 0, fmt.Errorf("schema: truncated at default type")
	}
	t := engine.Type(b[0])
	switch t {
	case engine.TypeInt64, engine.TypeUint64, engine.TypeFloat64:
		if len(b) < 9 {
			return engine.Datum{}, 0, fmt.Errorf("schema: truncated at 8-byte default value")
		}
		u := binary.BigEndian.Uint64(b[1:9])
		switch t {
		case engine.TypeInt64:
			return engine.Datum{Type: t, Value: int64(u)}, 9, nil
		case engine.TypeUint64:
			return engine.Datum{Type: t, Value: u}, 9, nil
		case engine.TypeFloat64:
			return engine.Datum{Type: t, Value: math.Float64frombits(u)}, 9, nil
		}
	case engine.TypeBool:
		if len(b) < 2 {
			return engine.Datum{}, 0, fmt.Errorf("schema: truncated at bool default value")
		}
		return engine.Datum{Type: t, Value: b[1] != 0}, 2, nil
	case engine.TypeString, engine.TypeBytes:
		if len(b) < 3 {
			return engine.Datum{}, 0, fmt.Errorf("schema: truncated at string/bytes default length")
		}
		l := int(binary.BigEndian.Uint16(b[1:3]))
		if len(b) < 3+l {
			return engine.Datum{}, 0, fmt.Errorf("schema: truncated at string/bytes default value")
		}
		if t == engine.TypeString {
			return engine.Datum{Type: t, Value: string(b[3 : 3+l])}, 3 + l, nil
		}
		cp := make([]byte, l)
		copy(cp, b[3:3+l])
		return engine.Datum{Type: t, Value: cp}, 3 + l, nil
	case engine.TypeNull:
		return engine.Datum{Type: t, Value: nil}, 1, nil
	}
	return engine.Datum{}, 0, fmt.Errorf("schema: unknown default type %d", t)
}
