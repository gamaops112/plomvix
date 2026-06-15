package runtime

import (
	"errors"
	"strings"
	"testing"
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
