# Plomvix Query API Reference

All query endpoints require authentication.
Use `Authorization: Bearer <token>` or `X-API-Key: <key>`.

---

## GET /query/logs

Query log records by time range with optional filtering.

**Auth:** JWT or API key

**Query parameters:**

| Param | Type | Default | Description |
|---|---|---|---|
| `from` | int64 | 0 | Start timestamp in Unix nanoseconds |
| `to` | int64 | now | End timestamp in Unix nanoseconds |
| `filter` | string | none | Filter expression (see Filter Syntax below) |
| `limit` | int | 100 | Max records to return (max 10000) |
| `offset` | int | 0 | Records to skip (for pagination) |

**Response 200:**
```json
{
  "status": "ok",
  "data": {
    "records": [{"level":"info","message":"hello","timestamp":1700000000000000000}],
    "count": 1,
    "total": 1,
    "limit": 100,
    "offset": 0,
    "query_ms": 2,
    "data_type": "logs"
  }
}
```

**curl example:**
```bash
curl "http://localhost:8080/query/logs?from=1700000000000000000&filter=level%3Dinfo&limit=50" \
  -H "Authorization: Bearer <token>"
```

---

## GET /query/metrics

Query metric records by time range.

**Auth:** JWT or API key

**Additional query parameter:**

| Param | Type | Default | Description |
|---|---|---|---|
| `name` | string | none | Filter by metric name (e.g. `cpu.usage`) |

All other params same as /query/logs.

**curl example:**
```bash
curl "http://localhost:8080/query/metrics?name=cpu.usage&from=1700000000000000000" \
  -H "Authorization: Bearer <token>"
```

---

## GET /query/json

Query JSON document records by time range.

Same parameters as /query/logs.

**curl example:**
```bash
curl "http://localhost:8080/query/json?filter=event%3Dorder_placed" \
  -H "Authorization: Bearer <token>"
```

---

## GET /query/kv/{key}

Retrieve a single key-value record by key.

**Auth:** JWT or API key

**Path parameter:** `key` — the KV key to look up

**Response 200 (found):**
```json
{
  "status": "ok",
  "data": {
    "records": [{"key":"mykey","value":"myval"}],
    "count": 1, "total": 1, "limit": 1, "offset": 0, "query_ms": 1, "data_type": "kv"
  }
}
```

**Response 200 (not found):** count=0, records=[]

**curl example:**
```bash
curl "http://localhost:8080/query/kv/user:alice:session" \
  -H "Authorization: Bearer <token>"
```

---

## GET /query/schema/{type}

Returns the inferred schema for a data type.

**Auth:** JWT or API key

**Path parameter:** `type` — one of: `logs`, `metrics`, `json`, `kv`

**Response 200:**
```json
{
  "status": "ok",
  "data": {
    "data_type": "logs",
    "fields": {"level":"string","message":"string","timestamp":"int64"},
    "updated_at": "2024-01-15T10:30:00Z",
    "record_count": 150
  }
}
```

**Response 400:** invalid type value

**curl example:**
```bash
curl "http://localhost:8080/query/schema/logs" \
  -H "Authorization: Bearer <token>"
```

---

## Filter Syntax

The `filter` query parameter accepts a simple expression:

**Single condition:**
```
filter=level=info
filter=value>50
filter=name!=debug
```

**Multiple conditions (AND only):**
```
filter=level=info AND value>50
filter=level!=debug AND value>=10 AND value<=100
```

**Supported operators:**

| Operator | Meaning |
|---|---|
| `=` | equals |
| `!=` | not equals |
| `>` | greater than (numeric) |
| `<` | less than (numeric) |
| `>=` | greater than or equal |
| `<=` | less than or equal |

URL-encode the filter value when using curl:
- `filter=level=info` → `?filter=level%3Dinfo`
- `filter=level=info AND value>50` → `?filter=level%3Dinfo%20AND%20value%3E50`

Numeric comparisons require the field value to be a number in the record.
If a field is absent in a record, the record is excluded.
OR is not supported — run two separate queries.

---

## Pagination

All time-range endpoints support pagination:

```bash
curl "http://localhost:8080/query/logs?limit=20&offset=40" \
  -H "Authorization: Bearer <token>"
```

Response includes `total` (all matching records) and `count` (records in this page).
Default limit: 100. Maximum limit: 10000.
