package pager

import (
	"encoding/binary"
	"hash/crc32"
)

// encodeDataPage encodes body into a PageSize-byte data page image.
// len(body) must equal DataPageBodySize.
func encodeDataPage(body []byte) ([]byte, error) {
	if len(body) != DataPageBodySize {
		return nil, ErrBodySizeMismatch
	}
	buf := make([]byte, PageSize)

	// Bytes [0,8): Reserved, zero-filled (already zero from make)
	// Bytes [8,12): Page checksum of [12, PageSize)
	cksum := crc32.ChecksumIEEE(body)
	binary.BigEndian.PutUint32(buf[8:12], cksum)
	// Bytes [12, PageSize): Page body
	copy(buf[12:], body)

	return buf, nil
}

// decodeDataPage validates and decodes a PageSize-byte data page image.
// Returns a COPY of the body bytes.
func decodeDataPage(data []byte) ([]byte, error) {
	if len(data) != PageSize {
		return nil, ErrPageCorrupt
	}

	// Verify checksum over [12, PageSize)
	storedCksum := binary.BigEndian.Uint32(data[8:12])
	computedCksum := crc32.ChecksumIEEE(data[12:])
	if storedCksum != computedCksum {
		return nil, ErrPageCorrupt
	}

	// Return a COPY of the body
	body := make([]byte, DataPageBodySize)
	copy(body, data[12:])
	return body, nil
}

// encodeFreeListPointer returns a DataPageBodySize-byte zero-filled body with
// nextPageID stored at body offset [0,8) in big-endian format. This is the
// body content (pre-encodeDataPage) for a free page.
func encodeFreeListPointer(nextPageID uint64) []byte {
	buf := make([]byte, DataPageBodySize)
	binary.BigEndian.PutUint64(buf[0:8], nextPageID)
	return buf
}

// decodeFreeListPointer extracts the next free-page ID from a DataPageBodySize-
// byte body. Returns ErrPageCorrupt if body has the wrong length.
func decodeFreeListPointer(body []byte) (nextPageID uint64, err error) {
	if len(body) != DataPageBodySize {
		return 0, ErrPageCorrupt
	}
	return binary.BigEndian.Uint64(body[0:8]), nil
}
