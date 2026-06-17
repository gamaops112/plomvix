package key_test

import (
	"testing"

	"github.com/plomvix/plomvix/internal/engine/sql/key"
)

func FuzzParseKey(f *testing.F) {
	kindSets := [][]key.Kind{
		{key.KindUint64},
		{key.KindInt64},
		{key.KindString},
		{key.KindBytes},
		{key.KindUint64, key.KindString},
		{key.KindString, key.KindBytes},
	}

	f.Add([]byte{})
	f.Add(make([]byte, 8))
	f.Add([]byte{0, 0, 0, 0, 0, 0, 0, 0})
	f.Add([]byte("hello\x00"))
	f.Add([]byte{1, 2, 3})
	f.Add([]byte("no_null_here"))

	f.Fuzz(func(t *testing.T, data []byte) {
		for _, kinds := range kindSets {
			k, err := key.ParseKey(data, kinds)
			if err != nil {
				continue
			}
			if len(kinds) == 1 {
				switch kinds[0] {
				case key.KindUint64:
					_, _ = key.DecodeUint64(k)
				case key.KindInt64:
					_, _ = key.DecodeInt64(k)
				case key.KindString:
					_, _ = key.DecodeString(k)
				case key.KindBytes:
					_, _ = key.DecodeBytes(k)
				}
			} else {
				_, _ = key.DecodeSortComposite(k)
			}
		}
	})
}

func FuzzParseStorageCompositeKey(f *testing.F) {
	kindSets := [][]key.Kind{
		{key.KindUint64},
		{key.KindString},
		{key.KindBytes},
		{key.KindUint64, key.KindString, key.KindBytes},
	}

	f.Add([]byte{})
	f.Add([]byte{0, 0, 0, 8, 0, 0, 0, 0, 0, 0, 0, 42})
	f.Add([]byte{0, 0, 0, 3, 'h', 'i', '!'})
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF})

	f.Fuzz(func(t *testing.T, data []byte) {
		for _, kinds := range kindSets {
			k, err := key.ParseStorageCompositeKey(data, kinds)
			if err != nil {
				continue
			}
			_, _ = key.DecodeStorageComposite(k)
		}
	})
}
