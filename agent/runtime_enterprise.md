runtime_enterprise.md
Plomvix Enterprise Runtime Hardening Plan
Purpose
Harden the completed Plomvix runtime setup into a safer enterprise-grade runtime composition layer.
This plan improves runtime correctness, startup/shutdown clarity, error classification, timeout policy, panic safety, documentation, and test coverage.
This is still runtime-composition hardening only.
Do not add database engines.
Do not add WAL.
Do not add storage.
Do not add query execution.
Do not add API server.
Do not add UI.
Do not add OS signal handling.
Do not add background orchestration.
Do not add CLI flags.
Do not add environment overrides.
---
Feature Name
```text
Enterprise Runtime Hardening
```
Plan file:
```text
runtime_enterprise.md
```
Existing package:
```text
internal/runtime
```
---
Required Starting State
This plan starts only after `runtime_setup.md` is completed and verified.
Before starting this plan, the project must already have:
```text
internal/runtime/runtime.go
internal/runtime/runtime_test.go
cmd/plomvix/main.go
cmd/plomvix/main_test.go
docs/runtime.md
```
The runtime package must already expose:
```go
type Options struct {
	ConfigPath string
}

const DefaultConfigPath = "config.toml"

func DefaultOptions() Options
func Run(ctx context.Context, opts Options) error
```
The runtime setup must already do the following:
load config through `internal/config.Load`
create logger through `internal/logger.New`
create component-scoped runtime logger through `logger.WithComponent(base, "runtime")`
create lifecycle manager through `internal/lifecycle.NewManager`
start lifecycle manager
stop lifecycle manager
call lifecycle stop even when lifecycle start fails
wire `cmd/plomvix/main.go` to `runtime.Run`
avoid WAL/storage/query/API/UI/engine features
pass `go test ./...`
pass `go build ./...`
pass `go test -race ./...`
If this starting state is not true, stop and report that `runtime_setup.md` is incomplete.
---
Current Project State
Completed foundation work:
```text
config foundation: done
enterprise config hardening: done
basic logger setup: done
enterprise logger hardening: done
lifecycle foundation: done
enterprise lifecycle hardening: done
runtime setup and core composition: done
```
Current stage:
```text
core runtime hardening only
```
Current feature area:
```text
enterprise runtime hardening
```
---
Go Version Requirement
Plomvix uses:
```text
Go 1.22 or later
```
Use only Go standard library plus dependencies already required by completed plans.
Do not use APIs added after Go 1.22.
Important Go 1.22 rule:
```text
Do not use t.Chdir.
```
If a test needs to change directories, use the manual `os.Getwd` / `os.Chdir` / `t.Cleanup` restore pattern.
---
Coding Agent
Coding agent:
```text
DeepSeek V4 Pro
```
If the local environment uses a different exact DeepSeek model identifier, use the configured DeepSeek coding model available there.
Tasks must be executed one at a time, in exact order.
Do not proceed to the next task until the current task passes verification.
---
Graphify Rule
For every task:
Search Graphify before starting the task if Graphify is available.
Update Graphify after completing the task if Graphify is available.
If Graphify is unavailable, do not block the task.
Mention Graphify availability in the task report.
---
Global Project Rules
Follow these rules for every task:
Keep implementation small.
Keep runtime focused on composition.
Do not add future placeholders.
Do not add unrelated folders.
Do not create database engines.
Do not create WAL code.
Do not create storage code.
Do not create query code.
Do not create API server code.
Do not create UI code.
Do not add OS signal handling.
Do not add goroutine supervision.
Do not add health checks.
Do not add readiness checks.
Do not add CLI flags.
Do not add environment variable overrides.
Do not add config hot reload.
Do not add logger hot reload.
Do not add file logging.
Do not add systemd integration.
Do not add Docker/Kubernetes files.
Do not add external dependencies.
Keep tests deterministic.
Use table-driven tests where useful.
Do not create a root-level `tests/` directory.
---
Dependency Direction Rules
Allowed dependency direction:
```text
cmd/plomvix imports internal/runtime
internal/runtime imports internal/config
internal/runtime imports internal/logger
internal/runtime imports internal/lifecycle
```
Forbidden dependency direction:
```text
internal/config imports internal/runtime
internal/logger imports internal/runtime
internal/lifecycle imports internal/runtime
internal/runtime imports cmd/plomvix
```
Runtime remains the composition layer above config, logger, and lifecycle.
Do not make config, logger, or lifecycle depend on runtime.
---
Enterprise Runtime Hardening Goals
This plan adds:
runtime error classification
runtime option timeout policy
explicit runtime state object
constructor-based runtime composition
start/stop methods on runtime
panic recovery around runtime operations
clearer startup/shutdown behavior
stronger runtime tests
hardened runtime documentation
final scope-control review
---
Non-Goals
Do not implement:
OS signal handling
graceful process signal shutdown
daemon mode
process supervisor
background service runner
goroutine supervision
health checks
readiness checks
metrics endpoint
API server
UI server
WAL
storage
query engine
metadata catalog
engine registration
workload engines
CLI flags
environment overrides
config reload
logger reload
file output
log rotation
OpenTelemetry
request IDs
trace IDs
systemd integration
Docker/Kubernetes files
Enterprise runtime hardening here means stronger correctness and safer composition, not broader system orchestration.
---
Final Public API Additions
Existing API must remain compatible:
```go
func DefaultOptions() Options
func Run(ctx context.Context, opts Options) error
```
Extend `Options`:
```go
type Options struct {
	ConfigPath      string
	StartupTimeout  time.Duration
	ShutdownTimeout time.Duration
}
```
Add timeout constants:
```go
const DefaultStartupTimeout = 30 * time.Second
const DefaultShutdownTimeout = 30 * time.Second
```
Add public runtime errors:
```go
var (
	ErrInvalidOptions  = errors.New("runtime: invalid options")
	ErrLoadConfig      = errors.New("runtime: load config")
	ErrCreateLogger    = errors.New("runtime: create logger")
	ErrStartLifecycle  = errors.New("runtime: start lifecycle")
	ErrStopLifecycle   = errors.New("runtime: stop lifecycle")
	ErrRuntimePanic    = errors.New("runtime: panic")
)
```
Add runtime object:
```go
type Runtime struct {
	// unexported fields only
}
```
Add constructor and methods:
```go
func New(opts Options) (*Runtime, error)
func (r *Runtime) Start(ctx context.Context) error
func (r *Runtime) Stop(ctx context.Context) error
func (r *Runtime) State() lifecycle.State
```
Important:
`Run(ctx, opts)` remains the simple top-level runtime entrypoint.
`Run(ctx, opts)` should use `New(opts)`, `Start(ctx)`, and `Stop(ctx)` internally.
`Runtime` fields must remain unexported.
`Runtime` must not expose config mutation.
`Runtime` must not expose logger mutation.
`Runtime` must not expose lifecycle manager mutation.
---
Final Enterprise Runtime Behavior
Runtime Construction
`New(opts)` must:
Resolve runtime options.
Apply default config path when empty.
Apply default startup timeout when zero.
Apply default shutdown timeout when zero.
Reject negative timeout values.
Load config using `config.Load`.
Create logger using `logger.New`.
Create component-scoped logger using `logger.WithComponent(base, "runtime")`.
Create lifecycle manager using `lifecycle.NewManager`.
Return a `*Runtime` object.
`New(opts)` must not:
start lifecycle
stop lifecycle
register production components
start goroutines
call `os.Exit`
---
Runtime Start
`Runtime.Start(ctx)` must:
create a startup context using `Options.StartupTimeout`
start the lifecycle manager using that context
return error matching `ErrStartLifecycle` if lifecycle start fails
recover runtime panics and return error matching `ErrRuntimePanic`
not call `os.Exit`
not start goroutines
Because no production components are registered in this plan, start should normally succeed.
---
Runtime Stop
`Runtime.Stop(ctx)` must:
create a shutdown context using `Options.ShutdownTimeout`
stop the lifecycle manager using that context
return error matching `ErrStopLifecycle` if lifecycle stop fails
recover runtime panics and return error matching `ErrRuntimePanic`
remain safe when called after start failure
rely on lifecycle idempotency for repeated stop calls
not call `os.Exit`
not start goroutines
Because no production components are registered in this plan, stop should normally succeed.
---
Runtime Run
`Run(ctx, opts)` must:
Create runtime using `New(opts)`.
Start runtime.
If start fails, still call `Stop(ctx)` before returning.
Stop runtime before returning on successful start.
Return classified errors.
If `Start(ctx)` fails and `Stop(ctx)` also fails:
return an error that preserves the start error
include the stop error using `errors.Join` if useful
do not hide the original start failure
---
Error Classification Rules
Errors must be compatible with `errors.Is`.
Expected classification:
```text
config load failure      -> ErrLoadConfig
logger creation failure  -> ErrCreateLogger
invalid runtime options  -> ErrInvalidOptions
lifecycle start failure  -> ErrStartLifecycle
lifecycle stop failure   -> ErrStopLifecycle
runtime panic            -> ErrRuntimePanic
```
Use wrapping like:
```go
fmt.Errorf("%w: %w", ErrLoadConfig, err)
```
`fmt.Errorf` with multiple `%w` verbs is supported in Go 1.20+ and is allowed under this Go 1.22 plan.
Do not rely only on string matching.
Tests should use `errors.Is` for classified errors.
---
Timeout Policy
Runtime options support startup and shutdown timeouts.
Defaults:
```text
StartupTimeout  = 30 seconds
ShutdownTimeout = 30 seconds
```
Rules:
zero timeout means use default
negative timeout is invalid
invalid timeout returns error matching `ErrInvalidOptions`
startup timeout applies only to lifecycle start
shutdown timeout applies only to lifecycle stop
timeout policy does not implement signal handling
timeout policy does not add goroutines
timeout policy does not add background supervision
---
Panic Recovery Policy
Runtime must recover from panics in runtime operations and return an error instead of crashing.
Rules:
recover panics around runtime construction/start/stop helper boundaries where practical
returned error must match `ErrRuntimePanic`
returned error must include the operation name
returned error must include the word `panic`
do not re-panic
do not call `os.Exit`
do not swallow ordinary returned errors
This panic recovery is for runtime composition safety only.
Do not add panic recovery to config, logger, or lifecycle packages in this plan.
---
Logging Behavior
Runtime may keep existing minimal log messages:
```text
runtime starting
runtime started
runtime stopping
runtime stopped
```
Rules:
use component-scoped runtime logger
use `logger.ErrorAttr(err)` when logging runtime errors
do not call `slog.SetDefault`
do not create global logger state
do not add request IDs
do not add trace IDs
do not add logging middleware
do not add a `discard` logger output
Test log output to stdout/stderr during `go test` is expected and acceptable.
Do not suppress test log output by changing logger behavior or adding hidden runtime switches.
---
Task Plan
---
TASK 01 — Add enterprise runtime errors
Goal
Add public error sentinels for runtime error classification.
Files
Modify:
```text
internal/runtime/runtime.go
```
Requirements
Add imports as needed:
```go
errors
```
Add:
```go
var (
	ErrInvalidOptions  = errors.New("runtime: invalid options")
	ErrLoadConfig      = errors.New("runtime: load config")
	ErrCreateLogger    = errors.New("runtime: create logger")
	ErrStartLifecycle  = errors.New("runtime: start lifecycle")
	ErrStopLifecycle   = errors.New("runtime: stop lifecycle")
	ErrRuntimePanic    = errors.New("runtime: panic")
)
```
Do not use these errors yet unless needed to keep code clean.
Do not change runtime behavior in this task.
Do not modify `cmd/plomvix/main.go`.
Verification
Run:
```bash
go test ./...
go build ./...
```
Expected:
```text
All tests pass.
Build succeeds.
```
Completion Report
Report:
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
TASK 02 — Add enterprise runtime error tests
Goal
Verify runtime error sentinel values are stable.
Files
Modify:
```text
internal/runtime/runtime_test.go
```
Package
Use existing external test package:
```go
package runtime_test
```
Requirements
Add tests confirming error strings:
```text
ErrInvalidOptions.Error() == "runtime: invalid options"
ErrLoadConfig.Error() == "runtime: load config"
ErrCreateLogger.Error() == "runtime: create logger"
ErrStartLifecycle.Error() == "runtime: start lifecycle"
ErrStopLifecycle.Error() == "runtime: stop lifecycle"
ErrRuntimePanic.Error() == "runtime: panic"
```
Add a test comment:
```go
// These values are part of the stable runtime API.
```
Do not test wrapping yet.
Verification
Run:
```bash
go test ./...
go build ./...
```
Expected:
```text
All tests pass.
Build succeeds.
```
Completion Report
Report:
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
TASK 03 — Add runtime timeout options and defaults
Goal
Add explicit startup and shutdown timeout policy to runtime options.
Files
Modify:
```text
internal/runtime/runtime.go
```
Requirements
Add import:
```go
time
```
Add constants:
```go
const DefaultStartupTimeout = 30 * time.Second
const DefaultShutdownTimeout = 30 * time.Second
```
Extend `Options`:
```go
type Options struct {
	ConfigPath      string
	StartupTimeout  time.Duration
	ShutdownTimeout time.Duration
}
```
Update `DefaultOptions()` so it returns:
```go
Options{
	ConfigPath:      DefaultConfigPath,
	StartupTimeout:  DefaultStartupTimeout,
	ShutdownTimeout: DefaultShutdownTimeout,
}
```
Add unexported helper:
```go
func resolveOptions(opts Options) (Options, error)
```
Behavior:
empty `ConfigPath` becomes `DefaultConfigPath`
zero `StartupTimeout` becomes `DefaultStartupTimeout`
zero `ShutdownTimeout` becomes `DefaultShutdownTimeout`
negative `StartupTimeout` returns error matching `ErrInvalidOptions`
negative `ShutdownTimeout` returns error matching `ErrInvalidOptions`
invalid option error includes field name
Do not wire timeout contexts into start/stop yet.
Verification
Run:
```bash
go test ./...
go build ./...
```
Expected:
```text
All tests pass.
Build succeeds.
```
Completion Report
Report:
```text
TASK 03 completed.
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
TASK 04 — Add runtime timeout option tests
Goal
Verify runtime option defaults and invalid timeout behavior.
Files
Modify:
```text
internal/runtime/runtime_test.go
```
Requirements
Add tests for:
`DefaultStartupTimeout == 30*time.Second`
`DefaultShutdownTimeout == 30*time.Second`
`DefaultOptions().StartupTimeout == DefaultStartupTimeout`
`DefaultOptions().ShutdownTimeout == DefaultShutdownTimeout`
empty `Options{}` still uses default config path behavior when a local `config.toml` exists
negative startup timeout returns error matching `runtime.ErrInvalidOptions`
negative shutdown timeout returns error matching `runtime.ErrInvalidOptions`
For empty options behavior:
create temp directory
write valid `config.toml`
manually change working directory using `os.Getwd` / `os.Chdir` / `t.Cleanup`
call `runtime.Run(context.Background(), runtime.Options{})`
Do not use `t.Chdir`.
Required manual chdir pattern:
```go
oldWD, err := os.Getwd()
if err != nil {
	t.Fatalf("get working directory: %v", err)
}

