package pager

import (
	"context"
	"testing"
)

// Benchmark helpers

// preallocatePages creates a pager with n pre-allocated data pages.
// The pager is opened, n pages are allocated, and the pager is returned
// ready for benchmark use. The caller must close it.
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

func BenchmarkAllocatePage(b *testing.B) {
	// Each iteration creates its own pager to measure allocation from
	// an empty free-list (file extension path).
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

	// Benchmark allocation from free-list (reuse path).
	b.Run("reuse", func(b *testing.B) {
		p := preallocatePages(b, 1000)
		defer p.Close(context.Background())
		ctx := context.Background()

		// Free all pages so the free-list is populated
		for id := uint64(1); id <= 1000; id++ {
			_ = p.FreePage(ctx, id)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			id, err := p.AllocatePage(ctx)
			if err != nil {
				b.Fatal(err)
			}
			// Re-free so we don't exhaust the free-list
			_ = p.FreePage(ctx, id)
		}
	})
}

func BenchmarkWritePage(b *testing.B) {
	p := preallocatePages(b, 1000)
	defer p.Close(context.Background())
	ctx := context.Background()
	body := make([]byte, DataPageBodySize)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pageID := uint64(i%1000 + 1)
		if err := p.WritePage(ctx, pageID, body); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReadPage(b *testing.B) {
	p := preallocatePages(b, 1000)
	defer p.Close(context.Background())
	ctx := context.Background()

	// Write some data so we're reading real content
	body := make([]byte, DataPageBodySize)
	for id := uint64(1); id <= 1000; id++ {
		if err := p.WritePage(ctx, id, body); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pageID := uint64(i%1000 + 1)
		_, _ = p.ReadPage(ctx, pageID)
	}
}
