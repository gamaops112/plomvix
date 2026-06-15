# Plomvix Config Setup Plan

## File Name

`config_setup.md`

---

## Goal

Create and perfect the initial configuration system for Plomvix.

This plan covers only config setup.

Do not add WAL, storage engines, query engines, API server, UI, metadata system, lifecycle manager, or database behavior in this plan.

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
5. Keep changes minimal.
6. Do not add future placeholders.
7. Do not add folders unless this plan explicitly requires them.
8. Do not touch `cmd/plomvix/main.go`.
9. Do not add WAL.
10. Do not add storage engine code.
11. Do not add query engine code.
12. Do not add API server code.
13. Do not add UI code.
14. Do not create a root-level `tests/` directory.
15. Config tests must live beside the config package.
16. All updates to `internal/config/config_test.go` must keep `package config_test`.
17. Do not switch the test file to `package config`.
18. Use idiomatic Go.
19. Use clear error messages.
20. Do not panic from config functions.
21. Do not mutate config values inside validation.
22. Search Graphify before starting each task if Graphify is available.
23. Update Graphify after completing each task if Graphify is available.
24. If Graphify is unavailable, do not block the task; mention it in the task report.

---

## Canonical Config Package Location

All config code must live here:

```text
internal/config/
```

Required files by the end of this plan:

```text
internal/config/
├── config.go
└── config_test.go
```

---

## Canonical Test Location

All config tests must live here:

```text
internal/config/config_test.go
```

Do not create:

```text
tests/config_test.go
```

Do not create:

```text
tests/
```

---

## Final Expected Config API

By the end of this plan, the config package must expose:

```go
func Default() Config
func Validate(cfg Config) error
func Load(path string) (Config, error)
```

---

## Final Expected Config Types

`internal/config/config.go` must contain these basic config types:

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

Important:

* `Server.Port` must be `int`.
* Do not use `uint`.
* Do not add unused config fields.
* Do not add WAL path.
* Do not add engine config.
* Do not add query config.
* Do not add UI config.

---

## Final Expected Default Values

`Default()` must return:

```text
Server.Host = "127.0.0.1"
Server.Port = 8080
Data.Path   = "./data"
```

---

## Root Config File

The root `config.toml` must contain only:

```toml
[server]
host = "127.0.0.1"
port = 8080

[data]
path = "./data"
```

No extra config sections are allowed in this plan.

---

# TASK 01 — Create Minimal Config Types and Defaults

## Objective

Create the minimal config package structure and define default config values.

---

## Required Changes

Create or update:

```text
internal/config/config.go
```

---

## Requirements

* Define `Config`.
* Define `ServerConfig`.
* Define `DataConfig`.
* Add `Default() Config`.
* Use only the Go standard library.
* Do not add TOML loading yet.
* Do not add validation yet.
* Do not add external dependencies.
* Do not touch `cmd/plomvix/main.go`.

---

## Required Implementation Behavior

`Default()` must return:

```text
Server.Host = "127.0.0.1"
Server.Port = 8080
Data.Path   = "./data"
```

`Server.Port` must be typed as:

```go
int
```

Do not use:

