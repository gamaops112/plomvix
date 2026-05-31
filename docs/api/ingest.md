# Plomvix Ingest API Reference

All ingest endpoints require authentication.
Use `Authorization: Bearer <token>` or `X-API-Key: <key>`.

All endpoints accept a batch of records via the `records` array.
Minimum 1 record per request.

---

## POST /ingest/logs

Ingest one or more log records.

**Auth:** JWT or API key

**Request body:**
```json
{
  "records": [
    {
      "timestamp": 1700000000000000000,
      "level": "info",
      "message": "user logged in",
      "fields": {"user_id": "abc123"}
    }
  ]
}
```

Field notes:
- timestamp: Unix nanoseconds. Omit or set 0 — server uses current time.
- level: any string (info, warn, error, debug, etc.)
- message: required, non-empty recommended
- fields: optional arbitrary key-value pairs

**Response 201:**
```json
{ "status": "ok", "data": { "ingested": 1, "request_id": "uuid" } }
```

**Response 400:** records array empty or body malformed
**Response 401:** missing or invalid credentials
**Response 500:** WAL or hot tier write failure

**curl example:**
```bash
curl -X POST http://localhost:8080/ingest/logs \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"records":[{"level":"info","message":"hello world"}]}'
```

---

## POST /ingest/metrics

Ingest one or more metric records.

**Auth:** JWT or API key

**Request body:**
```json
{
  "records": [
    {
      "timestamp": 1700000000000000000,
      "name": "cpu.usage",
      "value": 87.5,
      "tags": {"host": "server-01", "region": "in-south"}
    }
  ]
}
```

Field notes:
- name: required, non-empty
- value: float64
- timestamp: optional, defaults to server time
- tags: optional string key-value pairs

**Response 201:**
```json
{ "status": "ok", "data": { "ingested": 1 } }
```
**Response 400:** name missing, records empty, or body malformed

**curl example:**
```bash
curl -X POST http://localhost:8080/ingest/metrics \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"records":[{"name":"cpu.usage","value":42.5}]}'
```

---

## POST /ingest/json

Ingest one or more arbitrary JSON documents.

**Auth:** JWT or API key

**Request body:**
```json
{
  "records": [
    {
      "timestamp": 1700000000000000000,
      "data": {
        "event": "order_placed",
        "amount": 299.99,
        "currency": "INR"
      }
    }
  ]
}
```

Field notes:
- data: required, must be a JSON object (not null, not array)
- timestamp: optional, defaults to server time
- Schema is inferred from top-level fields of data

**Response 201:**
```json
{ "status": "ok", "data": { "ingested": 1 } }
```
**Response 400:** data null/missing, records empty, or body malformed

**curl example:**
```bash
curl -X POST http://localhost:8080/ingest/json \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"records":[{"data":{"event":"signup","plan":"pro"}}]}'
```

---

## POST /ingest/kv

Ingest one or more key-value pairs.

**Auth:** JWT or API key

**Request body:**
```json
{
  "records": [
    {
      "key": "user:alice:session",
      "value": "token_abc123"
    }
  ]
}
```

Field notes:
- key: required, non-empty string
- value: string, stored as-is
- No schema inference for kv — keys are user-defined

**Response 201:**
```json
{ "status": "ok", "data": { "ingested": 1 } }
```
**Response 400:** key empty, records empty, or body malformed

**curl example:**
```bash
curl -X POST http://localhost:8080/ingest/kv \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"records":[{"key":"config:max_retries","value":"5"}]}'
```

---

## Schema Inference

Plomvix automatically infers the schema of ingested JSON records.
Schemas are stored in RocksDB and updated on every ingest call.

**Inferred types:**

| JSON value | Type |
|---|---|
| true/false | bool |
| whole number | int64 |
| decimal number | float64 |
| string | string |
| null | null |
| object | object |
| array | array |

If the same field is seen with different types across records, its type becomes `mixed`.
Schema is available via the query API (Sprint 6).

---

## Batch Ingestion

All endpoints support batching. Send multiple records in one request:

```bash
curl -X POST http://localhost:8080/ingest/logs \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "records": [
      {"level":"info","message":"first"},
      {"level":"warn","message":"second"},
      {"level":"error","message":"third"}
    ]
  }'
```

**Response:**
```json
{ "status": "ok", "data": { "ingested": 3 } }
```
