# Plomvix Configuration

## Config Precedence

Configuration values are resolved in the following order (first match wins):

1. **Built-in defaults** — always available via `config.Default()`.
2. **config.toml** — overrides defaults for any key present in the file.
3. **Environment variables** — planned, not implemented yet.
4. **CLI flags** — planned, not implemented yet.

Built-in defaults ensure Plomvix always has a valid starting configuration. Each
subsequent layer overrides values from earlier layers. Missing keys at any layer
fall back to the previous layer.

Future override layers (environment variables and CLI flags) must preserve this
precedence order and must not break existing config loading behavior.

---

## Environment Override Policy

Environment variable overrides are **planned but not implemented yet**.

When implemented, the following rules will apply:

- All environment variable names must use the prefix `PLOMVIX_`.
- Each variable must map clearly to a config field.
- The mapping follows TOML section + key, using underscores.

| Config Field       | Future Env Var          |
|--------------------|-------------------------|
| `server.host`      | `PLOMVIX_SERVER_HOST`   |
| `server.port`      | `PLOMVIX_SERVER_PORT`   |
| `data.path`        | `PLOMVIX_DATA_PATH`     |

Environment overrides must apply **after** `config.toml` and **before** CLI
flags in the precedence chain. This task documents the policy only and does not
implement environment variable parsing.

---

## Fail-Fast Startup Policy

Plomvix must **never start with invalid configuration**.

At startup (in a future implementation):

1. Config is loaded once from the configured source.
2. Config is validated before any services start.
3. If the config file is missing, malformed, or contains invalid values, Plomvix
   must exit immediately with a clear, actionable error message.
4. No partial startup is allowed with invalid config.

This policy is documented here but not yet wired into `cmd/plomvix/main.go`.

---

## Config Immutability Policy

Configuration is treated as immutable after startup:

1. Config is loaded **once** at startup.
2. Config is **validated** before use.
3. Config is passed **explicitly** to components that need it — packages must
   not read config from hidden global state.
4. Packages must **not mutate** config after startup.
5. If runtime config changes are needed in the future, they must be designed as
   a separate, explicit feature with clear lifecycle management.
