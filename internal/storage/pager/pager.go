// Package pager provides a fixed-size page manager backed by a single on-disk
// file. It handles page allocation, read, write, and free with per-page
// checksum corruption detection and per-write fsync durability.
//
// This is the Basic tier — single-page writes are durable and checksummed,
// but multi-page operations are NOT crash-atomic. See docs/storage.md for
// the full contract.
package pager

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

// Constants
const (
	PageSize           = 4096 // bytes; matches common OS page size
	HeaderPageID       = 0
	MirrorHeaderPageID = 1          // Page 1 is permanently reserved for the mirrored header
	FirstDataPageID    = 2          // First page available for AllocatePage
	FormatVersion      = 2          // Enterprise layout (v1 files rejected with ErrUnsupportedVersion)
	MagicNumber        = 0x506C6D76 // "Plmv" as uint32, big-endian
)

// Sentinel errors.
var (
	ErrNotOpen            = errors.New("pager: not open")
	ErrAlreadyOpen        = errors.New("pager: already open")
	ErrClosed             = errors.New("pager: closed")
	ErrNotAPagerFile      = errors.New("pager: file is not a pager file (bad magic)")
	ErrUnsupportedVersion = errors.New("pager: unsupported format version")
	ErrPageSizeMismatch   = errors.New("pager: file page size does not match compiled PageSize")
	ErrHeaderCorrupt      = errors.New("pager: header checksum mismatch")
	ErrPageCorrupt        = errors.New("pager: page checksum mismatch")
	ErrInvalidPageID      = errors.New("pager: invalid page ID")
	ErrBodySizeMismatch   = errors.New("pager: body size does not match page body size")
	ErrAlreadyFree        = errors.New("pager: page is already free")
	ErrFreeListCorrupt    = errors.New("pager: free-list is corrupt (cycle, out-of-range, or invalid pointer)")
	ErrTxAlreadyActive    = errors.New("pager: transaction already active")
	ErrNoActiveTx         = errors.New("pager: no active transaction")
	ErrTxUnsupportedOp    = errors.New("pager: operation not supported inside transaction")
	ErrWALCorrupt         = errors.New("pager: WAL record checksum mismatch or malformed record")
)

// DataPageBodySize is the usable body size within each data page.
const DataPageBodySize = PageSize - 12

const freeListSentinel uint64 = 0xFFFFFFFFFFFFFFFF

// fileOps abstracts all file I/O so tests can inject fault behaviour.
type fileOps interface {
	ReadAt(p []byte, off int64) (n int, err error)
	WriteAt(p []byte, off int64) (n int, err error)
	Sync() error
	Close() error
	Stat() (os.FileInfo, error)
	Truncate(size int64) error
}

// realFileOps wraps an *os.File to implement fileOps.
type realFileOps struct {
	f *os.File
}

func (r realFileOps) ReadAt(p []byte, off int64) (n int, err error)  { return r.f.ReadAt(p, off) }
func (r realFileOps) WriteAt(p []byte, off int64) (n int, err error) { return r.f.WriteAt(p, off) }
func (r realFileOps) Sync() error                                    { return r.f.Sync() }
func (r realFileOps) Close() error                                   { return r.f.Close() }
func (r realFileOps) Stat() (os.FileInfo, error)                     { return r.f.Stat() }
func (r realFileOps) Truncate(size int64) error                      { return r.f.Truncate(size) }

// Options configures Enterprise pager behaviour.
type Options struct {
	WALPath             string // Path to WAL file. If empty, defaults to path + ".wal"
	CheckpointThreshold int64  // Trigger checkpoint when WAL exceeds this size (bytes). 0 = checkpoint on every CommitTx.
}

