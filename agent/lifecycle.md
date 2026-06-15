# lifecycle.md

# Plomvix Lifecycle Foundation Plan

## Purpose

Create the first minimal lifecycle foundation for Plomvix.

This plan adds a small `internal/lifecycle` package that can register components, start them in order, stop them in reverse order, handle context-aware shutdown, protect lifecycle state with a mutex, and provide predictable error behavior.

This is a core foundation feature only.

Do not wire lifecycle into `cmd/plomvix/main.go` yet.

Do not add WAL, storage, query engine, API server, UI, workload engines, signal handling, or background service orchestration.

---

## Current Project State

Completed foundation work:

```text
config foundation: done
basic logger setup: done
enterprise logger hardening: done
```

Current stage:

```text
core foundation only
```

Next feature area:

```text
lifecycle foundation
```

---

## Go Version Requirement

Plomvix uses:

```text
Go 1.22 or later
```

This plan may use standard library features available in Go 1.22.

`errors.Join` is available in Go 1.20+ and is allowed in this plan.

---

## Coding Agent

Coding agent:

```text
DeepSeek V4 Pro
```

If the local environment uses a different exact DeepSeek model identifier, use the configured DeepSeek coding model available there.

Tasks must be executed one at a time, in exact order.

Do not proceed to the next task until the current task passes its verification.

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
* Do not wire lifecycle into the application entrypoint yet.
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

Use only imports that are actually needed.

The lifecycle package must not import:

```text
internal/config
internal/logger
cmd/plomvix
net/http
os/signal
syscall
database/sql
```

No external dependencies are allowed.

---

## Design Decision: Context-Aware Lifecycle

Lifecycle components use context directly in their interface.

Final component interface:

```go
type Component interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}
```

Reason:

* Startup may need cancellation later.
* Shutdown must be context-aware from the beginning.
* The manager does not need to own or create contexts.
* Callers decide timeout/cancellation policy.
* This keeps lifecycle reusable across future engines and services.

---

## Final Public API

Package:

```text
internal/lifecycle
```

Final public API:

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

Public package errors:

```go
var (
	ErrNilComponent       = errors.New("lifecycle: nil component")
	ErrEmptyComponentName = errors.New("lifecycle: empty component name")
	ErrAlreadyStarted    = errors.New("lifecycle: already started")
)
```

Error wrapping rules:

* `Register(nil)` must return an error matching `ErrNilComponent`.
* Empty component name must return an error matching `ErrEmptyComponentName`.
* Registering after start has begun must return an error matching `ErrAlreadyStarted`.
* Component start failure must include the component name.
* Component stop failure must include the component name.
* Multiple stop errors may be combined with `errors.Join`.

---

## Lifecycle State Safety

`Manager` must protect its internal state with `sync.Mutex`.

The following methods mutate or read shared manager state and must lock around that state:

```text
Register
Start
Stop
```

Required manager shape may be similar to:

```go
type Manager struct {
	mu                sync.Mutex
	components        []Component
	startedComponents []Component
	started           bool
	stopped           bool
}
```

Rules:

* All fields must remain unexported.
* Do not expose the mutex.
* Do not introduce package-level lifecycle state.
* Do not use global manager instances.
* Do not add goroutines.
* Avoid holding the mutex while calling external component `Start(ctx)` or `Stop(ctx)` if it creates unnecessary deadlock risk.
* If the implementation unlocks around component calls, it must still preserve correct lifecycle state transitions.

Final verification must include:

```bash
go test -race ./...
```

---

## Lifecycle Behavior

### Registration

`Register(component)` must:

* reject nil components
* reject components with empty `Name()`
* reject registration once lifecycle start has begun
* preserve registration order
* not start the component
* not stop the component
* not mutate component names
* protect manager state with `sync.Mutex`

Registration after start is rejected.

Exact phrase required in docs:

```text
registration after start is rejected
```

---

### Start

`Start(ctx)` must:

* start registered components in registration order
* mark lifecycle as started before attempting to start any component
* reject future registration even if start fails partway through
* stop starting immediately after the first start error
* return the first start error with component name context
* track only successfully started components as eligible for stop
* be safe if called with no registered components
* protect manager state with `sync.Mutex`

Important rule:

```text
Mark the lifecycle as started before attempting to start any component, so that registration is rejected even if start fails partway through.
```

Repeated `Start(ctx)` after lifecycle has started must return an error matching `ErrAlreadyStarted`.

