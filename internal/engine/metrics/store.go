// Package metrics provides a time-series metrics engine for Plomvix.
// store.go implements a flat page-based append-only log store using
// the storage/pager for sequential metric record persistence.
package metrics

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/plomvix/plomvix/internal/storage/pager"
)

// Page layout constants.
const (
	headerSize         = 8 // num_points (uint32) + next_write_offset (uint32)
	pointTimestampSize = 8 // int64
	pointTagsLenSize   = 2 // uint16
	pointNameLenSize   = 2 // uint16
	pointValueSize     = 8 // float64
	maxBodySize        = pager.DataPageBodySize
)

// Sentinel errors.
var (
	ErrStoreNotOpen      = errors.New("metrics store: not open")
	ErrStoreAlreadyOpen  = errors.New("metrics store: already open")
	ErrStoreClosed       = errors.New("metrics store: closed")
	ErrRecordTooLarge    = errors.New("metrics store: record exceeds page body size")
	ErrNoCurrentPage     = errors.New("metrics store: no current page allocated")
	ErrCorruptPageHeader = errors.New("metrics store: corrupt page header")
)

// Point represents a single metric data point.
type Point struct {
	Timestamp  int64
	Tags       string // key=value,... or JSON
	MetricName string
	Value      float64
}

// serializedSize returns the number of bytes needed to serialize the point.
func (p *Point) serializedSize() int {
	return pointTimestampSize + pointTagsLenSize + len(p.Tags) +
		pointNameLenSize + len(p.MetricName) + pointValueSize
}

// MetricsStore is a flat page-based append-only time-series store.
type MetricsStore struct {
	pager         pager.Pager
	currentPageID uint64
	currentBody   []byte // cached copy of current page body
	opened        bool
	tagIndex      *TagIndex // enterprise: inverted tag index for fast lookup
}

// NewStore creates a MetricsStore backed by the given pager.
func NewStore(pg pager.Pager) *MetricsStore {
	return NewStoreWithIndex(pg, nil)
}

// NewStoreWithIndex creates a MetricsStore with an optional TagIndex
// for enterprise tag-filtered queries.
func NewStoreWithIndex(pg pager.Pager, idx *TagIndex) *MetricsStore {
	return &MetricsStore{
		pager:    pg,
		tagIndex: idx,
	}
}

// Open opens the store. If no pages exist yet, allocates the first data page.
func (s *MetricsStore) Open(ctx context.Context) error {
	if s.opened {
		return ErrStoreAlreadyOpen
	}
	if err := s.pager.Open(ctx); err != nil {
		return fmt.Errorf("metrics store: pager open: %w", err)
	}

	pageCount, err := s.pager.PageCount(ctx)
	if err != nil {
		return fmt.Errorf("metrics store: page count: %w", err)
	}

	if pageCount <= pager.FirstDataPageID {
		// Fresh store: allocate the first data page.
		pageID, err := s.pager.AllocatePage(ctx)
		if err != nil {
			return fmt.Errorf("metrics store: allocate first page: %w", err)
		}
		s.currentPageID = pageID
		s.currentBody = make([]byte, maxBodySize)
		// Write zero header to initialize.
		s.writeHeader(0, headerSize)
		if err := s.flushPage(ctx); err != nil {
			return err
		}
	} else {
		// Existing store: load the last allocated page.
		// Walk to the last data page and start from there.
		lastID := uint64(pager.FirstDataPageID)
		for id := uint64(pager.FirstDataPageID); id < pageCount; id++ {
			lastID = id
		}
		s.currentPageID = lastID
		body, err := s.pager.ReadPage(ctx, lastID)
		if err != nil {
			return fmt.Errorf("metrics store: read last page %d: %w", lastID, err)
		}
		s.currentBody = make([]byte, maxBodySize)
		copy(s.currentBody, body)
	}

	s.opened = true
	return nil
}

// Close flushes the current page and closes the pager.
func (s *MetricsStore) Close(ctx context.Context) error {
	if !s.opened {
		return ErrStoreNotOpen
	}
	if err := s.flushPage(ctx); err != nil {
		return err
	}
	s.opened = false
	return s.pager.Close(ctx)
}

