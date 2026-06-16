# runtime_signals.md

# Plomvix Runtime Signal Handling Plan

## Purpose

Add OS signal handling to the Plomvix runtime so the process shuts down
cleanly when it receives a termination signal.

This plan extends `internal/runtime` with a signal-aware context helper
that cancels the root context on SIGTERM, SIGINT, SIGHUP, or SIGQUIT,
allowing the existing lifecycle shutdown sequence to run before the process
exits.

This is still runtime-layer work only.

Do not add database engines.
Do not add WAL.
Do not add storage.
Do not add query execution.
Do not add API server.
Do not add UI.
Do not add config hot reload.
Do not add background orchestration.
Do not add CLI flags.
Do not add environment overrides.

---

## Feature Name

```text
Runtime Signal Handling
```

Plan file:

```text
runtime_signals.md
```

Existing package:

```text
internal/runtime
```

---

## Required Starting State

This plan starts only after `runtime_enterprise.md` is completed and
verified.

Before starting this plan, the project must already have:

```text
internal/runtime/runtime.go
internal/runtime/runtime_test.go
internal/runtime/runtime_internal_test.go
cmd/plomvix/main.go
cmd/plomvix/main_test.go
docs/runtime.md
```

The runtime package must already expose:

```go
type Options struct {
    ConfigPath      string
    StartupTimeout  time.Duration
    ShutdownTimeout time.Duration
}

const DefaultConfigPath = "config.toml"
const DefaultStartupTimeout = 30 * time.Second
const DefaultShutdownTimeout = 30 * time.Second

func DefaultOptions() Options
func New(opts Options) (*Runtime, error)
func (r *Runtime) Start(ctx context.Context) error
func (r *Runtime) Stop(ctx context.Context) error
func (r *Runtime) State() lifecycle.State
func Run(ctx context.Context, opts Options) error
```

Existing public errors must include:

```go
var (
    ErrInvalidOptions = errors.New("runtime: invalid options")
    ErrLoadConfig     = errors.New("runtime: load config")
    ErrCreateLogger   = errors.New("runtime: create logger")
    ErrStartLifecycle = errors.New("runtime: start lifecycle")
    ErrStopLifecycle  = errors.New("runtime: stop lifecycle")
    ErrRuntimePanic   = errors.New("runtime: panic")
)
```

The runtime must already:

* load config through `internal/config.Load`
* create logger through `internal/logger.New`
* create lifecycle manager through `internal/lifecycle.NewManager`
* start and stop lifecycle manager
* call lifecycle stop even when lifecycle start fails
* recover runtime panics
* classify errors using sentinel values
* wire `cmd/plomvix/main.go` to `runtime.Run`
* pass `go test ./...`
* pass `go build ./...`
* pass `go test -race ./...`

If this starting state is not true, stop and report that
`runtime_enterprise.md` is incomplete.

---

## Current Project State

Completed foundation work:

```text
config foundation:              done
enterprise config hardening:    done
basic logger setup:             done
enterprise logger hardening:    done
lifecycle foundation:           done
enterprise lifecycle hardening: done
runtime setup:                  done
enterprise runtime hardening:   done
```

Current stage:

```text
runtime layer completion
```

Current feature area:

```text
runtime signal handling
```

---

## Go Version Requirement

Plomvix uses:

```text
Go 1.22 or later
```

Use only Go standard library plus dependencies already required by
completed plans.

Do not use APIs added after Go 1.22.

Important Go 1.22 rule:

```text
Do not use t.Chdir.
```

If a test needs to change directories, use the manual
`os.Getwd` / `os.Chdir` / `t.Cleanup` restore pattern.

---

## Coding Agent

Coding agent:

```text
DeepSeek V4 Pro
```

If the local environment uses a different exact DeepSeek model identifier,
use the configured DeepSeek coding model available there.

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

