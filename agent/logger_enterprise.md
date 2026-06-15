# logger_enterprise.md

# Plomvix Enterprise Logger Hardening Plan

## Purpose

Harden the existing Plomvix logger foundation into a safer enterprise-grade internal logging package.

This plan improves consistency, safety, documentation, and future readiness.

This is still a logger-package hardening plan only.

Do not wire logging into lifecycle, API, WAL, storage engine, query engine, metrics engine, relational engine, or UI yet.

---

## Current Project State

The basic logger setup is complete.

Existing logger package:

```text
internal/logger
```

Existing logger files:

```text
internal/logger/logger.go
internal/logger/logger_test.go
internal/logger/logger_internal_test.go
```

Existing public logger API:

```go
func New(cfg config.LoggerConfig) (*slog.Logger, error)
```

Existing internal helper:

```go
func newWithWriter(cfg config.LoggerConfig, w io.Writer) (*slog.Logger, error)
```

Existing config section:

```toml
[logger]
level = "info"
format = "text"
output = "stdout"
```

Existing supported values:

```text
level:  debug, info, warn, error
format: text, json
output: stdout, stderr
```

Existing behavior:

* Uses Go standard library `log/slog`.
* No external logging dependency.
* Does not call `slog.SetDefault`.
* Supports text and JSON format.
* Supports stdout and stderr output.
* Rejects unsupported logger config values.
* Partial TOML preserves logger defaults.
* Unknown TOML fields are rejected by config loading.

---

## Go Version Requirement

This plan requires Go 1.21 or later because it uses:

```go
log/slog
```

The Plomvix project is expected to use Go 1.22, so this requirement is satisfied.

---

## Goal

Make the logger package safer and more consistent for future Plomvix components.

This plan adds:

* standard structured field constants
* component-scoped logger helper
* error attribute helper
* sensitive key constants
* redaction helper
* runtime level foundation
* logging policy documentation
* format vs output documentation
* unsupported output documentation
* enterprise logger tests

---

## Non-Goals

Do not implement:

* file output
* log rotation
* log retention
* async logging
* sampling
* rate limiting
* journald integration
* syslog integration
* network log shipping
* OpenTelemetry bridge
* request IDs
* trace IDs
* context propagation
* audit logging
* startup/shutdown wiring
* config hot reload
* per-component log levels
* API middleware logging
* storage/WAL/query logging
* lifecycle manager

These are future features and require other foundations first.

---

## Dependency Direction

Allowed:

```text
internal/logger imports internal/config
```

Forbidden:

```text
internal/config imports internal/logger
```

Do not create shared utility packages.

Do not add external logging dependencies.

Do not introduce global logger runtime state.

Package-level immutable constants or effectively immutable lookup tables are allowed.

Do not call `slog.SetDefault`.

---

## Graphify Rule

For every task:

1. Search Graphify before starting the task if Graphify is available.
2. Update Graphify after completing the task if Graphify is available.
3. If Graphify is unavailable, do not block the task.
4. Mention Graphify availability in the task report.

---

# TASK 01 — Add standard logger field constants

## Goal

Add standard structured logging field names to avoid inconsistent log attributes later.

## Files

Modify:

```text
internal/logger/logger.go
```

## Requirements

Add constants:

```go
const (
	FieldComponent = "component"
	FieldError     = "error"
	FieldPath      = "path"
	FieldDuration  = "duration_ms"
)
```

These constants should be exported because future Plomvix packages will use them.

Do not add unused complex field constants yet.

Do not add request ID, trace ID, tenant ID, engine ID, or shard ID fields yet.

Those belong to future features.

## Verification

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

---

# TASK 02 — Add standard logger field constant tests

## Goal

Verify logger field constants use stable expected names.

## Files

Modify:

```text
internal/logger/logger_test.go
```

## Package

Use existing package:

```go
package logger_test
```

## Requirements

Add tests confirming:

