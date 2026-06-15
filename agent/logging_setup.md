# logging_setup.md

# Plomvix Logging Setup Plan

## Purpose

Add the first production-grade logging foundation for Plomvix.

This feature introduces a small internal logger package and extends the existing configuration system with logger settings.

This is a core-adjacent foundation task only.

Do not build a full observability stack.
Do not add API logging middleware.
Do not add request logging.
Do not add WAL logging.
Do not add storage logging.
Do not add query logging.
Do not add server startup wiring.
Do not add external logging dependencies.

Use Go standard library `log/slog`.

---

## Go Version Requirement

This plan requires Go 1.21 or later because it uses:

```go
log/slog
```

The current Plomvix project is expected to use Go 1.22, so this requirement is satisfied.

---

## Current Project State

The configuration system is complete.

Existing config API:

```go
func Default() Config
func Validate(cfg Config) error
func Load(path string) (Config, error)
```

Existing config package:

```text
internal/config
```

Existing config type:

```go
type Config struct {
	Server ServerConfig `toml:"server"`
	Data   DataConfig   `toml:"data"`
}
```

Existing load order:

```text
Default → Decode TOML → Normalize → Validate → Return
```

Known config behavior:

* Unknown TOML fields are rejected.
* Partial TOML files are allowed.
* Missing TOML sections preserve defaults.
* `Data.Path` is normalized with `filepath.Clean`.
* Root `config.toml` exists.
* Root `config.example.toml` exists.
* `docs/config.md` exists.
* `setup.md` should still exist from the initial setup plan.

---

## Goal

Create a minimal logging foundation that future Plomvix components can use consistently.

The logger should support:

* configurable log level
* configurable log format
* configurable output
* safe defaults
* validation through the existing config system
* safe fallback errors inside the logger package
* no external dependency

---

## Non-Goals

Do not implement:

* logging middleware
* request IDs
* tracing
* metrics
* log rotation
* file output
* async logging
* custom log encoder
* structured event catalog
* lifecycle manager
* API server integration
* storage integration
* WAL integration
* query integration
* UI integration
* global process startup wiring
* external logging package

---

## Final Logger Config

Add this config section:

```go
type LoggerConfig struct {
	Level  string `toml:"level"`
	Format string `toml:"format"`
	Output string `toml:"output"`
}
```

Extend root config:

```go
type Config struct {
	Server ServerConfig `toml:"server"`
	Data   DataConfig   `toml:"data"`
	Logger LoggerConfig `toml:"logger"`
}
```

Default values:

```text
Logger.Level  = "info"
Logger.Format = "text"
Logger.Output = "stdout"
```

Allowed values:

```text
level:  debug, info, warn, error
format: text, json
output: stdout, stderr
```

Validation errors:

```text
logger.level must be one of: debug, info, warn, error
logger.format must be one of: text, json
logger.output must be one of: stdout, stderr
```

Logger values are case-sensitive.

Do not silently lowercase config values.

Invalid casing should fail validation.

---

## Final Logger Package

Create:

```text
internal/logger/
```

Expected files:

```text
internal/logger/logger.go
internal/logger/logger_test.go
internal/logger/logger_internal_test.go
```

Expected public API:

```go
func New(cfg config.LoggerConfig) (*slog.Logger, error)
```

Expected internal test helper:

```go
func newWithWriter(cfg config.LoggerConfig, w io.Writer) (*slog.Logger, error)
```

Expected behavior:

* `New` returns a configured `*slog.Logger`.
* `New` must not set the global default logger.
* `New` must not mutate config.
* `New` must not emit logs during construction.
* `New` must support text and JSON handlers.
* `New` must support stdout and stderr.
* `New` must support debug, info, warn, and error levels.
* Invalid config should return an error.
* Config validation remains the primary validation layer.
* Logger package invalid-value handling is a safety fallback only.

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

Do not create premature abstractions.

Do not add external logging dependencies.

---

## Graphify Rule

For every task:

1. Search Graphify before starting the task if Graphify is available.
2. Update Graphify after completing the task if Graphify is available.
3. If Graphify is unavailable, do not block the task.
4. Mention Graphify availability in the task report.

---

