package store_test

import (
	"fmt"
	"testing"

	"github.com/plomvix/plomvix/internal/engine/sql/key"
	"github.com/plomvix/plomvix/internal/engine/sql/store"
)

// newPopulatedStore creates a Store populated with n entries with keys 0..n-1.
func newPopulatedStore(n int) *store.Store {
	s := store.New()
	for i := 0; i < n; i++ {
		k := key.EncodeUint64(uint64(i))
		s.Put(k, []byte(fmt.Sprintf("val-%d", i)))
	}
	return s
}

// newPopulatedStoreFrom creates a Store populated with n entries with keys start..start+n-1.
func newPopulatedStoreFrom(start, n int) *store.Store {
	s := store.New()
	for i := start; i < start+n; i++ {
		k := key.EncodeUint64(uint64(i))
		s.Put(k, []byte(fmt.Sprintf("val-%d", i)))
	}
	return s
}

// -- Put benchmarks -----------------------------------------------------------

func BenchmarkPut(b *testing.B) {
	sizes := []int{1000, 10_000, 100_000}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				s := newPopulatedStore(size)
				k := key.EncodeUint64(uint64(size + 1))
				b.StartTimer()
				_ = s.Put(k, []byte("v"))
				// Delete the inserted key so the store size stays roughly constant.
				_ = s.Delete(k)
			}
		})
	}
}

// -- Worst-case Put benchmarks (front-insert) ---------------------------------

func BenchmarkPut_WorstCaseFrontInsert(b *testing.B) {
	sizes := []int{1000, 10_000, 100_000}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				// Start with keys [1..size] so inserting key 0 goes to the front.
				s := newPopulatedStoreFrom(1, size)
				k := key.EncodeUint64(0)
				b.StartTimer()
				_ = s.Put(k, []byte("v"))
				// Clean up inserted key so the store size stays roughly constant.
				_ = s.Delete(k)
			}
		})
	}
}

// -- Get benchmarks -----------------------------------------------------------

func BenchmarkGet(b *testing.B) {
	sizes := []int{1000, 10_000, 100_000}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			s := newPopulatedStore(size)
			middle := key.EncodeUint64(uint64(size / 2))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, _ = s.Get(middle)
			}
		})
	}
}

// -- Delete benchmarks --------------------------------------------------------

func BenchmarkDelete(b *testing.B) {
	sizes := []int{1000, 10_000, 100_000}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				s := newPopulatedStore(size)
				middle := key.EncodeUint64(uint64(size / 2))
				b.StartTimer()
				_ = s.Delete(middle)
			}
		})
	}
}

// -- Scan benchmarks ----------------------------------------------------------

func BenchmarkScan(b *testing.B) {
	sizes := []int{1000, 10_000, 100_000}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			s := newPopulatedStore(size)
			start := key.EncodeUint64(uint64(size / 4))
			end := key.EncodeUint64(uint64(size/4 + 100))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, _ = s.Scan(start, end)
			}
		})
	}
}