// Pager manages fixed-size pages in a single backing file.
type Pager interface {
	// Name identifies the pager for lifecycle/logging.
	Name() string

	// Open opens (or creates, if absent) the backing file at the configured
	// path, validates or initializes the header page, and prepares the
	// pager for use. Idempotent error if already open.
	Open(ctx context.Context) error

	// Close flushes and closes the backing file. Safe to call once after
	// Open.
	Close(ctx context.Context) error

	// AllocatePage returns the page ID of a newly allocated page, taken
	// from the free-list if non-empty, otherwise extending the file by one
	// page. The returned page's body is zero-filled. The allocation is
	// durable once AllocatePage returns nil error.
	AllocatePage(ctx context.Context) (pageID uint64, err error)

	// ReadPage returns a COPY of the body of the page identified by
	// pageID. Returns ErrPageCorrupt if the page's checksum does not match
	// its body. Returns ErrInvalidPageID if pageID is 0 or >= page count.
	ReadPage(ctx context.Context, pageID uint64) (body []byte, err error)

	// WritePage writes body as the new content of the page identified by
	// pageID. len(body) must equal DataPageBodySize; ErrBodySizeMismatch
	// otherwise. The write is durable (fsync'd) once WritePage returns nil.
	WritePage(ctx context.Context, pageID uint64, body []byte) error

	// FreePage returns pageID to the free-list, making it available for a
	// future AllocatePage call. Freeing an already-free page is
	// ErrAlreadyFree. Freeing the header page (0) is ErrInvalidPageID.
	FreePage(ctx context.Context, pageID uint64) error

	// PageCount returns the current total number of pages in the file,
	// including the header page.
	PageCount(ctx context.Context) (uint64, error)

	// BeginTx starts a new transaction. All subsequent WritePage calls
	// are buffered in the WAL and in-memory until CommitTx or RollbackTx.
	// Returns ErrTxAlreadyActive if a transaction is already active.
	BeginTx(ctx context.Context) error

	// CommitTx writes the EOT marker to the WAL, fsyncs the WAL, applies
	// all buffered page writes to the main file, fsyncs the main file,
	// and clears the transaction state.
	// Returns ErrNoActiveTx if no transaction is active.
	CommitTx(ctx context.Context) error

	// RollbackTx discards the in-memory transaction buffer and resets
	// transaction state. WAL records already appended for this transaction
	// remain on disk but are ignored during replay because they lack an
	// EOT marker.
	// Returns ErrNoActiveTx if no transaction is active.
	RollbackTx(ctx context.Context) error
}

type storeState int

const (
	stateNeverOpened storeState = iota
	stateOpen
	stateClosed
)

// filePager implements Pager backed by an on-disk file and optional WAL.
type filePager struct {
	mu           sync.RWMutex
	path         string
	opts         Options
	mainFileOps  fileOps
	walFileOps   fileOps
	state        storeState
	openErr      error
	pageCount    uint64
	freeListHead uint64
	freeCount    uint64
	freeSet      map[uint64]struct{}

	// Transaction state (Enterprise)
	inTxn      bool
	nextTxnID  uint64
	currentTxn uint64
	txnBuffer  map[uint64][]byte
	walSize    int64
}

// New creates a Pager backed by the file at the given path with default options.
func New(path string) Pager {
	return NewWithOptions(path, Options{})
}

// NewWithOptions creates a Pager with the given Options.
func NewWithOptions(path string, opts Options) Pager {
	p, err := newPager(path, opts, nil, nil)
	if err != nil {
		return &filePager{openErr: err}
	}
	return p
}

// newPager is the internal constructor that allows dependency injection of
// fileOps for testing. If mainFileOps is nil the real file is opened.
func newPager(path string, opts Options, mainFileOps, walFileOps fileOps) (*filePager, error) {
	p := &filePager{
		path:         path,
		opts:         opts,
		state:        stateNeverOpened,
		freeListHead: freeListSentinel,
		freeSet:      make(map[uint64]struct{}),
	}

	if mainFileOps != nil {
		p.mainFileOps = mainFileOps
	} else {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			return nil, fmt.Errorf("pager: open main file: %w", err)
		}
		p.mainFileOps = realFileOps{f}
	}

	if walFileOps != nil {
		p.walFileOps = walFileOps
	} else {
		walPath := opts.WALPath
		if walPath == "" {
			walPath = path + ".wal"
		}
		f, err := os.OpenFile(walPath, os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			// Close the main file we already opened.
			p.mainFileOps.Close()
			return nil, fmt.Errorf("pager: open WAL file: %w", err)
		}
		p.walFileOps = realFileOps{f}
	}

	return p, nil
}

