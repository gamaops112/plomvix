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
	"os"
	"sync"
)

// Constants
const (
	PageSize      = 4096 // bytes; matches common OS page size
	HeaderPageID  = 0
	FormatVersion = 1
	MagicNumber   = 0x506C6D76 // "Plmv" as uint32, big-endian
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
)

// DataPageBodySize is the usable body size within each data page.
const DataPageBodySize = PageSize - 12

const freeListSentinel uint64 = 0xFFFFFFFFFFFFFFFF

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
}

type storeState int

const (
	stateNeverOpened storeState = iota
	stateOpen
	stateClosed
)

// filePager implements Pager backed by an *os.File.
type filePager struct {
	mu           sync.RWMutex
	path         string
	file         *os.File
	state        storeState
	pageCount    uint64
	freeListHead uint64
	freeSet      map[uint64]struct{}
}

// New creates a Pager backed by the file at the given path.
func New(path string) Pager {
	return &filePager{
		path:         path,
		state:        stateNeverOpened,
		freeListHead: freeListSentinel,
		freeSet:      make(map[uint64]struct{}),
	}
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

	if p.state == stateOpen {
		return ErrAlreadyOpen
	}
	if p.state != stateNeverOpened {
		return ErrClosed
	}

	// Check if the file exists.
	_, statErr := os.Stat(p.path)
	exists := statErr == nil

	if !exists {
		// Create file and write initial header.
		f, err := os.OpenFile(p.path, os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			return fmt.Errorf("pager: create file: %w", err)
		}

		header := encodeHeader(pagerHeader{
			magic:        MagicNumber,
			version:      FormatVersion,
			pageSize:     PageSize,
			pageCount:    1, // just the header page
			freeListHead: freeListSentinel,
		})

		if _, err := f.WriteAt(header, 0); err != nil {
			f.Close() // best-effort
			return fmt.Errorf("pager: write initial header: %w", err)
		}
		if err := f.Sync(); err != nil {
			f.Close() // best-effort
			return fmt.Errorf("pager: fsync initial header: %w", err)
		}

		p.file = f
		p.state = stateOpen
		p.pageCount = 1
		p.freeListHead = freeListSentinel
		// freeSet is already empty from New()
		return nil
	}

	// File exists — open and validate.
	f, err := os.OpenFile(p.path, os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("pager: open file: %w", err)
	}

	// Read header page
	headerBuf := make([]byte, PageSize)
	if _, err := f.ReadAt(headerBuf, 0); err != nil {
		f.Close()
		return fmt.Errorf("pager: read header: %w", err)
	}

	h, err := decodeHeader(headerBuf)
	if err != nil {
		f.Close()
		return fmt.Errorf("pager: invalid header: %w", err)
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
				f.Close()
				return fmt.Errorf("pager: free-list %w: page %d out of range [0, %d)", ErrFreeListCorrupt, curr, h.pageCount)
			}
			if _, seen := walkSeen[curr]; seen {
				f.Close()
				return fmt.Errorf("pager: free-list %w: cycle detected at page %d", ErrFreeListCorrupt, curr)
			}
			walkSeen[curr] = struct{}{}

			body, err := readBodyUnchecked(f, curr)
			if err != nil {
				f.Close()
				return fmt.Errorf("pager: free-list walk page %d: %w", curr, err)
			}
			next, decErr := decodeFreeListPointer(body)
			if decErr != nil {
				f.Close()
				return fmt.Errorf("pager: free-list walk page %d: %w", curr, decErr)
			}

			freeSet[curr] = struct{}{}
			curr = next
		}
	}

	p.file = f
	p.state = stateOpen
	p.pageCount = h.pageCount
	p.freeListHead = h.freeListHead
	p.freeSet = freeSet
	return nil
}