```go
uint
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
* No external dependencies are added.
* `cmd/plomvix/main.go` is unchanged.
* No root-level `tests/` directory exists.

---

## Task Completion Report

Report:

1. File created or modified.
2. Config structs added.
3. `Default()` added.
4. Whether `go test ./...` passed.
5. Whether `go build ./...` passed.
6. Confirm no external dependencies were added.
7. Confirm `cmd/plomvix/main.go` was not modified.
8. Confirm no root-level `tests/` directory exists.
9. Confirm Graphify was searched and updated, or unavailable.

---

# TASK 02 — Add Default Config Tests

## Objective

Add tests for `config.Default()`.

---

## Required Changes

Create:

```text
internal/config/config_test.go
```

---

## Requirements

* Use external test package style:

```go
package config_test
```

* Import config using the module path:

```go
import "github.com/plomvix/plomvix/internal/config"
```

* Test only `Default()` in this task.
* Do not add validation tests yet.
* Do not add TOML loading tests yet.
* Do not add external dependencies.
* Do not touch `cmd/plomvix/main.go`.
* Do not create root-level `tests/`.

---

## Required Test Cases

### Test 1 — Default Config Exists

Verify:

* `cfg.Server.Host` is not empty.
* `cfg.Server.Port` is greater than `0`.
* `cfg.Data.Path` is not empty.

---

### Test 2 — Default Server Values

Verify:

* `cfg.Server.Host` equals `"127.0.0.1"`.
* `cfg.Server.Port` equals `8080`.

---

### Test 3 — Default Data Path

Verify:

* `cfg.Data.Path` equals `"./data"`.

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
* No external dependencies are added.
* `cmd/plomvix/main.go` is unchanged.
* No root-level `tests/` directory exists.

---

## Task Completion Report

Report:

1. File created.
2. Default config test cases added.
3. Whether `go test ./...` passed.
4. Whether `go build ./...` passed.
5. Confirm no external dependencies were added.
6. Confirm `cmd/plomvix/main.go` was not modified.
7. Confirm no root-level `tests/` directory was created.
8. Confirm Graphify was searched and updated, or unavailable.

---

# TASK 03 — Add Config Validation

## Objective

Add validation for config values.

---

## Required Changes

Update:

```text
internal/config/config.go
```

Add:

```go
func Validate(cfg Config) error
```

---

## Validation Rules

`Validate(cfg Config)` must return an error when:

1. `cfg.Server.Host` is empty.
2. `cfg.Server.Port` is less than `1`.
3. `cfg.Server.Port` is greater than `65535`.
4. `cfg.Data.Path` is empty.

`Validate(cfg Config)` must return `nil` when the config is valid.

---

## Error Requirements

* Use only the Go standard library.
* Error messages must be clear.
* Do not introduce custom error types yet.
* Do not panic.
* Do not mutate the config.

Acceptable error messages:

```text
server host is required
server port must be between 1 and 65535
data path is required
```

---

## Requirements

* Do not add TOML loading yet.
* Do not add external dependencies.
* Do not touch `cmd/plomvix/main.go`.
* Keep `Server.Port` as `int`.

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
* No external dependencies are added.
* `cmd/plomvix/main.go` is unchanged.
* No TOML loading is added.
* No root-level `tests/` directory exists.

---

## Task Completion Report

Report:

1. File modified.
2. `Validate(cfg Config) error` added.
3. Validation rules implemented.
4. Whether `go test ./...` passed.
5. Whether `go build ./...` passed.
6. Confirm no external dependencies were added.
7. Confirm no TOML loading was added.
8. Confirm `cmd/plomvix/main.go` was not modified.
9. Confirm no root-level `tests/` directory exists.
10. Confirm Graphify was searched and updated, or unavailable.

---

# TASK 04 — Add Config Validation Tests

## Objective

Add tests for `config.Validate()`.

---

## Required Changes

Update:

```text
internal/config/config_test.go
```

---

## Requirements

* Do not change the existing package declaration.
* The package declaration must remain:

```go
package config_test
```

* Use only Go standard testing tools.
* Do not add external dependencies.
* Do not add TOML loading tests yet.
* Do not touch `cmd/plomvix/main.go`.
* Do not create root-level `tests/`.

---

## Required Test Cases

### Test 1 — Default Config Is Valid

Verify:

```go
config.Validate(config.Default()) == nil
```

---

### Test 2 — Empty Server Host Is Invalid

Set:

```go
cfg := config.Default()
cfg.Server.Host = ""
```

Expected:

```go
config.Validate(cfg) != nil
```

---

### Test 3 — Zero Server Port Is Invalid

Set:

```go
cfg := config.Default()
cfg.Server.Port = 0
```

Expected:

```go
config.Validate(cfg) != nil
```

---

### Test 4 — Negative Server Port Is Invalid

Set:

```go
cfg := config.Default()
cfg.Server.Port = -1
```

Expected:

```go
config.Validate(cfg) != nil
```

---

### Test 5 — Server Port Above 65535 Is Invalid

Set:

```go
cfg := config.Default()
cfg.Server.Port = 65536
```

Expected:

```go
config.Validate(cfg) != nil
```

---

### Test 6 — Empty Data Path Is Invalid

Set:

```go
cfg := config.Default()
cfg.Data.Path = ""
```

Expected:

```go
config.Validate(cfg) != nil
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
* No external dependencies are added.
* `cmd/plomvix/main.go` is unchanged.
* No root-level `tests/` directory exists.

