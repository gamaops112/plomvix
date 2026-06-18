package kv

import (
	"encoding/binary"
	"fmt"

	"github.com/plomvix/plomvix/internal/engine/sql/key"
	"github.com/plomvix/plomvix/internal/storage/pager"
)

// --- Meta Page ---

// encodeMetaPage serializes the rootPageID into a PageSize-12 byte meta page body.
func encodeMetaPage(rootPageID uint64) []byte {
	body := make([]byte, pager.DataPageBodySize)
	body[0] = NodeTypeMeta
	binary.BigEndian.PutUint64(body[1:9], rootPageID)
	// Remainder zero-filled.
	return body
}

// decodeMetaPage validates and decodes a meta page body.
func decodeMetaPage(data []byte) (rootPageID uint64, err error) {
	if len(data) != pager.DataPageBodySize {
		return 0, fmt.Errorf("%w: wrong meta page size", ErrTreeCorrupt)
	}
	if data[0] != NodeTypeMeta {
		return 0, fmt.Errorf("%w: not a meta page", ErrTreeCorrupt)
	}
	rootPageID = binary.BigEndian.Uint64(data[1:9])
	return rootPageID, nil
}

// --- Leaf Node ---

// encodeLeafNode serializes keys, values, and nextLeaf into a leaf node body.
// The body size must equal pager.DataPageBodySize.
func encodeLeafNode(keys []key.Key, values [][]byte, nextLeaf uint64) ([]byte, error) {
	if len(keys) != len(values) {
		return nil, fmt.Errorf("%w: keys/values length mismatch", ErrTreeCorrupt)
	}
	if len(keys) > MaxLeafKeys {
		return nil, fmt.Errorf("%w: too many leaf keys", ErrTreeCorrupt)
	}
	// Validate keys sorted.
	for i := 1; i < len(keys); i++ {
		if keys[i-1].Compare(keys[i]) >= 0 {
			return nil, fmt.Errorf("%w: leaf keys not strictly ascending", ErrTreeCorrupt)
		}
	}

	body := make([]byte, pager.DataPageBodySize)
	body[0] = NodeTypeLeaf
	binary.BigEndian.PutUint16(body[1:3], uint16(len(keys)))
	binary.BigEndian.PutUint64(body[3:11], nextLeaf)

	offset := 11
	for i := 0; i < len(keys); i++ {
		k := keys[i]
		v := values[i]
		if len(k.Bytes()) > MaxKeySize {
			return nil, ErrKeyTooLarge
		}
		if len(v) > MaxValSize {
			return nil, ErrValueTooLarge
		}

		slot := body[offset : offset+LeafSlotSize]
		// KeyLength (1 byte)
		slot[0] = uint8(len(k.Bytes()))
		// Key (63 bytes, zero-padded after actual bytes)
		copy(slot[1:64], k.Bytes())
		// ValueLength (4 bytes)
		binary.BigEndian.PutUint32(slot[64:68], uint32(len(v)))
		// Value (512 bytes, zero-padded)
		copy(slot[68:580], v)

		offset += LeafSlotSize
	}

	return body, nil
}

// decodeLeafNode decodes a leaf node body. Returns copies of keys and values.
func decodeLeafNode(data []byte) (keys []key.Key, values [][]byte, nextLeaf uint64, err error) {
	if len(data) != pager.DataPageBodySize {
		return nil, nil, 0, fmt.Errorf("%w: wrong leaf body size", ErrTreeCorrupt)
	}
	if data[0] != NodeTypeLeaf {
		return nil, nil, 0, fmt.Errorf("%w: not a leaf node", ErrTreeCorrupt)
	}

	count := int(binary.BigEndian.Uint16(data[1:3]))
	if count > MaxLeafKeys {
		return nil, nil, 0, fmt.Errorf("%w: leaf NumKeys > MaxLeafKeys", ErrTreeCorrupt)
	}
	nextLeaf = binary.BigEndian.Uint64(data[3:11])

	keys = make([]key.Key, 0, count)
	values = make([][]byte, 0, count)

	offset := 11
	for i := 0; i < count; i++ {
		slot := data[offset : offset+LeafSlotSize]

		// KeyLength at slot[0], key bytes at slot[1:1+kLen].
		kLen := int(slot[0])
		if kLen > MaxKeySize {
			return nil, nil, 0, fmt.Errorf("%w: leaf key length exceeds max", ErrTreeCorrupt)
		}
		kBytes := make([]byte, kLen)
		copy(kBytes, slot[1:1+kLen])
		k := importKey(kBytes)

		valLen := int(binary.BigEndian.Uint32(slot[64:68]))
		if valLen > MaxValSize {
			return nil, nil, 0, fmt.Errorf("%w: leaf value length exceeds max", ErrTreeCorrupt)
		}
		v := make([]byte, valLen)
		copy(v, slot[68:68+valLen])

		keys = append(keys, k)
		values = append(values, v)

		offset += LeafSlotSize
	}

	return keys, values, nextLeaf, nil
}

