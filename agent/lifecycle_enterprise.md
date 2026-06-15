# lifecycle_enterprise.md

# Plomvix Enterprise Lifecycle Hardening Plan

## Purpose

Harden the existing Plomvix lifecycle foundation into a safer enterprise-grade core package.

This plan improves correctness, state clarity, failure safety, panic safety, duplicate protection, documentation, and test coverage.

This is still lifecycle-package hardening only.

Do not wire lifecycle into `cmd/plomvix/main.go`.

Do not add WAL, storage, query engine, API server, UI, workload engines, OS signal handling, background service orchestration, or logger/config integration.

---

## Required Starting State

This plan starts only after `lifecycle.md` is completed and verified.

Before starting this plan, the project must already have:

```text
internal/lifecycle/lifecycle.go
internal/lifecycle/lifecycle_test.go
docs/lifecycle.md
```

The lifecycle package must already expose:

```go
type Component interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

type Manager struct {
	// unexported fields only
}

func NewManager() *Manager
func (m *Manager) Register(component Component) error
func (m *Manager) Start(ctx context.Context) error
func (m *Manager) Stop(ctx context.Context) error
```

Existing public errors must include:

```go
var (
	ErrNilComponent       = errors.New("lifecycle: nil component")
	ErrEmptyComponentName = errors.New("lifecycle: empty component name")
	ErrAlreadyStarted    = errors.New("lifecycle: already started")
)
```

Existing behavior must already include:

* registration order is preserved
* start order follows registration order
* stop order follows reverse successful-start order
* registration after start is rejected
* stop is idempotent after successful stop
* stop is idempotent after failed stop
* manager state is protected with `sync.Mutex`
* `go test -race ./...` passes

If this starting state is not true, stop and report that `lifecycle.md` is incomplete.

---

## Current Project State

Completed foundation work:

```text
config foundation: done
basic logger setup: done
enterprise logger hardening: done
basic lifecycle foundation: done
```

Current stage:

```text
core foundation only
```

Current feature area:

```text
enterprise lifecycle hardening
```

---

## Go Version Requirement

Plomvix uses:

```text
Go 1.22 or later
```

This plan may use standard library features available in Go 1.22.

`errors.Join` is available in Go 1.20+ and is allowed.

---

## Coding Agent

Coding agent:

```text
DeepSeek V4 Pro
```

If the local environment uses a different exact DeepSeek model identifier, use the configured DeepSeek coding model available there.

Tasks must be executed one at a time, in exact order.

Do not proceed to the next task until the current task passes verification.

---

## Graphify Rule

For every task:

1. Search Graphify before starting the task if Graphify is available.
2. Update Graphify after completing the task if Graphify is available.
3. If Graphify is unavailable, do not block the task.
4. Mention Graphify availability in the task report.

---

## Global Project Rules

Follow these rules for every task:

* Keep the implementation small.
* Keep the package focused.
* Do not add unrelated folders.
* Do not modify unrelated files.
* Do not add new external dependencies.
* Use Go standard library only.
* Keep tests deterministic.
* Keep public API minimal.
* Do not introduce global runtime state.
* Do not call `os.Exit`.
* Do not use goroutines unless a task explicitly requires them.
* Protect manager state with `sync.Mutex`.
* Do not wire lifecycle into the application entrypoint.
* Do not import logger.
* Do not import config.
* Do not create WAL, storage, query, API, UI, or engine code.

---

## Dependency Direction Rules

Allowed imports inside `internal/lifecycle`:

```text
context
errors
fmt
sync
```

Additional standard library imports may be used in tests only when needed:

```text
strings
sync
sync/atomic
testing
time
```

Use only imports that are actually needed.

The lifecycle package must not import:

```text
internal/config
internal/logger
cmd/plomvix
net/http
os
os/signal
syscall
database/sql
```

No external dependencies are allowed.

---

## Enterprise Hardening Goals

This plan adds:

* lifecycle state enum
* state inspection API
* stable state transition rules
* duplicate component name rejection
* component panic recovery for `Start`
* component panic recovery for `Stop`
* clearer lifecycle errors
* lifecycle documentation hardening
* enterprise lifecycle tests
* race verification

---

## Non-Goals

Do not implement:

* lifecycle wiring in `cmd/plomvix/main.go`
* OS signal handling
* graceful process shutdown
* background service runner
* goroutine supervision
* health checks
* readiness checks
* dependency graph sorting
* component priority
* parallel start
* parallel stop
* restart behavior
* hot reload
* dynamic component unregister
* lifecycle hooks
* event bus
* metrics
* logger integration
* config integration
* OpenTelemetry
* WAL integration
* storage integration
* query integration
* API server integration
* metrics engine
* logs engine
* relational engine
* UI

Enterprise hardening here means stronger correctness and safety, not broader system orchestration.

---

# Final Public API Additions

The existing API must remain compatible.

Add lifecycle state type:

```go
type State string
```

Add state constants:

```go
const (
	StateNew      State = "new"
	StateStarting State = "starting"
	StateStarted  State = "started"
	StateStopping State = "stopping"
	StateStopped  State = "stopped"
	StateFailed   State = "failed"
)
```

Add public errors:

```go
var (
	ErrDuplicateComponent = errors.New("lifecycle: duplicate component")
	ErrInvalidState       = errors.New("lifecycle: invalid state")
)
```

Add method:

```go
func (m *Manager) State() State
```

Existing API must remain:

```go
func NewManager() *Manager
func (m *Manager) Register(component Component) error
func (m *Manager) Start(ctx context.Context) error
func (m *Manager) Stop(ctx context.Context) error
```

Do not remove or rename existing public errors.

Do not remove or rename existing methods.

---

# Final Enterprise Lifecycle Behavior

## State Rules

Initial manager state:

```text
new
```

State transition rules:

```text
new -> starting -> started
new -> starting -> failed
started -> stopping -> stopped
failed -> stopping -> stopped
new -> stopped
```

Meaning:

* `new`: manager created, registration allowed
* `starting`: start has begun, registration rejected
* `started`: all registered components started successfully
* `failed`: start failed or component panic occurred during start
* `stopping`: stop is in progress
* `stopped`: stop completed, even if one or more components returned stop errors

Important rules:

* Registration is allowed only in `new`.
* Registration after state leaves `new` is rejected.
* `Start(ctx)` is allowed only from `new`.
* Repeated `Start(ctx)` after state leaves `new` returns an error matching `ErrAlreadyStarted`.
* `Stop(ctx)` is safe from `new`, `started`, `failed`, and `stopped`.
* `Stop(ctx)` from `new` transitions to `stopped` and returns nil.
* `Stop(ctx)` from `failed` stops only successfully started components.
* `Stop(ctx)` from `stopped` returns nil.
* `Stop(ctx)` from `starting` returns an error matching `ErrInvalidState`.
* `Stop(ctx)` from `stopping` returns an error matching `ErrInvalidState`.
* Calling `Stop` while `Start` is in progress returns `ErrInvalidState`.
* Callers should wait for `Start` to complete before calling `Stop`.
* If `Stop(ctx)` returns an error, state is still `stopped`.

---

## Duplicate Component Names

`Register(component)` must reject duplicate component names.

Duplicate comparison is exact and case-sensitive.

Example:

```text
storage
storage
```

is duplicate.

Example:

```text
storage
Storage
```

is not duplicate in this plan.

Duplicate rejection must return an error matching:

```go
ErrDuplicateComponent
```

Duplicate error must include the component name.

Do not add component name normalization.

Do not add reserved names.

---

## Panic Recovery

Lifecycle must recover from component panics during `Start(ctx)` and `Stop(ctx)`.

### Start Panic Rule

If a component panics during `Start(ctx)`:

