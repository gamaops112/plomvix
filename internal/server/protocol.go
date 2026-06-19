// Package server provides a PostgreSQL Wire Protocol v3.0 compatible
// network server for Plomvix. It handles TCP connections, the startup
// handshake (SSL negotiation, authentication), and the Simple Query protocol.
package server

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/plomvix/plomvix/internal/engine"
)

// MsgType identifies a PostgreSQL v3 protocol message.
type MsgType byte

const (
	MsgQuery                 MsgType = 'Q'
	MsgRowDescription        MsgType = 'T'
	MsgDataRow               MsgType = 'D'
	MsgCommandComplete       MsgType = 'C'
	MsgReadyForQuery         MsgType = 'Z'
	MsgAuthenticationRequest MsgType = 'R'
	MsgErrorResponse         MsgType = 'E'
	MsgParameterStatus       MsgType = 'S'
	MsgBackendKeyData        MsgType = 'K'
	MsgTerminate             MsgType = 'X'
	MsgSSLRequestN           byte    = 'N'
)

// pgWriter wraps a raw network connection and formats PG protocol packets.
type pgWriter struct {
	w io.Writer
}

// newPGWriter creates a new pgWriter.
func newPGWriter(w io.Writer) *pgWriter { return &pgWriter{w: w} }

// WritePacket formats and transmits a standard PG packet frame.
// Format: [1-byte type][4-byte length including self][payload]
func (pw *pgWriter) WritePacket(t MsgType, payload []byte) error {
	length := int32(len(payload) + 4)
	header := make([]byte, 5)
	header[0] = byte(t)
	binary.BigEndian.PutUint32(header[1:5], uint32(length))
	if _, err := pw.w.Write(header); err != nil {
		return err
	}
	_, err := pw.w.Write(payload)
	return err
}

// WriteByte writes a single byte (used for SSL 'N' response).
func (pw *pgWriter) WriteByte(b byte) error {
	_, err := pw.w.Write([]byte{b})
	return err
}

// PG OID constants for type mapping.
type PGOID int32

const (
	OIDBool    PGOID = 16
	OIDInt64   PGOID = 20
	OIDFloat64 PGOID = 701
	OIDString  PGOID = 1043
	OIDNull    PGOID = 0
)

func engineTypeToOID(t engine.Type) PGOID {
	switch t {
	case engine.TypeBool:
		return OIDBool
	case engine.TypeInt64:
		return OIDInt64
	case engine.TypeUint64:
		return OIDInt64
	case engine.TypeFloat64:
		return OIDFloat64
	case engine.TypeString:
		return OIDString
	default:
		return OIDNull
	}
}

// WriteRowDescription serializes a RowDescription ('T') packet.
func (pw *pgWriter) WriteRowDescription(schema engine.Schema) error {
	var payload []byte
	// Number of columns (2 bytes, big-endian)
	colCount := make([]byte, 2)
	binary.BigEndian.PutUint16(colCount, uint16(len(schema.Columns)))
	payload = append(payload, colCount...)

	for _, col := range schema.Columns {
		// Column name (null-terminated string)
		payload = append(payload, []byte(col.Name)...)
		payload = append(payload, 0)
		// Table OID (4 bytes, 0 = not a table column)
		payload = append(payload, 0, 0, 0, 0)
		// Column attribute index (2 bytes, 0)
		payload = append(payload, 0, 0)
		// Type OID (4 bytes)
		oid := make([]byte, 4)
		binary.BigEndian.PutUint32(oid, uint32(engineTypeToOID(col.Type)))
		payload = append(payload, oid...)
		// Type size (2 bytes, -1 for variable-length)
		typeSize := int16(-1)
		if col.Type == engine.TypeInt64 || col.Type == engine.TypeUint64 || col.Type == engine.TypeFloat64 || col.Type == engine.TypeBool {
			typeSize = 8
		}
		ts := make([]byte, 2)
		binary.BigEndian.PutUint16(ts, uint16(typeSize))
		payload = append(payload, ts...)
		// Type modifier (4 bytes, -1)
		payload = append(payload, 0xFF, 0xFF, 0xFF, 0xFF)
		// Format code (2 bytes, 0 = text)
		payload = append(payload, 0, 0)
	}
	return pw.WritePacket(MsgRowDescription, payload)
}