// AppendPoint serializes and appends a single metric point to the store.
func (s *MetricsStore) AppendPoint(ctx context.Context, pt Point) error {
	if !s.opened {
		return ErrStoreNotOpen
	}

	recSize := pt.serializedSize()
	if recSize > maxBodySize-headerSize {
		return ErrRecordTooLarge
	}

	numPoints := s.readNumPoints()
	nextOffset := s.readNextWriteOffset()

	// If the record doesn't fit, close current page and allocate a new one.
	if int(nextOffset)+recSize > maxBodySize {
		if err := s.flushPage(ctx); err != nil {
			return err
		}
		newPageID, err := s.pager.AllocatePage(ctx)
		if err != nil {
			return fmt.Errorf("metrics store: allocate new page: %w", err)
		}
		s.currentPageID = newPageID
		s.currentBody = make([]byte, maxBodySize)
		s.writeHeader(0, headerSize)
		numPoints = 0
		nextOffset = headerSize
	}

	// Serialize the point record at nextOffset.
	offset := int(nextOffset)
	// timestamp (int64, 8 bytes)
	binary.LittleEndian.PutUint64(s.currentBody[offset:], uint64(pt.Timestamp))
	offset += pointTimestampSize
	// tags_length (uint16, 2 bytes)
	binary.LittleEndian.PutUint16(s.currentBody[offset:], uint16(len(pt.Tags)))
	offset += pointTagsLenSize
	// tags payload
	copy(s.currentBody[offset:], pt.Tags)
	offset += len(pt.Tags)
	// metric_name_len (uint16, 2 bytes)
	binary.LittleEndian.PutUint16(s.currentBody[offset:], uint16(len(pt.MetricName)))
	offset += pointNameLenSize
	// metric_name payload
	copy(s.currentBody[offset:], pt.MetricName)
	offset += len(pt.MetricName)
	// value (float64, 8 bytes)
	binary.LittleEndian.PutUint64(s.currentBody[offset:], math.Float64bits(pt.Value))
	offset += pointValueSize

	// Update header.
	s.writeHeader(numPoints+1, uint32(offset))

	// Enterprise: index tags for fast lookup.
	if s.tagIndex != nil && pt.Tags != "" {
		loc := RecordLocator{
			PageID:    s.currentPageID,
			Offset:    uint32(offset - pt.serializedSize()),
			Timestamp: pt.Timestamp,
		}
		s.tagIndex.Insert(pt.Tags, loc)
	}

	return s.flushPage(ctx)
}

// ScanRange iterates over all pages and returns points whose timestamp
// falls within [start, end] and whose tags contain all required constraints.
// An empty tags map means no tag filtering.
func (s *MetricsStore) ScanRange(ctx context.Context, start, end int64, tags map[string]string) ([]Point, error) {
	if !s.opened {
		return nil, ErrStoreNotOpen
	}

	pageCount, err := s.pager.PageCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("metrics store: page count: %w", err)
	}

	var results []Point
	for pageID := uint64(pager.FirstDataPageID); pageID < pageCount; pageID++ {
		body, err := s.pager.ReadPage(ctx, pageID)
		if err != nil {
			return nil, fmt.Errorf("metrics store: read page %d: %w", pageID, err)
		}
		points := decodePage(body)
		for _, pt := range points {
			if pt.Timestamp < start || pt.Timestamp > end {
				continue
			}
			if !matchTags(pt.Tags, tags) {
				continue
			}
			results = append(results, pt)
		}
	}
	return results, nil
}

// ScanRangeWithIndex uses the tag index to find matching records
// instead of scanning all pages. Falls back to full scan if no index
// is available or if no tags are specified.
func (s *MetricsStore) ScanRangeWithIndex(ctx context.Context, start, end int64, tags map[string]string) ([]Point, error) {
	if !s.opened {
		return nil, ErrStoreNotOpen
	}

	// If we have an index and tag constraints, use it.
	if s.tagIndex != nil && len(tags) > 0 {
		locs := s.tagIndex.SearchAll(tags)
		if locs != nil {
			return s.readLocations(ctx, locs, start, end)
		}
	}
	// Fallback: full scan.
	return s.ScanRange(ctx, start, end, tags)
}

