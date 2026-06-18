package pager

import (
	"io"
	"os"
	"sync"
	"testing"
)

// faultInjectingFileOps wraps realFileOps and injects faults on configurable
// call counts. It implements fileOps.
type faultInjectingFileOps struct {
	real          fileOps
	mu            sync.Mutex
	callCount     int              // total calls made
	failSyncOn    map[int]error    // call number -> error to return from Sync
	failWriteAtOn map[int]error    // call number -> error to return from WriteAt
	shortWriteOn  map[int]struct{} // call number -> return short write from WriteAt
}

func newFaultInjectingFileOps(real fileOps) *faultInjectingFileOps {
	return &faultInjectingFileOps{
		real:          real,
		failSyncOn:    make(map[int]error),
		failWriteAtOn: make(map[int]error),
		shortWriteOn:  make(map[int]struct{}),
	}
}

func (f *faultInjectingFileOps) next() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.callCount++
	return f.callCount
}

func (f *faultInjectingFileOps) ReadAt(p []byte, off int64) (n int, err error) {
	return f.real.ReadAt(p, off)
}

func (f *faultInjectingFileOps) WriteAt(p []byte, off int64) (n int, err error) {
	nth := f.next()
	if err, ok := f.failWriteAtOn[nth]; ok {
		return 0, err
	}
	if _, ok := f.shortWriteOn[nth]; ok && len(p) > 0 {
		return len(p) - 1, nil // return less than len(p)
	}
	return f.real.WriteAt(p, off)
}

func (f *faultInjectingFileOps) Sync() error {
	nth := f.next()
	if err, ok := f.failSyncOn[nth]; ok {
		return err
	}
	return f.real.Sync()
}

func (f *faultInjectingFileOps) Close() error {
	return f.real.Close()
}

func (f *faultInjectingFileOps) Stat() (os.FileInfo, error) {
	return f.real.Stat()
}

func (f *faultInjectingFileOps) Truncate(size int64) error {
	return f.real.Truncate(size)
}