# TASK 01 — Add logger config type and defaults

## Goal

Add logger configuration fields to the existing config system.

## Files

Modify:

```text
internal/config/config.go
```

## Requirements

Add:

```go
type LoggerConfig struct {
	Level  string `toml:"level"`
	Format string `toml:"format"`
	Output string `toml:"output"`
}
```

Update:

```go
type Config struct {
	Server ServerConfig `toml:"server"`
	Data   DataConfig   `toml:"data"`
	Logger LoggerConfig `toml:"logger"`
}
```

Update `Default()` so it returns:

```text
Logger.Level  = "info"
Logger.Format = "text"
Logger.Output = "stdout"
```

Do not modify TOML loading logic in this task.

Do not create the logger package in this task.

Do not modify `cmd/plomvix/main.go`.

## Verification

Run:

```bash
go test ./...
```

Expected:

```text
All tests pass.
```

---

# TASK 02 — Add logger default tests

## Goal

Verify logger defaults are part of the config defaults.

## Files

Modify:

```text
internal/config/config_test.go
```

## Requirements

Add tests confirming:

```text
Default().Logger.Level == "info"
Default().Logger.Format == "text"
Default().Logger.Output == "stdout"
```

Keep existing default tests passing.

Do not create the logger package in this task.

## Verification

Run:

```bash
go test ./...
```

Expected:

```text
All tests pass.
```

---

# TASK 03 — Add logger config validation

## Goal

Validate logger config values through the existing config validation system.

## Files

Modify:

```text
internal/config/config.go
```

## Requirements

Update:

```go
func Validate(cfg Config) error
```

Allowed logger levels:

```text
debug
info
warn
error
```

Allowed logger formats:

```text
text
json
```

Allowed logger outputs:

```text
stdout
stderr
```

Validation errors must be field-level.

Use these exact error messages:

```text
logger.level must be one of: debug, info, warn, error
logger.format must be one of: text, json
logger.output must be one of: stdout, stderr
```

Do not normalize logger values.

Do not silently lowercase values.

Invalid casing should fail.

Do not create the logger package in this task.

## Verification

Run:

```bash
go test ./...
```

Expected:

```text
All tests pass.
```

---

# TASK 04 — Add logger validation tests

## Goal

Test logger validation behavior.

## Files

Modify:

```text
internal/config/config_test.go
```

## Requirements

Add table-driven tests for invalid logger config values.

Test invalid level:

```text
trace
```

Expected error:

```text
logger.level must be one of: debug, info, warn, error
```

Test invalid format:

```text
console
```

Expected error:

```text
logger.format must be one of: text, json
```

Test invalid output:

```text
file
```

Expected error:

```text
logger.output must be one of: stdout, stderr
```

Also test that this valid logger config passes validation:

```text
level=debug
format=json
output=stderr
```

Do not create the logger package in this task.

## Verification

Run:

```bash
go test ./...
```

Expected:

```text
All tests pass.
```

---

# TASK 05 — Add TOML logger section support tests

## Goal

Verify TOML loading supports the new `[logger]` section.

## Files

Modify:

```text
internal/config/config_test.go
```

## Requirements

Add a test for loading TOML with:

```toml
[logger]
level = "debug"
format = "json"
output = "stderr"
```

Verify loaded config contains:

```text
Logger.Level == "debug"
Logger.Format == "json"
Logger.Output == "stderr"
```

Add a partial TOML test where `[logger]` is omitted.

Verify omitted logger section preserves defaults:

```text
Logger.Level == "info"
Logger.Format == "text"
Logger.Output == "stdout"
```

Add an unknown logger field test:

```toml
[logger]
color = true
```

Expected behavior:

```text
Load returns an error.
```

No production code changes are needed in this task.

Unknown field rejection from `config_enterprise.md` already handles this case through:

```go
decoder.DisallowUnknownFields()
```

If this test fails, inspect the existing TOML decode behavior before changing production code.

Do not create the logger package in this task.

## Verification

Run:

```bash
go test ./...
```

Expected:

```text
All tests pass.
```

---

# TASK 06 — Update root config.toml

## Goal

Add logger defaults to the root config file.

## Files

Modify:

```text
config.toml
```