* recover the panic
* stop starting immediately
* return an error
* returned error must include the component name
* returned error must include the word `panic`
* manager state becomes `failed`
* only components successfully started before the panic are eligible for stop

### Stop Panic Rule

If a component panics during `Stop(ctx)`:

* recover the panic
* continue stopping remaining eligible components
* returned error must include the component name
* returned error must include the word `panic`
* manager state becomes `stopped`
* repeated `Stop(ctx)` must not call component `Stop` again

Do not re-panic.

Do not call `os.Exit`.

Do not add logger usage.

---

## Error Rules

Existing errors must remain compatible with `errors.Is`.

Required public errors:

```go
ErrNilComponent
ErrEmptyComponentName
ErrAlreadyStarted
ErrDuplicateComponent
ErrInvalidState
```

Error message requirements:

* duplicate component error must include component name
* start failure error must include component name
* start panic error must include component name and `panic`
* stop failure error must include component name
* stop panic error must include component name and `panic`
* invalid state errors must match `ErrInvalidState`

Use `fmt.Errorf("...: %w", err)` where useful.

Do not introduce custom error structs yet.

---

## State Safety

`Manager` must continue protecting state with `sync.Mutex`.

Required internal state may be similar to:

```go
type Manager struct {
	mu                sync.Mutex
	components        []Component
	componentNames    map[string]struct{}
	startedComponents []Component
	state             State
}
```

Rules:

* all fields remain unexported
* no package-level manager
* no global mutable lifecycle state
* `State()` must lock before reading state
* `State()` must check for nil receiver before acquiring the mutex
* `Register`, `Start`, and `Stop` must protect shared state
* avoid holding the mutex while calling external component `Start(ctx)` or `Stop(ctx)`

Required lock pattern:

```text
lock -> read/set manager state -> copy required slices -> unlock -> call component -> lock -> update manager state -> unlock
```

Do not hold the manager mutex while calling component `Start(ctx)` or `Stop(ctx)`.

---

# Final Expected Structure

Expected files after this plan:

```text
plomvix/
├── internal/
│   └── lifecycle/
│       ├── lifecycle.go
│       └── lifecycle_test.go
├── docs/
│   └── lifecycle.md
├── lifecycle.md
└── lifecycle_enterprise.md
```

Clarification:

```text
The root lifecycle_enterprise.md file is this plan file, not generated runtime documentation.
```

Do not create:

```text
internal/wal/
internal/storage/
internal/query/
internal/api/
internal/server/
internal/ui/
internal/engine/
```

---

# Task Plan

---

## TASK 01 — Add lifecycle state type and constants

### Goal

Introduce explicit lifecycle states.

### Files

Modify:

```text
internal/lifecycle/lifecycle.go
```

### Requirements

Add:

```go
type State string
```

Add constants:

```go
const (
	StateNew      State = "new"
	StateStarting State = "starting"
	StateStarted  State = "started"
	StateStopping State = "stopping"
	StateStopped  State = "stopped"
	StateFailed   State = "failed"
)
```

Update `Manager` to store:

```go
state State
```

Update `NewManager()` so new managers start in:

```go
StateNew
```

Do not add `State()` method yet.

Do not change start/stop behavior yet except as needed to compile.

Keep existing tests passing.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

Report:

```text
TASK 01 completed.
Files changed:
- internal/lifecycle/lifecycle.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 02 — Add lifecycle state tests

### Goal

Verify lifecycle state constants are stable and manager starts in the correct state later.

### Files

Modify:

```text
internal/lifecycle/lifecycle_test.go
```

### Requirements

Use existing external test package:

```go
package lifecycle_test
```

Add tests confirming state constants:

```text
StateNew == "new"
StateStarting == "starting"
StateStarted == "started"
StateStopping == "stopping"
StateStopped == "stopped"
StateFailed == "failed"
```

Add a test comment:

```go
// These values are part of the stable lifecycle API.
```

Do not add `State()` method tests yet.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

Report:

```text
TASK 02 completed.
Files changed:
- internal/lifecycle/lifecycle_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 03 — Add State method

