package pager

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"testing"
)

// contains reports whether sub is a substring of s.
func contains(s, sub string) bool {
	if len(s) < len(sub) {
		return false
	}
	if len(s) == len(sub) {
		return s == sub
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestPager_New(t *testing.T) {
	p := New("/tmp/test_pager.db")
	if p == nil {
		t.Fatal("New returned nil")
	}
	if p.Name() != "/tmp/test_pager.db" {
		t.Errorf("Name() = %q, want %q", p.Name(), "/tmp/test_pager.db")
	}
}

func TestPager_StateMachine(t *testing.T) {
	p := New("/tmp/test_state.db").(*filePager)

	// Before Open, all data methods return ErrNotOpen
	ctx := context.Background()
	if _, err := p.ReadPage(ctx, 1); err != ErrNotOpen {
		t.Errorf("ReadPage before Open: got %v, want ErrNotOpen", err)
	}
	if err := p.WritePage(ctx, 1, nil); err != ErrNotOpen {
		t.Errorf("WritePage before Open: got %v, want ErrNotOpen", err)
	}
	if _, err := p.AllocatePage(ctx); err != ErrNotOpen {
		t.Errorf("AllocatePage before Open: got %v, want ErrNotOpen", err)
	}
	if err := p.FreePage(ctx, 1); err != ErrNotOpen {
		t.Errorf("FreePage before Open: got %v, want ErrNotOpen", err)
	}
	if _, err := p.PageCount(ctx); err != ErrNotOpen {
		t.Errorf("PageCount before Open: got %v, want ErrNotOpen", err)
	}
	// Close before Open returns ErrNotOpen
	if err := p.Close(ctx); err != ErrNotOpen {
		t.Errorf("Close before Open: got %v, want ErrNotOpen", err)
	}
}

// -- Header encode/decode tests (Task 2) --

func TestEncodeDecodeHeader_RoundTrip(t *testing.T) {
	h := pagerHeader{
		magic:         MagicNumber,
		version:       FormatVersion,
		pageSize:      PageSize,
		pageCount:     42,
		freeListHead:  freeListSentinel,
	}
	data := encodeHeader(h)
	if len(data) != PageSize {
		t.Fatalf("encodeHeader returned %d bytes, want %d", len(data), PageSize)
	}

	got, err := decodeHeader(data)
	if err != nil {
		t.Fatalf("decodeHeader: %v", err)
	}
	if got.magic != h.magic {
		t.Errorf("magic = %x, want %x", got.magic, h.magic)
	}
	if got.version != h.version {
		t.Errorf("version = %d, want %d", got.version, h.version)
	}
	if got.pageSize != h.pageSize {
		t.Errorf("pageSize = %d, want %d", got.pageSize, h.pageSize)
	}
	if got.pageCount != h.pageCount {
		t.Errorf("pageCount = %d, want %d", got.pageCount, h.pageCount)
	}
	if got.freeListHead != h.freeListHead {
		t.Errorf("freeListHead = %d, want %d", got.freeListHead, h.freeListHead)
	}
}

func TestDecodeHeader_WrongMagic(t *testing.T) {
	h := pagerHeader{
		magic:    0xDEADBEEF,
		version:  FormatVersion,
		pageSize: PageSize,
	}
	data := encodeHeader(h)
	_, err := decodeHeader(data)
	if err != ErrNotAPagerFile {
		t.Errorf("got %v, want ErrNotAPagerFile", err)
	}
}

func TestDecodeHeader_WrongVersion(t *testing.T) {
	h := pagerHeader{
		magic:    MagicNumber,
		version:  99,
		pageSize: PageSize,
	}
	data := encodeHeader(h)
	_, err := decodeHeader(data)
	if err != ErrUnsupportedVersion {
		t.Errorf("got %v, want ErrUnsupportedVersion", err)
	}
}

func TestDecodeHeader_WrongPageSize(t *testing.T) {
	h := pagerHeader{
		magic:    MagicNumber,
		version:  FormatVersion,
		pageSize: 8192,
	}
	data := encodeHeader(h)
	_, err := decodeHeader(data)
	if err != ErrPageSizeMismatch {
		t.Errorf("got %v, want ErrPageSizeMismatch", err)
	}
}

func TestDecodeHeader_CorruptedChecksum(t *testing.T) {
	h := pagerHeader{
		magic:    MagicNumber,
		version:  FormatVersion,
		pageSize: PageSize,
	}
	data := encodeHeader(h)
	// Flip a bit in the checksum field itself
	data[28] ^= 0xFF
	_, err := decodeHeader(data)
	if err != ErrHeaderCorrupt {
		t.Errorf("got %v, want ErrHeaderCorrupt", err)
	}
}

func TestDecodeHeader_WrongLength(t *testing.T) {
	_, err := decodeHeader(make([]byte, PageSize-1))
	if err != ErrHeaderCorrupt {
		t.Errorf("got %v, want ErrHeaderCorrupt", err)
	}
}

// goldenHeaderBytes is a hand-written golden byte vector for a valid header
// with magic=Plmv, version=1, pageSize=4096, pageCount=1, freeListHead=sentinel.
// Computed by hand from the spec table, not by calling encodeHeader.
//
//	Offset  Size  Value          Field
//	0       4     0x506C6D76     Magic number ("Plmv")
//	4       4     0x00000001     Format version
//	8       4     0x00001000     Page size (4096)
//	12      8     0x0000000000000001  Page count
//	20      8     0xFFFFFFFFFFFFFFFF  Free-list head (sentinel)
//	28      4     0x________     CRC32 of [0,28) — computed below
//	32     4064   0x00...        Reserved, zero-filled
func TestGoldenHeaderVector(t *testing.T) {
	// Compute expected checksum of bytes [0,28)
	pre := []byte{
		0x50, 0x6C, 0x6D, 0x76, // magic "Plmv"
		0x00, 0x00, 0x00, 0x01, // version 1
		0x00, 0x00, 0x10, 0x00, // page size 4096
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, // page count 1
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // free-list sentinel
	}
	expectedCksum := crc32.ChecksumIEEE(pre)

	// Build the full 4096-byte header image by hand
	golden := make([]byte, PageSize)
	copy(golden[0:4], []byte{0x50, 0x6C, 0x6D, 0x76})
	copy(golden[4:8], []byte{0x00, 0x00, 0x00, 0x01})
	copy(golden[8:12], []byte{0x00, 0x00, 0x10, 0x00})
	copy(golden[12:20], []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01})
	copy(golden[20:28], []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})
	binary.BigEndian.PutUint32(golden[28:32], expectedCksum)
	// Bytes [32,4096) are already zero from make

	h, err := decodeHeader(golden)
	if err != nil {
		t.Fatalf("decodeHeader(golden) failed: %v", err)
	}
	if h.magic != MagicNumber {
		t.Errorf("magic = %x, want %x", h.magic, MagicNumber)
	}
	if h.version != FormatVersion {
		t.Errorf("version = %d, want %d", h.version, FormatVersion)
	}
	if h.pageSize != PageSize {
		t.Errorf("pageSize = %d, want %d", h.pageSize, PageSize)
	}
	if h.pageCount != 1 {
		t.Errorf("pageCount = %d, want 1", h.pageCount)
	}
	if h.freeListHead != freeListSentinel {
		t.Errorf("freeListHead = %d, want sentinel", h.freeListHead)
	}
}