---

## Task Completion Report

Report:

1. File modified.
2. Validation test cases added.
3. Whether `go test ./...` passed.
4. Whether `go build ./...` passed.
5. Confirm no external dependencies were added.
6. Confirm no TOML loading tests were added.
7. Confirm `cmd/plomvix/main.go` was not modified.
8. Confirm no root-level `tests/` directory exists.
9. Confirm Graphify was searched and updated, or unavailable.

---

# TASK 05 — Add TOML Config Loading

## Objective

Add TOML loading for Plomvix config.

This is the first task in this plan where an external dependency is allowed.

---

## Dependency Decision

Use:

```text
github.com/pelletier/go-toml/v2
```

Reason:

* TOML v2 support.
* Clean Go API.
* Works with struct tags.
* Good fit for this config package.

Do not add any other external dependency.

---

## Required Changes

Update:

```text
go.mod
go.sum
internal/config/config.go
```

Add:

```go
func Load(path string) (Config, error)
```

---

## Required Behavior

`Load(path string)` must:

1. Return an error when `path` is empty.
2. Start with `cfg := Default()`.
3. Open the TOML file from `path`.
4. Decode TOML values into the already initialized `cfg`.
5. Preserve default values for TOML fields not present in the file.
6. Run `Validate(cfg)`.
7. Return the final config when valid.
8. Return an error when:

   * file path is empty,
   * file does not exist,
   * file cannot be read,
   * TOML is malformed,
   * decoded config is invalid.

---

## Required Implementation Shape

Use this implementation pattern:

```go
func Load(path string) (Config, error) {
	if path == "" {
		return Config{}, errors.New("config path is required")
	}

	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	defer file.Close()

	cfg := Default()

	if err := toml.NewDecoder(file).Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}

	if err := Validate(cfg); err != nil {
		return Config{}, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}
```

Required imports may include:

```go
import (
	"errors"
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)
```

---

## Default Preservation Requirement

This is mandatory:

```text
Do not decode into a zero-value Config and then return it.
Do not lose default values when TOML contains only partial sections.
The partial TOML loading test must pass.
```

Example:

```toml
[server]
port = 9090
```

Expected result:

```text
Server.Host remains "127.0.0.1"
Server.Port becomes 9090
Data.Path remains "./data"
```

---

## Error Requirements

* Return errors.
* Do not panic.
* Wrap underlying errors using `%w` where useful.
* Keep messages clear.
* Do not introduce custom error types yet.

Acceptable error messages:

```text
config path is required
read config: ...
decode config: ...
validate config: ...
```

---

## Requirements

* Do not touch `cmd/plomvix/main.go`.
* Do not add API server code.
* Do not add database behavior.
* Do not add any dependency except `github.com/pelletier/go-toml/v2`.
* Do not add config fields beyond `server` and `data`.

---

## Verification

Run:

```bash
go mod tidy
go test ./...
go build ./...
```

Expected:

* `go mod tidy` passes.
* `go test ./...` passes.
* `go build ./...` passes.
* Only `github.com/pelletier/go-toml/v2` is added as a new external dependency.
* `cmd/plomvix/main.go` is unchanged.
* No root-level `tests/` directory exists.

---

## Task Completion Report

Report:

1. Files modified.
2. `Load(path string) (Config, error)` added.
3. TOML dependency added.
4. Whether `go mod tidy` passed.
5. Whether `go test ./...` passed.
6. Whether `go build ./...` passed.
7. Confirm only `github.com/pelletier/go-toml/v2` was added.
8. Confirm `cmd/plomvix/main.go` was not modified.
9. Confirm no root-level `tests/` directory exists.
10. Confirm Graphify was searched and updated, or unavailable.

---

# TASK 06 — Add TOML Config Loading Tests

## Objective

Add tests for `config.Load(path string)`.

---

## Required Changes

Update:

```text
internal/config/config_test.go
```

---

## Requirements

* Do not change the existing package declaration.
* The package declaration must remain:

```go
package config_test
```