---

### Stop

`Stop(ctx)` must:

* stop only successfully started components
* stop components in reverse successful-start order
* attempt to stop all eligible components even if one stop fails
* include component names in stop errors
* combine multiple stop errors with `errors.Join`
* be safe if called before start
* be safe if called after successful stop
* be safe if called after failed stop
* protect manager state with `sync.Mutex`

Stop idempotency rule:

```text
If Stop(ctx) returns an error, the manager is still considered stopped. Repeated calls after a failed stop must be safe and must not call component Stop again.
```

Repeated calls after successful or failed stop should return `nil`.

Reason:

* Shutdown should be best-effort.
* A failed component stop should not cause repeated shutdown attempts to double-call already stopped components.
* Future callers should be able to safely defer `Stop(ctx)` without complex state handling.

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
* WAL integration
* storage integration
* query integration
* API server integration
* metrics engine
* logs engine
* relational engine
* UI
* OpenTelemetry
* config-driven lifecycle

---

## Final Expected Structure

Expected files after this plan:

```text
plomvix/
├── internal/
│   └── lifecycle/
│       ├── lifecycle.go
│       └── lifecycle_test.go
├── docs/
│   └── lifecycle.md
└── lifecycle.md
```

Clarification:

```text
The root lifecycle.md file is this plan file, not a generated output from the lifecycle package.
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

## TASK 01 — Create lifecycle package, component interface, manager skeleton

### Goal

Create the initial lifecycle package with the final public API shape.

### Files

Create:

```text
internal/lifecycle/lifecycle.go
```

### Requirements

Add package:

```go
package lifecycle
```

Add imports as needed:

```go
context
errors
```

Define:

```go
type Component interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}
```

Define exported errors:

```go
var (
	ErrNilComponent       = errors.New("lifecycle: nil component")
	ErrEmptyComponentName = errors.New("lifecycle: empty component name")
	ErrAlreadyStarted    = errors.New("lifecycle: already started")
)
```

Define manager:

```go
type Manager struct {
	// unexported fields only
}
```

Define constructor:

```go
func NewManager() *Manager
```

Define methods:

```go
func (m *Manager) Register(component Component) error
func (m *Manager) Start(ctx context.Context) error
func (m *Manager) Stop(ctx context.Context) error
```

Stub behavior:

* `NewManager()` must return a non-nil manager.
* Method stubs must return `nil`, not panic.
* Do not implement full behavior yet.
* Do not add tests yet.
* Do not add mutex yet in this task.
* Do not wire into `cmd/plomvix/main.go`.

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

## TASK 02 — Add manager registration behavior

### Goal

Implement component registration rules.

### Files

Modify:

```text
internal/lifecycle/lifecycle.go
```

### Requirements

Update `Manager` to store registered components in order.

Implement `Register(component Component) error`.

Rules:

* Reject nil component with `ErrNilComponent`.
* Reject empty component name with `ErrEmptyComponentName`.
* Reject registration after lifecycle start has begun with `ErrAlreadyStarted`.
* Preserve registration order.
* Do not call `Start`.
* Do not call `Stop`.
* Do not mutate component name.
* Use `errors.Is` compatible wrapping if additional context is added.
* Keep fields unexported.

`Register` should be safe for normal single-threaded use in this task.

Full mutex protection is added explicitly in TASK 04 when `Start(ctx)` begins mutating lifecycle state.

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
- internal/lifecycle/lifecycle.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 03 — Add registration tests

### Goal

Test registration validation and ordering behavior.

### Files

Create:

```text
internal/lifecycle/lifecycle_test.go
```

### Requirements

Use external test package:

```go
package lifecycle_test
```

Create a deterministic fake component type in this test file.

The fake component type created here will be reused in all subsequent test tasks.

Do not create duplicate fake component types in the same file.

The fake should support:

* configurable name
* start call recording
* stop call recording
* optional start error
* optional stop error
* captured start context
* captured stop context

Add tests for:

* `NewManager()` returns non-nil manager
* `Register(nil)` returns an error matching `lifecycle.ErrNilComponent`
* empty component name returns an error matching `lifecycle.ErrEmptyComponentName`
* valid component registration succeeds
* registration does not call `Start`
* registration does not call `Stop`

Do not test start ordering yet.

Do not test stop behavior yet.

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
- internal/lifecycle/lifecycle_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 04 — Add start behavior and mutex state protection

### Goal

Implement lifecycle start behavior and add mutex protection for manager state.

### Files

Modify:

```text
internal/lifecycle/lifecycle.go
```

### Requirements

Add `sync.Mutex` to `Manager`.

Protect manager state in:

```text
Register
Start
Stop
```

At this task, `Stop` may still have minimal behavior, but any state it reads or mutates must be protected.

Required manager shape may be similar to:

```go
type Manager struct {
	mu                sync.Mutex
	components        []Component
	startedComponents []Component
	started           bool
	stopped           bool
}
```

Implement `Start(ctx context.Context) error`.

Rules:

* Start registered components in registration order.
* Mark lifecycle as started before attempting to start any component.
* Registration must be rejected after start begins, even if start fails partway through.
* Repeated `Start(ctx)` after start has begun must return an error matching `ErrAlreadyStarted`.
* If a component fails to start, stop starting immediately.
* Return an error that includes the failed component name.
* Preserve successfully started components for later stop behavior.
* Only successfully started components are later eligible for stop.
* If there are no components, `Start(ctx)` should succeed and mark lifecycle as started.
* Pass the provided context to each component `Start(ctx)` call.
* Do not call `Stop` from `Start`.
* Do not add logger usage.
* Do not add goroutines.

Important concurrency rule:

* Do not hold the mutex while calling external component `Start(ctx)` if avoidable.
* Copy the registered component slice under lock, mark `started = true`, then release the lock before calling component `Start(ctx)`.
* When a component starts successfully, record it as successfully started under lock.
* This keeps manager state protected while reducing deadlock risk.

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
- internal/lifecycle/lifecycle.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 05 — Add start behavior tests

### Goal

Test lifecycle start ordering and start error behavior.

### Files

Modify:

```text
internal/lifecycle/lifecycle_test.go
```

### Requirements

Reuse the fake component type created in TASK 03.

Do not create duplicate fake component types.

Add tests for:

* components start in registration order
* `Start(ctx)` passes context to components
* start stops at first error
* failed start error includes failed component name
* components after the failed component are not started
* repeated start returns an error matching `lifecycle.ErrAlreadyStarted`
* starting with no registered components succeeds

Do not test stop eligibility here.

Stop eligibility is tested in TASK 07 after stop behavior exists.

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
- internal/lifecycle/lifecycle_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 06 — Add stop behavior

### Goal

Implement lifecycle stop behavior.

### Files

Modify:

```text
internal/lifecycle/lifecycle.go
```

### Requirements

Implement `Stop(ctx context.Context) error`.

Rules:

* Stop only successfully started components.
* Stop components in reverse successful-start order.
* Pass the provided context to each component `Stop(ctx)` call.
* Attempt to stop all eligible components even if one stop fails.
* Include component name in each stop error.
* Combine multiple stop errors with `errors.Join`.
* `errors.Join` is available in Go 1.20+ and is allowed under this Go 1.22 plan.
* Stop before start must return `nil`.
* Stop after successful stop must return `nil`.
* Stop after failed stop must be safe.
* If `Stop(ctx)` returns an error, the manager is still considered stopped.
* Repeated calls after failed stop must not call component `Stop` again.
* Repeated calls after failed stop should return `nil`.
* Repeated calls after successful stop should return `nil`.
* Protect manager state with `sync.Mutex`.
* Do not add goroutines.
* Do not add logger usage.
* Do not call `Start` from `Stop`.

Important concurrency rule:

* Take a copy of `startedComponents` under lock.
* Mark manager as stopped under lock before external component `Stop(ctx)` calls.
* Clear `startedComponents` under lock before or after the stop attempt.
* Do not hold the mutex while calling external component `Stop(ctx)` if avoidable.

A simple safe approach:

```text
If already stopped, return nil.
Copy started components.
Mark stopped.
Clear started components.
Release lock.
Iterate copied components in reverse order.
Collect stop errors.
Return joined error if any.
```

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
- internal/lifecycle/lifecycle.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 07 — Add stop behavior tests

### Goal

Test lifecycle stop ordering, stop idempotency, and stop error behavior.

### Files

Modify:

```text
internal/lifecycle/lifecycle_test.go
```

### Requirements

Reuse the fake component type created in TASK 03.

Do not create duplicate fake component types.

Add tests for:

* stop before start returns nil
* stop uses reverse successful-start order
* stop passes context to components
* stop only stops successfully started components
* stop attempts all eligible components even if one stop fails
* stop error includes failed component name
* multiple stop errors include all failed component names
* repeated stop after successful stop returns nil
* repeated stop after successful stop does not call component `Stop` again
* repeated stop after failed stop returns nil
* repeated stop after failed stop does not call component `Stop` again

Important test scenario for partial start failure:

```text
Register A, B, C.
A starts successfully.
B fails start.
C is never started.
Stop should only stop A.
```

Important test scenario for failed stop idempotency:

```text
Register A.
Start succeeds.
A Stop fails.
First Stop returns error.
Second Stop returns nil.
A Stop call count remains 1.
```

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
- internal/lifecycle/lifecycle_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 08 — Add registration-after-start tests

### Goal

Test that registration is rejected after lifecycle start has begun.

### Files

Modify:

```text
internal/lifecycle/lifecycle_test.go
```

### Requirements

Reuse the fake component type created in TASK 03.

Do not create duplicate fake component types.

Add tests for:

* registering after successful `Start(ctx)` returns an error matching `lifecycle.ErrAlreadyStarted`
* registering after failed `Start(ctx)` returns an error matching `lifecycle.ErrAlreadyStarted`
* failed start still counts as lifecycle start having begun

Important behavior dependency:

Task 04 must have marked lifecycle as started before attempting to start any component.

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

## TASK 09 — Add lifecycle documentation

### Goal

Document lifecycle package behavior and boundaries.

### Files

Create:

```text
docs/lifecycle.md
```

### Requirements

Create documentation with this heading:

```text
# Plomvix Lifecycle
```

The documentation must explain:

* lifecycle package purpose
* component interface
* manager role
* registration order
* start order
* reverse stop order
* context-aware start and stop
* start error behavior
* stop error behavior
* stop idempotency
* registration after start is rejected
* mutex-protected manager state
* non-goals

The documentation must include these exact strings because TASK 10 checks them:

```text
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
```

Non-goals section must clearly say lifecycle does not implement:

```text
signal handling
WAL
storage
query engine
API server
UI
```

Do not document future behavior as already implemented.

Do not mention file output, logging internals, config internals, or unrelated packages.

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
- docs/lifecycle.md

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 10 — Add lifecycle documentation tests

### Goal

Ensure lifecycle documentation exists and contains required behavior statements.

### Files

Modify:

```text
internal/lifecycle/lifecycle_test.go
```

### Requirements

Use external test package:

```go
package lifecycle_test
```

Add a documentation test that reads:

```go
os.ReadFile("../../docs/lifecycle.md")
```

Path note:

```text
This path assumes the test runs from internal/lifecycle/, which is the default behavior of go test ./...
```

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
signal handling
WAL
storage
query engine
API server
UI
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

## TASK 11 — Final lifecycle foundation review

### Goal

Review the lifecycle foundation for scope control, correctness, race safety, and project cleanliness.

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
* public API matches this plan
* component interface uses `context.Context`
* manager fields are unexported
* manager state is protected with `sync.Mutex`
* `Register`, `Start`, and `Stop` protect shared state
* registration rejects nil component
* registration rejects empty component name
* registration after start is rejected
* start marks lifecycle as started before component iteration begins
* start order is registration order
* stop order is reverse successful-start order
* start failure stops further start attempts
* stop failure still attempts remaining eligible components
* stop after successful stop is idempotent
* stop after failed stop is idempotent
* failed stop does not cause double stop on repeated calls
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
TASK 11 completed.
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
- lifecycle foundation complete
- safe to move to next core foundation feature
```

---

# Completion Criteria

This plan is complete only when:

* `internal/lifecycle/lifecycle.go` exists
* `internal/lifecycle/lifecycle_test.go` exists
* `docs/lifecycle.md` exists
* all lifecycle behavior tests pass
* lifecycle documentation test passes
* `go test ./...` passes
* `go build ./...` passes
* `go test -race ./...` passes
* `go mod tidy` produces no unwanted dependency changes
* final `go test ./...` passes
* no non-goal systems were introduced

---

# Recommended Next Step After Completion

After `lifecycle.md` is completed and verified, the next feature area should still remain core foundation.

Do not automatically move to WAL or storage unless the project owner confirms that lifecycle foundation is complete and the next feature area is selected.

Possible next core foundation feature after lifecycle:

```text
metadata foundation
```

But do not create that plan until lifecycle is completed and reviewed.