// -- Data page encode/decode tests (Task 3) --

func TestEncodeDecodeDataPage_RoundTrip(t *testing.T) {
	body := make([]byte, DataPageBodySize)
	for i := range body {
		body[i] = byte(i % 256)
	}

	data, err := encodeDataPage(body)
	if err != nil {
		t.Fatalf("encodeDataPage: %v", err)
	}
	if len(data) != PageSize {
		t.Fatalf("encodeDataPage returned %d bytes, want %d", len(data), PageSize)
	}

	got, err := decodeDataPage(data)
	if err != nil {
		t.Fatalf("decodeDataPage: %v", err)
	}
	if len(got) != DataPageBodySize {
		t.Fatalf("decodeDataPage returned %d bytes, want %d", len(got), DataPageBodySize)
	}
	for i := range body {
		if got[i] != body[i] {
			t.Fatalf("byte %d: got %d, want %d", i, got[i], body[i])
		}
	}
}

func TestEncodeDataPage_WrongBodyLength(t *testing.T) {
	_, err := encodeDataPage(make([]byte, DataPageBodySize+1))
	if err != ErrBodySizeMismatch {
		t.Errorf("got %v, want ErrBodySizeMismatch", err)
	}
	_, err = encodeDataPage(make([]byte, DataPageBodySize-1))
	if err != ErrBodySizeMismatch {
		t.Errorf("got %v, want ErrBodySizeMismatch", err)
	}
}

func TestDecodeDataPage_WrongLength(t *testing.T) {
	_, err := decodeDataPage(make([]byte, PageSize-1))
	if err != ErrPageCorrupt {
		t.Errorf("got %v, want ErrPageCorrupt", err)
	}
}

