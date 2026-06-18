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

// leafValueRef describes how a leaf slot's value is stored.
type leafValueRef struct {
	totalLen       uint32
	inline         []byte // up to 512 bytes (!hasOverflow) or 504 bytes (hasOverflow)
	overflowPageID uint64 // valid only if hasOverflow
	hasOverflow    bool
}

// encodeLeafNode serializes keys, values, and nextLeaf into a leaf node body.
// values is a slice of raw byte slices (for Basic compatibility) or can be
// constructed by callers. The function detects overflow based on len(v) > MaxValSize.
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
		if len(v) > MaxValSizeEnterprise {
			return nil, ErrValueTooLarge
		}

		slot := body[offset : offset+LeafSlotSize]
		slot[0] = uint8(len(k.Bytes()))
		copy(slot[1:64], k.Bytes())

		if len(v) > MaxValSize {
			// Overflow: v = [overflowPageID(8) | actual_value_bytes].
			// totalLen is the actual value length (len(v) - 8).
			totalLen := len(v) - 8
			binary.BigEndian.PutUint32(slot[64:68], uint32(totalLen))
			copy(slot[68:76], v[:8])  // overflowPageID
			copy(slot[76:580], v[8:]) // prefix (up to 504 bytes)
		} else {
			binary.BigEndian.PutUint32(slot[64:68], uint32(len(v)))
			copy(slot[68:580], v)
		}

		offset += LeafSlotSize
	}

	return body, nil
}

// encodeLeafNodeWithRefs serializes using leafValueRefs directly.
func encodeLeafNodeWithRefs(keys []key.Key, refs []leafValueRef, nextLeaf uint64) ([]byte, error) {
	if len(keys) != len(refs) {
		return nil, fmt.Errorf("%w: keys/refs length mismatch", ErrTreeCorrupt)
	}
	if len(keys) > MaxLeafKeys {
		return nil, fmt.Errorf("%w: too many leaf keys", ErrTreeCorrupt)
	}
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
		ref := refs[i]
		if len(k.Bytes()) > MaxKeySize {
			return nil, ErrKeyTooLarge
		}

		slot := body[offset : offset+LeafSlotSize]
		slot[0] = uint8(len(k.Bytes()))
		copy(slot[1:64], k.Bytes())
		binary.BigEndian.PutUint32(slot[64:68], ref.totalLen)

		if ref.hasOverflow {
			binary.BigEndian.PutUint64(slot[68:76], ref.overflowPageID)
			copy(slot[76:580], ref.inline)
		} else {
			copy(slot[68:580], ref.inline)
		}

		offset += LeafSlotSize
	}

	return body, nil
}

// decodeLeafNodeRefs decodes a leaf node body and returns keys with value references.
func decodeLeafNodeRefs(data []byte) (keys []key.Key, refs []leafValueRef, nextLeaf uint64, err error) {
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
	refs = make([]leafValueRef, 0, count)

	offset := 11
	for i := 0; i < count; i++ {
		slot := data[offset : offset+LeafSlotSize]

		kLen := int(slot[0])
		if kLen > MaxKeySize {
			return nil, nil, 0, fmt.Errorf("%w: leaf key length exceeds max", ErrTreeCorrupt)
		}
		kBytes := make([]byte, kLen)
		copy(kBytes, slot[1:1+kLen])
		k := importKey(kBytes)

		totalLen := binary.BigEndian.Uint32(slot[64:68])
		if totalLen > MaxValSizeEnterprise {
			return nil, nil, 0, fmt.Errorf("%w: leaf value length exceeds enterprise max", ErrTreeCorrupt)
		}

		ref := leafValueRef{totalLen: totalLen}
		if totalLen > uint32(MaxValSize) {
			ref.hasOverflow = true
			ref.overflowPageID = binary.BigEndian.Uint64(slot[68:76])
			ref.inline = make([]byte, 504)
			copy(ref.inline, slot[76:580])
		} else {
			ref.inline = make([]byte, totalLen)
			copy(ref.inline, slot[68:68+int(totalLen)])
		}

		keys = append(keys, k)
		refs = append(refs, ref)

		offset += LeafSlotSize
	}

	return keys, refs, nextLeaf, nil
}

// decodeLeafNode decodes a leaf node body. Returns copies of keys and values.
// For overflow values (totalLen > MaxValSize), the value is returned as
// [overflowPageID(8) | inline prefix(up to 504)] — 512 bytes total.
// Use decodeLeafNodeRefs for structured ref-based decoding.
func decodeLeafNode(data []byte) (keys []key.Key, values [][]byte, nextLeaf uint64, err error) {
	keys, refs, nextLeaf, err := decodeLeafNodeRefs(data)
	if err != nil {
		return nil, nil, 0, err
	}
	values = make([][]byte, len(refs))
	for i, ref := range refs {
		if ref.hasOverflow {
			v := make([]byte, 512)
			binary.BigEndian.PutUint64(v[0:8], ref.overflowPageID)
			copy(v[8:512], ref.inline)
			values[i] = v
		} else {
			values[i] = ref.inline
		}
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

// --- Overflow Page ---

const overflowChunkSize = 4075 // pager.DataPageBodySize - 1 (NodeType) - 8 (NextPtr)

// encodeOverflowPage serializes an overflow page.
func encodeOverflowPage(nextPtr uint64, chunk []byte) ([]byte, error) {
	if len(chunk) > overflowChunkSize {
		return nil, fmt.Errorf("%w: overflow chunk too large: %d > %d", ErrTreeCorrupt, len(chunk), overflowChunkSize)
	}
	body := make([]byte, pager.DataPageBodySize)
	body[0] = NodeTypeOverflow
	binary.BigEndian.PutUint64(body[1:9], nextPtr)
	copy(body[9:9+len(chunk)], chunk)
	// Remainder zero-filled.
	return body, nil
}

// decodeOverflowPage decodes an overflow page. Returns the next pointer and
// the chunk data (a copy of the actual bytes, not the full 4075-byte buffer).
func decodeOverflowPage(data []byte) (nextPtr uint64, chunk []byte, err error) {
	if len(data) != pager.DataPageBodySize {
		return 0, nil, fmt.Errorf("%w: wrong overflow body size", ErrTreeCorrupt)
	}
	if data[0] != NodeTypeOverflow {
		return 0, nil, fmt.Errorf("%w: not an overflow page", ErrTreeCorrupt)
	}
	nextPtr = binary.BigEndian.Uint64(data[1:9])
	// Copy all remaining bytes (the chunk uses up to overflowChunkSize bytes).
	chunk = make([]byte, overflowChunkSize)
	copy(chunk, data[9:])
	return nextPtr, chunk, nil
}
