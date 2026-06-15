# Plomvix Lifecycle

The lifecycle package provides a minimal **component interface** and a
**manager** for controlling component startup and shutdown.

## Component Interface

All lifecycle-managed components must implement the `Component` interface,
which provides `Name()`, `Start(ctx)`, and `Stop(ctx)` methods. Every method
is context-aware so callers control cancellation and timeouts.

## Manager

The `Manager` controls the lifecycle of registered components. It enforces
**registration order**, **start order** (same as registration), and
**reverse stop order**. All exported methods are protected by a
**mutex-protected manager state**.

### Lifecycle States

The manager tracks explicit **lifecycle states**:

| State | Meaning |
|-------|---------|
| `new` | Manager created, registration allowed |
| `starting` | Start has begun, registration rejected |
| `started` | All components started successfully |
| `failed` | Start failed or a component panicked |
| `stopping` | Stop is in progress |
| `stopped` | Stop completed |

### State Transitions

Explicit **state transitions** ensure predictable behavior:

```
new → starting → started
new → starting → failed
started → stopping → stopped
failed → stopping → stopped
new → stopped
```

### Registration

Components are registered via `Register`. **registration after start is rejected**
with `ErrAlreadyStarted`. **duplicate component** names are rejected with
`ErrDuplicateComponent`. Registration does not call `Start` or `Stop` on
the component.

### Start Behavior

`Start(ctx)` starts each registered component in registration order. If a
component fails to start, the **start error** is returned immediately with the
component name, and no further components are started. Only successfully started
components are eligible for later stop.

### Stop Behavior

`Stop(ctx)` stops successfully started components in reverse order. It
attempts to stop every eligible component even if one fails. Any **stop error**
includes the failing component name. Multiple errors are combined with
`errors.Join`. **stop idempotency** is guaranteed: repeated calls after a
successful or failed stop return `nil` and do not call component `Stop` again.
**stop remains idempotent** after stop errors and after stop panics.

### Concurrent Stop Safety

Calling Stop while Start is in progress returns ErrInvalidState. Callers should wait for Start to complete before calling Stop.

### Panic Recovery

The manager recovers from component panics. A **start panic** is recovered,
the state transitions to `failed`, and only components started before the panic
are eligible for stop. A **stop panic** is recovered, remaining components
continue to be stopped, and the state transitions to `stopped`. **panic recovery**
ensures the manager never crashes due to a misbehaving component.

## Non-Goals

The lifecycle package intentionally does not implement:

- signal handling
- WAL
- storage
- query engine
- API server
- UI
- logger integration
- config integration