func TestDecodeDataPage_CorruptedBody(t *testing.T) {
	body := make([]byte, DataPageBodySize)
	data, err := encodeDataPage(body)
	if err != nil {
		t.Fatal(err)
	}
	// Flip a bit in the body region
	data[12+10] ^= 0xFF
	_, err = decodeDataPage(data)
	if err != ErrPageCorrupt {
		t.Errorf("got %v, want ErrPageCorrupt", err)
	}
}

func TestDecodeDataPage_CorruptedChecksum(t *testing.T) {
	body := make([]byte, DataPageBodySize)
	data, err := encodeDataPage(body)
	if err != nil {
		t.Fatal(err)
	}
	// Flip a bit in the checksum field itself
	data[8] ^= 0xFF
	_, err = decodeDataPage(data)
	if err != ErrPageCorrupt {
		t.Errorf("got %v, want ErrPageCorrupt", err)
	}
}

func TestDecodeDataPage_CopySafety(t *testing.T) {
	body := make([]byte, DataPageBodySize)
	body[0] = 0xAB
	data, err := encodeDataPage(body)
	if err != nil {
		t.Fatal(err)
	}

	got, err := decodeDataPage(data)
	if err != nil {
		t.Fatal(err)
	}

	// Mutate the input after decode — output should be unaffected
	data[12] = 0x00
	if got[0] != 0xAB {
		t.Errorf("decode output mutated when input changed: got[0]=%x, want %x", got[0], 0xAB)
	}
}

// -- Free-list pointer encode/decode tests (Task 4) --

func TestEncodeDecodeFreeListPointer_RoundTrip(t *testing.T) {
	pageID := uint64(42)
	body := encodeFreeListPointer(pageID)
	if len(body) != DataPageBodySize {
		t.Fatalf("encodeFreeListPointer returned %d bytes, want %d", len(body), DataPageBodySize)
	}

	got, err := decodeFreeListPointer(body)
	if err != nil {
		t.Fatalf("decodeFreeListPointer: %v", err)
	}
	if got != pageID {
		t.Errorf("got %d, want %d", got, pageID)
	}
}

func TestEncodeDecodeFreeListPointer_Sentinel(t *testing.T) {
	body := encodeFreeListPointer(freeListSentinel)
	got, err := decodeFreeListPointer(body)
	if err != nil {
		t.Fatalf("decodeFreeListPointer: %v", err)
	}
	if got != freeListSentinel {
		t.Errorf("got %d, want sentinel", got)
	}
}

func TestDecodeFreeListPointer_WrongLength(t *testing.T) {
	_, err := decodeFreeListPointer(make([]byte, DataPageBodySize+1))
	if err != ErrPageCorrupt {
		t.Errorf("got %v, want ErrPageCorrupt", err)
	}
}

// -- Open tests (Task 5) --

func TestOpen_CreateNewFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/test.pager"
	ctx := context.Background()

	p := New(path)
	if err := p.Open(ctx); err != nil {
		t.Fatalf("Open on new path: %v", err)
	}
	defer p.Close(ctx)

	// Open a second pager on the same path to verify the header was written
	p2 := New(path)
	if err := p2.Open(ctx); err != nil {
		t.Fatalf("Open on existing file: %v", err)
	}
	defer p2.Close(ctx)

	// PageCount should be 1 (header page only)
	pc, err := p2.PageCount(ctx)
	if err != nil {
		t.Fatalf("PageCount: %v", err)
	}
	if pc != 1 {
		t.Errorf("PageCount = %d, want 1", pc)
	}
}

func TestOpen_ReopenExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/test.pager"
	ctx := context.Background()

	p := New(path)
	if err := p.Open(ctx); err != nil {
		t.Fatalf("First Open: %v", err)
	}
	p.Close(ctx)

	// Reopen with a new instance
	p2 := New(path)
	if err := p2.Open(ctx); err != nil {
		t.Fatalf("Second Open: %v", err)
	}
	pc, err := p2.PageCount(ctx)
	if err != nil {
		t.Fatalf("PageCount: %v", err)
	}
	if pc != 1 {
		t.Errorf("PageCount = %d, want 1", pc)
	}
	p2.Close(ctx)
}

func TestOpen_DoubleOpenReturnsErrAlreadyOpen(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/test.pager"
	ctx := context.Background()

	p := New(path)
	if err := p.Open(ctx); err != nil {
		t.Fatalf("First Open: %v", err)
	}
	defer p.Close(ctx)

	err := p.Open(ctx)
	if err != ErrAlreadyOpen {
		t.Fatalf("Second Open: got %v, want ErrAlreadyOpen", err)
	}
}

