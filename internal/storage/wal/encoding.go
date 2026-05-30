package wal

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

var ErrCorruptEntry = errors.New("corrupt WAL entry")

func EncodeEntry(e *Entry) ([]byte, error) {
	buf := new(bytes.Buffer)

	if err := binary.Write(buf, binary.BigEndian, Magic); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, e.SeqID); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, e.Timestamp); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, e.DataType); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, uint32(len(e.Payload))); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, e.CRC32); err != nil {
		return nil, err
	}
	if _, err := buf.Write(e.Payload); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func DecodeEntry(r io.Reader) (*Entry, error) {
	header := make([]byte, HeaderSize)
	n, err := io.ReadFull(r, header)
	if err != nil {
		if err == io.EOF && n == 0 {
			return nil, io.EOF
		}
		return nil, ErrCorruptEntry
	}

	magic := binary.BigEndian.Uint32(header[0:4])
	if magic != Magic {
		return nil, ErrCorruptEntry
	}

	seqID := binary.BigEndian.Uint64(header[4:12])
	timestamp := int64(binary.BigEndian.Uint64(header[12:20]))
	dataType := DataType(header[20])
	payloadLen := binary.BigEndian.Uint32(header[21:25])
	storedCRC := binary.BigEndian.Uint32(header[25:29])

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, ErrCorruptEntry
	}

	entry := &Entry{
		SeqID:     seqID,
		Timestamp: timestamp,
		DataType:  dataType,
		Payload:   payload,
		CRC32:     storedCRC,
	}

	if !VerifyCRC32(entry) {
		return nil, ErrCorruptEntry
	}

	return entry, nil
}