## Required content

Add:

```toml
[logger]
level = "info"
format = "text"
output = "stdout"
```

Do not remove existing sections.

Expected full shape:

```toml
[server]
host = "127.0.0.1"
port = 8080

[data]
path = "./data"

[logger]
level = "info"
format = "text"
output = "stdout"
```

Confirm existing config loading tests still pass.

Confirm the existing example config loading/validation test still passes.

Missing `[logger]` sections must still be valid because defaults are applied before TOML decode.

Do not create the logger package in this task.

## Verification

Run:

```bash
go test ./...
```

Expected:

```text
All tests pass.
```

---

# TASK 07 — Update config.example.toml

## Goal

Add logger defaults to the example config file.

## Files

Modify:

```text
config.example.toml
```

## Required content

Add:

```toml
[logger]
level = "info"
format = "text"
output = "stdout"
```

Do not remove existing sections.

Confirm the existing example config loading/validation test still passes.

Missing `[logger]` sections must still be valid because defaults are applied before TOML decode.

Do not create the logger package in this task.

## Verification

Run:

```bash
go test ./...
```

Expected:

```text
All tests pass.
```

---

# TASK 08 — Update config documentation

## Goal

Document logger configuration.

## Files

Modify:

```text
docs/config.md
```

## Requirements

Add a logger configuration section.

Document this example:

```toml
[logger]
level = "info"
format = "text"
output = "stdout"
```

Document allowed values:

```text
level: debug, info, warn, error
format: text, json
output: stdout, stderr
```

Mention:

```text
File output is not supported yet.
Log rotation is not supported yet.
Environment variable overrides are documented for future use only.
```

Add future env vars:

```text
PLOMVIX_LOGGER_LEVEL
PLOMVIX_LOGGER_FORMAT
PLOMVIX_LOGGER_OUTPUT
```

Do not claim env vars are implemented.

Do not create the logger package in this task.

## Verification

Run:

```bash
go test ./...
```

Expected:

```text
All tests pass.
```

---

# TASK 09 — Create internal logger package

## Goal

Create the first logger package using Go standard library `log/slog`.

## Files

Create:

```text
internal/logger/logger.go
```

## Requirements

Add package:

```go
package logger
```

Implement public constructor:

```go
func New(cfg config.LoggerConfig) (*slog.Logger, error)
```

Implement internal helper:

```go
func newWithWriter(cfg config.LoggerConfig, w io.Writer) (*slog.Logger, error)
```

`New(cfg)` should:

* choose stdout or stderr from `cfg.Output`
* call `newWithWriter(cfg, writer)`
* return the configured logger
* not call `slog.SetDefault`
* not write any logs during construction

`newWithWriter(cfg, w)` should:

* map logger level
* map logger format
* build the slog handler
* return `*slog.Logger`
* not mutate config
* not write logs during construction

Use:

```go
log/slog
```

Expected level mapping:

```text
debug -> slog.LevelDebug
info  -> slog.LevelInfo
warn  -> slog.LevelWarn
error -> slog.LevelError
```

Expected output mapping in `New`:

```text
stdout -> os.Stdout
stderr -> os.Stderr
```

Expected format mapping:

```text
text -> slog.NewTextHandler
json -> slog.NewJSONHandler
```

Invalid values should return errors using these exact messages:

```text
logger.level must be one of: debug, info, warn, error
logger.format must be one of: text, json
logger.output must be one of: stdout, stderr
```

Important validation rule:

The config package remains the primary validation layer.

The logger package must only use simple `switch` statements with default error returns as a safety fallback.

Do not build a second full validation system.

Do not add exported validation helpers.

Do not create shared utility packages.

Do not introduce external dependencies.

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

# TASK 10 — Add logger public API tests

## Goal

Verify logger construction through the public API.

## Files

Create:

```text
internal/logger/logger_test.go
```

## Package

Use:

```go
package logger_test
```

## Requirements

Test that valid configs create a non-nil logger.

Test combinations:

```text
level=debug format=text output=stdout
level=info  format=json output=stdout
level=warn  format=text output=stderr
level=error format=json output=stderr
```

Test invalid level returns:

```text
logger.level must be one of: debug, info, warn, error
```