func TestOpen_CorruptedHeaderFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/test.pager"
	ctx := context.Background()

	// Create a valid file first
	p := New(path)
	if err := p.Open(ctx); err != nil {
		t.Fatalf("First Open: %v", err)
	}
	p.Close(ctx)

	// Corrupt the magic number in the file
	f, err := os.OpenFile(path, os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt([]byte{0x00, 0x00, 0x00, 0x00}, 0); err != nil {
		t.Fatal(err)
	}
	f.Close()

	// Now try to open — should fail
	p2 := New(path)
	err = p2.Open(ctx)
	if err == nil {
		p2.Close(ctx)
		t.Fatal("expected error opening corrupted file, got nil")
	}
	// Verify it stays in NeverOpened state
	if _, err := p2.PageCount(ctx); err != ErrNotOpen {
		t.Errorf("expected ErrNotOpen after failed Open, got %v", err)
	}
}

// -- ReadPage/WritePage tests (Task 6) --

func TestReadWritePage_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	p := New(dir + "/rw.pager").(*filePager)
	if err := p.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer p.Close(ctx)

	// Allocate a page — for now, directly extend file to create one
	// (AllocatePage is Task 7, so we manually extend)
	if err := p.extendFile(ctx); err != nil {
		t.Fatal(err)
	}

	body := make([]byte, DataPageBodySize)
	for i := range body {
		body[i] = byte(i % 256)
	}

	if err := p.WritePage(ctx, 1, body); err != nil {
		t.Fatalf("WritePage: %v", err)
	}

	got, err := p.ReadPage(ctx, 1)
	if err != nil {
		t.Fatalf("ReadPage: %v", err)
	}
	if len(got) != DataPageBodySize {
		t.Fatalf("ReadPage returned %d bytes, want %d", len(got), DataPageBodySize)
	}
	for i := range body {
		if got[i] != body[i] {
			t.Fatalf("byte %d: got %d, want %d", i, got[i], body[i])
		}
	}
}

func TestReadPage_InvalidPageID(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	p := New(dir + "/invalid.pager").(*filePager)
	if err := p.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer p.Close(ctx)

	// Page 0 is the header — not readable as data page
	if _, err := p.ReadPage(ctx, 0); err != ErrInvalidPageID {
		t.Errorf("ReadPage(0): got %v, want ErrInvalidPageID", err)
	}
	// Page >= pageCount is out of range
	if _, err := p.ReadPage(ctx, 1); err != ErrInvalidPageID {
		t.Errorf("ReadPage(1) on empty file: got %v, want ErrInvalidPageID", err)
	}
}

func TestWritePage_InvalidPageID(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	p := New(dir + "/invalid2.pager").(*filePager)
	if err := p.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer p.Close(ctx)

	body := make([]byte, DataPageBodySize)
	if err := p.WritePage(ctx, 0, body); err != ErrInvalidPageID {
		t.Errorf("WritePage(0): got %v, want ErrInvalidPageID", err)
	}
	if err := p.WritePage(ctx, 1, body); err != ErrInvalidPageID {
		t.Errorf("WritePage(1) on empty file: got %v, want ErrInvalidPageID", err)
	}
}

func TestWritePage_WrongBodyLength(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	p := New(dir + "/wronglen.pager").(*filePager)
	if err := p.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer p.Close(ctx)
	if err := p.extendFile(ctx); err != nil {
		t.Fatal(err)
	}

	if err := p.WritePage(ctx, 1, make([]byte, DataPageBodySize+1)); err != ErrBodySizeMismatch {
		t.Errorf("got %v, want ErrBodySizeMismatch", err)
	}
	if err := p.WritePage(ctx, 1, make([]byte, DataPageBodySize-1)); err != ErrBodySizeMismatch {
		t.Errorf("got %v, want ErrBodySizeMismatch", err)
	}
}

func TestReadWritePage_FreePageRejected(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	p := New(dir + "/freerej.pager").(*filePager)
	if err := p.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer p.Close(ctx)

	// Extend to page 1, then manually add it to freeSet
	if err := p.extendFile(ctx); err != nil {
		t.Fatal(err)
	}
	p.freeSet[1] = struct{}{}

	body := make([]byte, DataPageBodySize)
	if _, err := p.ReadPage(ctx, 1); err != ErrInvalidPageID {
		t.Errorf("ReadPage on free page: got %v, want ErrInvalidPageID", err)
	}
	if err := p.WritePage(ctx, 1, body); err != ErrInvalidPageID {
		t.Errorf("WritePage on free page: got %v, want ErrInvalidPageID", err)
	}
}

