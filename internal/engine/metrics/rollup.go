// Package metrics provides a time-series metrics engine for Plomvix.
// rollup.go implements background downsampling workers that periodically
// consolidate raw metric points into 1-minute and 5-minute aggregation
// buckets stored in a separate rollup pager file.
package metrics

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/plomvix/plomvix/internal/storage/pager"
)

// Rollup store page layout constants.
const (
	rollupHeaderSize         = 12 // num_buckets (uint32) + next_write_offset (uint32) + reserved (uint32)
	rollupBucketTimestampSize = 8  // int64
	rollupPointCountSize      = 4  // uint32
	rollupSumValueSize        = 8  // float64
	rollupMinValueSize        = 8  // float64
	rollupMaxValueSize        = 8  // float64
	rollupBucketSize          = rollupBucketTimestampSize + rollupPointCountSize +
		rollupSumValueSize + rollupMinValueSize + rollupMaxValueSize // 36 bytes
)

// RollupBucket represents an aggregated time bucket.
type RollupBucket struct {
	BucketStart int64
	PointCount  uint32
	SumValue    float64
	MinValue    float64
	MaxValue    float64
}

// RollupManager periodically downsamples raw metric points into
// fixed-interval aggregation buckets, writing them to a dedicated
// rollup pager file.
type RollupManager struct {
	sourceStore *MetricsStore
	rollupPager pager.Pager
	resolutions []time.Duration // e.g. [1m, 5m]

	mu           sync.Mutex
	currentPageID uint64
	currentBody   []byte
	opened        bool

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// RollupConfig configures the rollup manager.
type RollupConfig struct {
	RollupDBPath string
	Resolutions  []time.Duration
	TickInterval time.Duration
}

// DefaultRollupConfig returns sensible defaults.
func DefaultRollupConfig() RollupConfig {
	return RollupConfig{
		RollupDBPath: "data/metrics_rollups.db",
		Resolutions:  []time.Duration{1 * time.Minute, 5 * time.Minute},
		TickInterval: 30 * time.Second,
	}
}

// Sentinel errors for rollup operations.
var (
	ErrRollupNotOpen      = errors.New("rollup: not open")
	ErrRollupAlreadyOpen  = errors.New("rollup: already open")
	ErrRollupNoSource     = errors.New("rollup: source store is nil")
)

// NewRollupManager creates a new rollup manager.
func NewRollupManager(source *MetricsStore, cfg RollupConfig) *RollupManager {
	pg := pager.New(cfg.RollupDBPath)
	return &RollupManager{
		sourceStore: source,
		rollupPager: pg,
		resolutions: cfg.Resolutions,
	}
}

// Open starts the rollup manager: opens the rollup pager and starts the
// background downsampling goroutine.
func (rm *RollupManager) Open(ctx context.Context) error {
	if rm.opened {
		return ErrRollupAlreadyOpen
	}
	if rm.sourceStore == nil {
		return ErrRollupNoSource
	}

	if err := rm.rollupPager.Open(ctx); err != nil {
		return fmt.Errorf("rollup: open pager: %w", err)
	}

	// Initialize the rollup store's first data page if needed.
	pageCount, err := rm.rollupPager.PageCount(ctx)
	if err != nil {
		return fmt.Errorf("rollup: page count: %w", err)
	}
	if pageCount <= pager.FirstDataPageID {
		pageID, err := rm.rollupPager.AllocatePage(ctx)
		if err != nil {
			return fmt.Errorf("rollup: allocate first page: %w", err)
		}
		rm.currentPageID = pageID
		rm.currentBody = make([]byte, pager.DataPageBodySize)
		rm.writeRollupHeader(0, rollupHeaderSize)
		if err := rm.flushRollupPage(ctx); err != nil {
			return err
		}
	} else {
		// Load last page.
		lastID := uint64(pager.FirstDataPageID)
		for id := uint64(pager.FirstDataPageID); id < pageCount; id++ {
			lastID = id
		}
		rm.currentPageID = lastID
		body, err := rm.rollupPager.ReadPage(ctx, lastID)
		if err != nil {
			return fmt.Errorf("rollup: read last page: %w", err)
		}
		rm.currentBody = make([]byte, pager.DataPageBodySize)
		copy(rm.currentBody, body)
	}

	rm.opened = true
	return nil
}

// Close stops the background worker and closes the rollup pager.
func (rm *RollupManager) Close(ctx context.Context) error {
	if !rm.opened {
		return ErrRollupNotOpen
	}
	if rm.cancel != nil {
		rm.cancel()
		rm.wg.Wait()
	}
	if err := rm.flushRollupPage(ctx); err != nil {
		return err
	}
	rm.opened = false
	return rm.rollupPager.Close(ctx)
}

// Name returns the component name for lifecycle management.
func (rm *RollupManager) Name() string { return "metrics.rollup" }

// Start begins the background downsampling goroutine.
func (rm *RollupManager) Start(ctx context.Context) error {
	rm.ctx, rm.cancel = context.WithCancel(ctx)
	rm.wg.Add(1)
	go rm.runLoop()
	return nil
}

// Stop cancels the background goroutine.
func (rm *RollupManager) Stop(ctx context.Context) error {
	if rm.cancel != nil {
		rm.cancel()
	}
	rm.wg.Wait()
	return nil
}

// runLoop periodically triggers downsampling.
func (rm *RollupManager) runLoop() {
	defer rm.wg.Done()
	ticker := time.NewTicker(DefaultRollupConfig().TickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rm.ctx.Done():
			return
		case <-ticker.C:
			for _, res := range rm.resolutions {
				_ = rm.downsample(rm.ctx, res)
			}
		}
	}
}

