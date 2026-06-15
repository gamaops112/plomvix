# Plomvix Runtime

The runtime package provides **runtime setup** and **core composition** for
Plomvix. It wires together config, logger, and lifecycle into a minimal
runnable entrypoint.

## Enterprise Runtime Hardening

The runtime has been hardened with enterprise runtime hardening through an
explicit **runtime object**, classified **runtime options**,
**startup timeout** and **shutdown timeout** policies,
**classified runtime errors**, and **panic recovery** for runtime operations.

## Core Composition

Runtime composes three foundations:

1. **config loading** — loads `config.toml` through the config package.
2. **logger creation** — creates a configured logger from the loaded config.
3. **lifecycle manager** — creates, starts, and stops the lifecycle manager.

## Runtime Object

The `Runtime` struct owns composed instances: config, logger, and lifecycle
manager. `New(opts)` creates a runtime without starting it. `Start(ctx)` and
`Stop(ctx)` manage the lifecycle with timeout contexts.

## Runtime Options

Options support:

- `ConfigPath` — path to `config.toml` (default: `config.toml`)
- `StartupTimeout` — max time for lifecycle start (default: 30s)
- `ShutdownTimeout` — max time for lifecycle stop (default: 30s)

## Classified Runtime Errors

All runtime errors are classified for use with `errors.Is`:

- `ErrInvalidOptions` — invalid runtime options
- `ErrLoadConfig` — config loading failure
- `ErrCreateLogger` — logger creation failure
- `ErrStartLifecycle` — lifecycle start failure
- `ErrStopLifecycle` — lifecycle stop failure
- `ErrRuntimePanic` — recovered panic in runtime operation

## Panic Recovery

Runtime operations (`New`, `Start`, `Stop`, `Run`) recover panics and return
`ErrRuntimePanic` instead of crashing.

## Zero-Component Lifecycle

No production components are registered in this minimal setup. The
**zero-component lifecycle** proves that config, logger, and lifecycle
foundations compose correctly without inventing fake services.

## Config Path Policy

Runtime defaults to `config.toml`. There are **no CLI flags**, **no environment overrides**, and no multi-path search.

## Non-Goals

The runtime setup intentionally does not implement:

- signal handling
- WAL
- storage
- query engine
- API server
- UI
- CLI flags
- environment overrides
