// Package logs provides a pluggable logs engine for Plomvix.
// store.go implements a flat page-based append-only store using
// the storage/pager for sequential log record persistence.
package logs

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"

	"github.com/plomvix/plomvix/internal/storage/pager"
)

// Page layout constants.
const (
	logsHeaderSize    = 8 // num_records (uint32) + next_write_offset (uint32)
	logsTimestampSize = 8 // int64
	logsSeveritySize  = 1 // uint8
	logsAttrsLenSize  = 2 // uint16
	logsBodyLenSize   = 4 // uint32
	logsMaxBodySize   = pager.DataPageBodySize
)

// Severity constants.
const (
	SeverityDebug uint8 = 1
	SeverityInfo  uint8 = 2
	SeverityWarn  uint8 = 3
	SeverityError uint8 = 4
	SeverityFatal uint8 = 5
)

// Sentinel errors.
var (
	ErrStoreNotOpen      = errors.New("logs store: not open")
	ErrStoreAlreadyOpen  = errors.New("logs store: already open")
	ErrStoreClosed       = errors.New("logs store: closed")
	ErrRecordTooLarge    = errors.New("logs store: record exceeds page body size")
	ErrCorruptPageHeader = errors.New("logs store: corrupt page header")
)

// LogRecord represents a single log entry.
type LogRecord struct {
	Timestamp  int64
	Severity   uint8
	Attributes string // flat JSON string
	Body       string // raw text or JSON body
}

// serializedSize returns the number of bytes needed to serialize the record.
func (r *LogRecord) serializedSize() int {
	return logsTimestampSize + logsSeveritySize + logsAttrsLenSize + len(r.Attributes) +
		logsBodyLenSize + len(r.Body)
}

// LogsStore is a flat page-based append-only log store.
type LogsStore struct {
	pager         pager.Pager
	currentPageID uint64
	currentBody   []byte // cached copy of current page body
	opened        bool
}

// NewStore creates a LogsStore backed by the given pager.
func NewStore(pg pager.Pager) *LogsStore {
	return &LogsStore{
		pager: pg,
	}
}

// Open opens the store. If no pages exist yet, allocates the first data page.
func (s *LogsStore) Open(ctx context.Context) error {
	if s.opened {
		return ErrStoreAlreadyOpen
	}
	if err := s.pager.Open(ctx); err != nil {
		return fmt.Errorf("logs store: pager open: %w", err)
	}

	pageCount, err := s.pager.PageCount(ctx)
	if err != nil {
		return fmt.Errorf("logs store: page count: %w", err)
	}

	if pageCount <= pager.FirstDataPageID {
		// Fresh store: allocate the first data page.
		pageID, err := s.pager.AllocatePage(ctx)
		if err != nil {
			return fmt.Errorf("logs store: allocate first page: %w", err)
		}
		s.currentPageID = pageID
		s.currentBody = make([]byte, logsMaxBodySize)
		// Write zero header to initialize.
		s.writeHeader(0, logsHeaderSize)
		if err := s.flushPage(ctx); err != nil {
			return err
		}
	} else {
		// Existing store: walk to the last data page.
		lastID := uint64(pager.FirstDataPageID)
		for id := uint64(pager.FirstDataPageID); id < pageCount; id++ {
			lastID = id
		}
		s.currentPageID = lastID
		body, err := s.pager.ReadPage(ctx, lastID)
		if err != nil {
			return fmt.Errorf("logs store: read last page %d: %w", lastID, err)
		}
		s.currentBody = make([]byte, logsMaxBodySize)
		copy(s.currentBody, body)
	}

	s.opened = true
	return nil
}

// Close flushes the current page and closes the pager.
func (s *LogsStore) Close(ctx context.Context) error {
	if !s.opened {
		return ErrStoreNotOpen
	}
	if err := s.flushPage(ctx); err != nil {
		return err
	}
	s.opened = false
	return s.pager.Close(ctx)
}

// AppendLog serializes and appends a single log record to the store.
func (s *LogsStore) AppendLog(ctx context.Context, rec LogRecord) error {
	if !s.opened {
		return ErrStoreNotOpen
	}

	recSize := rec.serializedSize()
	if recSize > logsMaxBodySize-logsHeaderSize {
		return ErrRecordTooLarge
	}

	numRecs := s.readNumRecords()
	nextOffset := s.readNextWriteOffset()

	// If the record doesn't fit, flush current page and allocate a new one.
	if int(nextOffset)+recSize > logsMaxBodySize {
		if err := s.flushPage(ctx); err != nil {
			return err
		}
		newPageID, err := s.pager.AllocatePage(ctx)
		if err != nil {
			return fmt.Errorf("logs store: allocate new page: %w", err)
		}
		s.currentPageID = newPageID
		s.currentBody = make([]byte, logsMaxBodySize)
		s.writeHeader(0, logsHeaderSize)
		numRecs = 0
		nextOffset = logsHeaderSize
	}

	// Serialize the log record at nextOffset.
	offset := int(nextOffset)
	// timestamp (int64, 8 bytes)
	binary.LittleEndian.PutUint64(s.currentBody[offset:], uint64(rec.Timestamp))
	offset += logsTimestampSize
	// severity (uint8, 1 byte)
	s.currentBody[offset] = rec.Severity
	offset += logsSeveritySize
	// attributes_len (uint16, 2 bytes)
	binary.LittleEndian.PutUint16(s.currentBody[offset:], uint16(len(rec.Attributes)))
	offset += logsAttrsLenSize
	// attributes payload
	copy(s.currentBody[offset:], rec.Attributes)
	offset += len(rec.Attributes)
	// body_len (uint32, 4 bytes)
	binary.LittleEndian.PutUint32(s.currentBody[offset:], uint32(len(rec.Body)))
	offset += logsBodyLenSize
	// body payload
	copy(s.currentBody[offset:], rec.Body)
	offset += len(rec.Body)

	// Update header.
	s.writeHeader(numRecs+1, uint32(offset))

	return s.flushPage(ctx)
}