func TestReadWritePage_SurvivesCloseReopen(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/survive.pager"
	ctx := context.Background()

	// Create, write, close
	p := New(path).(*filePager)
	if err := p.Open(ctx); err != nil {
		t.Fatal(err)
	}
	if err := p.extendFile(ctx); err != nil {
		t.Fatal(err)
	}
	body := make([]byte, DataPageBodySize)
	body[0] = 0xAB
	body[DataPageBodySize-1] = 0xCD
	if err := p.WritePage(ctx, 1, body); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(ctx); err != nil {
		t.Fatal(err)
	}

	// Reopen and read
	p2 := New(path).(*filePager)
	if err := p2.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer p2.Close(ctx)

	got, err := p2.ReadPage(ctx, 1)
	if err != nil {
		t.Fatalf("ReadPage after reopen: %v", err)
	}
	if got[0] != 0xAB || got[DataPageBodySize-1] != 0xCD {
		t.Errorf("data mismatch after reopen")
	}
}

// extendFile is a helper that extends the backing file by one page, updating
// the header accordingly. Used in tests before AllocatePage is implemented.
func (p *filePager) extendFile(ctx context.Context) error {
	newID := p.pageCount
	// Write a zero-filled body
	body := make([]byte, DataPageBodySize)
	if err := writeBodyUnchecked(p.file, newID, body); err != nil {
		return err
	}
	// Update header
	p.pageCount++
	header := encodeHeader(pagerHeader{
		magic:         MagicNumber,
		version:       FormatVersion,
		pageSize:      PageSize,
		pageCount:     p.pageCount,
		freeListHead:  p.freeListHead,
	})
	if _, err := p.file.WriteAt(header, 0); err != nil {
		return fmt.Errorf("extend: write header: %w", err)
	}
	return p.file.Sync()
}

// -- Free-list walk tests (Task 6) --

// writeHeaderToFile is a test helper that writes an arbitrary header to a file.
func writeHeaderToFile(t *testing.T, path string, h pagerHeader) {
	t.Helper()
	data := encodeHeader(h)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteAt(data, 0); err != nil {
		t.Fatal(err)
	}
}

// writeDataPageToFile is a test helper that writes a data page with given body
// to a file at a specific page offset.
func writeDataPageToFile(t *testing.T, f *os.File, pageID uint64, body []byte) {
	t.Helper()
	if err := writeBodyUnchecked(f, pageID, body); err != nil {
		t.Fatal(err)
	}
}

func TestFreeListWalk_EmptySentinel(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/empty.pager"
	ctx := context.Background()
	// Create a valid file with no free pages
	p := New(path)
	if err := p.Open(ctx); err != nil {
		t.Fatal(err)
	}
	p.Close(ctx)

	// Reopen and verify freeSet is empty
	p2 := New(path).(*filePager)
	if err := p2.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer p2.Close(ctx)
	if len(p2.freeSet) != 0 {
		t.Errorf("freeSet should be empty, got %d entries", len(p2.freeSet))
	}
}

func TestFreeListWalk_HeadPointsAtZero(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/headzero.pager"
	ctx := context.Background()

	// Create a file with header free-list head = 0
	writeHeaderToFile(t, path, pagerHeader{
		magic:         MagicNumber,
		version:       FormatVersion,
		pageSize:      PageSize,
		pageCount:     5,
		freeListHead:  0, // invalid: points at header page
	})

	p := New(path).(*filePager)
	err := p.Open(ctx)
	if err == nil {
		p.Close(ctx)
		t.Fatal("expected error for free-list head = 0, got nil")
	}
	if !errors.Is(err, ErrFreeListCorrupt) {
		t.Errorf("expected ErrFreeListCorrupt, got %v", err)
	}
	// State should still be NeverOpened
	if p.state != stateNeverOpened {
		t.Errorf("state = %v, want stateNeverOpened", p.state)
	}
}

func TestFreeListWalk_HeadOutOfRange(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/headoor.pager"
	ctx := context.Background()

	// Create a file with pageCount=3, head pointing at page 10 (>= pageCount)
	writeHeaderToFile(t, path, pagerHeader{
		magic:         MagicNumber,
		version:       FormatVersion,
		pageSize:      PageSize,
		pageCount:     3,
		freeListHead:  10,
	})

	p := New(path).(*filePager)
	err := p.Open(ctx)
	if err == nil {
		p.Close(ctx)
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrFreeListCorrupt) {
		t.Errorf("expected ErrFreeListCorrupt, got %v", err)
	}
}

