package wal

import (
	"fmt"
	"strconv"
	"strings"
)

type DataType uint8

const (
	DataTypeLog    DataType = 1
	DataTypeMetric DataType = 2
	DataTypeJSON   DataType = 3
	DataTypeKV     DataType = 4
)

const Magic uint32 = 0x504C4D58

const HeaderSize = 29

type Entry struct {
	SeqID     uint64
	Timestamp int64
	DataType  DataType
	Payload   []byte
	CRC32     uint32
}

func SegmentFileName(index uint64) string {
	return fmt.Sprintf("seg-%06d.wal", index)
}

func ParseSegmentIndex(filename string) (uint64, error) {
	if !strings.HasPrefix(filename, "seg-") || !strings.HasSuffix(filename, ".wal") {
		return 0, fmt.Errorf("not a WAL segment filename: %q", filename)
	}
	numeric := strings.TrimSuffix(strings.TrimPrefix(filename, "seg-"), ".wal")
	index, err := strconv.ParseUint(numeric, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid segment index in filename %q: %w", filename, err)
	}
	return index, nil
}