* Use `t.TempDir()` for temporary config files.
* Use `os.WriteFile` to create test TOML files.
* Use only Go standard testing tools plus the existing config package.
* Do not use external test libraries.
* Do not touch `cmd/plomvix/main.go`.
* Do not create root-level `tests/`.

---

## Required Test Cases

### Test 1 — Load Valid Config

Create TOML:

```toml
[server]
host = "0.0.0.0"
port = 9090

[data]
path = "/tmp/plomvix"
```

Expected:

* host equals `"0.0.0.0"`.
* port equals `9090`.
* data path equals `"/tmp/plomvix"`.

---

### Test 2 — Load Partial Config Preserves Defaults

Create TOML:

```toml
[server]
port = 9090
```

Expected:

* host remains `"127.0.0.1"`.
* port equals `9090`.
* data path remains `"./data"`.

This test is mandatory because it proves default preservation works correctly.

---

### Test 3 — Empty Path Is Error

Call:

```go
config.Load("")
```

Expected:

```go
err != nil
```

---

### Test 4 — Missing File Is Error

Call `config.Load()` with a path that does not exist.

Expected:

```go
err != nil
```

---

### Test 5 — Malformed TOML Is Error

Create malformed TOML.

Expected:

```go
err != nil
```

---

### Test 6 — Invalid Config Is Error

Create TOML:

```toml
[server]
port = 70000
```

Expected:

```go
err != nil
```

---

## Verification

Run:

```bash
go mod tidy
go test ./...
go build ./...
```

Expected:

* `go mod tidy` passes.
* `go test ./...` passes.
* `go build ./...` passes.
* No new dependency beyond `github.com/pelletier/go-toml/v2`.
* `cmd/plomvix/main.go` is unchanged.
* No root-level `tests/` directory exists.

---

## Task Completion Report

Report:

1. File modified.
2. TOML loading test cases added.
3. Whether `go mod tidy` passed.
4. Whether `go test ./...` passed.
5. Whether `go build ./...` passed.
6. Confirm no dependency beyond `github.com/pelletier/go-toml/v2` was added.
7. Confirm `cmd/plomvix/main.go` was not modified.
8. Confirm no root-level `tests/` directory exists.
9. Confirm Graphify was searched and updated, or unavailable.

---

# TASK 07 — Create Root Config File

## Objective

Create the root `config.toml` file for local Plomvix startup configuration.

---

## Required Changes

Create or update:

```text
config.toml
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

* `go test ./...` passes.
* `go build ./...` passes.
* `config.toml` exists at project root.
* `config.toml` contains only `[server]` and `[data]`.
* `cmd/plomvix/main.go` is unchanged.
* No root-level `tests/` directory exists.

---

## Task Completion Report

Report:

1. File created or modified.
2. Confirm root `config.toml` exists.
3. Confirm only `[server]` and `[data]` sections exist.
4. Whether `go test ./...` passed.
5. Whether `go build ./...` passed.
6. Confirm `cmd/plomvix/main.go` was not modified.
7. Confirm no root-level `tests/` directory exists.
8. Confirm Graphify was searched and updated, or unavailable.

---

# FINAL PLAN VERIFICATION

After all tasks are complete, run:

```bash
go mod tidy
go test ./...
go build ./...
```

Final expected result:

* All commands pass.
* `cmd/plomvix/main.go` remains unchanged.
* No WAL code exists.
* No storage engine code exists.
* No query engine code exists.
* No API server code exists.
* No UI code exists.
* No root-level `tests/` directory exists.
* Config package exposes:

  * `Default() Config`
  * `Validate(cfg Config) error`
  * `Load(path string) (Config, error)`
* Only allowed external dependency is:

  * `github.com/pelletier/go-toml/v2`
* Root `config.toml` contains only:

  * `[server]`
  * `[data]`

---

# FINAL COMPLETION REPORT

After the full plan is complete, report:

1. All files created or modified.
2. Final config API.
3. Final test coverage.
4. Final dependency list.
5. Whether `go mod tidy` passed.
6. Whether `go test ./...` passed.
7. Whether `go build ./...` passed.
8. Confirm `cmd/plomvix/main.go` was not modified.
9. Confirm no unrelated database features were added.
10. Confirm no root-level `tests/` directory exists.
11. Confirm Graphify was searched and updated after each task, or unavailable.