// Name returns the pager's path for identification.
func (p *filePager) Name() string { return p.path }

// checkState is a shared helper that returns the appropriate error if the
// pager is not in stateOpen. Caller must hold at least a read lock.
func (p *filePager) checkState() error {
	switch p.state {
	case stateOpen:
		return nil
	case stateNeverOpened:
		return ErrNotOpen
	case stateClosed:
		return ErrClosed
	default:
		return ErrNotOpen
	}
}

// -- Stub implementations for Task 1 (will be filled in by later tasks) --

func (p *filePager) Open(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.openErr != nil {
		return p.openErr
	}
	if p.state == stateOpen {
		return ErrAlreadyOpen
	}
	if p.state != stateNeverOpened {
		return ErrClosed
	}

	// Stat the already-opened main file to see if it is empty (fresh).
	info, err := p.mainFileOps.Stat()
	if err != nil {
		return fmt.Errorf("pager: stat main file: %w", err)
	}

	if info.Size() == 0 {
		// Fresh file — write initial headers to both page 0 and page 1.
		// Enterprise always starts with pageCount=2 (primary + mirror header).
		if err := p.writeHeader(2, freeListSentinel); err != nil {
			return err
		}

		p.state = stateOpen
		p.pageCount = 2
		p.freeListHead = freeListSentinel
		p.freeCount = 0
		// freeSet is already empty from newPager
		return nil
	}

	// Existing file — read and validate header.
	// Try primary (page 0) first; if it fails, try mirror (page 1).
	headerBuf := make([]byte, PageSize)
	var h pagerHeader
	var headerOK bool

	if _, err := p.mainFileOps.ReadAt(headerBuf, int64(HeaderPageID)*PageSize); err == nil {
		if h, err = decodeHeader(headerBuf); err == nil {
			headerOK = true
		}
	}

	if !headerOK {
		// Primary header failed — attempt recovery from mirror (page 1).
		if _, err := p.mainFileOps.ReadAt(headerBuf, int64(MirrorHeaderPageID)*PageSize); err != nil {
			return fmt.Errorf("pager: read mirror header: %w", err)
		}
		if h, err = decodeHeader(headerBuf); err != nil {
			return fmt.Errorf("pager: mirror header also invalid: %w", err)
		}
		// Recovery succeeded. Write the recovered header back to page 0
		// so the file is self-healing.
		if _, werr := p.mainFileOps.WriteAt(headerBuf, int64(HeaderPageID)*PageSize); werr == nil {
			p.mainFileOps.Sync() // best-effort, not critical
		}
	}

	// Walk the free-list to populate freeSet
	freeSet := make(map[uint64]struct{})
	if h.freeListHead != freeListSentinel {
		walkSeen := make(map[uint64]struct{})
		curr := h.freeListHead
		for {
			if curr == freeListSentinel {
				break
			}
			if curr == 0 || curr >= h.pageCount {
				return fmt.Errorf("pager: free-list %w: page %d out of range [0, %d)", ErrFreeListCorrupt, curr, h.pageCount)
			}
			if _, seen := walkSeen[curr]; seen {
				return fmt.Errorf("pager: free-list %w: cycle detected at page %d", ErrFreeListCorrupt, curr)
			}
			walkSeen[curr] = struct{}{}

			body, err := readBodyUnchecked(p.mainFileOps, curr)
			if err != nil {
				return fmt.Errorf("pager: free-list walk page %d: %w", curr, err)
			}
			next, decErr := decodeFreeListPointer(body)
			if decErr != nil {
				return fmt.Errorf("pager: free-list walk page %d: %w", curr, decErr)
			}

			freeSet[curr] = struct{}{}
			curr = next
		}
	}

	p.state = stateOpen
	p.pageCount = h.pageCount
	p.freeListHead = h.freeListHead
	p.freeCount = h.freePageCount
	p.freeSet = freeSet

	// Validate that the on-disk free-page count matches the walk result.
	if uint64(len(freeSet)) != p.freeCount {
		return fmt.Errorf("pager: free-list %w: header says %d free pages, walk found %d",
			ErrFreeListCorrupt, p.freeCount, len(freeSet))
	}

	// Initialize WAL size from the already-opened WAL file.
	walInfo, err := p.walFileOps.Stat()
	if err != nil {
		return fmt.Errorf("pager: stat WAL file: %w", err)
	}
	p.walSize = walInfo.Size()

	// Replay WAL if it contains data.
	if err := p.replayWAL(); err != nil {
		return err
	}

	return nil
}

