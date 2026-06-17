package key_test

import (
	"testing"

	"github.com/plomvix/plomvix/internal/engine/sql/key"
)

func BenchmarkEncodeUint64(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = key.EncodeUint64(uint64(i))
	}
}

func BenchmarkDecodeUint64(b *testing.B) {
	k := key.EncodeUint64(42)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = key.DecodeUint64(k)
	}
}

func BenchmarkEncodeSortComposite(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = key.EncodeSortComposite(uint64(1), uint64(2), int64(-3))
	}
}

func BenchmarkDecodeSortComposite(b *testing.B) {
	k, _ := key.EncodeSortComposite(uint64(1), uint64(2), int64(-3))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = key.DecodeSortComposite(k)
	}
}

func BenchmarkEncodeStorageComposite(b *testing.B) {
	s := "hello-world-12345"
	bb := make([]byte, 32)
	for i := 0; i < b.N; i++ {
		_, _ = key.EncodeStorageComposite(uint64(1), s, bb)
	}
}

func BenchmarkDecodeStorageComposite(b *testing.B) {
	s := "hello-world-12345"
	bb := make([]byte, 32)
	k, _ := key.EncodeStorageComposite(uint64(1), s, bb)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = key.DecodeStorageComposite(k)
	}
}
