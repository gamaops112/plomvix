runtime_setup.md
Plomvix Runtime Setup & Core Composition Plan
Purpose
Wire the completed core foundations together into the first minimal Plomvix runtime entrypoint.
This plan connects:
```text
config
logger
lifecycle
cmd/plomvix/main.go
```
The goal is to prove that Plomvix can load configuration, create a logger, create a lifecycle manager, start the lifecycle, stop the lifecycle, and exit cleanly.
This is still core foundation work only.
Do not add database engines.
Do not add WAL.
Do not add storage.
Do not add query execution.
Do not add API server.
Do not add UI.
Do not add OS signal handling yet.
Do not add background orchestration yet.
---
Feature Name
```text
Runtime Setup & Core Composition
```
Plan file:
```text
runtime_setup.md
```
Suggested package:
```text
internal/runtime
```
---
Required Starting State
This plan starts only after these plans are completed and verified:
```text
config_setup.md
config_enterprise.md
logging_setup.md
logger_enterprise.md
lifecycle.md
lifecycle_enterprise.md
```
Before starting this plan, the project must already have:
```text
internal/config/config.go
internal/config/config_test.go
internal/logger/logger.go
internal/logger/logger_test.go
internal/logger/logger_internal_test.go
internal/lifecycle/lifecycle.go
internal/lifecycle/lifecycle_test.go
docs/config.md
docs/logging.md
docs/lifecycle.md
config.toml
config.example.toml
cmd/plomvix/main.go
```
The config package must already expose:
```go
func Default() Config
func Validate(cfg Config) error
func Load(path string) (Config, error)
```
The logger package must already expose:
```go
func New(cfg config.LoggerConfig) (*slog.Logger, error)
func WithComponent(base *slog.Logger, component string) *slog.Logger
func ErrorAttr(err error) slog.Attr
```
The lifecycle package must already expose:
```go
type Component interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

func NewManager() *Manager
func (m *Manager) Register(component Component) error
func (m *Manager) Start(ctx context.Context) error
func (m *Manager) Stop(ctx context.Context) error
func (m *Manager) State() State
```
If this starting state is not true, stop and report which prerequisite is incomplete.
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
```
Current stage:
```text
core foundation only
```
Current feature area:
```text
runtime setup and core composition
```
---
Go Version Requirement
Plomvix uses:
```text
Go 1.22 or later
```
Use only Go standard library plus the dependencies already required by completed plans.
Do not add new external dependencies in this plan.
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
Keep runtime composition minimal.
Do not add future placeholders.
Do not add unrelated folders.
Do not create database engines.
Do not create WAL code.
Do not create storage code.
Do not create query code.
Do not create API server code.
Do not create UI code.
Do not add OS signal handling.
Do not add background goroutine supervision.
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
Do not make config, logger, or lifecycle depend on runtime.
Runtime is a composition layer above the foundational packages.
---
Design Decision: Minimal Runtime Composition
The runtime package should compose existing foundations without becoming an engine.
Runtime is allowed to:
load config from a provided path
create a logger from config
create a lifecycle manager
start lifecycle manager
stop lifecycle manager
return clear errors
expose a small `Run(ctx, opts)` function
Runtime is not allowed to:
own database storage
own WAL
own query execution
own API server
own UI
own OS signal handling
own background service supervision
own engine registration
---
Runtime Behavior
Final runtime behavior:
```text
1. Read runtime options.
2. Resolve config path.
3. Load config using internal/config.Load.
4. Create base logger using internal/logger.New.
5. Create component-scoped runtime logger using logger.WithComponent(base, "runtime").
6. Create lifecycle manager using lifecycle.NewManager.
7. Start lifecycle manager with provided context.
8. Stop lifecycle manager before returning.
9. Return any startup or shutdown error to caller.
```
Important:
```text
No lifecycle components are registered in this plan.
```
Reason:
config, logger, and lifecycle are foundations
engines are not implemented yet
no database services exist yet
zero-component lifecycle start/stop proves composition without inventing fake production services
---
Config Path Policy
Runtime should default to:
```text
config.toml
```
Add runtime constant:
```go
const DefaultConfigPath = "config.toml"
```
Do not add CLI flags yet.
Do not add environment overrides yet.
Do not search multiple config paths yet.
Do not silently fall back to defaults if `config.toml` is missing.
If the config file is missing, runtime must return an error.
---
Final Runtime API
Create package:
```text
internal/runtime
```
Final public API:
```go
type Options struct {
	ConfigPath string
}