* Keep implementation small.
* Keep runtime focused on composition.
* Do not add future placeholders.
* Do not add unrelated folders.
* Do not create database engines.
* Do not create WAL code.
* Do not create storage code.
* Do not create query code.
* Do not create API server code.
* Do not create UI code.
* Do not add background goroutine supervision.
* Do not add health checks.
* Do not add readiness checks.
* Do not add CLI flags.
* Do not add environment variable overrides.
* Do not add config hot reload.
* Do not add logger hot reload.
* Do not add file logging.
* Do not add systemd integration.
* Do not add Docker/Kubernetes files.
* Do not add external dependencies.
* Keep tests deterministic.
* Use table-driven tests where useful.
* Do not create a root-level `tests/` directory.
* Do not use `t.Chdir`.

---

## Dependency Direction Rules

Allowed dependency direction:

```text
cmd/plomvix      imports internal/runtime
internal/runtime imports internal/config
internal/runtime imports internal/logger
internal/runtime imports internal/lifecycle
internal/runtime imports os/signal (standard library)
internal/runtime imports syscall (standard library, for signal constants)
```

Forbidden dependency direction:

```text
internal/config    imports internal/runtime
internal/logger    imports internal/runtime
internal/lifecycle imports internal/runtime
internal/runtime   imports cmd/plomvix
```

Runtime remains the composition layer above config, logger, and lifecycle.

Do not make config, logger, or lifecycle depend on runtime.

---

## Design Decision: Signal-Aware Context

Signal handling is implemented as a context helper inside `internal/runtime`.

The helper:

* creates a child context that cancels when a signal is received
* registers for SIGTERM, SIGINT, SIGHUP, SIGQUIT
* returns the child context and a cleanup function
* does not block
* does not own shutdown logic
* does not call `os.Exit`

An additional unexported helper `withSignalContextFromChan` accepts a
pre-created signal channel, allowing tests to inject signal delivery
directly without sending real OS signals to the test process.

The existing `Run(ctx, opts)` function is extended with a signal-aware
variant `RunWithSignals(opts)` that:

* creates the signal context internally
* calls `Run(ctx, opts)` with that context
* returns any error to the caller

`cmd/plomvix/main.go` is updated to call `RunWithSignals` instead of
`Run`.

This keeps signal handling inside the runtime layer without changing the
`Run` API and without moving logic into `main`.

---

## Signal Decision

Handled signals:

```text
SIGTERM  — standard process termination (Kubernetes, systemd)
SIGINT   — Ctrl+C from terminal
SIGHUP   — treated as shutdown in this plan
           documented as future config reload signal
           config reload is not implemented in this plan
SIGQUIT  — quit with core dump signal, treated as shutdown
```

SIGHUP policy:

```text
SIGHUP triggers a clean shutdown in this plan.
It is documented as the designated future config reload signal.
Config reload must not be implemented in this plan.
```

---

## Shutdown Timeout and Force Exit Policy

When a signal is received:

1. The signal context is cancelled.
2. `Run` receives the cancelled context.
3. The lifecycle shutdown sequence begins using `Options.ShutdownTimeout`.
4. If shutdown completes within the timeout, the process exits cleanly.
5. If shutdown does not complete within the timeout:
   * `Stop` returns a context deadline exceeded error.
   * `Run` returns that error wrapped as `ErrStopLifecycle`.
   * `main` receives the error and calls `os.Exit(1)`.

Force exit rules:

```text
Only main may call os.Exit.
Runtime must not call os.Exit.
Runtime must not start a watchdog goroutine.
Runtime returns a timeout error to the caller.
main decides to exit non-zero on error.
```

This is consistent with all existing project rules.

---

## New Public API

Add to `internal/runtime`:

```go
var ErrShutdownTimeout = errors.New("runtime: shutdown timeout")
```

Add signal-aware runner:

```go
func RunWithSignals(opts Options) error
```

Add signal context helpers (both unexported):

```go
func withSignalContext(ctx context.Context) (context.Context, context.CancelFunc)

func withSignalContextFromChan(
    ctx context.Context,
    ch <-chan os.Signal,
) (context.Context, context.CancelFunc)
```

`withSignalContext` is the production entry point that creates and
registers the channel.

`withSignalContextFromChan` accepts an existing channel, enabling tests
to inject signal delivery without sending real OS signals.

