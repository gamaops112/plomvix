# Plomvix Logging Policy

## Core Rules

- Use **structured logging** fields instead of formatted strings.
- Use **component-scoped loggers** via `logger.WithComponent`.
- Use `logger.ErrorAttr` for error attributes.
- Use **ErrorAttr** to log errors consistently.
- Use `logger.RedactAttr` for sensitive attributes.
- Use **RedactAttr** to redact sensitive values.
- Never log passwords, tokens, API keys, cookies, or authorization headers.
- Do not log full payloads by default.
- Do not log high-cardinality values carelessly.
- Do not log secrets even at debug level.
- Prefer stable field names from `internal/logger` constants.
- Do not use `slog.SetDefault`.
- Do not add external logging dependencies without a dedicated plan.

---

## Supported Levels

| Level   | Purpose                        |
|---------|--------------------------------|
| `debug` | Development and troubleshooting |
| `info`  | General operational messages   |
| `warn`  | Recoverable issues             |
| `error` | Unrecoverable failures         |

---

## Supported Formats

| Format | Description                  |
|--------|------------------------------|
| `text` | Human-readable key=value     |
| `json` | Machine-readable JSON lines  |

text and json are formats.

---

## Supported Outputs

| Output   | Description            |
|----------|------------------------|
| `stdout` | Standard output stream |
| `stderr` | Standard error stream  |

stdout is an output, not a format.
stderr is an output, not a format.

---

## unsupported outputs

The following outputs are intentionally not yet supported:

- `file`
- `discard`
- `journald`
- `syslog`
- `network`

See `docs/config.md` for details on why these are not implemented.

---

## Future-Only Features

These features are planned but **not implemented**:

- file output
- log rotation
- runtime config reload
- per-component log levels
- request IDs
- trace IDs
- audit logging
- OpenTelemetry bridge

Do not depend on these in current code.