```text
FieldComponent == "component"
FieldError     == "error"
FieldPath      == "path"
FieldDuration  == "duration_ms"
```

Add a test comment:

```go
// These values are part of the stable logger API.
```

Do not access unexported logger internals.

## Verification

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

---

# TASK 03 — Add component-scoped logger helper

## Goal

Add a helper for creating component-scoped child loggers.

## Files

Modify:

```text
internal/logger/logger.go
```

## Requirements

Add:

```go
func WithComponent(base *slog.Logger, component string) *slog.Logger
```

Behavior:

* If `base` is not nil, return `base.With(FieldComponent, component)`.
* If `base` is nil, return a safe logger using `slog.New(slog.NewTextHandler(io.Discard, nil))` or equivalent safe discard behavior.
* Do not call `slog.Default`.
* Do not call `slog.SetDefault`.
* Do not mutate the original logger.
* Do not validate known component names yet.
* Do not create a component registry.

Expected future usage:

```go
log := logger.WithComponent(base, "config")
```

Expected structured field:

```text
component=config
```

## Verification

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

---

# TASK 04 — Add component-scoped logger tests

## Goal

Verify component-scoped loggers add the standard component field.

## Files

Modify:

```text
internal/logger/logger_internal_test.go
```

## Package

Use:

```go
package logger
```

## Requirements

Use `newWithWriter` and a buffer.

Test text output:

* Create base logger.
* Wrap it using:

```go
WithComponent(base, "config")
```

* Log message:

```text
hello
```

* Verify output contains:

```text
component=config
```

Test nil base behavior:

* Call:

```go
WithComponent(nil, "config")
```

* Verify it returns a non-nil logger.
* Verify logging does not panic.

Do not write to real stdout.

Do not write to real stderr.

## Verification

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

---

# TASK 05 — Add error attribute helper

## Goal

Add a standard helper for logging errors consistently.

## Files

Modify:

```text
internal/logger/logger.go
```

## Requirements

Add:

```go
func ErrorAttr(err error) slog.Attr
```

Behavior:

* If `err != nil`, return an attr with key `FieldError` and value `err.Error()`.
* If `err == nil`, return an attr with key `FieldError` and empty string value.
* Implement this using `slog.StringValue(err.Error())` for non-nil errors.
* Implement nil error using `slog.StringValue("")`.
* Must not panic on nil error.
* Must not use key `"err"`.
* Must not use key `"error_msg"`.
* Must not use key `"exception"`.
* Do not use `slog.AnyValue(err)` here, because tests expect the plain error message string.

Expected usage:

```go
log.Error("load config failed", logger.ErrorAttr(err))
```

## Verification

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

---

# TASK 06 — Add error attribute tests

## Goal

Verify `ErrorAttr` behavior.

## Files

Modify:

```text
internal/logger/logger_test.go
```

## Package

Use:

```go
package logger_test
```

## Requirements

Test non-nil error:

```go
attr := logger.ErrorAttr(errors.New("boom"))
```

Verify:

```text
attr.Key == "error"
attr.Value.String() == "boom"
```

This expectation depends on Task 05 implementing `ErrorAttr` with `slog.StringValue(err.Error())`.

Test nil error:

```go
attr := logger.ErrorAttr(nil)
```

Verify:

```text
attr.Key == "error"
attr.Value.String() == ""
```

Nil error must not panic.

## Verification

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

---

# TASK 07 — Add sensitive key constants

## Goal

Define standard sensitive keys that must be redacted in logs.

## Files

Modify:

```text
internal/logger/logger.go
```

## Requirements

Add sensitive key constants or a package-level lookup table.

Initial sensitive keys:

```text
password
passwd
secret
token
api_key
apikey
authorization
cookie
set-cookie
```

Acceptable implementation:

```go
var sensitiveKeys = map[string]struct{}{
	"password":      {},
	"passwd":        {},
	"secret":        {},
	"token":         {},
	"api_key":       {},
	"apikey":        {},
	"authorization": {},
	"cookie":        {},
	"set-cookie":    {},
}
```

Also add:

```go
const RedactedValue = "[REDACTED]"
```

Rules:

* Sensitive key matching should be case-insensitive.
* Do not export the sensitive key map unless needed.
* `RedactedValue` may be exported because tests and future packages may need it.
* Do not implement a custom slog handler in this task.
* The `sensitiveKeys` map is initialized once and must never be modified after package init.
* Treat `sensitiveKeys` as effectively immutable package-level lookup data, not runtime logger state.

## Verification

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

---

# TASK 08 — Add redaction helper

## Goal

Add a small helper for redacting sensitive structured log attributes.

## Files

Modify:

```text
internal/logger/logger.go
```

## Requirements

Add:

```go
func RedactAttr(attr slog.Attr) slog.Attr
```

Behavior:

* If `attr.Key` is sensitive, return an attr with the same key and value `RedactedValue`.
* If `attr.Key` is not sensitive, return the original attr unchanged.
* Sensitive key comparison must be case-insensitive.
* Must handle empty keys safely.
* Must not panic.
* Must not mutate unrelated attributes.
* Do not build a full custom slog handler yet.
* Do not automatically redact every log call yet.

Example:

```go
logger.RedactAttr(slog.String("password", "abc"))
```

Expected result:

```text
password=[REDACTED]
```

Non-sensitive example:

```go
logger.RedactAttr(slog.String("path", "/tmp/data"))
```

Expected result:

```text
path=/tmp/data
```

## Verification

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

---

# TASK 09 — Add redaction tests

## Goal

Verify sensitive attributes are redacted consistently.

## Files

Modify:

```text
internal/logger/logger_test.go
```

## Package

Use:

```go
package logger_test
```

## Requirements

Add table-driven tests.

Sensitive keys should redact:

```text
password
passwd
secret
token
api_key
apikey
authorization
cookie
set-cookie
```

Also test case-insensitive variants:

```text
Password
AUTHORIZATION
Set-Cookie
```

Expected value:

```text
[REDACTED]
```

Non-sensitive keys should not redact:

```text
path
component
duration_ms
message
```

Empty key should not panic and should not redact.

## Verification

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

---

# TASK 10 — Add runtime level foundation

## Goal

Prepare the logger for future runtime log level changes using `slog.LevelVar`.

Do not implement config reload yet.

## Files

Modify:

```text
internal/logger/logger.go
```

## Requirements

Introduce standalone level variable support.

Add type:

```go
type LevelController struct {
	level slog.LevelVar
}
```

Add constructor:

```go
func NewLevelController(level string) (*LevelController, error)
```

Add method:

```go
func (c *LevelController) HandlerOptions() *slog.HandlerOptions
```

Add method:

```go
func (c *LevelController) SetLevel(level string) error
```

Behavior:

* Use `slog.LevelVar`.
* Support existing levels only:

  * debug
  * info
  * warn
  * error
* Invalid levels return:

```text
logger.level must be one of: debug, info, warn, error
```

Relationship to existing logger construction:

* `LevelController` is a standalone foundation type for future runtime level changes.
* Do not wire `LevelController` into `New(cfg)` in this plan.
* `New(cfg)` should continue to behave exactly as it does today.
* Future lifecycle/config-reload work may use `LevelController` to construct handlers with runtime-mutable levels.
* Task 11 must prove `LevelController` works by building a test handler using `c.HandlerOptions()`.

Rules:

* Do not wire this into config reload.
* Do not expose global mutable state.
* Do not change `New(cfg)` behavior.
* Do not add per-component log levels.
* Do not add watchers.
* Do not add goroutines.
* Do not add background reload.

## Verification

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

---

# TASK 11 — Add runtime level foundation tests

## Goal

Verify runtime level foundation behavior.

## Files

Modify:

```text
internal/logger/logger_internal_test.go
```

## Package

Use:

```go
package logger
```

## Requirements

Test:

* `NewLevelController("debug")` succeeds.
* `NewLevelController("info")` succeeds.
* `NewLevelController("warn")` succeeds.
* `NewLevelController("error")` succeeds.
* `NewLevelController("trace")` fails.
* `SetLevel("debug")` enables debug logs.
* `SetLevel("info")` suppresses debug logs.
* Invalid `SetLevel("trace")` returns exact expected error.

Use buffer-backed slog handler.

Important construction pattern:

* Create `LevelController`.
* Build the test handler using `c.HandlerOptions()`.
* Create a logger from that handler.
* Call `SetLevel()` on the same controller.
* Verify the output behavior changes.

Example shape:

```go
controller, err := NewLevelController("info")
handler := slog.NewTextHandler(&buf, controller.HandlerOptions())
log := slog.New(handler)

controller.SetLevel("debug")
log.Debug("debug visible")
```

Do not write to stdout.

Do not write to stderr.

Do not implement config reload.

## Verification

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

---

# TASK 12 — Clarify format vs output in config docs

## Goal

Make it clear that `format` and `output` are different concepts.

## Files

Modify:

```text
docs/config.md
```

## Requirements

Document:

```toml
[logger]
level = "info"
format = "text"
output = "stdout"
```

Explain:

```text
level  = how much to log
format = how logs are encoded
output = where logs are written
```

Current supported values:

```text
level:  debug, info, warn, error
format: text, json
output: stdout, stderr
```

Use these exact phrases in the documentation:

```text
stdout is an output, not a format.
stderr is an output, not a format.
text and json are formats.
```

Document examples:

```toml
[logger]
level = "debug"
format = "text"
output = "stdout"
```

```toml
[logger]
level = "info"
format = "json"
output = "stderr"
```

Do not add new supported config values.

Do not implement file output.

## Verification

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

---

# TASK 13 — Document unsupported enterprise outputs

## Goal

Document unsupported logger outputs clearly.

## Files

Modify:

```text
docs/config.md
```

## Requirements

Document that these outputs are intentionally not supported yet:

```text
file
discard
journald
syslog
network
```

Reason:

```text
file output needs path policy, permissions, rotation, disk-full behavior, and cleanup rules.
journald/syslog need platform-specific design.
network output needs retry, backpressure, timeout, and drop policy.
discard output is useful for tests but should not be exposed as user config yet.
```

Do not implement these outputs.

Do not add these values to validation.

Do not change `config.toml`.

Do not change `config.example.toml`.

## Verification

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

---

# TASK 14 — Add logging policy documentation

## Goal

Create a clear enterprise logging policy for future Plomvix development.

## Files

Create:

```text
docs/logging.md
```

## Requirements

Document these rules:

```text
Use structured logging fields instead of formatted strings.
Use component-scoped loggers.
Use logger.ErrorAttr for errors.
Use logger.RedactAttr for sensitive attributes.
Never log passwords, tokens, API keys, cookies, or authorization headers.
Do not log full payloads by default.
Do not log high-cardinality values carelessly.
Do not log secrets even at debug level.
Prefer stable field names from internal/logger constants.
Do not use slog.SetDefault.
Do not add external logging dependencies without a dedicated plan.
```

Document supported levels:

```text
debug
info
warn
error
```

Document supported formats:

```text
text
json
```

Document supported outputs:

```text
stdout
stderr
```

Use these exact phrases in `docs/logging.md` because Task 15 checks them:

```text
structured logging
component-scoped loggers
ErrorAttr
RedactAttr
stdout
stderr
text
json
unsupported outputs
stdout is an output
text and json are formats
```

Document unsupported outputs:

```text
file
discard
journald
syslog
network
```

Document future-only features:

```text
file output
log rotation
runtime config reload
per-component log levels
request IDs
trace IDs
audit logging
OpenTelemetry bridge
```