// readBodyUnchecked reads and decodes a data page's body by pageID, bypassing
// the freeSet check. Used internally by free-list walk and AllocatePage.
// Takes an explicit fileOps to avoid requiring the caller to hold the lock.
func readBodyUnchecked(f fileOps, pageID uint64) ([]byte, error) {
	buf := make([]byte, PageSize)
	offset := int64(pageID) * PageSize
	if _, err := f.ReadAt(buf, offset); err != nil {
		return nil, fmt.Errorf("pager: read page %d: %w", pageID, err)
	}
	return decodeDataPage(buf)
}

// writeBodyUnchecked encodes and writes a data page body to disk, bypassing
// the freeSet check. The body MUST be DataPageBodySize bytes. The write is
// fsync'd before returning.
func writeBodyUnchecked(f fileOps, pageID uint64, body []byte) error {
	encoded, err := encodeDataPage(body)
	if err != nil {
		return err
	}
	offset := int64(pageID) * PageSize
	n, err := f.WriteAt(encoded, offset)
	if err != nil {
		return fmt.Errorf("pager: write page %d: %w", pageID, err)
	}
	if n != len(encoded) {
		return io.ErrShortWrite
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("pager: fsync page %d: %w", pageID, err)
	}
	return nil
}

// replayWAL reads the WAL file, applies all completed transactions to the main
// file, and truncates the WAL. If a complete record is corrupt, returns
// ErrWALCorrupt without truncating the WAL. Caller must hold the write lock.
func (p *filePager) replayWAL() error {
	return p.replayWALInternal(false)
}

// checkpoint reads the WAL, applies all completed transactions, and truncates
// the WAL. Used during normal operation (CommitTx, Close). Caller must hold
// the write lock.
func (p *filePager) checkpoint() error {
	return p.replayWALInternal(true)
}

// replayWALInternal implements the shared logic for WAL replay and checkpoint.
// If isCheckpoint is true, missing WAL file is not an error (just return nil).
func (p *filePager) replayWALInternal(isCheckpoint bool) error {
	if p.walSize == 0 {
		return nil
	}

	// Read the entire WAL content.
	walData := make([]byte, p.walSize)
	if _, err := p.walFileOps.ReadAt(walData, 0); err != nil {
		return fmt.Errorf("pager: read WAL for replay: %w", err)
	}

	// Parse all records.
	type walRecord struct {
		txnID  uint64
		pageID uint64
		body   []byte
	}
	var records []walRecord

	offset := 0
	for offset < len(walData) {
		txnID, pageID, body, consumed, err := decodeNextWALRecord(walData[offset:])
		if err == io.EOF {
			// Trailing incomplete record — stop parsing.
			break
		}
		if err != nil {
			// Corrupted complete record — fatal, do NOT truncate WAL.
			return err
		}
		records = append(records, walRecord{txnID, pageID, body})
		offset += consumed
	}

	if len(records) == 0 {
		// No complete records — truncate empty/garbage WAL.
		if err := p.walFileOps.Truncate(0); err != nil {
			return fmt.Errorf("pager: truncate WAL: %w", err)
		}
		p.walSize = 0
		return nil
	}

	// Group records by TxnID, preserving order.
	type txnGroup struct {
		txnID   uint64
		records []walRecord
	}
	var groups []txnGroup
	groupIdx := make(map[uint64]int) // txnID -> index in groups

	for _, rec := range records {
		if idx, exists := groupIdx[rec.txnID]; exists {
			groups[idx].records = append(groups[idx].records, rec)
		} else {
			idx = len(groups)
			groupIdx[rec.txnID] = idx
			groups = append(groups, txnGroup{txnID: rec.txnID, records: []walRecord{rec}})
		}
	}

	// Apply each completed transaction.
	for _, group := range groups {
		// Find if there's an EOT marker in this group.
		hasEOT := false
		for _, rec := range group.records {
			if isEOTMarker(rec.pageID, 0) {
				hasEOT = true
				break
			}
		}
		if !hasEOT {
			// Incomplete transaction — skip.
			continue
		}

		// Apply data records. Last write wins within the transaction.
		applied := make(map[uint64][]byte)
		for _, rec := range group.records {
			if isEOTMarker(rec.pageID, 0) {
				continue
			}
			applied[rec.pageID] = rec.body
		}

		for pageID, body := range applied {
			encoded, encErr := encodeDataPage(body)
			if encErr != nil {
				return encErr
			}
			offset := int64(pageID) * PageSize
			n, werr := p.mainFileOps.WriteAt(encoded, offset)
			if werr != nil {
				return fmt.Errorf("pager: replay write page %d: %w", pageID, werr)
			}
			if n != len(encoded) {
				return io.ErrShortWrite
			}
		}

		if err := p.mainFileOps.Sync(); err != nil {
			return fmt.Errorf("pager: replay fsync main file: %w", err)
		}
	}

	// All completed transactions applied. Truncate WAL.
	if err := p.walFileOps.Truncate(0); err != nil {
		return fmt.Errorf("pager: truncate WAL: %w", err)
	}
	p.walSize = 0

	return nil
}