### Goal

Expose safe read-only lifecycle state inspection.

### Files

Modify:

```text
internal/lifecycle/lifecycle.go
```

### Requirements

Add:

```go
func (m *Manager) State() State
```

Behavior:

* if `m == nil`, return `StateFailed`
* check for nil receiver before acquiring the mutex
* do not call `m.mu.Lock()` before the nil receiver check
* lock manager mutex after nil check
* return current state
* do not mutate state
* do not expose internal fields
* do not return empty state for a manager created by `NewManager()`

Required shape:

```go
func (m *Manager) State() State {
	if m == nil {
		return StateFailed
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	return m.state
}
```

Reason:

* Nil manager state inspection should not panic.
* `StateFailed` is the safest conservative state.

Do not add logger usage.

Do not add config usage.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

Report:

```text
TASK 03 completed.
Files changed:
- internal/lifecycle/lifecycle.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 04 — Add State method tests

### Goal

Test state inspection behavior.

### Files

Modify:

```text
internal/lifecycle/lifecycle_test.go
```

### Requirements

Add tests for:

* new manager state is `lifecycle.StateNew`
* nil manager `State()` returns `lifecycle.StateFailed`

Nil manager test shape:

```go
var m *lifecycle.Manager
if got := m.State(); got != lifecycle.StateFailed {
	t.Fatalf(...)
}
```

Do not test all transitions yet.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

Report:

```text
TASK 04 completed.
Files changed:
- internal/lifecycle/lifecycle_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 05 — Replace started/stopped booleans with state transitions

### Goal

Make lifecycle state transitions explicit.

### Files

Modify:

```text
internal/lifecycle/lifecycle.go
```

### Requirements

Refactor manager state handling to use `state State`.

Add:

```go
ErrInvalidState = errors.New("lifecycle: invalid state")
```

Rules:

* `NewManager()` initializes `state = StateNew`.
* `Register` is allowed only when state is `StateNew`.
* `Register` after state leaves `StateNew` returns error matching `ErrAlreadyStarted`.
* `Start(ctx)` is allowed only when state is `StateNew`.
* `Start(ctx)` sets state to `StateStarting` before component iteration.
* Successful full start sets state to `StateStarted`.
* Failed component start sets state to `StateFailed`.
* Start panic recovery is not implemented yet.
* `Stop(ctx)` from `StateNew` sets state to `StateStopped` and returns nil.
* `Stop(ctx)` from `StateStarted` or `StateFailed` sets state to `StateStopping`, then `StateStopped`.
* `Stop(ctx)` from `StateStopped` returns nil.
* `Stop(ctx)` from `StateStarting` returns error matching `ErrInvalidState`.
* `Stop(ctx)` from `StateStopping` returns error matching `ErrInvalidState`.

Required `Start(ctx)` lock/unlock pattern:

```text
1. Lock manager.
2. Confirm state is StateNew.
3. Set state to StateStarting.
4. Copy registered components to a local variable.
5. Unlock manager.
6. Call component Start(ctx) without holding the mutex.
7. After each successful component start, lock manager and record it in startedComponents.
8. If a component fails, lock manager and set state to StateFailed.
9. If all components start successfully, lock manager and set state to StateStarted.
```

Required `Stop(ctx)` lock/unlock pattern:

```text
1. Lock manager.
2. Inspect state.
3. If state is StateNew, set state to StateStopped, unlock, and return nil.
4. If state is StateStopped, unlock and return nil.
5. If state is StateStarting or StateStopping, unlock and return ErrInvalidState.
6. If state is StateStarted or StateFailed, copy startedComponents to a local variable.
7. Set state to StateStopping.
8. Clear startedComponents on the manager.
9. Unlock manager.
10. Iterate the local copied startedComponents slice in reverse order.
11. Call component Stop(ctx) without holding the mutex.
12. After stop iteration completes, lock manager and set state to StateStopped.
```