// --- Internal Node ---

// encodeInternalNode serializes child pointers and separator keys into an internal node body.
// len(childPtrs) must equal len(keys) + 1.
func encodeInternalNode(childPtrs []uint64, keys []key.Key) ([]byte, error) {
	if len(childPtrs) != len(keys)+1 {
		return nil, fmt.Errorf("%w: internal childPtrs/keys count mismatch", ErrTreeCorrupt)
	}
	if len(keys) > MaxInternalKeys {
		return nil, fmt.Errorf("%w: too many internal keys", ErrTreeCorrupt)
	}
	for i := 1; i < len(keys); i++ {
		if keys[i-1].Compare(keys[i]) >= 0 {
			return nil, fmt.Errorf("%w: internal keys not strictly ascending", ErrTreeCorrupt)
		}
	}

	body := make([]byte, pager.DataPageBodySize)
	body[0] = NodeTypeInternal
	binary.BigEndian.PutUint16(body[1:3], uint16(len(keys)))
	binary.BigEndian.PutUint64(body[3:11], childPtrs[0])

	offset := 11
	for i := 0; i < len(keys); i++ {
		if len(keys[i].Bytes()) > MaxKeySize {
			return nil, ErrKeyTooLarge
		}

		slot := body[offset : offset+InternalSlotSize]
		// KeyLength (1 byte)
		slot[0] = uint8(len(keys[i].Bytes()))
		// Key (63 bytes, zero-padded after actual bytes)
		copy(slot[1:64], keys[i].Bytes())
		binary.BigEndian.PutUint64(slot[64:72], childPtrs[i+1])

		offset += InternalSlotSize
	}

	return body, nil
}

// decodeInternalNode decodes an internal node body.
func decodeInternalNode(data []byte) (childPtrs []uint64, keys []key.Key, err error) {
	if len(data) != pager.DataPageBodySize {
		return nil, nil, fmt.Errorf("%w: wrong internal body size", ErrTreeCorrupt)
	}
	if data[0] != NodeTypeInternal {
		return nil, nil, fmt.Errorf("%w: not an internal node", ErrTreeCorrupt)
	}

	count := int(binary.BigEndian.Uint16(data[1:3]))
	if count > MaxInternalKeys {
		return nil, nil, fmt.Errorf("%w: internal NumKeys > MaxInternalKeys", ErrTreeCorrupt)
	}
	childPtr0 := binary.BigEndian.Uint64(data[3:11])

	childPtrs = make([]uint64, 0, count+1)
	childPtrs = append(childPtrs, childPtr0)
	keys = make([]key.Key, 0, count)

	offset := 11
	for i := 0; i < count; i++ {
		slot := data[offset : offset+InternalSlotSize]
		kLen := int(slot[0])
		if kLen > MaxKeySize {
			return nil, nil, fmt.Errorf("%w: internal key length exceeds max", ErrTreeCorrupt)
		}
		kBytes := make([]byte, kLen)
		copy(kBytes, slot[1:1+kLen])
		k := importKey(kBytes)
		childPtr := binary.BigEndian.Uint64(slot[64:72])

		keys = append(keys, k)
		childPtrs = append(childPtrs, childPtr)

		offset += InternalSlotSize
	}

	return childPtrs, keys, nil
}

// importKey creates a key.Key from raw bytes, preserving byte-wise ordering.
func importKey(b []byte) key.Key {
	return key.EncodeBytes(b)
}