// -- Transaction methods (Enterprise) --

func (p *filePager) BeginTx(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.checkState(); err != nil {
		return err
	}
	if p.inTxn {
		return ErrTxAlreadyActive
	}
	p.inTxn = true
	p.nextTxnID++
	p.currentTxn = p.nextTxnID
	p.txnBuffer = make(map[uint64][]byte)
	return nil
}

func (p *filePager) CommitTx(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.checkState(); err != nil {
		return err
	}
	if !p.inTxn {
		return ErrNoActiveTx
	}

	// Step 1: Encode EOT marker.
	eot := encodeEOTMarker(p.currentTxn)

	// Step 2: Append EOT to WAL file and fsync.
	n, err := p.walFileOps.WriteAt(eot, p.walSize)
	if err != nil {
		return fmt.Errorf("pager: write EOT to WAL: %w", err)
	}
	if n != len(eot) {
		return io.ErrShortWrite
	}
	if err := p.walFileOps.Sync(); err != nil {
		return fmt.Errorf("pager: fsync WAL after EOT: %w", err)
	}

	// EOT is durably on disk. Clear transaction state immediately.
	p.walSize += int64(len(eot))
	applyBuffer := p.txnBuffer
	p.inTxn = false
	p.txnBuffer = nil
	p.currentTxn = 0

	// Step 3: Apply buffered pages to the main file.
	for pageID, body := range applyBuffer {
		encoded, encErr := encodeDataPage(body)
		if encErr != nil {
			return encErr
		}
		offset := int64(pageID) * PageSize
		nw, werr := p.mainFileOps.WriteAt(encoded, offset)
		if werr != nil {
			return fmt.Errorf("pager: commit write page %d: %w", pageID, werr)
		}
		if nw != len(encoded) {
			return io.ErrShortWrite
		}
	}

	// Step 4: Fsync main file ONCE.
	if err := p.mainFileOps.Sync(); err != nil {
		return fmt.Errorf("pager: commit fsync main file: %w", err)
	}

	// Step 5: Trigger checkpoint if threshold exceeded.
	if p.opts.CheckpointThreshold == 0 || p.walSize >= p.opts.CheckpointThreshold {
		if err := p.checkpoint(); err != nil {
			return fmt.Errorf("pager: commit checkpoint: %w", err)
		}
	}

	return nil
}

func (p *filePager) RollbackTx(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.checkState(); err != nil {
		return err
	}
	if !p.inTxn {
		return ErrNoActiveTx
	}
	p.inTxn = false
	p.txnBuffer = nil
	p.currentTxn = 0
	return nil
}