if err := os.Chdir(tempDir); err != nil {
	t.Fatalf("change working directory: %v", err)
}

t.Cleanup(func() {
	if err := os.Chdir(oldWD); err != nil {
		t.Fatalf("restore working directory: %v", err)
	}
})
```
Use `errors.Is` for invalid option checks.
Test log output to stderr during `go test` is expected and acceptable. Do not suppress it.
Verification
Run:
```bash
go test ./...
go build ./...
```
Expected:
```text
All tests pass.
Build succeeds.
```
Completion Report
Report:
```text
TASK 04 completed.
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
TASK 05 — Add Runtime struct and constructor
Goal
Introduce an explicit runtime object that owns composed foundation instances.
Files
Modify:
```text
internal/runtime/runtime.go
```
Requirements
Required imports must include:
```go
log/slog

"github.com/plomvix/plomvix/internal/config"
"github.com/plomvix/plomvix/internal/lifecycle"
"github.com/plomvix/plomvix/internal/logger"
```
Add:
```go
type Runtime struct {
	opts    Options
	cfg     config.Config
	log     *slog.Logger
	manager *lifecycle.Manager
}
```
All fields must be unexported.
Add constructor:
```go
func New(opts Options) (*Runtime, error)
```
`New(opts)` must:
resolve options using `resolveOptions`
load config with `config.Load(resolved.ConfigPath)`
create base logger with `logger.New(cfg.Logger)`
create component-scoped logger with `logger.WithComponent(baseLog, "runtime")`
create lifecycle manager with `lifecycle.NewManager()`
return `*Runtime`
Error wrapping:
invalid options -> `ErrInvalidOptions`
config load failure -> `ErrLoadConfig`
logger creation failure -> `ErrCreateLogger`
Use `fmt.Errorf("%w: %w", ErrLoadConfig, err)` style wrapping.
`fmt.Errorf` with multiple `%w` verbs is supported in Go 1.20+ and is allowed under this Go 1.22 plan.
Do not start lifecycle in `New`.
Do not register production components.
Do not modify `cmd/plomvix/main.go`.
Verification
Run:
```bash
go test ./...
go build ./...
```
Expected:
```text
All tests pass.
Build succeeds.
```
Completion Report
Report:
```text
TASK 05 completed.
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
TASK 06 — Add Runtime constructor tests
Goal
Verify runtime construction behavior and error classification.
Files
Modify:
```text
internal/runtime/runtime_test.go
```
Requirements
Add tests for:
`runtime.New(validOptions)` returns non-nil runtime
missing config path returns error matching `runtime.ErrLoadConfig`
malformed config returns error matching `runtime.ErrLoadConfig`
invalid logger config returns error matching `runtime.ErrLoadConfig`
negative startup timeout returns error matching `runtime.ErrInvalidOptions`
negative shutdown timeout returns error matching `runtime.ErrInvalidOptions`
Important:
Invalid logger config should still be classified as `ErrLoadConfig` because config validation rejects it before logger construction.
These tests verify the config-validation path only. A separate `ErrCreateLogger` path exists for cases where config is valid but logger construction itself fails, which is not testable without mocking and is not tested in this plan.
Use `errors.Is` for classification checks.
Use temp config files.
Do not assert log output.
Test log output to stderr during `go test` is expected and acceptable. Do not suppress it.
Verification
Run:
```bash
go test ./...
go build ./...
```
Expected:
```text
All tests pass.
Build succeeds.
```
Completion Report
Report:
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
TASK 07 — Add Runtime Start, Stop, and State methods
Goal
Move lifecycle start/stop behavior onto the explicit runtime object.
Files
Modify:
```text
internal/runtime/runtime.go
```
Requirements
Add:
```go
func (r *Runtime) Start(ctx context.Context) error
func (r *Runtime) Stop(ctx context.Context) error
func (r *Runtime) State() lifecycle.State
```
`Start(ctx)` behavior:
if `r == nil`, return error matching `ErrInvalidOptions`
create startup context using `r.opts.StartupTimeout`
call `r.manager.Start(startCtx)`
wrap lifecycle start error with `ErrStartLifecycle`
log `runtime starting`
log `runtime started` after success
`Stop(ctx)` behavior:
if `r == nil`, return error matching `ErrInvalidOptions`
create shutdown context using `r.opts.ShutdownTimeout`
call `r.manager.Stop(stopCtx)`
wrap lifecycle stop error with `ErrStopLifecycle`
log `runtime stopping`
log `runtime stopped` after success
`State()` behavior:
if `r == nil`, return `lifecycle.StateFailed`
return `r.manager.State()`
Required imports must include `github.com/plomvix/plomvix/internal/lifecycle` because `State()` returns `lifecycle.State`.
Timeout context pattern:
```go
startCtx, cancel := context.WithTimeout(ctx, r.opts.StartupTimeout)
defer cancel()
```
Use the equivalent for shutdown.
Both startup and shutdown timeout contexts must be derived independently from the caller-provided `ctx`, not from each other. Do not derive shutdown context from startup context.
Do not add goroutines.
Do not add signal handling.
Do not register production components.
Verification
Run:
```bash
go test ./...
go build ./...
```
Expected:
```text
All tests pass.
Build succeeds.
```
Completion Report
Report:
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
TASK 08 — Add Runtime Start, Stop, and State tests
Goal
Verify runtime methods compose lifecycle safely.
Files
Modify:
```text
internal/runtime/runtime_test.go
```
Requirements
Add tests for:
new runtime state is `lifecycle.StateNew`
`Start(ctx)` transitions runtime state to `lifecycle.StateStarted`
`Stop(ctx)` transitions runtime state to `lifecycle.StateStopped`
repeated `Stop(ctx)` returns nil
nil runtime `State()` returns `lifecycle.StateFailed`
nil runtime `Start(ctx)` returns error matching `runtime.ErrInvalidOptions`
nil runtime `Stop(ctx)` returns error matching `runtime.ErrInvalidOptions`
Use `errors.Is` for error checks.
Use valid temp config path.
Do not assert exact log output.
Test log output to stderr during `go test` is expected and acceptable. Do not suppress it.
Verification
Run:
```bash
go test ./...
go build ./...
```
Expected:
```text
All tests pass.
Build succeeds.
```
Completion Report
Report:
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
TASK 09 — Refactor Run to use Runtime object
Goal
Make the top-level runtime entrypoint use the explicit runtime object.
Files
Modify:
```text
internal/runtime/runtime.go
```
Requirements
Refactor:
```go
func Run(ctx context.Context, opts Options) error
```
Expected behavior:
Create runtime using `New(opts)`.
If `New` fails, return the classified error.
Call `rt.Start(ctx)`.
If start fails, still call `rt.Stop(ctx)` before returning.
If start failed and stop succeeds, return the start error.
If start failed and stop also fails, return `errors.Join(startErr, stopErr)`.
If start succeeds, call `rt.Stop(ctx)`.
Return stop error if stop fails.
Return nil on success.
Important:
Preserve the original start error when both start and stop fail.
Do not hide start failure behind shutdown cleanup failure.
Do not call `os.Exit`.
Do not register production components.
Verification
Run:
```bash
go test ./...
go build ./...
```
Expected:
```text
All tests pass.
Build succeeds.
```
Completion Report
Report:
```text
TASK 09 completed.
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
TASK 10 — Add Run classification tests
Goal
Verify `Run(ctx, opts)` returns classified errors.
Files
Modify:
```text
internal/runtime/runtime_test.go
```
Requirements
Add or update tests for:
valid config path returns nil
missing config path returns error matching `runtime.ErrLoadConfig`
malformed config returns error matching `runtime.ErrLoadConfig`
invalid config returns error matching `runtime.ErrLoadConfig`
invalid logger config returns error matching `runtime.ErrLoadConfig`
negative startup timeout returns error matching `runtime.ErrInvalidOptions`
negative shutdown timeout returns error matching `runtime.ErrInvalidOptions`
Use `errors.Is`.
Do not rely only on string contains checks for error classification.
String contains checks may still be used only as secondary checks for useful context.
Verification
Run:
```bash
go test ./...
go build ./...
```
Expected:
```text
All tests pass.
Build succeeds.
```
Completion Report
Report:
```text
TASK 10 completed.
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
TASK 11 — Add runtime panic recovery helper
Goal
Make runtime operation boundaries recover panics and return classified errors.
Files
Modify:
```text
internal/runtime/runtime.go
```
Requirements
Add unexported helper:
```go
func recoverRuntimePanic(operation string, errp *error)
```
Suggested shape:
```go
func recoverRuntimePanic(operation string, errp *error) {
	if r := recover(); r != nil {
		panicErr := fmt.Errorf("%w: %s panic: %v", ErrRuntimePanic, operation, r)
		if *errp == nil {
			*errp = panicErr
		} else {
			*errp = errors.Join(*errp, panicErr)
		}
	}
}
```
Use named return values and defer in:
```go
func New(opts Options) (rt *Runtime, err error)
func (r *Runtime) Start(ctx context.Context) (err error)
func (r *Runtime) Stop(ctx context.Context) (err error)
func Run(ctx context.Context, opts Options) (err error)
```
Rules:
caller must always pass a valid non-nil `*error` pointer
`recoverRuntimePanic` must not be called with a nil pointer
if `Run` already has a non-nil named return error when a panic occurs, panic recovery must preserve the original error or join it with the panic error rather than overwriting it
safe approach: `if *errp == nil { *errp = panicErr } else { *errp = errors.Join(*errp, panicErr) }`
returned panic error must match `ErrRuntimePanic`
returned panic error must include operation name
returned panic error must include word `panic`
do not recover panics inside config/logger/lifecycle packages separately
do not re-panic
do not call `os.Exit`
Important:
This helper is a runtime safety net. It should not hide ordinary returned errors.
Verification
Run:
```bash
go test ./...
go build ./...
```
Expected:
```text
All tests pass.
Build succeeds.
```
Completion Report
Report:
```text
TASK 11 completed.
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
TASK 12 — Add runtime panic recovery tests
Goal
Verify runtime panic recovery helper behavior without adding fake production components.
Files
Create or modify:
```text
internal/runtime/runtime_internal_test.go
```
Package
Use internal package:
```go
package runtime
```
Requirements
Add tests for the unexported panic recovery helper through a small local test function.
Example test shape:
```go
func callWithRuntimeRecover(operation string, fn func()) (err error) {
	defer recoverRuntimePanic(operation, &err)
	fn()
	return nil
}
```
Test:
panic is recovered
returned error matches `ErrRuntimePanic`
returned error includes operation name
returned error includes `panic`
non-panicking function returns nil
if the named return error is already non-nil and a panic occurs, the returned error preserves the original error and also matches `ErrRuntimePanic`
Use `errors.Is` for classification.
Do not add fake production lifecycle components.
Do not add goroutines.
Verification
Run:
```bash
go test ./...
go build ./...
```
Expected:
```text
All tests pass.
Build succeeds.
```
Completion Report
Report:
```text
TASK 12 completed.
Files changed:
- internal/runtime/runtime_internal_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```
---
TASK 13 — Harden cmd/plomvix tests for enterprise runtime
Goal
Ensure main helper tests remain correct after runtime API hardening.
Files
Modify if needed:
```text
cmd/plomvix/main_test.go
```
Requirements
Review and update tests so they verify:
valid temp config path passed through `runtime.Options{ConfigPath: path}` returns nil
missing config path returns an error
missing config path error matches `runtime.ErrLoadConfig`
Important path rule:
do not use `runtime.DefaultOptions()` in the valid config test
Go runs package tests from `cmd/plomvix/`, not the project root
`runtime.DefaultOptions()` would look for `config.toml` in `cmd/plomvix/`
Use explicit temp config path for valid config test.
Do not test `main()` directly.
Do not call `os.Exit` from tests.
Do not add CLI flags.
Verification
Run:
```bash
go test ./...
go build ./...
```
Expected:
```text
All tests pass.
Build succeeds.
```
Completion Report
Report:
```text
TASK 13 completed.
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
TASK 14 — Harden runtime documentation
Goal
Document enterprise runtime behavior and hardening boundaries.
Files
Modify:
```text
docs/runtime.md
```
Requirements
Update runtime documentation to include:
enterprise runtime hardening
runtime object
runtime options
startup timeout
shutdown timeout
classified runtime errors
panic recovery
config loading
logger creation
lifecycle manager
zero-component lifecycle
no CLI flags
no environment overrides
no signal handling
no WAL
no storage
no query engine
no API server
no UI
The documentation must include these exact strings because Task 15 checks them:
```text
# Plomvix Runtime
runtime setup
core composition
enterprise runtime hardening
runtime object
runtime options
startup timeout
shutdown timeout
classified runtime errors
panic recovery
config loading
logger creation
lifecycle manager
zero-component lifecycle
config.toml
no CLI flags
no environment overrides
signal handling
WAL
storage
query engine
API server
UI
```
Non-goals section must clearly say runtime does not implement:
```text
signal handling
WAL
storage
query engine
API server
UI
CLI flags
environment overrides
```
Do not document future behavior as already implemented.
Verification
Run:
```bash
go test ./...
go build ./...
```
Expected:
```text
All tests pass.
Build succeeds.
```
Completion Report
Report:
```text
TASK 14 completed.
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
TASK 15 — Harden runtime documentation tests
Goal
Verify enterprise runtime documentation statements.
Files
Modify:
```text
internal/runtime/runtime_test.go
```
Package
Keep:
```go
package runtime_test
```
Requirements
Update the existing documentation test that reads:
```go
os.ReadFile("../../docs/runtime.md")
```
This path assumes the test runs from `internal/runtime/`, which is the default behavior of `go test ./...`.
Test that the document contains these exact strings:
```text
# Plomvix Runtime
runtime setup
core composition
enterprise runtime hardening
runtime object
runtime options
startup timeout
shutdown timeout
classified runtime errors
panic recovery
config loading
logger creation
lifecycle manager
zero-component lifecycle
config.toml
no CLI flags
no environment overrides
signal handling
WAL
storage
query engine
API server
UI
```
Use stable substring checks.
Do not make fragile checks for full paragraphs.
Verification
Run:
```bash
go test ./...
go build ./...
```
Expected:
```text
All tests pass.
Build succeeds.
```
Completion Report
Report:
```text
TASK 15 completed.
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
TASK 16 — Final enterprise runtime review
Goal
Review enterprise runtime hardening for correctness, classification, panic safety, scope control, and project cleanliness.
Files
Review only unless fixes are required:
```text
internal/runtime/runtime.go
internal/runtime/runtime_test.go
internal/runtime/runtime_internal_test.go
cmd/plomvix/main.go
cmd/plomvix/main_test.go
docs/runtime.md
go.mod
go.sum
```
Requirements
Confirm:
package `internal/runtime` exists
existing `Run(ctx, opts)` API remains compatible
`Options.ConfigPath` remains supported
`Options.StartupTimeout` exists
`Options.ShutdownTimeout` exists
`DefaultStartupTimeout` exists
`DefaultShutdownTimeout` exists
zero timeout values resolve to defaults
negative timeout values return `ErrInvalidOptions`
runtime error sentinels exist
config load failures match `ErrLoadConfig`
logger creation failures match `ErrCreateLogger` if they occur
lifecycle start failures match `ErrStartLifecycle`
lifecycle stop failures match `ErrStopLifecycle`
runtime panics match `ErrRuntimePanic`
error tests use `errors.Is`
`Runtime` struct exists
`Runtime` fields are unexported
`New(opts)` exists
`New(opts)` loads config
`New(opts)` creates logger
`New(opts)` creates lifecycle manager
`New(opts)` does not start lifecycle
`Runtime.Start(ctx)` exists
`Runtime.Stop(ctx)` exists
`Runtime.State()` exists
`Runtime.State()` handles nil runtime safely
`Run(ctx, opts)` uses `New`, `Start`, and `Stop`
`Run(ctx, opts)` calls `Stop` after start failure
`Run(ctx, opts)` preserves start error if stop also fails
panic recovery helper exists
`internal/runtime/runtime_internal_test.go` exists
panic recovery tests exist
panic recovery does not re-panic
`cmd/plomvix/main.go` still only calls `os.Exit` from `main`
`cmd/plomvix/main_test.go` uses explicit temp config path for valid config test
no CLI flags were added
no environment overrides were added
no signal handling was added
no goroutine supervision was added
no WAL was added
no storage was added
no query engine was added
no API server was added
no UI was added
no database engines were added
no external dependencies were added
config package does not import runtime
logger package does not import runtime
lifecycle package does not import runtime
runtime docs exist
runtime docs tests exist
runtime tests do not use `t.Chdir`
no root-level `tests/` directory exists
If issues are found:
Fix them.
Run final verification again.
Report what was fixed.
Final Verification
Run:
```bash
go test ./...
go build ./...
go test -race ./...
go mod tidy
go test ./...
```
Expected:
```text
All tests pass.
Build succeeds.
Race tests pass.
go mod tidy produces no unwanted dependency changes.
```
Completion Report
Report:
```text
TASK 16 completed.
Files reviewed:
- internal/runtime/runtime.go
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
- enterprise runtime hardening complete
- config/logger/lifecycle/runtime foundations are composed and hardened
- no database engine features added
```
---
Final Expected Structure
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
Completion Criteria
This plan is complete only when:
`internal/runtime/runtime.go` has enterprise runtime errors
runtime errors are compatible with `errors.Is`
`Options` supports startup and shutdown timeouts
timeout defaults are tested
negative timeouts are rejected
`Runtime` struct exists
`New(opts)` exists
`Runtime.Start(ctx)` exists
`Runtime.Stop(ctx)` exists
`Runtime.State()` exists
`Run(ctx, opts)` uses `New`, `Start`, and `Stop`
runtime panic recovery exists and is tested
`cmd/plomvix/main_test.go` still avoids `runtime.DefaultOptions()` for valid config path test
runtime documentation is hardened
runtime documentation test passes
tests avoid `t.Chdir`
`go test ./...` passes
`go build ./...` passes
`go test -race ./...` passes
`go mod tidy` produces no unwanted dependency changes
final `go test ./...` passes
no non-goal systems were introduced
---
Recommended Next Step After Completion
After `runtime_enterprise.md` is completed and verified, Plomvix has a composed and hardened core runtime foundation.
Do not automatically move to WAL, storage, query, API, UI, or engines unless the project owner confirms the next feature area.
Possible next directions after enterprise runtime hardening:
```text
1. choose the first workload engine direction
2. design minimal storage/WAL only when an engine requires it
3. create RDBMS metadata later when relational engine work begins
4. add OS signal handling later as a separate runtime hardening plan
```
For now, enterprise runtime hardening should only strengthen the runtime composition layer and stop there.