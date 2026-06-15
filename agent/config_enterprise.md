# Plomvix Enterprise Config Hardening Plan

## File Name

`config_enterprise.md`

---

## Goal

Harden the Plomvix config system from a basic developer setup into a clean, production-grade foundation.

This plan starts only after `config_setup.md` is completed successfully.

This plan must improve config quality without adding unrelated database features.

---

## Required Starting State

Before starting this plan, the project must already have:

```text
internal/config/
├── config.go
└── config_test.go
```

The config package must already expose:

```go
func Default() Config
func Validate(cfg Config) error
func Load(path string) (Config, error)
```

The existing config types must be:

```go
type Config struct {
	Server ServerConfig `toml:"server"`
	Data   DataConfig   `toml:"data"`
}

type ServerConfig struct {
	Host string `toml:"host"`
	Port int    `toml:"port"`
}

type DataConfig struct {
	Path string `toml:"path"`
}
```

The allowed existing external dependency is:

```text
github.com/pelletier/go-toml/v2
```

If this starting state is not true, stop and report that `config_setup.md` is incomplete.

---

## Coding Agent

Use: **DeepSeek V4 Pro**

If the local coding tool uses a different exact DeepSeek model identifier, use the configured DeepSeek coding model available in the environment.

---

## Global Project Rules

1. Execute tasks in exact order.
2. Complete one task fully before starting the next task.
3. Verify after every task.
4. Do not skip verification.
5. Keep each task small.
6. Do not add future placeholders.
7. Do not add folders unless this plan explicitly requires them.
8. Do not touch `cmd/plomvix/main.go`.
9. Do not add WAL.
10. Do not add storage engine code.
11. Do not add query engine code.
12. Do not add API server code.
13. Do not add UI code.
14. Do not add CLI flag parsing.
15. Do not add config hot reload.
16. Do not add remote config.
17. Do not add secrets management.
18. Do not create a root-level `tests/` directory.
19. All config tests must remain in `internal/config/config_test.go`.
20. All updates to `internal/config/config_test.go` must keep `package config_test`.
21. Do not switch tests to `package config`.
22. Do not add any dependency beyond `github.com/pelletier/go-toml/v2`.
23. Use idiomatic Go.
24. Use clear field-level error messages.
25. Do not panic from config functions.
26. Do not use global mutable config state.
27. Search Graphify before starting each task if Graphify is available.
28. Update Graphify after completing each task if Graphify is available.
29. If Graphify is unavailable, do not block the task; mention it in the task report.

---

## Final Expected Config Behavior

By the end of this plan:

1. Unknown TOML fields must be rejected.
2. Missing TOML fields must preserve defaults.
3. Validation errors must identify the exact failing field.
4. Config path values must be normalized.
5. Config loading must preserve defaults.
6. Config must not use global mutable state.
7. Config precedence must be documented.
8. Environment override policy must be documented but not implemented.
9. Startup fail-fast policy must be documented but not wired into `main.go`.
10. Tests must be table-driven where useful.
11. Example config must be validated by tests.
12. No unrelated database features must be added.

---

## Final Expected Files

By the end of this plan, these files must exist:

```text
internal/config/config.go
internal/config/config_test.go
docs/config.md
config.example.toml
config.toml
go.mod
go.sum
```

---

# TASK 01 — Add Field-Level Validation Errors

## Objective

Improve `Validate(cfg Config) error` so errors clearly identify the failing config field.

---

## Required Changes

Update:

```text
internal/config/config.go
```

---

## Requirements

* Keep the function signature unchanged:

```go
func Validate(cfg Config) error
```

* Return clear field-level errors.
* Do not introduce custom error types yet.
* Use only Go standard library.
* Do not mutate the config.
* Do not touch `Load()`.
* Do not touch `cmd/plomvix/main.go`.

---

## Required Error Messages

Use these exact error messages:

```text
server.host is required
server.port must be between 1 and 65535
data.path is required
```

---

## Validation Rules

`Validate(cfg Config)` must return an error when:

1. `cfg.Server.Host` is empty.
2. `cfg.Server.Port` is less than `1`.
3. `cfg.Server.Port` is greater than `65535`.
4. `cfg.Data.Path` is empty.

`Validate(cfg Config)` must return `nil` when config is valid.