func (p *filePager) Close(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.checkState(); err != nil {
		return err
	}
	// Checkpoint any remaining WAL data before closing.
	if err := p.checkpoint(); err != nil {
		return fmt.Errorf("pager: close checkpoint: %w", err)
	}
	if err := p.mainFileOps.Sync(); err != nil {
		return fmt.Errorf("pager: close fsync: %w", err)
	}
	if err := p.mainFileOps.Close(); err != nil {
		return fmt.Errorf("pager: close main file: %w", err)
	}
	if err := p.walFileOps.Close(); err != nil {
		return fmt.Errorf("pager: close WAL file: %w", err)
	}
	p.state = stateClosed
	p.mainFileOps = nil
	p.walFileOps = nil
	return nil
}

func (p *filePager) AllocatePage(ctx context.Context) (uint64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.checkState(); err != nil {
		return 0, err
	}

	if p.inTxn {
		return 0, ErrTxUnsupportedOp
	}

	if p.freeListHead != freeListSentinel {
		// Reuse a page from the free-list.
		allocID := p.freeListHead

		// Read the current free-list head page to find the next free page.
		body, err := readBodyUnchecked(p.mainFileOps, allocID)
		if err != nil {
			return 0, err
		}
		nextHead, decErr := decodeFreeListPointer(body)
		if decErr != nil {
			return 0, decErr
		}

		// Zero-fill the allocated page's body and write it.
		zeroBody := make([]byte, DataPageBodySize)
		if err := writeBodyUnchecked(p.mainFileOps, allocID, zeroBody); err != nil {
			return 0, err
		}

		// Save old state for rollback.
		oldHead := p.freeListHead
		oldCount := p.freeCount

		// Update in-memory state BEFORE writing header.
		delete(p.freeSet, allocID)
		p.freeListHead = nextHead
		if p.freeCount > 0 {
			p.freeCount--
		}

		// Write header with the updated free-list head and free-page count.
		if err := p.writeHeader(p.pageCount, nextHead); err != nil {
			// Roll back in-memory changes on failure.
			p.freeSet[allocID] = struct{}{}
			p.freeListHead = oldHead
			p.freeCount = oldCount
			return 0, err
		}

		return allocID, nil
	}

	// Free-list empty — extend the file by one page.
	newID := p.pageCount
	zeroBody := make([]byte, DataPageBodySize)
	if err := writeBodyUnchecked(p.mainFileOps, newID, zeroBody); err != nil {
		return 0, err
	}

	// Update in-memory state BEFORE writing header.
	oldPageCount := p.pageCount
	p.pageCount++

	// Update header with the new page count.
	if err := p.writeHeader(p.pageCount, p.freeListHead); err != nil {
		// Roll back in-memory change on failure.
		p.pageCount = oldPageCount
		return 0, err
	}

	return newID, nil
}

// writeHeader writes the header page to disk with mirror redundancy.
// Write order: page 1 first, fsync, page 0 second, fsync.
// If either write or fsync fails, the in-memory metadata is NOT updated.
// Caller must hold the write lock.
func (p *filePager) writeHeader(pageCount uint64, freeListHead uint64) error {
	header := encodeHeader(pagerHeader{
		magic:         MagicNumber,
		version:       FormatVersion,
		pageSize:      PageSize,
		pageCount:     pageCount,
		freeListHead:  freeListHead,
		freePageCount: p.freeCount,
	})

	// Step 1: Write to page 1 (mirror).
	n, err := p.mainFileOps.WriteAt(header, int64(MirrorHeaderPageID)*PageSize)
	if err != nil {
		return fmt.Errorf("pager: write mirror header: %w", err)
	}
	if n != len(header) {
		return io.ErrShortWrite
	}

	// Step 2: Fsync the main file.
	if err := p.mainFileOps.Sync(); err != nil {
		return fmt.Errorf("pager: fsync mirror header: %w", err)
	}

	// Step 3: Write to page 0 (primary).
	n, err = p.mainFileOps.WriteAt(header, int64(HeaderPageID)*PageSize)
	if err != nil {
		return fmt.Errorf("pager: write primary header: %w", err)
	}
	if n != len(header) {
		return io.ErrShortWrite
	}

	// Step 4: Fsync the main file.
	if err := p.mainFileOps.Sync(); err != nil {
		return fmt.Errorf("pager: fsync primary header: %w", err)
	}

	return nil
}

