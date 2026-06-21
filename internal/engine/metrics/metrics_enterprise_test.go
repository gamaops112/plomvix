// Package metrics provides a time-series metrics engine for Plomvix.
// metrics_enterprise_test.go validates Gorilla compression, tag index
// lookups, and rollup downsampling correctness.
package metrics

import (
	"context"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/plomvix/plomvix/internal/storage/pager"
)

// --- Gorilla Compression Tests ---

func TestGorillaTimestampRoundTrip(t *testing.T) {
	inputs := []int64{
		1700000000,
		1700000001,
		1700000001, // same delta-of-delta = 0
		1700000010, // delta 9, dod 8
		1700000100, // delta 90, dod 81
		1700001000, // delta 900, dod 810
		2000000000, // large jump
		2000000001,
		2000000002,
	}

	enc := newGorillaEncoder(256)
	for _, ts := range inputs {
		enc.writeTimestamp(ts)
	}
	data := enc.bytes()

	dec := newGorillaDecoder(data)
	for i, want := range inputs {
		got := dec.readTimestamp()
		if got != want {
			t.Fatalf("ts[%d]: got %d, want %d", i, got, want)
		}
	}
}

func TestGorillaFloatRoundTrip(t *testing.T) {
	inputs := []float64{
		42.5,
		42.5, // same value
		43.0,
		100.0,
		0.001,
		-5.5,
		math.Pi,
		1e9,
	}

	enc := newGorillaEncoder(256)
	for _, v := range inputs {
		enc.writeFloat(v)
	}
	data := enc.bytes()

	dec := newGorillaDecoder(data)
	for i, want := range inputs {
		got := dec.readFloat()
		if got != want {
			t.Fatalf("float[%d]: got %f, want %f", i, got, want)
		}
	}
}

func TestGorillaCompressionRatio(t *testing.T) {
	// Generate 1000 timestamps with 1-second intervals (highly compressible).
	enc := newGorillaEncoder(8192)
	base := int64(1700000000)
	for i := 0; i < 1000; i++ {
		enc.writeTimestamp(base + int64(i))
		enc.writeFloat(float64(i) * 0.5)
	}
	compressed := len(enc.bytes())
	rawSize := 1000 * 16 // 8 bytes ts + 8 bytes float
	ratio := float64(compressed) / float64(rawSize)

	t.Logf("compressed %d bytes, raw %d bytes, ratio %.2f", compressed, rawSize, ratio)
	if ratio > 0.6 {
		t.Errorf("expected compression ratio < 0.6, got %.2f", ratio)
	}
}

func TestGorillaLeadingTrailingZeros(t *testing.T) {
	tests := []struct {
		x        uint64
		leading  int
		trailing int
	}{
		{0, 64, 64},
		{1, 63, 0},
		{0x8000000000000000, 0, 63},
		{0x3FF0000000000000, 2, 52}, // IEEE 754 for 1.0
		{0x0000FFFFFFFFFFFF, 16, 0},
	}

	for _, tt := range tests {
		l := leadingZeros64(tt.x)
		tr := trailingZeros64(tt.x)
		if l != tt.leading {
			t.Errorf("leadingZeros64(%016x) = %d, want %d", tt.x, l, tt.leading)
		}
		if tr != tt.trailing {
			t.Errorf("trailingZeros64(%016x) = %d, want %d", tt.x, tr, tt.trailing)
		}
	}
}

// --- Tag Index Tests ---

func TestTagIndexInsertAndSearch(t *testing.T) {
	cfg := DefaultTagIndexConfig()
	idx := NewTagIndex(cfg)

	loc := RecordLocator{PageID: 2, Offset: 100, Timestamp: 1000}
	idx.Insert("host=server1,region=us-east", loc)
	idx.Insert("host=server2,region=us-west", RecordLocator{PageID: 2, Offset: 200, Timestamp: 2000})

	// Search single tag.
	locs := idx.Search("host", "server1")
	if len(locs) != 1 {
		t.Fatalf("Search(host,server1) returned %d locs, want 1", len(locs))
	}
	if locs[0].Offset != 100 {
		t.Errorf("loc offset = %d, want 100", locs[0].Offset)
	}

	// Search non-existent tag.
	locs = idx.Search("host", "nonexistent")
	if len(locs) != 0 {
		t.Errorf("Search(host,nonexistent) returned %d locs, want 0", len(locs))
	}
}