// readBodyUnchecked reads and decodes a data page's body by pageID, bypassing
// the freeSet check. Used internally by free-list walk and AllocatePage.
// Takes an explicit *os.File to avoid requiring the caller to hold the lock.
func readBodyUnchecked(f *os.File, pageID uint64) ([]byte, error) {
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
func writeBodyUnchecked(f *os.File, pageID uint64, body []byte) error {
	encoded, err := encodeDataPage(body)
	if err != nil {
		return err
	}
	offset := int64(pageID) * PageSize
	if _, err := f.WriteAt(encoded, offset); err != nil {
		return fmt.Errorf("pager: write page %d: %w", pageID, err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("pager: fsync page %d: %w", pageID, err)
	}
	return nil
}

func (p *filePager) Close(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.checkState(); err != nil {
		return err
	}
	if err := p.file.Sync(); err != nil {
		return fmt.Errorf("pager: close fsync: %w", err)
	}
	if err := p.file.Close(); err != nil {
		return fmt.Errorf("pager: close file: %w", err)
	}
	p.state = stateClosed
	p.file = nil
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

	if p.freeListHead != freeListSentinel {
		// Reuse a page from the free-list.
		allocID := p.freeListHead

		// Read the current free-list head page to find the next free page.
		body, err := readBodyUnchecked(p.file, allocID)
		if err != nil {
			return 0, err
		}
		nextHead, decErr := decodeFreeListPointer(body)
		if decErr != nil {
			return 0, decErr
		}

		// Zero-fill the allocated page's body and write it.
		zeroBody := make([]byte, DataPageBodySize)
		if err := writeBodyUnchecked(p.file, allocID, zeroBody); err != nil {
			return 0, err
		}

		// Update header with the new free-list head.
		if err := p.writeHeader(p.pageCount, nextHead); err != nil {
			return 0, err
		}

		// Commit in-memory changes.
		delete(p.freeSet, allocID)
		p.freeListHead = nextHead
		return allocID, nil
	}

	// Free-list empty — extend the file by one page.
	newID := p.pageCount
	zeroBody := make([]byte, DataPageBodySize)
	if err := writeBodyUnchecked(p.file, newID, zeroBody); err != nil {
		return 0, err
	}

	// Update header with the new page count.
	if err := p.writeHeader(p.pageCount+1, p.freeListHead); err != nil {
		return 0, err
	}

	// Commit in-memory changes.
	p.pageCount++
	return newID, nil
}

// writeHeader writes and fsyncs the header page with the given pageCount and
// freeListHead. Caller must hold the write lock.
func (p *filePager) writeHeader(pageCount uint64, freeListHead uint64) error {
	header := encodeHeader(pagerHeader{
		magic:        MagicNumber,
		version:      FormatVersion,
		pageSize:     PageSize,
		pageCount:    pageCount,
		freeListHead: freeListHead,
	})
	if _, err := p.file.WriteAt(header, 0); err != nil {
		return fmt.Errorf("pager: write header: %w", err)
	}
	if err := p.file.Sync(); err != nil {
		return fmt.Errorf("pager: fsync header: %w", err)
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
	return readBodyUnchecked(p.file, pageID)
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
	return writeBodyUnchecked(p.file, pageID, body)
}

// validatePageID returns ErrInvalidPageID if pageID is 0 or >= pageCount.
// Caller must hold at least a read lock.
func (p *filePager) validatePageID(pageID uint64) error {
	if pageID == 0 {
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
	if pageID == 0 {
		return ErrInvalidPageID
	}
	if pageID >= p.pageCount {
		return ErrInvalidPageID
	}
	if _, already := p.freeSet[pageID]; already {
		return ErrAlreadyFree
	}

	// Encode the free-list pointer pointing at the current head.
	body := encodeFreeListPointer(p.freeListHead)
	if err := writeBodyUnchecked(p.file, pageID, body); err != nil {
		return err
	}

	// Update header with the new free-list head.
	if err := p.writeHeader(p.pageCount, pageID); err != nil {
		return err
	}

	// Commit in-memory changes.
	p.freeSet[pageID] = struct{}{}
	p.freeListHead = pageID
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
