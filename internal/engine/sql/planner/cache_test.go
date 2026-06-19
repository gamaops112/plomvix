package planner

import (
	"context"
	"io"
	"sync"
	"testing"

	"github.com/plomvix/plomvix/internal/engine"
)

func TestPlanCache_MissOnEmpty(t *testing.T) {
	c := NewPlanCache(10)
	if tmpl := c.Lookup(CacheKey{"abc", 1}); tmpl != nil {
		t.Error("empty cache should miss")
	}
}

func TestPlanCache_HitAfterStore(t *testing.T) {
	c := NewPlanCache(10)
	tmpl := &PlanTemplate{TableID: 42}
	c.Store(CacheKey{"abc", 1}, tmpl)
	got := c.Lookup(CacheKey{"abc", 1})
	if got != tmpl {
		t.Error("cache should hit after Store")
	}
}

func TestPlanCache_FIFOEviction(t *testing.T) {
	c := NewPlanCache(3)
	for i := 0; i < 4; i++ {
		c.Store(CacheKey{string(rune('a' + i)), 1}, &PlanTemplate{TableID: uint64(i)})
	}
	if tmpl := c.Lookup(CacheKey{"a", 1}); tmpl != nil {
		t.Error("first entry should have been evicted")
	}
	if tmpl := c.Lookup(CacheKey{"d", 1}); tmpl == nil {
		t.Error("last entry should be present")
	}
}

func TestPlanCache_Disabled(t *testing.T) {
	c := NewPlanCache(0)
	tmpl := &PlanTemplate{TableID: 1}
	c.Store(CacheKey{"x", 1}, tmpl)
	if got := c.Lookup(CacheKey{"x", 1}); got != nil {
		t.Error("disabled cache should always miss")
	}
	h, m, sz := c.Stats()
	if sz != 0 {
		t.Error("disabled cache should report size 0")
	}
	_ = h
	_ = m
}

func TestPlanCache_SchemaVersionInvalidation(t *testing.T) {
	c := NewPlanCache(10)
	c.Store(CacheKey{"abc", 1}, &PlanTemplate{TableID: 1})
	if c.Lookup(CacheKey{"abc", 2}) != nil {
		t.Error("different schema version should miss")
	}
}

func TestPlanCache_Concurrency(t *testing.T) {
	c := NewPlanCache(10000)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			c.Store(CacheKey{string(rune('a' + id)), 1}, &PlanTemplate{TableID: uint64(id)})
			c.Lookup(CacheKey{string(rune('a' + id)), 1})
		}(i)
	}
	wg.Wait()
}

func TestPlanTemplate_Build(t *testing.T) {
	schema := engine.Schema{Columns: []engine.Column{{Name: "id", Type: engine.TypeInt64}}}
	tmpl := &PlanTemplate{
		TableID:      1,
		InputSchema:  schema,
		OutputSchema: schema,
	}
	heap := &fakeTableHeap{rows: [][]byte{}}
	decoder := &fakeRowDecoder{}
	op := tmpl.Build(heap, decoder, engine.TxContext{})
	if op == nil {
		t.Error("Build returned nil")
	}
	op.Close()
}

type fakeTableHeap struct{ rows [][]byte }

func (f *fakeTableHeap) Scan(ctx context.Context, tx engine.TxContext) (HeapScanIterator, error) {
	return &fakeIter{rows: f.rows}, nil
}

type fakeIter struct {
	rows [][]byte
	idx  int
}

func (f *fakeIter) Next(ctx context.Context) ([]byte, uint64, error) {
	if f.idx >= len(f.rows) {
		return nil, 0, io.EOF
	}
	r := f.rows[f.idx]
	f.idx++
	return r, uint64(f.idx), nil
}
func (f *fakeIter) Close() error { return nil }

type fakeRowDecoder struct{}

func (f *fakeRowDecoder) Decode(raw []byte, s engine.Schema) (engine.Row, error) {
	return engine.Row{}, nil
}
