package wal

import "testing"

func TestSegmentFileName(t *testing.T) {
	tests := []struct {
		index    uint64
		expected string
	}{
		{1, "seg-000001.wal"},
		{42, "seg-000042.wal"},
		{999999, "seg-999999.wal"},
	}
	for _, tt := range tests {
		got := SegmentFileName(tt.index)
		if got != tt.expected {
			t.Errorf("SegmentFileName(%d) = %q, want %q", tt.index, got, tt.expected)
		}
	}
}

func TestParseSegmentIndex(t *testing.T) {
	valid := []struct {
		filename string
		expected uint64
	}{
		{"seg-000001.wal", 1},
		{"seg-000042.wal", 42},
		{"seg-999999.wal", 999999},
	}
	for _, tt := range valid {
		got, err := ParseSegmentIndex(tt.filename)
		if err != nil {
			t.Errorf("ParseSegmentIndex(%q) unexpected error: %v", tt.filename, err)
		}
		if got != tt.expected {
			t.Errorf("ParseSegmentIndex(%q) = %d, want %d", tt.filename, got, tt.expected)
		}
	}

	invalid := []string{"notawal.txt", "", "seg-.wal", "seg-abc.wal"}
	for _, name := range invalid {
		_, err := ParseSegmentIndex(name)
		if err == nil {
			t.Errorf("ParseSegmentIndex(%q) expected error, got nil", name)
		}
	}
}

func TestComputeCRC32Deterministic(t *testing.T) {
	e := &Entry{SeqID: 1, Timestamp: 12345, DataType: DataTypeLog, Payload: []byte(`{"a":1}`)}
	c1 := ComputeCRC32(e)
	c2 := ComputeCRC32(e)
	if c1 != c2 {
		t.Error("ComputeCRC32 not deterministic for identical entry")
	}
}

func TestComputeCRC32ChangesWithFields(t *testing.T) {
	base := &Entry{SeqID: 1, Timestamp: 12345, DataType: DataTypeLog, Payload: []byte(`{"a":1}`)}
	baseHash := ComputeCRC32(base)

	changed := &Entry{SeqID: 2, Timestamp: 12345, DataType: DataTypeLog, Payload: []byte(`{"a":1}`)}
	if ComputeCRC32(changed) == baseHash {
		t.Error("CRC32 did not change when SeqID changed")
	}

	changedPayload := &Entry{SeqID: 1, Timestamp: 12345, DataType: DataTypeLog, Payload: []byte(`{"a":2}`)}
	if ComputeCRC32(changedPayload) == baseHash {
		t.Error("CRC32 did not change when Payload changed")
	}
}

func TestVerifyCRC32(t *testing.T) {
	e := &Entry{SeqID: 1, Timestamp: 12345, DataType: DataTypeLog, Payload: []byte(`{"a":1}`)}
	e.CRC32 = ComputeCRC32(e)

	if !VerifyCRC32(e) {
		t.Error("VerifyCRC32 returned false for valid entry")
	}

	e.Payload = []byte(`{"a":9}`)
	if VerifyCRC32(e) {
		t.Error("VerifyCRC32 returned true after payload was modified")
	}
}