`RunWithSignals(opts)` behavior:

* create signal context using `withSignalContext(context.Background())`
* defer cleanup
* call `Run(signalCtx, opts)`
* return any error

`cmd/plomvix/main.go` update:

* call `RunWithSignals(runtime.DefaultOptions())` instead of
  `runtime.Run(context.Background(), runtime.DefaultOptions())`
* on error, write to stderr and exit non-zero

---

## Error Classification

Add:

```go
ErrShutdownTimeout = errors.New("runtime: shutdown timeout")
```

When context deadline exceeded is received during `Stop`:

* wrap with `ErrStopLifecycle` as already required
* additionally check if the cause is context deadline exceeded
* if so, also wrap with `ErrShutdownTimeout` so callers can detect timeout

Wrapping example:

```go
fmt.Errorf("%w: %w: %w", ErrStopLifecycle, ErrShutdownTimeout, err)
```

`fmt.Errorf` with multiple `%w` verbs is supported in Go 1.20+ and is
allowed under this Go 1.22 plan.

Tests may use `errors.Is(err, runtime.ErrShutdownTimeout)` to detect
timeout expiry.

---

## Logging Behavior

The signal helper and signal goroutine must not log.

Runtime may log shutdown progress in `Run` and the runtime methods only:

```text
runtime starting
runtime started
runtime stopping
runtime stopped
```

Use the existing component-scoped runtime logger.

Use `logger.ErrorAttr(err)` when logging runtime errors.

Do not call `slog.SetDefault`.

Do not create global logger state.

Do not add logging inside `withSignalContext`, `withSignalContextFromChan`,
or `RunWithSignals`.

Test log output to stdout/stderr during `go test` is expected and
acceptable.

Do not suppress test log output.

---

## SIGHUP Documentation Requirement

`docs/runtime.md` must be updated to include:

```text
SIGHUP is currently treated as a shutdown signal.
SIGHUP is the designated future signal for config reload.
Config reload is not implemented yet.
```

---

## Non-Goals

Do not implement:

* config hot reload on SIGHUP
* log reload on SIGHUP
* SIGUSR1 or SIGUSR2 handling
* signal masking
* signal forwarding to child processes
* daemon mode
* process supervisor
* PID file management
* double-fork daemonization
* watchdog goroutine calling os.Exit
* background service runner
* goroutine supervision beyond the single signal goroutine
* health checks
* readiness checks
* metrics endpoint
* API server
* UI server
* WAL
* storage
* query engine
* CLI flags
* environment overrides
* OpenTelemetry
* systemd notify protocol (sd_notify)
* Docker/Kubernetes files

---

## Task Plan

---

## TASK 01 — Add ErrShutdownTimeout sentinel

### Goal

Add the shutdown timeout error sentinel to the runtime package.

### Files

Modify:

```text
internal/runtime/runtime.go
```

### Requirements

Add:

```go
ErrShutdownTimeout = errors.New("runtime: shutdown timeout")
```

Do not change any existing behavior.

Do not modify `cmd/plomvix/main.go`.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 01 completed.
Files changed:
- internal/runtime/runtime.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 02 — Add ErrShutdownTimeout sentinel test

### Goal

Verify the shutdown timeout error sentinel is stable.

### Files

Modify:

```text
internal/runtime/runtime_test.go
```

### Package

Use existing external test package:

```go
package runtime_test
```

### Requirements

Add test confirming:

```text
ErrShutdownTimeout.Error() == "runtime: shutdown timeout"
```

Add to the existing stable error sentinel test comment:

```go
// These values are part of the stable runtime API.
```

Do not test wrapping yet.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 02 completed.
Files changed:
- internal/runtime/runtime_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 03 — Add signal context helper

### Goal

Add the signal-aware context helper to the runtime package.

### Files

Create:

```text
internal/runtime/signals.go
```

### Package

```go
package runtime
```

### Requirements

Required imports must include:

```go
context
os
os/signal
syscall
```

`os` is required because `withSignalContextFromChan` uses `os.Signal`
as a parameter type. Do not import `os` for any other reason in
`signals.go`.

Add unexported production helper:

```go
func withSignalContext(ctx context.Context) (context.Context, context.CancelFunc)
```

Add unexported testable helper:

```go
func withSignalContextFromChan(
    ctx context.Context,
    ch <-chan os.Signal,
) (context.Context, context.CancelFunc)
```

`withSignalContext` must:

* create a buffered `chan os.Signal` with capacity 1
* register signal notify for SIGTERM, SIGINT, SIGHUP, SIGQUIT
* call `withSignalContextFromChan(ctx, ch)`
* return the context and a cleanup function that calls both
  `signal.Stop(ch)` and the inner cancel

`withSignalContextFromChan` must:

* create a child context from the provided parent using
  `context.WithCancel`
* start exactly one goroutine using the select pattern:

```go
signalCtx, cancel := context.WithCancel(ctx)
go func() {
    select {
    case <-ch:
        cancel()
    case <-signalCtx.Done():
    }
}()
return signalCtx, cancel
```

* return the child context and the cancel function

Important:

```text
The goroutine must select on signalCtx.Done(), not ctx.Done().
The cleanup function returns the child cancel. When cleanup calls
cancel(), signalCtx.Done() closes and unblocks the goroutine.
If the goroutine selected on the parent ctx.Done() instead, cleanup
would not unblock it unless the caller also cancelled the parent,
which cannot be guaranteed.
```

Note: the `ch` parameter in `withSignalContextFromChan` is
`<-chan os.Signal` (receive-only). The `withSignalContext` production
wrapper holds the send side for `signal.Notify` and passes the
receive side to `withSignalContextFromChan`.

`signals.go` requires import of `os` because `withSignalContextFromChan`
uses `os.Signal` in its parameter type.

Required imports for `signals.go`:

```go
context
os
os/signal
syscall
```

No-logging rule:

```text
withSignalContext, withSignalContextFromChan, and the signal goroutine
must not log anything.
RunWithSignals must not log anything.
All shutdown logging belongs in Run and the Runtime methods only.
```

Do not add signal handling to `Run` yet.

Do not modify `cmd/plomvix/main.go` yet.

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 03 completed.
Files changed:
- internal/runtime/signals.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 04 — Add signal context helper tests

### Goal

Verify signal context helper behavior using the internal test package.

### Files

Modify:

```text
internal/runtime/runtime_internal_test.go
```

### Package

Use internal package:

```go
package runtime
```

### Requirements

Add tests for:

* `withSignalContext` returns a non-nil context
* `withSignalContext` returns a non-nil cancel function
* calling the cancel function cancels the context
* signal delivery through the internal channel cancels the context

Signal delivery test approach:

Do not send real OS signals to the test process.

Sending SIGINT to the whole test process is unsafe under
`go test -race ./...` and can interfere with other packages or the
test runner itself.

Instead, add a second unexported helper that accepts an already-created
signal channel, so tests can inject signal delivery directly:

```go
func withSignalContextFromChan(
    ctx context.Context,
    ch <-chan os.Signal,
) (context.Context, context.CancelFunc)
```

This helper has the same goroutine shape as `withSignalContext` but
accepts the channel from outside instead of creating and registering it.

`withSignalContext` is implemented by creating the channel, registering
signals, and delegating to `withSignalContextFromChan`.

Test shape using channel injection:

```go
ch := make(chan os.Signal, 1)
ctx, cancel := withSignalContextFromChan(context.Background(), ch)
defer cancel()

ch <- syscall.SIGINT

select {
case <-ctx.Done():
    // expected
case <-time.After(2 * time.Second):
    t.Fatal("context was not cancelled after signal")
}
```

Important:

```text
Use syscall.SIGINT as the injected signal value.
This is a real supported signal and avoids confusion with the
SIGUSR1/SIGUSR2 non-goal rule.
The signal value is only sent into the channel directly.
It is not sent to the OS process.
No real OS signal is delivered in this test.
```

Rules:

* do not send real OS signals in tests
* use channel injection via `withSignalContextFromChan` for signal tests
* `withSignalContextFromChan` must remain unexported
* `withSignalContext` remains the production entry point that creates
  and registers the channel
