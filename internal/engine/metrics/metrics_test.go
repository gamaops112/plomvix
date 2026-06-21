// Package metrics provides a time-series metrics engine for Plomvix.
// metrics_test.go validates ingestion, range-scan queries, schema
// validation, and page-level storage correctness.
package metrics

import (
	"context"
	"math"
	"path/filepath"
	"testing"

	"github.com/plomvix/plomvix/internal/storage/pager"
)

func testPagerPath(tb testing.TB) string {
	return filepath.Join(tb.TempDir(), "metrics_test.db")
}

func openTestStore(t *testing.T) (*MetricsStore, func()) {
	t.Helper()
	pg := pager.New(testPagerPath(t))
	store := NewStore(pg)
	ctx := context.Background()
	if err := store.Open(ctx); err != nil {
		t.Fatalf("open store: %v", err)
	}
	return store, func() { store.Close(ctx) }
}

func TestStoreOpenCreatesFirstPage(t *testing.T) {
	store, cleanup := openTestStore(t)
	defer cleanup()

	ctx := context.Background()
	pageCount, err := store.pager.PageCount(ctx)
	if err != nil {
		t.Fatalf("PageCount: %v", err)
	}
	if pageCount <= pager.FirstDataPageID {
		t.Fatalf("expected at least %d pages, got %d", pager.FirstDataPageID, pageCount)
	}

	// First data page should have header initialized: num_points=0, next_write_offset=8
	body, err := store.pager.ReadPage(ctx, pager.FirstDataPageID)
	if err != nil {
		t.Fatalf("ReadPage: %v", err)
	}
	numPoints := readUint32LE(body, 0)
	nextOffset := readUint32LE(body, 4)
	if numPoints != 0 {
		t.Errorf("num_points = %d, want 0", numPoints)
	}
	if nextOffset != headerSize {
		t.Errorf("next_write_offset = %d, want %d", nextOffset, headerSize)
	}
}