Important clarification:

```text
Do not iterate m.startedComponents after clearing it.
Always copy startedComponents to a local variable before clearing the manager field.
```

Concurrent stop safety depends on setting `StateStopping` before releasing the mutex and beginning component stop iteration.

Concurrent stop callers that observe `StateStopping` must return an error matching `ErrInvalidState` and must not call component `Stop`.

Do not add duplicate component handling yet.

Do not add panic recovery yet.

Important mutex rule:

* protect all state reads/writes with mutex
* avoid holding mutex during component calls
* do not hold the mutex while calling external component `Start(ctx)` or `Stop(ctx)`

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

Report:

```text
TASK 05 completed.
Files changed:
- internal/lifecycle/lifecycle.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 06 — Add lifecycle state transition tests

### Goal

Verify explicit state transitions.

### Files

Modify:

```text
internal/lifecycle/lifecycle_test.go
```

### Requirements

Reuse the existing fake component type.

Do not create duplicate fake component types.

Add tests for:

* new manager starts as `StateNew`
* successful start transitions to `StateStarted`
* failed start transitions to `StateFailed`
* stop before start transitions to `StateStopped`
* stop after successful start transitions to `StateStopped`
* stop after failed start transitions to `StateStopped`
* repeated stop remains `StateStopped`
* repeated start after successful start returns `ErrAlreadyStarted`
* repeated start after failed start returns `ErrAlreadyStarted`

Use `errors.Is` for error checks.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

Report:

```text
TASK 06 completed.
Files changed:
- internal/lifecycle/lifecycle_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 07 — Add duplicate component name rejection

### Goal

Prevent accidental duplicate lifecycle component names.

### Files

Modify:

```text
internal/lifecycle/lifecycle.go
```

### Requirements

Add public error:

```go
ErrDuplicateComponent = errors.New("lifecycle: duplicate component")
```

Update manager to track registered names.

Allowed internal shape:

```go
componentNames map[string]struct{}
```

Rules:

* initialize `componentNames` in `NewManager()`
* reject duplicate component names in `Register`
* duplicate check is exact and case-sensitive
* duplicate error must match `ErrDuplicateComponent`
* duplicate error must include the duplicate component name
* do not normalize names
* do not add reserved names
* do not mutate component names

Example error shape:

```go
fmt.Errorf("lifecycle: duplicate component %q: %w", name, ErrDuplicateComponent)
```

Do not add duplicate detection during `Start`.

Registration remains the only place that checks this.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

Report:

```text
TASK 07 completed.
Files changed:
- internal/lifecycle/lifecycle.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 08 — Add duplicate component tests

### Goal

Verify duplicate component name rejection.

### Files

Modify:

```text
internal/lifecycle/lifecycle_test.go
```

### Requirements

Reuse the existing fake component type.

Add tests for:

* registering duplicate component name returns error matching `lifecycle.ErrDuplicateComponent`
* duplicate component error includes duplicate name
* duplicate check is case-sensitive

Case-sensitive scenario:

```text
storage
Storage
```

Expected:

```text
both registrations succeed
```

Do not add name normalization.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

Report:

```text
TASK 08 completed.
Files changed:
- internal/lifecycle/lifecycle_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 09 — Add panic recovery for Start

### Goal

Make component start panics safe and deterministic.

### Files

Modify:

```text
internal/lifecycle/lifecycle.go
```

### Requirements

Recover from panics during component `Start(ctx)`.

Behavior:

* if component `Start(ctx)` panics, recover
* stop starting immediately
* return an error
* error must include component name
* error must include word `panic`
* state must become `StateFailed`
* only components successfully started before the panic remain eligible for stop
* do not re-panic
* do not call `os.Exit`
* do not add logger usage

Implementation approach:

Add small unexported helper if useful:

```go
func startComponent(ctx context.Context, component Component) (err error)
```

Helper may use named return and `defer recover`.

Helper rules:

* `startComponent` must not acquire the manager mutex
* it is called only after the manager mutex has already been released
* do not lock manager state inside this helper
* helper should only call the component and recover panic

Named return recovery pattern:

```go
func startComponent(ctx context.Context, component Component) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("lifecycle: start component %q panic: %v", component.Name(), r)
		}
	}()

	return component.Start(ctx)
}
```

Do not expose panic helper.

Do not change component interface.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

Report:

```text
TASK 09 completed.
Files changed:
- internal/lifecycle/lifecycle.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 10 — Add Start panic recovery tests

### Goal

Verify start panic recovery behavior.

### Files

Modify:

```text
internal/lifecycle/lifecycle_test.go
```

### Requirements

Extend existing fake component type if needed.

Do not create duplicate fake component types.

Add tests for:

* start panic returns error
* start panic error includes component name
* start panic error includes `panic`
* start panic transitions state to `StateFailed`
* components after panic are not started
* stop after start panic stops only components started before panic

Important scenario:

```text
Register A, B, C.
A starts successfully.
B panics during Start.
C is never started.
Stop should only stop A.
```

Use stable substring checks for component name and `panic`.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

Report:

```text
TASK 10 completed.
Files changed:
- internal/lifecycle/lifecycle_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 11 — Add panic recovery for Stop

### Goal

Make component stop panics safe and deterministic.

### Files

Modify:

```text
internal/lifecycle/lifecycle.go
```

### Requirements

Recover from panics during component `Stop(ctx)`.

Behavior:

* if component `Stop(ctx)` panics, recover
* continue stopping remaining eligible components
* return an error
* error must include component name
* error must include word `panic`
* multiple stop failures and stop panics may be combined with `errors.Join`
* state must become `StateStopped`
* repeated `Stop(ctx)` must not call component `Stop` again
* do not re-panic
* do not call `os.Exit`
* do not add logger usage

Implementation approach:

Add small unexported helper if useful:

```go
func stopComponent(ctx context.Context, component Component) (err error)
```

Helper may use named return and `defer recover`.

Helper rules:

* `stopComponent` must not acquire the manager mutex
* it is called only after the manager mutex has already been released
* do not lock manager state inside this helper
* helper should only call the component and recover panic

Named return recovery pattern:

```go
func stopComponent(ctx context.Context, component Component) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("lifecycle: stop component %q panic: %v", component.Name(), r)
		}
	}()

	return component.Stop(ctx)
}
```

Do not expose panic helper.

Do not change component interface.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

Report:

```text
TASK 11 completed.
Files changed:
- internal/lifecycle/lifecycle.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 12 — Add Stop panic recovery tests

### Goal

Verify stop panic recovery behavior.

### Files

Modify:

```text
internal/lifecycle/lifecycle_test.go
```

### Requirements

Extend existing fake component type if needed.

Do not create duplicate fake component types.

Add tests for:

* stop panic returns error
* stop panic error includes component name
* stop panic error includes `panic`
* stop continues after one component panics
* stop panic transitions state to `StateStopped`
* repeated stop after stop panic returns nil
* repeated stop after stop panic does not call component `Stop` again
* stop can join normal stop errors and panic errors

Important scenario:

```text
Register A, B, C.
Start succeeds for all.
Stop runs in reverse order: C, then B, then A.
C panics during Stop.
B returns stop error.
A stops successfully.
Stop returns an error containing C, panic, and B.
State is stopped.
Second Stop returns nil.
No component Stop is called twice.
```

Use stable substring checks.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

Report:

```text
TASK 12 completed.
Files changed:
- internal/lifecycle/lifecycle_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 13 — Harden lifecycle documentation

### Goal

Document enterprise lifecycle behavior and state transitions.

### Files

Modify:

```text
docs/lifecycle.md
```

### Requirements

Update docs to include:

* lifecycle states
* state transitions
* duplicate component rejection
* panic recovery during start
* panic recovery during stop
* stop remains idempotent after stop errors
* stop remains idempotent after stop panics
* mutex-protected manager state
* registration after start is rejected
* `Stop` during `StateStarting` behavior
* enterprise lifecycle non-goals

The documentation must include this exact sentence:

```text
Calling Stop while Start is in progress returns ErrInvalidState. Callers should wait for Start to complete before calling Stop.
```

The documentation must include these exact strings because TASK 14 checks them:

```text
lifecycle states
state transitions
duplicate component
panic recovery
start panic
stop panic
stop remains idempotent
mutex-protected manager state
registration after start is rejected
Stop while Start is in progress returns ErrInvalidState
```

Non-goals section must clearly say lifecycle does not implement:

```text
signal handling
WAL
storage
query engine
API server
UI
logger integration
config integration
```

Do not document future behavior as already implemented.

Do not mention file output or logger internals beyond saying logger integration is not implemented.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

Report:

```text
TASK 13 completed.
Files changed:
- docs/lifecycle.md

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 14 — Harden lifecycle documentation tests

### Goal

Verify enterprise lifecycle documentation statements.

### Files

Modify:

```text
internal/lifecycle/lifecycle_test.go
```

### Requirements

Update the existing documentation test that reads:

```go
os.ReadFile("../../docs/lifecycle.md")
```

This path assumes the test runs from `internal/lifecycle/`, which is the default behavior of `go test ./...`.

Test that the document contains these exact strings:

```text
# Plomvix Lifecycle
component interface
manager
registration order
start order
reverse stop order
context-aware
start error
stop error
stop idempotency
registration after start is rejected
mutex-protected manager state
lifecycle states
state transitions
duplicate component
panic recovery
start panic
stop panic
stop remains idempotent
Stop while Start is in progress returns ErrInvalidState
signal handling
WAL
storage
query engine
API server
UI
logger integration
config integration
```

Use stable substring checks.

Do not make fragile checks for full paragraphs.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

Report:

```text
TASK 14 completed.
Files changed:
- internal/lifecycle/lifecycle_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 15 — Add lifecycle race-focused tests

### Goal

Add minimal concurrency safety coverage for lifecycle state inspection and stop idempotency.

### Files

Modify:

```text
internal/lifecycle/lifecycle_test.go
```

### Requirements

Add race-focused tests using only the standard library.

Tests should remain deterministic.

Use `sync/atomic` or a `sync.Mutex` in the fake component to protect call counters used by concurrent tests.

Plain int counters must not be read or written concurrently.

Add test:

```text
concurrent State calls are safe
```

Shape:

* create manager
* start several goroutines that call `State()`
* wait for them
* test should pass under `go test -race ./...`

Add test:

```text
concurrent repeated Stop calls are safe after start
```

Shape:

* create manager
* register one fake component
* start manager
* run several goroutines that call `Stop(ctx)`
* wait for them
* component `Stop` call count must be exactly 1
* test should pass under `go test -race ./...`

Important dependency:

```text
The concurrent stop safety test relies on Stop setting state to StateStopping before releasing the mutex and beginning component stop iteration.
```

Concurrent Stop callers that observe `StateStopping` may return an error matching `ErrInvalidState`.

The test must not treat `ErrInvalidState` as a test failure.

Only unexpected errors should fail the test.

The assertion is that component Stop call count equals exactly 1, not that all concurrent Stop calls return nil.

Important:

* Fake component call counters used by concurrent tests must be protected with `sync/atomic` or `sync.Mutex`.
* Do not use plain int counters for concurrently accessed values.
* Do not introduce sleeps as synchronization.
* Use `sync.WaitGroup`.
* Do not add goroutines to production lifecycle code.

### Verification

Run:

```bash
go test ./...
go test -race ./...
go build ./...
```

### Completion Report

Report:

```text
TASK 15 completed.
Files changed:
- internal/lifecycle/lifecycle_test.go

Verification:
- go test ./...
- go test -race ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 16 — Final enterprise lifecycle review

### Goal

Review enterprise lifecycle hardening for correctness, race safety, scope control, and project cleanliness.

### Files

Review only unless fixes are required:

```text
internal/lifecycle/lifecycle.go
internal/lifecycle/lifecycle_test.go
docs/lifecycle.md
go.mod
go.sum
```

### Requirements

Confirm:

* package is `internal/lifecycle`
* existing public API remains compatible
* `State` type exists
* state constants exist
* `State()` exists
* `State()` checks nil receiver before mutex access
* new manager state is `StateNew`
* nil manager `State()` returns `StateFailed`
* lifecycle state transitions are explicit
* `ErrDuplicateComponent` exists
* `ErrInvalidState` exists
* duplicate component names are rejected
* duplicate check is case-sensitive
* duplicate error includes component name
* duplicate error matches `ErrDuplicateComponent`
* start uses lock/unlock pattern around component calls
* stop uses lock/unlock pattern around component calls
* stop copies `startedComponents` to a local variable before clearing manager state
* stop sets `StateStopping` before releasing mutex and stopping components
* concurrent stop calls do not double-call component `Stop`
* concurrent stop tests allow expected `ErrInvalidState` responses
* start panic is recovered
* start panic error includes component name
* start panic error includes `panic`
* start panic transitions to `StateFailed`
* stop after start panic stops only successfully started components
* stop panic is recovered
* stop panic error includes component name
* stop panic error includes `panic`
* stop continues after panic
* stop joins normal errors and panic errors
* stop after panic is idempotent
* stop after error is idempotent
* state is protected with `sync.Mutex`
* `Register`, `Start`, `Stop`, and `State` protect shared state
* panic helper functions do not acquire the manager mutex
* fake component counters used by race tests are protected with `sync/atomic` or `sync.Mutex`
* no external dependencies were added
* no logger dependency was added
* no config dependency was added
* no `cmd/plomvix/main.go` wiring was added
* no signal handling was added
* no WAL was added
* no storage was added
* no query engine was added
* no API server was added
* no UI was added
* no workload engines were added
* docs exist
* docs tests exist
* enterprise docs strings are covered
* `go.mod` has no new dependency for lifecycle
* `go.sum` has no new dependency entries for lifecycle

If issues are found:

1. Fix them.
2. Run final verification again.
3. Report what was fixed.

### Final Verification

Run:

```bash
go test ./...
go build ./...
go test -race ./...
go mod tidy
go test ./...
```

### Completion Report

Report:

```text
TASK 16 completed.
Files reviewed:
- internal/lifecycle/lifecycle.go
- internal/lifecycle/lifecycle_test.go
- docs/lifecycle.md
- go.mod
- go.sum

Final verification:
- go test ./...
- go build ./...
- go test -race ./...
- go mod tidy
- go test ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable

Final status:
- enterprise lifecycle hardening complete
- safe to move to next core foundation feature
```

---

# Completion Criteria

This plan is complete only when:

* `internal/lifecycle/lifecycle.go` has explicit state support
* `internal/lifecycle/lifecycle_test.go` covers state transitions
* duplicate component registration is rejected
* start panic recovery is implemented and tested
* stop panic recovery is implemented and tested
* lifecycle documentation is hardened
* lifecycle documentation test passes
* race-focused tests exist
* `go test ./...` passes
* `go build ./...` passes
* `go test -race ./...` passes
* `go mod tidy` produces no unwanted dependency changes
* final `go test ./...` passes
* no non-goal systems were introduced

---

# Recommended Next Step After Completion

After `lifecycle_enterprise.md` is completed and verified, the next feature area should still remain core foundation.

Do not automatically move to WAL or storage unless the project owner confirms that lifecycle enterprise hardening is complete and the next feature area is selected.

Possible next core foundation feature after enterprise lifecycle:

```text
metadata foundation
```

But do not create that plan until lifecycle enterprise hardening is completed and reviewed.