const DefaultConfigPath = "config.toml"

func DefaultOptions() Options
func Run(ctx context.Context, opts Options) error
```
Expected behavior:
`DefaultOptions()` returns `Options{ConfigPath: DefaultConfigPath}`.
`Run(ctx, opts)` loads config from `opts.ConfigPath`.
If `opts.ConfigPath` is empty, `Run` uses `DefaultConfigPath`.
`Run` returns errors instead of panicking.
`Run` does not call `os.Exit`.
`Run` does not start goroutines.
`Run` does not register fake production components.
---
Main Entrypoint Behavior
Update:
```text
cmd/plomvix/main.go
```
Expected behavior:
main creates `context.Background()`.
main calls a small testable helper.
helper calls `runtime.Run(ctx, runtime.DefaultOptions())`.
on success, process exits normally.
on error, main writes the error to `stderr` and exits with non-zero status.
Suggested shape:
```go
func main() {
	if err := run(context.Background(), runtime.DefaultOptions()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, opts runtime.Options) error {
	return runtime.Run(ctx, opts)
}
```
Rules:
`runtime.Run` must not call `os.Exit`.
Only `main` may call `os.Exit`.
Keep `run(ctx, opts)` small so it can be tested without exiting the test process.
Do not add CLI flags.
Do not add signal handling.
---
Error Behavior
Runtime errors should preserve context.
Expected wrapping examples:
```text
load config: ...
create logger: ...
start lifecycle: ...
stop lifecycle: ...
```
Rules:
Do not panic.
Do not swallow errors.
Do not log and return duplicate noisy errors in tests.
Use `%w` where useful.
Return stop errors from `Run` if start succeeded and stop failed.
If lifecycle start fails, still call `manager.Stop(ctx)` before returning, because `Stop` is safe from any lifecycle state after enterprise lifecycle hardening.
Return the start error wrapped as `start lifecycle: ...`.
If both start and stop fail, prioritize returning the start error; do not hide the original start failure behind shutdown cleanup noise in this minimal runtime plan.
Because no components are registered in this plan, lifecycle start/stop should normally succeed unless context behavior or lifecycle implementation changes later.
---
Logging Behavior
Runtime may log minimal lifecycle messages.
Allowed log messages:
```text
runtime starting
runtime started
runtime stopping
runtime stopped
```
Rules:
Use component-scoped logger:
```go
log := logger.WithComponent(base, "runtime")
```
Use structured logging fields where useful.
Use `logger.ErrorAttr(err)` when logging errors.
Do not call `slog.SetDefault`.
Do not create a global logger.
Do not add request IDs.
Do not add trace IDs.
Do not add logging middleware.
If tests emit runtime logs to stdout/stderr, that is expected and acceptable during this plan.
Do not suppress test log output by adding hidden runtime switches, changing logger behavior, or adding a `discard` logger output to config.
Do not add a `discard` logger output to config.
---
Documentation Requirement
Create:
```text
docs/runtime.md
```
Documentation must explain:
runtime setup purpose
config loading
logger creation
lifecycle manager creation
zero-component lifecycle behavior
fixed config path policy
no CLI flags yet
no environment overrides yet
no signal handling yet
no WAL
no storage
no query engine
no API server
no UI
Exact strings required by docs tests:
```text
# Plomvix Runtime
runtime setup
core composition
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
---
Task Plan
---
TASK 01 — Create runtime package skeleton
Goal
Create the runtime package with final API shape.
Files
Create:
```text
internal/runtime/runtime.go
```
Requirements
Add package:
```go
package runtime
```
Add:
```go
const DefaultConfigPath = "config.toml"
```
Add:
```go
type Options struct {
	ConfigPath string
}
```
Add:
```go
func DefaultOptions() Options
```
Add stub:
```go
func Run(ctx context.Context, opts Options) error
```
Stub behavior:
`DefaultOptions()` returns `Options{ConfigPath: DefaultConfigPath}`.
`Run(ctx, opts)` returns nil for now.
`Run` must accept context even if unused in this task.
Do not load config yet.
Do not create logger yet.
Do not create lifecycle manager yet.
Do not modify `cmd/plomvix/main.go` yet.
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
TASK 02 — Add runtime options tests
Goal
Verify runtime options and defaults.
Files
Create:
```text
internal/runtime/runtime_test.go
```
Package
Use external test package:
```go
package runtime_test
```
Requirements
Add tests for:
`runtime.DefaultConfigPath == "config.toml"`
`runtime.DefaultOptions().ConfigPath == runtime.DefaultConfigPath`
`runtime.Run(context.Background(), runtime.Options{})` succeeds while still stubbed
Do not test config loading yet.
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
TASK 03 — Add config loading to runtime
Goal
Make runtime load config using the existing config package.
Files
Modify:
```text
internal/runtime/runtime.go
```
Requirements
Update `Run(ctx, opts)` to:
Resolve config path.
Use `DefaultConfigPath` when `opts.ConfigPath` is empty.
Load config using:
```go
config.Load(path)
```
Return wrapped error on load failure:
```text
load config: ...
```
Return nil on successful config load for now.
Rules:
Do not manually parse TOML in runtime.
Do not duplicate config validation.
Do not add environment overrides.
Do not add CLI flags.
Do not modify `cmd/plomvix/main.go` yet.
Expected imports must include the config package because `config.Load` is required:
```go
context
fmt

"github.com/plomvix/plomvix/internal/config"
```
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
TASK 04 — Add runtime config loading tests
Goal
Verify runtime config loading behavior.
Files
Modify:
```text
internal/runtime/runtime_test.go
```
Requirements
Add tests using `t.TempDir()` and `os.WriteFile`.
Test valid config path:
```toml
[server]
host = "127.0.0.1"
port = 8080

[data]
path = "./data"

[logger]
level = "info"
format = "text"
output = "stderr"
```
Expected:
```go
runtime.Run(context.Background(), runtime.Options{ConfigPath: path}) == nil
```
Test missing config path:
```go
runtime.Run(context.Background(), runtime.Options{ConfigPath: missingPath})
```
Expected:
returns error
error contains `load config`
Test invalid config:
```toml
[server]
port = 70000
```
Expected:
returns error
error contains `load config`
Important:
Use `output = "stderr"` in test config if logs are later added.
Test log output to stderr during `go test` is expected and acceptable. Do not suppress it.
Do not assert exact logger output.
Do not add a `discard` output.
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
TASK 05 — Add logger creation to runtime
Goal
Make runtime create the configured logger.
Files
Modify:
```text
internal/runtime/runtime.go
```
Requirements
After config loading, create logger using:
```go
baseLog, err := logger.New(cfg.Logger)
```
Then create component-scoped runtime logger:
```go
log := logger.WithComponent(baseLog, "runtime")
```
Runtime may log:
```text
runtime starting
```
Rules:
Do not call `slog.SetDefault`.
Do not create global logger state.
Do not mutate config.
Do not duplicate logger validation.
Return wrapped logger construction error:
```text
create logger: ...
```
Expected imports must include the logger package because `logger.New` and `logger.WithComponent` are required:
```go
"github.com/plomvix/plomvix/internal/logger"
```
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
TASK 06 — Add runtime logger behavior tests
Goal
Verify runtime returns logger creation errors through config validation path.
Files
Modify:
```text
internal/runtime/runtime_test.go
```
Requirements
Add test for invalid logger config:
```toml
[server]
host = "127.0.0.1"
port = 8080

[data]
path = "./data"

[logger]
level = "trace"
format = "text"
output = "stderr"
```
Expected:
`runtime.Run` returns error
error contains `load config`
Reason:
config validation should reject invalid logger values before logger construction
runtime must not duplicate logger validation
Add test for valid JSON logger config:
```toml
[server]
host = "127.0.0.1"
port = 8080

[data]
path = "./data"

[logger]
level = "debug"
format = "json"
output = "stderr"
```
Expected:
```go
runtime.Run(context.Background(), runtime.Options{ConfigPath: path}) == nil
```
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
TASK 07 — Add lifecycle manager composition
Goal
Make runtime create, start, and stop the lifecycle manager.
Files
Modify:
```text
internal/runtime/runtime.go
```
Requirements
After logger creation:
Create manager:
```go
manager := lifecycle.NewManager()
```
Start manager.
If `manager.Start(ctx)` returns an error, still call `manager.Stop(ctx)` before returning, because `Stop` is safe from any lifecycle state after enterprise lifecycle hardening.
Return the start error wrapped as:
```text
start lifecycle: ...
```
Suggested shape:
```go
if err := manager.Start(ctx); err != nil {
	_ = manager.Stop(ctx)
	return fmt.Errorf("start lifecycle: %w", err)
}
```
Stop manager before returning:
```go
if err := manager.Stop(ctx); err != nil {
	return fmt.Errorf("stop lifecycle: %w", err)
}
```
Log minimal messages if useful:
```text
runtime started
runtime stopping
runtime stopped
```
Rules:
Do not register any production components in this plan.
Do not create fake runtime components in production code.
Do not add goroutines.
Do not add signal handling.
Do not add background service loops.
Do not add sleep/wait behavior.
Expected imports must include the lifecycle package because `lifecycle.NewManager` is required:
```go
"github.com/plomvix/plomvix/internal/lifecycle"
```
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
TASK 08 — Harden runtime tests after lifecycle composition
Goal
Ensure runtime tests still prove full composition succeeds.
Files
Modify:
```text
internal/runtime/runtime_test.go
```
Requirements
Review and update existing tests if needed so they verify final runtime behavior:
valid text logger config succeeds
valid JSON logger config succeeds
missing config returns error containing `load config`
invalid config returns error containing `load config`
invalid logger config returns error containing `load config`
empty `Options.ConfigPath` attempts `config.toml`
For empty config path behavior:
temporarily change the working directory to a temp directory containing `config.toml`
do not use `t.Chdir`, because this project targets Go 1.22 and `t.Chdir` is not available there
use the manual `os.Getwd` / `os.Chdir` / deferred restore pattern
write valid `config.toml`
call:
```go
runtime.Run(context.Background(), runtime.Options{})
```
Expected:
```text
Run succeeds using DefaultConfigPath.
```
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
Rules:
Do not use sleeps.
Do not test exact log output.
Do not add root-level tests directory.
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
TASK 09 — Wire cmd/plomvix main to runtime
Goal
Make the actual Plomvix binary use runtime composition.
Files
Modify:
```text
cmd/plomvix/main.go
```
Create if needed:
```text
cmd/plomvix/main_test.go
```
Requirements
Update `main.go` so:
`main()` calls a small helper named `run`.
`run(ctx, opts)` calls `runtime.Run(ctx, opts)`.
`main()` uses `runtime.DefaultOptions()`.
`main()` writes returned errors to `stderr`.
`main()` exits non-zero on error.
`run` returns an error and does not call `os.Exit`.
Suggested shape:
```go
func main() {
	if err := run(context.Background(), runtime.DefaultOptions()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, opts runtime.Options) error {
	return runtime.Run(ctx, opts)
}
```
Add tests in `cmd/plomvix/main_test.go`:
valid temp config makes `run(ctx, opts)` return nil
missing config makes `run(ctx, opts)` return error
Important test path rule:
the valid config test must create a temp config file and pass it explicitly with `runtime.Options{ConfigPath: path}`
do not use `runtime.DefaultOptions()` in the valid config test, because Go runs package tests from `cmd/plomvix/`, not the project root where root `config.toml` lives
only tests that intentionally verify default-path behavior may rely on the working directory
Rules:
Do not add CLI flags.
Do not add signal handling.
Do not parse environment variables.
Do not call `os.Exit` from tests.
Do not test `main()` directly.
Test `run()` helper instead.
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
- cmd/plomvix/main.go
- cmd/plomvix/main_test.go

Verification:
- go test ./...
- go build ./...

Graphify:
- searched: yes/no/unavailable
- updated: yes/no/unavailable
```
---
TASK 10 — Add runtime documentation
Goal
Document runtime setup behavior and scope boundaries.
Files
Create:
```text
docs/runtime.md
```
Requirements
Create documentation with heading:
```text
# Plomvix Runtime
```
Documentation must explain:
runtime setup purpose
core composition role
config loading from `config.toml`
logger creation from config
lifecycle manager creation
zero-component lifecycle behavior
fixed config path policy
no CLI flags yet
no environment overrides yet
no signal handling yet
non-goals
The documentation must include these exact strings:
```text
runtime setup
core composition
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
Non-goals section must clearly say runtime setup does not implement:
```text
signal handling
WAL
storage
query engine
API server
UI
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
TASK 10 completed.
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
TASK 11 — Add runtime documentation tests
Goal
Ensure runtime documentation exists and covers required behavior.
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
Add a documentation test that reads:
```go
os.ReadFile("../../docs/runtime.md")
```
Path note:
```text
This path assumes the test runs from internal/runtime/, which is the default behavior of go test ./...
```
Test that the document contains these exact strings:
```text
# Plomvix Runtime
runtime setup
core composition
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
TASK 11 completed.
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
TASK 12 — Final runtime setup review
Goal
Review runtime setup for correctness, scope control, and project cleanliness.
Files
Review only unless fixes are required:
```text
internal/runtime/runtime.go
internal/runtime/runtime_test.go
cmd/plomvix/main.go
cmd/plomvix/main_test.go
docs/runtime.md
go.mod
go.sum
```
Requirements
Confirm:
package `internal/runtime` exists
runtime has `DefaultConfigPath`
runtime has `Options`
runtime has `DefaultOptions()`
runtime has `Run(ctx, opts)`
`Run` loads config through `config.Load`
`Run` creates logger through `logger.New`
`Run` uses `logger.WithComponent(base, "runtime")`
`Run` creates lifecycle manager through `lifecycle.NewManager`
`Run` starts lifecycle manager
`Run` calls lifecycle stop even if lifecycle start fails
`Run` stops lifecycle manager before returning
no production lifecycle components are registered
zero-component lifecycle behavior is intentional
`Run` returns errors instead of panicking
`Run` does not call `os.Exit`
only `main` calls `os.Exit`
`main` writes startup errors to `stderr`
`main` uses `runtime.DefaultOptions()`
`run(ctx, opts)` helper is testable
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
runtime tests do not use `t.Chdir`, because Go 1.22 is the target
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
TASK 12 completed.
Files reviewed:
- internal/runtime/runtime.go
- internal/runtime/runtime_test.go
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
- runtime setup complete
- config/logger/lifecycle foundations are composed
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
│       └── runtime_test.go
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
`internal/runtime/runtime.go` exists
`internal/runtime/runtime_test.go` exists
`cmd/plomvix/main.go` is wired to runtime
`cmd/plomvix/main_test.go` exists
`docs/runtime.md` exists
runtime documentation test passes
runtime loads config through config package
runtime creates logger through logger package
runtime creates lifecycle manager through lifecycle package
lifecycle start/stop are called by runtime
lifecycle stop is called even after lifecycle start failure
zero production components are registered
tests avoid `t.Chdir` and use Go 1.22-compatible working-directory handling
`go test ./...` passes
`go build ./...` passes
`go test -race ./...` passes
`go mod tidy` produces no unwanted dependency changes
final `go test ./...` passes
no non-goal systems were introduced
---
Recommended Next Step After Completion
After `runtime_setup.md` is completed and verified, the core foundation has enough composition to run as a minimal binary.
Do not automatically move to WAL, storage, query, API, UI, or engines unless the project owner confirms the next feature area.
Possible next directions after runtime setup:
```text
1. choose first workload engine direction
2. create a minimal storage/WAL plan only when engine direction needs it
3. create RDBMS metadata later when relational engine work begins
4. add OS signal handling later as a separate runtime hardening plan
```
For now, runtime setup should only compose existing foundations and stop there.