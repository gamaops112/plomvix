// Package lifecycle_test provides tests for the lifecycle package.
package lifecycle_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/plomvix/plomvix/internal/lifecycle"
)

// fakeComponent is a test double that records calls and supports
// configurable behavior for start and stop.
type fakeComponent struct {
	name       string
	startErr   error
	stopErr    error
	startPanic bool
	stopPanic  bool
	startCalls int
	stopCalls  int
	startCtx   context.Context
	stopCtx    context.Context
}

func (f *fakeComponent) Name() string { return f.name }

func (f *fakeComponent) Start(ctx context.Context) error {
	f.startCalls++
	if f.startPanic {
		panic("start panic")
	}
	f.startCtx = ctx
	return f.startErr
}

func (f *fakeComponent) Stop(ctx context.Context) error {
	f.stopCalls++
	if f.stopPanic {
		panic("stop panic")
	}
	f.stopCtx = ctx
	return f.stopErr
}

func TestNewManagerReturnsNonNil(t *testing.T) {
	m := lifecycle.NewManager()
	if m == nil {
		t.Fatal("NewManager() returned nil")
	}
}

func TestRegisterNilReturnsError(t *testing.T) {
	m := lifecycle.NewManager()
	err := m.Register(nil)
	if err == nil {
		t.Fatal("expected error for nil component")
	}
	if err != lifecycle.ErrNilComponent {
		t.Errorf("error = %v, want %v", err, lifecycle.ErrNilComponent)
	}
}

func TestRegisterEmptyNameReturnsError(t *testing.T) {
	m := lifecycle.NewManager()
	c := &fakeComponent{name: ""}
	err := m.Register(c)
	if err == nil {
		t.Fatal("expected error for empty component name")
	}
	if err != lifecycle.ErrEmptyComponentName {
		t.Errorf("error = %v, want %v", err, lifecycle.ErrEmptyComponentName)
	}
}

