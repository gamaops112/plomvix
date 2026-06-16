package runtime

import (
	"context"
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

func callWithRuntimeRecover(operation string, fn func()) (err error) {
	defer recoverRuntimePanic(operation, &err)
	fn()
	return nil
}

func TestRecoverRuntimePanicRecovers(t *testing.T) {
	err := callWithRuntimeRecover("test", func() {
		panic("boom")
	})
	if err == nil {
		t.Fatal("expected error from panic recovery")
	}
	if !errors.Is(err, ErrRuntimePanic) {
		t.Errorf("error = %v, want ErrRuntimePanic", err)
	}
}

func TestRecoverRuntimePanicIncludesOperation(t *testing.T) {
	err := callWithRuntimeRecover("config", func() {
		panic("oops")
	})
	if !strings.Contains(err.Error(), "config") {
		t.Errorf("error should contain operation name: %v", err)
	}
}

func TestRecoverRuntimePanicIncludesPanic(t *testing.T) {
	err := callWithRuntimeRecover("start", func() {
		panic("oops")
	})
	if !strings.Contains(err.Error(), "panic") {
		t.Errorf("error should contain 'panic': %v", err)
	}
}

func TestRecoverRuntimePanicNonPanickingReturnsNil(t *testing.T) {
	err := callWithRuntimeRecover("test", func() {
		// no panic
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRecoverRuntimePanicPreservesExistingError(t *testing.T) {
	existingErr := errors.New("original error")
	err := existingErr
	func() {
		defer recoverRuntimePanic("test", &err)
		panic("boom")
	}()
	if !errors.Is(err, ErrRuntimePanic) {
		t.Errorf("error should match ErrRuntimePanic: %v", err)
	}
	if !errors.Is(err, existingErr) {
		t.Errorf("error should still match original: %v", err)
	}
}

func TestWithSignalContextReturnsNonNil(t *testing.T) {
	ctx, cancel := withSignalContext(context.Background())
	defer cancel()
	if ctx == nil {
		t.Error("ctx is nil")
	}
	if cancel == nil {
		t.Error("cancel is nil")
	}
}

func TestCancelUnblocksGoroutine(t *testing.T) {
	ch := make(chan os.Signal, 1)
	ctx, cancel := withSignalContextFromChan(context.Background(), ch)
	defer cancel()
	cancel()
	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("context was not cancelled after cleanup")
	}
}

func TestSignalDeliveryCancelsContext(t *testing.T) {
	ch := make(chan os.Signal, 1)
	ctx, cancel := withSignalContextFromChan(context.Background(), ch)
	defer cancel()
	ch <- syscall.SIGINT
	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("context was not cancelled after signal")
	}
}