func TestTagIndexSearchAllIntersection(t *testing.T) {
	cfg := DefaultTagIndexConfig()
	idx := NewTagIndex(cfg)

	idx.Insert("host=a,region=x", RecordLocator{PageID: 2, Offset: 10, Timestamp: 100})
	idx.Insert("host=a,region=y", RecordLocator{PageID: 2, Offset: 20, Timestamp: 200})
	idx.Insert("host=a,region=x", RecordLocator{PageID: 2, Offset: 30, Timestamp: 300})

	// Search host=a AND region=x should return 2 locs.
	locs := idx.SearchAll(map[string]string{"host": "a", "region": "x"})
	if len(locs) != 2 {
		t.Fatalf("SearchAll(host=a,region=x) returned %d locs, want 2", len(locs))
	}
}

func TestTagIndexSweepExpired(t *testing.T) {
	cfg := DefaultTagIndexConfig()
	cfg.RetentionWindow = 1 * time.Hour
	idx := NewTagIndex(cfg)

	now := time.Now()
	old := now.Add(-2 * time.Hour).Unix()
	fresh := now.Unix()

	idx.Insert("host=old", RecordLocator{PageID: 2, Offset: 10, Timestamp: old})
	idx.Insert("host=fresh", RecordLocator{PageID: 2, Offset: 20, Timestamp: fresh})

	idx.Sweep(now)

	locs := idx.Search("host", "old")
	if len(locs) != 0 {
		t.Errorf("old entry should be swept, got %d locs", len(locs))
	}
	locs = idx.Search("host", "fresh")
	if len(locs) != 1 {
		t.Errorf("fresh entry should remain, got %d locs", len(locs))
	}
}

// --- Rollup Tests ---

func TestRollupDownsampleAndScan(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	// Create source store with some points.
	srcPath := filepath.Join(dir, "src.db")
	srcPager := pager.New(srcPath)
	srcStore := NewStore(srcPager)
	if err := srcStore.Open(ctx); err != nil {
		t.Fatalf("open src store: %v", err)
	}
	defer srcStore.Close(ctx)

	// Write points at various timestamps within 1-minute buckets.
	baseTime := int64(1700000000)
	points := []Point{
		{Timestamp: baseTime, Tags: "", MetricName: "cpu", Value: 10.0},
		{Timestamp: baseTime + 10, Tags: "", MetricName: "cpu", Value: 20.0},
		{Timestamp: baseTime + 20, Tags: "", MetricName: "cpu", Value: 30.0},
		{Timestamp: baseTime + 60, Tags: "", MetricName: "cpu", Value: 5.0},
		{Timestamp: baseTime + 61, Tags: "", MetricName: "cpu", Value: 15.0},
	}
	for _, pt := range points {
		if err := srcStore.AppendPoint(ctx, pt); err != nil {
			t.Fatalf("AppendPoint: %v", err)
		}
	}

	// Create rollup manager.
	cfg := DefaultRollupConfig()
	cfg.RollupDBPath = filepath.Join(dir, "rollup.db")
	cfg.Resolutions = []time.Duration{1 * time.Minute}
	rm := NewRollupManager(srcStore, cfg)

	if err := rm.Open(ctx); err != nil {
		t.Fatalf("open rollup: %v", err)
	}
	defer rm.Close(ctx)

	// Force downsampling at 1-minute resolution.
	if err := rm.ForceDownsample(ctx, 1*time.Minute); err != nil {
		t.Fatalf("ForceDownsample: %v", err)
	}

	// Scan rollups.
	buckets, err := rm.ScanRollups(ctx, baseTime-1000, baseTime+1000)
	if err != nil {
		t.Fatalf("ScanRollups: %v", err)
	}

	// We expect 2 buckets: one for baseTime (first 3 points) and one for baseTime+60 (last 2 points).
	if len(buckets) < 2 {
		t.Fatalf("ScanRollups returned %d buckets, want at least 2", len(buckets))
	}

	// Bucket 1: sum=60, count=3, min=10, max=30.
	bucket1Start := (baseTime / 60) * 60
	for _, b := range buckets {
		if b.BucketStart == bucket1Start {
			if b.PointCount != 3 {
				t.Errorf("bucket1 count = %d, want 3", b.PointCount)
			}
			if b.SumValue != 60.0 {
				t.Errorf("bucket1 sum = %f, want 60.0", b.SumValue)
			}
			if b.MinValue != 10.0 {
				t.Errorf("bucket1 min = %f, want 10.0", b.MinValue)
			}
			if b.MaxValue != 30.0 {
				t.Errorf("bucket1 max = %f, want 30.0", b.MaxValue)
			}
		}
	}
}

