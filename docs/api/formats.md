# Plomvix Supported Input Formats

In Sprint 8, Plomvix supports multiple input formats through the `Content-Type` header.

## Supported Formats

| Content-Type | Format | Endpoints |
|---|---|---|
| `application/json` | JSON | all ingest endpoints |
| `text/csv` | CSV with header row | `/ingest/logs`, `/ingest/json` |
| `text/x-logfmt` | Logfmt key=value pairs | `/ingest/logs` |
| `application/x-syslog` | Syslog RFC 5424 or RFC 3164 | `/ingest/logs` |

If `Content-Type` is absent or unrecognised, `application/json` is assumed.

## JSON

Accepted on all ingest endpoints. Existing Sprint 5 wrapper format is still supported:

```bash
curl -X POST http://localhost:8080/ingest/logs \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"records":[{"level":"info","message":"hello world"}]}'
```

`/ingest/logs` and `/ingest/json` also accept bare objects and arrays:

```bash
curl -X POST http://localhost:8080/ingest/logs \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '[{"level":"info","message":"first"},{"level":"warn","message":"second"}]'
```

## CSV

First row is the header. Each subsequent row becomes one record. CSV is supported
on `/ingest/logs` and `/ingest/json`.

```bash
curl -X POST http://localhost:8080/ingest/logs \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: text/csv" \
  --data-binary $'level,message,host\ninfo,server started,web-01\nwarn,high memory,web-02'
```

## Logfmt

One record per non-blank line. Logfmt is supported on `/ingest/logs` only.

```bash
curl -X POST http://localhost:8080/ingest/logs \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: text/x-logfmt" \
  --data-binary $'level=info msg="server started" host=web-01\nlevel=warn msg="high load" host=web-01'
```

## Syslog

Syslog supports RFC 5424 and RFC 3164. Syslog is supported on `/ingest/logs` only.

```bash
curl -X POST http://localhost:8080/ingest/logs \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/x-syslog" \
  --data-binary '<34>1 2024-01-15T10:30:00Z web-01 myapp 1234 ID47 - message here'
```

## Error Responses

| Condition | HTTP Status | Code |
|---|---|---|
| Empty or blank body | 400 | `VALIDATION_FAILED` |
| Malformed input | 400 | `VALIDATION_FAILED` |
| Unsupported format for endpoint | 400 | `VALIDATION_FAILED` |
| Missing required fields for metrics/KV JSON | 400 | `VALIDATION_FAILED` |
