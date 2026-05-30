package wal

import (
	"encoding/binary"
	"hash/crc32"
)

func ComputeCRC32(e *Entry) uint32 {
	h := crc32.NewIEEE()

	var buf [4]byte

	binary.BigEndian.PutUint32(buf[:], Magic)
	h.Write(buf[:4])

	var buf8 [8]byte
	binary.BigEndian.PutUint64(buf8[:], e.SeqID)
	h.Write(buf8[:])

	binary.BigEndian.PutUint64(buf8[:], uint64(e.Timestamp))
	h.Write(buf8[:])

	h.Write([]byte{byte(e.DataType)})

	binary.BigEndian.PutUint32(buf[:], uint32(len(e.Payload)))
	h.Write(buf[:4])

	h.Write(e.Payload)

	return h.Sum32()
}

func VerifyCRC32(e *Entry) bool {
	return ComputeCRC32(e) == e.CRC32
}