func (p *filePager) ReadPage(ctx context.Context, pageID uint64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if err := p.checkState(); err != nil {
		return nil, err
	}
	if err := p.validatePageID(pageID); err != nil {
		return nil, err
	}
	if _, free := p.freeSet[pageID]; free {
		return nil, ErrInvalidPageID
	}

	// Inside a transaction, serve the buffered version (read-your-own-writes).
	if p.inTxn {
		if buf, ok := p.txnBuffer[pageID]; ok {
			cp := make([]byte, DataPageBodySize)
			copy(cp, buf)
			return cp, nil
		}
	}

	return readBodyUnchecked(p.mainFileOps, pageID)
}

func (p *filePager) WritePage(ctx context.Context, pageID uint64, body []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.checkState(); err != nil {
		return err
	}
	if err := p.validatePageID(pageID); err != nil {
		return err
	}
	if _, free := p.freeSet[pageID]; free {
		return ErrInvalidPageID
	}
	if len(body) != DataPageBodySize {
		return ErrBodySizeMismatch
	}

	if !p.inTxn {
		// Outside transaction: behave exactly as Basic — direct write + fsync.
		return writeBodyUnchecked(p.mainFileOps, pageID, body)
	}

	// Inside transaction: append to WAL, do NOT touch the main file.
	return p.appendWAL(pageID, body)
}

// appendWAL appends a WAL record for the given pageID and body, fsyncs the
// WAL, and buffers the body in txnBuffer. Caller must hold the write lock.
func (p *filePager) appendWAL(pageID uint64, body []byte) error {
	rec := encodeWALRecord(p.currentTxn, pageID, body)
	n, err := p.walFileOps.WriteAt(rec, p.walSize)
	if err != nil {
		return fmt.Errorf("pager: append WAL: %w", err)
	}
	if n != len(rec) {
		return io.ErrShortWrite
	}
	if err := p.walFileOps.Sync(); err != nil {
		return fmt.Errorf("pager: fsync WAL: %w", err)
	}
	// Buffer a COPY of the body for read-your-own-writes and commit.
	cp := make([]byte, DataPageBodySize)
	copy(cp, body)
	p.txnBuffer[pageID] = cp
	p.walSize += int64(len(rec))
	return nil
}

// validatePageID returns ErrInvalidPageID if pageID is 0, 1 (mirror header),
// or >= pageCount. Caller must hold at least a read lock.
func (p *filePager) validatePageID(pageID uint64) error {
	if pageID == 0 || pageID == MirrorHeaderPageID {
		return ErrInvalidPageID
	}
	if pageID >= p.pageCount {
		return ErrInvalidPageID
	}
	return nil
}

func (p *filePager) FreePage(ctx context.Context, pageID uint64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.checkState(); err != nil {
		return err
	}
	if p.inTxn {
		return ErrTxUnsupportedOp
	}
	if pageID == 0 {
		return ErrInvalidPageID
	}
	if pageID >= p.pageCount {
		return ErrInvalidPageID
	}
	if _, already := p.freeSet[pageID]; already {
		return ErrAlreadyFree
	}

	// Save old state for rollback.
	oldHead := p.freeListHead
	oldCount := p.freeCount

	// Encode the free-list pointer pointing at the current head.
	body := encodeFreeListPointer(p.freeListHead)
	if err := writeBodyUnchecked(p.mainFileOps, pageID, body); err != nil {
		return err
	}

	// Update in-memory state BEFORE writing the header so the on-disk
	// free-page count matches the actual free-list.
	p.freeSet[pageID] = struct{}{}
	p.freeListHead = pageID
	p.freeCount++

	// Update header with the new free-list head.
	if err := p.writeHeader(p.pageCount, pageID); err != nil {
		// Roll back in-memory changes on failure.
		delete(p.freeSet, pageID)
		p.freeListHead = oldHead
		p.freeCount = oldCount
		return err
	}

	return nil
}

func (p *filePager) PageCount(ctx context.Context) (uint64, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if err := p.checkState(); err != nil {
		return 0, err
	}
	return p.pageCount, nil
}

// compile-time interface check
var _ Pager = (*filePager)(nil)