* the test must pass under `go test -race ./...`
* do not use `time.Sleep` as synchronization
* use the `select` with `time.After` timeout pattern to keep tests fast

Also add test for cleanup unblocking the goroutine:

* call `withSignalContextFromChan` with an empty channel
* call the cleanup cancel function
* verify `ctx.Done()` is closed promptly

This proves the goroutine does not leak when cleanup is called without
a signal.

### Verification

Run:

```bash
go test ./...
go test -race ./...
go build ./...
```

### Completion Report

```text
TASK 04 completed.
Files changed:
- internal/runtime/runtime_internal_test.go

Verification:
- go test ./...
- go test -race ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 05 — Add RunWithSignals

### Goal

Add the signal-aware top-level runtime entry point.

### Files

Modify:

```text
internal/runtime/signals.go
```

### Requirements

Add:

```go
func RunWithSignals(opts Options) error
```

Behavior:

* create signal context using `withSignalContext(context.Background())`
* defer the cleanup function
* call `Run(signalCtx, opts)`
* return any error from `Run`

Rules:

* do not duplicate `Run` logic
* do not call `os.Exit`
* do not start additional goroutines
* do not register additional signals
* do not add logging directly in `RunWithSignals`
* logging remains inside `Run` and the runtime methods

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 05 completed.
Files changed:
- internal/runtime/signals.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 06 — Add RunWithSignals error-path tests

### Goal

Verify `RunWithSignals` returns classified errors on deterministic
failure paths.

### Files

Modify:

```text
internal/runtime/runtime_test.go
```

### Package

Use existing external test package:

```go
package runtime_test
```

### Requirements

Add tests for:

* `RunWithSignals` with missing config returns error matching
  `runtime.ErrLoadConfig`
* `RunWithSignals` with invalid config returns error matching
  `runtime.ErrLoadConfig`

Use `errors.Is` for error classification checks.

Use temp config files with explicit paths.

Do not add a test that calls `RunWithSignals` with a valid config and
expects nil return.

Reason:

```text
RunWithSignals blocks until the signal context is cancelled or Run
returns. With valid config and no signal, Run starts the lifecycle,
then stops it immediately since no components are registered. In the
current one-shot runtime this returns nil quickly. But this assumption
is fragile — if Run is ever changed to wait for a signal before
stopping, the test will hang indefinitely.

Instead test RunWithSignals only through deterministic error paths
where Run returns immediately due to config or option errors.

Signal context behavior is tested separately in
runtime_internal_test.go using channel injection.
```

Do not assert exact log output.

Test log output to stderr during `go test` is expected and acceptable.
Do not suppress it.

Important:

```text
Do not test actual signal delivery in the external test package.
Signal delivery tests live in runtime_internal_test.go.
RunWithSignals tests here only verify the error classification path.
```

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 06 completed.
Files changed:
- internal/runtime/runtime_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 07 — Add shutdown timeout error classification

### Goal

Make shutdown timeout detectable through `errors.Is`.

### Files

Modify:

```text
internal/runtime/runtime.go
```

### Requirements

Update `Runtime.Stop(ctx)` so that when `manager.Stop(stopCtx)` returns
an error and the shutdown context has exceeded its deadline:

* wrap the error with both `ErrStopLifecycle` and `ErrShutdownTimeout`

Required wrapping:

```go
if stopCtx.Err() == context.DeadlineExceeded {
    return fmt.Errorf("%w: %w: %w", ErrStopLifecycle, ErrShutdownTimeout, err)
}
return fmt.Errorf("%w: %w", ErrStopLifecycle, err)
```

`fmt.Errorf` with multiple `%w` verbs is supported in Go 1.20+ and is
allowed under this Go 1.22 plan.

Rules:

* do not change any other existing behavior
* do not add goroutines
* do not call `os.Exit`
* keep the existing `ErrStopLifecycle` wrapping for non-timeout errors

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 07 completed.
Files changed:
- internal/runtime/runtime.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 08 — Add shutdown timeout classification tests

### Goal

Verify shutdown timeout errors are detectable with `errors.Is`.

### Files

Modify:

```text
internal/runtime/runtime_test.go
```

### Package

Use existing external test package:

```go
package runtime_test
```

### Requirements

Add a test that verifies `ErrShutdownTimeout` is wrapped correctly.

Because no real slow components exist in this plan, test the
classification using a very short `ShutdownTimeout` and a cancelled
context:

```go
opts := runtime.Options{
    ConfigPath:      path,
    StartupTimeout:  runtime.DefaultStartupTimeout,
    ShutdownTimeout: 1 * time.Nanosecond,
}
```

With an extremely short shutdown timeout:

* `Stop` may or may not hit the deadline depending on timing
* this test is inherently timing-sensitive

Acceptable alternative test approach:

* verify that `ErrShutdownTimeout.Error() == "runtime: shutdown timeout"`
* verify that a manually constructed wrapped error containing
  `ErrShutdownTimeout` is detectable with `errors.Is`

```go
wrapped := fmt.Errorf("%w: %w", runtime.ErrStopLifecycle,
    runtime.ErrShutdownTimeout)