---

## Verification

Run:

```bash
go test ./...
go build ./...
```

Expected:

* `go test ./...` passes.
* `go build ./...` passes.
* No external dependency is added.
* `cmd/plomvix/main.go` is unchanged.

---

## Task Completion Report

Report:

1. File modified.
2. Field-level validation errors added.
3. Whether `go test ./...` passed.
4. Whether `go build ./...` passed.
5. Confirm no external dependency was added.
6. Confirm `cmd/plomvix/main.go` was not modified.
7. Confirm Graphify was searched and updated, or unavailable.

---

# TASK 02 — Convert Validation Tests to Table-Driven Tests

## Objective

Make validation tests scalable and easier to extend.

---

## Required Changes

Update:

```text
internal/config/config_test.go
```

---

## Requirements

* Keep package declaration:

```go
package config_test
```

* Convert validation tests into table-driven tests.
* Check exact error messages for invalid configs.
* Keep existing default config tests.
* Do not remove coverage.
* Do not add external test libraries.
* Do not touch `cmd/plomvix/main.go`.

---

## Required Test Coverage

Validation tests must cover:

1. Default config is valid.

2. Empty `server.host` returns:

```text
server.host is required
```

3. Zero `server.port` returns:

```text
server.port must be between 1 and 65535
```

4. Negative `server.port` returns:

```text
server.port must be between 1 and 65535
```

5. `server.port` above `65535` returns:

```text
server.port must be between 1 and 65535
```

6. Empty `data.path` returns:

```text
data.path is required
```

---

## Verification

Run:

```bash
go test ./...
go build ./...
```

Expected:

* `go test ./...` passes.
* `go build ./...` passes.
* Exact error message checks pass.
* No external dependency is added.
* `cmd/plomvix/main.go` is unchanged.

---

## Task Completion Report

Report:

1. File modified.
2. Validation tests converted to table-driven tests.
3. Exact error message checks added.
4. Whether `go test ./...` passed.
5. Whether `go build ./...` passed.
6. Confirm no external dependency was added.
7. Confirm `cmd/plomvix/main.go` was not modified.
8. Confirm Graphify was searched and updated, or unavailable.

---

# TASK 03 — Normalize Config Paths

## Objective

Normalize `Data.Path` after loading config so path handling is consistent.

---

## Required Changes

Update:

```text
internal/config/config.go
```

---

## Requirements

* Add a small unexported helper:

```go
func normalize(cfg Config) Config
```

* `normalize(cfg Config)` must return a normalized copy.
* Do not mutate the input config directly.
* Use Go standard library package:

```go
path/filepath
```

* Normalize only `cfg.Data.Path`.
* Use:

```go
filepath.Clean(cfg.Data.Path)
```

* Do not convert relative paths to absolute paths.
* Do not create directories.
* Do not check filesystem existence.
* Do not modify `Server.Host`.
* Do not modify `Server.Port`.
* Do not touch `cmd/plomvix/main.go`.

---

## Load Behavior Update

Update `Load(path string)` flow to:

```text
Default → Decode TOML → Normalize → Validate → Return
```

Required order:

1. `cfg := Default()`
2. Decode TOML into `cfg`
3. `cfg = normalize(cfg)`
4. `Validate(cfg)`
5. Return `cfg`

---

## Important

Do not normalize before decoding TOML.

Do not validate before normalization.

---

## Verification

Run:

```bash
go test ./...
go build ./...
```

Expected:

* `go test ./...` passes.
* `go build ./...` passes.
* No external dependency is added.
* `cmd/plomvix/main.go` is unchanged.

---

## Task Completion Report

Report:

1. File modified.
2. `normalize(cfg Config) Config` added.
3. `Load()` updated to normalize before validation.
4. Whether `go test ./...` passed.
5. Whether `go build ./...` passed.
6. Confirm no external dependency was added.
7. Confirm `cmd/plomvix/main.go` was not modified.
8. Confirm Graphify was searched and updated, or unavailable.

---

# TASK 04 — Add Path Normalization Tests

## Objective

Add tests that prove config paths are normalized correctly.

---

## Required Changes

Update:

```text
internal/config/config_test.go
```

---

## Requirements

* Keep package declaration:

```go
package config_test
```