Do not claim any future-only feature is implemented.

## Verification

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

---

# TASK 15 — Add logging documentation tests

## Goal

Verify logger documentation exists without introducing a heavy docs framework.

## Files

Modify:

```text
internal/logger/logger_test.go
```

## Package

Use:

```go
package logger_test
```

## Requirements

Add a small documentation test using:

```go
os.ReadFile("../../docs/logging.md")
```

This path assumes the test runs from `internal/logger/`, which is the default behavior of `go test ./...`.

Verify `docs/logging.md` contains these exact strings:

```text
structured logging
component-scoped loggers
ErrorAttr
RedactAttr
stdout
stderr
text
json
unsupported outputs
stdout is an output
text and json are formats
```

These exact strings are required by Task 14.

Do not create a root-level `tests/` directory.

Do not add external test dependencies.

## Verification

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

---

# TASK 16 — Final enterprise logger review

## Goal

Perform final enterprise logger review.

## Review Checklist

Confirm:

* Go version is 1.21 or later.
* Project remains compatible with Go 1.22.
* `internal/logger` still exists.
* Logger still uses `log/slog`.
* No external logging dependency was added.
* `slog.SetDefault` is not used.
* No global logger runtime state was introduced.
* `New(cfg config.LoggerConfig)` still exists.
* `New(cfg)` behavior was not changed for `LevelController`.
* `LevelController` is standalone foundation code.
* `newWithWriter` remains unexported.
* Field constants exist.
* Field constant tests mark values as stable API.
* `WithComponent` exists.
* `ErrorAttr` exists.
* `ErrorAttr` uses `slog.StringValue(err.Error())` for non-nil errors.
* Sensitive key list exists.
* Sensitive key list is treated as effectively immutable.
* `RedactAttr` exists.
* Runtime level foundation uses `slog.LevelVar`.
* Runtime level tests build the handler using `LevelController.HandlerOptions()`.
* Runtime level foundation does not implement config reload.
* stdout and stderr remain outputs only.
* text and json remain formats only.
* Unsupported outputs are documented but not implemented.
* Unsupported outputs remain rejected by validation.
* `docs/config.md` explains format vs output.
* `docs/config.md` documents unsupported outputs.
* `docs/logging.md` exists.
* `docs/logging.md` documents logging policy.
* `docs/logging.md` contains exact phrases required by documentation tests.
* `docs/logging.md` has a documentation test.
* No file output was added.
* No log rotation was added.
* No journald/syslog/network output was added.
* No API server was added.
* No lifecycle manager was added.
* No WAL/storage/query integration was added.
* No UI was added.
* No unrelated folders were created.
* `go.mod` has no new logging dependency.
* `go.sum` has no new logging dependency entries.

## Verification

Run:

```bash
go test ./...
go build ./...
go mod tidy
go test ./...
```

Expected:

```text
All tests pass.
Build succeeds.
go.mod contains no new logging dependency.
go.sum contains no new logging dependency entries.
```

---

# Final Expected Structure

After this plan, relevant structure should include:

```text
plomvix/
├── cmd/
│   └── plomvix/
│       └── main.go
├── docs/
│   ├── config.md
│   └── logging.md
├── internal/
│   ├── config/
│   │   ├── config.go
│   │   └── config_test.go
│   └── logger/
│       ├── logger.go
│       ├── logger_test.go
│       └── logger_internal_test.go
├── config.toml
├── config.example.toml
├── go.mod
├── go.sum
├── setup.md
├── config_setup.md
├── config_enterprise.md
├── logging_setup.md
├── logger_enterprise.md
└── README.md
```

---

# Completion Criteria

This feature is complete only when:

```bash
go test ./...
go build ./...
go mod tidy
go test ./...
```

all pass successfully.

The logger package should be more consistent, safer, and better documented.

No future Plomvix component should be implemented in this plan.