if !errors.Is(wrapped, runtime.ErrShutdownTimeout) {
    t.Fatal("expected ErrShutdownTimeout to be detectable")
}
if !errors.Is(wrapped, runtime.ErrStopLifecycle) {
    t.Fatal("expected ErrStopLifecycle to be detectable")
}
```

Rules:

* do not use sleeps as synchronization
* do not make the test rely on exact timing
* the wrapping test approach is preferred over timing-sensitive tests
* `errors.Is` must work correctly with the wrapping pattern

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 08 completed.
Files changed:
- internal/runtime/runtime_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 09 — Update cmd/plomvix/main.go to use RunWithSignals

### Goal

Wire the production binary to use signal-aware shutdown.

### Files

Modify:

```text
cmd/plomvix/main.go
```

### Requirements

Update `main.go` so:

* `run(ctx, opts)` is replaced or updated to call
  `runtime.RunWithSignals(opts)` instead of
  `runtime.Run(context.Background(), opts)`
* `main()` still calls `run` and handles errors

Because `RunWithSignals` does not take a context parameter, the
suggested updated shape is:

```go
func main() {
    if err := run(runtime.DefaultOptions()); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}

func run(opts runtime.Options) error {
    return runtime.RunWithSignals(opts)
}
```

Rules:

* only `main` calls `os.Exit`
* `run` must not call `os.Exit`
* do not add CLI flags
* do not add signal handling directly in `main.go`
* do not add environment variable parsing

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 09 completed.
Files changed:
- cmd/plomvix/main.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 10 — Update cmd/plomvix/main_test.go

### Goal

Update main tests to match the updated `run` helper signature.

### Files

Modify:

```text
cmd/plomvix/main_test.go
```

### Requirements

Update existing tests so they match the new `run(opts)` signature.

Add or update tests for:

* missing config path returns error matching `runtime.ErrLoadConfig`
* invalid config content returns error matching `runtime.ErrLoadConfig`

Use `errors.Is` for error classification checks.

Use temp config files with explicit paths.

Do not add a valid-config test that expects `run(opts)` to return nil.

Reason:

```text
run(opts) now calls RunWithSignals(opts), which blocks until the signal
context is cancelled or Run returns an error. With valid config and no
signal, the current one-shot runtime returns quickly — but this
assumption is fragile. If Run is ever changed to wait for a signal
before stopping, the test will hang indefinitely.