* Add tests for `Load()` path normalization.
* Use `t.TempDir()`.
* Use `os.WriteFile`.
* Use only standard testing tools.
* Do not add external test libraries.
* Do not touch `cmd/plomvix/main.go`.

---

## Required Test Case

### Test — Load Normalizes Data Path

Create TOML with:

```toml
[server]
host = "127.0.0.1"
port = 8080

[data]
path = "./data/../data"
```

Expected:

```text
cfg.Data.Path == "data"
```

Important:

* `filepath.Clean("./data/../data")` returns `"data"`.
* The expected value must be `"data"`.

---

## Verification

Run:

```bash
go test ./...
go build ./...
```

Expected:

* `go test ./...` passes.
* `go build ./...` passes.
* Path normalization test passes.
* No external dependency is added.
* `cmd/plomvix/main.go` is unchanged.

---

## Task Completion Report

Report:

1. File modified.
2. Path normalization test added.
3. Whether `go test ./...` passed.
4. Whether `go build ./...` passed.
5. Confirm no external dependency was added.
6. Confirm `cmd/plomvix/main.go` was not modified.
7. Confirm Graphify was searched and updated, or unavailable.

---

# TASK 05 — Reject Unknown TOML Fields

## Objective

Make TOML config loading strict so typo fields are not silently accepted.

---

## Required Changes

Update:

```text
internal/config/config.go
```

---

## Requirements

* Keep `Load(path string) (Config, error)` signature unchanged.
* Use strict decoding with `github.com/pelletier/go-toml/v2`.
* Unknown TOML fields must return an error.
* Missing TOML fields must still preserve defaults.
* Partial TOML must still work.
* Do not add new dependencies.
* Do not touch `cmd/plomvix/main.go`.

---

## Important Clarification

Missing fields and unknown fields are different.

A field present in the struct but absent from TOML is allowed.

Example allowed partial TOML:

```toml
[server]
port = 9090
```

This is allowed because `server.host` and `data.path` are known fields with default values.

A field present in TOML but absent from the struct is not allowed.

Example invalid TOML:

```toml
[server]
prt = 8080
```

This is invalid because `prt` is not a known field.

---

## Required Implementation Detail

Use this decoder behavior:

```go
decoder := toml.NewDecoder(file)
decoder.DisallowUnknownFields()

if err := decoder.Decode(&cfg); err != nil {
	return Config{}, fmt.Errorf("decode config: %w", err)
}
```

Important:

* `cfg` must already be initialized with `Default()`.
* Do not decode into zero-value `Config`.
* Do not lose default values for partial TOML.

---

## Verification

Run:

```bash
go test ./...
go build ./...
```

Expected:

* `go test ./...` passes.
* `go build ./...` passes.
* Existing partial TOML test still passes.
* No external dependency is added.
* `cmd/plomvix/main.go` is unchanged.

---

## Task Completion Report

Report:

1. File modified.
2. Strict TOML decoding added.
3. Whether `go test ./...` passed.
4. Whether `go build ./...` passed.
5. Confirm partial TOML loading still works.
6. Confirm no external dependency was added.
7. Confirm `cmd/plomvix/main.go` was not modified.
8. Confirm Graphify was searched and updated, or unavailable.

---

# TASK 06 — Add Unknown TOML Field Tests

## Objective

Add tests that prove unknown TOML fields are rejected.

---

## Required Changes

Update:

```text
internal/config/config_test.go
```

---

## Requirements

* Keep package declaration:

```go
package config_test
```

* Use `t.TempDir()`.
* Use `os.WriteFile`.
* Use only standard testing tools.
* Do not add external test libraries.
* Do not touch `cmd/plomvix/main.go`.

---

## Required Test Cases

### Test 1 — Unknown Top-Level Field Is Error

Create TOML:

```toml
unknown = true

[server]
host = "127.0.0.1"
port = 8080

[data]
path = "./data"
```

Expected:

```go
err != nil
```

---

### Test 2 — Unknown Server Field Is Error

Create TOML:

```toml
[server]
host = "127.0.0.1"
prt = 8080

[data]
path = "./data"
```

Expected:

```go
err != nil
```

---

### Test 3 — Unknown Data Field Is Error

Create TOML:

```toml
[server]
host = "127.0.0.1"
port = 8080

[data]
path = "./data"
directory = "./other"
```