func TestRollupBucketRoundTrip(t *testing.T) {
	// Encode a rollup bucket manually and decode it.
	body := make([]byte, pager.DataPageBodySize)
	writeUint32LE(body, 0, 1)       // num_buckets
	writeUint32LE(body, 4, 36+12)   // next_write_offset

	off := 12
	rb := RollupBucket{
		BucketStart: 1700000000,
		PointCount:  42,
		SumValue:    100.5,
		MinValue:    0.1,
		MaxValue:    99.9,
	}
	writeUint64LE(body, off, uint64(rb.BucketStart))
	off += 8
	writeUint32LE(body, off, rb.PointCount)
	off += 4
	writeUint64LE(body, off, math.Float64bits(rb.SumValue))
	off += 8
	writeUint64LE(body, off, math.Float64bits(rb.MinValue))
	off += 8
	writeUint64LE(body, off, math.Float64bits(rb.MaxValue))

	buckets := decodeRollupPage(body)
	if len(buckets) != 1 {
		t.Fatalf("decodeRollupPage returned %d buckets, want 1", len(buckets))
	}
	got := buckets[0]
	if got.BucketStart != rb.BucketStart {
		t.Errorf("BucketStart = %d, want %d", got.BucketStart, rb.BucketStart)
	}
	if got.PointCount != rb.PointCount {
		t.Errorf("PointCount = %d, want %d", got.PointCount, rb.PointCount)
	}
	if got.SumValue != rb.SumValue {
		t.Errorf("SumValue = %f, want %f", got.SumValue, rb.SumValue)
	}
}

func TestEncodeDecodeRawPoint(t *testing.T) {
	pt := Point{Timestamp: 999, Tags: "k=v", MetricName: "m1", Value: 3.14}
	data := encodeRawPoint(pt)
	decoded, consumed := decodeRawPoint(data)
	if consumed == 0 {
		t.Fatal("decodeRawPoint returned 0 bytes")
	}
	if decoded.Timestamp != pt.Timestamp {
		t.Errorf("Timestamp = %d, want %d", decoded.Timestamp, pt.Timestamp)
	}
	if decoded.Tags != pt.Tags {
		t.Errorf("Tags = %q, want %q", decoded.Tags, pt.Tags)
	}
	if decoded.MetricName != pt.MetricName {
		t.Errorf("MetricName = %q, want %q", decoded.MetricName, pt.MetricName)
	}
	if decoded.Value != pt.Value {
		t.Errorf("Value = %f, want %f", decoded.Value, pt.Value)
	}
}

func TestStoreScanRangeWithIndex(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	idx := NewTagIndex(DefaultTagIndexConfig())
	pg := pager.New(filepath.Join(dir, "test.db"))
	store := NewStoreWithIndex(pg, idx)
	if err := store.Open(ctx); err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close(ctx)

	// Insert points with different tags.
	pts := []Point{
		{Timestamp: 100, Tags: "host=a,region=us", MetricName: "cpu", Value: 1.0},
		{Timestamp: 200, Tags: "host=b,region=us", MetricName: "cpu", Value: 2.0},
		{Timestamp: 300, Tags: "host=a,region=eu", MetricName: "cpu", Value: 3.0},
	}
	for _, pt := range pts {
		if err := store.AppendPoint(ctx, pt); err != nil {
			t.Fatalf("AppendPoint: %v", err)
		}
	}

	// Use ScanRangeWithIndex with tag constraint.
	tags := map[string]string{"host": "a"}
	results, err := store.ScanRangeWithIndex(ctx, 0, 1000, tags)
	if err != nil {
		t.Fatalf("ScanRangeWithIndex: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("ScanRangeWithIndex(host=a) returned %d points, want 2", len(results))
	}
}

func TestIntersectLocators(t *testing.T) {
	a := []RecordLocator{
		{PageID: 2, Offset: 10},
		{PageID: 2, Offset: 20},
		{PageID: 3, Offset: 5},
	}
	b := []RecordLocator{
		{PageID: 2, Offset: 20},
		{PageID: 3, Offset: 5},
		{PageID: 3, Offset: 10},
	}

	result := intersectLocs(a, b)
	if len(result) != 2 {
		t.Fatalf("intersectLocs returned %d, want 2", len(result))
	}
	if result[0].Offset != 20 || result[1].Offset != 5 {
		t.Errorf("unexpected intersection result: %+v", result)
	}
}
