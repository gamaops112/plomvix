package wal

import (
	"bytes"
	"io"
	"testing"
)

func makeTestEntry() *Entry {
	e := &Entry{
		SeqID:     1,
		Timestamp: 1234567890,
		DataType:  DataTypeLog,
		Payload:   []byte(`{"msg":"test"}`),
	}
	e.CRC32 = ComputeCRC32(e)
	return e
}

func TestEncodeDecodeRoundtrip(t *testing.T) {
	e := &Entry{
		SeqID:     42,
		Timestamp: 9876543210,
		DataType:  DataTypeMetric,
		Payload:   []byte(`{"metric":"cpu","value":87.5}`),
	}
	e.CRC32 = ComputeCRC32(e)

	encoded, err := EncodeEntry(e)
	if err != nil {
		t.Fatalf("EncodeEntry failed: %v", err)
	}

	expectedLen := HeaderSize + len(e.Payload)
	if len(encoded) != expectedLen {
		t.Errorf("encoded length = %d, want %d", len(encoded), expectedLen)
	}

	decoded, err := DecodeEntry(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("DecodeEntry failed: %v", err)
	}

	if decoded.SeqID != e.SeqID {
		t.Errorf("SeqID = %d, want %d", decoded.SeqID, e.SeqID)
	}
	if decoded.Timestamp != e.Timestamp {
		t.Errorf("Timestamp = %d, want %d", decoded.Timestamp, e.Timestamp)
	}
	if decoded.DataType != e.DataType {
		t.Errorf("DataType = %d, want %d", decoded.DataType, e.DataType)
	}
	if decoded.CRC32 != e.CRC32 {
		t.Errorf("CRC32 = %d, want %d", decoded.CRC32, e.CRC32)
	}
	if !bytes.Equal(decoded.Payload, e.Payload) {
		t.Errorf("Payload = %q, want %q", decoded.Payload, e.Payload)
	}
}

func TestDecodeEOF(t *testing.T) {
	_, err := DecodeEntry(bytes.NewReader([]byte{}))
	if err != io.EOF {
		t.Errorf("expected io.EOF on empty reader, got %v", err)
	}
}

func TestDecodeCorruptMagic(t *testing.T) {
	e := makeTestEntry()
	encoded, _ := EncodeEntry(e)
	encoded[0] = 0xFF
	_, err := DecodeEntry(bytes.NewReader(encoded))
	if err != ErrCorruptEntry {
		t.Errorf("expected ErrCorruptEntry for corrupt magic, got %v", err)
	}
}

func TestDecodeCorruptCRC(t *testing.T) {
	e := makeTestEntry()
	encoded, _ := EncodeEntry(e)
	encoded[len(encoded)-1] ^= 0xFF
	_, err := DecodeEntry(bytes.NewReader(encoded))
	if err != ErrCorruptEntry {
		t.Errorf("expected ErrCorruptEntry for corrupt payload, got %v", err)
	}
}

func TestDecodeTruncated(t *testing.T) {
	e := makeTestEntry()
	encoded, _ := EncodeEntry(e)
	truncated := encoded[:HeaderSize/2]
	_, err := DecodeEntry(bytes.NewReader(truncated))
	if err != ErrCorruptEntry {
		t.Errorf("expected ErrCorruptEntry for truncated entry, got %v", err)
	}
}

func TestDecodeMultipleEntries(t *testing.T) {
	buf := new(bytes.Buffer)

	for i := uint64(1); i <= 3; i++ {
		e := &Entry{
			SeqID:     i,
			Timestamp: int64(i * 1000),
			DataType:  DataTypeLog,
			Payload:   []byte(`{"seq":"test"}`),
		}
		e.CRC32 = ComputeCRC32(e)
		encoded, err := EncodeEntry(e)
		if err != nil {
			t.Fatalf("EncodeEntry failed: %v", err)
		}
		buf.Write(encoded)
	}

	r := bytes.NewReader(buf.Bytes())
	for i := uint64(1); i <= 3; i++ {
		entry, err := DecodeEntry(r)
		if err != nil {
			t.Fatalf("DecodeEntry[%d] failed: %v", i, err)
		}
		if entry.SeqID != i {
			t.Errorf("entry %d SeqID = %d, want %d", i, entry.SeqID, i)
		}
	}

	_, err := DecodeEntry(r)
	if err != io.EOF {
		t.Errorf("expected io.EOF after 3 entries, got %v", err)
	}
}