Expected:

```go
err != nil
```

---

## Verification

Run:

```bash
go test ./...
go build ./...
```

Expected:

* `go test ./...` passes.
* `go build ./...` passes.
* Unknown TOML field tests pass.
* Partial TOML test still passes.
* No external dependency is added.
* `cmd/plomvix/main.go` is unchanged.

---

## Task Completion Report

Report:

1. File modified.
2. Unknown TOML field tests added.
3. Whether `go test ./...` passed.
4. Whether `go build ./...` passed.
5. Confirm partial TOML test still passes.
6. Confirm no external dependency was added.
7. Confirm `cmd/plomvix/main.go` was not modified.
8. Confirm Graphify was searched and updated, or unavailable.

---

# TASK 07 — Create Config Documentation

## Objective

Create enterprise-grade config documentation in one focused docs task.

---

## Required Changes

Create:

```text
docs/config.md
```

If `docs/` does not exist, create only this folder and this file.

---

## Requirements

* Documentation only.
* Do not add code.
* Do not add tests.
* Do not touch `cmd/plomvix/main.go`.

---

## Required Documentation Sections

`docs/config.md` must include these sections:

```text
# Plomvix Configuration

## Config Precedence

## Environment Override Policy

## Fail-Fast Startup Policy

## Config Immutability Policy
```

---

## Required Documentation Content

### Config Precedence

Document this precedence order:

```text
1. Built-in defaults
2. config.toml
3. Environment variables later
4. CLI flags later
```

Explain:

* Built-in defaults are always available.
* `config.toml` overrides defaults.
* Environment variables are not implemented yet.
* CLI flags are not implemented yet.
* Future override layers must not break this order.

---

### Environment Override Policy

Document:

* Environment overrides are planned but not implemented yet.
* Future env var names must use prefix:

```text
PLOMVIX_
```

* Future env vars should map clearly to config fields.
* Example future env vars:

```text
PLOMVIX_SERVER_HOST
PLOMVIX_SERVER_PORT
PLOMVIX_DATA_PATH
```

* Environment overrides must apply after TOML and before CLI flags.
* This task must not implement environment overrides.

---

### Fail-Fast Startup Policy

Document:

* Plomvix must not start with invalid config.
* Later, startup must load config once.
* Later, startup must validate config before starting services.
* Later, startup must exit with a clear error if config is missing, malformed, or invalid.
* This task only documents the policy.

---

### Config Immutability Policy

Document:

* Config is loaded once at startup.
* Config is validated before use.
* Config should be passed explicitly to components that need it.
* Packages must not read config from hidden global state.
* Packages must not mutate config after startup.
* If runtime config changes are needed later, they must be designed as a separate feature.

---

## Verification

Run:

```bash
go test ./...
go build ./...
```

Expected:

* `docs/config.md` exists.
* Config precedence is documented.
* Environment override policy is documented.
* Fail-fast startup policy is documented.
* Config immutability policy is documented.
* `go test ./...` passes.
* `go build ./...` passes.
* No code behavior changed.
* `cmd/plomvix/main.go` is unchanged.

---

## Task Completion Report

Report:

1. File created.
2. Config documentation sections added.
3. Whether `go test ./...` passed.
4. Whether `go build ./...` passed.
5. Confirm no code behavior changed.
6. Confirm `cmd/plomvix/main.go` was not modified.
7. Confirm Graphify was searched and updated, or unavailable.

---

# TASK 08 — Add Example Config File

## Objective

Add a committed example config file for users and tests.

---

## Required Changes

Create:

```text
config.example.toml
```

---

## Required Content

```toml
[server]
host = "127.0.0.1"
port = 8080

[data]
path = "./data"
```

---

## Requirements

* Do not modify `config.toml`.
* Do not add extra sections.
* Do not add WAL path.
* Do not add engine settings.
* Do not add UI settings.
* Do not touch `cmd/plomvix/main.go`.

---

## Verification

Run:

```bash
go test ./...
go build ./...
```

Expected:

* `config.example.toml` exists.
* `config.example.toml` contains only `[server]` and `[data]`.
* `go test ./...` passes.
* `go build ./...` passes.
* `cmd/plomvix/main.go` is unchanged.

---

## Task Completion Report

