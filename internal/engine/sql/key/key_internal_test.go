package key

import (
	"errors"
	"math"
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

func TestValidateField(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		f       Field
		wantErr error
	}{
		{"valid uint64", make([]byte, 8), Field{Kind: KindUint64, Offset: 0, Length: 8}, nil},
		{"valid int64", make([]byte, 8), Field{Kind: KindInt64, Offset: 0, Length: 8}, nil},
		{"valid string", []byte("hello"), Field{Kind: KindString, Offset: 0, Length: 5}, nil},
		{"valid bytes zero", []byte{}, Field{Kind: KindBytes, Offset: 0, Length: 0}, nil},
		{"neg Offset", []byte{0}, Field{Kind: KindUint64, Offset: -1, Length: 8}, ErrInvalidKey},
		{"neg Length", []byte{0}, Field{Kind: KindUint64, Offset: 0, Length: -1}, ErrInvalidKey},
		{"Length > data", make([]byte, 4), Field{Kind: KindUint64, Offset: 0, Length: 8}, ErrInvalidKey},
		{"uint64 len=7", make([]byte, 8), Field{Kind: KindUint64, Offset: 0, Length: 7}, ErrInvalidKey},
		{"uint64 len=9", make([]byte, 9), Field{Kind: KindUint64, Offset: 0, Length: 9}, ErrInvalidKey},
		{"int64 len=4", make([]byte, 4), Field{Kind: KindInt64, Offset: 0, Length: 4}, ErrInvalidKey},
		{"string short", []byte("ab"), Field{Kind: KindString, Offset: 0, Length: 100}, ErrInvalidKey},
		{"MaxInt offset", make([]byte, 8), Field{Kind: KindUint64, Offset: math.MaxInt, Length: 1}, ErrInvalidKey},
		{"MaxInt both", make([]byte, 8), Field{Kind: KindUint64, Offset: math.MaxInt, Length: math.MaxInt}, ErrInvalidKey},
		{"0+MaxInt", make([]byte, 8), Field{Kind: KindUint64, Offset: 0, Length: math.MaxInt}, ErrInvalidKey},
		{"MaxInt-1+2", make([]byte, 8), Field{Kind: KindUint64, Offset: math.MaxInt - 1, Length: 2}, ErrInvalidKey},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateField(tt.data, tt.f); !errors.Is(err, tt.wantErr) {
				t.Errorf("validateField = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