// ForceDownsample triggers an immediate downsample at the given resolution.
func (rm *RollupManager) ForceDownsample(ctx context.Context, resolution time.Duration) error {
	return rm.downsample(ctx, resolution)
}

// downsample scans source pages, groups points by time bucket at the given
// resolution, computes aggregates, and writes rollup buckets.
func (rm *RollupManager) downsample(ctx context.Context, resolution time.Duration) error {
	bucketSecs := int64(resolution.Seconds())
	if bucketSecs < 1 {
		bucketSecs = 1
	}

	// Scan all source pages for raw points.
	pageCount, err := rm.sourceStore.pager.PageCount(ctx)
	if err != nil {
		return fmt.Errorf("rollup: source page count: %w", err)
	}

	// Collect bucket aggregates: map[bucket_start] -> accumulator.
	type bucketAcc struct {
		count    uint32
		sum      float64
		min      float64
		max      float64
		hasFirst bool
	}
	buckets := make(map[int64]*bucketAcc)

	for pageID := uint64(pager.FirstDataPageID); pageID < pageCount; pageID++ {
		body, err := rm.sourceStore.pager.ReadPage(ctx, pageID)
		if err != nil {
			continue // skip corrupt pages
		}
		points := decodePage(body)
		for _, pt := range points {
			bucketStart := (pt.Timestamp / bucketSecs) * bucketSecs
			acc, ok := buckets[bucketStart]
			if !ok {
				acc = &bucketAcc{}
				buckets[bucketStart] = acc
			}
			acc.count++
			acc.sum += pt.Value
			if !acc.hasFirst {
				acc.min = pt.Value
				acc.max = pt.Value
				acc.hasFirst = true
			} else {
				if pt.Value < acc.min {
					acc.min = pt.Value
				}
				if pt.Value > acc.max {
					acc.max = pt.Value
				}
			}
		}
	}

	// Write rollup buckets to the rollup store.
	rm.mu.Lock()
	defer rm.mu.Unlock()

	for bucketStart, acc := range buckets {
		rb := RollupBucket{
			BucketStart: bucketStart,
			PointCount:  acc.count,
			SumValue:    acc.sum,
			MinValue:    acc.min,
			MaxValue:    acc.max,
		}
		if err := rm.appendRollupBucket(ctx, rb); err != nil {
			return err
		}
	}
	return nil
}