// ScanRange iterates over all pages and returns records whose timestamp
// falls within [start, end] and whose body contains the given substring.
// An empty substring means no body filtering.
func (s *LogsStore) ScanRange(ctx context.Context, start, end int64, bodyFilter string) ([]LogRecord, error) {
	if !s.opened {
		return nil, ErrStoreNotOpen
	}

	pageCount, err := s.pager.PageCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("logs store: page count: %w", err)
	}

	var results []LogRecord
	for pageID := uint64(pager.FirstDataPageID); pageID < pageCount; pageID++ {
		body, err := s.pager.ReadPage(ctx, pageID)
		if err != nil {
			return nil, fmt.Errorf("logs store: read page %d: %w", pageID, err)
		}
		recs := decodeLogPage(body)
		for _, rec := range recs {
			if start != 0 && rec.Timestamp < start {
				continue
			}
			if end != 0 && rec.Timestamp > end {
				continue
			}
			if bodyFilter != "" && !strings.Contains(rec.Body, bodyFilter) {
				continue
			}
			results = append(results, rec)
		}
	}
	return results, nil
}

// writeHeader writes the page header fields into the in-memory body.
func (s *LogsStore) writeHeader(numRecords, nextWriteOffset uint32) {
	binary.LittleEndian.PutUint32(s.currentBody[0:4], numRecords)
	binary.LittleEndian.PutUint32(s.currentBody[4:8], nextWriteOffset)
}

// readNumRecords reads the number of records from the in-memory body.
func (s *LogsStore) readNumRecords() uint32 {
	return binary.LittleEndian.Uint32(s.currentBody[0:4])
}

// readNextWriteOffset reads the next write offset from the in-memory body.
func (s *LogsStore) readNextWriteOffset() uint32 {
	return binary.LittleEndian.Uint32(s.currentBody[4:8])
}

// flushPage writes the current body to disk.
func (s *LogsStore) flushPage(ctx context.Context) error {
	return s.pager.WritePage(ctx, s.currentPageID, s.currentBody)
}

// decodeLogPage decodes all log records from a page body.
func decodeLogPage(body []byte) []LogRecord {
	if len(body) < logsHeaderSize {
		return nil
	}
	numRecords := binary.LittleEndian.Uint32(body[0:4])
	if numRecords == 0 {
		return nil
	}
	recs := make([]LogRecord, 0, numRecords)
	offset := int(logsHeaderSize)

	for i := uint32(0); i < numRecords; i++ {
		if offset+logsTimestampSize > len(body) {
			break
		}
		rec := LogRecord{}

		// timestamp
		rec.Timestamp = int64(binary.LittleEndian.Uint64(body[offset:]))
		offset += logsTimestampSize

		// severity
		if offset+logsSeveritySize > len(body) {
			break
		}
		rec.Severity = body[offset]
		offset += logsSeveritySize

		// attributes
		if offset+logsAttrsLenSize > len(body) {
			break
		}
		attrsLen := int(binary.LittleEndian.Uint16(body[offset:]))
		offset += logsAttrsLenSize
		if attrsLen > 0 {
			if offset+attrsLen > len(body) {
				break
			}
			rec.Attributes = string(body[offset : offset+attrsLen])
			offset += attrsLen
		}

		// body
		if offset+logsBodyLenSize > len(body) {
			break
		}
		bodyLen := int(binary.LittleEndian.Uint32(body[offset:]))
		offset += logsBodyLenSize
		if bodyLen > 0 {
			if offset+bodyLen > len(body) {
				break
			}
			rec.Body = string(body[offset : offset+bodyLen])
			offset += bodyLen
		}

		recs = append(recs, rec)
	}
	return recs
}

// severityToString maps severity uint8 to its string representation.
func severityToString(sev uint8) string {
	switch sev {
	case SeverityDebug:
		return "DEBUG"
	case SeverityInfo:
		return "INFO"
	case SeverityWarn:
		return "WARN"
	case SeverityError:
		return "ERROR"
	case SeverityFatal:
		return "FATAL"
	default:
		return "INFO"
	}
}

// parseSeverity maps a string to its severity uint8 value.
func parseSeverity(s string) uint8 {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return SeverityDebug
	case "INFO", "INFORMATION":
		return SeverityInfo
	case "WARN", "WARNING":
		return SeverityWarn
	case "ERROR":
		return SeverityError
	case "FATAL", "CRITICAL":
		return SeverityFatal
	default:
		return SeverityInfo
	}
}
