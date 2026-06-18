package pager

import (
	"context"
	"fmt"
	"testing"
)

// preallocatePages creates a pager with n pre-allocated data pages.
// The pager is opened, n pages are allocated, and the pager is returned
// ready for benchmark use. The caller must close it.
// Pages are allocated starting from FirstDataPageID (2).
func preallocatePages(b *testing.B, n int) *filePager {
	b.Helper()
	dir := b.TempDir()
	ctx := context.Background()
	p := New(dir + "/bench.pager").(*filePager)
	if err := p.Open(ctx); err != nil {
		b.Fatal(err)
	}
	for i := 0; i < n; i++ {
		if _, err := p.AllocatePage(ctx); err != nil {
			b.Fatal(err)
		}
	}
	return p
}

// allocatedPageIDs returns the expected page IDs after allocating n pages.
func allocatedPageIDs(n int) []uint64 {
	ids := make([]uint64, n)
	for i := 0; i < n; i++ {
		ids[i] = uint64(FirstDataPageID + i)
	}
	return ids
}

func BenchmarkAllocatePage(b *testing.B) {
	b.Run("extend", func(b *testing.B) {
		dir := b.TempDir()
		ctx := context.Background()
		p := New(dir + "/extend.pager").(*filePager)
		if err := p.Open(ctx); err != nil {
			b.Fatal(err)
		}
		defer p.Close(ctx)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = p.AllocatePage(ctx)
		}
	})

	b.Run("reuse", func(b *testing.B) {
		p := preallocatePages(b, 1000)
		defer p.Close(context.Background())
		ctx := context.Background()

		// Free all allocated pages so the free-list is populated.
		for _, id := range allocatedPageIDs(1000) {
			_ = p.FreePage(ctx, id)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			id, err := p.AllocatePage(ctx)
			if err != nil {
				b.Fatal(err)
			}
			_ = p.FreePage(ctx, id)
		}
	})
}

func BenchmarkWritePage(b *testing.B) {
	p := preallocatePages(b, 1000)
	defer p.Close(context.Background())
	ctx := context.Background()
	body := make([]byte, DataPageBodySize)
	ids := allocatedPageIDs(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pageID := ids[i%1000]
		if err := p.WritePage(ctx, pageID, body); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReadPage(b *testing.B) {
	p := preallocatePages(b, 1000)
	defer p.Close(context.Background())
	ctx := context.Background()
	ids := allocatedPageIDs(1000)

	body := make([]byte, DataPageBodySize)
	for _, id := range ids {
		if err := p.WritePage(ctx, id, body); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pageID := ids[i%1000]
		_, _ = p.ReadPage(ctx, pageID)
	}
}

// -- Transaction benchmarks (Task 9) --

func BenchmarkCommitTx(b *testing.B) {
	body := make([]byte, DataPageBodySize)

	for _, pagesPerTx := range []int{1, 10, 100} {
		b.Run(fmt.Sprintf("pages=%d", pagesPerTx), func(b *testing.B) {
			dir := b.TempDir()
			ctx := context.Background()
			p := New(dir + "/committx.pager").(*filePager)
			if err := p.Open(ctx); err != nil {
				b.Fatal(err)
			}
			defer p.Close(ctx)

			// Pre-allocate enough pages for all transactions.
			totalPages := b.N * pagesPerTx
			if totalPages == 0 {
				totalPages = pagesPerTx
			}
			for i := 0; i < totalPages; i++ {
				if _, err := p.AllocatePage(ctx); err != nil {
					b.Fatal(err)
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := p.BeginTx(ctx); err != nil {
					b.Fatal(err)
				}
				for j := 0; j < pagesPerTx; j++ {
					pageID := uint64(FirstDataPageID + i*pagesPerTx + j)
					if err := p.WritePage(ctx, pageID, body); err != nil {
						b.Fatal(err)
					}
				}
				if err := p.CommitTx(ctx); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