// appendRollupBucket writes a single rollup bucket to the current page.
func (rm *RollupManager) appendRollupBucket(ctx context.Context, rb RollupBucket) error {
	numBuckets := binary.LittleEndian.Uint32(rm.currentBody[0:4])
	nextOffset := binary.LittleEndian.Uint32(rm.currentBody[4:8])

	if int(nextOffset)+rollupBucketSize > pager.DataPageBodySize {
		if err := rm.flushRollupPage(ctx); err != nil {
			return err
		}
		newPageID, err := rm.rollupPager.AllocatePage(ctx)
		if err != nil {
			return fmt.Errorf("rollup: allocate new page: %w", err)
		}
		rm.currentPageID = newPageID
		rm.currentBody = make([]byte, pager.DataPageBodySize)
		rm.writeRollupHeader(0, rollupHeaderSize)
		numBuckets = 0
		nextOffset = rollupHeaderSize
	}

	off := int(nextOffset)
	binary.LittleEndian.PutUint64(rm.currentBody[off:], uint64(rb.BucketStart))
	off += rollupBucketTimestampSize
	binary.LittleEndian.PutUint32(rm.currentBody[off:], rb.PointCount)
	off += rollupPointCountSize
	binary.LittleEndian.PutUint64(rm.currentBody[off:], math.Float64bits(rb.SumValue))
	off += rollupSumValueSize
	binary.LittleEndian.PutUint64(rm.currentBody[off:], math.Float64bits(rb.MinValue))
	off += rollupMinValueSize
	binary.LittleEndian.PutUint64(rm.currentBody[off:], math.Float64bits(rb.MaxValue))

	rm.writeRollupHeader(numBuckets+1, uint32(off+rollupMaxValueSize))
	return rm.flushRollupPage(ctx)
}

// ScanRollups reads all rollup buckets from the rollup store.
func (rm *RollupManager) ScanRollups(ctx context.Context, start, end int64) ([]RollupBucket, error) {
	pageCount, err := rm.rollupPager.PageCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("rollup: page count: %w", err)
	}

	var results []RollupBucket
	for pageID := uint64(pager.FirstDataPageID); pageID < pageCount; pageID++ {
		body, err := rm.rollupPager.ReadPage(ctx, pageID)
		if err != nil {
			continue
		}
		buckets := decodeRollupPage(body)
		for _, rb := range buckets {
			if rb.BucketStart >= start && rb.BucketStart <= end {
				results = append(results, rb)
			}
		}
	}
	return results, nil
}

// writeRollupHeader updates the in-memory rollup page header.
func (rm *RollupManager) writeRollupHeader(numBuckets, nextOffset uint32) {
	binary.LittleEndian.PutUint32(rm.currentBody[0:4], numBuckets)
	binary.LittleEndian.PutUint32(rm.currentBody[4:8], nextOffset)
}

// flushRollupPage writes the current rollup body to disk.
func (rm *RollupManager) flushRollupPage(ctx context.Context) error {
	return rm.rollupPager.WritePage(ctx, rm.currentPageID, rm.currentBody)
}

// decodeRollupPage decodes rollup buckets from a page body.
func decodeRollupPage(body []byte) []RollupBucket {
	if len(body) < rollupHeaderSize {
		return nil
	}
	numBuckets := binary.LittleEndian.Uint32(body[0:4])
	if numBuckets == 0 {
		return nil
	}
	buckets := make([]RollupBucket, 0, numBuckets)
	off := int(rollupHeaderSize)

	for i := uint32(0); i < numBuckets; i++ {
		if off+rollupBucketSize > len(body) {
			break
		}
		rb := RollupBucket{
			BucketStart: int64(binary.LittleEndian.Uint64(body[off:])),
		}
		off += rollupBucketTimestampSize
		rb.PointCount = binary.LittleEndian.Uint32(body[off:])
		off += rollupPointCountSize
		rb.SumValue = math.Float64frombits(binary.LittleEndian.Uint64(body[off:]))
		off += rollupSumValueSize
		rb.MinValue = math.Float64frombits(binary.LittleEndian.Uint64(body[off:]))
		off += rollupMinValueSize
		rb.MaxValue = math.Float64frombits(binary.LittleEndian.Uint64(body[off:]))
		off += rollupMaxValueSize

		buckets = append(buckets, rb)
	}
	return buckets
}