Test invalid format returns:

```text
logger.format must be one of: text, json
```

Test invalid output returns:

```text
logger.output must be one of: stdout, stderr
```

Do not assert exact internal slog handler type.

Do not write logs as part of this public API test.

Do not access unexported helpers from this file.

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

# TASK 11 — Add logger internal output behavior tests

## Goal

Verify text output, JSON output, and level filtering without writing to real stdout or stderr.

## Files

Create:

```text
internal/logger/logger_internal_test.go
```

## Package

Use:

```go
package logger
```

This file must use the same package as `logger.go` so it can access the unexported helper:

```go
newWithWriter
```

Do not place these tests in `package logger_test`.

Do not export `newWithWriter`.

## Requirements

Use a buffer, such as:

```go
bytes.Buffer
```

Test text format:

* Create logger with format `text`.
* Log message:

```text
hello
```

* Verify output contains:

```text
msg=hello
```

This assumes Go 1.21 or later `log/slog` text handler behavior.

Test JSON format:

* Create logger with format `json`.
* Log message:

```text
hello
```

* Treat slog JSON output as newline-delimited JSON.
* Parse the first line of output as JSON.
* Verify the first line is valid JSON.
* Verify JSON field:

```text
msg == "hello"
```

Test debug level:

* Create logger with level `debug`.
* Emit debug message.
* Verify debug message is present.

Test info level:

* Create logger with level `info`.
* Emit debug message.
* Verify debug message is not present.

Do not write to real stdout.

Do not write to real stderr.

Do not change public API to support this test.

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

# TASK 12 — Confirm intentional minimal validation fallback

## Goal

Confirm logger validation fallback duplication is intentional and minimal.

## Files

Review only:

```text
internal/config/config.go
internal/logger/logger.go
```

## Requirements

No refactor is required in this task.

Duplication is acceptable at this stage.

The config package is the primary validation layer.

The logger package has simple `switch` fallback checks only so direct calls to `logger.New` fail safely.

Do not create shared helper packages.

Do not move logger validation into the config package.

Do not make `internal/config` import `internal/logger`.

Do not export logger validation helpers.

Do not change production code unless there is a compile or test failure.

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

Task report should explicitly confirm:

```text
Validation fallback duplication reviewed.
Duplication is small and intentional.
No refactor performed.
Dependency direction remains valid.
```

---

# TASK 13 — Final logging setup review

## Goal

Perform final review of the logging setup.

## Review Checklist

Confirm:

* Go version is 1.21 or later.
* Project is expected to remain on Go 1.22.
* `setup.md` still exists in the project root, unless intentionally removed earlier.
* `internal/logger` exists.
* `internal/logger/logger.go` exists.
* `internal/logger/logger_test.go` exists.
* `internal/logger/logger_internal_test.go` exists.
* Logger uses `log/slog`.
* No external logging dependency was added.
* `slog.SetDefault` is not used.
* `newWithWriter` is unexported.
* Public API is only `New(cfg config.LoggerConfig)`.
* Public API tests use `package logger_test`.
* Internal writer tests use `package logger`.
* JSON output tests parse the first output line as JSON.
* Logger config exists under `[logger]`.
* Logger defaults are correct.
* Logger validation messages are field-level.
* Logger values are case-sensitive.
* Partial TOML preserves logger defaults.
* Unknown logger TOML fields are rejected.
* `config.toml` includes logger section.
* `config.example.toml` includes logger section.
* Existing example config validation test still passes.
* `docs/config.md` documents logger options.
* Logger env vars are documented only, not implemented.
* File output is not implemented.
* Log rotation is not implemented.
* No API server was added.
* No WAL was added.
* No storage engine was added.
* No query engine was added.
* No UI was added.
* No lifecycle manager was added.
* No unrelated folders were created.
* `go.sum` has no new logging-related dependency entries.
* `go.mod` has no new logging-related dependency entries.

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
│   └── config.md
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
└── README.md
```

Note: `setup.md` is listed because it was part of the initial project setup. This plan does not create it. Task 13 only confirms whether it still exists.

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

The logger foundation must be usable by future Plomvix components, but no future component should be implemented in this plan.

---