Report:

1. File created.
2. Confirm `config.example.toml` exists.
3. Confirm only `[server]` and `[data]` sections exist.
4. Whether `go test ./...` passed.
5. Whether `go build ./...` passed.
6. Confirm `cmd/plomvix/main.go` was not modified.
7. Confirm Graphify was searched and updated, or unavailable.

---

# TASK 09 — Add Example Config Validation Test

## Objective

Make sure the committed example config always remains valid.

---

## Required Changes

Update:

```text
internal/config/config_test.go
```

---

## Requirements

* Keep package declaration:

```go
package config_test
```

* Add a test that loads `config.example.toml`.
* Do not add external test libraries.
* Do not touch `cmd/plomvix/main.go`.

---

## Path Requirement

Because this test file is inside `internal/config`, the test must load the example file using this relative path:

```text
../../config.example.toml
```

This path is acceptable because Go runs each package test with the package directory as the working directory.

For `internal/config/config_test.go`, the package working directory is:

```text
internal/config/
```

So this relative path reaches the project root:

```text
../../config.example.toml
```

This assumes the config package remains at:

```text
internal/config/
```

If the config package is moved in the future, this test path must be updated.

---

## Required Test Case

### Test — Example Config Is Valid

Call:

```go
cfg, err := config.Load("../../config.example.toml")
```

Expected:

* `err == nil`
* `cfg.Server.Host == "127.0.0.1"`
* `cfg.Server.Port == 8080`
* `cfg.Data.Path == "data"`

Important:

After path normalization, `filepath.Clean("./data")` returns:

```text
data
```

The expected value must be exactly:

```text
data
```

Do not accept `"./data"` in this test.

---

## Verification

Run:

```bash
go test ./...
go build ./...
```

Expected:

* `go test ./...` passes.
* `go build ./...` passes.
* Example config test passes.
* No external dependency is added.
* `cmd/plomvix/main.go` is unchanged.

---

## Task Completion Report

Report:

1. File modified.
2. Example config validation test added.
3. Whether `go test ./...` passed.
4. Whether `go build ./...` passed.
5. Confirm no external dependency was added.
6. Confirm `cmd/plomvix/main.go` was not modified.
7. Confirm Graphify was searched and updated, or unavailable.

---

# TASK 10 — Final Config Hardening Review

## Objective

Run final verification for the complete enterprise config hardening plan.

---

## Required Checks

Run:

```bash
go mod tidy
go test ./...
go build ./...
```

Then inspect:

```text
internal/config/config.go
internal/config/config_test.go
docs/config.md
config.example.toml
config.toml
go.mod
go.sum
```

---

## Final Expected Result

* `go mod tidy` passes.
* `go test ./...` passes.
* `go build ./...` passes.
* `docs/config.md` exists.
* `config.example.toml` exists.
* `cmd/plomvix/main.go` is unchanged.
* No WAL code exists.
* No storage engine code exists.
* No query engine code exists.
* No API server code exists.
* No UI code exists.
* No root-level `tests/` directory exists.
* No env override code exists.
* No CLI parsing exists.
* No global config variable exists.
* Unknown TOML fields are rejected.
* Missing TOML fields preserve defaults.
* Partial TOML preserves defaults.
* `Data.Path` is normalized by `Load()`.
* Validation errors use field-level messages.
* Config precedence is documented.
* Env override policy is documented but not implemented.
* Fail-fast startup policy is documented but not wired into `main.go`.
* Config immutability policy is documented.
* `config.example.toml` exists and is tested.
* Only allowed external dependency remains:

```text
github.com/pelletier/go-toml/v2
```

---

## Final Completion Report

Report:

1. All files created or modified.
2. Final config API.
3. Final validation behavior.
4. Final load behavior.
5. Final test coverage.
6. Final documentation added.
7. Final dependency list.
8. Whether `go mod tidy` passed.
9. Whether `go test ./...` passed.
10. Whether `go build ./...` passed.
11. Confirm `docs/config.md` exists.
12. Confirm `config.example.toml` exists.
13. Confirm `cmd/plomvix/main.go` was not modified.
14. Confirm no unrelated database features were added.
15. Confirm no root-level `tests/` directory exists.
16. Confirm Graphify was searched and updated after each task, or unavailable.