// WriteDataRow serializes a DataRow ('D') packet.
func (pw *pgWriter) WriteDataRow(row engine.Row) error {
	var payload []byte
	// Field count (2 bytes)
	fc := make([]byte, 2)
	binary.BigEndian.PutUint16(fc, uint16(len(row.Datums)))
	payload = append(payload, fc...)

	for _, d := range row.Datums {
		if d.Value == nil {
			// NULL: length = -1 (4 bytes)
			payload = append(payload, 0xFF, 0xFF, 0xFF, 0xFF)
			continue
		}
		text := datumToText(d)
		// Field length (4 bytes)
		fl := make([]byte, 4)
		binary.BigEndian.PutUint32(fl, uint32(len(text)))
		payload = append(payload, fl...)
		// Field data
		payload = append(payload, []byte(text)...)
	}
	return pw.WritePacket(MsgDataRow, payload)
}

// datumToText converts an engine.Datum to its text representation.
func datumToText(d engine.Datum) string {
	if d.Value == nil {
		return ""
	}
	switch v := d.Value.(type) {
	case int64:
		return fmt.Sprintf("%d", v)
	case uint64:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%f", v)
	case bool:
		if v {
			return "t"
		}
		return "f"
	case string:
		return v
	case []byte:
		return string(v)
	}
	return ""
}

// WriteCommandComplete serializes a CommandComplete ('C') packet.
func (pw *pgWriter) WriteCommandComplete(tag string) error {
	payload := append([]byte(tag), 0)
	return pw.WritePacket(MsgCommandComplete, payload)
}

// WriteReadyForQuery sends the ReadyForQuery ('Z') packet.
func (pw *pgWriter) WriteReadyForQuery(status byte) error {
	return pw.WritePacket(MsgReadyForQuery, []byte{status})
}

// WriteErrorResponse sends an ErrorResponse ('E') packet.
func (pw *pgWriter) WriteErrorResponse(severity, code, message string) error {
	var payload []byte
	// Severity 'S'
	payload = append(payload, 'S')
	payload = append(payload, []byte(severity)...)
	payload = append(payload, 0)
	// SQLSTATE 'C'
	payload = append(payload, 'C')
	payload = append(payload, []byte(code)...)
	payload = append(payload, 0)
	// Message 'M'
	payload = append(payload, 'M')
	payload = append(payload, []byte(message)...)
	payload = append(payload, 0)
	// Terminating null
	payload = append(payload, 0)
	return pw.WritePacket(MsgErrorResponse, payload)
}

// WriteAuthOK sends an AuthenticationOK (trust, code 0) message.
func (pw *pgWriter) WriteAuthOK() error {
	return pw.WritePacket(MsgAuthenticationRequest, []byte{0, 0, 0, 0})
}

// WriteParameterStatus sends a ParameterStatus ('S') packet.
func (pw *pgWriter) WriteParameterStatus(key, value string) error {
	payload := append([]byte(key), 0)
	payload = append(payload, []byte(value)...)
	payload = append(payload, 0)
	return pw.WritePacket(MsgParameterStatus, payload)
}

// WriteBackendKeyData sends BackendKeyData ('K') packet.
func (pw *pgWriter) WriteBackendKeyData(pid, key uint32) error {
	data := make([]byte, 8)
	binary.BigEndian.PutUint32(data[0:4], pid)
	binary.BigEndian.PutUint32(data[4:8], key)
	return pw.WritePacket(MsgBackendKeyData, data)
}
