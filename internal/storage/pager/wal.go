package pager

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
)

// walEOTPageID is the sentinel PageID used in the End-of-Transaction marker.
const walEOTPageID uint64 = 0xFFFFFFFFFFFFFFFF

// WAL record binary layout:
//
//	[8-byte TxnID][8-byte PageID][4-byte BodyLength][N-byte Body][4-byte CRC32]
//
// CRC32 covers TxnID + PageID + BodyLength + Body (all bytes preceding CRC32).
//
// EOT marker layout:
//
//	[8-byte TxnID][8-byte walEOTPageID][4-byte 0][4-byte CRC32]
//
// CRC32 covers the 20 bytes preceding it.

// encodeWALRecord returns the full framed WAL record including trailing CRC32.
func encodeWALRecord(txnID, pageID uint64, body []byte) []byte {
	bodyLen := uint32(len(body))
	// 8 + 8 + 4 + bodyLen + 4
	rec := make([]byte, 20+bodyLen+4)

	binary.BigEndian.PutUint64(rec[0:8], txnID)
	binary.BigEndian.PutUint64(rec[8:16], pageID)
	binary.BigEndian.PutUint32(rec[16:20], bodyLen)
	copy(rec[20:20+bodyLen], body)

	// CRC32 covers bytes [0, 20+bodyLen)
	cksum := crc32.ChecksumIEEE(rec[:20+bodyLen])
	binary.BigEndian.PutUint32(rec[20+bodyLen:], cksum)

	return rec
}

// encodeEOTMarker returns the End-of-Transaction marker with CRC32.
func encodeEOTMarker(txnID uint64) []byte {
	rec := make([]byte, 20+4) // 8+8+4 + 4

	binary.BigEndian.PutUint64(rec[0:8], txnID)
	binary.BigEndian.PutUint64(rec[8:16], walEOTPageID)
	// BodyLength = 0, already zero
	// rec[16:20] is already 0

	// CRC32 covers bytes [0, 20)
	cksum := crc32.ChecksumIEEE(rec[:20])
	binary.BigEndian.PutUint32(rec[20:], cksum)

	return rec
}

// isEOTMarker returns true if the record represents an End-of-Transaction marker.
func isEOTMarker(pageID uint64, bodyLength uint32) bool {
	return pageID == walEOTPageID && bodyLength == 0
}

// decodeNextWALRecord parses the next WAL record from data.
// On success, returns the decoded fields and the number of bytes consumed.
// On trailing incomplete record, returns io.EOF.
// On corrupted record (CRC mismatch or invalid shape), returns ErrWALCorrupt.
func decodeNextWALRecord(data []byte) (
	txnID uint64,
	pageID uint64,
	body []byte,
	consumed int,
	err error,
) {
	const minRecordLen = 20 // 8+8+4 = 20 (header minimum, no body, no CRC)
	if len(data) < minRecordLen {
		return 0, 0, nil, 0, io.EOF
	}

	txnID = binary.BigEndian.Uint64(data[0:8])
	pageID = binary.BigEndian.Uint64(data[8:16])
	bodyLength := binary.BigEndian.Uint32(data[16:20])

	// Validate record shape.
	if pageID == walEOTPageID {
		if bodyLength != 0 {
			return 0, 0, nil, 0, fmt.Errorf("%w: EOT marker must have BodyLength=0, got %d", ErrWALCorrupt, bodyLength)
		}
	} else {
		if bodyLength != uint32(DataPageBodySize) {
			return 0, 0, nil, 0, fmt.Errorf("%w: data record BodyLength must be %d, got %d", ErrWALCorrupt, DataPageBodySize, bodyLength)
		}
	}

	// 20 bytes header + bodyLength bytes body + 4 bytes CRC
	requiredLen := 20 + int(bodyLength) + 4
	if len(data) < requiredLen {
		return 0, 0, nil, 0, io.EOF
	}

	// Verify CRC32 over [0, 20+bodyLength)
	storedCksum := binary.BigEndian.Uint32(data[20+bodyLength : requiredLen])
	computedCksum := crc32.ChecksumIEEE(data[:20+bodyLength])
	if storedCksum != computedCksum {
		return 0, 0, nil, 0, fmt.Errorf("%w: CRC mismatch", ErrWALCorrupt)
	}

	if bodyLength > 0 {
		body = make([]byte, bodyLength)
		copy(body, data[20:20+bodyLength])
	}

	return txnID, pageID, body, requiredLen, nil
}