func TestRegisterValidComponentSucceeds(t *testing.T) {
	m := lifecycle.NewManager()
	c := &fakeComponent{name: "test"}
	err := m.Register(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegisterDoesNotCallStart(t *testing.T) {
	m := lifecycle.NewManager()
	c := &fakeComponent{name: "test"}
	if err := m.Register(c); err != nil {
		t.Fatal(err)
	}
	if c.startCalls != 0 {
		t.Errorf("Register called Start %d times, want 0", c.startCalls)
	}
}

func TestRegisterDoesNotCallStop(t *testing.T) {
	m := lifecycle.NewManager()
	c := &fakeComponent{name: "test"}
	if err := m.Register(c); err != nil {
		t.Fatal(err)
	}
	if c.stopCalls != 0 {
		t.Errorf("Register called Stop %d times, want 0", c.stopCalls)
	}
}

// These values are part of the stable lifecycle API.
func TestStateConstants(t *testing.T) {
	if lifecycle.StateNew != "new" {
		t.Errorf("StateNew = %q, want %q", lifecycle.StateNew, "new")
	}
	if lifecycle.StateStarting != "starting" {
		t.Errorf("StateStarting = %q, want %q", lifecycle.StateStarting, "starting")
	}
	if lifecycle.StateStarted != "started" {
		t.Errorf("StateStarted = %q, want %q", lifecycle.StateStarted, "started")
	}
	if lifecycle.StateStopping != "stopping" {
		t.Errorf("StateStopping = %q, want %q", lifecycle.StateStopping, "stopping")
	}
	if lifecycle.StateStopped != "stopped" {
		t.Errorf("StateStopped = %q, want %q", lifecycle.StateStopped, "stopped")
	}
	if lifecycle.StateFailed != "failed" {
		t.Errorf("StateFailed = %q, want %q", lifecycle.StateFailed, "failed")
	}
}

func TestNewManagerStateIsNew(t *testing.T) {
	m := lifecycle.NewManager()
	if m.State() != lifecycle.StateNew {
		t.Errorf("new manager state = %q, want %q", m.State(), lifecycle.StateNew)
	}
}

func TestNilManagerStateIsFailed(t *testing.T) {
	var m *lifecycle.Manager
	if got := m.State(); got != lifecycle.StateFailed {
		t.Errorf("nil manager state = %q, want %q", got, lifecycle.StateFailed)
	}
}

func TestStartWithNoComponentsSucceeds(t *testing.T) {
	m := lifecycle.NewManager()
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStartComponentsInRegistrationOrder(t *testing.T) {
	m := lifecycle.NewManager()
	var order []string
	// Use a recording wrapper
	c1 := &recordingComponent{name: "a", started: &order}
	c2 := &recordingComponent{name: "b", started: &order}
	c3 := &recordingComponent{name: "c", started: &order}

	m.Register(c1)
	m.Register(c2)
	m.Register(c3)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(order) != 3 || order[0] != "a" || order[1] != "b" || order[2] != "c" {
		t.Errorf("start order = %v, want [a b c]", order)
	}
}

// recordingComponent is a Component that appends its name to started on Start
// and to stopped on Stop.
type recordingComponent struct {
	name    string
	started *[]string
	stopped *[]string
}

func (r *recordingComponent) Name() string { return r.name }
func (r *recordingComponent) Start(ctx context.Context) error {
	if r.started != nil {
		*r.started = append(*r.started, r.name)
	}
	return nil
}
func (r *recordingComponent) Stop(ctx context.Context) error {
	if r.stopped != nil {
		*r.stopped = append(*r.stopped, r.name)
	}
	return nil
}

func TestStartPassesContext(t *testing.T) {
	m := lifecycle.NewManager()
	c := &fakeComponent{name: "test"}
	m.Register(c)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if c.startCtx != ctx {
		t.Error("Start did not pass the provided context")
	}
}

func TestStartStopsAtFirstError(t *testing.T) {
	m := lifecycle.NewManager()
	errFail := errors.New("boom")
	c1 := &fakeComponent{name: "a"}
	c2 := &fakeComponent{name: "b", startErr: errFail}
	c3 := &fakeComponent{name: "c"}

	m.Register(c1)
	m.Register(c2)
	m.Register(c3)

	err := m.Start(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "b") {
		t.Errorf("error should contain component name 'b': %v", err)
	}
	if !errors.Is(err, errFail) {
		t.Errorf("error should wrap errFail: %v", err)
	}
	if c3.startCalls > 0 {
		t.Error("component after failed component should not be started")
	}
}

func TestStartFailedErrorIncludesComponentName(t *testing.T) {
	m := lifecycle.NewManager()
	c := &fakeComponent{name: "failing", startErr: errors.New("boom")}
	m.Register(c)

	err := m.Start(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failing") {
		t.Errorf("error should contain 'failing': %v", err)
	}
}

func TestRepeatedStartReturnsAlreadyStarted(t *testing.T) {
	m := lifecycle.NewManager()
	c := &fakeComponent{name: "test"}
	m.Register(c)

	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	err := m.Start(context.Background())
	if err == nil {
		t.Fatal("expected error for repeated start")
	}
	if err != lifecycle.ErrAlreadyStarted {
		t.Errorf("error = %v, want %v", err, lifecycle.ErrAlreadyStarted)
	}
}

func TestStopBeforeStartReturnsNil(t *testing.T) {
	m := lifecycle.NewManager()
	if err := m.Stop(context.Background()); err != nil {
		t.Errorf("stop before start should return nil, got: %v", err)
	}
}

func TestStopReverseSuccessfulStartOrder(t *testing.T) {
	m := lifecycle.NewManager()
	var stopped []string
	c1 := &recordingComponent{name: "a", stopped: &stopped}
	c2 := &recordingComponent{name: "b", stopped: &stopped}
	c3 := &recordingComponent{name: "c", stopped: &stopped}

	m.Register(c1)
	m.Register(c2)
	m.Register(c3)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(stopped) != 3 || stopped[0] != "c" || stopped[1] != "b" || stopped[2] != "a" {
		t.Errorf("stop order = %v, want [c b a]", stopped)
	}
}

func TestStopPassesContext(t *testing.T) {
	m := lifecycle.NewManager()
	c := &fakeComponent{name: "test"}
	m.Register(c)

	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := m.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if c.stopCtx != ctx {
		t.Error("Stop did not pass the provided context")
	}
}

func TestStopOnlyStopsSuccessfullyStarted(t *testing.T) {
	m := lifecycle.NewManager()
	errFail := errors.New("fail")
	cA := &fakeComponent{name: "a"}
	cB := &fakeComponent{name: "b", startErr: errFail}
	cC := &fakeComponent{name: "c"}

	m.Register(cA)
	m.Register(cB)
	m.Register(cC)

	_ = m.Start(context.Background())
	_ = m.Stop(context.Background())

	if cA.stopCalls != 1 {
		t.Errorf("A should be stopped once, got %d", cA.stopCalls)
	}
	if cB.stopCalls != 0 {
		t.Error("B failed start, should not be stopped")
	}
	if cC.stopCalls != 0 {
		t.Error("C was never started, should not be stopped")
	}
}

func TestStopAttemptsAllEvenIfOneFails(t *testing.T) {
	m := lifecycle.NewManager()
	errFail := errors.New("stop fail")
	cA := &fakeComponent{name: "a", stopErr: errFail}
	cB := &fakeComponent{name: "b"}

	m.Register(cA)
	m.Register(cB)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	_ = m.Stop(context.Background())

	if cA.stopCalls != 1 {
		t.Errorf("A should be stopped once, got %d", cA.stopCalls)
	}
	if cB.stopCalls != 1 {
		t.Errorf("B should be stopped once even though A failed, got %d", cB.stopCalls)
	}
}

func TestStopErrorIncludesComponentName(t *testing.T) {
	m := lifecycle.NewManager()
	c := &fakeComponent{name: "failing", stopErr: errors.New("boom")}
	m.Register(c)

	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	err := m.Stop(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failing") {
		t.Errorf("error should contain 'failing': %v", err)
	}
}

func TestMultipleStopErrorsIncludeAllNames(t *testing.T) {
	m := lifecycle.NewManager()
	cA := &fakeComponent{name: "alpha", stopErr: errors.New("a")}
	cB := &fakeComponent{name: "beta", stopErr: errors.New("b")}

	m.Register(cA)
	m.Register(cB)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	err := m.Stop(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "alpha") {
		t.Errorf("error should contain 'alpha': %v", err)
	}
	if !strings.Contains(msg, "beta") {
		t.Errorf("error should contain 'beta': %v", err)
	}
}

func TestRepeatedStopAfterSuccessReturnsNil(t *testing.T) {
	m := lifecycle.NewManager()
	c := &fakeComponent{name: "test"}
	m.Register(c)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Second stop should be nil.
	if err := m.Stop(context.Background()); err != nil {
		t.Errorf("second stop should return nil, got: %v", err)
	}
	if c.stopCalls != 1 {
		t.Errorf("component stop called %d times, want 1", c.stopCalls)
	}
}

func TestRepeatedStopAfterFailedStopReturnsNil(t *testing.T) {
	m := lifecycle.NewManager()
	c := &fakeComponent{name: "test", stopErr: errors.New("fail")}
	m.Register(c)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	_ = m.Stop(context.Background())
	// Second stop should be nil, not double-call the component.
	if err := m.Stop(context.Background()); err != nil {
		t.Errorf("second stop after failed stop should return nil, got: %v", err)
	}
	if c.stopCalls != 1 {
		t.Errorf("component stop called %d times after failed stop, want 1", c.stopCalls)
	}
}

func TestRegisterAfterSuccessfulStartIsRejected(t *testing.T) {
	m := lifecycle.NewManager()
	c := &fakeComponent{name: "a"}
	m.Register(c)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	err := m.Register(&fakeComponent{name: "b"})
	if err == nil {
		t.Fatal("expected error for registration after start")
	}
	if err != lifecycle.ErrAlreadyStarted {
		t.Errorf("error = %v, want %v", err, lifecycle.ErrAlreadyStarted)
	}
}

func TestRegisterAfterFailedStartIsRejected(t *testing.T) {
	m := lifecycle.NewManager()
	c := &fakeComponent{name: "a", startErr: errors.New("fail")}
	m.Register(c)
	_ = m.Start(context.Background())

	err := m.Register(&fakeComponent{name: "b"})
	if err == nil {
		t.Fatal("expected error for registration after failed start")
	}
	if err != lifecycle.ErrAlreadyStarted {
		t.Errorf("error = %v, want %v", err, lifecycle.ErrAlreadyStarted)
	}
}

func TestStateTransitions(t *testing.T) {
	t.Run("new manager is StateNew", func(t *testing.T) {
		m := lifecycle.NewManager()
		if m.State() != lifecycle.StateNew {
			t.Errorf("state = %q, want StateNew", m.State())
		}
	})

	t.Run("successful start is StateStarted", func(t *testing.T) {
		m := lifecycle.NewManager()
		m.Register(&fakeComponent{name: "a"})
		if err := m.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
		if m.State() != lifecycle.StateStarted {
			t.Errorf("state = %q, want StateStarted", m.State())
		}
	})

	t.Run("failed start is StateFailed", func(t *testing.T) {
		m := lifecycle.NewManager()
		m.Register(&fakeComponent{name: "a", startErr: errors.New("boom")})
		_ = m.Start(context.Background())
		if m.State() != lifecycle.StateFailed {
			t.Errorf("state = %q, want StateFailed", m.State())
		}
	})

	t.Run("stop before start is StateStopped", func(t *testing.T) {
		m := lifecycle.NewManager()
		_ = m.Stop(context.Background())
		if m.State() != lifecycle.StateStopped {
			t.Errorf("state = %q, want StateStopped", m.State())
		}
	})

	t.Run("stop after successful start is StateStopped", func(t *testing.T) {
		m := lifecycle.NewManager()
		m.Register(&fakeComponent{name: "a"})
		m.Start(context.Background())
		m.Stop(context.Background())
		if m.State() != lifecycle.StateStopped {
			t.Errorf("state = %q, want StateStopped", m.State())
		}
	})

	t.Run("stop after failed start is StateStopped", func(t *testing.T) {
		m := lifecycle.NewManager()
		m.Register(&fakeComponent{name: "a", startErr: errors.New("boom")})
		_ = m.Start(context.Background())
		_ = m.Stop(context.Background())
		if m.State() != lifecycle.StateStopped {
			t.Errorf("state = %q, want StateStopped", m.State())
		}
	})

	t.Run("repeated stop remains StateStopped", func(t *testing.T) {
		m := lifecycle.NewManager()
		m.Stop(context.Background())
		m.Stop(context.Background())
		if m.State() != lifecycle.StateStopped {
			t.Errorf("state = %q, want StateStopped", m.State())
		}
	})

	t.Run("repeated start after success returns ErrAlreadyStarted", func(t *testing.T) {
		m := lifecycle.NewManager()
		m.Register(&fakeComponent{name: "a"})
		m.Start(context.Background())
		err := m.Start(context.Background())
		if !errors.Is(err, lifecycle.ErrAlreadyStarted) {
			t.Errorf("error = %v, want ErrAlreadyStarted", err)
		}
	})

	t.Run("repeated start after failure returns ErrAlreadyStarted", func(t *testing.T) {
		m := lifecycle.NewManager()
		m.Register(&fakeComponent{name: "a", startErr: errors.New("boom")})
		_ = m.Start(context.Background())
		err := m.Start(context.Background())
		if !errors.Is(err, lifecycle.ErrAlreadyStarted) {
			t.Errorf("error = %v, want ErrAlreadyStarted", err)
		}
	})
}

func TestLifecycleDocumentation(t *testing.T) {
	data, err := os.ReadFile("../../docs/markdown/lifecycle.md")
	if err != nil {
		t.Fatalf("docs/markdown/lifecycle.md not found: %v", err)
	}
	content := string(data)

	required := []string{
		"# Plomvix Lifecycle",
		"component interface",
		"manager",
		"registration order",
		"start order",
		"reverse stop order",
		"context-aware",
		"start error",
		"stop error",
		"stop idempotency",
		"registration after start is rejected",
		"mutex-protected manager state",
		"lifecycle states",
		"state transitions",
		"duplicate component",
		"panic recovery",
		"start panic",
		"stop panic",
		"stop remains idempotent",
		"Stop while Start is in progress returns ErrInvalidState",
		"signal handling",
		"WAL",
		"storage",
		"query engine",
		"API server",
		"UI",
		"logger integration",
		"config integration",
	}
	for _, s := range required {
		if !strings.Contains(content, s) {
			t.Errorf("docs/markdown/lifecycle.md missing required string: %q", s)
		}
	}
}

func TestRegisterDuplicateNameReturnsError(t *testing.T) {
	m := lifecycle.NewManager()
	if err := m.Register(&fakeComponent{name: "storage"}); err != nil {
		t.Fatal(err)
	}
	err := m.Register(&fakeComponent{name: "storage"})
	if err == nil {
		t.Fatal("expected error for duplicate component name")
	}
	if !errors.Is(err, lifecycle.ErrDuplicateComponent) {
		t.Errorf("error = %v, want ErrDuplicateComponent", err)
	}
	if !strings.Contains(err.Error(), "storage") {
		t.Errorf("error should contain duplicate name 'storage': %v", err)
	}
}

func TestRegisterDuplicateCaseSensitive(t *testing.T) {
	m := lifecycle.NewManager()
	if err := m.Register(&fakeComponent{name: "storage"}); err != nil {
		t.Fatal(err)
	}
	// Different case — should succeed (case-sensitive check).
	if err := m.Register(&fakeComponent{name: "Storage"}); err != nil {
		t.Fatalf("case-different name should succeed: %v", err)
	}
}

func TestStartPanicReturnsError(t *testing.T) {
	m := lifecycle.NewManager()
	m.Register(&fakeComponent{name: "panicker", startPanic: true})

	err := m.Start(context.Background())
	if err == nil {
		t.Fatal("expected error for start panic")
	}
	if !strings.Contains(err.Error(), "panicker") {
		t.Errorf("error should contain component name: %v", err)
	}
	if !strings.Contains(err.Error(), "panic") {
		t.Errorf("error should contain 'panic': %v", err)
	}
}

func TestStartPanicTransitionsToStateFailed(t *testing.T) {
	m := lifecycle.NewManager()
	m.Register(&fakeComponent{name: "panicker", startPanic: true})

	_ = m.Start(context.Background())
	if m.State() != lifecycle.StateFailed {
		t.Errorf("state = %q, want StateFailed", m.State())
	}
}

func TestComponentsAfterStartPanicAreNotStarted(t *testing.T) {
	m := lifecycle.NewManager()
	cA := &fakeComponent{name: "a"}
	cB := &fakeComponent{name: "b", startPanic: true}
	cC := &fakeComponent{name: "c"}

	m.Register(cA)
	m.Register(cB)
	m.Register(cC)

	_ = m.Start(context.Background())

	if cA.startCalls != 1 {
		t.Errorf("A should be started, got %d", cA.startCalls)
	}
	if cC.startCalls != 0 {
		t.Error("C should not be started after panic")
	}
}

func TestStopAfterStartPanicStopsOnlyStartedBeforePanic(t *testing.T) {
	m := lifecycle.NewManager()
	cA := &fakeComponent{name: "a"}
	cB := &fakeComponent{name: "b", startPanic: true}
	cC := &fakeComponent{name: "c"}

	m.Register(cA)
	m.Register(cB)
	m.Register(cC)

	_ = m.Start(context.Background())
	_ = m.Stop(context.Background())

	if cA.stopCalls != 1 {
		t.Errorf("A should be stopped, got %d", cA.stopCalls)
	}
	if cB.stopCalls != 0 {
		t.Error("B panicked during start, should not be stopped")
	}
	if cC.stopCalls != 0 {
		t.Error("C was never started, should not be stopped")
	}
}

func TestStopPanicReturnsError(t *testing.T) {
	m := lifecycle.NewManager()
	c := &fakeComponent{name: "panicker", stopPanic: true}
	m.Register(c)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	err := m.Stop(context.Background())
	if err == nil {
		t.Fatal("expected error for stop panic")
	}
	if !strings.Contains(err.Error(), "panicker") {
		t.Errorf("error should contain component name: %v", err)
	}
	if !strings.Contains(err.Error(), "panic") {
		t.Errorf("error should contain 'panic': %v", err)
	}
}

func TestStopContinuesAfterPanic(t *testing.T) {
	m := lifecycle.NewManager()
	cC := &fakeComponent{name: "c", stopPanic: true}
	cB := &fakeComponent{name: "b", stopErr: errors.New("fail")}
	cA := &fakeComponent{name: "a"}

	m.Register(cA)
	m.Register(cB)
	m.Register(cC)

	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	err := m.Stop(context.Background())
	// Stop order: C (panics), B (error), A (ok)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "c") || !strings.Contains(msg, "panic") {
		t.Errorf("error should contain C and panic: %v", err)
	}
	if !strings.Contains(msg, "b") {
		t.Errorf("error should contain B: %v", err)
	}
	if cA.stopCalls != 1 {
		t.Errorf("A should be stopped even after C panic and B error, got %d", cA.stopCalls)
	}
}

func TestStopPanicTransitionsToStateStopped(t *testing.T) {
	m := lifecycle.NewManager()
	m.Register(&fakeComponent{name: "panicker", stopPanic: true})
	m.Start(context.Background())
	_ = m.Stop(context.Background())
	if m.State() != lifecycle.StateStopped {
		t.Errorf("state = %q, want StateStopped", m.State())
	}
}

func TestRepeatedStopAfterStopPanicReturnsNil(t *testing.T) {
	m := lifecycle.NewManager()
	c := &fakeComponent{name: "panicker", stopPanic: true}
	m.Register(c)
	m.Start(context.Background())
	_ = m.Stop(context.Background())

	err := m.Stop(context.Background())
	if err != nil {
		t.Errorf("repeated stop after stop panic should return nil, got: %v", err)
	}
	if c.stopCalls != 1 {
		t.Errorf("component stop called %d times after panic, want 1", c.stopCalls)
	}
}

func TestConcurrentStateCallsSafe(t *testing.T) {
	m := lifecycle.NewManager()
	m.Register(&fakeComponent{name: "test"})
	m.Start(context.Background())

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = m.State()
		}()
	}
	wg.Wait()
}

// atomicFakeComponent is a concurrent-safe fake component for race tests.
type atomicFakeComponent struct {
	name      string
	stopCalls atomic.Int32
}

func (a *atomicFakeComponent) Name() string                    { return a.name }
func (a *atomicFakeComponent) Start(ctx context.Context) error { return nil }
func (a *atomicFakeComponent) Stop(ctx context.Context) error  { a.stopCalls.Add(1); return nil }

func TestConcurrentRepeatedStopSafe(t *testing.T) {
	m := lifecycle.NewManager()
	c := &atomicFakeComponent{name: "test"}
	m.Register(c)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := m.Stop(context.Background())
			// ErrInvalidState is expected for concurrent callers that
			// observe StateStopping. Only unexpected errors should fail.
			if err != nil && !errors.Is(err, lifecycle.ErrInvalidState) {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	if c.stopCalls.Load() != 1 {
		t.Errorf("component stop called %d times, want 1", c.stopCalls.Load())
	}
}