func TestFreeListWalk_Cycle(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/cycle.pager"
	ctx := context.Background()

	// Create a file with 3 pages, head pointing at page 1,
	// page 1's free-list pointer points at page 2,
	// page 2's free-list pointer points at page 1 (cycle)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}

	// Write header: pageCount=3, freeListHead=1
	header := encodeHeader(pagerHeader{
		magic:         MagicNumber,
		version:       FormatVersion,
		pageSize:      PageSize,
		pageCount:     3,
		freeListHead:  1,
	})
	if _, err := f.WriteAt(header, 0); err != nil {
		t.Fatal(err)
	}
	// Page 1 points to page 2
	writeDataPageToFile(t, f, 1, encodeFreeListPointer(2))
	// Page 2 points to page 1 (cycle!)
	writeDataPageToFile(t, f, 2, encodeFreeListPointer(1))
	f.Close()

	p := New(path).(*filePager)
	err = p.Open(ctx)
	if err == nil {
		p.Close(ctx)
		t.Fatal("expected error for cycle, got nil")
	}
	if !errors.Is(err, ErrFreeListCorrupt) {
		t.Errorf("expected ErrFreeListCorrupt, got %v", err)
	}
	if p.state != stateNeverOpened {
		t.Errorf("state = %v, want stateNeverOpened", p.state)
	}

	// Verify a fresh Open against a different valid file works
	validPath := dir + "/valid.pager"
	p2 := New(validPath)
	if err := p2.Open(ctx); err != nil {
		t.Errorf("fresh valid Open should work after cycle failure, got: %v", err)
	}
	p2.Close(ctx)
}

func TestFreeListWalk_CorruptChecksum(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/corruptfl.pager"
	ctx := context.Background()

	// Create a file with head pointing at page 1, but page 1 has corrupted body
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	header := encodeHeader(pagerHeader{
		magic:         MagicNumber,
		version:       FormatVersion,
		pageSize:      PageSize,
		pageCount:     3,
		freeListHead:  1,
	})
	if _, err := f.WriteAt(header, 0); err != nil {
		t.Fatal(err)
	}
	// Write a page with garbled data (checksum won't match)
	garbled := make([]byte, PageSize)
	if _, err := f.WriteAt(garbled, int64(1)*PageSize); err != nil {
		t.Fatal(err)
	}
	f.Close()

	p := New(path).(*filePager)
	err = p.Open(ctx)
	if err == nil {
		p.Close(ctx)
		t.Fatal("expected error for corrupt free page, got nil")
	}
	if !errors.Is(err, ErrPageCorrupt) {
		t.Errorf("expected ErrPageCorrupt, got %v", err)
	}
	if p.state != stateNeverOpened {
		t.Errorf("state = %v, want stateNeverOpened", p.state)
	}
}

func TestFreeListWalk_SelfCycle(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/selfcycle.pager"
	ctx := context.Background()

	// Page 1 points to itself
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	header := encodeHeader(pagerHeader{
		magic:         MagicNumber,
		version:       FormatVersion,
		pageSize:      PageSize,
		pageCount:     3,
		freeListHead:  1,
	})
	if _, err := f.WriteAt(header, 0); err != nil {
		t.Fatal(err)
	}
	writeDataPageToFile(t, f, 1, encodeFreeListPointer(1)) // points at itself
	f.Close()

	p := New(path).(*filePager)
	err = p.Open(ctx)
	if err == nil {
		p.Close(ctx)
		t.Fatal("expected error for self-cycle, got nil")
	}
	if !errors.Is(err, ErrFreeListCorrupt) {
		t.Errorf("expected ErrFreeListCorrupt, got %v", err)
	}
}

func TestFreeListWalk_ValidFreeList(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/validfl.pager"
	ctx := context.Background()

	// Create a file with free-list: 1 -> 2 -> sentinel
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	header := encodeHeader(pagerHeader{
		magic:         MagicNumber,
		version:       FormatVersion,
		pageSize:      PageSize,
		pageCount:     4,
		freeListHead:  1,
	})
	if _, err := f.WriteAt(header, 0); err != nil {
		t.Fatal(err)
	}
	writeDataPageToFile(t, f, 1, encodeFreeListPointer(2))
	writeDataPageToFile(t, f, 2, encodeFreeListPointer(freeListSentinel))
	f.Close()

	p := New(path).(*filePager)
	if err := p.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer p.Close(ctx)

	if len(p.freeSet) != 2 {
		t.Errorf("freeSet should have 2 entries, got %d", len(p.freeSet))
	}
	if _, ok := p.freeSet[1]; !ok {
		t.Error("freeSet missing page 1")
	}
	if _, ok := p.freeSet[2]; !ok {
		t.Error("freeSet missing page 2")
	}
	if p.freeListHead != 1 {
		t.Errorf("freeListHead = %d, want 1", p.freeListHead)
	}
}

// -- AllocatePage/FreePage tests (Task 7) --

