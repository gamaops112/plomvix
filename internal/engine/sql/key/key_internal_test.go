package key

import (
	"errors"
	"testing"
)

func TestDecodeSortComposite_RejectsWrongLength(t *testing.T) {
	k := Key{
		data:   make([]byte, 4),
		fields: []Field{{Kind: KindUint64, Offset: 0, Length: 4}},
	}
	_, err := DecodeSortComposite(k)
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("expected ErrInvalidKey, got %v", err)
	}
}

func TestDecodeSortComposite_RejectsOutOfBoundsOffset(t *testing.T) {
	k := Key{
		data:   make([]byte, 8),
		fields: []Field{{Kind: KindUint64, Offset: 4, Length: 8}},
	}
	_, err := DecodeSortComposite(k)
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("expected ErrInvalidKey, got %v", err)
	}
}

func TestDecodeStorageComposite_RejectsWrongIntegerLength(t *testing.T) {
	k := Key{
		data:   make([]byte, 4),
		fields: []Field{{Kind: KindUint64, Offset: 0, Length: 4}},
	}
	_, err := DecodeStorageComposite(k)
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("expected ErrInvalidKey, got %v", err)
	}
}

func TestDecodeStorageComposite_RejectsOutOfBoundsStringField(t *testing.T) {
	k := Key{
		data:   []byte("hi"),
		fields: []Field{{Kind: KindString, Offset: 0, Length: 10}},
	}
	_, err := DecodeStorageComposite(k)
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("expected ErrInvalidKey, got %v", err)
	}
}