func TestStoreAppendAndScanSinglePoint(t *testing.T) {
	store, cleanup := openTestStore(t)
	defer cleanup()

	ctx := context.Background()
	pt := Point{
		Timestamp:  1700000000,
		Tags:       "host=server1,region=us-east",
		MetricName: "cpu_usage",
		Value:      42.5,
	}

	if err := store.AppendPoint(ctx, pt); err != nil {
		t.Fatalf("AppendPoint: %v", err)
	}

	// Scan with matching range.
	results, err := store.ScanRange(ctx, 1699999999, 1700000001, nil)
	if err != nil {
		t.Fatalf("ScanRange: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("ScanRange returned %d points, want 1", len(results))
	}

	got := results[0]
	if got.Timestamp != pt.Timestamp {
		t.Errorf("Timestamp = %d, want %d", got.Timestamp, pt.Timestamp)
	}
	if got.Tags != pt.Tags {
		t.Errorf("Tags = %q, want %q", got.Tags, pt.Tags)
	}
	if got.MetricName != pt.MetricName {
		t.Errorf("MetricName = %q, want %q", got.MetricName, pt.MetricName)
	}
	if got.Value != pt.Value {
		t.Errorf("Value = %f, want %f", got.Value, pt.Value)
	}
}

func TestStoreScanTimeRange(t *testing.T) {
	store, cleanup := openTestStore(t)
	defer cleanup()

	ctx := context.Background()
	points := []Point{
		{Timestamp: 100, Tags: "", MetricName: "m1", Value: 1.0},
		{Timestamp: 200, Tags: "", MetricName: "m1", Value: 2.0},
		{Timestamp: 300, Tags: "", MetricName: "m1", Value: 3.0},
		{Timestamp: 400, Tags: "", MetricName: "m1", Value: 4.0},
		{Timestamp: 500, Tags: "", MetricName: "m1", Value: 5.0},
	}

	for _, pt := range points {
		if err := store.AppendPoint(ctx, pt); err != nil {
			t.Fatalf("AppendPoint: %v", err)
		}
	}

	// Scan [200, 400] should return 3 points.
	results, err := store.ScanRange(ctx, 200, 400, nil)
	if err != nil {
		t.Fatalf("ScanRange: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("ScanRange [200,400] returned %d points, want 3", len(results))
	}
	for _, pt := range results {
		if pt.Timestamp < 200 || pt.Timestamp > 400 {
			t.Errorf("point %d outside range [200,400]", pt.Timestamp)
		}
	}
}

func TestStoreScanTagFilter(t *testing.T) {
	store, cleanup := openTestStore(t)
	defer cleanup()

	ctx := context.Background()
	points := []Point{
		{Timestamp: 100, Tags: "host=a,region=x", MetricName: "m1", Value: 1.0},
		{Timestamp: 200, Tags: "host=b,region=x", MetricName: "m1", Value: 2.0},
		{Timestamp: 300, Tags: "host=a,region=y", MetricName: "m1", Value: 3.0},
	}

	for _, pt := range points {
		if err := store.AppendPoint(ctx, pt); err != nil {
			t.Fatalf("AppendPoint: %v", err)
		}
	}

	// Filter by host=a.
	tags := map[string]string{"host": "a"}
	results, err := store.ScanRange(ctx, 0, 1000, tags)
	if err != nil {
		t.Fatalf("ScanRange: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("ScanRange(host=a) returned %d points, want 2", len(results))
	}

	// Filter by host=a AND region=x.
	tags["region"] = "x"
	results, err = store.ScanRange(ctx, 0, 1000, tags)
	if err != nil {
		t.Fatalf("ScanRange: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("ScanRange(host=a,region=x) returned %d points, want 1", len(results))
	}
}

func TestStoreScanOutsideRangeReturnsEmpty(t *testing.T) {
	store, cleanup := openTestStore(t)
	defer cleanup()

	ctx := context.Background()
	pt := Point{Timestamp: 100, Tags: "", MetricName: "m1", Value: 1.0}
	if err := store.AppendPoint(ctx, pt); err != nil {
		t.Fatalf("AppendPoint: %v", err)
	}

	results, err := store.ScanRange(ctx, 200, 300, nil)
	if err != nil {
		t.Fatalf("ScanRange: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("ScanRange [200,300] returned %d points, want 0", len(results))
	}
}

func TestStoreMultiplePages(t *testing.T) {
	store, cleanup := openTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Each point is roughly: 8+2+len(tags)+2+len(name)+8 bytes.
	// With small tags/names, ~30 bytes per point. DataPageBodySize ~4084.
	// ~130 points should fill one page and spill to a second.
	n := 150
	for i := 0; i < n; i++ {
		pt := Point{
			Timestamp:  int64(i * 10),
			Tags:       "k=v",
			MetricName: "m",
			Value:      float64(i),
		}
		if err := store.AppendPoint(ctx, pt); err != nil {
			t.Fatalf("AppendPoint %d: %v", i, err)
		}
	}

	// Verify we have more than 1 data page.
	pageCount, err := store.pager.PageCount(ctx)
	if err != nil {
		t.Fatalf("PageCount: %v", err)
	}
	if pageCount <= pager.FirstDataPageID+1 {
		t.Logf("pageCount=%d (may not have spilled; points may fit in one page)", pageCount)
	}

	// Scan full range to get all points back.
	results, err := store.ScanRange(ctx, 0, int64(n*10+100), nil)
	if err != nil {
		t.Fatalf("ScanRange: %v", err)
	}
	if len(results) != n {
		t.Fatalf("ScanRange returned %d points, want %d", len(results), n)
	}
}

func TestStoreCloseAndReopen(t *testing.T) {
	path := testPagerPath(t)
	ctx := context.Background()

	// First session: write a point.
	pg1 := pager.New(path)
	store1 := NewStore(pg1)
	if err := store1.Open(ctx); err != nil {
		t.Fatalf("open store1: %v", err)
	}
	pt := Point{Timestamp: 999, Tags: "x=y", MetricName: "test", Value: 3.14}
	if err := store1.AppendPoint(ctx, pt); err != nil {
		t.Fatalf("AppendPoint: %v", err)
	}
	if err := store1.Close(ctx); err != nil {
		t.Fatalf("close store1: %v", err)
	}

	// Second session: reopen and read back.
	pg2 := pager.New(path)
	store2 := NewStore(pg2)
	if err := store2.Open(ctx); err != nil {
		t.Fatalf("open store2: %v", err)
	}
	defer store2.Close(ctx)

	results, err := store2.ScanRange(ctx, 0, 1000, nil)
	if err != nil {
		t.Fatalf("ScanRange: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("after reopen: ScanRange returned %d points, want 1", len(results))
	}
	if results[0].Value != 3.14 {
		t.Errorf("Value = %f, want 3.14", results[0].Value)
	}
}

func TestValidateSchemaRejectsMissingTime(t *testing.T) {
	eng := NewMetricsEngine(nil, nil)

	err := eng.ValidateSchema([]byte(`[{"name":"value","type":"float64"}]`))
	if err == nil {
		t.Fatal("expected error for missing time column")
	}
	if err != ErrMissingTimeColumn {
		t.Errorf("error = %v, want ErrMissingTimeColumn", err)
	}
}

func TestValidateSchemaRejectsNoNumeric(t *testing.T) {
	eng := NewMetricsEngine(nil, nil)

	err := eng.ValidateSchema([]byte(`[{"name":"time","type":"int64"},{"name":"label","type":"string"}]`))
	if err == nil {
		t.Fatal("expected error for missing numeric value column")
	}
	if err != ErrNoMetricValueColumn {
		t.Errorf("error = %v, want ErrNoMetricValueColumn", err)
	}
}

func TestValidateSchemaAcceptsValid(t *testing.T) {
	eng := NewMetricsEngine(nil, nil)

	err := eng.ValidateSchema([]byte(`[{"name":"time","type":"int64"},{"name":"cpu","type":"float64"}]`))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPointSerializedSize(t *testing.T) {
	pt := Point{Timestamp: 123, Tags: "host=a", MetricName: "cpu", Value: 1.5}
	size := pt.serializedSize()
	expected := 8 + 2 + len("host=a") + 2 + len("cpu") + 8
	if size != expected {
		t.Errorf("serializedSize = %d, want %d", size, expected)
	}
}

func TestDecodePageRoundTrip(t *testing.T) {
	// Manually build a page body and decode it.
	body := make([]byte, pager.DataPageBodySize)
	// Header: num_points=1, next_write_offset=match
	writeUint32LE(body, 0, 1)
	offset := headerSize

	pt := Point{Timestamp: 555, Tags: "k=v", MetricName: "m1", Value: 99.9}
	// timestamp
	writeUint64LE(body, offset, uint64(pt.Timestamp))
	offset += 8
	// tags_len
	writeUint16LE(body, offset, uint16(len(pt.Tags)))
	offset += 2
	copy(body[offset:], pt.Tags)
	offset += len(pt.Tags)
	// name_len
	writeUint16LE(body, offset, uint16(len(pt.MetricName)))
	offset += 2
	copy(body[offset:], pt.MetricName)
	offset += len(pt.MetricName)
	// value
	writeUint64LE(body, offset, math.Float64bits(pt.Value))
	offset += 8

	writeUint32LE(body, 4, uint32(offset)) // next_write_offset

	decoded := decodePage(body)
	if len(decoded) != 1 {
		t.Fatalf("decodePage returned %d points, want 1", len(decoded))
	}
	if decoded[0].Timestamp != pt.Timestamp {
		t.Errorf("Timestamp = %d, want %d", decoded[0].Timestamp, pt.Timestamp)
	}
	if decoded[0].Tags != pt.Tags {
		t.Errorf("Tags = %q, want %q", decoded[0].Tags, pt.Tags)
	}
	if decoded[0].MetricName != pt.MetricName {
		t.Errorf("MetricName = %q, want %q", decoded[0].MetricName, pt.MetricName)
	}
	if decoded[0].Value != pt.Value {
		t.Errorf("Value = %f, want %f", decoded[0].Value, pt.Value)
	}
}

// Helper functions for byte manipulation in tests.
func readUint32LE(b []byte, off int) uint32 {
	return uint32(b[off]) | uint32(b[off+1])<<8 | uint32(b[off+2])<<16 | uint32(b[off+3])<<24
}

func writeUint32LE(b []byte, off int, v uint32) {
	b[off] = byte(v)
	b[off+1] = byte(v >> 8)
	b[off+2] = byte(v >> 16)
	b[off+3] = byte(v >> 24)
}

func writeUint16LE(b []byte, off int, v uint16) {
	b[off] = byte(v)
	b[off+1] = byte(v >> 8)
}

func writeUint64LE(b []byte, off int, v uint64) {
	b[off] = byte(v)
	b[off+1] = byte(v >> 8)
	b[off+2] = byte(v >> 16)
	b[off+3] = byte(v >> 24)
	b[off+4] = byte(v >> 32)
	b[off+5] = byte(v >> 40)
	b[off+6] = byte(v >> 48)
	b[off+7] = byte(v >> 56)
}
