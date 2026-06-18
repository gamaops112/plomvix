package pager

import (
	"encoding/binary"
	"hash/crc32"
)

// pagerHeader represents the decoded contents of the header page (page 0 / page 1).
type pagerHeader struct {
	magic         uint32
	version       uint32
	pageSize      uint32
	pageCount     uint64
	freeListHead  uint64
	freePageCount uint64
}

// encodeHeader serializes h into a PageSize-byte header page image (Enterprise layout).
//
//	Offset  Size  Field
//	0       4     Magic number
//	4       4     Format version (must equal 2)
//	8       4     Page size (must equal PageSize)
//	12      8     Page count (uint64)
//	20      8     Free-list head (uint64, sentinel 0xFFFFFFFFFFFFFFFF = empty)
//	28      8     Free-page count (uint64)
//	36      4     CRC32 (IEEE) of bytes [0, 36)
//	40      ...   Reserved, zero-filled
func encodeHeader(h pagerHeader) []byte {
	buf := make([]byte, PageSize)

	// Bytes [0,4): Magic number
	binary.BigEndian.PutUint32(buf[0:4], h.magic)
	// Bytes [4,8): Format version
	binary.BigEndian.PutUint32(buf[4:8], h.version)
	// Bytes [8,12): Page size
	binary.BigEndian.PutUint32(buf[8:12], h.pageSize)
	// Bytes [12,20): Page count
	binary.BigEndian.PutUint64(buf[12:20], h.pageCount)
	// Bytes [20,28): Free-list head
	binary.BigEndian.PutUint64(buf[20:28], h.freeListHead)
	// Bytes [28,36): Free-page count
	binary.BigEndian.PutUint64(buf[28:36], h.freePageCount)

	// Bytes [36,40): Header checksum of bytes [0, 36)
	cksum := crc32.ChecksumIEEE(buf[0:36])
	binary.BigEndian.PutUint32(buf[36:40], cksum)

	// Bytes [40, PageSize): Reserved, zero-filled (already zero from make)

	return buf
}

// decodeHeader validates and decodes a PageSize-byte header page image.
// Validation order: magic -> version -> page size -> checksum.
// Uses the Enterprise layout (checksum at offset 36 covering [0,36)).
func decodeHeader(data []byte) (pagerHeader, error) {
	if len(data) != PageSize {
		return pagerHeader{}, ErrHeaderCorrupt
	}

	magic := binary.BigEndian.Uint32(data[0:4])
	if magic != MagicNumber {
		return pagerHeader{}, ErrNotAPagerFile
	}

	version := binary.BigEndian.Uint32(data[4:8])
	if version != FormatVersion {
		return pagerHeader{}, ErrUnsupportedVersion
	}

	pageSize := binary.BigEndian.Uint32(data[8:12])
	if pageSize != PageSize {
		return pagerHeader{}, ErrPageSizeMismatch
	}

	// Verify checksum over [0,36)
	storedCksum := binary.BigEndian.Uint32(data[36:40])
	computedCksum := crc32.ChecksumIEEE(data[0:36])
	if storedCksum != computedCksum {
		return pagerHeader{}, ErrHeaderCorrupt
	}

	return pagerHeader{
		magic:         magic,
		version:       version,
		pageSize:      pageSize,
		pageCount:     binary.BigEndian.Uint64(data[12:20]),
		freeListHead:  binary.BigEndian.Uint64(data[20:28]),
		freePageCount: binary.BigEndian.Uint64(data[28:36]),
	}, nil
}
