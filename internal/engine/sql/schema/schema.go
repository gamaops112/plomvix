// Package schema provides binary encoding/decoding for engine.Schema payloads
// stored in the catalog. The format is a simple length-prefixed binary layout:
//
//	[num_cols: uint16]
//	( [name_len: uint16] [name: bytes] [type: uint8] )...
package schema

import (
	"encoding/binary"
	"fmt"

	"github.com/plomvix/plomvix/internal/engine"
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
	}
	return buf, nil
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
		if pos >= len(payload) {
			return engine.Schema{}, fmt.Errorf("schema: truncated at type for column %d", i)
		}
		colType := engine.Type(payload[pos])
		pos++
		cols = append(cols, engine.Column{Name: name, Type: colType})
	}
	if pos != len(payload) {
		return engine.Schema{}, fmt.Errorf("schema: extra bytes after columns")
	}
	return engine.Schema{Columns: cols}, nil
}