func TestFaultInjection_BasicDelegation(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/faultbasic.pager"

	p, err := newPager(path, Options{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Wrap both main and WAL file ops with fault injectors
	mainFI := newFaultInjectingFileOps(p.mainFileOps)
	walFI := newFaultInjectingFileOps(p.walFileOps)
	p.mainFileOps = mainFI
	p.walFileOps = walFI

	ctx := t.Context()
	if err := p.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer p.Close(ctx)

	// Basic operations should work unchanged
	id, err := p.AllocatePage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	body := make([]byte, DataPageBodySize)
	if err := p.WritePage(ctx, id, body); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReadPage(ctx, id); err != nil {
		t.Fatal(err)
	}
}

func TestFaultInjection_ShortWriteDetected(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/shortwrite.pager"

	p, err := newPager(path, Options{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()
	if err := p.Open(ctx); err != nil {
		t.Fatal(err)
	}

	mainFI := newFaultInjectingFileOps(p.mainFileOps)
	p.mainFileOps = mainFI
	mainFI.callCount = 0

	if _, err := p.AllocatePage(ctx); err != nil {
		t.Fatal(err)
	}
	// After AllocatePage: writeBodyUnchecked (WriteAt+Sync) + writeHeader
	// (WriteAt+Sync+WriteAt+Sync) → callCount = 6

	// WritePage → writeBodyUnchecked → WriteAt (call 7), Sync (call 8)
	// Make WriteAt (call 7) return short write.
	mainFI.shortWriteOn[7] = struct{}{}

	body := make([]byte, DataPageBodySize)
	err = p.WritePage(ctx, FirstDataPageID, body)
	if err != io.ErrShortWrite {
		t.Fatalf("expected io.ErrShortWrite, got %v", err)
	}
}

func TestFaultInjection_SyncFailure(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/syncfail.pager"

	// Use newPager with nil mainFileOps so the real file is opened.
	p, err := newPager(path, Options{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Wrap with fault injector AFTER Open so we control counting from a
	// known state.
	ctx := t.Context()
	if err := p.Open(ctx); err != nil {
		t.Fatal(err)
	}

	mainFI := newFaultInjectingFileOps(p.mainFileOps)
	p.mainFileOps = mainFI

	// Reset counter after wrap (it starts at 0)
	mainFI.callCount = 0

	// AllocatePage: writeBodyUnchecked (WriteAt+Sync) then writeHeader
	// (WriteAt+Sync+WriteAt+Sync) = 2 + 4 = 6 calls through mainFI.
	id, err := p.AllocatePage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// callCount is now 6.

	// WritePage → writeBodyUnchecked → WriteAt (call 7), Sync (call 8).
	// Fail Sync (call 8).
	mainFI.failSyncOn[8] = io.ErrUnexpectedEOF

	body := make([]byte, DataPageBodySize)
	err = p.WritePage(ctx, id, body)
	if err == nil {
		t.Fatal("expected error from sync failure, got nil")
	}
}

// -- Task 8: Fault injection tests for crash recovery scenarios --

func TestFaultInjection_WALFsyncFails_CommitTxReturnsError_InTxnRemains(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/walfsyncfail.pager"

	p, err := newPager(path, Options{CheckpointThreshold: 1 << 62}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()
	if err := p.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer p.Close(ctx)

	if err := p.extendFile(ctx); err != nil {
		t.Fatal(err)
	}

	// Wrap WAL with fault injector.
	walFI := newFaultInjectingFileOps(p.walFileOps)
	p.walFileOps = walFI

	// BeginTx -> calls checkState only (no I/O)
	if err := p.BeginTx(ctx); err != nil {
		t.Fatal(err)
	}

	body := make([]byte, DataPageBodySize)
	body[0] = 0xE1
	if err := p.WritePage(ctx, FirstDataPageID, body); err != nil {
		t.Fatal(err)
	}
	// WritePage in tx: appendWAL -> WriteAt (call 1) + Sync (call 2)

	// CommitTx will:
	// 1. WriteAt EOT (call 3)
	// 2. Sync WAL (call 4) — we'll fail this.
	walFI.failSyncOn[4] = io.ErrUnexpectedEOF

	err = p.CommitTx(ctx)
	if err == nil {
		t.Fatal("expected error from WAL fsync failure, got nil")
	}

	// InTxn should remain true after failed commit.
	if !p.inTxn {
		t.Error("inTxn should be true after failed WAL fsync")
	}
}

func TestFaultInjection_MainFileWriteFailure_RecoveryOnReopen(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/mfail.pager"

	p, err := newPager(path, Options{CheckpointThreshold: 1 << 62}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()
	if err := p.Open(ctx); err != nil {
		t.Fatal(err)
	}

	if err := p.extendFile(ctx); err != nil {
		t.Fatal(err)
	}

	// Wrap main file with fault injector.
	mainFI := newFaultInjectingFileOps(p.mainFileOps)
	p.mainFileOps = mainFI

	if err := p.BeginTx(ctx); err != nil {
		t.Fatal(err)
	}

	body := make([]byte, DataPageBodySize)
	body[0] = 0xF0
	if err := p.WritePage(ctx, FirstDataPageID, body); err != nil {
		t.Fatal(err)
	}

	// CommitTx does:
	// WAL: WriteAt EOT (wal call 1), Sync WAL (wal call 2) — these use walFileOps, not mainFI
	// Then main file: WriteAt encoded page (main call 1), Sync main (main call 2)
	// Fail the main file WriteAt (call 1).
	mainFI.failWriteAtOn[1] = io.ErrShortWrite

	err = p.CommitTx(ctx)
	if err == nil {
		t.Fatal("expected error from main file write failure, got nil")
	}

	// inTxn should be cleared (EOT was written successfully to WAL).
	if p.inTxn {
		t.Error("inTxn should be cleared after main file write failure")
	}

	// Close the pager — the WAL still contains the committed data.
	if err := p.Close(ctx); err != nil {
		t.Fatal(err)
	}

	// Reopen — WAL replay should recover the committed data.
	p2, err := newPager(path, Options{CheckpointThreshold: 1 << 62}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := p2.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer p2.Close(ctx)

	got, err := p2.ReadPage(ctx, FirstDataPageID)
	if err != nil {
		t.Fatal(err)
	}
	if got[0] != 0xF0 {
		t.Errorf("replay did not recover data: got 0x%x, want 0xF0", got[0])
	}
}

func TestFaultInjection_HeaderPage0WriteFails_RecoverFromPage1(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/hdr0fail.pager"

	p, err := newPager(path, Options{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()
	if err := p.Open(ctx); err != nil {
		t.Fatal(err)
	}

	// Allocate a page to trigger a header write.
	mainFI := newFaultInjectingFileOps(p.mainFileOps)
	p.mainFileOps = mainFI

	// AllocatePage (extend): writeBodyUnchecked (WriteAt+Sync) + writeHeader
	// writeHeader: WriteAt(mirror)+Sync+WriteAt(primary)+Sync = 4 calls
	// So "extend" is: WriteAt(data)+Sync(data) + WriteAt(header1)+Sync+WriteAt(header0)+Sync
	// = calls 1,2,3,4,5,6
	// We want to fail WriteAt(header0) — that's call 5.
	// Let's just make WriteAt(header0) fail.
	mainFI.failWriteAtOn[5] = io.ErrUnexpectedEOF

	_, err = p.AllocatePage(ctx)
	if err == nil {
		t.Fatal("expected error from header page 0 write failure, got nil")
	}

	// In-memory state should NOT be updated (pageCount unchanged).
	if p.pageCount != 2 {
		t.Errorf("pageCount should be 2 after failed header write, got %d", p.pageCount)
	}

	// Close and reopen — recovery should read from page 1 mirror.
	if err := p.Close(ctx); err != nil {
		t.Fatal(err)
	}

	// Page 1 should be valid (written before page 0 failed).
	p2, err := newPager(path, Options{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := p2.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer p2.Close(ctx)

	if p2.pageCount != 2 {
		t.Errorf("after recovery, pageCount = %d, want 2", p2.pageCount)
	}
}

func TestFaultInjection_HeaderPage1WriteFails_NeitherPageUpdated(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/hdr1fail.pager"

	p, err := newPager(path, Options{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()
	if err := p.Open(ctx); err != nil {
		t.Fatal(err)
	}

	mainFI := newFaultInjectingFileOps(p.mainFileOps)
	p.mainFileOps = mainFI

	// Fail the mirror header write (first write in writeHeader = call 3).
	mainFI.failWriteAtOn[3] = io.ErrUnexpectedEOF

	_, err = p.AllocatePage(ctx)
	if err == nil {
		t.Fatal("expected error from header page 1 write failure, got nil")
	}

	// In-memory state should be unchanged.
	if p.pageCount != 2 {
		t.Errorf("pageCount should be 2 after failed header write, got %d", p.pageCount)
	}

	// Close and reopen — page 0 should still have the original header (pageCount=2).
	if err := p.Close(ctx); err != nil {
		t.Fatal(err)
	}

	p2, err := newPager(path, Options{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := p2.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer p2.Close(ctx)

	if p2.pageCount != 2 {
		t.Errorf("after recovery, pageCount = %d, want 2", p2.pageCount)
	}
}