func TestAllocatePage_Sequential(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	p := New(dir + "/alloc.pager").(*filePager)
	if err := p.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer p.Close(ctx)

	const n = 10
	ids := make(map[uint64]bool)
	for i := 0; i < n; i++ {
		id, err := p.AllocatePage(ctx)
		if err != nil {
			t.Fatalf("AllocatePage %d: %v", i, err)
		}
		if id == 0 {
			t.Fatalf("AllocatePage returned 0 (header page)")
		}
		if ids[id] {
			t.Fatalf("duplicate page ID %d", id)
		}
		ids[id] = true
	}
	pc, err := p.PageCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if pc != uint64(n+1) { // +1 for header page
		t.Errorf("PageCount = %d, want %d", pc, n+1)
	}
}

func TestAllocateFreeReuse(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	p := New(dir + "/reuse.pager").(*filePager)
	if err := p.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer p.Close(ctx)

	// Allocate two pages
	id1, err := p.AllocatePage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.AllocatePage(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Free the first one
	if err := p.FreePage(ctx, id1); err != nil {
		t.Fatal(err)
	}

	// Allocate again — should reuse id1 (LIFO)
	id3, err := p.AllocatePage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if id3 != id1 {
		t.Errorf("expected reuse of %d (free-list LIFO), got %d", id1, id3)
	}

	// PageCount should still be 3 (header + id1 + the second page)
	pc, _ := p.PageCount(ctx)
	if pc != 3 {
		t.Errorf("PageCount = %d, want 3", pc)
	}
}

func TestFreePage_ThenReadRejected(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	p := New(dir + "/freeread.pager").(*filePager)
	if err := p.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer p.Close(ctx)

	id, err := p.AllocatePage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.FreePage(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReadPage(ctx, id); err != ErrInvalidPageID {
		t.Errorf("ReadPage after free: got %v, want ErrInvalidPageID", err)
	}
	if err := p.WritePage(ctx, id, make([]byte, DataPageBodySize)); err != ErrInvalidPageID {
		t.Errorf("WritePage after free: got %v, want ErrInvalidPageID", err)
	}
}

func TestFreePage_DoubleFree(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	p := New(dir + "/doublefree.pager").(*filePager)
	if err := p.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer p.Close(ctx)

	id, err := p.AllocatePage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.FreePage(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := p.FreePage(ctx, id); err != ErrAlreadyFree {
		t.Errorf("double free: got %v, want ErrAlreadyFree", err)
	}
}

func TestFreePage_HeaderPage(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	p := New(dir + "/freehdr.pager").(*filePager)
	if err := p.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer p.Close(ctx)

	if err := p.FreePage(ctx, 0); err != ErrInvalidPageID {
		t.Errorf("free page 0: got %v, want ErrInvalidPageID", err)
	}
}

func TestFreePage_InvalidPageID(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	p := New(dir + "/freeinv.pager").(*filePager)
	if err := p.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer p.Close(ctx)

	// Page beyond end
	if err := p.FreePage(ctx, 100); err != ErrInvalidPageID {
		t.Errorf("free out-of-range: got %v, want ErrInvalidPageID", err)
	}
}

func TestAllocateFree_SurvivesCloseReopen(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/persist.pager"
	ctx := context.Background()

	p := New(path).(*filePager)
	if err := p.Open(ctx); err != nil {
		t.Fatal(err)
	}

	// Allocate 3 pages, free page 2, keep 1 and 3
	_, _ = p.AllocatePage(ctx)
	id2, _ := p.AllocatePage(ctx)
	_, _ = p.AllocatePage(ctx)
	if err := p.FreePage(ctx, id2); err != nil {
		t.Fatal(err)
	}
	pcBefore, _ := p.PageCount(ctx)
	if err := p.Close(ctx); err != nil {
		t.Fatal(err)
	}

	// Reopen
	p2 := New(path).(*filePager)
	if err := p2.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer p2.Close(ctx)

	pcAfter, _ := p2.PageCount(ctx)
	if pcAfter != pcBefore {
		t.Errorf("PageCount after reopen = %d, want %d", pcAfter, pcBefore)
	}

	// Next allocation should reuse the freed page
	id, err := p2.AllocatePage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if id != id2 {
		t.Errorf("expected reuse of %d after reopen, got %d", id2, id)
	}
}

// -- Lifecycle Component wiring test (Task 9) --

func TestLifecyclePattern(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/lifecycle.pager"
	ctx := context.Background()

	// First lifecycle: create, allocate, write, close
	p1 := New(path)
	if err := p1.Open(ctx); err != nil {
		t.Fatal(err)
	}
	id1, err := p1.AllocatePage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	body := make([]byte, DataPageBodySize)
	copy(body, []byte("hello pager"))
	if err := p1.WritePage(ctx, id1, body); err != nil {
		t.Fatal(err)
	}
	if err := p1.Close(ctx); err != nil {
		t.Fatal(err)
	}

	// After Close, calling any method on p1 returns ErrClosed
	if _, err := p1.PageCount(ctx); err != ErrClosed {
		t.Errorf("PageCount after Close: got %v, want ErrClosed", err)
	}

	// Second lifecycle: fresh Pager instance, same path
	p2 := New(path)
	if err := p2.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer p2.Close(ctx)

	// Verify data persisted
	got, err := p2.ReadPage(ctx, id1)
	if err != nil {
		t.Fatal(err)
	}
	if string(got[:11]) != "hello pager" {
		t.Errorf("read back %q, want %q", string(got[:11]), "hello pager")
	}

	// Verify we can still allocate (not hitting any closed state issues)
	id2, err := p2.AllocatePage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if id2 == id1 {
		t.Error("new allocation returned the same ID as an allocated page")
	}
}

// -- Crash-consistency tests (Task 10) --
// These prove the "detection, not prevention" durability contract.

func TestCrashConsistency_DataPageCorruptedChecksum(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/crash_data.pager"
	ctx := context.Background()

	// Create, allocate a page, write known content, close
	p := New(path)
	if err := p.Open(ctx); err != nil {
		t.Fatal(err)
	}
	id, err := p.AllocatePage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	body := make([]byte, DataPageBodySize)
	copy(body, []byte("important data"))
	if err := p.WritePage(ctx, id, body); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(ctx); err != nil {
		t.Fatal(err)
	}

	// Corrupt the checksum byte on disk
	f, err := os.OpenFile(path, os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	offset := int64(id)*PageSize + 8 // checksum is at raw offset [8,12)
	corrupt := make([]byte, 1)
	if _, err := f.ReadAt(corrupt, offset); err != nil {
		t.Fatal(err)
	}
	corrupt[0] ^= 0xFF // flip all bits
	if _, err := f.WriteAt(corrupt, offset); err != nil {
		t.Fatal(err)
	}
	f.Close()

	// Reopen — ReadPage must detect corruption
	p2 := New(path)
	if err := p2.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer p2.Close(ctx)

	_, err = p2.ReadPage(ctx, id)
	if err != ErrPageCorrupt {
		t.Fatalf("ReadPage after checksum corruption: got %v, want ErrPageCorrupt", err)
	}
}

func TestCrashConsistency_HeaderCorruptedChecksum(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/crash_hdr.pager"
	ctx := context.Background()

	// Create a valid pager file
	p := New(path)
	if err := p.Open(ctx); err != nil {
		t.Fatal(err)
	}
	// Allocate a page so pageCount > 1
	if _, err := p.AllocatePage(ctx); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(ctx); err != nil {
		t.Fatal(err)
	}

	// Corrupt the header checksum byte on disk
	f, err := os.OpenFile(path, os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	// Header checksum is at raw offset [28,32)
	corrupt := make([]byte, 1)
	if _, err := f.ReadAt(corrupt, 28); err != nil {
		t.Fatal(err)
	}
	corrupt[0] ^= 0xFF
	if _, err := f.WriteAt(corrupt, 28); err != nil {
		t.Fatal(err)
	}
	f.Close()

	// Reopen — Open must fail with ErrHeaderCorrupt
	p2 := New(path).(*filePager)
	err = p2.Open(ctx)
	if err == nil {
		p2.Close(ctx)
		t.Fatal("expected error opening file with corrupted header checksum, got nil")
	}
	if !errors.Is(err, ErrHeaderCorrupt) {
		t.Errorf("expected ErrHeaderCorrupt, got %v", err)
	}
	// Pager must remain in NeverOpened state
	if p2.state != stateNeverOpened {
		t.Errorf("state = %v, want stateNeverOpened", p2.state)
	}
}

// -- Documentation test (Task 12) --

func TestStorageDocumentation(t *testing.T) {
	data, err := os.ReadFile("../../../docs/storage.md")
	if err != nil {
		t.Fatalf("docs/storage.md not found: %v", err)
	}
	doc := string(data)
	required := []string{
		"# Plomvix Storage: Pager (Basic Tier)",
		"pager",
		"fixed-size page",
		"CRC32",
		"ErrPageCorrupt",
		"Single-page writes are durable",
		"NOT atomic against torn writes",
		"NOT crash-atomic",
		"Header page is the single point of failure",
		"Free-List",
		"WAL",
	}
	for _, s := range required {
		if !contains(doc, s) {
			t.Errorf("missing required phrase in docs/storage.md: %q", s)
		}
	}
}