Only test deterministic error paths here. Valid composition behavior
is covered by runtime_test.go through Run(ctx, opts) directly with
a cancellable context.
```

Do not test `main()` directly.

Do not call `os.Exit` from tests.

Do not add CLI flags.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 10 completed.
Files changed:
- cmd/plomvix/main_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 11 — Update runtime documentation

### Goal

Document signal handling behavior and SIGHUP policy.

### Files

Modify:

```text
docs/runtime.md
```

### Requirements

Add a signal handling section to the existing runtime documentation.

Document:

* handled signals: SIGTERM, SIGINT, SIGHUP, SIGQUIT
* all four signals trigger clean shutdown
* SIGHUP is the designated future config reload signal
* config reload is not implemented yet
* shutdown timeout behavior
* force exit via main on timeout error
* `RunWithSignals` as the production entry point
* `Run` remains available for programmatic use without signals

The documentation must include these exact strings because TASK 12
checks them:

```text
signal handling
SIGTERM
SIGINT
SIGHUP
SIGQUIT
RunWithSignals
shutdown timeout
force exit
SIGHUP is the designated future signal for config reload
config reload is not implemented yet
```

SIGHUP policy must include this exact sentence:

```text
SIGHUP is the designated future signal for config reload. Config reload is not implemented yet.
```

Do not document future behavior as already implemented.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 11 completed.
Files changed:
- docs/runtime.md

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 12 — Update runtime documentation tests

### Goal

Verify signal handling documentation exists and covers required statements.

### Files

Modify:

```text
internal/runtime/runtime_test.go
```

### Package

Keep:

```go
package runtime_test
```

### Requirements

Update the existing documentation test that reads:

```go
os.ReadFile("../../docs/runtime.md")
```

This path assumes the test runs from `internal/runtime/`, which is the
default behavior of `go test ./...`.

Add checks that the document contains these exact strings:

```text
signal handling
SIGTERM
SIGINT
SIGHUP
SIGQUIT
RunWithSignals
shutdown timeout
force exit
SIGHUP is the designated future signal for config reload
config reload is not implemented yet
```

Keep all existing documentation string checks from `runtime_enterprise.md`.

Use stable substring checks.

Do not make fragile checks for full paragraphs.

### Verification

Run:

```bash
go test ./...
go build ./...
```

### Completion Report

```text
TASK 12 completed.
Files changed:
- internal/runtime/runtime_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```

---

## TASK 13 — Final runtime signal handling review

### Goal

Review signal handling for correctness, safety, scope control, and
project cleanliness.

### Files

Review only unless fixes are required:

```text
internal/runtime/runtime.go
internal/runtime/signals.go
internal/runtime/runtime_test.go
internal/runtime/runtime_internal_test.go
cmd/plomvix/main.go
cmd/plomvix/main_test.go
docs/runtime.md
go.mod
go.sum
```

### Requirements

Confirm:

* `ErrShutdownTimeout` exists
* `ErrShutdownTimeout` is compatible with `errors.Is`
* `withSignalContext` exists and is unexported
* `withSignalContextFromChan` exists and is unexported
* `withSignalContext` registers SIGTERM, SIGINT, SIGHUP, SIGQUIT
* `withSignalContext` delegates to `withSignalContextFromChan`
* signal channel is buffered with capacity 1
* goroutine selects on both signal channel and `signalCtx.Done()`
* goroutine does not leak when cleanup is called without a signal
* signal goroutine does not call `os.Exit`
* signal goroutine does not interact with lifecycle directly
* signal tests use channel injection via `withSignalContextFromChan`
* no real OS signals are sent to the test process
* `RunWithSignals` exists
* `RunWithSignals` calls `Run` with signal context
* `RunWithSignals` defers cleanup
* `Run` API remains unchanged and compatible
* `Runtime.Stop` wraps timeout errors with `ErrShutdownTimeout`
* `Runtime.Stop` wraps timeout errors with `ErrStopLifecycle`
* `cmd/plomvix/main.go` calls `RunWithSignals`
* only `main` calls `os.Exit`
* `run` helper does not call `os.Exit`
* `cmd/plomvix/main_test.go` only tests deterministic error paths
* `cmd/plomvix/main_test.go` does not test valid-config-returns-nil via `RunWithSignals`
* signal context tests exist in `runtime_internal_test.go`
* `RunWithSignals` tests exist in `runtime_test.go`
* shutdown timeout classification tests exist
* docs updated with signal handling section
* SIGHUP documented as future reload signal
* config reload not implemented
* no config reload was added
* no CLI flags were added
* no environment overrides were added
* no watchdog goroutine was added
* no additional goroutines beyond the signal goroutine were added
* no WAL was added
* no storage was added
* no query engine was added
* no API server was added
* no UI was added
* no database engines were added
* no external dependencies were added
* config package does not import runtime
* logger package does not import runtime
* lifecycle package does not import runtime
* `go test -race ./...` passes
* no root-level `tests/` directory exists
* `go.mod` has no new dependencies
* `go.sum` has no new dependency entries

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

```text
TASK 13 completed.
Files reviewed:
- internal/runtime/runtime.go
- internal/runtime/signals.go
- internal/runtime/runtime_test.go
- internal/runtime/runtime_internal_test.go
- cmd/plomvix/main.go
- cmd/plomvix/main_test.go
- docs/runtime.md
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
- runtime signal handling complete
- runtime layer is now complete
- process shuts down cleanly on SIGTERM, SIGINT, SIGHUP, SIGQUIT
- no database engine features added
```

---

## Final Expected Structure

After this plan, relevant structure should include:

```text
plomvix/
├── cmd/
│   └── plomvix/
│       ├── main.go
│       └── main_test.go
├── docs/
│   ├── config.md
│   ├── logging.md
│   ├── lifecycle.md
│   └── runtime.md
├── internal/
│   ├── config/
│   │   ├── config.go
│   │   └── config_test.go
│   ├── logger/
│   │   ├── logger.go
│   │   ├── logger_test.go
│   │   └── logger_internal_test.go
│   ├── lifecycle/
│   │   ├── lifecycle.go
│   │   └── lifecycle_test.go
│   └── runtime/
│       ├── runtime.go
│       ├── signals.go
│       ├── runtime_test.go
│       └── runtime_internal_test.go
├── config.toml
├── config.example.toml
├── go.mod
├── go.sum
├── setup.md
├── config_setup.md
├── config_enterprise.md
├── logging_setup.md
├── logger_enterprise.md
├── lifecycle.md
├── lifecycle_enterprise.md
├── runtime_setup.md
├── runtime_enterprise.md
├── runtime_signals.md
└── README.md
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
tests/
```

---

## Completion Criteria

This plan is complete only when:

* `internal/runtime/signals.go` exists
* `withSignalContext` is implemented and unexported
* `withSignalContextFromChan` is implemented and unexported
* `withSignalContext` delegates to `withSignalContextFromChan`
* signal channel is buffered with capacity 1
* goroutine selects on both signal channel and `signalCtx.Done()`
* goroutine does not leak when cleanup cancels without a signal
* SIGTERM, SIGINT, SIGHUP, SIGQUIT are all registered
* signal tests use channel injection, not real OS signals
* `RunWithSignals` is implemented
* `RunWithSignals` tests use deterministic error paths only
* `ErrShutdownTimeout` exists and is compatible with `errors.Is`
* shutdown timeout is wrapped with `ErrShutdownTimeout` and `ErrStopLifecycle`
* `cmd/plomvix/main.go` calls `RunWithSignals`
* signal context tests pass in `runtime_internal_test.go`
* `RunWithSignals` tests pass in `runtime_test.go`
* runtime documentation includes signal handling section
* SIGHUP is documented as future reload signal
* runtime documentation test passes
* `go test ./...` passes
* `go build ./...` passes
* `go test -race ./...` passes
* `go mod tidy` produces no unwanted dependency changes
* final `go test ./...` passes
* no non-goal systems were introduced

---

## Recommended Next Step After Completion

After `runtime_signals.md` is completed and verified, the runtime layer
is complete.

```text
config     → done and hardened
logger     → done and hardened
lifecycle  → done and hardened
runtime    → done, hardened, and signal-aware
```

The project is ready to begin the first database foundation feature.

Do not automatically choose the next feature here.

Continue with the selected database foundation roadmap as confirmed by
the project owner.

Possible next directions depending on the chosen roadmap:

```text
metadata_setup.md          — internal catalog foundation
kv_encoding_setup.md       — key encoding for KV engine
kv_store_setup.md          — KV store foundation
storage_setup.md           — page-based storage foundation
```

Do not start any database foundation feature until runtime signal
handling is completed and verified and the project owner confirms
the next feature area.