// readLocations reads specific record locations from pages.
func (s *MetricsStore) readLocations(ctx context.Context, locs []RecordLocator, start, end int64) ([]Point, error) {
	// Group locators by page to minimize reads.
	pageLocs := make(map[uint64][]uint32)
	for _, loc := range locs {
		pageLocs[loc.PageID] = append(pageLocs[loc.PageID], loc.Offset)
	}

	var results []Point
	for pageID, offsets := range pageLocs {
		body, err := s.pager.ReadPage(ctx, pageID)
		if err != nil {
			continue
		}
		for _, off := range offsets {
			if int(off) >= len(body) {
				continue
			}
			pt, consumed := decodeRawPoint(body[off:])
			if consumed == 0 {
				continue
			}
			if pt.Timestamp >= start && (end == 0 || pt.Timestamp <= end) {
				results = append(results, pt)
			}
		}
	}
	return results, nil
}

// writeHeader writes the page header fields into the in-memory body.
func (s *MetricsStore) writeHeader(numPoints, nextWriteOffset uint32) {
	binary.LittleEndian.PutUint32(s.currentBody[0:4], numPoints)
	binary.LittleEndian.PutUint32(s.currentBody[4:8], nextWriteOffset)
}

// readNumPoints reads the number of points from the in-memory body.
func (s *MetricsStore) readNumPoints() uint32 {
	return binary.LittleEndian.Uint32(s.currentBody[0:4])
}

// readNextWriteOffset reads the next write offset from the in-memory body.
func (s *MetricsStore) readNextWriteOffset() uint32 {
	return binary.LittleEndian.Uint32(s.currentBody[4:8])
}

// flushPage writes the current body to disk.
func (s *MetricsStore) flushPage(ctx context.Context) error {
	return s.pager.WritePage(ctx, s.currentPageID, s.currentBody)
}

// decodePage decodes all metric points from a page body.
func decodePage(body []byte) []Point {
	if len(body) < headerSize {
		return nil
	}
	numPoints := binary.LittleEndian.Uint32(body[0:4])
	if numPoints == 0 {
		return nil
	}
	points := make([]Point, 0, numPoints)
	offset := int(headerSize)

	for i := uint32(0); i < numPoints; i++ {
		if offset+pointTimestampSize > len(body) {
			break
		}
		pt := Point{}

		// timestamp
		pt.Timestamp = int64(binary.LittleEndian.Uint64(body[offset:]))
		offset += pointTimestampSize

		// tags
		if offset+pointTagsLenSize > len(body) {
			break
		}
		tagsLen := int(binary.LittleEndian.Uint16(body[offset:]))
		offset += pointTagsLenSize
		if offset+tagsLen > len(body) {
			break
		}
		pt.Tags = string(body[offset : offset+tagsLen])
		offset += tagsLen

		// metric_name
		if offset+pointNameLenSize > len(body) {
			break
		}
		nameLen := int(binary.LittleEndian.Uint16(body[offset:]))
		offset += pointNameLenSize
		if offset+nameLen > len(body) {
			break
		}
		pt.MetricName = string(body[offset : offset+nameLen])
		offset += nameLen

		// value
		if offset+pointValueSize > len(body) {
			break
		}
		bits := binary.LittleEndian.Uint64(body[offset:])
		pt.Value = math.Float64frombits(bits)
		offset += pointValueSize

		points = append(points, pt)
	}
	return points
}

// matchTags checks if a tags string contains all required key=value pairs.
func matchTags(tagsStr string, required map[string]string) bool {
	if len(required) == 0 {
		return true
	}
	// Simple comma-separated key=value matching.
	pairs := splitTagPairs(tagsStr)
	for k, v := range required {
		if pairs[k] != v {
			return false
		}
	}
	return true
}

// splitTagPairs parses a simple key=value,key=value,... string into a map.
func splitTagPairs(s string) map[string]string {
	m := make(map[string]string)
	if s == "" {
		return m
	}
	// Handle both comma-separated and JSON formats.
	if len(s) > 0 && s[0] == '{' {
		// Simple JSON object parser for flat key-value pairs.
		s = s[1 : len(s)-1] // strip { }
	}
	for _, pair := range bytes.Split([]byte(s), []byte(",")) {
		kv := bytes.SplitN(bytes.TrimSpace(pair), []byte("="), 2)
		if len(kv) == 2 {
			key := string(bytes.TrimSpace(kv[0]))
			val := string(bytes.Trim(bytes.TrimSpace(kv[1]), "\""))
			m[key] = val
		}
	}
	return m
}
